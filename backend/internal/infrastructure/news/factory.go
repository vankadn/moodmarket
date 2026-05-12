// infrastructure/news/factory.go
package news

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewNewsProvider reads NEWS_PROVIDER and returns the matching implementation.
func NewNewsProvider() (ports.NewsProvider, error) {
	provider := os.Getenv("NEWS_PROVIDER")
	if provider == "" {
		return nil, fmt.Errorf("news factory: NEWS_PROVIDER is required; set to 'polygon', or use MOCK_ALL=true for local dev")
	}
	switch provider {
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("news factory: NEWS_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return newMockNewsProvider(), nil
	case "polygon":
		apiKey := os.Getenv("POLYGON_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("news factory: POLYGON_API_KEY is required for provider %q — add it to .env", provider)
		}
		return newPolygonNewsProvider(apiKey), nil
	default:
		return nil, fmt.Errorf("news factory: unknown provider %q (set NEWS_PROVIDER=polygon or use MOCK_ALL=true for local dev)", provider)
	}
}
