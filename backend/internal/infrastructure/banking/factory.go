// infrastructure/banking/factory.go
package banking

import (
	"fmt"
	"log"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewFinancialDataProvider reads FINANCIAL_DATA_PROVIDER and returns the matching implementation.
// Defaults to mock so the flow works out of the box without Plaid credentials.
func NewFinancialDataProvider() (ports.FinancialDataProvider, error) {
	provider := os.Getenv("FINANCIAL_DATA_PROVIDER")
	if provider == "" {
		log.Println("[banking] FINANCIAL_DATA_PROVIDER not set — defaulting to mock")
		return NewMockProvider(), nil
	}

	switch provider {
	case "mock":
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
		return NewPlaidProvider(clientID, secret, env), nil
	default:
		return nil, fmt.Errorf("banking factory: unknown provider %q (set FINANCIAL_DATA_PROVIDER=mock or plaid)", provider)
	}
}
