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
}
