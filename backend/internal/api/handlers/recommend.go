package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type recommendationUseCase interface {
	GetDailyRecommendation(ctx context.Context, userID string, req models.InvestmentRequest) (*models.Recommendation, error)
}

type RecommendHandler struct {
	service  recommendationUseCase
	identity ports.IdentityProvider
}

func NewRecommendHandler(svc recommendationUseCase, identity ports.IdentityProvider) *RecommendHandler {
	return &RecommendHandler{service: svc, identity: identity}
}

func (h *RecommendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	var req models.InvestmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.BaseBudget == 0 {
		req.BaseBudget = 100
	}

	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()

	rec, err := h.service.GetDailyRecommendation(ctx, userID, req)
	if err != nil {
		log.Printf("recommend handler: %v", err)
		http.Error(w, "failed to generate recommendation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}
