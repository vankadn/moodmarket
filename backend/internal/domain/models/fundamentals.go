// domain/models/fundamentals.go
package models

// Fundamentals holds key value metrics for a single ticker sourced from Finnhub /stock/metric.
// All fields zero-default safely — a missing Finnhub key returns 0/"" rather than an error.
type Fundamentals struct {
	Ticker               string
	PE                   float64 // metric.peTTM
	ForwardPE            float64 // metric.forwardPE
	ForwardPEG           float64 // metric.forwardPEG
	FiftyTwoWeekHigh     float64 // metric.52WeekHigh
	FiftyTwoWeekHighDate string  // metric.52WeekHighDate
	FiftyTwoWeekLow      float64 // metric.52WeekLow
	FiftyTwoWeekLowDate  string  // metric.52WeekLowDate
	DebtToEquity         float64 // metric["longTermDebt/equityQuarterly"]   — slash is literal in Finnhub key
	TotalDebtToEquity    float64 // metric["totalDebt/totalEquityQuarterly"] — slash is literal in Finnhub key
	CurrentRatio         float64 // metric.currentRatioQuarterly
	// Valuation depth — sourced from the same /stock/metric?metric=all response, zero extra API calls.
	EVToEBITDA        float64 // metric.evEbitdaTTM — enterprise value / EBITDA trailing twelve months
	FCFYieldPct       float64 // 1/metric.pfcfTTM * 100; 0 when pfcfTTM ≤ 0 (negative or missing FCF)
	PriceToBook       float64 // metric.pb
	PEVsOwnFiveYearAvg  float64 // peTTM / avg(series.annual.pe, trailing 5 non-null entries); 0 if < 3 entries
	EVEBITDAVsOwnAvg  float64 // evEbitdaTTM / avg(series.annual.evEbitda, trailing 5 non-null entries); 0 if < 3 entries
}

// EarningsSurprise holds one quarterly EPS beat/miss from Finnhub /stock/earnings.
// SurprisePct is already in percentage units (e.g. 5.15 = 5.15%).
type EarningsSurprise struct {
	Period      string
	ActualEPS   float64
	EstimateEPS float64
	SurprisePct float64
}

// InsiderMonth holds one calendar month of Finnhub insider-sentiment data.
type InsiderMonth struct {
	Year   int
	Month  int
	Change int     // net share change that month (buys minus sells)
	MSPR   float64 // Monthly Share Purchase Ratio: -100 to 100
}

// InsiderActivity holds the recent insider-sentiment window for a ticker.
// RecentMonths is sorted most-recent-first (from /stock/insider-sentiment, kept for context).
// ConsecutiveNegativeMonths counts calendar months since the last month containing a
// genuine open-market purchase (SEC code "P", price > $0). Grants (code "A"),
// tax-withholding sales (code "F"), gifts (code "G"), and option exercises (code "M")
// do not count as buying signal regardless of MSPR sign that month.
// LastGenuinePurchaseDate is the most recent transaction date with code "P" and price > 0
// within the lookback window; "" if none found.
type InsiderActivity struct {
	Ticker                    string
	RecentMonths              []InsiderMonth
	ConsecutiveNegativeMonths int
	LastGenuinePurchaseDate   string
}
