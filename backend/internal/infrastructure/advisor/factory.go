package advisor

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewAdvisor reads AI_PROVIDER and returns the matching implementation.
// Adding a new provider requires one new file and one new case here — nothing else changes.
func NewAdvisor() (ports.InvestmentAdvisor, error) {
	provider := os.Getenv("AI_PROVIDER")
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude":
		return newClaudeAdvisor(), nil
	default:
		return nil, fmt.Errorf("advisor factory: unknown provider %q (set AI_PROVIDER=claude)", provider)
	}
}
