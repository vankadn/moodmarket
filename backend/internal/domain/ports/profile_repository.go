// domain/ports/profile_repository.go
package ports

import (
	"context"
	"errors"

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

	// GetBrokerageConnection returns the decrypted brokerage credentials for the user.
	// Returns nil, nil when no connection exists.
	GetBrokerageConnection(ctx context.Context, userID string) (*models.BrokerageConnection, error)

	// SaveBrokerageConnection stores encrypted Alpaca credentials on the user's document.
	// A second call overwrites the previous connection.
	SaveBrokerageConnection(ctx context.Context, userID string, conn models.BrokerageConnection) error

	// ClearBrokerageConnection removes brokerage credentials from the user's document.
	ClearBrokerageConnection(ctx context.Context, userID string) error
}
