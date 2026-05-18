// application/services/verdict_stamper.go
package services

import (
	"context"
	"log"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// stampVerdicts processes all unverdicted decisions for a single user.
// Polygon prices are fetched once per unique ticker (not once per decision)
// to stay within the free-tier rate limit.
func stampVerdicts(
	ctx context.Context,
	userID string,
	minAge time.Duration,
	decisionRepo ports.DecisionRepository,
	profileRepo ports.ProfileRepository,
	brokerageFactory ports.BrokerageProviderFactory,
	marketProvider ports.MarketDataProvider,
) {
	decisions, err := decisionRepo.ListUnverdicted(ctx, userID, minAge)
	if err != nil {
		log.Printf("[verdict] user=%s fetch unverdicted: %v", userID, err)
		return
	}
	log.Printf("[verdict] user=%s decisions to process: %d", userID, len(decisions))
	if len(decisions) == 0 {
		return
	}

	// Attempt to build a per-user brokerage provider for real-time Alpaca prices.
	var brokerageProvider ports.BrokerageProvider
	conns, connErr := profileRepo.GetBrokerageConnections(ctx, userID)
	if connErr != nil {
		log.Printf("[verdict] user=%s get brokerage connections failed: %v — will use Polygon fallback", userID, connErr)
	} else if len(conns) == 0 {
		log.Printf("[verdict] user=%s no brokerage connections stored — will use Polygon fallback", userID)
	} else {
		p, factoryErr := brokerageFactory.ForUser(&conns[0])
		if factoryErr != nil {
			log.Printf("[verdict] user=%s brokerage factory failed: %v — will use Polygon fallback", userID, factoryErr)
		} else {
			log.Printf("[verdict] user=%s brokerage provider ready (conn=%s)", userID, conns[0].ID)
			brokerageProvider = p
		}
	}

	// Collect unique tickers across all decisions (including SPY for benchmark).
	tickerSet := map[string]bool{"SPY": true}
	for _, d := range decisions {
		for _, r := range d.Receipts {
			if r.Ticker != "" {
				tickerSet[r.Ticker] = true
			}
		}
	}
	tickers := make([]string, 0, len(tickerSet))
	for t := range tickerSet {
		tickers = append(tickers, t)
	}
	log.Printf("[verdict] user=%s fetching Polygon prices for %d unique tickers: %v", userID, len(tickers), tickers)

	// Fetch each unique ticker once from Polygon — one call total per ticker regardless
	// of how many decisions reference it. This avoids hitting the free-tier rate limit.
	priceCache := buildPriceCache(ctx, tickers, marketProvider)
	log.Printf("[verdict] user=%s price cache ready — %d/%d tickers fetched", userID, len(priceCache), len(tickers))

	for i := range decisions {
		d := &decisions[i]
		log.Printf("[verdict] user=%s decision=%s receipts=%d", userID, d.ID, len(d.Receipts))
		verdict := buildVerdict(ctx, d, brokerageProvider, priceCache)
		if err := decisionRepo.StampVerdict(ctx, d.ID, verdict); err != nil {
			log.Printf("[verdict] user=%s decision=%s stamp failed: %v", userID, d.ID, err)
			continue
		}
		log.Printf("[verdict] user=%s decision=%s stamped — overall=%.2f%% spy=%.2f%% beat=%v tickers=%d",
			userID, d.ID, verdict.OverallReturnPct, verdict.SPYReturnPct, verdict.BeatMarket, len(verdict.TickerVerdicts))
	}
}

// buildPriceCache fetches prev-day close prices for each ticker from Polygon.
// Each ticker is fetched exactly once; failures are logged and omitted from the cache.
func buildPriceCache(ctx context.Context, tickers []string, marketProvider ports.MarketDataProvider) map[string]float64 {
	cache := make(map[string]float64, len(tickers))
	for _, ticker := range tickers {
		price, err := marketProvider.GetPrice(ctx, ticker)
		if err != nil {
			log.Printf("[verdict] price cache: ticker=%s polygon failed: %v", ticker, err)
			continue
		}
		log.Printf("[verdict] price cache: ticker=%s prevDayPrice=%.4f", ticker, price)
		cache[ticker] = price
	}
	return cache
}

// buildVerdict constructs a DecisionVerdict for a single decision.
// priceCache holds Polygon prev-day prices keyed by ticker (pre-fetched once per run).
// Alpaca real-time prices are fetched per ticker if a brokerage provider is available.
func buildVerdict(
	ctx context.Context,
	decision *models.InvestmentDecision,
	brokerageProvider ports.BrokerageProvider,
	priceCache map[string]float64,
) *models.DecisionVerdict {
	now := time.Now()
	verdict := &models.DecisionVerdict{StampedAt: now}

	var totalWeight float64
	var weightedReturn float64

	for _, receipt := range decision.Receipts {
		prevPrice, hasPrev := priceCache[receipt.Ticker]

		log.Printf("[verdict] decision=%s ticker=%s filledPrice=%.4f filledAmount=%.4f status=%s polygonPrev=%.4f(ok=%v)",
			decision.ID, receipt.Ticker, receipt.FilledPrice, receipt.FilledAmount, receipt.Status, prevPrice, hasPrev)

		// Entry price: use Alpaca fill price if available, else Polygon prev-day as proxy.
		entryPrice := receipt.FilledPrice
		if entryPrice <= 0 {
			if !hasPrev || prevPrice <= 0 {
				log.Printf("[verdict] decision=%s ticker=%s no entry price and no Polygon price — skipping", decision.ID, receipt.Ticker)
				continue
			}
			log.Printf("[verdict] decision=%s ticker=%s filledPrice=0 — using Polygon prevDay=%.4f as entry", decision.ID, receipt.Ticker, prevPrice)
			entryPrice = prevPrice
		}

		tv := models.TickerVerdict{
			Ticker:     receipt.Ticker,
			EntryPrice: entryPrice,
		}
		if hasPrev {
			tv.PrevDayPrice = prevPrice
			tv.PrevDayTimestamp = now.AddDate(0, 0, -1).Truncate(24 * time.Hour)
		}

		// Alpaca: real-time last trade price (requires per-user credentials).
		if brokerageProvider != nil {
			currentPrice, err := brokerageProvider.GetCurrentPrice(ctx, receipt.Ticker)
			if err != nil {
				log.Printf("[verdict] decision=%s ticker=%s alpaca price failed: %v", decision.ID, receipt.Ticker, err)
			} else {
				log.Printf("[verdict] decision=%s ticker=%s alpaca currentPrice=%.4f", decision.ID, receipt.Ticker, currentPrice)
				tv.CurrentPrice = currentPrice
				tv.CurrentTimestamp = now
				if tv.PrevDayPrice > 0 {
					tv.TodayChangePct = (currentPrice - tv.PrevDayPrice) / tv.PrevDayPrice * 100
				}
			}
		}

		// Fallback: use Polygon prev-day close as current price if Alpaca is unavailable.
		if tv.CurrentPrice == 0 && tv.PrevDayPrice > 0 {
			log.Printf("[verdict] decision=%s ticker=%s using Polygon prevDay=%.4f as current (Alpaca unavailable)", decision.ID, receipt.Ticker, tv.PrevDayPrice)
			tv.CurrentPrice = tv.PrevDayPrice
			tv.CurrentTimestamp = tv.PrevDayTimestamp
		}

		if tv.CurrentPrice > 0 {
			tv.ReturnPct = (tv.CurrentPrice - entryPrice) / entryPrice * 100
			log.Printf("[verdict] decision=%s ticker=%s entry=%.4f current=%.4f return=%.4f%%",
				decision.ID, receipt.Ticker, entryPrice, tv.CurrentPrice, tv.ReturnPct)
		} else {
			log.Printf("[verdict] decision=%s ticker=%s currentPrice still 0 — return will be 0", decision.ID, receipt.Ticker)
		}

		verdict.TickerVerdicts = append(verdict.TickerVerdicts, tv)

		if tv.CurrentPrice > 0 {
			// Use actual fill amount for weighting; fall back to equal weight when order fill wasn't captured.
			weight := receipt.FilledAmount
			if weight <= 0 {
				weight = 1.0
			}
			totalWeight += weight
			weightedReturn += tv.ReturnPct * weight
		}
	}

	if totalWeight > 0 {
		verdict.OverallReturnPct = weightedReturn / totalWeight
	}
	log.Printf("[verdict] decision=%s totalWeight=%.4f overallReturn=%.4f%%", decision.ID, totalWeight, verdict.OverallReturnPct)

	// SPY benchmark — uses cached Polygon price; no per-user credentials needed.
	if decision.MarketSnapshot == nil {
		log.Printf("[verdict] decision=%s no market_snapshot — SPY benchmark skipped", decision.ID)
	} else if decision.MarketSnapshot.SPYPrice <= 0 {
		log.Printf("[verdict] decision=%s market_snapshot.spy_price=0 — SPY benchmark skipped (decision predates Phase 22)", decision.ID)
	} else {
		currentSPY, hasSPY := priceCache["SPY"]
		if !hasSPY {
			log.Printf("[verdict] decision=%s SPY not in price cache — benchmark skipped", decision.ID)
		} else {
			verdict.SPYReturnPct = (currentSPY - decision.MarketSnapshot.SPYPrice) / decision.MarketSnapshot.SPYPrice * 100
			verdict.BeatMarket = verdict.OverallReturnPct > verdict.SPYReturnPct
			log.Printf("[verdict] decision=%s spyEntry=%.4f spyCurrent=%.4f spyReturn=%.4f%% beatMarket=%v",
				decision.ID, decision.MarketSnapshot.SPYPrice, currentSPY, verdict.SPYReturnPct, verdict.BeatMarket)
		}
	}

	return verdict
}
