// domain/ports/portfolio_connector.go
package ports

import "context"

// PortfolioConnector manages the OAuth lifecycle for connecting external brokerages
// via a portfolio aggregation provider.
// Implementations live in infrastructure/portfolio — never imported by domain or application directly.
type PortfolioConnector interface {
	// RegisterUser registers the user with the aggregation provider.
	// Returns providerUserID and providerUserSecret that must be stored encrypted.
	RegisterUser(ctx context.Context, userID string) (providerUserID, providerUserSecret string, err error)

	// GenerateConnectURL returns a redirect URL the user visits to link their external broker.
	GenerateConnectURL(ctx context.Context, providerUserID, providerUserSecret string) (redirectURL string, err error)

	// DeleteUser de-registers the user from the aggregation provider, invalidating their credentials.
	DeleteUser(ctx context.Context, providerUserID, providerUserSecret string) error
}
