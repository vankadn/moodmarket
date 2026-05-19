package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// Classifier is the read/write cache interface.
// Implementations must never hit Mongo during Classify — memory only.
type Classifier interface {
	Classify(ticker string) (assetClass string, known bool)
	// Store writes a new classification into the in-memory map immediately.
	// Called after StoreClassification persists to Mongo.
	Store(ticker, assetClass string)
}

// ClassificationRepository persists ticker→asset-class mappings.
type ClassificationRepository interface {
	// LoadAll returns every ticker in the collection for cache hydration at startup.
	// The approved field is preserved for future use but not used as a filter.
	LoadAll(ctx context.Context) ([]models.ClassificationEntry, error)
	// StoreClassification upserts a Claude-classified ticker.
	// Uses $setOnInsert for first_seen_at so re-classification never resets the timestamp.
	StoreClassification(ctx context.Context, ticker, assetClass string) error
}
