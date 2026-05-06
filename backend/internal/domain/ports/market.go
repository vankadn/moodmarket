// domain/ports/market.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// MarketDataProvider is the port any market data source must implement.
// The Alpha Vantage implementation lives in infrastructure/market — this
// interface is the only thing the application layer ever touches.
type MarketDataProvider interface {
	GetDailySnapshot(ctx context.Context) (*models.MarketSnapshot, error)
}
