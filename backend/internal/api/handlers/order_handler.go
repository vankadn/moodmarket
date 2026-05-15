// api/handlers/order_handler.go
package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type OrderHandler struct {
	identityProvider ports.IdentityProvider
	profileRepo      ports.ProfileRepository
	brokerageFactory ports.BrokerageProviderFactory
}

func NewOrderHandler(identityProvider ports.IdentityProvider, profileRepo ports.ProfileRepository, brokerageFactory ports.BrokerageProviderFactory) *OrderHandler {
	return &OrderHandler{
		identityProvider: identityProvider,
		profileRepo:      profileRepo,
		brokerageFactory: brokerageFactory,
	}
}

// GetOrder handles GET /orders/{orderID} — returns the current status of a brokerage order.
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orderID := strings.TrimPrefix(r.URL.Path, "/orders/")
	if orderID == "" {
		http.Error(w, "missing order ID", http.StatusBadRequest)
		return
	}

	connections, _ := h.profileRepo.GetBrokerageConnections(r.Context(), userID)
	if len(connections) == 0 {
		http.Error(w, "no brokerage account connected", http.StatusBadRequest)
		return
	}
	// Use the first connected connection — order lookup doesn't need routing.
	brokerage, err := h.brokerageFactory.ForUser(&connections[0])
	if err != nil {
		if errors.Is(err, ports.ErrBrokerageNotConnected) {
			http.Error(w, "no brokerage account connected", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	receipt, err := brokerage.GetOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("[order] get order %s: %v", orderID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(receipt)
}
