// domain/ports/brokerage.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// Position is a holding the user currently owns in their brokerage account.
type Position struct {
	Ticker      string
	Quantity    float64
	MarketValue float64
}

// BrokerageProvider is the port any trade execution system must implement.
// Implementations live in infrastructure/brokerage — never imported by domain or application directly.
type BrokerageProvider interface {
	// PlaceMarketOrder places a notional (dollar-based) market buy order for the given ticker.
	PlaceMarketOrder(ctx context.Context, order models.TradeOrder) (*models.TradeReceipt, error)
	// GetPositions returns all open positions for the given user.
	GetPositions(ctx context.Context, userID string) ([]Position, error)
}
