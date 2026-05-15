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

	// GetBrokerageConnections returns all brokerage connections for the user with decrypted credentials.
	// Falls back to legacy brokerage_connection field for users with no multi-connection array,
	// synthesizing a single entry with ID="default" and AssetCategories=["default"].
	// Returns nil, nil when no connection exists anywhere.
	GetBrokerageConnections(ctx context.Context, userID string) ([]models.BrokerageConnection, error)

	// UpsertBrokerageConnection adds or updates a connection identified by conn.ID.
	// Encrypts credentials before writing. If no entry with matching ID exists, appends a new one.
	UpsertBrokerageConnection(ctx context.Context, userID string, conn models.BrokerageConnection) error

	// RemoveBrokerageConnection removes the connection with the given ID from brokerage_connections.
	RemoveBrokerageConnection(ctx context.Context, userID string, connectionID string) error

	// SaveLegacySingleBrokerageConnection stores encrypted Alpaca credentials using the old
	// single-connection field. Used only by the legacy POST /brokerage/connect endpoint.
	SaveLegacySingleBrokerageConnection(ctx context.Context, userID string, conn models.BrokerageConnection) error

	// ClearLegacySingleBrokerageConnection removes the legacy brokerage_connection field.
	// Used only by the legacy DELETE /brokerage/connect endpoint.
	ClearLegacySingleBrokerageConnection(ctx context.Context, userID string) error
}
