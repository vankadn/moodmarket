// api/handlers/portfolio_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type PortfolioHandler struct {
	profileRepo      ports.ProfileRepository
	brokerageFactory ports.BrokerageProviderFactory
	identity         ports.IdentityProvider
}

func NewPortfolioHandler(profileRepo ports.ProfileRepository, brokerageFactory ports.BrokerageProviderFactory, identity ports.IdentityProvider) *PortfolioHandler {
	return &PortfolioHandler{profileRepo: profileRepo, brokerageFactory: brokerageFactory, identity: identity}
}

type portfolioPosition struct {
	Ticker              string  `json:"ticker"`
	Name                string  `json:"name"`
	Quantity            float64 `json:"quantity"`
	MarketValue         float64 `json:"market_value"`
	CostBasis           float64 `json:"cost_basis"`
	AvgEntryPrice       float64 `json:"avg_entry_price"`
	UnrealizedPL        float64 `json:"unrealized_pl"`
	UnrealizedPLPercent float64 `json:"unrealized_pl_percent"`
}

type portfolioAccount struct {
	BrokerageID      string              `json:"brokerage_id"`
	BrokerageName    string              `json:"brokerage_name"`
	Positions        []portfolioPosition `json:"positions"`
	TotalValue       float64             `json:"total_value"`
	TotalCost        float64             `json:"total_cost"`
	TotalUnrealizedPL float64            `json:"total_unrealized_pl"`
}

type portfolioResponse struct {
	Accounts              []portfolioAccount `json:"accounts"`
	TotalValue            float64            `json:"total_value"`
	TotalCost             float64            `json:"total_cost"`
	TotalUnrealizedPL     float64            `json:"total_unrealized_pl"`
	TotalUnrealizedPLPct  float64            `json:"total_unrealized_pl_percent"`
}

// periodTimeframe maps a UI period label to the Alpaca period + timeframe parameters.
var periodTimeframe = map[string][2]string{
	"1D": {"1D", "5Min"},
	"5D": {"5D", "1H"},
	"1M": {"1M", "1D"},
	"1Y": {"1A", "1D"},
	"5Y": {"5A", "1D"},
}

func (h *PortfolioHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Dispatch sub-paths
	if r.URL.Path == "/portfolio/history" {
		h.getHistory(w, r)
		return
	}

	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	connections, err := h.profileRepo.GetBrokerageConnections(r.Context(), userID)
	if err != nil {
		log.Printf("portfolio handler: get connections: %v", err)
		http.Error(w, "failed to load brokerage connections", http.StatusInternalServerError)
		return
	}

	resp := portfolioResponse{Accounts: []portfolioAccount{}}

	for i := range connections {
		conn := &connections[i]
		provider, err := h.brokerageFactory.ForUser(conn)
		if err != nil {
			log.Printf("portfolio handler: build provider for %s: %v", conn.ID, err)
			continue
		}

		positions, err := provider.GetPositions(r.Context(), userID)
		if err != nil {
			log.Printf("portfolio handler: get positions from %s: %v", conn.ID, err)
			continue
		}

		account := portfolioAccount{
			BrokerageID:   conn.ID,
			BrokerageName: conn.Name,
			Positions:     toPortfolioPositions(positions),
		}
		for _, p := range positions {
			account.TotalValue += p.MarketValue
			account.TotalCost += p.CostBasis
			account.TotalUnrealizedPL += p.UnrealizedPL
		}

		resp.Accounts = append(resp.Accounts, account)
		resp.TotalValue += account.TotalValue
		resp.TotalCost += account.TotalCost
		resp.TotalUnrealizedPL += account.TotalUnrealizedPL
	}

	if resp.TotalCost > 0 {
		resp.TotalUnrealizedPLPct = (resp.TotalUnrealizedPL / resp.TotalCost) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type historyPointJSON struct {
	Timestamp     int64   `json:"timestamp"`
	Equity        float64 `json:"equity"`
	ProfitLoss    float64 `json:"profit_loss"`
	ProfitLossPct float64 `json:"profit_loss_pct"`
}

type portfolioHistoryResponse struct {
	Period    string              `json:"period"`
	Points    []historyPointJSON  `json:"points"`
	BaseValue float64             `json:"base_value"`
}

func (h *PortfolioHandler) getHistory(w http.ResponseWriter, r *http.Request) {
	userID, err := h.identity.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "1D"
	}
	params, ok := periodTimeframe[period]
	if !ok {
		http.Error(w, "invalid period; valid values: 1D 5D 1M 1Y 5Y", http.StatusBadRequest)
		return
	}
	alpacaPeriod, timeframe := params[0], params[1]

	connections, err := h.profileRepo.GetBrokerageConnections(r.Context(), userID)
	if err != nil {
		log.Printf("portfolio history: get connections: %v", err)
		http.Error(w, "failed to load brokerage connections", http.StatusInternalServerError)
		return
	}

	// Aggregate equity across all connections; timestamps align because same period/timeframe.
	var combined []models.HistoryPoint
	for i := range connections {
		provider, err := h.brokerageFactory.ForUser(&connections[i])
		if err != nil {
			log.Printf("portfolio history: build provider for %s: %v", connections[i].ID, err)
			continue
		}
		pts, err := provider.GetPortfolioHistory(r.Context(), userID, alpacaPeriod, timeframe)
		if err != nil {
			log.Printf("portfolio history: fetch from %s: %v", connections[i].ID, err)
			continue
		}
		if len(combined) == 0 {
			combined = pts
		} else {
			for j := range pts {
				if j < len(combined) {
					combined[j].Equity += pts[j].Equity
					combined[j].ProfitLoss += pts[j].ProfitLoss
				}
			}
		}
	}

	points := make([]historyPointJSON, len(combined))
	for i, p := range combined {
		points[i] = historyPointJSON{
			Timestamp:     p.Timestamp,
			Equity:        p.Equity,
			ProfitLoss:    p.ProfitLoss,
			ProfitLossPct: p.ProfitLossPct,
		}
	}

	var baseValue float64
	if len(points) > 0 {
		baseValue = points[0].Equity
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(portfolioHistoryResponse{
		Period:    period,
		Points:    points,
		BaseValue: baseValue,
	})
}

func toPortfolioPositions(positions []models.Position) []portfolioPosition {
	out := make([]portfolioPosition, len(positions))
	for i, p := range positions {
		out[i] = portfolioPosition{
			Ticker:              p.Ticker,
			Name:                p.Name,
			Quantity:            p.Quantity,
			MarketValue:         p.MarketValue,
			CostBasis:           p.CostBasis,
			AvgEntryPrice:       p.AvgEntryPrice,
			UnrealizedPL:        p.UnrealizedPL,
			UnrealizedPLPercent: p.UnrealizedPLPercent,
		}
	}
	return out
}
