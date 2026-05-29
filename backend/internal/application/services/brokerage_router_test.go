// services/brokerage_router_test.go
//
// Routing decides which connected brokerage account each allocation is placed
// against. A wrong answer means a trade executed on the wrong account, so the
// pure routing logic is covered here exhaustively.
package services

import (
	"testing"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

func TestNormalizeAssetCategory(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		allocType string
		want      models.AssetCategory
	}{
		{"bond", "bond", models.AssetCategoryBond},
		{"bonds_plural", "bonds", models.AssetCategoryBond},
		{"bond_etf", "bond etf", models.AssetCategoryBond},
		{"fixed_income", "fixed income", models.AssetCategoryBond},
		{"treasury", "treasury", models.AssetCategoryBond},
		{"tips", "tips", models.AssetCategoryBond},
		{"municipal_bond", "municipal bond", models.AssetCategoryBond},
		{"high_yield_bond", "high yield bond", models.AssetCategoryBond},
		{"uppercase_is_normalized", "BOND", models.AssetCategoryBond},
		{"mixed_case_with_spaces", "  Fixed Income  ", models.AssetCategoryBond},
		{"stock_falls_through_to_equity", "stock", models.AssetCategoryEquity},
		{"etf_falls_through_to_equity", "etf", models.AssetCategoryEquity},
		{"reit_falls_through_to_equity", "reit", models.AssetCategoryEquity},
		{"crypto_etf_falls_through_to_equity", "crypto etf", models.AssetCategoryEquity},
		{"unknown_defaults_to_equity", "something-novel", models.AssetCategoryEquity},
		{"empty_defaults_to_equity", "", models.AssetCategoryEquity},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeAssetCategory(tc.allocType); got != tc.want {
				t.Errorf("NormalizeAssetCategory(%q) = %q, want %q", tc.allocType, got, tc.want)
			}
		})
	}
}

func TestRouteAllocation(t *testing.T) {
	t.Parallel()

	equityConn := models.BrokerageConnection{
		ID:              "equity-acct",
		AssetCategories: []models.AssetCategory{models.AssetCategoryEquity},
		Connected:       true,
	}
	bondConn := models.BrokerageConnection{
		ID:              "bond-acct",
		AssetCategories: []models.AssetCategory{models.AssetCategoryBond},
		Connected:       true,
	}
	defaultConn := models.BrokerageConnection{
		ID:              "default-acct",
		AssetCategories: []models.AssetCategory{models.AssetCategoryDefault},
		Connected:       true,
	}
	disconnectedEquity := models.BrokerageConnection{
		ID:              "disconnected-equity",
		AssetCategories: []models.AssetCategory{models.AssetCategoryEquity},
		Connected:       false,
	}

	cases := []struct {
		name        string
		alloc       models.Allocation
		connections []models.BrokerageConnection
		wantID      string // "" means expect nil
	}{
		{
			name:        "exact_category_match_wins",
			alloc:       models.Allocation{Type: "stock"},
			connections: []models.BrokerageConnection{bondConn, equityConn},
			wantID:      "equity-acct",
		},
		{
			name:        "bond_routes_to_bond_account",
			alloc:       models.Allocation{Type: "treasury"},
			connections: []models.BrokerageConnection{equityConn, bondConn},
			wantID:      "bond-acct",
		},
		{
			name:        "falls_back_to_default_when_no_exact_match",
			alloc:       models.Allocation{Type: "bond"},
			connections: []models.BrokerageConnection{equityConn, defaultConn},
			wantID:      "default-acct",
		},
		{
			name:        "exact_match_preferred_over_default",
			alloc:       models.Allocation{Type: "bond"},
			connections: []models.BrokerageConnection{defaultConn, bondConn},
			wantID:      "bond-acct",
		},
		{
			name:        "disconnected_connection_is_skipped",
			alloc:       models.Allocation{Type: "stock"},
			connections: []models.BrokerageConnection{disconnectedEquity},
			wantID:      "",
		},
		{
			name:        "disconnected_exact_falls_back_to_connected_default",
			alloc:       models.Allocation{Type: "stock"},
			connections: []models.BrokerageConnection{disconnectedEquity, defaultConn},
			wantID:      "default-acct",
		},
		{
			name:        "no_matching_or_default_returns_nil",
			alloc:       models.Allocation{Type: "stock"},
			connections: []models.BrokerageConnection{bondConn},
			wantID:      "",
		},
		{
			name:        "empty_connections_returns_nil",
			alloc:       models.Allocation{Type: "stock"},
			connections: nil,
			wantID:      "",
		},
		{
			name: "first_connected_default_is_kept_when_multiple_defaults",
			alloc: models.Allocation{Type: "stock"},
			connections: []models.BrokerageConnection{
				defaultConn,
				{ID: "second-default", AssetCategories: []models.AssetCategory{models.AssetCategoryDefault}, Connected: true},
			},
			wantID: "default-acct",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RouteAllocation(tc.alloc, tc.connections)
			if tc.wantID == "" {
				if got != nil {
					t.Errorf("RouteAllocation() = %q, want nil", got.ID)
				}
				return
			}
			if got == nil {
				t.Fatalf("RouteAllocation() = nil, want %q", tc.wantID)
			}
			if got.ID != tc.wantID {
				t.Errorf("RouteAllocation() = %q, want %q", got.ID, tc.wantID)
			}
		})
	}
}
