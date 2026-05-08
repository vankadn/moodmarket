// api/handlers/auto_invest_handler.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
	"github.com/krishnarajivvns/investiq/internal/middleware"
)

type autoInvestHandler struct {
	profileRepo ports.ProfileRepository
	idp         middleware.ContextIdentityProvider
}

func NewAutoInvestHandler(profileRepo ports.ProfileRepository, idp middleware.ContextIdentityProvider) http.Handler {
	return &autoInvestHandler{profileRepo: profileRepo, idp: idp}
}

type autoInvestRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *autoInvestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.idp.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req autoInvestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var enabledAt time.Time
	if req.Enabled {
		enabledAt = time.Now()
	}

	if err := h.profileRepo.SetAutoInvest(r.Context(), userID, req.Enabled, enabledAt); err != nil {
		http.Error(w, "failed to update auto-invest setting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"auto_invest_enabled":    req.Enabled,
		"auto_invest_enabled_at": enabledAt,
	})
}
