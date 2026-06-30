// application/scheduler/auto_invest_runner_test.go
//
// runForUser is the per-user investment pipeline. These tests pin the branches
// that decide whether (and how much) money moves: the agentic daily-budget cap,
// Claude's $0 skip, the over-budget scaling, the not-connected soft-skip, and
// the failure-notification paths.
package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

func target() ports.NotificationTarget {
	return ports.NotificationTarget{UserID: "user-1", Source: "auto"}
}

func TestRunForUser_RecommendationFailureNotifiesAndErrors(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{err: errors.New("claude timeout")}
	inv := &fakeInvestor{}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Amount: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err == nil {
		t.Fatal("expected error when recommendation fails")
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if notes.failureCalls != 1 {
		t.Errorf("expected one failure notification, got %d", notes.failureCalls)
	}
	if inv.called {
		t.Error("investor must not run when recommendation fails")
	}
}

func TestRunForUser_ClaudeSkipSavesSkipDecision(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 0, SkipReason: "valuations stretched"}}
	inv := &fakeInvestor{}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Amount: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("a $0 skip must not be an error, got: %v", err)
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if inv.called {
		t.Error("investor must not run on a skip")
	}
	if notes.skipCalls != 1 || notes.skipReason != "valuations stretched" {
		t.Errorf("expected skip summary with reason, got calls=%d reason=%q", notes.skipCalls, notes.skipReason)
	}
	if len(repo.saved) != 1 || repo.saved[0].DecisionType != "skip" {
		t.Errorf("expected one persisted skip decision, got %+v", repo.saved)
	}
}

func TestRunForUser_AgenticBudgetExhausted(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 100}} // should never be consulted
	inv := &fakeInvestor{}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{spentToday: 100} // already spent the whole daily budget
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Mode: "agentic", DailyBudget: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("budget-exhausted must not be an error, got: %v", err)
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if rec.calls != 0 {
		t.Error("recommender must not be called once the daily budget is exhausted")
	}
	if notes.skipCalls != 1 {
		t.Errorf("expected a skip summary, got %d", notes.skipCalls)
	}
	if len(repo.saved) != 1 || repo.saved[0].DecisionType != "skip" {
		t.Errorf("expected a persisted skip decision, got %+v", repo.saved)
	}
}

// TestRunForUser_AgenticBudgetExhausted_SecondTickSuppressed verifies that when
// HasSkipToday returns true (a budget-exhausted skip was already recorded earlier today),
// the runner returns cleanly without writing another skip doc or firing SendSkipSummary.
func TestRunForUser_AgenticBudgetExhausted_SecondTickSuppressed(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 100}}
	inv := &fakeInvestor{}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{
		spentToday:   100,
		hasSkipToday: true, // skip doc already written on an earlier tick today
	}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Mode: "agentic", DailyBudget: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("suppressed tick must not be an error, got: %v", err)
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if rec.calls != 0 {
		t.Error("recommender must not be called on a suppressed tick")
	}
	if notes.skipCalls != 0 {
		t.Errorf("SendSkipSummary must not fire on suppressed tick, got %d calls", notes.skipCalls)
	}
	if len(repo.saved) != 0 {
		t.Errorf("no skip doc must be written on suppressed tick, got %d docs", len(repo.saved))
	}
}

// TestRunForUser_AgenticBudgetExhausted_NewDayAllowsEvaluation verifies that on a new
// calendar day HasSkipToday returns false (no matching record for today), so the pipeline
// proceeds normally to the recommender rather than short-circuiting.
func TestRunForUser_AgenticBudgetExhausted_NewDayAllowsEvaluation(t *testing.T) {
	t.Parallel()

	// Budget not yet spent today (new day), skip guard reports no prior skip today.
	rec := &fakeRecommender{rec: &models.Recommendation{
		TotalBudget: 100,
		Allocations: []models.Allocation{{Ticker: "VTI", Amount: 100}},
	}}
	inv := &fakeInvestor{receipts: []models.TradeReceipt{{Ticker: "VTI", FilledAmount: 100}}}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{
		spentToday:   0,
		hasSkipToday: false,
	}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Mode: "agentic", DailyBudget: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("new-day evaluation must not error, got: %v", err)
	}
	if invested == 0 {
		t.Error("new-day evaluation should have invested, got 0")
	}
	if rec.calls != 1 {
		t.Errorf("recommender should be called once on a new day, got %d", rec.calls)
	}
}

// TestRunForUser_AgenticBudgetExhausted_GuardErrorSuppressesWithoutWrite verifies that
// if HasSkipToday itself returns an error, the tick is suppressed silently (no skip doc,
// no notification) rather than risking a duplicate write.
func TestRunForUser_AgenticBudgetExhausted_GuardErrorSuppressesWithoutWrite(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 100}}
	inv := &fakeInvestor{}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{
		spentToday:      100,
		hasSkipTodayErr: errors.New("mongo timeout"),
	}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Mode: "agentic", DailyBudget: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("guard error must not propagate as an error, got: %v", err)
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if notes.skipCalls != 0 {
		t.Errorf("SendSkipSummary must not fire when guard errors, got %d calls", notes.skipCalls)
	}
	if len(repo.saved) != 0 {
		t.Errorf("no skip doc must be written when guard errors, got %d docs", len(repo.saved))
	}
}

func TestRunForUser_AgenticCapsOverBudgetRecommendation(t *testing.T) {
	t.Parallel()

	// Remaining budget is 60 (100 budget − 40 spent). Claude proposes 120, which
	// must be scaled down to 60 with allocations scaled by the same factor.
	rec := &fakeRecommender{rec: &models.Recommendation{
		TotalBudget: 120,
		RiskLevel:   "moderate",
		Allocations: []models.Allocation{
			{Ticker: "VTI", Amount: 80},
			{Ticker: "BND", Amount: 40},
		},
	}}
	inv := &fakeInvestor{receipts: []models.TradeReceipt{{Ticker: "VTI", FilledAmount: 40}, {Ticker: "BND", FilledAmount: 20}}, decisionID: "d1"}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{spentToday: 40}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Mode: "agentic", DailyBudget: 100}

	_, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !inv.called {
		t.Fatal("investor should have been called")
	}
	if inv.gotTotal != 60 {
		t.Errorf("capped total = %v, want 60", inv.gotTotal)
	}
	// 80 * (60/120) = 40, 40 * 0.5 = 20.
	got := map[string]float64{}
	for _, a := range inv.gotAllocs {
		got[a.Ticker] = a.Amount
	}
	if got["VTI"] != 40 || got["BND"] != 20 {
		t.Errorf("allocations not scaled: %+v", got)
	}
}

func TestRunForUser_NotConnectedIsSoftSkip(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 100, Allocations: []models.Allocation{{Ticker: "VTI", Amount: 100}}}}
	inv := &fakeInvestor{err: ports.ErrBrokerageNotConnected}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Amount: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("not-connected must be a soft skip, got: %v", err)
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if notes.failureCalls != 0 {
		t.Error("not-connected must not trigger a failure notification")
	}
}

func TestRunForUser_ExecutionErrorNotifiesAndErrors(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 100, Allocations: []models.Allocation{{Ticker: "VTI", Amount: 100}}}}
	inv := &fakeInvestor{err: errors.New("alpaca rejected")}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Amount: 100}

	_, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err == nil {
		t.Fatal("expected execution error to propagate")
	}
	if notes.failureCalls != 1 {
		t.Errorf("expected a failure notification, got %d", notes.failureCalls)
	}
}

func TestRunForUser_SuccessSendsSummaryAndReportsFilled(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{
		TotalBudget: 100,
		Allocations: []models.Allocation{{Ticker: "VTI", Amount: 60}, {Ticker: "BND", Amount: 40}},
	}}
	inv := &fakeInvestor{receipts: []models.TradeReceipt{
		{Ticker: "VTI", FilledAmount: 60},
		{Ticker: "BND", FilledAmount: 40},
	}}
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Amount: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invested != 100 {
		t.Errorf("invested = %v, want 100", invested)
	}
	if notes.summaryCalls != 1 || notes.lastSummaryTotal != 100 {
		t.Errorf("expected one summary with total 100, got calls=%d total=%v", notes.summaryCalls, notes.lastSummaryTotal)
	}
}

func TestRunForUser_EmptyReceiptsSkipsNotification(t *testing.T) {
	t.Parallel()

	rec := &fakeRecommender{rec: &models.Recommendation{TotalBudget: 100, Allocations: []models.Allocation{{Ticker: "VTI", Amount: 100}}}}
	inv := &fakeInvestor{receipts: nil} // all orders skipped/failed inside Execute
	notes := &fakeNotifications{}
	repo := &fakeDecisionRepo{}
	cfg := models.AutoInvestConfig{UserID: "user-1", ID: "c1", Amount: 100}

	invested, err := runForUser(context.Background(), cfg, target(), rec, inv, notes, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invested != 0 {
		t.Errorf("invested = %v, want 0", invested)
	}
	if notes.summaryCalls != 0 {
		t.Error("no summary should be sent when no positions were placed")
	}
}

