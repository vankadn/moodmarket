package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type RecommendationService struct {
	advisor     ports.InvestmentAdvisor
	profileRepo ports.ProfileRepository
}

func NewRecommendationService(advisor ports.InvestmentAdvisor, profileRepo ports.ProfileRepository) *RecommendationService {
	return &RecommendationService{advisor: advisor, profileRepo: profileRepo}
}

// GetDailyRecommendation fetches the user's profile then asks the advisor for an allocation.
// A missing profile is not an error — the advisor falls back to generic guidance.
func (s *RecommendationService) GetDailyRecommendation(ctx context.Context, userID string, req models.InvestmentRequest) (*models.Recommendation, error) {
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ports.ErrProfileNotFound) {
		return nil, fmt.Errorf("recommendation service: fetch profile: %w", err)
	}

	rec, err := s.advisor.GetRecommendation(ctx, req, profile)
	if err != nil {
		return nil, fmt.Errorf("recommendation service: advisor: %w", err)
	}
	return rec, nil
}
