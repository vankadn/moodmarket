// domain/models/decision.go
package models

import "time"

// InvestmentDecision is the permanent record of one invest event.
// It captures what the market looked like, what Claude recommended,
// and — when trades were placed — what the brokerage returned for each order.
type InvestmentDecision struct {
	ID             string
	UserID         string
	Timestamp      time.Time
	MarketSnapshot *MarketSnapshot
	Allocations    []Allocation
	Receipts       []TradeReceipt // populated only when trades were actually placed
	TotalAmount    float64
	RiskLevel      string
	Summary        string
}
