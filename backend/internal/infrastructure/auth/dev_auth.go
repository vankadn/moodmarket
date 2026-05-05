package auth

import (
	"context"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// DevAuthProvider is used when DEV_MODE=true.
// ValidateToken always succeeds — token content is irrelevant in local development.
// Identity values are read from env at construction time, not per-request.
type DevAuthProvider struct {
	identity *ports.UserIdentity
}

func NewDevAuthProvider() *DevAuthProvider {
	return &DevAuthProvider{
		identity: &ports.UserIdentity{
			UserID: os.Getenv("DEV_USER_ID"),
			Email:  os.Getenv("DEV_USER_EMAIL"),
			Name:   os.Getenv("DEV_USER_NAME"),
		},
	}
}

func (p *DevAuthProvider) ValidateToken(_ context.Context, _ string) (*ports.UserIdentity, error) {
	return p.identity, nil
}

func (p *DevAuthProvider) GetLoginURL() string {
	return ""
}
