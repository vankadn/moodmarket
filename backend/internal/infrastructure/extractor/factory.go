package extractor

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewDocumentExtractor reads DOCUMENT_EXTRACTOR and returns the matching implementation.
// Defaults to mock when the env var is unset (safe for local dev).
// Adding a new provider requires one new file and one new case here — nothing else changes.
func NewDocumentExtractor() (ports.DocumentExtractor, error) {
	provider := os.Getenv("DOCUMENT_EXTRACTOR")
	if provider == "" {
		provider = "mock"
	}
	switch provider {
	case "claude":
		return newClaudeExtractor(), nil
	case "mock":
		return newMockExtractor()
	default:
		return nil, fmt.Errorf("extractor factory: unknown provider %q (set DOCUMENT_EXTRACTOR=claude or mock)", provider)
	}
}
