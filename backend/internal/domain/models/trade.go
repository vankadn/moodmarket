// domain/models/trade.go
package models

import "time"

// Position is a holding the user currently owns in their brokerage account.
type Position struct {
	Ticker              string
	Name                string
	Quantity            float64
	MarketValue         float64
	CostBasis           float64
	AvgEntryPrice       float64
	UnrealizedPL        float64
	UnrealizedPLPercent float64
}

// HistoryPoint is one data point in a portfolio value time series.
type HistoryPoint struct {
	Timestamp     int64   // Unix epoch seconds
	Equity        float64
	ProfitLoss    float64
	ProfitLossPct float64
}

// TradeOrder is the instruction sent to a brokerage: buy this ticker for this dollar amount.
// Amount is notional (dollars), not shares — the brokerage calculates fractional shares.
type TradeOrder struct {
	UserID string
	Ticker string
	Amount float64
}

// TradeReceipt is the brokerage's acknowledgement of a placed order.
// FilledAmount and FilledPrice may be zero if the order is pending settlement.
type TradeReceipt struct {
	OrderID       string    `json:"order_id"`
	Ticker        string    `json:"ticker"`
	FilledAmount  float64   `json:"filled_amount"`
	FilledPrice   float64   `json:"filled_price"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
	BrokerageID   string    `json:"brokerage_id,omitempty"`   // connection that executed this order
	BrokerageName string    `json:"brokerage_name,omitempty"` // display name of that connection
}
