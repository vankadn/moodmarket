package advisor

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockAdvisor struct{}

func newMockAdvisor() *mockAdvisor { return &mockAdvisor{} }

func (m *mockAdvisor) GetRecommendation(_ context.Context, req models.InvestmentRequest, profile *models.UserProfile, _ *models.MarketSnapshot) (*models.Recommendation, error) {
	risk := "moderate"
	if profile != nil {
		risk = string(profile.RiskTolerance)
	}

	total := req.BaseBudget + req.ExtraMoney
	allocations := []models.Allocation{
		{Ticker: "VTI", Name: "Vanguard Total Stock Market ETF", Type: "etf", Amount: total * 0.6, Percentage: 60, Rationale: "Broad US market exposure"},
		{Ticker: "VXUS", Name: "Vanguard Total International ETF", Type: "etf", Amount: total * 0.3, Percentage: 30, Rationale: "International diversification"},
		{Ticker: "BND", Name: "Vanguard Total Bond Market ETF", Type: "etf", Amount: total * 0.1, Percentage: 10, Rationale: "Stability and income"},
	}

	return &models.Recommendation{
		TotalBudget: total,
		Allocations: allocations,
		Summary:     "Mock recommendation: diversified three-fund portfolio suitable for long-term growth.",
		RiskLevel:   risk,
	}, nil
}
