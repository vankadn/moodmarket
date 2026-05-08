package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// CORS wraps next and handles preflight + response headers for every route.
// This is the single source of CORS truth — individual handlers need not call setCORSHeaders.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserIdentity wraps next, validating the bearer token on every non-OPTIONS request.
// /auth/* routes are passed through without identity injection — they are the login endpoints.
func UserIdentity(provider ports.AuthProvider, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Auth routes are unauthenticated by definition — they establish identity.
		if strings.HasPrefix(r.URL.Path, "/auth/") {
			next.ServeHTTP(w, r)
			return
		}

		token := bearerToken(r)
		identity, err := provider.ValidateToken(r.Context(), token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := ports.WithUserIdentity(r.Context(), identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bearerToken extracts the token from "Authorization: Bearer <token>".
// Returns empty string if the header is absent or malformed.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if after, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// ContextIdentityProvider implements ports.IdentityProvider by reading UserIdentity from context.
// Handlers depend on this interface — they never touch context keys or middleware directly.
type ContextIdentityProvider struct{}

func (ContextIdentityProvider) GetCurrentUser(ctx context.Context) (string, error) {
	return ports.UserIDFrom(ctx)
}
