// TECH DEBT: These are basic smoke tests covering critical prompt rules.
// A proper prompt test suite should test:
//   - Claude-level assertion: actual LLM output respects the 40% concentration rule
//   - Fuzz/property testing: allocations always sum to total_budget
//   - Regression tests: save known-good prompts and diff against them
//   - Diversity rule: back-to-back calls with same history don't produce identical splits
package advisor

import (
	"strings"
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// buildCase is a convenience wrapper so table rows stay readable.
type buildCase struct {
	name     string
	req      models.InvestmentRequest
	profile  *models.UserProfile
	snapshot *models.MarketSnapshot
	// assertions: each string must (or must not) appear in the output
	mustContain    []string
	mustNotContain []string
}

func TestBuildUserMessage(t *testing.T) {
	t.Parallel()

	baseReq := models.InvestmentRequest{BaseBudget: 100, ExtraMoney: 0}

	cases := []buildCase{
		{
			name:           "no_positions_shows_fallback",
			req:            baseReq,
			mustContain:    []string{"No existing positions"},
			mustNotContain: []string{"CURRENT BROKERAGE POSITIONS"},
		},
		{
			name: "positions_below_limit_no_warning",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				// VTI 30%, BND 35%, VXUS 35% — all below 40%
				Positions: []models.Position{
					{Ticker: "VTI", MarketValue: 300},
					{Ticker: "BND", MarketValue: 350},
					{Ticker: "VXUS", MarketValue: 350},
				},
			},
			mustContain:    []string{"CURRENT BROKERAGE POSITIONS", "VTI", "BND", "VXUS"},
			mustNotContain: []string{"concentration limit"},
		},
		{
			name: "position_at_40pct_gets_warning",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				Positions: []models.Position{
					{Ticker: "NVDA", MarketValue: 400},
					{Ticker: "VTI", MarketValue: 600},
				},
			},
			// NVDA is exactly 40% — warning fires at >= 40
			mustContain: []string{"concentration limit", "NVDA"},
		},
		{
			name: "position_above_40pct_gets_warning",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				Positions: []models.Position{
					{Ticker: "AAPL", MarketValue: 900},
					{Ticker: "VTI", MarketValue: 100},
				},
			},
			mustContain: []string{"concentration limit", "AAPL"},
		},
		{
			name:           "no_history_omits_section",
			req:            baseReq,
			mustNotContain: []string{"RECENT INVESTMENT HISTORY"},
		},
		{
			name: "history_present_shows_section_and_diversity_instruction",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				RecentDecisions: []models.InvestmentDecision{
					{
						Timestamp:   time.Now().AddDate(0, 0, -1),
						TotalAmount: 100,
						Allocations: []models.Allocation{
							{Ticker: "VTI", Percentage: 60},
							{Ticker: "BND", Percentage: 40},
						},
					},
				},
			},
			mustContain: []string{"RECENT INVESTMENT HISTORY", "Vary today's allocation", "VTI", "BND"},
		},
		{
			name:           "no_balance_summary_shows_fallback",
			req:            baseReq,
			mustContain:    []string{"No bank accounts connected"},
			mustNotContain: []string{"CONNECTED FINANCIAL ACCOUNTS"},
		},
		{
			name: "balance_summary_shows_live_section",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				BalanceSummary: &models.BalanceSummary{
					TotalCash:    5000,
					AccountCount: 2,
					PulledAt:     time.Now(),
				},
			},
			mustContain:    []string{"CONNECTED FINANCIAL ACCOUNTS", "5000"},
			mustNotContain: []string{"No bank accounts connected"},
		},
		{
			name: "no_profile_uses_balanced_defaults",
			req:  baseReq,
			mustContain: []string{"No profile on file", "balanced moderate"},
		},
		{
			name: "profile_included_in_message",
			req:  baseReq,
			profile: &models.UserProfile{
				FullName:      "Test User",
				RiskTolerance: models.RiskTolerance("aggressive"),
				TimeHorizon:   models.TimeHorizon("long"),
			},
			mustContain:    []string{"Test User", "aggressive", "long"},
			mustNotContain: []string{"No profile on file"},
		},
		{
			name: "market_snapshot_included",
			req:  baseReq,
			snapshot: &models.MarketSnapshot{
				Date:             "2026-05-08",
				SPYChangePercent: 1.23,
				MarketSentiment:  "bullish",
			},
			mustContain: []string{"2026-05-08", "bullish", "SPY"},
		},
		{
			name:           "total_budget_reflects_base_plus_extra",
			req:            models.InvestmentRequest{BaseBudget: 75, ExtraMoney: 25},
			mustContain:    []string{"100.00"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			msg := buildUserMessage(tc.req, tc.profile, tc.snapshot)

			for _, want := range tc.mustContain {
				if !strings.Contains(msg, want) {
					t.Errorf("expected %q to contain %q\nmessage:\n%s", tc.name, want, msg)
				}
			}
			for _, notWant := range tc.mustNotContain {
				if strings.Contains(msg, notWant) {
					t.Errorf("expected %q NOT to contain %q\nmessage:\n%s", tc.name, notWant, msg)
				}
			}
		})
	}
}
