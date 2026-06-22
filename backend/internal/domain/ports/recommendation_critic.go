package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// RecommendationCritic reviews a specific Recommendation adversarially and decides
// whether to approve or block it. It is intentionally separate from InvestmentAdvisor —
// different system prompt, different inputs, and an explicit adversarial stance.
// profile may be nil when no profile has been saved yet.
type RecommendationCritic interface {
	ReviewRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, rec *models.Recommendation) (*models.CriticReview, error)
}
