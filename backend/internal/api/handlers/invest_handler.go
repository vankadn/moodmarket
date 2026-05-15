// api/handlers/invest_handler.go
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// investUseCase is the local interface the handler depends on.
// Defined here so the handler never imports the services package directly.
type investUseCase interface {
	Execute(ctx context.Context, userID string, allocations []models.Allocation, totalAmount float64, riskLevel, summary string, perAllocBrokerage map[string]string) ([]models.TradeReceipt, string, error)
}

type InvestHandler struct {
	service  investUseCase
	identity ports.IdentityProvider
}

func NewInvestHandler(svc investUseCase, identity ports.IdentityProvider) *InvestHandler {
	return &InvestHandler{service: svc, identity: identity}
}

func (h *InvestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Allocations       []models.Allocation `json:"allocations"`
		TotalAmount       float64             `json:"total_amount"`
		RiskLevel         string              `json:"risk_level"`
		Summary           string              `json:"summary"`
		PerAllocBrokerage map[string]string   `json:"per_allocation_brokerage,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(body.Allocations) == 0 {
		http.Error(w, "allocations must not be empty", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	receipts, decisionID, err := h.service.Execute(ctx, userID, body.Allocations, body.TotalAmount, body.RiskLevel, body.Summary, body.PerAllocBrokerage)
	if err != nil {
		if errors.Is(err, ports.ErrBrokerageNotConnected) {
			http.Error(w, "no brokerage account connected", http.StatusBadRequest)
			return
		}
		log.Printf("invest handler: %v", err)
		http.Error(w, "failed to execute investment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Receipts   []models.TradeReceipt `json:"receipts"`
		DecisionID string                `json:"decision_id"`
	}{
		Receipts:   receipts,
		DecisionID: decisionID,
	})
}
