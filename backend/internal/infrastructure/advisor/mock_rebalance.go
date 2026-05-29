package advisor

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockRebalanceAdvisor struct{}

func newMockRebalanceAdvisor() *mockRebalanceAdvisor { return &mockRebalanceAdvisor{} }

// AnalyzePortfolio returns a deterministic mock analysis that cycles through all four actions.
func (m *mockRebalanceAdvisor) AnalyzePortfolio(_ context.Context, req models.RebalanceRequest, _ *models.UserProfile) (*models.RebalanceAnalysis, error) {
	actions := []models.SuggestedAction{
		models.ActionHold,
		models.ActionAdd,
		models.ActionTrim,
		models.ActionReconsider,
	}

	insights := make([]models.PositionInsight, 0, len(req.Positions))
	for i, p := range req.Positions {
		insights = append(insights, models.PositionInsight{
			Ticker:            p.Ticker,
			Name:              p.Name,
			Source:            p.Source,
			AccountName:       p.AccountName,
			CurrentValue:      p.MarketValue,
			UnrealizedPL:      p.UnrealizedPL,
			UnrealizedPLPct:   p.UnrealizedPLPercent,
			OriginalBuyThesis: req.BuyReasoningByTicker[p.Ticker],
			ClaudeAssessment:  "Mock assessment: position looks reasonable given current conditions.",
			SuggestedAction:   actions[i%len(actions)],
			TaxFlag:           computeTaxFlag(p.Ticker, req.FirstPurchaseByTicker),
		})
	}

	return &models.RebalanceAnalysis{
		Insights:               insights,
		PortfolioHealthSummary: "Mock analysis: portfolio is balanced with minor adjustments recommended.",
		GeneratedAt:            time.Now(),
	}, nil
}
