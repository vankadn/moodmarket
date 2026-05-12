// infrastructure/banking/factory.go
package banking

import (
	"fmt"
	"log"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewFinancialDataProvider reads FINANCIAL_DATA_PROVIDER and returns the matching implementation.
func NewFinancialDataProvider() (ports.FinancialDataProvider, error) {
	provider := os.Getenv("FINANCIAL_DATA_PROVIDER")
	if provider == "" {
		return nil, fmt.Errorf("banking factory: FINANCIAL_DATA_PROVIDER is required; set to 'plaid', or use MOCK_ALL=true for local dev")
	}

	switch provider {
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("banking factory: FINANCIAL_DATA_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		log.Println("[banking] using mock financial data provider")
		return NewMockProvider(), nil
	case "plaid":
		clientID := os.Getenv("PLAID_CLIENT_ID")
		secret := os.Getenv("PLAID_SECRET")
		if clientID == "" || secret == "" {
			return nil, fmt.Errorf("banking factory: PLAID_CLIENT_ID and PLAID_SECRET are required when FINANCIAL_DATA_PROVIDER=plaid")
		}
		env := os.Getenv("PLAID_ENV")
		if env == "" {
			env = "sandbox"
		}
		cacheTTL := os.Getenv("PLAID_CACHE_TTL")
		return NewPlaidProvider(clientID, secret, env, cacheTTL), nil
	default:
		return nil, fmt.Errorf("banking factory: unknown provider %q (set FINANCIAL_DATA_PROVIDER=plaid or use MOCK_ALL=true for local dev)", provider)
	}
}
