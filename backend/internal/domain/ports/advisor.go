package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// InvestmentAdvisor is the port any AI provider must implement.
// profile may be nil when no profile has been saved yet.
// snapshot may be nil when market data is unavailable — advisors must degrade gracefully.
type InvestmentAdvisor interface {
	GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot) (*models.Recommendation, error)
}
