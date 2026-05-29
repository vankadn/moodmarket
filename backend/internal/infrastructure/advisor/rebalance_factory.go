package advisor

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewRebalanceAdvisor reads AI_PROVIDER and returns the matching RebalanceAdvisor implementation.
// Adding a new provider requires one new file and one new case here — nothing else changes.
func NewRebalanceAdvisor() (ports.RebalanceAdvisor, error) {
	provider := os.Getenv("AI_PROVIDER")
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude":
		return newClaudeRebalanceAdvisor(), nil
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("rebalance advisor factory: AI_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return newMockRebalanceAdvisor(), nil
	default:
		return nil, fmt.Errorf("rebalance advisor factory: unknown provider %q (set AI_PROVIDER=claude or mock)", provider)
	}
}
