package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// InvestmentAdvisor is the port any AI provider must implement.
// profile may be nil when no profile has been saved yet.
type InvestmentAdvisor interface {
	GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile) (*models.Recommendation, error)
}
