// api/handlers/auto_invest_configs_handler.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
	"github.com/krishnarajivvns/investiq/internal/middleware"
)

const autoInvestConfigsBase = "/users/auto-invest/configs"

type autoInvestConfigsHandler struct {
	repo ports.AutoInvestRepository
	idp  middleware.ContextIdentityProvider
}

func NewAutoInvestConfigsHandler(repo ports.AutoInvestRepository, idp middleware.ContextIdentityProvider) http.Handler {
	return &autoInvestConfigsHandler{repo: repo, idp: idp}
}

func (h *autoInvestConfigsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, err := h.idp.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	suffix := strings.TrimPrefix(r.URL.Path, autoInvestConfigsBase)
	configID := strings.Trim(suffix, "/")
	hasID := configID != ""

	switch {
	case r.Method == http.MethodGet && !hasID:
		h.list(w, r, userID)
	case r.Method == http.MethodPost && !hasID:
		h.create(w, r, userID)
	case r.Method == http.MethodPut && hasID:
		h.update(w, r, userID, configID)
	case r.Method == http.MethodDelete && hasID:
		h.delete(w, r, userID, configID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *autoInvestConfigsHandler) list(w http.ResponseWriter, r *http.Request, userID string) {
	configs, err := h.repo.GetAllByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to load configs", http.StatusInternalServerError)
		return
	}
	if configs == nil {
		configs = []models.AutoInvestConfig{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configs) //nolint:errcheck
}

type multiConfigRequest struct {
	Name            string               `json:"name"`
	Enabled         bool                 `json:"enabled"`
	Amount          float64              `json:"amount"`
	Risk            models.RiskTolerance `json:"risk"`
	Strategy        string               `json:"strategy"`
	IntervalDays    int                  `json:"interval_days,omitempty"`
	IntervalSeconds int                  `json:"interval_seconds,omitempty"`
	EnabledAt       *time.Time           `json:"enabled_at,omitempty"`
}

func (h *autoInvestConfigsHandler) create(w http.ResponseWriter, r *http.Request, userID string) {
	var req multiConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	var enabledAt time.Time
	if req.Enabled {
		enabledAt = time.Now()
	}

	config := &models.AutoInvestConfig{
		UserID:          userID,
		Name:            req.Name,
		Enabled:         req.Enabled,
		Amount:          req.Amount,
		Risk:            req.Risk,
		Strategy:        req.Strategy,
		IntervalDays:    req.IntervalDays,
		IntervalSeconds: req.IntervalSeconds,
		EnabledAt:       enabledAt,
	}

	created, err := h.repo.Create(r.Context(), config)
	if err != nil {
		http.Error(w, "failed to create config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created) //nolint:errcheck
}

func (h *autoInvestConfigsHandler) update(w http.ResponseWriter, r *http.Request, userID, configID string) {
	var req multiConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Preserve EnabledAt if enabling and the frontend sent back the existing timestamp.
	// If enabling with no prior timestamp, set to now. If disabling, leave zero to clear it.
	var enabledAt time.Time
	if req.Enabled {
		if req.EnabledAt != nil && !req.EnabledAt.IsZero() {
			enabledAt = *req.EnabledAt
		} else {
			enabledAt = time.Now()
		}
	}

	config := &models.AutoInvestConfig{
		Name:            req.Name,
		Enabled:         req.Enabled,
		Amount:          req.Amount,
		Risk:            req.Risk,
		Strategy:        req.Strategy,
		IntervalDays:    req.IntervalDays,
		IntervalSeconds: req.IntervalSeconds,
		EnabledAt:       enabledAt,
	}

	updated, err := h.repo.UpdateByID(r.Context(), configID, userID, config)
	if err != nil {
		http.Error(w, "failed to update config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated) //nolint:errcheck
}

func (h *autoInvestConfigsHandler) delete(w http.ResponseWriter, r *http.Request, userID, configID string) {
	if err := h.repo.DeleteByID(r.Context(), configID, userID); err != nil {
		http.Error(w, "failed to delete config", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
