package auth

import (
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewAuthProvider is the single place in the codebase that reads DEV_MODE.
// No other file should branch on DEV_MODE — swap the provider here, behavior follows everywhere.
func NewAuthProvider() ports.AuthProvider {
	if os.Getenv("DEV_MODE") == "true" {
		return NewDevAuthProvider()
	}
	return NewClerkAuthProvider()
}
