// infrastructure/market/factory.go
package market

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewMarketDataProvider reads MARKET_PROVIDER and returns the matching implementation.
// Adding a new provider requires one new file and one new case here — nothing else changes.
func NewMarketDataProvider() (ports.MarketDataProvider, error) {
	provider := os.Getenv("MARKET_PROVIDER")
	if provider == "" {
		provider = "mock"
	}
	switch provider {
	case "mock":
		return newMockProvider(), nil
	case "polygon":
		apiKey := os.Getenv("POLYGON_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("market factory: POLYGON_API_KEY is required for provider %q — add it to .env", provider)
		}
		return newPolygonProvider(apiKey), nil
	default:
		return nil, fmt.Errorf("market factory: unknown provider %q (set MARKET_PROVIDER=mock or polygon)", provider)
	}
}
