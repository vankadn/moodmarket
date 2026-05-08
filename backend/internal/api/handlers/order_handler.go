// api/handlers/order_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type OrderHandler struct {
	identityProvider ports.IdentityProvider
	brokerage        ports.BrokerageProvider
}

func NewOrderHandler(identityProvider ports.IdentityProvider, brokerage ports.BrokerageProvider) *OrderHandler {
	return &OrderHandler{identityProvider: identityProvider, brokerage: brokerage}
}

// GetOrder handles GET /orders/{orderID} — returns the current status of a brokerage order.
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	if _, err := h.identityProvider.GetCurrentUser(r.Context()); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orderID := strings.TrimPrefix(r.URL.Path, "/orders/")
	if orderID == "" {
		http.Error(w, "missing order ID", http.StatusBadRequest)
		return
	}

	receipt, err := h.brokerage.GetOrder(r.Context(), orderID)
	if err != nil {
		log.Printf("[order] get order %s: %v", orderID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(receipt)
}
