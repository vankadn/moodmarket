// infrastructure/portfolio/mock.go
package portfolio

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// MockPortfolioProvider implements both PortfolioAggregator and PortfolioConnector with no API calls.
// Used when SNAPTRADE_PROVIDER=mock so the full portfolio flow works without SnapTrade credentials.
type MockPortfolioProvider struct{}

func NewMockPortfolioProvider() *MockPortfolioProvider { return &MockPortfolioProvider{} }

func (m *MockPortfolioProvider) RegisterUser(_ context.Context, userID string) (string, string, error) {
	return "mock-provider-user-" + userID, "mock-provider-secret", nil
}

func (m *MockPortfolioProvider) GenerateConnectURL(_ context.Context, _, _ string) (string, error) {
	return "https://app.snaptrade.com/snapTrade/connect?token=mock-token", nil
}

func (m *MockPortfolioProvider) DeleteUser(_ context.Context, _, _ string) error {
	return nil
}

func (m *MockPortfolioProvider) ListAccounts(_ context.Context, _, _ string) ([]models.LinkedAccount, error) {
	return []models.LinkedAccount{
		{ID: "mock-account-1", Name: "Robinhood Individual"},
	}, nil
}

func (m *MockPortfolioProvider) GetHoldings(_ context.Context, _, _ string) ([]models.Position, error) {
	return []models.Position{
		{Ticker: "AAPL", Name: "Apple Inc", Quantity: 5.0, MarketValue: 875.00, CostBasis: 750.00, AvgEntryPrice: 150.00, UnrealizedPL: 125.00, UnrealizedPLPercent: 16.67},
		{Ticker: "MSFT", Name: "Microsoft Corporation", Quantity: 2.0, MarketValue: 830.00, CostBasis: 700.00, AvgEntryPrice: 350.00, UnrealizedPL: 130.00, UnrealizedPLPercent: 18.57},
		{Ticker: "SPY", Name: "SPDR S&P 500 ETF", Quantity: 1.5, MarketValue: 783.00, CostBasis: 720.00, AvgEntryPrice: 480.00, UnrealizedPL: 63.00, UnrealizedPLPercent: 8.75},
	}, nil
}
