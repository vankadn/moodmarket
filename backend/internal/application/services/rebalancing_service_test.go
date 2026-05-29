// application/services/rebalancing_service_test.go
//
// CheckDrift is the core of the rebalancing alert system: it compares live
// brokerage weights against (1) the targets from Claude's last recommendation
// and (2) the user's hard asset-class limits, emitting a TickerDrift for each
// breach past the threshold. These tests pin the maths and the early-exit
// guards that decide whether an alert fires at all.
package services

import (
	"context"
	"errors"
	"testing"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// driftByTicker indexes a drift slice for order-independent assertions.
func driftByTicker(drifts []models.TickerDrift) map[string]models.TickerDrift {
	m := make(map[string]models.TickerDrift, len(drifts))
	for _, d := range drifts {
		m[d.Ticker] = d
	}
	return m
}

// newRebalancingServiceForTest wires the service with a fixed threshold so tests
// don't depend on REBALANCE_DRIFT_THRESHOLD being set in the environment.
func newRebalancingServiceForTest(dr *fakeDecisionRepo, pr *fakeProfileRepo, bf *fakeBrokerageFactory, threshold float64) *RebalancingService {
	svc := NewRebalancingService(dr, pr, bf)
	svc.threshold = threshold
	return svc
}

func decisionWithAllocations(allocs ...models.Allocation) models.InvestmentDecision {
	return models.InvestmentDecision{Allocations: allocs}
}

func TestCheckDrift_EarlyExits(t *testing.T) {
	t.Parallel()

	connected := []models.BrokerageConnection{{ID: "a", Connected: true}}

	cases := []struct {
		name      string
		decisions []models.InvestmentDecision
		listErr   error
		conns     []models.BrokerageConnection
		connErr   error
		positions []models.Position
		posErr    error
		wantErr   bool
		wantNil   bool
	}{
		{
			name:    "list_decisions_error_is_propagated",
			listErr: errors.New("mongo down"),
			wantErr: true,
		},
		{
			name:      "no_prior_decision_returns_nil",
			decisions: nil,
			wantNil:   true,
		},
		{
			name:      "decision_with_no_allocations_returns_nil",
			decisions: []models.InvestmentDecision{decisionWithAllocations()},
			wantNil:   true,
		},
		{
			name:      "no_brokerage_connection_returns_nil",
			decisions: []models.InvestmentDecision{decisionWithAllocations(models.Allocation{Ticker: "VTI", Percentage: 100})},
			conns:     nil,
			wantNil:   true,
		},
		{
			name:      "connection_error_returns_nil_not_error",
			decisions: []models.InvestmentDecision{decisionWithAllocations(models.Allocation{Ticker: "VTI", Percentage: 100})},
			connErr:   errors.New("decrypt failed"),
			wantNil:   true,
		},
		{
			name:      "no_positions_returns_nil",
			decisions: []models.InvestmentDecision{decisionWithAllocations(models.Allocation{Ticker: "VTI", Percentage: 100})},
			conns:     connected,
			positions: nil,
			wantNil:   true,
		},
		{
			name:      "get_positions_error_is_propagated",
			decisions: []models.InvestmentDecision{decisionWithAllocations(models.Allocation{Ticker: "VTI", Percentage: 100})},
			conns:     connected,
			posErr:    errors.New("alpaca 500"),
			wantErr:   true,
		},
		{
			name:      "zero_total_market_value_returns_nil",
			decisions: []models.InvestmentDecision{decisionWithAllocations(models.Allocation{Ticker: "VTI", Percentage: 100})},
			conns:     connected,
			positions: []models.Position{{Ticker: "VTI", MarketValue: 0}},
			wantNil:   true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dr := newFakeDecisionRepo()
			dr.decisions = tc.decisions
			dr.listErr = tc.listErr

			pr := newFakeProfileRepo()
			pr.connections = tc.conns
			pr.connErr = tc.connErr

			bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: tc.positions, positionsErr: tc.posErr}}

			svc := newRebalancingServiceForTest(dr, pr, bf, 10)
			got, err := svc.CheckDrift(context.Background(), "user-1")

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil && got != nil {
				t.Errorf("expected nil drifts, got %+v", got)
			}
		})
	}
}

func TestCheckDrift_TickerLevel(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}

	dr := newFakeDecisionRepo()
	// Targets: VTI 50%, BND 50%.
	dr.decisions = []models.InvestmentDecision{decisionWithAllocations(
		models.Allocation{Ticker: "VTI", Percentage: 50},
		models.Allocation{Ticker: "BND", Percentage: 50},
	)}

	// Actual: VTI 70%, BND 30% → ±20pp drift, threshold 10 → both breach.
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{
		{Ticker: "VTI", MarketValue: 70},
		{Ticker: "BND", MarketValue: 30},
	}}}

	svc := newRebalancingServiceForTest(dr, pr, bf, 10)
	got, err := svc.CheckDrift(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byTicker := driftByTicker(got)
	if len(byTicker) != 2 {
		t.Fatalf("expected 2 drifts, got %d (%+v)", len(byTicker), got)
	}
	vti := byTicker["VTI"]
	if vti.TargetPct != 50 || vti.ActualPct != 70 || vti.DriftPct != 20 {
		t.Errorf("VTI drift = %+v, want target=50 actual=70 drift=20", vti)
	}
	bnd := byTicker["BND"]
	if bnd.DriftPct != -20 {
		t.Errorf("BND DriftPct = %.1f, want -20 (underweight)", bnd.DriftPct)
	}
}

func TestCheckDrift_WithinThresholdEmitsNothing(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}

	dr := newFakeDecisionRepo()
	dr.decisions = []models.InvestmentDecision{decisionWithAllocations(
		models.Allocation{Ticker: "VTI", Percentage: 50},
		models.Allocation{Ticker: "BND", Percentage: 50},
	)}
	// Actual 55/45 → 5pp drift, below threshold of 10.
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{
		{Ticker: "VTI", MarketValue: 55},
		{Ticker: "BND", MarketValue: 45},
	}}}

	svc := newRebalancingServiceForTest(dr, pr, bf, 10)
	got, err := svc.CheckDrift(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no drifts within threshold, got %+v", got)
	}
}

func TestCheckDrift_TargetDerivedFromAmountWhenPercentageUnset(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}

	dr := newFakeDecisionRepo()
	// No Percentage set; amounts 250/750 → derived targets 25%/75%.
	dr.decisions = []models.InvestmentDecision{decisionWithAllocations(
		models.Allocation{Ticker: "VTI", Amount: 250},
		models.Allocation{Ticker: "BND", Amount: 750},
	)}
	// Actual 50/50. VTI: 50-25=25 breach; BND: 50-75=-25 breach.
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{
		{Ticker: "VTI", MarketValue: 50},
		{Ticker: "BND", MarketValue: 50},
	}}}

	svc := newRebalancingServiceForTest(dr, pr, bf, 10)
	got, err := svc.CheckDrift(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byTicker := driftByTicker(got)
	if vti := byTicker["VTI"]; vti.TargetPct != 25 {
		t.Errorf("VTI TargetPct derived = %.1f, want 25", vti.TargetPct)
	}
	if bnd := byTicker["BND"]; bnd.TargetPct != 75 {
		t.Errorf("BND TargetPct derived = %.1f, want 75", bnd.TargetPct)
	}
}

func TestCheckDrift_AssetClassLimits(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}
	pr.profile = &models.UserProfile{
		AllocationPreferences: &models.AllocationPreferences{
			AssetClassLimits: []models.AssetClassLimit{
				{AssetClass: "Crypto", MaxPct: 20},     // breached: actual 40
				{AssetClass: "Bonds", MinPct: 30},      // breached: actual 0
				{AssetClass: "US Equity", MaxPct: 100}, // not breached
			},
		},
	}

	dr := newFakeDecisionRepo()
	// Targets match actual exactly so NO ticker-level drift fires — isolating the
	// asset-class checks. asset_class mapping comes from the decision allocations.
	dr.decisions = []models.InvestmentDecision{decisionWithAllocations(
		models.Allocation{Ticker: "BTC", AssetClass: "Crypto", Percentage: 40},
		models.Allocation{Ticker: "VTI", AssetClass: "US Equity", Percentage: 60},
	)}
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{
		{Ticker: "BTC", MarketValue: 40},
		{Ticker: "VTI", MarketValue: 60},
	}}}

	svc := newRebalancingServiceForTest(dr, pr, bf, 10)
	got, err := svc.CheckDrift(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	byTicker := driftByTicker(got)
	crypto, ok := byTicker["Crypto (asset class)"]
	if !ok {
		t.Fatalf("expected Crypto asset-class breach, got %+v", got)
	}
	if crypto.TargetPct != 20 || crypto.ActualPct != 40 || crypto.DriftPct != 20 {
		t.Errorf("Crypto breach = %+v, want target=20 actual=40 drift=20", crypto)
	}
	bonds, ok := byTicker["Bonds (asset class)"]
	if !ok {
		t.Fatalf("expected Bonds min breach, got %+v", got)
	}
	if bonds.TargetPct != 30 || bonds.ActualPct != 0 || bonds.DriftPct != -30 {
		t.Errorf("Bonds breach = %+v, want target=30 actual=0 drift=-30", bonds)
	}
	if _, ok := byTicker["US Equity (asset class)"]; ok {
		t.Error("US Equity is within its limit and must not produce a drift")
	}
}

func TestCheckDrift_ProfileErrorSkipsAssetClassButKeepsTickerDrift(t *testing.T) {
	t.Parallel()

	pr := newFakeProfileRepo()
	pr.connections = []models.BrokerageConnection{{ID: "a", Connected: true}}
	pr.profileErr = errors.New("profile read failed")

	dr := newFakeDecisionRepo()
	dr.decisions = []models.InvestmentDecision{decisionWithAllocations(
		models.Allocation{Ticker: "VTI", AssetClass: "US Equity", Percentage: 50},
		models.Allocation{Ticker: "BND", AssetClass: "Bonds", Percentage: 50},
	)}
	bf := &fakeBrokerageFactory{provider: &fakeBrokerageProvider{positions: []models.Position{
		{Ticker: "VTI", MarketValue: 80},
		{Ticker: "BND", MarketValue: 20},
	}}}

	svc := newRebalancingServiceForTest(dr, pr, bf, 10)
	got, err := svc.CheckDrift(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("profile error must be non-fatal, got: %v", err)
	}
	// Ticker-level drift still reported; no asset-class entries.
	for _, d := range got {
		if d.Ticker == "Bonds (asset class)" || d.Ticker == "US Equity (asset class)" {
			t.Errorf("asset-class drift must be skipped when profile read fails, got %+v", d)
		}
	}
	if len(got) != 2 {
		t.Errorf("expected 2 ticker drifts, got %d (%+v)", len(got), got)
	}
}
