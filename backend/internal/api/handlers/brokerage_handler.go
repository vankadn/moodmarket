// api/handlers/brokerage_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type BrokerageHandler struct {
	profileRepo      ports.ProfileRepository
	identityProvider ports.IdentityProvider
}

func NewBrokerageHandler(profileRepo ports.ProfileRepository, idp ports.IdentityProvider) *BrokerageHandler {
	return &BrokerageHandler{profileRepo: profileRepo, identityProvider: idp}
}

func (h *BrokerageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.connect(w, r)
	case http.MethodDelete:
		h.disconnect(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BrokerageHandler) connect(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		APIKey    string `json:"api_key"`
		SecretKey string `json:"secret_key"`
		BaseURL   string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.APIKey == "" || body.SecretKey == "" {
		http.Error(w, "api_key and secret_key are required", http.StatusBadRequest)
		return
	}
	if body.BaseURL == "" {
		body.BaseURL = "https://paper-api.alpaca.markets"
	}

	conn := models.BrokerageConnection{
		APIKey:      body.APIKey,
		SecretKey:   body.SecretKey,
		BaseURL:     body.BaseURL,
		Connected:   true,
		ConnectedAt: time.Now(),
	}
	if err := h.profileRepo.SaveBrokerageConnection(r.Context(), userID, conn); err != nil {
		log.Printf("[brokerage] connect: save failed for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("[brokerage] user=%s connected base_url=%s", userID, body.BaseURL)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"connected": true, "base_url": body.BaseURL})
}

func (h *BrokerageHandler) disconnect(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.profileRepo.ClearBrokerageConnection(r.Context(), userID); err != nil {
		log.Printf("[brokerage] disconnect: clear failed for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("[brokerage] user=%s disconnected", userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"connected": false})
}
