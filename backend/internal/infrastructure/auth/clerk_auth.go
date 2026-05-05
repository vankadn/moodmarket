package auth

import (
	"context"
	"errors"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ClerkAuthProvider will validate Clerk JWTs in Phase 4.
// Until then, ValidateToken returns an explicit "not yet implemented" error
// so any accidental DEV_MODE=false in dev fails loudly, not silently.
type ClerkAuthProvider struct {
	loginURL string
}

func NewClerkAuthProvider() *ClerkAuthProvider {
	return &ClerkAuthProvider{
		loginURL: os.Getenv("CLERK_LOGIN_URL"),
	}
}

func (p *ClerkAuthProvider) ValidateToken(_ context.Context, _ string) (*ports.UserIdentity, error) {
	return nil, errors.New("Clerk auth not yet implemented — coming Phase 4")
}

func (p *ClerkAuthProvider) GetLoginURL() string {
	return p.loginURL
}
