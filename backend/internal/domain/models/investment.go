// domain/models/investment.go
package models

type InvestmentRequest struct {
	BaseBudget     float64         `json:"base_budget"`
	ExtraMoney     float64         `json:"extra_money"`
	BalanceSummary *BalanceSummary `json:"-"` // injected by recommendation service; never decoded from HTTP body
}

type Allocation struct {
	Ticker     string  `json:"ticker"`
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
}
