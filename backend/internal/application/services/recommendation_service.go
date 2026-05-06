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
	advisor      ports.InvestmentAdvisor
	profileRepo  ports.ProfileRepository
	marketData   ports.MarketDataProvider
	decisionRepo ports.DecisionRepository
}

func NewRecommendationService(
	advisor ports.InvestmentAdvisor,
	profileRepo ports.ProfileRepository,
	marketData ports.MarketDataProvider,
	decisionRepo ports.DecisionRepository,
) *RecommendationService {
	return &RecommendationService{
		advisor:      advisor,
		profileRepo:  profileRepo,
		marketData:   marketData,
		decisionRepo: decisionRepo,
	}
}

// GetDailyRecommendation fetches the user's profile and today's market snapshot,
// asks the advisor for an allocation, then logs the decision to the repository.
// Market data failure and decision save failure are both non-fatal — the
// recommendation is returned to the caller regardless.
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
