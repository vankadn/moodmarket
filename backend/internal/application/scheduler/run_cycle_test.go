// application/scheduler/run_cycle_test.go
//
// runCycle is the fan-out: it filters enabled configs to those that are due,
// runs each user's pipeline concurrently, and records an audit run. These tests
// pin the market-closed short-circuit, per-user error isolation (one user's
// failure must not block others or stamp their last-run), and that only
// successful users get their LastRunAt stamped.
package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

func newScheduler(
	autoInvestRepo *fakeAutoInvestRepo,
	profileRepo *fakeProfileRepo,
	rec Recommender,
	inv Investor,
	schedRepo *fakeSchedulerRepo,
	notes *fakeNotifications,
	cal *fakeCalendar,
	decisionRepo *fakeDecisionRepo,
) *AutoInvestScheduler {
	return NewAutoInvestScheduler(autoInvestRepo, profileRepo, rec, inv, schedRepo, notes, cal, decisionRepo)
}

func TestRunCycle_MarketClosedShortCircuits(t *testing.T) {
	t.Parallel()

	air := newFakeAutoInvestRepo()
	air.enabled = []models.AutoInvestConfig{{UserID: "u1", ID: "c1", Enabled: true}}
	schedRepo := &fakeSchedulerRepo{}

	s := newScheduler(air, &fakeProfileRepo{}, &fakeRecommender{}, &fakeInvestor{}, schedRepo, &fakeNotifications{}, &fakeCalendar{tradingDay: false}, &fakeDecisionRepo{})
	s.runCycle(context.Background())

	if len(schedRepo.runs) != 0 {
		t.Errorf("no audit run should be saved when the market is closed, got %d", len(schedRepo.runs))
	}
	if len(air.stamped) != 0 {
		t.Errorf("no config should be stamped when the market is closed, got %v", air.stamped)
	}
}

func TestRunCycle_PerUserErrorIsolation(t *testing.T) {
	t.Parallel()

	// Two due users (LastRunAt nil → always due). u-bad's recommendation fails;
	// u-good succeeds. The failure must be isolated: u-good still runs and is
	// stamped, u-bad is not stamped, and the error is recorded in the audit run.
	air := newFakeAutoInvestRepo()
	air.enabled = []models.AutoInvestConfig{
		{UserID: "u-good", ID: "c-good", Enabled: true, Amount: 100},
		{UserID: "u-bad", ID: "c-bad", Enabled: true, Amount: 100},
	}

	rec := &recRouter{
		byUser: map[string]*models.Recommendation{
			"u-good": {TotalBudget: 100, Allocations: []models.Allocation{{Ticker: "VTI", Amount: 100}}},
		},
		errByUser: map[string]error{
			"u-bad": errors.New("claude exploded"),
		},
	}
	inv := &fakeInvestor{receipts: []models.TradeReceipt{{Ticker: "VTI", FilledAmount: 100}}}
	schedRepo := &fakeSchedulerRepo{}
	profile := &fakeProfileRepo{profile: &models.UserProfile{NotificationEmail: "u@x.com"}}

	s := newScheduler(air, profile, rec, inv, schedRepo, &fakeNotifications{}, &fakeCalendar{tradingDay: true}, &fakeDecisionRepo{})
	s.runCycle(context.Background())

	if _, ok := air.stamped["c-good"]; !ok {
		t.Error("successful user's config should be stamped")
	}
	if _, ok := air.stamped["c-bad"]; ok {
		t.Error("failed user's config must NOT be stamped")
	}
	if len(schedRepo.runs) != 1 {
		t.Fatalf("expected one audit run, got %d", len(schedRepo.runs))
	}
	run := schedRepo.runs[0]
	if run.UsersProcessed != 2 {
		t.Errorf("UsersProcessed = %d, want 2", run.UsersProcessed)
	}
	if len(run.Errors) != 1 {
		t.Errorf("expected 1 recorded error, got %d (%v)", len(run.Errors), run.Errors)
	}
}

func TestRunCycle_NotDueUsersAreSkipped(t *testing.T) {
	t.Parallel()

	// One user enabled but not due (ran very recently with a long interval).
	recent := time.Now()
	air := newFakeAutoInvestRepo()
	air.enabled = []models.AutoInvestConfig{
		{UserID: "u1", ID: "c1", Enabled: true, IntervalDays: 7, LastRunAt: &recent},
	}
	schedRepo := &fakeSchedulerRepo{}

	s := newScheduler(air, &fakeProfileRepo{}, &fakeRecommender{}, &fakeInvestor{}, schedRepo, &fakeNotifications{}, &fakeCalendar{tradingDay: true}, &fakeDecisionRepo{})
	s.runCycle(context.Background())

	if len(air.stamped) != 0 {
		t.Errorf("not-due user must not run/stamp, got %v", air.stamped)
	}
	// No audit run is saved when nobody is due (function returns before Save).
	if len(schedRepo.runs) != 0 {
		t.Errorf("no audit run expected when nobody is due, got %d", len(schedRepo.runs))
	}
}

// recRouter returns a per-user recommendation or error, so a single cycle can
// have some users succeed and others fail.
type recRouter struct {
	byUser    map[string]*models.Recommendation
	errByUser map[string]error
}

func (r *recRouter) GetDailyRecommendation(_ context.Context, userID string, _ models.InvestmentRequest) (*models.Recommendation, error) {
	if err := r.errByUser[userID]; err != nil {
		return nil, err
	}
	if rec := r.byUser[userID]; rec != nil {
		return rec, nil
	}
	return &models.Recommendation{TotalBudget: 0}, nil
}
