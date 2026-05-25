// api/handlers/performance_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type PerformanceHandler struct {
	identityProvider ports.IdentityProvider
	decisionRepo     ports.DecisionRepository
}

func NewPerformanceHandler(identityProvider ports.IdentityProvider, decisionRepo ports.DecisionRepository) *PerformanceHandler {
	return &PerformanceHandler{identityProvider: identityProvider, decisionRepo: decisionRepo}
}

type winRateTrendPoint struct {
	Week    string  `json:"week"`
	Total   int     `json:"total"`
	Wins    int     `json:"wins"`
	WinRate float64 `json:"win_rate"`
}

func (h *PerformanceHandler) GetWinRateTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	points, err := h.decisionRepo.WinRateTrend(r.Context(), userID, 12)
	if err != nil {
		log.Printf("[performance] win rate trend: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]winRateTrendPoint, len(points))
	for i, p := range points {
		resp[i] = winRateTrendPoint{
			Week:    p.Week,
			Total:   p.Total,
			Wins:    p.Wins,
			WinRate: safeFloat(p.WinRate),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type assetClassBreakdownItem struct {
	AssetClass string  `json:"asset_class"`
	Total      int     `json:"total"`
	Wins       int     `json:"wins"`
	WinRate    float64 `json:"win_rate"`
}

func (h *PerformanceHandler) GetAssetClassBreakdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.decisionRepo.AssetClassBreakdown(r.Context(), userID)
	if err != nil {
		log.Printf("[performance] asset class breakdown: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]assetClassBreakdownItem, len(items))
	for i, item := range items {
		resp[i] = assetClassBreakdownItem{
			AssetClass: item.AssetClass,
			Total:      item.Total,
			Wins:       item.Wins,
			WinRate:    safeFloat(item.WinRate),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
