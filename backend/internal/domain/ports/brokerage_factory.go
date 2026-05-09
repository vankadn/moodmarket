// domain/ports/brokerage_factory.go
package ports

import (
	"errors"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// ErrBrokerageNotConnected is returned when a user has no connected brokerage account.
var ErrBrokerageNotConnected = errors.New("no brokerage account connected")

// BrokerageProviderFactory creates a per-user BrokerageProvider from stored credentials.
// Implementations live in infrastructure/brokerage — never imported by application directly.
// Providers are not cached; construct fresh per request.
type BrokerageProviderFactory interface {
	ForUser(conn *models.BrokerageConnection) (BrokerageProvider, error)
}
