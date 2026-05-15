// api/handlers/notification_settings_handler.go
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type NotificationSettingsHandler struct {
	repo     ports.ProfileRepository
	identity ports.IdentityProvider
}

func NewNotificationSettingsHandler(repo ports.ProfileRepository, identity ports.IdentityProvider) *NotificationSettingsHandler {
	return &NotificationSettingsHandler{repo: repo, identity: identity}
}

func (h *NotificationSettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.getNotifications(w, r)
	case http.MethodPatch:
		h.patchNotifications(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type notificationSettingsResponse struct {
	NotificationEmail string `json:"notification_email"`
	Phone             string `json:"phone"`
}

func (h *NotificationSettingsHandler) getNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	profile, err := h.repo.GetByUserID(r.Context(), userID)
	if errors.Is(err, ports.ErrProfileNotFound) {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[notifications] get: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notificationSettingsResponse{
		NotificationEmail: profile.NotificationEmail,
		Phone:             profile.Phone,
	})
}

func (h *NotificationSettingsHandler) patchNotifications(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req notificationSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	profile, err := h.repo.GetByUserID(r.Context(), userID)
	if errors.Is(err, ports.ErrProfileNotFound) {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("[notifications] patch get: %v", err)
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	profile.NotificationEmail = req.NotificationEmail
	profile.Phone = req.Phone

	if err := h.repo.Upsert(r.Context(), profile); err != nil {
		log.Printf("[notifications] patch upsert: %v", err)
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notificationSettingsResponse{
		NotificationEmail: profile.NotificationEmail,
		Phone:             profile.Phone,
	})
}
