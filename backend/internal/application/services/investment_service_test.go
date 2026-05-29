// application/services/investment_service_test.go
//
// Execute is the money path: it routes each allocation to a brokerage, places
// orders, tolerates partial failures, and persists a decision record. These
// tests pin the routing, the sub-$1 skip, the partial-failure tolerance, the
// FilledPrice backfill, and the not-connected guard — all without a real broker.
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

func newInvestmentServiceForTest(bf *fakeBrokerageFactory, pr *fakeProfileRepo, dr *fakeDecisionRepo) *InvestmentService {
	return NewInvestmentService(bf, pr, dr, &fakeMarketData{snapshotErr: errors.New("snapshot unavailable")})
}

// defaultConn is a single connection that handles every asset category.
func defaultConn() []models.BrokerageConnection {
	return []models.BrokerageConnection{{
		ID:              "default",
		Name:            "Main",
		AssetCategories: []models.AssetCategory{models.AssetCategoryDefault},
		Connected:       true,
	}}
}

func TestExecute_NoBrokerageConnected(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = nil // no connections
	svc := newInvestmentServiceForTest(&fakeBrokerageFactory{}, pr, newFakeDecisionRepo())

	_, _, err := svc.Execute(context.Background(), "user-1",
		[]models.Allocation{{Ticker: "VTI", Amount: 100}}, 100, "moderate", "s", "", nil, "manual")

	if !errors.Is(err, ports.ErrBrokerageNotConnected) {
		t.Fatalf("expected ErrBrokerageNotConnected, got %v", err)
	}
}

func TestExecute_LoadConnectionsError(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connErr = errors.New("mongo timeout")
	svc := newInvestmentServiceForTest(&fakeBrokerageFactory{}, pr, newFakeDecisionRepo())

	_, _, err := svc.Execute(context.Background(), "user-1",
		[]models.Allocation{{Ticker: "VTI", Amount: 100}}, 100, "moderate", "s", "", nil, "manual")
	if err == nil {
		t.Fatal("expected error when loading connections fails")
	}
}

func TestExecute_PlacesOrdersAndPersistsDecision(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = defaultConn()
	provider := &fakeBrokerageProvider{}
	bf := &fakeBrokerageFactory{provider: provider}
	dr := newFakeDecisionRepo()

	svc := newInvestmentServiceForTest(bf, pr, dr)

	allocs := []models.Allocation{
		{Ticker: "VTI", Amount: 60, Reasoning: "core"},
		{Ticker: "BND", Amount: 40},
	}
	receipts, _, err := svc.Execute(context.Background(), "user-1", allocs, 100, "moderate", "summary", "thesis", nil, "manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("expected 2 receipts, got %d", len(receipts))
	}
	for _, r := range receipts {
		if r.BrokerageID != "default" || r.BrokerageName != "Main" {
			t.Errorf("receipt %s missing brokerage tags: %+v", r.Ticker, r)
		}
	}
	if dr.saved == nil {
		t.Fatal("decision was not persisted")
	}
	if dr.saved.TotalAmount != 100 || dr.saved.OverallReasoning != "thesis" || dr.saved.DecisionType != "invest" {
		t.Errorf("persisted decision fields wrong: %+v", dr.saved)
	}
	// Only VTI carried Reasoning, so TickerReasoning should have exactly one entry.
	if got := dr.saved.TickerReasoning; len(got) != 1 || got["VTI"] != "core" {
		t.Errorf("TickerReasoning = %+v, want {VTI:core}", got)
	}
}

func TestExecute_SkipsSubDollarAllocations(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = defaultConn()
	provider := &fakeBrokerageProvider{}
	bf := &fakeBrokerageFactory{provider: provider}

	svc := newInvestmentServiceForTest(bf, pr, newFakeDecisionRepo())

	allocs := []models.Allocation{
		{Ticker: "VTI", Amount: 0.50}, // below $1.00 minimum → skipped
		{Ticker: "BND", Amount: 5.00},
	}
	receipts, _, err := svc.Execute(context.Background(), "user-1", allocs, 5.5, "moderate", "s", "", nil, "manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(provider.placedOrders) != 1 || provider.placedOrders[0].Ticker != "BND" {
		t.Errorf("expected only BND ordered, got %+v", provider.placedOrders)
	}
	if len(receipts) != 1 {
		t.Errorf("expected 1 receipt, got %d", len(receipts))
	}
}

func TestExecute_PartialFailureStillReturnsSuccessfulReceipts(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = defaultConn()
	// VTI fails, BND succeeds.
	provider := &fakeBrokerageProvider{
		placeFn: func(o models.TradeOrder) (*models.TradeReceipt, error) {
			if o.Ticker == "VTI" {
				return nil, errors.New("insufficient buying power")
			}
			return &models.TradeReceipt{OrderID: "ok", Ticker: o.Ticker, FilledAmount: o.Amount, FilledPrice: 50, Status: "filled"}, nil
		},
	}
	bf := &fakeBrokerageFactory{provider: provider}

	svc := newInvestmentServiceForTest(bf, pr, newFakeDecisionRepo())
	receipts, _, err := svc.Execute(context.Background(), "user-1",
		[]models.Allocation{{Ticker: "VTI", Amount: 50}, {Ticker: "BND", Amount: 50}}, 100, "moderate", "s", "", nil, "manual")
	if err != nil {
		t.Fatalf("partial failure must not be fatal, got: %v", err)
	}
	if len(receipts) != 1 || receipts[0].Ticker != "BND" {
		t.Errorf("expected only BND receipt, got %+v", receipts)
	}
}

func TestExecute_BackfillsFilledPriceWhenZero(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = defaultConn()
	// Broker fills async: FilledPrice 0, but GetCurrentPrice returns 123.45.
	provider := &fakeBrokerageProvider{
		placeFn: func(o models.TradeOrder) (*models.TradeReceipt, error) {
			return &models.TradeReceipt{OrderID: "ok", Ticker: o.Ticker, FilledAmount: o.Amount, FilledPrice: 0, Status: "accepted"}, nil
		},
		price: 123.45,
	}
	bf := &fakeBrokerageFactory{provider: provider}

	svc := newInvestmentServiceForTest(bf, pr, newFakeDecisionRepo())
	receipts, _, err := svc.Execute(context.Background(), "user-1",
		[]models.Allocation{{Ticker: "VTI", Amount: 100}}, 100, "moderate", "s", "", nil, "manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receipts) != 1 || receipts[0].FilledPrice != 123.45 {
		t.Errorf("expected FilledPrice backfilled to 123.45, got %+v", receipts)
	}
}

func TestExecute_PerAllocationOverrideRoutesToNamedConnection(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{
		{ID: "equity", Name: "Equity", AssetCategories: []models.AssetCategory{models.AssetCategoryEquity}, Connected: true},
		{ID: "bonds", Name: "Bonds", AssetCategories: []models.AssetCategory{models.AssetCategoryBond}, Connected: true},
	}
	provider := &fakeBrokerageProvider{}
	bf := &fakeBrokerageFactory{provider: provider}

	svc := newInvestmentServiceForTest(bf, pr, newFakeDecisionRepo())

	// VTI is an equity by type, but the user pins it to the bonds connection.
	override := map[string]string{"VTI": "bonds"}
	receipts, _, err := svc.Execute(context.Background(), "user-1",
		[]models.Allocation{{Ticker: "VTI", Type: "etf", Amount: 100}}, 100, "moderate", "s", "", override, "manual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(receipts) != 1 || receipts[0].BrokerageID != "bonds" {
		t.Errorf("expected override to route VTI to bonds connection, got %+v", receipts)
	}
}
