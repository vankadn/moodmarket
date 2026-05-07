// infrastructure/brokerage/factory.go
package brokerage

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewBrokerageProvider reads BROKERAGE_PROVIDER and returns the matching implementation.
// Defaults to mock so the invest flow works out of the box without any credentials.
func NewBrokerageProvider() (ports.BrokerageProvider, error) {
	provider := os.Getenv("BROKERAGE_PROVIDER")
	if provider == "" {
		provider = "mock"
	}
	switch provider {
	case "mock":
		return NewMockBrokerageProvider(), nil
	case "alpaca":
		apiKey := os.Getenv("ALPACA_API_KEY")
		apiSecret := os.Getenv("ALPACA_API_SECRET")
		if apiKey == "" || apiSecret == "" {
			return nil, fmt.Errorf("brokerage factory: ALPACA_API_KEY and ALPACA_API_SECRET are required for provider %q — add them to .env", provider)
		}
		baseURL := os.Getenv("ALPACA_BASE_URL")
		if baseURL == "" {
			baseURL = "https://paper-api.alpaca.markets"
		}
		return NewAlpacaProvider(apiKey, apiSecret, baseURL), nil
	default:
		return nil, fmt.Errorf("brokerage factory: unknown provider %q (set BROKERAGE_PROVIDER=mock or alpaca)", provider)
	}
}
