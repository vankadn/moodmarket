// domain/models/investment.go
package models

type InvestmentRequest struct {
	BaseBudget         float64              `json:"base_budget"`
	ExtraMoney         float64              `json:"extra_money"`
	BalanceSummary     *BalanceSummary      `json:"-"` // injected by recommendation service; never decoded from HTTP body
	TransactionSummary *TransactionSummary  `json:"-"` // recent spending signals; injected by recommendation service
	Positions          []Position           `json:"-"` // current brokerage holdings; injected by recommendation service
	RecentDecisions    []InvestmentDecision `json:"-"` // last N decisions; injected by recommendation service
	NewsItems          []NewsItem           `json:"-"` // today's market headlines; injected by recommendation service
	TaxDocuments       []*TaxDocument       `json:"-"` // verified tax docs (W2/1099/1098); injected by recommendation service
	StrategyPrompt     string               `json:"-"` // optional; prepended to Claude system prompt when set
	PerformanceSummary *EvalSummary         `json:"-"` // injected by recommendation service; nil for new users or when below verdict threshold
}

type Allocation struct {
	Ticker     string  `json:"ticker"`
	AssetClass string  `json:"asset_class,omitempty"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
	Rationale  string  `json:"rationale"`
}

type Recommendation struct {
	TotalBudget float64      `json:"total_budget"`
	Allocations []Allocation `json:"allocations"`
	Summary     string       `json:"summary"`
	RiskLevel   string       `json:"risk_level"`
	FromCache   bool         `json:"from_cache,omitempty"`
}
