// infrastructure/fundamentals/mock_fundamentals.go
package fundamentals

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockFundamentalsProvider struct{}

func newMockFundamentalsProvider() *mockFundamentalsProvider { return &mockFundamentalsProvider{} }

func (m *mockFundamentalsProvider) GetFundamentals(_ context.Context, ticker string) (*models.Fundamentals, error) {
	return &models.Fundamentals{
		Ticker:               ticker,
		PE:                   18.5,
		ForwardPE:            16.2,
		ForwardPEG:           1.1,
		FiftyTwoWeekHigh:     120.50,
		FiftyTwoWeekHighDate: "2025-11-14",
		FiftyTwoWeekLow:      72.10,
		FiftyTwoWeekLowDate:  "2026-04-07",
		DebtToEquity:         0.43,
		TotalDebtToEquity:    0.58,
		CurrentRatio:         2.1,
		EVToEBITDA:           12.4,  // moderate valuation
		FCFYieldPct:          4.8,   // ~21x price-to-FCF inverted
		PriceToBook:          3.2,
		PEVsOwnFiveYearAvg:   0.87, // trading at a 13% discount to its own 5yr PE average
		EVEBITDAVsOwnAvg:     1.05, // roughly in line with own history
	}, nil
}

func (m *mockFundamentalsProvider) GetEarningsSurprises(_ context.Context, _ string, limit int) ([]models.EarningsSurprise, error) {
	results := []models.EarningsSurprise{
		{Period: "2025-09-30", ActualEPS: 1.92, EstimateEPS: 1.85, SurprisePct: 3.78},
		{Period: "2025-06-30", ActualEPS: 1.74, EstimateEPS: 1.80, SurprisePct: -3.33},
		{Period: "2025-03-31", ActualEPS: 2.10, EstimateEPS: 2.05, SurprisePct: 2.44},
		{Period: "2024-12-31", ActualEPS: 1.88, EstimateEPS: 1.90, SurprisePct: -1.05},
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// GetInsiderActivity returns a mock scenario representing "insiders selling into strength":
// 8 consecutive months with no genuine code-P purchase (ConsecutiveNegativeMonths == 8).
// October 2025 has positive MSPR (grant month) — this correctly does NOT break the streak
// under the fixed logic, since grants are code-A, not code-P.
// LastGenuinePurchaseDate is set to a date in 2025-09 as the last real open-market buy.
func (m *mockFundamentalsProvider) GetInsiderActivity(_ context.Context, ticker string) (*models.InsiderActivity, error) {
	months := []models.InsiderMonth{
		{Year: 2026, Month: 6, Change: -42000, MSPR: -18.4},
		{Year: 2026, Month: 5, Change: -38500, MSPR: -15.2},
		{Year: 2026, Month: 4, Change: -51000, MSPR: -21.7},
		{Year: 2026, Month: 3, Change: -29000, MSPR: -11.3},
		{Year: 2026, Month: 2, Change: -44000, MSPR: -16.9},
		{Year: 2026, Month: 1, Change: -33000, MSPR: -13.1},
		{Year: 2025, Month: 12, Change: -27000, MSPR: -9.8},
		{Year: 2025, Month: 11, Change: -18000, MSPR: -7.2},
		{Year: 2025, Month: 10, Change: 5000, MSPR: 2.1}, // grant month — positive MSPR but no code-P purchase
		{Year: 2025, Month: 9, Change: -12000, MSPR: -4.5},
	}
	return &models.InsiderActivity{
		Ticker:                    ticker,
		RecentMonths:              months,
		ConsecutiveNegativeMonths: 8,
		LastGenuinePurchaseDate:   "2025-09-12",
	}, nil
}
