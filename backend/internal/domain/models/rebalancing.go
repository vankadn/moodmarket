// domain/models/rebalancing.go
package models

// TickerDrift describes how far one ticker has moved from its target allocation weight.
type TickerDrift struct {
	Ticker    string
	TargetPct float64 // weight in last Claude recommendation (0–100)
	ActualPct float64 // current portfolio weight by market value (0–100)
	DriftPct  float64 // ActualPct - TargetPct; positive = overweight, negative = underweight
}
