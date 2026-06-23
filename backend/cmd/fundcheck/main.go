// fundcheck is a one-shot diagnostic tool for FundamentalsProvider.
// It calls all three methods against real Finnhub endpoints and prints the
// mapped struct values alongside raw log output — use this to confirm
// field name mapping (peTTM, 52WeekHigh, totalDebt/totalEquityAnnual, etc.)
// before building strategy logic on top of this provider.
//
// Usage:
//
//	go run ./cmd/fundcheck          # defaults to MU
//	go run ./cmd/fundcheck MU AAPL  # multiple tickers
//
// Requires FINNHUB_API_KEY in .env or environment.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	infrafundamentals "github.com/krishnarajivvns/investiq/internal/infrastructure/fundamentals"
	inframarket "github.com/krishnarajivvns/investiq/internal/infrastructure/market"
)

func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, strings.Trim(value, `"'`))
		}
	}
}

func main() {
	loadEnv(".env")
	os.Setenv("FUNDAMENTALS_PROVIDER", "finnhub")
	os.Setenv("MARKET_PROVIDER", "finnhub")

	tickers := os.Args[1:]
	if len(tickers) == 0 {
		tickers = []string{"MU"}
	}

	provider, err := infrafundamentals.NewFundamentalsProvider()
	if err != nil {
		log.Fatalf("init: %v", err)
	}

	marketProvider, err := inframarket.NewMarketDataProvider()
	if err != nil {
		log.Fatalf("market init: %v", err)
	}

	ctx := context.Background()
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for _, ticker := range tickers {
		fmt.Printf("\n══════════════ %s ══════════════\n", ticker)

		fmt.Println("── GetFundamentals ──")
		fund, err := provider.GetFundamentals(ctx, ticker)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			enc.Encode(fund) //nolint:errcheck
		}

		fmt.Println("── GetEarningsSurprises (last 4) ──")
		surprises, err := provider.GetEarningsSurprises(ctx, ticker, 4)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			enc.Encode(surprises) //nolint:errcheck
		}

		fmt.Println("── GetInsiderActivity ──")
		insider, err := provider.GetInsiderActivity(ctx, ticker)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			enc.Encode(insider) //nolint:errcheck
		}

		fmt.Println("── GetPrice + PctBelow52WeekHigh ──")
		price, err := marketProvider.GetPrice(ctx, ticker)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			pctBelow := "n/a"
			if fund != nil && fund.FiftyTwoWeekHigh > 0 {
				pctBelow = fmt.Sprintf("%.2f%%", (fund.FiftyTwoWeekHigh-price)/fund.FiftyTwoWeekHigh*100)
			}
			enc.Encode(struct { //nolint:errcheck
				CurrentPrice       float64 `json:"CurrentPrice"`
				FiftyTwoWeekHigh   float64 `json:"FiftyTwoWeekHigh,omitempty"`
				PctBelow52WeekHigh string  `json:"PctBelow52WeekHigh"`
			}{
				CurrentPrice:       price,
				FiftyTwoWeekHigh:   func() float64 {
					if fund != nil { return fund.FiftyTwoWeekHigh }
					return 0
				}(),
				PctBelow52WeekHigh: pctBelow,
			})
		}
	}
}
