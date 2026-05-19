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
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// stubClassifier satisfies ports.Classifier for tests without hitting Mongo.
type stubClassifier struct {
	m map[string]string
}

func (s stubClassifier) Classify(ticker string) (string, bool) {
	ac, ok := s.m[ticker]
	if !ok {
		return "Other", false
	}
	return ac, true
}

func (s stubClassifier) Store(_, _ string) {}

// buildCase is a convenience wrapper so table rows stay readable.
type buildCase struct {
	name       string
	req        models.InvestmentRequest
	profile    *models.UserProfile
	snapshot   *models.MarketSnapshot
	classifier stubClassifier
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
		{
			name:           "no_transactions_omits_section",
			req:            baseReq,
			mustNotContain: []string{"SPENDING CONTEXT"},
		},
		{
			name: "spending_context_omitted_without_opt_in",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				TransactionSummary: &models.TransactionSummary{
					SpendLast7Days:  342.50,
					SpendLast30Days: 1240.00,
				},
			},
			// IncludeCashContext defaults to false — section must not appear
			mustNotContain: []string{"SPENDING CONTEXT"},
		},
		{
			name: "spending_context_shown_when_opted_in",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				TransactionSummary: &models.TransactionSummary{
					SpendLast7Days:  342.50,
					SpendLast30Days: 1240.00,
				},
			},
			profile: &models.UserProfile{IncludeCashContext: true},
			mustContain:    []string{"SPENDING CONTEXT", "background context only", "342.50", "1240.00"},
			mustNotContain: []string{"SPENDING HISTORY"},
		},
		{
			name: "spending_context_with_runway_when_opted_in",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				BalanceSummary: &models.BalanceSummary{
					TotalCash:    4200,
					AccountCount: 1,
					PulledAt:     time.Now(),
				},
				TransactionSummary: &models.TransactionSummary{
					SpendLast7Days:  350,
					SpendLast30Days: 1500, // ~$50/day → runway ~84 days
				},
			},
			profile:     &models.UserProfile{IncludeCashContext: true},
			mustContain: []string{"SPENDING CONTEXT", "Cash runway", "84"},
		},
		{
			name: "spending_context_omitted_no_profile",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				TransactionSummary: &models.TransactionSummary{
					SpendLast7Days: 200, SpendLast30Days: 600,
				},
			},
			// nil profile — can't opt in, section must not appear
			mustNotContain: []string{"SPENDING CONTEXT"},
		},
		{
			// Phase 10: news is no longer pre-fetched into the prompt.
			// Claude calls get_market_news via tool use during the conversation.
			name: "news_absent_from_prompt_claude_fetches_via_tool",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				NewsItems: []models.NewsItem{
					{Headline: "Fed holds rates steady", Source: "Reuters"},
				},
			},
			mustNotContain: []string{"TODAY'S MARKET NEWS"},
		},

		// Phase 24 — Performance feedback section

		{
			name: "performance_summary_absent_for_new_user_with_no_verdicts",
			// New users have no verdicted decisions yet.
			// The section must be absent so the prompt is unchanged from the pre-Phase-24 baseline.
			req:            models.InvestmentRequest{BaseBudget: 100},
			mustNotContain: []string{"PAST PERFORMANCE"},
		},
		{
			name: "performance_summary_absent_when_performance_summary_is_nil",
			// RecommendationService sets PerformanceSummary=nil when verdict count < threshold.
			// The prompt must not include the section regardless of what the eval API returned.
			req: models.InvestmentRequest{
				BaseBudget:         100,
				PerformanceSummary: nil,
			},
			mustNotContain: []string{"PAST PERFORMANCE"},
		},
		{
			name: "performance_summary_present_with_win_rate_and_avg_return_when_threshold_met",
			// When PerformanceSummary is set (service guarantees >= 5 verdicts),
			// the section must appear so Claude can calibrate whether its strategy is working.
			req: models.InvestmentRequest{
				BaseBudget: 100,
				PerformanceSummary: &models.EvalSummary{
					VerdictedDecisions: 14,
					WinRate:            0.71,
					AvgReturnPct:       2.3,
					AvgSPYReturnPct:    1.1,
					BestDecision:       &models.EvalDecisionRef{ReturnPct: 8.2},
					WorstDecision:      &models.EvalDecisionRef{ReturnPct: -4.1},
				},
			},
			mustContain: []string{
				"PAST PERFORMANCE",
				"14 evaluated decisions",
				"71%",
				"+2.3%",
				"+1.1%",
			},
		},
		{
			name: "performance_summary_instruction_says_context_only_not_directive",
			// The feedback section must explicitly say it is non-directive.
			// Without this guard, Claude might override the user's stated risk tolerance
			// based on past returns — the same mistake Phase 8ca fixed for spending context.
			req: models.InvestmentRequest{
				BaseBudget: 100,
				PerformanceSummary: &models.EvalSummary{
					VerdictedDecisions: 7,
					WinRate:            0.43,
					AvgReturnPct:       -0.5,
					AvgSPYReturnPct:    1.2,
				},
			},
			mustContain: []string{"context only", "do not override"},
		},

		// Concentration block tests

		{
			name: "concentration_block_absent_without_classifier",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				Positions: []models.Position{
					{Ticker: "VTI", MarketValue: 600},
					{Ticker: "BND", MarketValue: 400},
				},
			},
			// nil classifier → concentration block must not appear
			mustNotContain: []string{"CURRENT PORTFOLIO CONCENTRATION"},
		},
		{
			name: "concentration_block_groups_by_asset_class",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				Positions: []models.Position{
					{Ticker: "VTI", MarketValue: 500},
					{Ticker: "QQQ", MarketValue: 300},
					{Ticker: "BND", MarketValue: 200},
				},
			},
			classifier: stubClassifier{m: map[string]string{
				"VTI": "US Equity",
				"QQQ": "US Equity",
				"BND": "Bonds",
			}},
			mustContain: []string{
				"CURRENT PORTFOLIO CONCENTRATION",
				"US Equity",
				"Bonds",
				"VTI",
				"QQQ",
				"BND",
			},
		},
		{
			name: "concentration_block_sorts_classes_by_pct_descending",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				Positions: []models.Position{
					{Ticker: "BND", MarketValue: 100},
					{Ticker: "VTI", MarketValue: 900},
				},
			},
			classifier: stubClassifier{m: map[string]string{
				"VTI": "US Equity",
				"BND": "Bonds",
			}},
			// US Equity (90%) must appear before Bonds (10%)
			mustContain: []string{"US Equity: 90%", "Bonds: 10%"},
		},
		{
			name: "concentration_block_unknown_ticker_goes_to_other",
			req: models.InvestmentRequest{
				BaseBudget: 100,
				Positions: []models.Position{
					{Ticker: "UNKNOWN", MarketValue: 1000},
				},
			},
			classifier: stubClassifier{m: map[string]string{}},
			mustContain: []string{"Other: 100%", "UNKNOWN"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cl ports.Classifier
			if tc.classifier.m != nil {
				cl = tc.classifier
			}
			msg := buildUserMessage(tc.req, tc.profile, tc.snapshot, cl)

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
