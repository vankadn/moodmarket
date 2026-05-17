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
	// GetPrice returns the previous-day close price for a single ticker.
	// Used by the verdict stamper as the Polygon data point alongside Alpaca's real-time quote.
	GetPrice(ctx context.Context, ticker string) (float64, error)
}
