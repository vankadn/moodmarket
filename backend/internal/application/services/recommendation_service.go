// application/services/recommendation_service.go
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type RecommendationService struct {
	advisor       ports.InvestmentAdvisor
	profileRepo   ports.ProfileRepository
	marketData    ports.MarketDataProvider
	decisionRepo  ports.DecisionRepository
	financialData ports.FinancialDataProvider
	brokerage     ports.BrokerageProvider
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
	financialData ports.FinancialDataProvider,
	brokerage ports.BrokerageProvider,
) *RecommendationService {
	return &RecommendationService{
		advisor:       advisor,
		profileRepo:   profileRepo,
		marketData:    marketData,
		decisionRepo:  decisionRepo,
		financialData: financialData,
		brokerage:     brokerage,
	}
}

// GetDailyRecommendation fetches the user's profile, today's market snapshot, and live Plaid
// balances (if accounts are connected), then asks the advisor for an allocation and logs the decision.
// Market data and Plaid failures are both non-fatal — the recommendation is returned regardless.
func (s *RecommendationService) GetDailyRecommendation(ctx context.Context, userID string, req models.InvestmentRequest) (*models.Recommendation, error) {
	log.Printf("[recommend] ── START for user %s ──────────────────────────────────", userID)

	// Step 1: user profile
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ports.ErrProfileNotFound) {
		return nil, fmt.Errorf("recommendation service: fetch profile: %w", err)
	}
	if profile != nil {
		log.Printf("[recommend] step 1/6  profile loaded")
	} else {
		log.Printf("[recommend] step 1/6  no profile found — using balanced defaults")
	}

	// Step 2: market snapshot
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("[recommend] step 2/6  market data unavailable (%v) — skipped", err)
		snapshot = nil
	} else if snapshot != nil {
		log.Printf("[recommend] step 2/6  market snapshot loaded (SPY %+.2f%% QQQ %+.2f%% sentiment=%s)", snapshot.SPYChangePercent, snapshot.QQQChangePercent, snapshot.MarketSentiment)
	}

	// Step 3: Plaid bank balances
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("[recommend] step 3/6  plaid connections fetch failed (%v) — skipped", err)
	} else if len(connections) == 0 {
		log.Printf("[recommend] step 3/6  no bank accounts connected — Claude will use profile estimates only")
	} else {
		log.Printf("[recommend] step 3/6  %d plaid connection(s) found — fetching live balances", len(connections))

		summary, err := s.financialData.GetBalanceSummary(ctx, connections)
		if err != nil {
			log.Printf("[recommend] step 3/6  balance fetch failed (%v) — skipped", err)
		} else {
			req.BalanceSummary = &summary
			log.Printf("[recommend] step 3/6  balances loaded (%d accounts)", summary.AccountCount)
		}
	}

	// Step 4: current brokerage positions — Claude avoids over-concentrating existing holdings
	positions, err := s.brokerage.GetPositions(ctx, userID)
	if err != nil {
		log.Printf("[recommend] step 4/6  positions fetch failed (%v) — skipped", err)
	} else {
		req.Positions = positions
		log.Printf("[recommend] step 4/6  %d position(s) loaded", len(positions))
		// TODO: remove after 8a testing
		var totalVal float64
		for _, p := range positions {
			totalVal += p.MarketValue
		}
		for _, p := range positions {
			pct := 0.0
			if totalVal > 0 {
				pct = p.MarketValue / totalVal * 100
			}
			log.Printf("[8a-debug] position: %s $%.2f (%.0f%%)", p.Ticker, p.MarketValue, pct)
		}
	}

	// Step 5: recent decision history — Claude avoids repeating the same allocation daily
	recentDecisions, err := s.decisionRepo.ListByUser(ctx, userID, 10)
	if err != nil {
		log.Printf("[recommend] step 5/6  recent decisions fetch failed (%v) — skipped", err)
	} else {
		req.RecentDecisions = recentDecisions
		log.Printf("[recommend] step 5/6  %d recent decision(s) loaded", len(recentDecisions))
		// TODO: remove after 8a testing
		for i, d := range recentDecisions {
			if i >= 5 {
				break
			}
			tickers := make([]string, len(d.Allocations))
			for j, a := range d.Allocations {
				tickers[j] = fmt.Sprintf("%s %.0f%%", a.Ticker, a.Percentage)
			}
			log.Printf("[8a-debug] history[%d]: %s $%.0f — %s", i, d.Timestamp.Format("Jan 2"), d.TotalAmount, strings.Join(tickers, ", "))
		}
	}

	// Step 6: Claude generates allocation
	log.Printf("[recommend] step 6/6  sending to Claude (profile=%v market=%v plaid=%v positions=%d decisions=%d)", profile != nil, snapshot != nil, req.BalanceSummary != nil, len(req.Positions), len(req.RecentDecisions))
	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	log.Printf("[recommend] step 6/6  Claude returned %d allocations (risk=%s)", len(rec.Allocations), rec.RiskLevel)

	// Persist decision
	decision := &models.InvestmentDecision{
		UserID:         userID,
		Timestamp:      time.Now(),
		MarketSnapshot: snapshot,
		Allocations:    rec.Allocations,
		TotalAmount:    rec.TotalBudget,
		RiskLevel:      rec.RiskLevel,
		Summary:        rec.Summary,
	}
	if err := s.decisionRepo.Save(ctx, decision); err != nil {
		log.Printf("[recommend] persist  save decision failed: %v", err)
	} else {
		log.Printf("[recommend] persist  decision saved to MongoDB")
	}

	log.Printf("[recommend] ── DONE ─────────────────────────────────────────────────")
	return rec, nil
}
