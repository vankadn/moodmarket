// infrastructure/portfolio/factory.go
package portfolio

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewPortfolioProvider reads SNAPTRADE_PROVIDER and constructs the matching provider.
// Returns both interfaces since SnapTradeClient and MockPortfolioProvider implement both.
// mock → no API calls (DEV_MODE only). snaptrade → live SnapTrade API.
func NewPortfolioProvider() (ports.PortfolioAggregator, ports.PortfolioConnector, error) {
	provider := os.Getenv("SNAPTRADE_PROVIDER")
	if provider == "" {
		return nil, nil, fmt.Errorf("portfolio factory: SNAPTRADE_PROVIDER is required; set to 'snaptrade' or use MOCK_ALL=true for local dev")
	}
	switch provider {
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, nil, fmt.Errorf("portfolio factory: SNAPTRADE_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		m := NewMockPortfolioProvider()
		return m, m, nil
	case "snaptrade":
		clientID := os.Getenv("SNAPTRADE_CLIENT_ID")
		consumerKey := os.Getenv("SNAPTRADE_CONSUMER_KEY")
		if clientID == "" || consumerKey == "" {
			return nil, nil, fmt.Errorf("portfolio factory: SNAPTRADE_CLIENT_ID and SNAPTRADE_CONSUMER_KEY are required when SNAPTRADE_PROVIDER=snaptrade")
		}
		c := NewSnapTradeClient(clientID, consumerKey)
		return c, c, nil
	default:
		return nil, nil, fmt.Errorf("portfolio factory: unknown provider %q (set SNAPTRADE_PROVIDER=snaptrade or use MOCK_ALL=true for local dev)", provider)
	}
}
