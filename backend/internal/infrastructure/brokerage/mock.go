// infrastructure/brokerage/mock.go
package brokerage

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// MockBrokerageProvider returns realistic hardcoded receipts with no API calls.
// Used when BROKERAGE_PROVIDER=mock so the full invest flow works without Alpaca credentials.
type MockBrokerageProvider struct{}

func NewMockBrokerageProvider() *MockBrokerageProvider { return &MockBrokerageProvider{} }

func (m *MockBrokerageProvider) PlaceMarketOrder(_ context.Context, order models.TradeOrder) (*models.TradeReceipt, error) {
	return &models.TradeReceipt{
		OrderID:      fmt.Sprintf("mock-order-%s-%d", order.Ticker, time.Now().UnixMilli()),
		Ticker:       order.Ticker,
		FilledAmount: order.Amount,
		FilledPrice:  150.00,
		Status:       "filled",
		Timestamp:    time.Now(),
	}, nil
}

func (m *MockBrokerageProvider) GetOrder(_ context.Context, orderID string) (*models.TradeReceipt, error) {
	return &models.TradeReceipt{
		OrderID:      orderID,
		Ticker:       "MOCK",
		FilledAmount: 100.00,
		FilledPrice:  150.00,
		Status:       "filled",
		Timestamp:    time.Now(),
	}, nil
}

func (m *MockBrokerageProvider) GetPortfolioHistory(_ context.Context, _ string, period, _ string) ([]models.HistoryPoint, error) {
	// Generate 30 realistic-looking points with a gentle upward trend + noise
	n := 30
	base := 3000.0
	now := time.Now().Unix()
	var stepSecs int64
	switch period {
	case "1D":
		stepSecs = 5 * 60 // 5 minutes
	case "5D":
		stepSecs = 3600 // 1 hour
	default:
		stepSecs = 86400 // 1 day
	}
	start := now - int64(n)*stepSecs

	points := make([]models.HistoryPoint, n)
	for i := 0; i < n; i++ {
		// Gentle sine wave uptrend for a realistic shape
		equity := base + float64(i)*2.5 + 40*math.Sin(float64(i)*0.4)
		pl := equity - base
		points[i] = models.HistoryPoint{
			Timestamp:     start + int64(i)*stepSecs,
			Equity:        equity,
			ProfitLoss:    pl,
			ProfitLossPct: (pl / base) * 100,
		}
	}
	return points, nil
}

func (m *MockBrokerageProvider) GetCurrentPrice(_ context.Context, ticker string) (float64, error) {
	prices := map[string]float64{
		"SPY":  522.00,
		"QQQ":  447.00,
		"VTI":  267.00,
		"BND":  73.50,
		"VXUS": 62.50,
		"XLE":  89.00,
		"XLF":  45.00,
		"XLV":  143.00,
		"XLI":  132.00,
	}
	if p, ok := prices[ticker]; ok {
		return p, nil
	}
	return 152.00, nil
}

func (m *MockBrokerageProvider) GetPositions(_ context.Context, _ string) ([]models.Position, error) {
	return []models.Position{
		{Ticker: "VTI", Name: "Vanguard Total Market ETF", Quantity: 0.42, MarketValue: 98.50, CostBasis: 87.00, AvgEntryPrice: 207.14, UnrealizedPL: 11.50, UnrealizedPLPercent: 13.22},
		{Ticker: "QQQ", Name: "Invesco Nasdaq 100 ETF", Quantity: 0.25, MarketValue: 112.00, CostBasis: 103.50, AvgEntryPrice: 414.00, UnrealizedPL: 8.50, UnrealizedPLPercent: 8.21},
		{Ticker: "BND", Name: "Vanguard Total Bond Market ETF", Quantity: 0.50, MarketValue: 37.25, CostBasis: 38.00, AvgEntryPrice: 76.00, UnrealizedPL: -0.75, UnrealizedPLPercent: -1.97},
	}, nil
}
