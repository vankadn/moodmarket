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
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ports.ErrProfileNotFound) {
		return nil, fmt.Errorf("recommendation service: fetch profile: %w", err)
	}

	snapshot, err := s.marketData.GetDailySnapshot(ctx)
	if err != nil {
		log.Printf("recommendation service: market data unavailable (%v) — proceeding without it", err)
		snapshot = nil
	}

	// Enrich with live Plaid balances if the user has connected accounts.
	connections, err := s.profileRepo.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("recommendation service: fetch plaid connections (%v) — proceeding without bank data", err)
	} else if len(connections) > 0 {
		summary, err := s.financialData.GetBalanceSummary(ctx, connections)
		if err != nil {
			log.Printf("recommendation service: fetch balance summary (%v) — proceeding without bank data", err)
		} else {
			req.BalanceSummary = &summary
		}
	}

	rec, err := s.advisor.GetRecommendation(ctx, req, profile, snapshot)
	if err != nil {
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}

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
		log.Printf("recommendation service: save decision: %v", err)
	}

	return rec, nil
}
