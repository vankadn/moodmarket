// api/handlers/brokerage_connections_handler.go
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/api/router"
	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// BrokerageConnectionsHandler handles POST /brokerage/connections and DELETE /brokerage/connections/{id}.
type BrokerageConnectionsHandler struct {
	profileRepo      ports.ProfileRepository
	identityProvider ports.IdentityProvider
}

func NewBrokerageConnectionsHandler(profileRepo ports.ProfileRepository, idp ports.IdentityProvider) *BrokerageConnectionsHandler {
	return &BrokerageConnectionsHandler{profileRepo: profileRepo, identityProvider: idp}
}

func (h *BrokerageConnectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if r.URL.Path == router.BrokerageConnectionsURI {
			h.add(w, r)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	case http.MethodDelete:
		id := strings.TrimPrefix(r.URL.Path, router.BrokerageConnectionsURI+"/")
		if id == "" {
			http.Error(w, "missing connection ID", http.StatusBadRequest)
			return
		}
		h.remove(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BrokerageConnectionsHandler) add(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		ID              string   `json:"id"`
		Name            string   `json:"name"`
		AssetCategories []string `json:"asset_categories"`
		APIKey          string   `json:"api_key"`
		SecretKey       string   `json:"secret_key"`
		BaseURL         string   `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.APIKey == "" || body.SecretKey == "" {
		http.Error(w, "api_key and secret_key are required", http.StatusBadRequest)
		return
	}
	if len(body.AssetCategories) == 0 {
		http.Error(w, "asset_categories must not be empty", http.StatusBadRequest)
		return
	}
	for _, cat := range body.AssetCategories {
		if cat != string(models.AssetCategoryEquity) && cat != string(models.AssetCategoryBond) && cat != string(models.AssetCategoryDefault) {
			http.Error(w, "asset_categories must be one of: equity, bond, default", http.StatusBadRequest)
			return
		}
	}
	if body.BaseURL == "" {
		body.BaseURL = "https://paper-api.alpaca.markets"
	}
	if body.ID == "" {
		body.ID = generateID()
	}

	cats := make([]models.AssetCategory, len(body.AssetCategories))
	for i, c := range body.AssetCategories {
		cats[i] = models.AssetCategory(c)
	}

	conn := models.BrokerageConnection{
		ID:              body.ID,
		Name:            body.Name,
		AssetCategories: cats,
		APIKey:          body.APIKey,
		SecretKey:       body.SecretKey,
		BaseURL:         body.BaseURL,
		Connected:       true,
		ConnectedAt:     time.Now(),
	}
	if err := h.profileRepo.UpsertBrokerageConnection(r.Context(), userID, conn); err != nil {
		log.Printf("[brokerage-connections] add: upsert failed for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("[brokerage-connections] user=%s added connection id=%s name=%q cats=%v", userID, body.ID, body.Name, body.AssetCategories)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.BrokerageStatus{
		ID:              conn.ID,
		Name:            conn.Name,
		AssetCategories: conn.AssetCategories,
		Connected:       true,
		BaseURL:         conn.BaseURL,
		ConnectedAt:     conn.ConnectedAt.Format(time.RFC3339),
	})
}

func (h *BrokerageConnectionsHandler) remove(w http.ResponseWriter, r *http.Request, connectionID string) {
	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.profileRepo.RemoveBrokerageConnection(r.Context(), userID, connectionID); err != nil {
		log.Printf("[brokerage-connections] remove: failed for user %s connection %s: %v", userID, connectionID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("[brokerage-connections] user=%s removed connection id=%s", userID, connectionID)
	w.WriteHeader(http.StatusNoContent)
}

// generateID creates a random 8-byte hex string suitable as a connection ID.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
