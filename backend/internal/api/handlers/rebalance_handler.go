package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
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

type rebalanceAnalysisStore interface {
	SaveAnalysis(ctx context.Context, analysis *models.RebalanceAnalysis) error
	GetLatestAnalysis(ctx context.Context, userID string) (*models.RebalanceAnalysis, error)
}

type RebalanceHandler struct {
	aggregator    rebalanceAggregator
	advisor       rebalanceAdvisorUseCase
	profileRepo   rebalanceProfileReader
	identity      ports.IdentityProvider
	analysisStore rebalanceAnalysisStore
}

func NewRebalanceHandler(
	aggregator rebalanceAggregator,
	advisor rebalanceAdvisorUseCase,
	profileRepo rebalanceProfileReader,
	identity ports.IdentityProvider,
	analysisStore rebalanceAnalysisStore,
) *RebalanceHandler {
	return &RebalanceHandler{
		aggregator:    aggregator,
		advisor:       advisor,
		profileRepo:   profileRepo,
		identity:      identity,
		analysisStore: analysisStore,
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

	// Parse force flag from request body — defaults to false (use cache).
	var reqBody struct {
		Force bool `json:"force"`
	}
	json.NewDecoder(r.Body).Decode(&reqBody) //nolint:errcheck

	// Cache-and-serve: if a recent analysis exists and force=false, return it immediately.
	// REBALANCE_CACHE_HOURS controls freshness window (default 24h = 1 day).
	cacheHours := 24
	if raw := os.Getenv("REBALANCE_CACHE_HOURS"); raw != "" {
		if n, parseErr := strconv.Atoi(raw); parseErr == nil && n > 0 {
			cacheHours = n
		}
	}
	if !reqBody.Force {
		cached, err := h.analysisStore.GetLatestAnalysis(ctx, userID)
		if err != nil {
			log.Printf("rebalance handler: cache lookup: %v", err)
			// non-fatal — fall through to Claude
		} else if cached != nil && time.Since(cached.GeneratedAt) < time.Duration(cacheHours)*time.Hour {
			log.Printf("rebalance handler: returning cached analysis (age=%s)", time.Since(cached.GeneratedAt).Round(time.Minute))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cached) //nolint:errcheck
			return
		}
	}

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

	// Persist asynchronously — never block the response on the DB write.
	go func() {
		analysis.UserID = userID
		if saveErr := h.analysisStore.SaveAnalysis(context.WithoutCancel(ctx), analysis); saveErr != nil {
			log.Printf("rebalance handler: save analysis: %v", saveErr)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analysis) //nolint:errcheck
}
