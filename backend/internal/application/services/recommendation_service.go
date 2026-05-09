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
	advisor          ports.InvestmentAdvisor
	profileRepo      ports.ProfileRepository
	marketData       ports.MarketDataProvider
	decisionRepo     ports.DecisionRepository
	financialData    ports.FinancialDataProvider
	brokerageFactory ports.BrokerageProviderFactory
	news             ports.NewsProvider
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
	financialData ports.FinancialDataProvider,
	brokerageFactory ports.BrokerageProviderFactory,
	news ports.NewsProvider,
) *RecommendationService {
	return &RecommendationService{
		advisor:          advisor,
		profileRepo:      profileRepo,
		marketData:       marketData,
		decisionRepo:     decisionRepo,
		financialData:    financialData,
		brokerageFactory: brokerageFactory,
		news:             news,
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
		log.Printf("[recommend] step 1/7  profile loaded")
	} else {
		log.Printf("[recommend] step 1/7  no profile found — using balanced defaults")
	}

	// Step 2: market snapshot
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("[recommend] step 2/7  market data unavailable (%v) — skipped", err)
		snapshot = nil
	} else if snapshot != nil {
		log.Printf("[recommend] step 2/7  market snapshot loaded (SPY %+.2f%% QQQ %+.2f%% sentiment=%s)", snapshot.SPYChangePercent, snapshot.QQQChangePercent, snapshot.MarketSentiment)
	}

	// Step 3: Plaid bank balances
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("[recommend] step 3/8  plaid connections fetch failed (%v) — skipped", err)
	} else if len(connections) == 0 {
		log.Printf("[recommend] step 3/8  no bank accounts connected — Claude will use profile estimates only")
	} else {
		log.Printf("[recommend] step 3/8  %d plaid connection(s) found — fetching live balances", len(connections))

		summary, err := s.financialData.GetBalanceSummary(ctx, connections)
		if err != nil {
			log.Printf("[recommend] step 3/8  balance fetch failed (%v) — skipped", err)
		} else {
			req.BalanceSummary = &summary
			log.Printf("[recommend] step 3/8  balances loaded (%d accounts)", summary.AccountCount)
		}

		// Step 4: Plaid transaction history — Claude can flag large pending charges
		txSummary, err := s.financialData.GetTransactionSummary(ctx, connections)
		if err != nil {
			log.Printf("[recommend] step 4/8  transaction fetch failed (%v) — skipped", err)
		} else {
			req.TransactionSummary = &txSummary
			log.Printf("[recommend] step 4/8  transactions loaded (7d=$%.0f 30d=$%.0f pending=%s)", txSummary.SpendLast7Days, txSummary.SpendLast30Days, txSummary.LargestPendingName)
			// TODO: remove after 8c testing
			log.Printf("[8c-debug] spend 7d=$%.2f 30d=$%.2f largest_pending=%s $%.2f", txSummary.SpendLast7Days, txSummary.SpendLast30Days, txSummary.LargestPendingName, txSummary.LargestPendingAmount)
			// Spending spike: compare 7-day to a normal week (30d avg / 4)
			if txSummary.SpendLast30Days > 0 {
				avgWeekly := txSummary.SpendLast30Days / 4
				spikePct := (txSummary.SpendLast7Days - avgWeekly) / avgWeekly * 100
				if spikePct > 20 {
					log.Printf("[8c-debug] spending spike: this week $%.2f vs avg week $%.2f (+%.0f%%)", txSummary.SpendLast7Days, avgWeekly, spikePct)
				} else if spikePct < -20 {
					log.Printf("[8c-debug] spending dip: this week $%.2f vs avg week $%.2f (%.0f%%)", txSummary.SpendLast7Days, avgWeekly, spikePct)
				} else {
					log.Printf("[8c-debug] spending normal: this week $%.2f vs avg week $%.2f (%+.0f%%)", txSummary.SpendLast7Days, avgWeekly, spikePct)
				}
			}
			// Cash runway using balance summary if available
			if req.BalanceSummary != nil && txSummary.SpendLast30Days > 0 {
				dailySpend := txSummary.SpendLast30Days / 30
				runway := int(req.BalanceSummary.TotalCash / dailySpend)
				log.Printf("[8c-debug] cash runway: $%.2f cash ÷ $%.2f/day = ~%d days", req.BalanceSummary.TotalCash, dailySpend, runway)
			}
			if txSummary.LargestPendingAmount > 0 {
				log.Printf("[8c-debug] largest pending: %s $%.2f — flagged to Claude", txSummary.LargestPendingName, txSummary.LargestPendingAmount)
			}
		}
	}

	// Step 5: current brokerage positions — Claude avoids over-concentrating existing holdings
	brokerageConn, _ := s.profileRepo.GetBrokerageConnection(ctx, userID)
	brokerageProvider, err := s.brokerageFactory.ForUser(brokerageConn)
	if err != nil {
		log.Printf("[recommend] step 5/8  brokerage not connected — skipping positions")
	} else {
		positions, err := brokerageProvider.GetPositions(ctx, userID)
		if err != nil {
			log.Printf("[recommend] step 5/8  positions fetch failed (%v) — skipped", err)
		} else {
			req.Positions = positions
			log.Printf("[recommend] step 5/8  %d position(s) loaded", len(positions))
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
	}

	// Step 6: recent decision history — Claude avoids repeating the same allocation daily
	recentDecisions, err := s.decisionRepo.ListByUser(ctx, userID, 10)
	if err != nil {
		log.Printf("[recommend] step 6/8  recent decisions fetch failed (%v) — skipped", err)
	} else {
		req.RecentDecisions = recentDecisions
		log.Printf("[recommend] step 6/8  %d recent decision(s) loaded", len(recentDecisions))
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

	// Step 7: today's market news — Claude can factor in macro events and breaking news
	newsItems, err := s.news.GetDailyNews(ctx)
	if err != nil {
		log.Printf("[recommend] step 7/8  news fetch failed (%v) — skipped", err)
	} else {
		req.NewsItems = newsItems
		log.Printf("[recommend] step 7/8  %d headline(s) loaded", len(newsItems))
		// TODO: remove after 8b testing
		for i, n := range newsItems {
			if i >= 5 {
				break
			}
			log.Printf("[8b-debug] news[%d]: [%s] %s", i, n.Source, n.Headline)
		}
	}

	// Step 8: Claude generates allocation
	log.Printf("[recommend] step 8/8  sending to Claude (profile=%v market=%v plaid=%v txns=%v positions=%d decisions=%d news=%d)", profile != nil, snapshot != nil, req.BalanceSummary != nil, req.TransactionSummary != nil, len(req.Positions), len(req.RecentDecisions), len(req.NewsItems))
	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	log.Printf("[recommend] step 8/8  Claude returned %d allocations (risk=%s)", len(rec.Allocations), rec.RiskLevel)

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

// GetCashContext returns a pre-computed spending insight for the given user.
// Called by the frontend before the user taps invest so the UI can surface a nudge.
// Returns has_data=false when no Plaid connections exist or data is unavailable —
// the frontend renders nothing in that case.
func (s *RecommendationService) GetCashContext(ctx context.Context, userID string) (models.CashContext, error) {
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil || len(connections) == 0 {
		log.Printf("[cash-context] no plaid connections for user %s — returning no data", userID)
		return models.CashContext{HasData: false}, nil
	}

	balance, err := s.financialData.GetBalanceSummary(ctx, connections)
	if err != nil {
		log.Printf("[cash-context] balance fetch failed: %v — returning no data", err)
		return models.CashContext{HasData: false}, nil
	}

	txSummary, err := s.financialData.GetTransactionSummary(ctx, connections)
	if err != nil {
		log.Printf("[cash-context] transaction fetch failed: %v — returning no data", err)
		return models.CashContext{HasData: false}, nil
	}

	// Need spend data to be meaningful; zero spend means no transactions came through
	if txSummary.SpendLast30Days == 0 {
		log.Printf("[cash-context] no spend data available — returning no data")
		return models.CashContext{HasData: false}, nil
	}

	dailySpend := txSummary.SpendLast30Days / 30
	runwayDays := int(balance.TotalCash / dailySpend)

	label, message := runwayLabelAndMessage(runwayDays)

	log.Printf("[cash-context] runway=%d days label=%s", runwayDays, label)
	// TODO: remove after 8ca testing
	log.Printf("[8ca-debug] cash-context computed: runway=%d label=%s", runwayDays, label)

	return models.CashContext{
		HasData:              true,
		RunwayDays:           runwayDays,
		RunwayLabel:          label,
		SpendLast7D:          txSummary.SpendLast7Days,
		SpendLast30D:         txSummary.SpendLast30Days,
		LargestPendingAmount: txSummary.LargestPendingAmount,
		LargestPendingName:   txSummary.LargestPendingName,
		Message:              message,
	}, nil
}

func runwayLabelAndMessage(days int) (string, string) {
	switch {
	case days > 30:
		return "healthy", "Your cash position looks strong."
	case days >= 14:
		return "moderate", "Moderate cash runway. Proceed as planned or adjust."
	default:
		return "tight", fmt.Sprintf("Your cash covers about %d days of typical spending. You may want to invest a smaller amount today.", days)
	}
}
