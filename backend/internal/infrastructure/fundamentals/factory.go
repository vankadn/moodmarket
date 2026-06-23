// infrastructure/fundamentals/factory.go
package fundamentals

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewFundamentalsProvider reads FUNDAMENTALS_PROVIDER and returns the matching implementation.
// Adding a new provider requires one new file and one new case here — nothing else changes.
//
// Providers:
//
//	finnhub — company metrics, earnings surprises, insider sentiment via Finnhub API (reuses FINNHUB_API_KEY).
//	mock    — hardcoded plausible values; no external calls (requires DEV_MODE=true).
func NewFundamentalsProvider() (ports.FundamentalsProvider, error) {
	provider := os.Getenv("FUNDAMENTALS_PROVIDER")
	if provider == "" {
		return nil, fmt.Errorf("fundamentals factory: FUNDAMENTALS_PROVIDER is required; set to 'finnhub', or use MOCK_ALL=true for local dev")
	}
	switch provider {
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("fundamentals factory: FUNDAMENTALS_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return newMockFundamentalsProvider(), nil
	case "finnhub":
		apiKey := os.Getenv("FINNHUB_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("fundamentals factory: FINNHUB_API_KEY is required for provider %q — add it to .env", provider)
		}
		return newFinnhubFundamentalsProvider(apiKey), nil
	default:
		return nil, fmt.Errorf("fundamentals factory: unknown provider %q (set FUNDAMENTALS_PROVIDER=finnhub or use MOCK_ALL=true for local dev)", provider)
	}
}
