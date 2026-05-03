package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// defaultUserID is a stub for the single-user phase. Replace with token extraction when auth lands.
const defaultUserID = "default"

// recommendationUseCase is the application-layer contract this handler depends on.
type recommendationUseCase interface {
	GetDailyRecommendation(ctx context.Context, userID string, req models.InvestmentRequest) (*models.Recommendation, error)
}

type RecommendHandler struct {
	service recommendationUseCase
}

func NewRecommendHandler(svc recommendationUseCase) *RecommendHandler {
	return &RecommendHandler{service: svc}
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

	var req models.InvestmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.BaseBudget == 0 {
		req.BaseBudget = 100
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	rec, err := h.service.GetDailyRecommendation(ctx, defaultUserID, req)
	if err != nil {
		log.Printf("recommend handler: %v", err)
		http.Error(w, "failed to generate recommendation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}
