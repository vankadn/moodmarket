package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// Classifier is the read-only cache interface. Infrastructure implementations
// must never hit Mongo during a Classify call — memory only.
type Classifier interface {
	Classify(ticker string) (assetClass string, known bool)
}

// ClassificationRepository persists ticker→asset-class mappings.
type ClassificationRepository interface {
	// Seed writes entries using $setOnInsert so existing records are never overwritten.
	Seed(ctx context.Context, entries []models.ClassificationEntry) error
	// LoadApproved returns all approved:true entries for cache hydration.
	LoadApproved(ctx context.Context) ([]models.ClassificationEntry, error)
	// QueueUnknown records an unapproved Claude-suggested classification.
	// Upserts on ticker — never duplicates. Never overwrites an existing entry.
	QueueUnknown(ctx context.Context, ticker, suggestedClass string) error
}
