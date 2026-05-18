// api/handlers/eval_handler_test.go
//
// Reading this file explains what the eval HTTP layer does:
// - safeFloat prevents json.Encoder from crashing on Infinity/NaN stored from old bugs
// - GET /users/eval/summary returns aggregated verdict stats from the repository
// - GET /users/eval/decisions returns a paginated list of verdicted decisions
// - Pagination defaults and limits are enforced at the handler level
//
// Each test name is a sentence stating a requirement.
package handlers

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// ---------- mock implementations ----------

// mockDecisionRepoForEval implements only the two methods EvalHandler uses.
// All other DecisionRepository methods panic — tests fail loudly if called unexpectedly.
type mockDecisionRepoForEval struct {
	summary      *models.EvalSummary
	summaryErr   error
	decisions    []models.InvestmentDecision
	decisionsErr error
	// capture what page/limit the handler passed
	capturedPage  int
	capturedLimit int
}

func (m *mockDecisionRepoForEval) GetEvalSummary(_ context.Context, _ string) (*models.EvalSummary, error) {
	return m.summary, m.summaryErr
}

func (m *mockDecisionRepoForEval) ListVerdictedDecisions(_ context.Context, _ string, page, limit int) ([]models.InvestmentDecision, error) {
	m.capturedPage = page
	m.capturedLimit = limit
	return m.decisions, m.decisionsErr
}

// Stub all other DecisionRepository methods — not used by EvalHandler.
func (m *mockDecisionRepoForEval) Save(_ context.Context, _ *models.InvestmentDecision) error {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) ListByUser(_ context.Context, _ string, _ int) ([]models.InvestmentDecision, error) {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) ListByUserSince(_ context.Context, _ string, _ *time.Time) ([]models.InvestmentDecision, error) {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) ActivityByStrategy(_ context.Context, _ string) ([]models.StrategyActivity, error) {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) CostBasisByStrategy(_ context.Context, _ string) (map[string]map[string]float64, error) {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) StampVerdict(_ context.Context, _ string, _ *models.DecisionVerdict) error {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) ListUnverdicted(_ context.Context, _ string, _ time.Duration) ([]models.InvestmentDecision, error) {
	panic("not used in eval handler tests")
}
func (m *mockDecisionRepoForEval) GetUsersWithPendingVerdicts(_ context.Context, _ time.Duration) ([]string, error) {
	panic("not used in eval handler tests")
}

// mockIdentityProvider returns a fixed user ID for any request.
type mockIdentityProvider struct{ userID string }

func (m *mockIdentityProvider) GetCurrentUser(_ context.Context) (string, error) {
	return m.userID, nil
}

// ---------- helpers ----------

func evalHandler(repo ports.DecisionRepository) *EvalHandler {
	return NewEvalHandler(&mockIdentityProvider{userID: "test-user"}, repo)
}

func getRequest(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	// Inject user identity into context the same way auth middleware does.
	ctx := ports.WithUserIdentity(r.Context(), &ports.UserIdentity{UserID: "test-user"})
	return r.WithContext(ctx)
}

// ---------- TestSafeFloat ----------

func TestSafeFloat(t *testing.T) {
	t.Parallel()

	// safeFloat exists because MongoDB can store +Infinity when a previous buggy verdict
	// computed (currentPrice - 0) / 0 * 100. json.Encoder panics on Infinity and returns
	// no bytes, causing "Unexpected end of JSON input" on the frontend.

	cases := []struct {
		name  string
		input float64
		want  float64
	}{
		{
			name:  "normal_finite_float_is_returned_unchanged",
			input: 3.14,
			want:  3.14,
		},
		{
			name:  "positive_infinity_is_replaced_with_zero",
			input: math.Inf(1),
			want:  0,
		},
		{
			name:  "negative_infinity_is_replaced_with_zero",
			input: math.Inf(-1),
			want:  0,
		},
		{
			name:  "nan_is_replaced_with_zero",
			input: math.NaN(),
			want:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := safeFloat(tc.input)
			if math.IsNaN(tc.want) {
				if !math.IsNaN(got) {
					t.Errorf("safeFloat(%v): got %v, want NaN", tc.input, got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("safeFloat(%v): got %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------- TestEvalSummaryHandler ----------

func TestEvalSummaryHandler(t *testing.T) {
	t.Parallel()

	t.Run("returns_win_rate_as_fraction_of_verdicted_decisions_that_beat_spy", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{
			summary: &models.EvalSummary{
				TotalDecisions:     10,
				VerdictedDecisions: 7,
				WinRate:            0.71,
			},
		}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/summary"))

		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", w.Code)
		}
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if got := resp["win_rate"].(float64); math.Abs(got-0.71) > 0.001 {
			t.Errorf("win_rate: got %f, want 0.71", got)
		}
		if got := resp["verdicted_decisions"].(float64); got != 7 {
			t.Errorf("verdicted_decisions: got %f, want 7", got)
		}
		if got := resp["total_decisions"].(float64); got != 10 {
			t.Errorf("total_decisions: got %f, want 10", got)
		}
	})

	t.Run("returns_avg_return_and_avg_spy_return_from_repository", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{
			summary: &models.EvalSummary{
				VerdictedDecisions: 5,
				AvgReturnPct:       2.3,
				AvgSPYReturnPct:    1.1,
			},
		}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/summary"))

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		if got := resp["avg_return_pct"].(float64); math.Abs(got-2.3) > 0.001 {
			t.Errorf("avg_return_pct: got %f, want 2.3", got)
		}
		if got := resp["avg_spy_return_pct"].(float64); math.Abs(got-1.1) > 0.001 {
			t.Errorf("avg_spy_return_pct: got %f, want 1.1", got)
		}
	})

	t.Run("returns_best_and_worst_decision_when_verdicts_exist", func(t *testing.T) {
		t.Parallel()
		best := &models.EvalDecisionRef{ID: "best-id", ReturnPct: 8.2, Amount: 100}
		worst := &models.EvalDecisionRef{ID: "worst-id", ReturnPct: -4.1, Amount: 100}
		repo := &mockDecisionRepoForEval{
			summary: &models.EvalSummary{
				VerdictedDecisions: 5,
				BestDecision:       best,
				WorstDecision:      worst,
			},
		}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/summary"))

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		bestResp := resp["best_decision"].(map[string]interface{})
		if bestResp["id"] != "best-id" {
			t.Errorf("best_decision.id: got %v, want best-id", bestResp["id"])
		}
		worstResp := resp["worst_decision"].(map[string]interface{})
		if worstResp["id"] != "worst-id" {
			t.Errorf("worst_decision.id: got %v, want worst-id", worstResp["id"])
		}
	})

	t.Run("returns_by_strategy_array_with_win_rate_per_config", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{
			summary: &models.EvalSummary{
				VerdictedDecisions: 10,
				ByStrategy: []models.StrategyEval{
					{ConfigID: "abc123", WinRate: 0.78, AvgReturnPct: 3.1, DecisionCount: 7},
					{ConfigID: "manual", WinRate: 0.50, AvgReturnPct: 0.5, DecisionCount: 3},
				},
			},
		}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/summary"))

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		strategies := resp["by_strategy"].([]interface{})
		if len(strategies) != 2 {
			t.Fatalf("by_strategy length: got %d, want 2", len(strategies))
		}
		first := strategies[0].(map[string]interface{})
		if first["config_id"] != "abc123" {
			t.Errorf("by_strategy[0].config_id: got %v, want abc123", first["config_id"])
		}
		if math.Abs(first["win_rate"].(float64)-0.78) > 0.001 {
			t.Errorf("by_strategy[0].win_rate: got %v, want 0.78", first["win_rate"])
		}
	})

	t.Run("returns_zero_verdicted_decisions_and_empty_by_strategy_when_no_verdicts_yet", func(t *testing.T) {
		t.Parallel()
		// New users with no verdicts yet should get a valid response, not a 500.
		repo := &mockDecisionRepoForEval{
			summary: &models.EvalSummary{
				TotalDecisions:     3,
				VerdictedDecisions: 0,
				ByStrategy:         []models.StrategyEval{},
			},
		}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/summary"))

		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", w.Code)
		}
		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		if got := resp["verdicted_decisions"].(float64); got != 0 {
			t.Errorf("verdicted_decisions: got %f, want 0", got)
		}
		strategies := resp["by_strategy"].([]interface{})
		if len(strategies) != 0 {
			t.Errorf("by_strategy should be empty, got %d items", len(strategies))
		}
	})

	t.Run("returns_405_for_non_GET_requests", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{}
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/users/eval/summary", nil)
		ctx := ports.WithUserIdentity(r.Context(), &ports.UserIdentity{UserID: "test-user"})
		evalHandler(repo).ServeHTTP(w, r.WithContext(ctx))

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("status: got %d, want 405", w.Code)
		}
	})
}

// ---------- TestEvalDecisionsHandler ----------

func TestEvalDecisionsHandler(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("returns_list_of_decisions_with_verdict_embedded", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{
			decisions: []models.InvestmentDecision{
				{
					ID:          "dec-1",
					Timestamp:   now,
					TotalAmount: 100,
					RiskLevel:   "moderate",
					Verdict: &models.DecisionVerdict{
						StampedAt:        now,
						OverallReturnPct: 2.5,
						SPYReturnPct:     1.1,
						BeatMarket:       true,
						TickerVerdicts: []models.TickerVerdict{
							{Ticker: "VTI", ReturnPct: 2.5},
						},
					},
				},
			},
		}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/decisions"))

		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", w.Code)
		}
		var items []map[string]interface{}
		json.NewDecoder(w.Body).Decode(&items)
		if len(items) != 1 {
			t.Fatalf("decisions count: got %d, want 1", len(items))
		}
		if items[0]["id"] != "dec-1" {
			t.Errorf("decision id: got %v, want dec-1", items[0]["id"])
		}
		verdict := items[0]["verdict"].(map[string]interface{})
		if math.Abs(verdict["overall_return_pct"].(float64)-2.5) > 0.001 {
			t.Errorf("verdict.overall_return_pct: got %v, want 2.5", verdict["overall_return_pct"])
		}
		if verdict["beat_market"] != true {
			t.Errorf("verdict.beat_market: got %v, want true", verdict["beat_market"])
		}
	})

	t.Run("returns_empty_array_when_no_verdicted_decisions_exist", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{decisions: []models.InvestmentDecision{}}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/decisions"))

		if w.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", w.Code)
		}
		var items []interface{}
		json.NewDecoder(w.Body).Decode(&items)
		if len(items) != 0 {
			t.Errorf("expected empty array, got %d items", len(items))
		}
	})

	t.Run("respects_page_and_limit_query_parameters", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{decisions: []models.InvestmentDecision{}}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/decisions?page=3&limit=50"))

		if repo.capturedPage != 3 {
			t.Errorf("page passed to repo: got %d, want 3", repo.capturedPage)
		}
		if repo.capturedLimit != 50 {
			t.Errorf("limit passed to repo: got %d, want 50", repo.capturedLimit)
		}
	})

	t.Run("defaults_to_page_1_and_limit_20_when_params_absent", func(t *testing.T) {
		t.Parallel()
		repo := &mockDecisionRepoForEval{decisions: []models.InvestmentDecision{}}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/decisions"))

		if repo.capturedPage != 1 {
			t.Errorf("page default: got %d, want 1", repo.capturedPage)
		}
		if repo.capturedLimit != 20 {
			t.Errorf("limit default: got %d, want 20", repo.capturedLimit)
		}
	})

	t.Run("resets_limit_to_default_when_limit_exceeds_maximum_of_100", func(t *testing.T) {
		t.Parallel()
		// Handler validates: if limit < 1 or limit > 100, reset to default 20.
		// This prevents runaway queries with huge page sizes.
		repo := &mockDecisionRepoForEval{decisions: []models.InvestmentDecision{}}
		w := httptest.NewRecorder()
		evalHandler(repo).ServeHTTP(w, getRequest("/users/eval/decisions?limit=999"))

		if repo.capturedLimit != 20 {
			t.Errorf("limit reset to default: got %d, want 20", repo.capturedLimit)
		}
	})
}
