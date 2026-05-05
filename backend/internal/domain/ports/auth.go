package ports

import "context"

// AuthProvider is the port any authentication system must implement.
// Implementations live in infrastructure/auth — never imported by handlers directly.
type AuthProvider interface {
	// ValidateToken validates an incoming bearer token and returns the caller's identity.
	// In DEV_MODE the token may be empty; the implementation decides whether to accept it.
	ValidateToken(ctx context.Context, token string) (*UserIdentity, error)
	// GetLoginURL returns the OAuth redirect URL used by the login page.
	GetLoginURL() string
}

// UserIdentity is the verified identity of the authenticated caller.
// It is a value object — no database ID, no mutable state.
type UserIdentity struct {
	UserID string
	Email  string
	Name   string
}
