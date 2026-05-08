// domain/ports/auto_invest_repository.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// AutoInvestRepository persists autonomous investment configurations.
type AutoInvestRepository interface {
	// GetByUserID returns the config for a user, or a default (disabled, $100, moderate)
	// if none exists yet. Never returns ErrNotFound — always returns a usable config.
	GetByUserID(ctx context.Context, userID string) (*models.AutoInvestConfig, error)

	// Upsert creates or replaces the config for the user.
	Upsert(ctx context.Context, config *models.AutoInvestConfig) error

	// GetAllEnabled returns configs for all users with enabled: true.
	GetAllEnabled(ctx context.Context) ([]models.AutoInvestConfig, error)
}
