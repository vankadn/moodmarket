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
		return nil, fmt.Errorf("market factory: MARKET_PROVIDER is required; set to 'polygon', or use MOCK_ALL=true for local dev")
	}
	switch provider {
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("market factory: MARKET_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return newMockProvider(), nil
	case "polygon":
		apiKey := os.Getenv("POLYGON_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("market factory: POLYGON_API_KEY is required for provider %q — add it to .env", provider)
		}
		return newPolygonProvider(apiKey), nil
	default:
		return nil, fmt.Errorf("market factory: unknown provider %q (set MARKET_PROVIDER=polygon or use MOCK_ALL=true for local dev)", provider)
	}
}
