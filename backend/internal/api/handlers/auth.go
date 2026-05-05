package handlers

import (
	"encoding/json"
	"net/http"
)

// DevLoginHandler serves GET /auth/dev-login.
// Returns a placeholder token the frontend stores in localStorage.
// DevAuthProvider ignores the token value — it is only a "logged in" signal to the frontend.
// In Phase 4 this route is superseded by Clerk's OAuth redirect.
type DevLoginHandler struct{}

func NewDevLoginHandler() *DevLoginHandler {
	return &DevLoginHandler{}
}

func (h *DevLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": "dev-token"})
}
