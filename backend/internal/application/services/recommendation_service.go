// application/services/recommendation_service.go
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type RecommendationService struct {
	advisor              ports.InvestmentAdvisor
	profileRepo          ports.ProfileRepository
	marketData           ports.MarketDataProvider
	decisionRepo         ports.DecisionRepository
	financialData        ports.FinancialDataProvider
	brokerageFactory     ports.BrokerageProviderFactory
	documentRepo         ports.DocumentRepository
	portfolioAggregator  ports.PortfolioAggregator
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
	financialData ports.FinancialDataProvider,
	brokerageFactory ports.BrokerageProviderFactory,
	documentRepo ports.DocumentRepository,
	portfolioAggregator ports.PortfolioAggregator,
) *RecommendationService {
	return &RecommendationService{
		advisor:             advisor,
		profileRepo:         profileRepo,
		marketData:          marketData,
		decisionRepo:        decisionRepo,
		financialData:       financialData,
		brokerageFactory:    brokerageFactory,
		documentRepo:        documentRepo,
		portfolioAggregator: portfolioAggregator,
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
		log.Printf("[recommend] step 1/8  profile loaded")
	} else {
		log.Printf("[recommend] step 1/8  no profile found — using balanced defaults")
	}

	// Step 2: market snapshot
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("[recommend] step 2/8  market data unavailable (%v) — skipped", err)
		snapshot = nil
	} else if snapshot != nil {
		log.Printf("[recommend] step 2/8  market snapshot loaded (SPY %+.2f%% QQQ %+.2f%% sentiment=%s)", snapshot.SPYChangePercent, snapshot.QQQChangePercent, snapshot.MarketSentiment)
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
		}
	}

	// Steps 5 & 5b: current positions — Alpaca connections first, then external aggregator.
	// seen deduplicates by ticker across both sources (first-seen wins).
	seen := make(map[string]bool)

	brokerageConns, _ := s.profileRepo.GetBrokerageConnections(ctx, userID)
	if len(brokerageConns) == 0 {
		log.Printf("[recommend] step 5/9   brokerage not connected — skipping positions")
	} else {
		for i := range brokerageConns {
			provider, err := s.brokerageFactory.ForUser(&brokerageConns[i])
			if err != nil {
				log.Printf("[recommend] step 5/9   build provider for %s failed (%v) — skipped", brokerageConns[i].ID, err)
				continue
			}
			positions, err := provider.GetPositions(ctx, userID)
			if err != nil {
				log.Printf("[recommend] step 5/9   positions fetch from %s failed (%v) — skipped", brokerageConns[i].ID, err)
				continue
			}
			for _, p := range positions {
				if seen[p.Ticker] {
					log.Printf("[recommend] step 5/9   duplicate ticker %s from %s — skipped", p.Ticker, brokerageConns[i].ID)
					continue
				}
				seen[p.Ticker] = true
				req.Positions = append(req.Positions, p)
			}
		}
		log.Printf("[recommend] step 5/9   %d position(s) loaded across %d connection(s)", len(req.Positions), len(brokerageConns))
	}

	// Step 5b: external portfolio holdings merged with Alpaca positions; ticker deduplication continues.
	portfolioConn, _ := s.profileRepo.GetPortfolioConnection(ctx, userID)
	if portfolioConn == nil {
		log.Printf("[recommend] step 5b/9  no portfolio aggregator connected — skipping external holdings")
	} else {
		log.Printf("[recommend] step 5b/9  fetching external holdings (provider=%s)", portfolioConn.Provider)
		extPositions, err := s.portfolioAggregator.GetHoldings(ctx, portfolioConn.ProviderUserID, portfolioConn.ProviderUserSecret)
		if err != nil {
			log.Printf("[recommend] step 5b/9  external holdings fetch failed (%v) — skipped", err)
		} else {
			log.Printf("[recommend] step 5b/9  received %d position(s) from portfolio aggregator", len(extPositions))
			added := 0
			var merged, duped []string
			for _, p := range extPositions {
				if seen[p.Ticker] {
					duped = append(duped, p.Ticker)
					continue
				}
				seen[p.Ticker] = true
				req.Positions = append(req.Positions, p)
				merged = append(merged, p.Ticker)
				added++
			}
			if len(merged) > 0 {
				log.Printf("[recommend] step 5b/9  merged tickers: %v", merged)
			}
			if len(duped) > 0 {
				log.Printf("[recommend] step 5b/9  skipped duplicate tickers: %v", duped)
			}
			log.Printf("[recommend] step 5b/9  %d external position(s) merged, total positions now %d", added, len(req.Positions))
		}
	}

	// Step 6: recent decision history — Claude avoids repeating the same allocation daily
	recentDecisions, err := s.decisionRepo.ListByUser(ctx, userID, 10)
	if err != nil {
		log.Printf("[recommend] step 6/9  recent decisions fetch failed (%v) — skipped", err)
	} else {
		req.RecentDecisions = recentDecisions
		log.Printf("[recommend] step 6/9  %d recent decision(s) loaded", len(recentDecisions))
	}

	// Step 6.5: stamp verdicts for any decisions older than 24h that haven't been evaluated yet.
	// Runs concurrently with step 7 so tax doc fetch and Polygon calls happen in parallel.
	// Must complete before step 8 so GetEvalSummary reflects freshly-stamped verdicts.
	log.Printf("[recommend] step 6.5/9  verdict stamping started")
	stampDone := make(chan struct{})
	go func() {
		defer close(stampDone)
		stampVerdicts(ctx, userID, 24*time.Hour,
			s.decisionRepo, s.profileRepo, s.brokerageFactory, s.marketData)
	}()

	// Step 7: tax documents — gives Claude income, withholding, and housing context
	taxDocs, err := s.documentRepo.GetByUserID(ctx, userID)
	if err != nil {
		log.Printf("[recommend] step 7/9  tax documents fetch failed (%v) — skipped", err)
	} else {
		req.TaxDocuments = taxDocs
		log.Printf("[recommend] step 7/9  %d tax document(s) loaded", len(taxDocs))
	}

	<-stampDone
	log.Printf("[recommend] step 6.5/9  verdict stamping complete")

	// Step 8: performance summary — gives Claude feedback on its own track record for this user.
	// Non-fatal; omitted silently if below the 5-verdict threshold or on any error.
	const minVerdictsForFeedback = 5
	if ps, psErr := s.decisionRepo.GetEvalSummary(ctx, userID); psErr != nil {
		log.Printf("[recommend] step 8/9  performance summary fetch failed (%v) — skipped", psErr)
	} else if ps != nil && ps.VerdictedDecisions >= minVerdictsForFeedback {
		req.PerformanceSummary = ps
		log.Printf("[recommend] step 8/9  performance summary loaded (verdicts=%d winRate=%.0f%%)", ps.VerdictedDecisions, ps.WinRate*100)
	} else {
		log.Printf("[recommend] step 8/9  performance summary skipped (verdicts=%d, threshold=%d)", func() int {
			if ps != nil {
				return ps.VerdictedDecisions
			}
			return 0
		}(), minVerdictsForFeedback)
	}

	// Step 9: Claude generates allocation (fetches market news itself via get_market_news tool)
	log.Printf("[recommend] step 9/9  sending to Claude (profile=%v market=%v plaid=%v txns=%v positions=%d decisions=%d taxdocs=%d perf=%v)", profile != nil, snapshot != nil, req.BalanceSummary != nil, req.TransactionSummary != nil, len(req.Positions), len(req.RecentDecisions), len(req.TaxDocuments), req.PerformanceSummary != nil)
	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		if errors.Is(err, ports.ErrAdvisorOverloaded) {
			cached, fallbackErr := s.cachedRecommendation(ctx, userID, req.BaseBudget)
			if fallbackErr == nil {
				log.Printf("[recommend] step 9/9  advisor overloaded — returning cached recommendation from %s", cached.Summary)
				return cached, nil
			}
			log.Printf("[recommend] step 9/9  advisor overloaded and no cached recommendation available: %v", fallbackErr)
		}
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	log.Printf("[recommend] step 9/9  Claude returned %d allocations (risk=%s)", len(rec.Allocations), rec.RiskLevel)

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

// cachedRecommendation pulls the user's most recent investment decision from MongoDB and
// rescales the allocation amounts to budget. Returns an error when no prior decision exists.
func (s *RecommendationService) cachedRecommendation(ctx context.Context, userID string, budget float64) (*models.Recommendation, error) {
	decisions, err := s.decisionRepo.ListByUser(ctx, userID, 1)
	if err != nil || len(decisions) == 0 {
		return nil, fmt.Errorf("no cached decision available")
	}
	d := decisions[0]
	if len(d.Allocations) == 0 {
		return nil, fmt.Errorf("cached decision has no allocations")
	}

	// Rescale amounts to the requested budget while preserving percentages.
	scale := budget / d.TotalAmount
	allocs := make([]models.Allocation, len(d.Allocations))
	for i, a := range d.Allocations {
		allocs[i] = a
		allocs[i].Amount = a.Amount * scale
	}

	return &models.Recommendation{
		TotalBudget: budget,
		Allocations: allocs,
		Summary:     d.Summary,
		RiskLevel:   d.RiskLevel,
		FromCache:   true,
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
