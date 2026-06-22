package critic

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewRecommendationCritic reads AI_PROVIDER and returns the matching RecommendationCritic.
// Adding a new provider requires one new file and one new case here — nothing else changes.
func NewRecommendationCritic() (ports.RecommendationCritic, error) {
	provider := os.Getenv("AI_PROVIDER")
	if provider == "" {
		provider = "claude"
	}
	switch provider {
	case "claude":
		return newClaudeRecommendationCritic(), nil
	case "mock":
		if os.Getenv("DEV_MODE") != "true" {
			return nil, fmt.Errorf("critic factory: AI_PROVIDER=mock is not allowed in production (DEV_MODE != true)")
		}
		return newMockRecommendationCritic(), nil
	default:
		return nil, fmt.Errorf("critic factory: unknown provider %q (set AI_PROVIDER=claude or mock)", provider)
	}
}
