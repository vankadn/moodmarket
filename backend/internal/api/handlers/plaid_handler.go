// api/handlers/plaid_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// plaidFinancialProvider and plaidProfileRepo are local interfaces so this handler
// never imports the infrastructure or services packages directly.
type plaidFinancialProvider interface {
	CreateLinkToken(ctx context.Context, userID string) (string, error)
	ExchangePublicToken(ctx context.Context, publicToken string) (accessToken string, itemID string, institutionName string, err error)
	RevokeToken(ctx context.Context, accessToken string) error
}

type plaidProfileRepo interface {
	SavePlaidConnection(ctx context.Context, userID string, connection models.PlaidConnection) error
	GetPlaidConnections(ctx context.Context, userID string) ([]models.PlaidConnection, error)
	RemovePlaidConnection(ctx context.Context, userID string, itemID string) error
}

type PlaidHandler struct {
	financial plaidFinancialProvider
	profile   plaidProfileRepo
	identity  ports.IdentityProvider
}

func NewPlaidHandler(financial plaidFinancialProvider, profile plaidProfileRepo, identity ports.IdentityProvider) *PlaidHandler {
	return &PlaidHandler{financial: financial, profile: profile, identity: identity}
}

func (h *PlaidHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case r.URL.Path == "/plaid/link-token" && r.Method == http.MethodPost:
		h.createLinkToken(w, r)
	case r.URL.Path == "/plaid/exchange" && r.Method == http.MethodPost:
		h.exchangeToken(w, r)
	case strings.HasPrefix(r.URL.Path, "/plaid/accounts/") && r.Method == http.MethodDelete:
		h.disconnectAccount(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// createLinkToken calls Plaid to generate a Link initialization token for the authenticated user.
func (h *PlaidHandler) createLinkToken(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	linkToken, err := h.financial.CreateLinkToken(ctx, userID)
	if err != nil {
		log.Printf("plaid handler: create link token: %v", err)
		http.Error(w, "failed to create link token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		LinkToken string `json:"link_token"`
	}{LinkToken: linkToken})
}

// exchangeToken exchanges a public_token for a permanent access_token, fetches the institution
// name, and persists the encrypted connection to the user's profile document.
func (h *PlaidHandler) exchangeToken(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		PublicToken string `json:"public_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PublicToken == "" {
		http.Error(w, "public_token is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	accessToken, itemID, institution, err := h.financial.ExchangePublicToken(ctx, body.PublicToken)
	if err != nil {
		log.Printf("plaid handler: exchange token: %v", err)
		http.Error(w, "failed to exchange token", http.StatusInternalServerError)
		return
	}

	connection := models.PlaidConnection{
		Institution: institution,
		AccessToken: accessToken,
		ItemID:      itemID,
	}
	if err := h.profile.SavePlaidConnection(ctx, userID, connection); err != nil {
		log.Printf("plaid handler: save connection: %v", err)
		http.Error(w, "failed to save connection", http.StatusInternalServerError)
		return
	}

	connections, err := h.profile.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("plaid handler: count connections: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Institution       string `json:"institution"`
		ConnectedAccounts int    `json:"connected_accounts"`
	}{
		Institution:       institution,
		ConnectedAccounts: len(connections),
	})
}

// disconnectAccount revokes the Plaid item and removes the connection from the user's profile.
func (h *PlaidHandler) disconnectAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	itemID := strings.TrimPrefix(r.URL.Path, "/plaid/accounts/")
	if itemID == "" {
		http.Error(w, "item_id is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Find the access_token for this item_id so we can call Plaid's item/remove.
	connections, err := h.profile.GetPlaidConnections(ctx, userID)
	if err != nil {
		log.Printf("plaid handler: fetch connections for disconnect: %v", err)
		http.Error(w, "failed to fetch connections", http.StatusInternalServerError)
		return
	}

	var accessToken string
	for _, c := range connections {
		if c.ItemID == itemID {
			accessToken = c.AccessToken
			break
		}
	}
	if accessToken == "" {
		http.Error(w, "connection not found", http.StatusNotFound)
		return
	}

	if err := h.financial.RevokeToken(ctx, accessToken); err != nil {
		log.Printf("plaid handler: revoke token for item %s: %v", itemID, err)
		// Continue to remove from DB even if Plaid revocation fails — avoid orphaned records.
	}

	if err := h.profile.RemovePlaidConnection(ctx, userID, itemID); err != nil {
		log.Printf("plaid handler: remove connection: %v", err)
		http.Error(w, "failed to remove connection", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
