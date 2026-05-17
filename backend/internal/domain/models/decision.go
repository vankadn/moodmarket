// domain/models/decision.go
package models

import "time"

// StrategyActivity is the aggregated performance summary for one config_id group.
// ConfigID is the AutoInvestConfig.ID, "manual" for user-initiated, or "" for legacy docs.
type StrategyActivity struct {
	ConfigID      string
	TotalInvested float64
	DecisionCount int
	FirstRunAt    time.Time
	LastRunAt     time.Time
}

// InvestmentDecision is the permanent record of one invest event.
// It captures what the market looked like, what Claude recommended,
// and — when trades were placed — what the brokerage returned for each order.
type InvestmentDecision struct {
	ID             string
	UserID         string
	ConfigID       string // auto-invest config that triggered this; "manual" for user-initiated; "" for legacy docs
	Timestamp      time.Time
	MarketSnapshot *MarketSnapshot
	Allocations    []Allocation
	Receipts       []TradeReceipt // populated only when trades were actually placed
	TotalAmount    float64
	RiskLevel      string
	Summary        string
}
