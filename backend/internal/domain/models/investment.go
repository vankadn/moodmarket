// domain/models/investment.go
package models

import "time"

// TickerContext captures the user's purchase history for a single ticker.
// Derived from InvestmentDecision.Receipts — zero new DB calls.
type TickerContext struct {
	AverageCostBasis float64   // average FilledPrice across all receipts for this ticker
	TotalInvested    float64   // sum of FilledAmount across all receipts
	PurchaseCount    int       // number of distinct receipts
	FirstPurchasedAt time.Time // timestamp of the earliest receipt
	MonthsHeld       int       // months since first purchase; <12 = short-term, >=12 = long-term
}

type InvestmentRequest struct {
	BaseBudget         float64              `json:"base_budget"`
	ExtraMoney         float64              `json:"extra_money"`
	BalanceSummary     *BalanceSummary      `json:"-"` // injected by recommendation service; never decoded from HTTP body
	TransactionSummary *TransactionSummary  `json:"-"` // recent spending signals; injected by recommendation service
	Positions          []Position           `json:"-"` // current brokerage holdings; injected by recommendation service
	RecentDecisions        []InvestmentDecision `json:"-"` // last N decisions; injected by recommendation service
	RecentBlockedDecisions []InvestmentDecision `json:"-"` // last 3 blocked decisions; injected by recommendation service
	NewsItems              []NewsItem           `json:"-"` // today's market headlines; injected by recommendation service
	TaxDocuments           []*TaxDocument       `json:"-"` // verified tax docs (W2/1099/1098); injected by recommendation service
	StrategyPrompt         string               `json:"-"` // optional; prepended to Claude system prompt when set
	PerformanceSummary     *EvalSummary         `json:"-"` // injected by recommendation service; nil for new users or when below verdict threshold
	// Agentic budget fields — set by scheduler when Mode == "agentic"; zero values mean fixed mode.
	AgenticMode bool    `json:"-"`
	DailyBudget float64 `json:"-"`
	SpentToday  float64 `json:"-"`
	Remaining   float64 `json:"-"`
	// Rebalance intelligence — both injected by recommendation service; nil/empty when no analysis exists.
	RebalanceAnalysis *RebalanceAnalysis      `json:"-"` // latest saved rebalance analysis; nil for new users
	PositionContext   map[string]TickerContext `json:"-"` // per-ticker purchase history derived from recent decisions
}

type Allocation struct {
	Ticker     string  `json:"ticker"`
	AssetClass string  `json:"asset_class,omitempty"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
	Rationale  string  `json:"rationale"`
	Reasoning  string  `json:"reasoning,omitempty"` // one-sentence per-ticker reasoning from Claude; omitempty — old allocations unaffected
}

type Recommendation struct {
	TotalBudget      float64      `json:"total_budget"`
	Allocations      []Allocation `json:"allocations"`
	Summary          string       `json:"summary"`
	RiskLevel        string       `json:"risk_level"`
	SkipReason       string       `json:"skip_reason,omitempty"`        // set by Claude when total_budget == 0 in agentic mode
	OverallReasoning string       `json:"overall_reasoning,omitempty"` // 1-2 sentence investment thesis from Claude
	FromCache        bool         `json:"from_cache,omitempty"`
}
