package critic

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockRecommendationCritic struct{}

func newMockRecommendationCritic() *mockRecommendationCritic { return &mockRecommendationCritic{} }

func (m *mockRecommendationCritic) ReviewRecommendation(_ context.Context, _ models.InvestmentRequest, _ *models.UserProfile, _ *models.Recommendation) (*models.CriticReview, error) {
	return &models.CriticReview{
		Verdict:   "approve",
		Concerns:  []string{},
		RiskLevel: "low",
		Reasoning: "Mock critic: recommendation approved without review.",
	}, nil
}
