// application/services/rebalance_aggregation_service_test.go
//
// BuildRequest merges live Alpaca positions with SnapTrade external holdings
// (Alpaca-first dedupe) and enriches them with buy reasoning and first-purchase
// dates mined from decision history. These tests pin the dedupe precedence, the
// per-account tagging, the newest-wins reasoning rule, and the oldest-wins
// first-purchase rule.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

func newAggregationServiceForTest(bf *fakeBrokerageFactory, pr *fakeProfileRepo, dr *fakeDecisionRepo, pa *fakePortfolioAggregator) *RebalanceAggregationService {
	return NewRebalanceAggregationService(bf, pr, dr, pa)
}

func positionsByTicker(positions []models.RebalancePosition) map[string]models.RebalancePosition {
	m := make(map[string]models.RebalancePosition, len(positions))
	for _, p := range positions {
		m[p.Ticker] = p
	}
	return m
}

func TestBuildRequest_LoadConnectionsError(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connErr = errors.New("mongo down")
	svc := newAggregationServiceForTest(&fakeBrokerageFactory{}, pr, newFakeDecisionRepo(), newFakePortfolioAggregator())

	if _, err := svc.BuildRequest(context.Background(), "user-1"); err == nil {
		t.Fatal("expected error when brokerage connections fail to load")
	}
}

func TestBuildRequest_AlpacaFirstDeduplication(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}
	pr.portfolioConn = &models.PortfolioConnection{ProviderUserID: "u", ProviderUserSecret: "s"}

	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{
		{Ticker: "VTI", MarketValue: 1000},
	}}}

	pa := newFakePortfolioAggregator()
	pa.byAccount = map[string][]models.Position{
		"Robinhood": {
			{Ticker: "VTI", MarketValue: 999}, // dup of Alpaca → dropped
			{Ticker: "AAPL", MarketValue: 500}, // new → kept, tagged Robinhood
		},
	}

	dr := newFakeDecisionRepo()
	svc := newAggregationServiceForTest(bf, pr, dr, pa)

	req, err := svc.BuildRequest(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byTicker := positionsByTicker(req.Positions)
	if len(byTicker) != 2 {
		t.Fatalf("expected 2 deduped positions, got %d (%+v)", len(byTicker), req.Positions)
	}
	if vti := byTicker["VTI"]; vti.Source != "alpaca" || vti.MarketValue != 1000 {
		t.Errorf("VTI should be the Alpaca copy (source=alpaca, value=1000), got %+v", vti)
	}
	if aapl := byTicker["AAPL"]; aapl.Source != "snaptrade" || aapl.AccountName != "Robinhood" {
		t.Errorf("AAPL should be tagged snaptrade/Robinhood, got %+v", aapl)
	}
}

func TestBuildRequest_BrokerageProviderErrorIsSkipped(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "broken", Connected: true}}
	// Provider build fails — that connection is skipped, no positions, no error.
	bf := &fakeBrokerageFactory{err: errors.New("decrypt failed")}

	svc := newAggregationServiceForTest(bf, pr, newFakeDecisionRepo(), newFakePortfolioAggregator())
	req, err := svc.BuildRequest(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("provider build failure must be non-fatal, got: %v", err)
	}
	if len(req.Positions) != 0 {
		t.Errorf("expected no positions, got %+v", req.Positions)
	}
}

func TestBuildRequest_ReasoningNewestWins_FirstPurchaseOldestWins(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{{Ticker: "VTI", MarketValue: 100}}}}

	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

	dr := newFakeDecisionRepo()
	// ListByUser returns newest-first (recent → mid → old); the service relies on
	// this ordering to take the newest reasoning and the oldest purchase date.
	dr.decisions = []models.InvestmentDecision{
		{
			Timestamp:       recent,
			TickerReasoning: map[string]string{"VTI": "newest reasoning"},
			Receipts:        []models.TradeReceipt{{Ticker: "VTI", Timestamp: recent}},
		},
		{
			Timestamp: mid,
			Receipts:  []models.TradeReceipt{{Ticker: "VTI", Timestamp: mid}},
		},
		{
			Timestamp:       old,
			TickerReasoning: map[string]string{"VTI": "oldest reasoning"},
			Receipts:        []models.TradeReceipt{{Ticker: "VTI", Timestamp: old}},
		},
	}

	svc := newAggregationServiceForTest(bf, pr, dr, newFakePortfolioAggregator())
	req, err := svc.BuildRequest(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.BuyReasoningByTicker["VTI"]; got != "newest reasoning" {
		t.Errorf("BuyReasoning = %q, want newest reasoning", got)
	}
	if got := req.FirstPurchaseByTicker["VTI"]; !got.Equal(old) {
		t.Errorf("FirstPurchase = %v, want oldest (%v)", got, old)
	}
}

func TestBuildRequest_FirstPurchaseFallsBackToDecisionTimestamp(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{{Ticker: "VTI", MarketValue: 100}}}}

	decTime := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	dr := newFakeDecisionRepo()
	dr.decisions = []models.InvestmentDecision{
		{
			Timestamp: decTime,
			Receipts:  []models.TradeReceipt{{Ticker: "VTI"}}, // zero receipt timestamp
		},
	}

	svc := newAggregationServiceForTest(bf, pr, dr, newFakePortfolioAggregator())
	req, err := svc.BuildRequest(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.FirstPurchaseByTicker["VTI"]; !got.Equal(decTime) {
		t.Errorf("FirstPurchase = %v, want decision timestamp %v", got, decTime)
	}
}
