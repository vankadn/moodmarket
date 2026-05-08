// api/handlers/cash_context_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/krishnarajivvns/investiq/internal/application/services"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type CashContextHandler struct {
	svc              *services.RecommendationService
	identityProvider ports.IdentityProvider
}

func NewCashContextHandler(svc *services.RecommendationService, idp ports.IdentityProvider) *CashContextHandler {
	return &CashContextHandler{svc: svc, identityProvider: idp}
}

func (h *CashContextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx, err := h.svc.GetCashContext(r.Context(), userID)
	if err != nil {
		log.Printf("[cash-context] handler error for user %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ctx)
}
