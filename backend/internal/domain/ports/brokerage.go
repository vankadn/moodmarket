// domain/ports/brokerage.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// BrokerageProvider is the port any trade execution system must implement.
// Implementations live in infrastructure/brokerage — never imported by domain or application directly.
type BrokerageProvider interface {
	// PlaceMarketOrder places a notional (dollar-based) market buy order for the given ticker.
	PlaceMarketOrder(ctx context.Context, order models.TradeOrder) (*models.TradeReceipt, error)
	// GetPositions returns all open positions for the given user.
	GetPositions(ctx context.Context, userID string) ([]models.Position, error)
	// GetOrder returns the current status of a previously placed order.
	GetOrder(ctx context.Context, orderID string) (*models.TradeReceipt, error)
	// GetPortfolioHistory returns timestamped equity values for the given period and timeframe.
	// period examples: "1D", "5D", "1M", "1A", "5A". timeframe examples: "5Min", "1H", "1D".
	GetPortfolioHistory(ctx context.Context, userID, period, timeframe string) ([]models.HistoryPoint, error)
	// GetCurrentPrice returns the real-time last trade price for a single ticker.
	// Used by the verdict stamper to compute returns since entry.
	GetCurrentPrice(ctx context.Context, ticker string) (float64, error)
}
