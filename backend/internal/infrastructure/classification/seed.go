package classification

import (
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// SeedEntries returns the canonical list of tickers to upsert into ticker_classifications.
// All entries are approved:true, suggested_by_claude:false.
// Mongo is the only source of truth — never hardcode this map elsewhere.
func SeedEntries() []models.ClassificationEntry {
	now := time.Now()
	type kv struct {
		ticker string
		class  string
	}
	seeds := []kv{
		// US Equity
		{"VTI", "US Equity"}, {"SPY", "US Equity"}, {"QQQ", "US Equity"},
		{"XLK", "US Equity"}, {"XLF", "US Equity"}, {"XLE", "US Equity"},
		{"XLV", "US Equity"}, {"XLI", "US Equity"}, {"AAPL", "US Equity"},
		{"MSFT", "US Equity"}, {"NVDA", "US Equity"}, {"AMZN", "US Equity"},
		{"TSLA", "US Equity"},
		// International
		{"VXUS", "International"}, {"EFA", "International"}, {"EEM", "International"},
		{"VEA", "International"}, {"VWO", "International"},
		// Bonds
		{"BND", "Bonds"}, {"AGG", "Bonds"}, {"SGOV", "Bonds"},
		{"SHV", "Bonds"}, {"TLT", "Bonds"}, {"IEF", "Bonds"},
		// Real Estate
		{"VNQ", "Real Estate"}, {"SCHH", "Real Estate"},
		// Commodities
		{"GLD", "Commodities"}, {"SLV", "Commodities"}, {"USO", "Commodities"},
	}
	entries := make([]models.ClassificationEntry, len(seeds))
	for i, s := range seeds {
		entries[i] = models.ClassificationEntry{
			Ticker:            s.ticker,
			AssetClass:        s.class,
			Approved:          true,
			SuggestedByClaude: false,
			FirstSeenAt:       now,
		}
	}
	return entries
}
