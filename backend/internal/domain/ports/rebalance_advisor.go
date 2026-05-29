package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// RebalanceAdvisor is the port any AI-based portfolio rebalance analysis provider must implement.
// It is intentionally separate from InvestmentAdvisor — different inputs, different outputs,
// and a different system prompt focused on hold/trim/reconsider suggestions, not buy decisions.
type RebalanceAdvisor interface {
	AnalyzePortfolio(ctx context.Context, req models.RebalanceRequest, profile *models.UserProfile) (*models.RebalanceAnalysis, error)
}
