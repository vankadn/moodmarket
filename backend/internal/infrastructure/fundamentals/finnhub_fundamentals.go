// infrastructure/fundamentals/finnhub_fundamentals.go
package fundamentals

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const finnhubFundamentalsBaseURL = "https://finnhub.io/api/v1"

type finnhubFundamentalsProvider struct {
	apiKey     string
	httpClient *http.Client
}

func newFinnhubFundamentalsProvider(apiKey string) *finnhubFundamentalsProvider {
	return &finnhubFundamentalsProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (f *finnhubFundamentalsProvider) get(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API %d: %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return nil
}

// mFloat pulls a float64 from the metric map; returns 0 if the key is absent or the wrong type.
func mFloat(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return f
}

// mString pulls a string from the metric map; returns "" if the key is absent or the wrong type.
func mString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// GetFundamentals calls GET /stock/metric?symbol={ticker}&metric=all.
// The metric object is decoded into map[string]interface{} because several
// Finnhub keys contain literal '/' characters (e.g. "longTermDebt/equityQuarterly")
// which cannot be expressed as Go struct json tags inside a typed inner struct.
func (f *finnhubFundamentalsProvider) GetFundamentals(ctx context.Context, ticker string) (*models.Fundamentals, error) {
	url := fmt.Sprintf("%s/stock/metric?symbol=%s&metric=all&token=%s", finnhubFundamentalsBaseURL, ticker, f.apiKey)
	log.Printf("[finnhub-fundamentals] GET metric %s", ticker)

	var result struct {
		Metric map[string]interface{} `json:"metric"`
	}
	if err := f.get(ctx, url, &result); err != nil {
		return nil, fmt.Errorf("finnhub-fundamentals GetFundamentals %s: %w", ticker, err)
	}

	m := result.Metric
	out := &models.Fundamentals{
		Ticker:               ticker,
		PE:                   mFloat(m, "peTTM"),
		ForwardPE:            mFloat(m, "forwardPE"),
		ForwardPEG:           mFloat(m, "forwardPEG"),
		FiftyTwoWeekHigh:     mFloat(m, "52WeekHigh"),
		FiftyTwoWeekHighDate: mString(m, "52WeekHighDate"),
		FiftyTwoWeekLow:      mFloat(m, "52WeekLow"),
		FiftyTwoWeekLowDate:  mString(m, "52WeekLowDate"),
		DebtToEquity:         mFloat(m, "longTermDebt/equityQuarterly"),
		TotalDebtToEquity:    mFloat(m, "totalDebt/totalEquityQuarterly"),
		CurrentRatio:         mFloat(m, "currentRatioQuarterly"),
	}

	log.Printf("[finnhub-fundamentals] %s pe=%.2f fwdPE=%.2f fwdPEG=%.2f 52wkH=%.2f 52wkL=%.2f ltD/E=%.2f totD/E=%.2f cr=%.2f",
		ticker, out.PE, out.ForwardPE, out.ForwardPEG,
		out.FiftyTwoWeekHigh, out.FiftyTwoWeekLow,
		out.DebtToEquity, out.TotalDebtToEquity, out.CurrentRatio)
	return out, nil
}

// GetEarningsSurprises calls GET /stock/earnings?symbol={ticker}.
// Finnhub returns a flat JSON array (not wrapped), most recent quarter first.
func (f *finnhubFundamentalsProvider) GetEarningsSurprises(ctx context.Context, ticker string, limit int) ([]models.EarningsSurprise, error) {
	url := fmt.Sprintf("%s/stock/earnings?symbol=%s&token=%s", finnhubFundamentalsBaseURL, ticker, f.apiKey)
	log.Printf("[finnhub-fundamentals] GET earnings %s limit=%d", ticker, limit)

	var raw []struct {
		Period          string  `json:"period"`
		Actual          float64 `json:"actual"`
		Estimate        float64 `json:"estimate"`
		SurprisePercent float64 `json:"surprisePercent"`
	}
	if err := f.get(ctx, url, &raw); err != nil {
		return nil, fmt.Errorf("finnhub-fundamentals GetEarningsSurprises %s: %w", ticker, err)
	}

	if limit > 0 && len(raw) > limit {
		raw = raw[:limit]
	}

	out := make([]models.EarningsSurprise, len(raw))
	for i, r := range raw {
		out[i] = models.EarningsSurprise{
			Period:      r.Period,
			ActualEPS:   r.Actual,
			EstimateEPS: r.Estimate,
			SurprisePct: r.SurprisePercent,
		}
	}
	log.Printf("[finnhub-fundamentals] %s earnings: %d quarters returned", ticker, len(out))
	return out, nil
}

// GetInsiderActivity calls two Finnhub endpoints:
//  1. GET /stock/insider-sentiment — 14-month window for monthly MSPR display data
//  2. GET /stock/insider-transactions — raw Form-4 transactions to identify genuine purchases
//
// ConsecutiveNegativeMonths counts calendar months since the last month containing a
// genuine open-market purchase (SEC code "P", price > $0). MSPR sign alone is not used
// because code-A grants at $0 produce positive MSPR without representing insider conviction.
func (f *finnhubFundamentalsProvider) GetInsiderActivity(ctx context.Context, ticker string) (*models.InsiderActivity, error) {
	now := time.Now()
	to := now.Format("2006-01-02")
	from := now.AddDate(0, -14, 0).Format("2006-01-02")
	sentimentURL := fmt.Sprintf("%s/stock/insider-sentiment?symbol=%s&from=%s&to=%s&token=%s",
		finnhubFundamentalsBaseURL, ticker, from, to, f.apiKey)
	log.Printf("[finnhub-fundamentals] GET insider-sentiment %s (%s→%s)", ticker, from, to)

	var sentimentResult struct {
		Data []struct {
			Year   int     `json:"year"`
			Month  int     `json:"month"`
			Change int     `json:"change"`
			MSPR   float64 `json:"mspr"`
		} `json:"data"`
	}
	if err := f.get(ctx, sentimentURL, &sentimentResult); err != nil {
		return nil, fmt.Errorf("finnhub-fundamentals GetInsiderActivity %s: %w", ticker, err)
	}

	months := make([]models.InsiderMonth, len(sentimentResult.Data))
	for i, d := range sentimentResult.Data {
		months[i] = models.InsiderMonth{
			Year:   d.Year,
			Month:  d.Month,
			Change: d.Change,
			MSPR:   d.MSPR,
		}
	}
	sort.Slice(months, func(i, j int) bool {
		if months[i].Year != months[j].Year {
			return months[i].Year > months[j].Year
		}
		return months[i].Month > months[j].Month
	})

	// Fetch raw transactions to determine which months contain genuine code-P purchases.
	txURL := fmt.Sprintf("%s/stock/insider-transactions?symbol=%s&token=%s",
		finnhubFundamentalsBaseURL, ticker, f.apiKey)
	log.Printf("[finnhub-fundamentals] GET insider-transactions %s", ticker)

	var txResult struct {
		Data []struct {
			TransactionCode  string  `json:"transactionCode"`
			TransactionPrice float64 `json:"transactionPrice"`
			TransactionDate  string  `json:"transactionDate"` // "YYYY-MM-DD"
		} `json:"data"`
	}
	if err := f.get(ctx, txURL, &txResult); err != nil {
		// Non-fatal: fall back to MSPR-based consecutive count so the sentiment data
		// is still usable even when the transactions endpoint is unavailable.
		log.Printf("[finnhub-fundamentals] %s insider-transactions unavailable (%v) — using MSPR sign fallback", ticker, err)
		consecutive := 0
		for _, m := range months {
			if m.MSPR < 0 {
				consecutive++
			} else {
				break
			}
		}
		return &models.InsiderActivity{
			Ticker:                    ticker,
			RecentMonths:              months,
			ConsecutiveNegativeMonths: consecutive,
		}, nil
	}

	// Build a set of months that contain at least one genuine open-market purchase:
	// transactionCode == "P" (open-market buy) AND transactionPrice > 0 (excludes grants).
	// Key is (year, month); value is the most-recent transaction date in that month,
	// used to populate LastGenuinePurchaseDate.
	type monthKey struct{ year, month int }
	genuineMonths := make(map[monthKey]string)
	lastGenuineDate := ""
	windowStart := now.AddDate(0, -14, 0)

	for _, tx := range txResult.Data {
		if tx.TransactionCode != "P" || tx.TransactionPrice <= 0 {
			continue
		}
		t, err := time.Parse("2006-01-02", tx.TransactionDate)
		if err != nil || t.Before(windowStart) {
			continue
		}
		key := monthKey{t.Year(), int(t.Month())}
		if existing, ok := genuineMonths[key]; !ok || tx.TransactionDate > existing {
			genuineMonths[key] = tx.TransactionDate
		}
		if lastGenuineDate == "" || tx.TransactionDate > lastGenuineDate {
			lastGenuineDate = tx.TransactionDate
		}
	}

	// Count consecutive months (from most recent) that had no genuine code-P purchase.
	// A month with MSPR > 0 due solely to grants does NOT break the streak.
	consecutive := 0
	for _, m := range months {
		if _, hasPurchase := genuineMonths[monthKey{m.Year, m.Month}]; hasPurchase {
			break
		}
		consecutive++
	}

	log.Printf("[finnhub-fundamentals] %s insider: %d months, %d genuine-P months in window, consecutiveNeg=%d lastBuy=%q",
		ticker, len(months), len(genuineMonths), consecutive, lastGenuineDate)
	return &models.InsiderActivity{
		Ticker:                    ticker,
		RecentMonths:              months,
		ConsecutiveNegativeMonths: consecutive,
		LastGenuinePurchaseDate:   lastGenuineDate,
	}, nil
}

// compile-time check that finnhubFundamentalsProvider satisfies FundamentalsProvider.
var _ interface {
	GetFundamentals(context.Context, string) (*models.Fundamentals, error)
	GetEarningsSurprises(context.Context, string, int) ([]models.EarningsSurprise, error)
	GetInsiderActivity(context.Context, string) (*models.InsiderActivity, error)
} = (*finnhubFundamentalsProvider)(nil)
