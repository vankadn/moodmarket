// domain/models/decision.go
package models

import "time"

// InvestmentDecision is the permanent record of one recommendation event.
// It captures both what the market looked like and what Claude decided,
// so the user can review their decision history with full context.
type InvestmentDecision struct {
	ID             string
	UserID         string
	Timestamp      time.Time
	MarketSnapshot *MarketSnapshot
	Allocations    []Allocation
	TotalAmount    float64
	RiskLevel      string
	Summary        string
}
