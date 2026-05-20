// domain/ports/portfolio_aggregator.go
package ports

import (
	"context"
	"errors"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// ErrPortfolioNotConnected is returned when the user has no connected portfolio aggregator.
var ErrPortfolioNotConnected = errors.New("no portfolio aggregator connected")

// PortfolioAggregator is the port any read-only portfolio aggregation service must implement.
// Implementations live in infrastructure/portfolio — never imported by domain or application directly.
type PortfolioAggregator interface {
	// GetHoldings returns all external holdings for the user identified by provider credentials.
	GetHoldings(ctx context.Context, providerUserID, providerUserSecret string) ([]models.Position, error)
}
