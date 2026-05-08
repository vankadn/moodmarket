// api/handlers/auto_invest_config_handler.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
	"github.com/krishnarajivvns/investiq/internal/middleware"
)

type autoInvestConfigHandler struct {
	repo ports.AutoInvestRepository
	idp  middleware.ContextIdentityProvider
}

func NewAutoInvestConfigHandler(repo ports.AutoInvestRepository, idp middleware.ContextIdentityProvider) http.Handler {
	return &autoInvestConfigHandler{repo: repo, idp: idp}
}

func (h *autoInvestConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, err := h.idp.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r, userID)
	case http.MethodPut:
		h.saveConfig(w, r, userID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *autoInvestConfigHandler) getConfig(w http.ResponseWriter, r *http.Request, userID string) {
	config, err := h.repo.GetByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

type saveConfigRequest struct {
	Enabled bool                  `json:"enabled"`
	Amount  float64               `json:"amount"`
	Risk    models.RiskTolerance  `json:"risk"`
}

func (h *autoInvestConfigHandler) saveConfig(w http.ResponseWriter, r *http.Request, userID string) {
	var req saveConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	existing, err := h.repo.GetByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load config", http.StatusInternalServerError)
		return
	}

	existing.Enabled = req.Enabled
	existing.Amount = req.Amount
	existing.Risk = req.Risk
	existing.UserID = userID

	// Record consent timestamp on first enable or re-enable.
	if req.Enabled && existing.EnabledAt.IsZero() {
		existing.EnabledAt = time.Now()
	}
	if !req.Enabled {
		existing.EnabledAt = time.Time{}
	}

	if err := h.repo.Upsert(r.Context(), existing); err != nil {
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}
