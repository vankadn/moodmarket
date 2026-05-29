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

type rebalanceAggregator interface {
	BuildRequest(ctx context.Context, userID string) (*models.RebalanceRequest, error)
}

type rebalanceAdvisorUseCase interface {
	AnalyzePortfolio(ctx context.Context, req models.RebalanceRequest, profile *models.UserProfile) (*models.RebalanceAnalysis, error)
}

type rebalanceProfileReader interface {
	GetByUserID(ctx context.Context, userID string) (*models.UserProfile, error)
}

type RebalanceHandler struct {
	aggregator  rebalanceAggregator
	advisor     rebalanceAdvisorUseCase
	profileRepo rebalanceProfileReader
	identity    ports.IdentityProvider
}

func NewRebalanceHandler(
	aggregator rebalanceAggregator,
	advisor rebalanceAdvisorUseCase,
	profileRepo rebalanceProfileReader,
	identity ports.IdentityProvider,
) *RebalanceHandler {
	return &RebalanceHandler{
		aggregator:  aggregator,
		advisor:     advisor,
		profileRepo: profileRepo,
		identity:    identity,
	}
}

func (h *RebalanceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Allow enough time for Alpaca + SnapTrade fetches + Claude analysis.
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// Step 1: assemble positions + buy reasoning from all sources.
	req, err := h.aggregator.BuildRequest(ctx, userID)
	if err != nil {
		log.Printf("rebalance handler: build request: %v", err)
		http.Error(w, "failed to assemble portfolio data", http.StatusInternalServerError)
		return
	}

	// Step 2: load user profile (non-fatal if missing — Claude degrades gracefully).
	profile, err := h.profileRepo.GetByUserID(ctx, userID)
	if err != nil && !errors.Is(err, ports.ErrProfileNotFound) {
		log.Printf("rebalance handler: load profile: %v", err)
	}

	// Step 3: Claude analysis.
	analysis, err := h.advisor.AnalyzePortfolio(ctx, *req, profile)
	if err != nil {
		log.Printf("rebalance handler: analyze: %v", err)
		if errors.Is(err, ports.ErrAdvisorOverloaded) {
			http.Error(w, "advisor temporarily unavailable, please try again", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, "failed to generate rebalance analysis", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis)
}
