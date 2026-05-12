package advisor

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewAdvisor reads AI_PROVIDER and returns the matching implementation.
// newsProvider is passed to the Claude advisor so it can call get_market_news as a tool.
// Adding a new provider requires one new file and one new case here — nothing else changes.
func NewAdvisor(newsProvider ports.NewsProvider) (ports.InvestmentAdvisor, error) {
	provider := os.Getenv("AI_PROVIDER")
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude":
		return newClaudeAdvisor(newsProvider), nil
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("advisor factory: AI_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return newMockAdvisor(), nil
	default:
		return nil, fmt.Errorf("advisor factory: unknown provider %q (set AI_PROVIDER=claude or mock)", provider)
	}
}
