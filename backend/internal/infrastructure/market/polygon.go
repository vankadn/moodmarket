// infrastructure/market/polygon.go
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const polygonBaseURL = "https://api.polygon.io"

// sectorETFs maps human-readable sector names to their representative ETF tickers.
var sectorETFs = map[string]string{
	"Technology":  "QQQ",
	"Energy":      "XLE",
	"Financials":  "XLF",
	"Healthcare":  "XLV",
	"Industrials": "XLI",
}

type polygonProvider struct {
	apiKey     string
	httpClient *http.Client

	mu             sync.Mutex
	cachedSnapshot *models.MarketSnapshot
	cacheDate      string
}

func newPolygonProvider(apiKey string) *polygonProvider {
	return &polygonProvider{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetDailySnapshot returns the cached snapshot if it was fetched today,
// otherwise fetches fresh data from Polygon and updates the cache.
func (p *polygonProvider) GetDailySnapshot(ctx context.Context) (*models.MarketSnapshot, error) {
	today := time.Now().Format("2006-01-02")

	p.mu.Lock()
	if p.cacheDate == today && p.cachedSnapshot != nil {
		snapshot := p.cachedSnapshot
		p.mu.Unlock()
		log.Printf("[polygon] cache hit for %s", today)
		return snapshot, nil
	}
	p.mu.Unlock()

	snapshot, err := p.fetch(ctx)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	p.cachedSnapshot = snapshot
	p.cacheDate = today
	p.mu.Unlock()

	log.Printf("market snapshot fetched from Polygon")
	return snapshot, nil
}

// fetch makes the Polygon API calls and assembles a MarketSnapshot.
func (p *polygonProvider) fetch(ctx context.Context) (*models.MarketSnapshot, error) {
	spyPct, spyClose, err := p.fetchPrevDay(ctx, "SPY")
	if err != nil {
		return nil, fmt.Errorf("polygon: fetch SPY: %w", err)
	}

	qqqPct, _, err := p.fetchPrevDay(ctx, "QQQ")
	if err != nil {
		return nil, fmt.Errorf("polygon: fetch QQQ: %w", err)
	}

	sectors, err := p.fetchSectorPerformance(ctx)
	if err != nil {
		return nil, fmt.Errorf("polygon: fetch sectors: %w", err)
	}

	return &models.MarketSnapshot{
		Date:              time.Now().Format("2006-01-02"),
		SPYPrice:          spyClose,
		SPYChangePercent:  spyPct,
		QQQChangePercent:  qqqPct,
		SectorPerformance: sectors,
		MarketSentiment:   deriveSentiment(spyPct),
		TopMovers:         topMovingSectors(sectors, 5),
	}, nil
}

// fetchPrevDay calls /v2/aggs/ticker/{ticker}/prev and returns both
// the open-to-close percentage change and the closing price.
func (p *polygonProvider) fetchPrevDay(ctx context.Context, ticker string) (changePct float64, closePrice float64, err error) {
	url := fmt.Sprintf("%s/v2/aggs/ticker/%s/prev?adjusted=true&apiKey=%s", polygonBaseURL, ticker, p.apiKey)
	log.Printf("[polygon] GET prev-day %s", ticker)
	body, fetchErr := p.get(ctx, url)
	if fetchErr != nil {
		return 0, 0, fetchErr
	}
	log.Printf("[polygon] prev-day %s raw response: %s", ticker, body)

	var resp struct {
		Results []struct {
			Open  float64 `json:"o"`
			Close float64 `json:"c"`
		} `json:"results"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0, fmt.Errorf("parse prev response: %w", err)
	}
	log.Printf("[polygon] prev-day %s status=%q results=%d", ticker, resp.Status, len(resp.Results))
	if len(resp.Results) == 0 {
		return 0, 0, fmt.Errorf("no results for %s (markets may be closed)", ticker)
	}
	r := resp.Results[0]
	if r.Open == 0 {
		return 0, 0, fmt.Errorf("zero open price for %s", ticker)
	}
	pct := (r.Close - r.Open) / r.Open * 100
	log.Printf("[polygon] prev-day %s open=%.4f close=%.4f change=%.4f%%", ticker, r.Open, r.Close, pct)
	return pct, r.Close, nil
}

// GetPrice returns the previous-day close price for a single ticker.
// Satisfies the MarketDataProvider.GetPrice port — used by the verdict stamper as the Polygon data point.
func (p *polygonProvider) GetPrice(ctx context.Context, ticker string) (float64, error) {
	_, closePrice, err := p.fetchPrevDay(ctx, ticker)
	return closePrice, err
}

// fetchSectorPerformance calls the snapshot endpoint for all sector ETFs and returns
// a map of sector name → today's change percent.
func (p *polygonProvider) fetchSectorPerformance(ctx context.Context) (map[string]float64, error) {
	type result struct {
		sector string
		pct    float64
		err    error
	}

	ch := make(chan result, len(sectorETFs))
	for sectorName, etf := range sectorETFs {
		sectorName, etf := sectorName, etf
		go func() {
			pct, _, err := p.fetchPrevDay(ctx, etf)
			ch <- result{sector: sectorName, pct: pct, err: err}
		}()
	}

	sectors := make(map[string]float64, len(sectorETFs))
	for range sectorETFs {
		r := <-ch
		if r.err != nil {
			log.Printf("[polygon] sector %s fetch error (skipped): %v", r.sector, r.err)
			continue
		}
		sectors[r.sector] = r.pct
	}
	log.Printf("[polygon] final sectors: %v", sectors)
	return sectors, nil
}

// get performs a GET request and returns the raw body bytes.
func (p *polygonProvider) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

// deriveSentiment converts SPY daily change into a human-readable sentiment label.
func deriveSentiment(spyPct float64) string {
	switch {
	case spyPct > 0.5:
		return "bullish"
	case spyPct < -0.5:
		return "bearish"
	default:
		return "neutral"
	}
}

// topMovingSectors returns the n sectors with the largest absolute % change.
func topMovingSectors(sectors map[string]float64, n int) []models.TickerSnapshot {
	type entry struct {
		name string
		pct  float64
	}
	entries := make([]entry, 0, len(sectors))
	for name, pct := range sectors {
		entries = append(entries, entry{name, pct})
	}
	sort.Slice(entries, func(i, j int) bool {
		return math.Abs(entries[i].pct) > math.Abs(entries[j].pct)
	})
	if n > len(entries) {
		n = len(entries)
	}
	movers := make([]models.TickerSnapshot, n)
	for i := 0; i < n; i++ {
		movers[i] = models.TickerSnapshot{
			Symbol:        entries[i].name,
			ChangePercent: entries[i].pct,
			Price:         0,
		}
	}
	return movers
}
