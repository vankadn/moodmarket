// api/handlers/strategy_pnl_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type StrategyPnLHandler struct {
	identity         ports.IdentityProvider
	decisionRepo     ports.DecisionRepository
	profileRepo      ports.ProfileRepository
	brokerageFactory ports.BrokerageProviderFactory
}

func NewStrategyPnLHandler(
	identity ports.IdentityProvider,
	decisionRepo ports.DecisionRepository,
	profileRepo ports.ProfileRepository,
	brokerageFactory ports.BrokerageProviderFactory,
) *StrategyPnLHandler {
	return &StrategyPnLHandler{
		identity:         identity,
		decisionRepo:     decisionRepo,
		profileRepo:      profileRepo,
		brokerageFactory: brokerageFactory,
	}
}

type strategyPnLItem struct {
	ConfigID           string   `json:"config_id"`
	TotalInvested      float64  `json:"total_invested"`
	CurrentValue       float64  `json:"current_value"`
	UnrealizedPL       float64  `json:"unrealized_pl"`
	UnrealizedPLPct    float64  `json:"unrealized_pl_pct"`
	BrokerageConnected bool     `json:"brokerage_connected"`
	Tickers            []string `json:"tickers"`
}

func (h *StrategyPnLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Step 1: cost basis per (configID, ticker) from all tagged decisions.
	basisMap, err := h.decisionRepo.CostBasisByStrategy(r.Context(), userID)
	if err != nil {
		log.Printf("[strategy-pnl] cost basis: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Step 2: total cost basis per ticker across all strategies — used for proportional attribution.
	totalBasis := make(map[string]float64)
	for _, tickerMap := range basisMap {
		for ticker, amt := range tickerMap {
			totalBasis[ticker] += amt
		}
	}

	// Step 3: fetch live positions from all brokerage connections.
	// Both maps are keyed by ticker; values are summed across connections when a ticker
	// appears in multiple accounts (deduplication follows the same first-seen logic as
	// RecommendationService, but for P&L we sum rather than deduplicate — each connection
	// holds different shares of the same ticker).
	posValue := make(map[string]float64) // ticker → market_value
	posPL := make(map[string]float64)    // ticker → unrealized_pl
	brokerageConnected := false

	connections, err := h.profileRepo.GetBrokerageConnections(r.Context(), userID)
	if err != nil {
		log.Printf("[strategy-pnl] load connections: %v", err)
	} else if len(connections) > 0 {
		brokerageConnected = true
		for i := range connections {
			provider, err := h.brokerageFactory.ForUser(&connections[i])
			if err != nil {
				log.Printf("[strategy-pnl] build provider for %s: %v", connections[i].ID, err)
				continue
			}
			positions, err := provider.GetPositions(r.Context(), userID)
			if err != nil {
				log.Printf("[strategy-pnl] get positions from %s: %v", connections[i].ID, err)
				continue
			}
			for _, pos := range positions {
				posValue[pos.Ticker] += pos.MarketValue
				posPL[pos.Ticker] += pos.UnrealizedPL
			}
		}
	}

	// Step 4: attribute proportional P&L to each strategy.
	result := make([]strategyPnLItem, 0, len(basisMap))
	for configID, tickerMap := range basisMap {
		item := strategyPnLItem{
			ConfigID:           configID,
			BrokerageConnected: brokerageConnected,
			Tickers:            make([]string, 0, len(tickerMap)),
		}
		for ticker, amt := range tickerMap {
			item.TotalInvested += amt
			item.Tickers = append(item.Tickers, ticker)
			if brokerageConnected && totalBasis[ticker] > 0 {
				fraction := amt / totalBasis[ticker]
				item.CurrentValue += fraction * posValue[ticker]
				item.UnrealizedPL += fraction * posPL[ticker]
			}
		}
		if item.TotalInvested > 0 && brokerageConnected {
			item.UnrealizedPLPct = (item.UnrealizedPL / item.TotalInvested) * 100
		}
		sort.Strings(item.Tickers)
		result = append(result, item)
	}

	// Sort by total invested descending for stable, consistent ordering.
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalInvested > result[j].TotalInvested
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
