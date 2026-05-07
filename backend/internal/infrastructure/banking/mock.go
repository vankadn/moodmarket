// infrastructure/banking/mock.go
package banking

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// mockProvider returns deterministic data so the invest flow works without Plaid credentials.
type mockProvider struct{}

func NewMockProvider() *mockProvider {
	return &mockProvider{}
}

func (m *mockProvider) CreateLinkToken(_ context.Context, _ string) (string, error) {
	return "mock-link-token-12345", nil
}

func (m *mockProvider) ExchangePublicToken(_ context.Context, _ string) (string, string, string, error) {
	return "mock-access-token", "mock-item-id", "Sandbox Bank", nil
}

func (m *mockProvider) GetAccounts(_ context.Context, _ string) ([]models.BankAccount, error) {
	return []models.BankAccount{
		{AccountID: "mock-checking-1", Institution: "Sandbox Bank", Name: "Checking", Type: "depository", Subtype: "checking", Balance: 4200.00, Currency: "USD"},
		{AccountID: "mock-invest-1", Institution: "Sandbox Bank", Name: "Brokerage", Type: "investment", Subtype: "brokerage", Balance: 12000.00, Currency: "USD"},
	}, nil
}

func (m *mockProvider) GetBalanceSummary(_ context.Context, _ []models.PlaidConnection) (models.BalanceSummary, error) {
	return models.BalanceSummary{
		TotalCash:        4200.00,
		TotalInvestments: 12000.00,
		Institutions:     []string{"Sandbox Bank"},
		AccountCount:     2,
		PulledAt:         time.Now(),
	}, nil
}

func (m *mockProvider) RevokeToken(_ context.Context, _ string) error {
	return nil
}
