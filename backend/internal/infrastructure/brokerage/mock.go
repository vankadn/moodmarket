// infrastructure/brokerage/mock.go
package brokerage

import (
	"context"
	"fmt"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
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

func (m *MockBrokerageProvider) GetPositions(_ context.Context, _ string) ([]ports.Position, error) {
	return []ports.Position{
		{Ticker: "VTI", Quantity: 0.42, MarketValue: 98.50},
		{Ticker: "BND", Quantity: 0.50, MarketValue: 37.25},
	}, nil
}
