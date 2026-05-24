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
	DecisionType   string // "invest" | "skip"; empty for legacy records
	SkipReason     string // Claude's one-sentence reason when DecisionType=="skip"; empty otherwise
	Verdict        *DecisionVerdict // nil until verdict_job stamps it (Phase 22)
}

// EvalSummary is the aggregate performance summary for a user's verdicted decisions.
// Returned by GET /users/eval/summary.
type EvalSummary struct {
	TotalDecisions     int
	VerdictedDecisions int
	WinRate            float64 // fraction 0.0–1.0: verdicted decisions that beat SPY
	AvgReturnPct       float64
	AvgSPYReturnPct    float64
	BestDecision       *EvalDecisionRef
	WorstDecision      *EvalDecisionRef
	ByStrategy         []StrategyEval
}

// EvalDecisionRef is a lightweight reference to a single decision used in best/worst fields.
type EvalDecisionRef struct {
	ID        string
	Date      time.Time
	ReturnPct float64
	Amount    float64
}

// StrategyEval is the per-strategy performance breakdown within EvalSummary.
type StrategyEval struct {
	ConfigID      string
	WinRate       float64
	AvgReturnPct  float64
	DecisionCount int
}

// DecisionVerdict is stamped once on each InvestmentDecision after a 24h minimum age.
// Append-only — never overwritten once set.
type DecisionVerdict struct {
	StampedAt        time.Time
	OverallReturnPct float64 // weighted-avg return (entry→current) across all tickers
	SPYReturnPct     float64 // SPY return over same holding period; 0 when SPYPrice not stored at decision time
	BeatMarket       bool
	TickerVerdicts   []TickerVerdict
}

// TickerVerdict holds two price snapshots per ticker: Polygon's prev-day close and
// Alpaca's real-time quote — giving both a settled baseline and a live reading.
type TickerVerdict struct {
	Ticker           string
	EntryPrice       float64   // TradeReceipt.FilledPrice — exact fill at trade time
	PrevDayPrice     float64   // Polygon prev-day close at verdict stamp time
	PrevDayTimestamp time.Time // date of PrevDayPrice
	CurrentPrice     float64   // Alpaca real-time quote at verdict stamp time; 0 if unavailable
	CurrentTimestamp time.Time // when Alpaca was queried
	ReturnPct        float64   // (CurrentPrice - EntryPrice) / EntryPrice * 100; 0 if CurrentPrice == 0
	TodayChangePct   float64   // (CurrentPrice - PrevDayPrice) / PrevDayPrice * 100; 0 if either == 0
}
