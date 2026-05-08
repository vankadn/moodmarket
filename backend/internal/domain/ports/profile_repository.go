// domain/ports/profile_repository.go
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// ErrProfileNotFound is returned by ProfileRepository when no profile exists for the given user.
var ErrProfileNotFound = errors.New("profile not found")

// ProfileRepository is the port any persistence layer must implement.
type ProfileRepository interface {
	GetByUserID(ctx context.Context, userID string) (*models.UserProfile, error)
	Upsert(ctx context.Context, profile *models.UserProfile) error

	// SavePlaidConnection appends a new Plaid connection to the user's document.
	// It does not overwrite existing connections — each call adds one entry.
	// The implementation is responsible for encrypting AccessToken before writing.
	SavePlaidConnection(ctx context.Context, userID string, connection models.PlaidConnection) error

	// GetPlaidConnections returns all Plaid connections for the user with decrypted access tokens.
	GetPlaidConnections(ctx context.Context, userID string) ([]models.PlaidConnection, error)

	// RemovePlaidConnection removes the connection matching itemID from the user's document.
	RemovePlaidConnection(ctx context.Context, userID string, itemID string) error

	// GetAutoInvestUsers returns all users with auto_invest_enabled: true.
	GetAutoInvestUsers(ctx context.Context) ([]models.UserProfile, error)

	// SetAutoInvest updates only the auto_invest_enabled flag and consent timestamp.
	// It never touches any other profile field.
	SetAutoInvest(ctx context.Context, userID string, enabled bool, enabledAt time.Time) error
}
