// infrastructure/brokerage/factory.go
package brokerage

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewBrokerageFactory reads BROKERAGE_PROVIDER and returns the matching factory.
// mock → always returns the mock provider regardless of user credentials (dev/test only).
// alpaca → constructs a per-user AlpacaProvider from stored credentials on each call.
func NewBrokerageFactory() (ports.BrokerageProviderFactory, error) {
	provider := os.Getenv("BROKERAGE_PROVIDER")
	if provider == "" {
		return nil, fmt.Errorf("brokerage factory: BROKERAGE_PROVIDER is required; set to 'alpaca', or use MOCK_ALL=true for local dev")
	}
	switch provider {
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("brokerage factory: BROKERAGE_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return &mockProviderFactory{mock: NewMockBrokerageProvider()}, nil
	case "alpaca":
		return &alpacaProviderFactory{}, nil
	default:
		return nil, fmt.Errorf("brokerage factory: unknown provider %q (set BROKERAGE_PROVIDER=alpaca or use MOCK_ALL=true for local dev)", provider)
	}
}

// mockProviderFactory always returns the single mock instance — user credentials are ignored.
type mockProviderFactory struct {
	mock ports.BrokerageProvider
}

func (f *mockProviderFactory) ForUser(_ *models.BrokerageConnection) (ports.BrokerageProvider, error) {
	return f.mock, nil
}

// alpacaProviderFactory constructs a fresh AlpacaProvider from the user's stored credentials.
type alpacaProviderFactory struct{}

func (f *alpacaProviderFactory) ForUser(conn *models.BrokerageConnection) (ports.BrokerageProvider, error) {
	if conn == nil || !conn.Connected {
		return nil, ports.ErrBrokerageNotConnected
	}
	return NewAlpacaProvider(conn.APIKey, conn.SecretKey, conn.BaseURL), nil
}
