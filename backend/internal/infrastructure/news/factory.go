// infrastructure/news/factory.go
package news

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewNewsProvider reads NEWS_PROVIDER and returns the matching implementation.
// Defaults to mock so no extra env var is needed for local dev with MOCK_ALL=false.
func NewNewsProvider() (ports.NewsProvider, error) {
	provider := os.Getenv("NEWS_PROVIDER")
	if provider == "" {
		provider = "mock"
	}
	switch provider {
	case "mock":
		return newMockNewsProvider(), nil
	case "polygon":
		apiKey := os.Getenv("POLYGON_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("news factory: POLYGON_API_KEY is required for provider %q — add it to .env", provider)
		}
		return newPolygonNewsProvider(apiKey), nil
	default:
		return nil, fmt.Errorf("news factory: unknown provider %q (set NEWS_PROVIDER=mock or polygon)", provider)
	}
}
