package classification

import (
	"context"
	"fmt"
	"sync"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ClassificationCache is an in-memory ticker→asset-class map loaded at startup.
// It is the only path queried during recommendation — zero Mongo reads per request.
type ClassificationCache struct {
	mu   sync.RWMutex
	data map[string]string
}

func NewClassificationCache() *ClassificationCache {
	return &ClassificationCache{data: make(map[string]string)}
}

// Classify returns the asset class for ticker from the in-memory cache.
// Returns ("Other", false) for any ticker not in the approved set.
func (c *ClassificationCache) Classify(ticker string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ac, ok := c.data[ticker]
	if !ok {
		return "Other", false
	}
	return ac, true
}

// Store writes a single ticker→assetClass entry into the in-memory map immediately.
// Call this after StoreClassification persists to Mongo so the cache stays hot
// without requiring a full reload.
func (c *ClassificationCache) Store(ticker, assetClass string) {
	c.mu.Lock()
	c.data[ticker] = assetClass
	c.mu.Unlock()
}

// RefreshCache reloads all approved entries from Mongo into memory.
// Call on startup only — never during a live recommendation request.
func (c *ClassificationCache) RefreshCache(ctx context.Context, repo ports.ClassificationRepository) error {
	entries, err := repo.LoadAll(ctx)
	if err != nil {
		return fmt.Errorf("classification cache: refresh: %w", err)
	}
	m := make(map[string]string, len(entries))
	for _, e := range entries {
		m[e.Ticker] = e.AssetClass
	}
	c.mu.Lock()
	c.data = m
	c.mu.Unlock()
	return nil
}
