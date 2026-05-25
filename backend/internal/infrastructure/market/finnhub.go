// infrastructure/market/finnhub.go
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const finnhubBaseURL = "https://finnhub.io/api/v1"

type priceCacheEntry struct {
	price float64
	exp   time.Time
}

type finnhubProvider struct {
	apiKey     string
	ttl        time.Duration
	httpClient *http.Client

	mu             sync.RWMutex
	cachedSnapshot *models.MarketSnapshot
	snapshotExp    time.Time
	priceCache     map[string]priceCacheEntry
}

func newFinnhubProvider(apiKey string, ttl time.Duration) *finnhubProvider {
	return &finnhubProvider{
		apiKey:     apiKey,
		ttl:        ttl,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		priceCache: make(map[string]priceCacheEntry),
	}
}

// GetDailySnapshot returns a cached snapshot if within TTL, otherwise fetches
// real-time quotes from Finnhub for SPY, QQQ, and the five sector ETFs.
func (f *finnhubProvider) GetDailySnapshot(ctx context.Context) (*models.MarketSnapshot, error) {
	f.mu.RLock()
	if f.cachedSnapshot != nil && time.Now().Before(f.snapshotExp) {
		snapshot := f.cachedSnapshot
		f.mu.RUnlock()
		log.Printf("[finnhub] snapshot cache hit (expires %s)", f.snapshotExp.Format(time.RFC3339))
		return snapshot, nil
	}
	f.mu.RUnlock()

	snapshot, err := f.fetchSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	f.cachedSnapshot = snapshot
	f.snapshotExp = time.Now().Add(f.ttl)
	f.mu.Unlock()

	log.Printf("[finnhub] market snapshot fetched (next refresh at %s)", f.snapshotExp.Format(time.RFC3339))
	return snapshot, nil
}

// finnhubQuote maps the fields returned by GET /api/v1/quote.
type finnhubQuote struct {
	C  float64 `json:"c"`  // current price
	O  float64 `json:"o"`  // open
	H  float64 `json:"h"`  // high
	L  float64 `json:"l"`  // low
	PC float64 `json:"pc"` // previous close
	DP float64 `json:"dp"` // percent change from previous close
	D  float64 `json:"d"`  // change (absolute)
	T  int64   `json:"t"`  // unix timestamp of quote
}

func (f *finnhubProvider) fetchQuote(ctx context.Context, ticker string) (finnhubQuote, error) {
	url := fmt.Sprintf("%s/quote?symbol=%s&token=%s", finnhubBaseURL, ticker, f.apiKey)
	log.Printf("[finnhub] GET quote %s", ticker)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return finnhubQuote{}, fmt.Errorf("finnhub: build request for %s: %w", ticker, err)
	}

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return finnhubQuote{}, fmt.Errorf("finnhub: http %s: %w", ticker, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return finnhubQuote{}, fmt.Errorf("finnhub: read body %s: %w", ticker, err)
	}
	if resp.StatusCode != http.StatusOK {
		return finnhubQuote{}, fmt.Errorf("finnhub: API %d for %s: %s", resp.StatusCode, ticker, body)
	}

	var q finnhubQuote
	if err := json.Unmarshal(body, &q); err != nil {
		return finnhubQuote{}, fmt.Errorf("finnhub: parse %s: %w", ticker, err)
	}

	log.Printf("[finnhub] quote %s c=%.4f pc=%.4f dp=%+.4f%%", ticker, q.C, q.PC, q.DP)
	return q, nil
}

// resolvePrice returns the current price from a quote, falling back to previous
// close when the market is closed (c == 0) and logging a warning.
func resolvePrice(ticker string, q finnhubQuote) (float64, error) {
	if q.C != 0 {
		return q.C, nil
	}
	log.Printf("[finnhub] %s current price is 0 (market closed or bad ticker) — using previous close %.4f", ticker, q.PC)
	if q.PC == 0 {
		return 0, fmt.Errorf("finnhub: %s: both current price and previous close are 0", ticker)
	}
	return q.PC, nil
}

func (f *finnhubProvider) fetchSnapshot(ctx context.Context) (*models.MarketSnapshot, error) {
	spyQ, err := f.fetchQuote(ctx, "SPY")
	if err != nil {
		return nil, fmt.Errorf("finnhub: fetch SPY: %w", err)
	}
	qqqQ, err := f.fetchQuote(ctx, "QQQ")
	if err != nil {
		return nil, fmt.Errorf("finnhub: fetch QQQ: %w", err)
	}

	spyPrice, err := resolvePrice("SPY", spyQ)
	if err != nil {
		return nil, err
	}

	sectors, err := f.fetchSectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("finnhub: fetch sectors: %w", err)
	}

	return &models.MarketSnapshot{
		Date:              time.Now().Format("2006-01-02"),
		SPYPrice:          spyPrice,
		SPYChangePercent:  spyQ.DP,
		QQQChangePercent:  qqqQ.DP,
		SectorPerformance: sectors,
		MarketSentiment:   deriveSentiment(spyQ.DP),
		TopMovers:         topMovingSectors(sectors, 5),
	}, nil
}

func (f *finnhubProvider) fetchSectors(ctx context.Context) (map[string]float64, error) {
	type result struct {
		sector string
		pct    float64
		err    error
	}

	ch := make(chan result, len(sectorETFs))
	for sectorName, etf := range sectorETFs {
		sectorName, etf := sectorName, etf
		go func() {
			q, err := f.fetchQuote(ctx, etf)
			ch <- result{sector: sectorName, pct: q.DP, err: err}
		}()
	}

	sectors := make(map[string]float64, len(sectorETFs))
	for range sectorETFs {
		r := <-ch
		if r.err != nil {
			log.Printf("[finnhub] sector %s fetch error (skipped): %v", r.sector, r.err)
			continue
		}
		sectors[r.sector] = r.pct
	}
	log.Printf("[finnhub] sectors: %v", sectors)
	return sectors, nil
}

// GetPrice returns the current real-time price for a single ticker, falling back
// to the previous close when the market is closed. Results are cached per-ticker
// for the configured TTL to avoid redundant Finnhub calls during verdict stamping.
func (f *finnhubProvider) GetPrice(ctx context.Context, ticker string) (float64, error) {
	f.mu.RLock()
	if entry, ok := f.priceCache[ticker]; ok && time.Now().Before(entry.exp) {
		f.mu.RUnlock()
		log.Printf("[finnhub] price cache hit %s=%.4f", ticker, entry.price)
		return entry.price, nil
	}
	f.mu.RUnlock()

	q, err := f.fetchQuote(ctx, ticker)
	if err != nil {
		return 0, err
	}

	price, err := resolvePrice(ticker, q)
	if err != nil {
		return 0, err
	}

	f.mu.Lock()
	f.priceCache[ticker] = priceCacheEntry{price: price, exp: time.Now().Add(f.ttl)}
	f.mu.Unlock()

	return price, nil
}

// compile-time check that finnhubProvider satisfies MarketDataProvider.
var _ interface {
	GetDailySnapshot(context.Context) (*models.MarketSnapshot, error)
	GetPrice(context.Context, string) (float64, error)
} = (*finnhubProvider)(nil)
