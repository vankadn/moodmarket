// domain/models/rebalance_analysis.go
package models

import "time"

// RebalancePosition wraps a Position with its data source for portfolio analysis.
// Source is the provider name ("alpaca" | "snaptrade").
// AccountName is the human-readable brokerage name — empty for Alpaca (implicit),
// set to the institution name (e.g. "Robinhood", "Fidelity") for SnapTrade positions.
type RebalancePosition struct {
	Position
	Source      string // "alpaca" | "snaptrade"
	AccountName string // e.g. "Robinhood", "Fidelity"; empty for Alpaca
}

// RebalanceRequest is the input to a rebalance analysis.
// Assembled by the aggregation service before being passed to RebalanceAdvisor.
type RebalanceRequest struct {
	Positions             []RebalancePosition
	BuyReasoningByTicker  map[string]string    // keyed by ticker; from InvestmentDecision.TickerReasoning
	FirstPurchaseByTicker map[string]time.Time // estimated acquisition date per ticker; used for tax flag computation
}

// SuggestedAction is Claude's recommendation for a position.
type SuggestedAction string

const (
	ActionHold       SuggestedAction = "hold"
	ActionAdd        SuggestedAction = "add"        // underweight or strong thesis — consider buying more
	ActionTrim       SuggestedAction = "trim"
	ActionReconsider SuggestedAction = "reconsider"
)

// TaxFlag indicates the likely capital gains treatment if the position were sold.
type TaxFlag string

const (
	// TaxFlagShortTerm means the position has been held < 1 year — higher tax cost if sold.
	TaxFlagShortTerm TaxFlag = "short_term"
	// TaxFlagLongTerm means the position has been held >= 1 year — lower tax cost if sold.
	TaxFlagLongTerm TaxFlag = "long_term"
	// TaxFlagUnknown means no acquisition date is available to determine treatment.
	TaxFlagUnknown TaxFlag = "unknown"
)

// PositionInsight is Claude's assessment of a single holding.
type PositionInsight struct {
	Ticker            string
	Name              string
	Source            string          // "alpaca" | "snaptrade"
	AccountName       string          // e.g. "Robinhood", "Fidelity"; empty for Alpaca
	CurrentValue      float64
	UnrealizedPL      float64
	UnrealizedPLPct   float64
	OriginalBuyThesis string          // from InvestmentDecision.TickerReasoning; empty if not previously recommended by InvestIQ
	ClaudeAssessment  string          // Claude's narrative analysis of this position
	SuggestedAction   SuggestedAction // "hold" | "trim" | "reconsider"
	TaxFlag           TaxFlag         // "short_term" | "long_term" | "unknown"
}

// RebalanceAnalysis is Claude's complete portfolio assessment.
type RebalanceAnalysis struct {
	Insights               []PositionInsight
	PortfolioHealthSummary string
	GeneratedAt            time.Time
}
