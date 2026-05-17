// domain/ports/auto_invest_repository.go
package ports

import (
	"context"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// AutoInvestRepository persists autonomous investment configurations.
type AutoInvestRepository interface {
	// GetByUserID returns the config for a user, or a default (disabled, $100, moderate)
	// if none exists yet. Never returns ErrNotFound — always returns a usable config.
	GetByUserID(ctx context.Context, userID string) (*models.AutoInvestConfig, error)

	// Upsert creates or replaces the single config for the user (legacy single-config endpoint).
	Upsert(ctx context.Context, config *models.AutoInvestConfig) error

	// GetAllByUserID returns all configs for a user. Returns empty slice (not error) if none exist.
	GetAllByUserID(ctx context.Context, userID string) ([]models.AutoInvestConfig, error)

	// Create inserts a new config and returns it with the generated ID.
	Create(ctx context.Context, config *models.AutoInvestConfig) (*models.AutoInvestConfig, error)

	// UpdateByID replaces a specific config. Returns error if not found or not owned by userID.
	UpdateByID(ctx context.Context, configID, userID string, config *models.AutoInvestConfig) (*models.AutoInvestConfig, error)

	// DeleteByID removes a specific config. Returns error if not found or not owned by userID.
	DeleteByID(ctx context.Context, configID, userID string) error

	// GetAllEnabled returns all configs with enabled: true across all users.
	GetAllEnabled(ctx context.Context) ([]models.AutoInvestConfig, error)

	// StampLastRunAt records the time a scheduled run was executed for a specific config.
	StampLastRunAt(ctx context.Context, configID string, t time.Time) error
}
