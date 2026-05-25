// infrastructure/market/factory.go
package market

import (
	"fmt"
	"os"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewMarketDataProvider reads MARKET_PROVIDER and returns the matching implementation.
// Adding a new provider requires one new file and one new case here — nothing else changes.
//
// Providers:
//   - finnhub  — real-time quotes via Finnhub /quote; TTL-cached (default 5m). Recommended for live use.
//   - polygon  — previous-day OHLCV via Polygon /v2/aggs/prev; daily-cached. Good for free-tier users.
//   - mock     — hardcoded snapshot; no external calls (requires DEV_MODE=true).
func NewMarketDataProvider() (ports.MarketDataProvider, error) {
	provider := os.Getenv("MARKET_PROVIDER")
	if provider == "" {
		return nil, fmt.Errorf("market factory: MARKET_PROVIDER is required; set to 'finnhub', 'polygon', or use MOCK_ALL=true for local dev")
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
	case "finnhub":
		apiKey := os.Getenv("FINNHUB_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("market factory: FINNHUB_API_KEY is required for provider %q — add it to .env", provider)
		}
		ttl := 5 * time.Minute
		if ttlStr := os.Getenv("FINNHUB_CACHE_TTL"); ttlStr != "" {
			parsed, err := time.ParseDuration(ttlStr)
			if err != nil {
				return nil, fmt.Errorf("market factory: invalid FINNHUB_CACHE_TTL %q: %w", ttlStr, err)
			}
			ttl = parsed
		}
		return newFinnhubProvider(apiKey, ttl), nil
	default:
		return nil, fmt.Errorf("market factory: unknown provider %q (set MARKET_PROVIDER=finnhub, polygon, or use MOCK_ALL=true for local dev)", provider)
	}
}
