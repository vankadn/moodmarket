// services/verdict_stamper_test.go
//
// Reading this file explains what the verdict stamping system does:
// - How it picks entry prices (fill price → Polygon fallback)
// - How it picks current prices (Alpaca → Polygon fallback)
// - How it weights per-ticker returns into an overall return
// - How it computes the SPY benchmark
// - How it deduplicates Polygon calls across decisions (the Phase 22 rate-limit fix)
//
// These tests exist because all five Phase 22/23 bugs were invisible until the UI
// surfaced them. Each test name is a sentence stating a requirement.
package services

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ---------- mock implementations ----------

// mockMarketProvider stubs MarketDataProvider.GetPrice.
// GetDailySnapshot is not used by the verdict stamper and panics if called.
type mockMarketProvider struct {
	prices map[string]float64
	err    error // when non-nil, every GetPrice call returns this error
}

func (m *mockMarketProvider) GetPrice(_ context.Context, ticker string) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	p, ok := m.prices[ticker]
	if !ok {
		return 0, errors.New("ticker not in mock: " + ticker)
	}
	return p, nil
}

func (m *mockMarketProvider) GetDailySnapshot(_ context.Context) (*models.MarketSnapshot, error) {
	panic("GetDailySnapshot not used by verdict stamper")
}

// mockBrokerageProvider stubs BrokerageProvider.GetCurrentPrice.
// All other methods panic — tests fail loudly if the stamper ever calls them unexpectedly.
type mockBrokerageProvider struct {
	prices map[string]float64
	err    error // when non-nil, every GetCurrentPrice call returns this error
}

func (m *mockBrokerageProvider) GetCurrentPrice(_ context.Context, ticker string) (float64, error) {
	if m.err != nil {
		return 0, m.err
	}
	p, ok := m.prices[ticker]
	if !ok {
		return 0, errors.New("ticker not in mock: " + ticker)
	}
	return p, nil
}

func (m *mockBrokerageProvider) PlaceMarketOrder(_ context.Context, _ models.TradeOrder) (*models.TradeReceipt, error) {
	panic("not used in verdict tests")
}
func (m *mockBrokerageProvider) GetPositions(_ context.Context, _ string) ([]models.Position, error) {
	panic("not used in verdict tests")
}
func (m *mockBrokerageProvider) GetOrder(_ context.Context, _ string) (*models.TradeReceipt, error) {
	panic("not used in verdict tests")
}
func (m *mockBrokerageProvider) GetPortfolioHistory(_ context.Context, _, _, _ string) ([]models.HistoryPoint, error) {
	panic("not used in verdict tests")
}

// ---------- helpers ----------

func makeReceipt(ticker string, filledPrice, filledAmount float64) models.TradeReceipt {
	return models.TradeReceipt{
		Ticker:       ticker,
		FilledPrice:  filledPrice,
		FilledAmount: filledAmount,
		Status:       "filled",
		Timestamp:    time.Now(),
	}
}

func makeDecision(receipts ...models.TradeReceipt) *models.InvestmentDecision {
	return &models.InvestmentDecision{
		ID:        "test-decision",
		Receipts:  receipts,
		Timestamp: time.Now().Add(-25 * time.Hour),
	}
}

func makeDecisionWithSPY(spyEntryPrice float64, receipts ...models.TradeReceipt) *models.InvestmentDecision {
	d := makeDecision(receipts...)
	d.MarketSnapshot = &models.MarketSnapshot{SPYPrice: spyEntryPrice}
	return d
}

func brokerPrices(prices map[string]float64) ports.BrokerageProvider {
	return &mockBrokerageProvider{prices: prices}
}

func brokerError(err error) ports.BrokerageProvider {
	return &mockBrokerageProvider{err: err}
}

// ---------- TestBuildVerdict ----------

func TestBuildVerdict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name              string
		decision          *models.InvestmentDecision
		brokerage         ports.BrokerageProvider // nil = no Alpaca available
		priceCache        map[string]float64
		wantTickerCount   int
		wantReturnOnFirst float64 // checked only when > 0
		wantOverallReturn float64 // checked only when != 0 or wantTickerCount > 0
		wantSPYReturn     float64
		wantBeatMarket    bool
	}{
		{
			name: "entry_price_uses_filled_price_from_trade_receipt",
			// Normal path: Alpaca fills the order at $100, current Alpaca price is $110.
			// Return = (110 - 100) / 100 * 100 = +10%
			decision:          makeDecision(makeReceipt("VTI", 100.0, 50.0)),
			brokerage:         brokerPrices(map[string]float64{"VTI": 110.0}),
			priceCache:        map[string]float64{"VTI": 108.0},
			wantTickerCount:   1,
			wantReturnOnFirst: 10.0,
			wantOverallReturn: 10.0,
		},
		{
			name: "entry_price_falls_back_to_polygon_prev_day_when_filled_price_is_zero",
			// Alpaca async orders arrive with FilledPrice=0 at submission time.
			// Polygon prev-day ($100) is used as the entry price proxy.
			// Current (Alpaca real-time): $110 → return +10%
			decision:          makeDecision(makeReceipt("VTI", 0.0, 50.0)),
			brokerage:         brokerPrices(map[string]float64{"VTI": 110.0}),
			priceCache:        map[string]float64{"VTI": 100.0},
			wantTickerCount:   1,
			wantReturnOnFirst: 10.0,
			wantOverallReturn: 10.0,
		},
		{
			name: "ticker_is_skipped_when_filled_price_is_zero_and_polygon_has_no_price",
			// Both price sources unavailable → ticker cannot have a return computed.
			// Excluded from verdict to avoid NaN/Inf in the output.
			decision:          makeDecision(makeReceipt("UNKNOWN", 0.0, 50.0)),
			brokerage:         brokerPrices(map[string]float64{}),
			priceCache:        map[string]float64{},
			wantTickerCount:   0,
			wantOverallReturn: 0.0,
		},
		{
			name: "current_price_comes_from_alpaca_when_brokerage_provider_is_available",
			// When brokerage is available, Alpaca real-time wins over Polygon prev-day.
			// Entry $100, Alpaca $115, Polygon $108 → return uses Alpaca: +15%
			decision:          makeDecision(makeReceipt("VTI", 100.0, 50.0)),
			brokerage:         brokerPrices(map[string]float64{"VTI": 115.0}),
			priceCache:        map[string]float64{"VTI": 108.0},
			wantTickerCount:   1,
			wantReturnOnFirst: 15.0,
			wantOverallReturn: 15.0,
		},
		{
			name: "current_price_falls_back_to_polygon_prev_day_when_alpaca_is_unavailable",
			// No brokerage provider (nil) → Polygon prev-day used as current price.
			// Entry $100, Polygon $105 → return +5%
			decision:          makeDecision(makeReceipt("VTI", 100.0, 50.0)),
			brokerage:         nil,
			priceCache:        map[string]float64{"VTI": 105.0},
			wantTickerCount:   1,
			wantReturnOnFirst: 5.0,
			wantOverallReturn: 5.0,
		},
		{
			name: "current_price_falls_back_to_polygon_prev_day_when_alpaca_returns_error",
			// Alpaca fails (network error, bad credentials) → Polygon prev-day as fallback.
			// Entry $100, Polygon $104 → return +4%
			decision:          makeDecision(makeReceipt("VTI", 100.0, 50.0)),
			brokerage:         brokerError(errors.New("alpaca unavailable")),
			priceCache:        map[string]float64{"VTI": 104.0},
			wantTickerCount:   1,
			wantReturnOnFirst: 4.0,
			wantOverallReturn: 4.0,
		},
		{
			name: "return_pct_is_computed_as_entry_to_current_percentage_change",
			// Formula verification: (current - entry) / entry * 100
			// Entry $200, current $220 → (220-200)/200*100 = +10%
			decision:          makeDecision(makeReceipt("QQQ", 200.0, 100.0)),
			brokerage:         brokerPrices(map[string]float64{"QQQ": 220.0}),
			priceCache:        map[string]float64{"QQQ": 218.0},
			wantTickerCount:   1,
			wantReturnOnFirst: 10.0,
			wantOverallReturn: 10.0,
		},
		{
			name: "overall_return_is_dollar_weighted_average_of_per_ticker_returns",
			// VTI: entry $100 → current $110, filled $200 → +10%, weight 200
			// QQQ: entry $200 → current $180, filled $100 → -10%, weight 100
			// Weighted avg: (10×200 + (−10)×100) / 300 = 1000/300 ≈ +3.33%
			decision: makeDecision(
				makeReceipt("VTI", 100.0, 200.0),
				makeReceipt("QQQ", 200.0, 100.0),
			),
			brokerage:         brokerPrices(map[string]float64{"VTI": 110.0, "QQQ": 180.0}),
			priceCache:        map[string]float64{"VTI": 108.0, "QQQ": 198.0},
			wantTickerCount:   2,
			wantOverallReturn: 10.0*200.0/300.0 + (-10.0)*100.0/300.0,
		},
		{
			name: "equal_weight_is_used_when_filled_amount_is_zero_async_alpaca_order",
			// Alpaca async orders report FilledAmount=0 at submission time.
			// Without an equal-weight fallback, totalWeight stays 0 → OverallReturnPct stays 0.
			// Equal weight (1.0) per ticker gives a correct unweighted average.
			// Both tickers: +10% → overall +10%
			decision: makeDecision(
				makeReceipt("VTI", 100.0, 0.0),
				makeReceipt("QQQ", 200.0, 0.0),
			),
			brokerage:         brokerPrices(map[string]float64{"VTI": 110.0, "QQQ": 220.0}),
			priceCache:        map[string]float64{"VTI": 108.0, "QQQ": 218.0},
			wantTickerCount:   2,
			wantOverallReturn: 10.0,
		},
		{
			name: "spy_benchmark_is_computed_when_market_snapshot_has_spy_price",
			// SPY entry stored at decision time: $500; current Polygon price: $510
			// SPY return = (510−500)/500×100 = +2%
			// Portfolio: VTI $100→$105 = +5% → beats market
			decision:        makeDecisionWithSPY(500.0, makeReceipt("VTI", 100.0, 50.0)),
			brokerage:       brokerPrices(map[string]float64{"VTI": 105.0}),
			priceCache:      map[string]float64{"VTI": 103.0, "SPY": 510.0},
			wantTickerCount: 1,
			wantSPYReturn:   2.0,
			wantBeatMarket:  true,
		},
		{
			name: "spy_benchmark_is_skipped_when_market_snapshot_is_nil",
			// Decisions before Phase 22 have no MarketSnapshot.
			// SPYReturnPct must be 0 and BeatMarket must be false (not computed).
			decision:        makeDecision(makeReceipt("VTI", 100.0, 50.0)),
			brokerage:       brokerPrices(map[string]float64{"VTI": 110.0}),
			priceCache:      map[string]float64{"VTI": 108.0, "SPY": 510.0},
			wantTickerCount: 1,
			wantSPYReturn:   0.0,
			wantBeatMarket:  false,
		},
		{
			name: "spy_benchmark_is_skipped_when_spy_price_is_zero_predates_phase22",
			// Phase 19 decisions had MarketSnapshot but SPYPrice was not yet populated.
			// SPYPrice=0 would cause division-by-zero → benchmark skipped.
			decision:        makeDecisionWithSPY(0.0, makeReceipt("VTI", 100.0, 50.0)),
			brokerage:       brokerPrices(map[string]float64{"VTI": 110.0}),
			priceCache:      map[string]float64{"VTI": 108.0, "SPY": 510.0},
			wantTickerCount: 1,
			wantSPYReturn:   0.0,
			wantBeatMarket:  false,
		},
		{
			name: "beat_market_is_true_when_portfolio_return_exceeds_spy_return",
			// Portfolio +5% vs SPY +2% → beat_market = true
			decision:       makeDecisionWithSPY(500.0, makeReceipt("VTI", 100.0, 50.0)),
			brokerage:      brokerPrices(map[string]float64{"VTI": 105.0}),
			priceCache:     map[string]float64{"VTI": 103.0, "SPY": 510.0},
			wantBeatMarket: true,
		},
		{
			name: "beat_market_is_false_when_portfolio_return_is_below_spy_return",
			// Portfolio +1% vs SPY +5% → beat_market = false
			decision:       makeDecisionWithSPY(500.0, makeReceipt("VTI", 100.0, 50.0)),
			brokerage:      brokerPrices(map[string]float64{"VTI": 101.0}),
			priceCache:     map[string]float64{"VTI": 100.5, "SPY": 525.0},
			wantBeatMarket: false,
		},
		{
			name: "verdict_has_empty_ticker_verdicts_when_all_receipts_lack_prices",
			// All receipts have FilledPrice=0 and are absent from the price cache.
			// The verdict is stamped but with no ticker data and 0 overall return.
			// This is the "bad verdict" state that triggers re-stamping on the next run.
			decision: makeDecision(
				makeReceipt("VTI", 0.0, 0.0),
				makeReceipt("QQQ", 0.0, 0.0),
			),
			brokerage:         nil,
			priceCache:        map[string]float64{},
			wantTickerCount:   0,
			wantOverallReturn: 0.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			verdict := buildVerdict(ctx, tc.decision, tc.brokerage, tc.priceCache)

			if verdict == nil {
				t.Fatal("buildVerdict returned nil; expected a non-nil verdict")
			}

			if len(verdict.TickerVerdicts) != tc.wantTickerCount {
				t.Errorf("ticker_verdicts count: got %d, want %d", len(verdict.TickerVerdicts), tc.wantTickerCount)
			}

			if tc.wantReturnOnFirst != 0 && len(verdict.TickerVerdicts) > 0 {
				got := verdict.TickerVerdicts[0].ReturnPct
				if math.Abs(got-tc.wantReturnOnFirst) > 0.001 {
					t.Errorf("ticker_verdicts[0].return_pct: got %.4f, want %.4f", got, tc.wantReturnOnFirst)
				}
			}

			if tc.wantOverallReturn != 0 || tc.wantTickerCount > 0 {
				if math.Abs(verdict.OverallReturnPct-tc.wantOverallReturn) > 0.001 {
					t.Errorf("overall_return_pct: got %.4f, want %.4f", verdict.OverallReturnPct, tc.wantOverallReturn)
				}
			}

			if math.Abs(verdict.SPYReturnPct-tc.wantSPYReturn) > 0.001 {
				t.Errorf("spy_return_pct: got %.4f, want %.4f", verdict.SPYReturnPct, tc.wantSPYReturn)
			}

			if verdict.BeatMarket != tc.wantBeatMarket {
				t.Errorf("beat_market: got %v, want %v (overall=%.2f%%, spy=%.2f%%)",
					verdict.BeatMarket, tc.wantBeatMarket,
					verdict.OverallReturnPct, verdict.SPYReturnPct)
			}
		})
	}
}

// ---------- TestBuildPriceCache ----------

func TestBuildPriceCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("fetches_each_unique_ticker_exactly_once_regardless_of_how_many_decisions_reference_it", func(t *testing.T) {
		t.Parallel()
		// The Polygon free tier allows ~5 req/min.
		// 37 decisions × 5 tickers naively = 185 calls → every call gets 429.
		// The fix lives in stampVerdicts: it builds a deduplicated ticker set across ALL
		// decisions before calling buildPriceCache — so this function always receives a
		// unique list. Verify here that given a unique list, each ticker is called once.
		callCount := map[string]int{}
		mp := &countingMarketProvider{
			prices:    map[string]float64{"VTI": 263.0, "QQQ": 443.0, "SPY": 518.0},
			callCount: callCount,
		}
		tickers := []string{"VTI", "QQQ", "SPY"} // unique — stampVerdicts guarantees this
		cache := buildPriceCache(ctx, tickers, mp)

		for _, ticker := range tickers {
			if callCount[ticker] != 1 {
				t.Errorf("ticker %s fetched %d times; want exactly 1", ticker, callCount[ticker])
			}
			if _, ok := cache[ticker]; !ok {
				t.Errorf("ticker %s should be in cache but is absent", ticker)
			}
		}
		if len(cache) != len(tickers) {
			t.Errorf("cache has %d entries; want %d", len(cache), len(tickers))
		}
	})

	t.Run("failed_ticker_is_omitted_from_cache_without_blocking_other_tickers", func(t *testing.T) {
		t.Parallel()
		// QQQ is absent from the provider → GetPrice returns error for QQQ.
		// VTI and SPY should still be fetched successfully.
		mp := &mockMarketProvider{
			prices: map[string]float64{"VTI": 263.0, "SPY": 518.0},
		}
		cache := buildPriceCache(ctx, []string{"VTI", "QQQ", "SPY"}, mp)

		if _, ok := cache["QQQ"]; ok {
			t.Error("failed ticker QQQ should be absent from cache")
		}
		if _, ok := cache["VTI"]; !ok {
			t.Error("successful ticker VTI should be in cache")
		}
		if _, ok := cache["SPY"]; !ok {
			t.Error("successful ticker SPY should be in cache")
		}
	})

	t.Run("returns_empty_cache_when_all_ticker_fetches_fail", func(t *testing.T) {
		t.Parallel()
		mp := &mockMarketProvider{err: errors.New("polygon rate limited (429)")}
		cache := buildPriceCache(ctx, []string{"VTI", "QQQ"}, mp)

		if len(cache) != 0 {
			t.Errorf("expected empty cache when all fetches fail, got %d entries", len(cache))
		}
	})
}

// countingMarketProvider tracks how many times each ticker is fetched.
type countingMarketProvider struct {
	prices    map[string]float64
	callCount map[string]int
}

func (c *countingMarketProvider) GetPrice(_ context.Context, ticker string) (float64, error) {
	c.callCount[ticker]++
	p, ok := c.prices[ticker]
	if !ok {
		return 0, errors.New("ticker not in mock: " + ticker)
	}
	return p, nil
}

func (c *countingMarketProvider) GetDailySnapshot(_ context.Context) (*models.MarketSnapshot, error) {
	panic("GetDailySnapshot not used by verdict stamper")
}
