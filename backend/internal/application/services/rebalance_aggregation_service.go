// application/services/rebalance_aggregation_service.go
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// RebalanceAggregationService assembles a RebalanceRequest from live brokerage positions,
// SnapTrade external holdings, and MongoDB decision history.
// The assembled request is passed directly to RebalanceAdvisor — no data fetching in the handler.
type RebalanceAggregationService struct {
	brokerageFactory    ports.BrokerageProviderFactory
	profileRepo         ports.ProfileRepository
	decisionRepo        ports.DecisionRepository
	portfolioAggregator ports.PortfolioAggregator
}

func NewRebalanceAggregationService(
	brokerageFactory ports.BrokerageProviderFactory,
	profileRepo ports.ProfileRepository,
	decisionRepo ports.DecisionRepository,
	portfolioAggregator ports.PortfolioAggregator,
) *RebalanceAggregationService {
	return &RebalanceAggregationService{
		brokerageFactory:    brokerageFactory,
		profileRepo:         profileRepo,
		decisionRepo:        decisionRepo,
		portfolioAggregator: portfolioAggregator,
	}
}

// BuildRequest assembles a RebalanceRequest for the given user.
//
// Assembly order:
//  1. Alpaca positions — authoritative InvestIQ-managed holdings with real P&L
//  2. SnapTrade external holdings — per-account, tagged with institution name
//  3. Alpaca-first deduplication by ticker (first-seen wins)
//  4. Buy reasoning + first purchase date from MongoDB decision history
func (s *RebalanceAggregationService) BuildRequest(ctx context.Context, userID string) (*models.RebalanceRequest, error) {
	seen := make(map[string]bool)
	var positions []models.RebalancePosition

	// Step 1: Alpaca positions.
	brokerageConns, err := s.profileRepo.GetBrokerageConnections(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("rebalance aggregation: load brokerage connections: %w", err)
	}
	alpacaCount := 0
	for i := range brokerageConns {
		provider, err := s.brokerageFactory.ForUser(&brokerageConns[i])
		if err != nil {
			log.Printf("[rebalance] build provider for %s failed (%v) — skipped", brokerageConns[i].ID, err)
			continue
		}
		alpacaPositions, err := provider.GetPositions(ctx, userID)
		if err != nil {
			log.Printf("[rebalance] positions fetch from %s failed (%v) — skipped", brokerageConns[i].ID, err)
			continue
		}
		for _, p := range alpacaPositions {
			if seen[p.Ticker] {
				continue
			}
			seen[p.Ticker] = true
			positions = append(positions, models.RebalancePosition{
				Position: p,
				Source:   "alpaca",
			})
			alpacaCount++
		}
	}
	log.Printf("[rebalance] step 1: %d Alpaca position(s) across %d connection(s)", alpacaCount, len(brokerageConns))

	// Step 2: SnapTrade external holdings, per-account so each position carries its institution name.
	portfolioConn, _ := s.profileRepo.GetPortfolioConnection(ctx, userID)
	if portfolioConn != nil {
		byAccount, err := s.portfolioAggregator.GetHoldingsByAccount(ctx, portfolioConn.ProviderUserID, portfolioConn.ProviderUserSecret)
		if err != nil {
			log.Printf("[rebalance] SnapTrade holdings fetch failed (%v) — skipped", err)
		} else {
			snapCount := 0
			for accountName, accountPositions := range byAccount {
				for _, p := range accountPositions {
					if seen[p.Ticker] {
						continue
					}
					seen[p.Ticker] = true
					positions = append(positions, models.RebalancePosition{
						Position:    p,
						Source:      "snaptrade",
						AccountName: accountName,
					})
					snapCount++
				}
			}
			log.Printf("[rebalance] step 2: %d SnapTrade position(s) merged across %d account(s)", snapCount, len(byAccount))
		}
	}
	log.Printf("[rebalance] %d total position(s) assembled (Alpaca-first deduplication applied)", len(positions))

	// Steps 3 & 4: scan decision history for buy reasoning and first purchase dates.
	// ListByUser returns newest-first. We need:
	//   - BuyReasoningByTicker: most recent TickerReasoning entry per ticker (newest-first, first-seen wins)
	//   - FirstPurchaseByTicker: earliest decision timestamp where a ticker appears in Receipts (oldest-first)
	// Using 200 decisions as a practical cap (~6 months of daily investing).
	decisions, err := s.decisionRepo.ListByUser(ctx, userID, 200)
	if err != nil {
		log.Printf("[rebalance] decision history fetch failed (%v) — buy reasoning and tax flags will be unavailable", err)
	}

	buyReasoning := make(map[string]string)
	firstPurchase := make(map[string]time.Time)

	// Newest-first pass: most recent TickerReasoning wins.
	for _, d := range decisions {
		for ticker, reasoning := range d.TickerReasoning {
			if _, exists := buyReasoning[ticker]; !exists {
				buyReasoning[ticker] = reasoning
			}
		}
	}

	// Oldest-first pass: earliest receipt timestamp per ticker.
	for i := len(decisions) - 1; i >= 0; i-- {
		d := decisions[i]
		for _, r := range d.Receipts {
			if r.Ticker == "" {
				continue
			}
			if _, exists := firstPurchase[r.Ticker]; !exists {
				ts := r.Timestamp
				if ts.IsZero() {
					ts = d.Timestamp
				}
				firstPurchase[r.Ticker] = ts
			}
		}
	}

	log.Printf("[rebalance] step 3/4: %d ticker reasoning(s), %d first-purchase date(s) from %d decision(s)",
		len(buyReasoning), len(firstPurchase), len(decisions))

	return &models.RebalanceRequest{
		Positions:             positions,
		BuyReasoningByTicker:  buyReasoning,
		FirstPurchaseByTicker: firstPurchase,
	}, nil
}
