package ports

import (
	"context"
	"errors"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// ErrAdvisorOverloaded is returned when the AI provider is temporarily capacity-constrained (HTTP 529).
// Callers may fall back to cached data instead of surfacing a hard error.
var ErrAdvisorOverloaded = errors.New("advisor overloaded")

// InvestmentAdvisor is the port any AI provider must implement.
// profile may be nil when no profile has been saved yet.
// snapshot may be nil when market data is unavailable — advisors must degrade gracefully.
type InvestmentAdvisor interface {
	GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot) (*models.Recommendation, error)
}
