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
	advisor       ports.InvestmentAdvisor
	profileRepo   ports.ProfileRepository
	marketData    ports.MarketDataProvider
	decisionRepo  ports.DecisionRepository
	financialData ports.FinancialDataProvider
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
	financialData ports.FinancialDataProvider,
) *RecommendationService {
	return &RecommendationService{
		advisor:       advisor,
		profileRepo:   profileRepo,
		marketData:    marketData,
		decisionRepo:  decisionRepo,
		financialData: financialData,
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
		log.Printf("[recommend] step 1/5  profile loaded")
	} else {
		log.Printf("[recommend] step 1/5  no profile found — using balanced defaults")
	}

	// Step 2: market snapshot
	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("[recommend] step 2/5  market data unavailable (%v) — skipped", err)
		snapshot = nil
	} else if snapshot != nil {
		log.Printf("[recommend] step 2/5  market snapshot loaded (SPY %+.2f%% QQQ %+.2f%% sentiment=%s)", snapshot.SPYChangePercent, snapshot.QQQChangePercent, snapshot.MarketSentiment)
	}

	// Step 3: Plaid bank balances
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("[recommend] step 3/5  plaid connections fetch failed (%v) — skipped", err)
	} else if len(connections) == 0 {
		log.Printf("[recommend] step 3/5  no bank accounts connected — Claude will use profile estimates only")
	} else {
		log.Printf("[recommend] step 3/5  %d plaid connection(s) found — fetching live balances", len(connections))

		summary, err := s.financialData.GetBalanceSummary(ctx, connections)
		if err != nil {
			log.Printf("[recommend] step 3/5  balance fetch failed (%v) — skipped", err)
		} else {
			req.BalanceSummary = &summary
			log.Printf("[recommend] step 3/5  balances loaded (%d accounts)", summary.AccountCount)
		}
	}

	// Step 4: Claude generates allocation
	log.Printf("[recommend] step 4/5  sending to Claude (profile=%v market=%v plaid=%v)", profile != nil, snapshot != nil, req.BalanceSummary != nil)
	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	log.Printf("[recommend] step 4/5  Claude returned %d allocations (risk=%s)", len(rec.Allocations), rec.RiskLevel)

	// Step 5: persist decision
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
		log.Printf("[recommend] step 5/5  save decision failed: %v", err)
	} else {
		log.Printf("[recommend] step 5/5  decision saved to MongoDB")
	}

	log.Printf("[recommend] ── DONE ─────────────────────────────────────────────────")
	return rec, nil
}
