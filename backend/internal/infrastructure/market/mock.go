// infrastructure/market/mock.go
package market

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type mockProvider struct{}

func newMockProvider() *mockProvider { return &mockProvider{} }

// GetDailySnapshot returns a realistic hardcoded snapshot.
// Used in local development so the full recommendation flow works without any API calls.
func (m *mockProvider) GetDailySnapshot(_ context.Context) (*models.MarketSnapshot, error) {
	return &models.MarketSnapshot{
		Date:             time.Now().Format("2006-01-02"),
		SPYPrice:         518.00,
		SPYChangePercent: 0.82,
		QQQChangePercent: -0.34,
		SectorPerformance: map[string]float64{
			"Energy":      2.10,
			"Technology":  -0.34,
			"Financials":  0.65,
			"Healthcare":  -1.20,
			"Industrials": 0.45,
		},
		MarketSentiment: "bullish",
		TopMovers: []models.TickerSnapshot{
			{Symbol: "Energy", ChangePercent: 2.10, Price: 0},
			{Symbol: "Healthcare", ChangePercent: -1.20, Price: 0},
			{Symbol: "Financials", ChangePercent: 0.65, Price: 0},
			{Symbol: "Industrials", ChangePercent: 0.45, Price: 0},
			{Symbol: "Technology", ChangePercent: -0.34, Price: 0},
		},
	}, nil
}

// GetPrice returns a deterministic previous-day close price for the given ticker.
func (m *mockProvider) GetPrice(_ context.Context, ticker string) (float64, error) {
	prices := map[string]float64{
		"SPY":  518.00,
		"QQQ":  443.00,
		"VTI":  263.00,
		"BND":  72.50,
		"VXUS": 61.50,
		"XLE":  88.00,
		"XLF":  44.50,
		"XLV":  142.00,
		"XLI":  131.00,
	}
	if p, ok := prices[ticker]; ok {
		return p, nil
	}
	return 148.00, nil
}

// ensure mockProvider satisfies MarketDataProvider at compile time
var _ interface {
	GetDailySnapshot(context.Context) (*models.MarketSnapshot, error)
	GetPrice(context.Context, string) (float64, error)
} = (*mockProvider)(nil)
