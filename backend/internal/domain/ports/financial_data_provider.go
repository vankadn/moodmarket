// domain/ports/financial_data_provider.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// FinancialDataProvider is the port any bank-account aggregator must implement.
// Plaid is the current implementation; MX, Finicity, or any other aggregator is a drop-in replacement.
type FinancialDataProvider interface {
	// CreateLinkToken generates a short-lived Plaid Link initialization token for the given user.
	CreateLinkToken(ctx context.Context, userID string) (string, error)

	// ExchangePublicToken exchanges the short-lived public_token returned by Plaid Link for
	// a permanent access_token and returns the item_id and resolved institution name.
	ExchangePublicToken(ctx context.Context, publicToken string) (accessToken string, itemID string, institutionName string, err error)

	// GetAccounts returns all accounts associated with the given access token.
	GetAccounts(ctx context.Context, accessToken string) ([]models.BankAccount, error)

	// GetBalanceSummary aggregates live balances across all connected institutions.
	// Failures on individual connections are logged and skipped — partial results are returned.
	GetBalanceSummary(ctx context.Context, connections []models.PlaidConnection) (models.BalanceSummary, error)

	// RevokeToken permanently revokes an access token (calls item/remove on Plaid).
	RevokeToken(ctx context.Context, accessToken string) error
}
