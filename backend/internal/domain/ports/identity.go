package ports

import (
	"context"
	"errors"
)

// ErrUnauthenticated is returned when no user identity is present in the context.
var ErrUnauthenticated = errors.New("unauthenticated")

// IdentityProvider is the port any auth system must implement.
// Handlers depend on this interface, never on middleware directly.
type IdentityProvider interface {
	GetCurrentUser(ctx context.Context) (string, error)
}

type identityKey struct{}

// WithUserIdentity stores the full UserIdentity in ctx. Called only by auth middleware.
func WithUserIdentity(ctx context.Context, identity *UserIdentity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

// UserIDFrom retrieves the userID from ctx. Returns ErrUnauthenticated if absent.
func UserIDFrom(ctx context.Context) (string, error) {
	id, ok := ctx.Value(identityKey{}).(*UserIdentity)
	if !ok || id == nil {
		return "", ErrUnauthenticated
	}
	return id.UserID, nil
}
