// api/handlers/eval_handler.go
package handlers

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// safeFloat replaces NaN and Inf with 0 so json.Encoder never fails.
// These can appear in MongoDB when a previous verdict was stamped with a division-by-zero bug.
func safeFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return f
}

type EvalHandler struct {
	identityProvider ports.IdentityProvider
	decisionRepo     ports.DecisionRepository
}

func NewEvalHandler(identityProvider ports.IdentityProvider, decisionRepo ports.DecisionRepository) *EvalHandler {
	return &EvalHandler{identityProvider: identityProvider, decisionRepo: decisionRepo}
}

// evalDecisionRef is the JSON-safe version of models.EvalDecisionRef.
type evalDecisionRef struct {
	ID        string    `json:"id"`
	Date      time.Time `json:"date"`
	ReturnPct float64   `json:"return_pct"`
	Amount    float64   `json:"amount"`
}

type strategyEvalItem struct {
	ConfigID      string  `json:"config_id"`
	WinRate       float64 `json:"win_rate"`
	AvgReturnPct  float64 `json:"avg_return_pct"`
	DecisionCount int     `json:"decision_count"`
}

type evalSummaryResponse struct {
	TotalDecisions     int               `json:"total_decisions"`
	VerdictedDecisions int               `json:"verdicted_decisions"`
	WinRate            float64           `json:"win_rate"`
	AvgReturnPct       float64           `json:"avg_return_pct"`
	AvgSPYReturnPct    float64           `json:"avg_spy_return_pct"`
	BestDecision       *evalDecisionRef  `json:"best_decision,omitempty"`
	WorstDecision      *evalDecisionRef  `json:"worst_decision,omitempty"`
	ByStrategy         []strategyEvalItem `json:"by_strategy"`
}

// ServeHTTP dispatches between /users/eval/summary and /users/eval/decisions.
func (h *EvalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := h.identityProvider.GetCurrentUser(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch r.URL.Path {
	case "/users/eval/summary":
		h.getSummary(w, r, userID)
	case "/users/eval/decisions":
		h.getDecisions(w, r, userID)
	default:
		http.NotFound(w, r)
	}
}

func (h *EvalHandler) getSummary(w http.ResponseWriter, r *http.Request, userID string) {
	summary, err := h.decisionRepo.GetEvalSummary(r.Context(), userID)
	if err != nil {
		log.Printf("[eval] summary: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := evalSummaryResponse{
		TotalDecisions:     summary.TotalDecisions,
		VerdictedDecisions: summary.VerdictedDecisions,
		WinRate:            safeFloat(summary.WinRate),
		AvgReturnPct:       safeFloat(summary.AvgReturnPct),
		AvgSPYReturnPct:    safeFloat(summary.AvgSPYReturnPct),
		ByStrategy:         make([]strategyEvalItem, len(summary.ByStrategy)),
	}
	for i, s := range summary.ByStrategy {
		resp.ByStrategy[i] = strategyEvalItem{
			ConfigID:      s.ConfigID,
			WinRate:       safeFloat(s.WinRate),
			AvgReturnPct:  safeFloat(s.AvgReturnPct),
			DecisionCount: s.DecisionCount,
		}
	}
	if summary.BestDecision != nil {
		resp.BestDecision = &evalDecisionRef{
			ID:        summary.BestDecision.ID,
			Date:      summary.BestDecision.Date,
			ReturnPct: safeFloat(summary.BestDecision.ReturnPct),
			Amount:    summary.BestDecision.Amount,
		}
	}
	if summary.WorstDecision != nil {
		resp.WorstDecision = &evalDecisionRef{
			ID:        summary.WorstDecision.ID,
			Date:      summary.WorstDecision.Date,
			ReturnPct: safeFloat(summary.WorstDecision.ReturnPct),
			Amount:    summary.WorstDecision.Amount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type evalDecisionItem struct {
	ID               string               `json:"id"`
	Timestamp        time.Time            `json:"timestamp"`
	TotalAmount      float64              `json:"total_amount"`
	RiskLevel        string               `json:"risk_level"`
	ConfigID         string               `json:"config_id,omitempty"`
	Summary          string               `json:"summary,omitempty"`
	Verdict          *verdictResponse     `json:"verdict"`
	DecisionType     string               `json:"decision_type,omitempty"`
	BlockedReason    string               `json:"blocked_reason,omitempty"`
	OverallReasoning string               `json:"overall_reasoning,omitempty"`
	TickerReasoning  map[string]string    `json:"ticker_reasoning,omitempty"`
	CriticReview     *models.CriticReview `json:"critic_review,omitempty"`
	Allocations      []models.Allocation  `json:"allocations,omitempty"`
}

type verdictResponse struct {
	StampedAt        time.Time             `json:"stamped_at"`
	OverallReturnPct float64               `json:"overall_return_pct"`
	SPYReturnPct     float64               `json:"spy_return_pct"`
	BeatMarket       bool                  `json:"beat_market"`
	TickerVerdicts   []tickerVerdictResponse `json:"ticker_verdicts"`
}

type tickerVerdictResponse struct {
	Ticker           string    `json:"ticker"`
	EntryPrice       float64   `json:"entry_price"`
	PrevDayPrice     float64   `json:"prev_day_price"`
	PrevDayTimestamp time.Time `json:"prev_day_timestamp"`
	CurrentPrice     float64   `json:"current_price"`
	CurrentTimestamp time.Time `json:"current_timestamp"`
	ReturnPct        float64   `json:"return_pct"`
	TodayChangePct   float64   `json:"today_change_pct"`
}

func (h *EvalHandler) getDecisions(w http.ResponseWriter, r *http.Request, userID string) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	decisions, err := h.decisionRepo.ListDecisions(r.Context(), userID, page, limit)
	if err != nil {
		log.Printf("[eval] decisions: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	items := make([]evalDecisionItem, len(decisions))
	for i, d := range decisions {
		item := evalDecisionItem{
			ID:               d.ID,
			Timestamp:        d.Timestamp,
			TotalAmount:      d.TotalAmount,
			RiskLevel:        d.RiskLevel,
			ConfigID:         d.ConfigID,
			Summary:          d.Summary,
			DecisionType:     d.DecisionType,
			BlockedReason:    d.BlockedReason,
			OverallReasoning: d.OverallReasoning,
			TickerReasoning:  d.TickerReasoning,
			CriticReview:     d.CriticReview,
			Allocations:      d.Allocations,
		}
		if d.Verdict != nil {
			tvs := make([]tickerVerdictResponse, len(d.Verdict.TickerVerdicts))
			for j, tv := range d.Verdict.TickerVerdicts {
				tvs[j] = tickerVerdictResponse{
					Ticker:           tv.Ticker,
					EntryPrice:       safeFloat(tv.EntryPrice),
					PrevDayPrice:     safeFloat(tv.PrevDayPrice),
					PrevDayTimestamp: tv.PrevDayTimestamp,
					CurrentPrice:     safeFloat(tv.CurrentPrice),
					CurrentTimestamp: tv.CurrentTimestamp,
					ReturnPct:        safeFloat(tv.ReturnPct),
					TodayChangePct:   safeFloat(tv.TodayChangePct),
				}
			}
			item.Verdict = &verdictResponse{
				StampedAt:        d.Verdict.StampedAt,
				OverallReturnPct: safeFloat(d.Verdict.OverallReturnPct),
				SPYReturnPct:     safeFloat(d.Verdict.SPYReturnPct),
				BeatMarket:       d.Verdict.BeatMarket,
				TickerVerdicts:   tvs,
			}
		}
		items[i] = item
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
