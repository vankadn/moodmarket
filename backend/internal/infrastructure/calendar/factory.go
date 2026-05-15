// infrastructure/calendar/factory.go
package calendar

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewMarketCalendar constructs a MarketCalendar from the MARKET_CALENDAR env var.
// Valid values: "nyse" (default) | "mock"
func NewMarketCalendar() (ports.MarketCalendar, error) {
	provider := os.Getenv("MARKET_CALENDAR")
	if provider == "" {
		provider = "nyse"
	}
	switch provider {
	case "nyse":
		return &NYSECalendar{}, nil
	case "mock":
		return &MockCalendar{}, nil
	default:
		return nil, fmt.Errorf("unknown MARKET_CALENDAR=%q; valid: nyse, mock", provider)
	}
}
