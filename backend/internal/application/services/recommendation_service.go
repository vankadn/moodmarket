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
	advisor          ports.InvestmentAdvisor
	profileRepo      ports.ProfileRepository
	marketData       ports.MarketDataProvider
	decisionRepo     ports.DecisionRepository
	financialData    ports.FinancialDataProvider
	brokerageFactory ports.BrokerageProviderFactory
	documentRepo     ports.DocumentRepository
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
	financialData ports.FinancialDataProvider,
	brokerageFactory ports.BrokerageProviderFactory,
	documentRepo ports.DocumentRepository,
) *RecommendationService {
	return &RecommendationService{
		advisor:          advisor,
		profileRepo:      profileRepo,
		marketData:       marketData,
		decisionRepo:     decisionRepo,
		financialData:    financialData,
		brokerageFactory: brokerageFactory,
		documentRepo:     documentRepo,
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
		}
	}

	// Step 6: recent decision history — Claude avoids repeating the same allocation daily
	recentDecisions, err := s.decisionRepo.ListByUser(ctx, userID, 10)
	if err != nil {
		log.Printf("[recommend] step 6/8  recent decisions fetch failed (%v) — skipped", err)
	} else {
		req.RecentDecisions = recentDecisions
		log.Printf("[recommend] step 6/8  %d recent decision(s) loaded", len(recentDecisions))
	}

	// Step 7: tax documents — gives Claude income, withholding, and housing context
	taxDocs, err := s.documentRepo.GetByUserID(ctx, userID)
	if err != nil {
		log.Printf("[recommend] step 7/8  tax documents fetch failed (%v) — skipped", err)
	} else {
		req.TaxDocuments = taxDocs
		log.Printf("[recommend] step 7/8  %d tax document(s) loaded", len(taxDocs))
	}

	// Step 8: Claude generates allocation (fetches market news itself via get_market_news tool)
	log.Printf("[recommend] step 8/8  sending to Claude (profile=%v market=%v plaid=%v txns=%v positions=%d decisions=%d taxdocs=%d)", profile != nil, snapshot != nil, req.BalanceSummary != nil, req.TransactionSummary != nil, len(req.Positions), len(req.RecentDecisions), len(req.TaxDocuments))
	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	log.Printf("[recommend] step 7/7  Claude returned %d allocations (risk=%s)", len(rec.Allocations), rec.RiskLevel)

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
