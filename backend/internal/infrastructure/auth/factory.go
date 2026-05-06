// infrastructure/auth/factory.go
package auth

import (
	"fmt"
	"os"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// NewAuthProvider is the single place in the codebase that reads DEV_MODE.
// No other file should branch on DEV_MODE — swap the provider here, behaviour follows everywhere.
func NewAuthProvider() (ports.AuthProvider, error) {
	if os.Getenv("DEV_MODE") == "true" {
		return NewDevAuthProvider(), nil
	}
	secretKey := os.Getenv("CLERK_SECRET_KEY")
	if secretKey == "" {
		return nil, fmt.Errorf("auth factory: CLERK_SECRET_KEY is required when DEV_MODE=false — add it to .env")
	}
	return NewClerkAuthProvider(secretKey), nil
}
