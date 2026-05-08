// api/handlers/activity_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type ActivityHandler struct {
	identityProvider ports.IdentityProvider
	decisionRepo     ports.DecisionRepository
}

func NewActivityHandler(identityProvider ports.IdentityProvider, decisionRepo ports.DecisionRepository) *ActivityHandler {
	return &ActivityHandler{identityProvider: identityProvider, decisionRepo: decisionRepo}
}

type activityDecision struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	TotalAmount float64   `json:"total_amount"`
	RiskLevel   string    `json:"risk_level"`
}

type activityResponse struct {
	TotalDecisions int                `json:"total_decisions"`
	TotalInvested  float64            `json:"total_invested"`
	Decisions      []activityDecision `json:"decisions"`
}

func (h *ActivityHandler) GetActivity(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var since *time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			http.Error(w, "invalid since param — use RFC3339", http.StatusBadRequest)
			return
		}
		since = &t
	}

	decisions, err := h.decisionRepo.ListByUserSince(r.Context(), userID, since)
	if err != nil {
		log.Printf("[activity] list decisions: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := activityResponse{
		TotalDecisions: len(decisions),
		Decisions:      make([]activityDecision, len(decisions)),
	}
	for i, d := range decisions {
		resp.TotalInvested += d.TotalAmount
		resp.Decisions[i] = activityDecision{
			ID:          d.ID,
			Timestamp:   d.Timestamp,
			TotalAmount: d.TotalAmount,
			RiskLevel:   d.RiskLevel,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
