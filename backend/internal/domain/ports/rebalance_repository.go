// domain/ports/rebalance_repository.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// RebalanceRepository persists and retrieves portfolio rebalance analyses.
// One document per user — SaveAnalysis upserts, keeping only the latest.
type RebalanceRepository interface {
	SaveAnalysis(ctx context.Context, analysis *models.RebalanceAnalysis) error
	// GetLatestAnalysis returns the most recent analysis for the user,
	// or nil, nil if none exists.
	GetLatestAnalysis(ctx context.Context, userID string) (*models.RebalanceAnalysis, error)
}
