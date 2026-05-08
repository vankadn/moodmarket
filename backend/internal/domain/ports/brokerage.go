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
}
