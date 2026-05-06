// domain/models/market.go
package models

// TickerSnapshot is a lightweight value object representing one instrument's
// price state at a point in time. Used for both individual tickers and sectors.
type TickerSnapshot struct {
	Symbol        string  `json:"symbol"`
	ChangePercent float64 `json:"change_percent"`
	Price         float64 `json:"price"`
}

// MarketSnapshot captures the macro context for a single trading day.
// All fields are optional in the sense that any sub-fetch failure returns nil
// from the provider, and the advisor builds its prompt defensively.
type MarketSnapshot struct {
	Date              string             `json:"date"`
	SPYChangePercent  float64            `json:"spy_change_percent"`
	QQQChangePercent  float64            `json:"qqq_change_percent"`
	SectorPerformance map[string]float64 `json:"sector_performance"`
	MarketSentiment   string             `json:"market_sentiment"`
	TopMovers         []TickerSnapshot   `json:"top_movers"`
}
