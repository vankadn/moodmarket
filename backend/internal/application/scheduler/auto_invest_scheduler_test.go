// application/scheduler/auto_invest_scheduler_test.go
//
// isDue is the gate that decides whether a user's auto-invest config runs on a
// given tick. A wrong answer means either a missed investment or a double
// investment, so the interval-priority rules and the never-run case are pinned
// here exhaustively.
//
// The wider runCycle/runForUser orchestration is intentionally not unit-tested:
// it binds to concrete *services.RecommendationService / *services.InvestmentService
// types rather than interfaces, which would require a DI refactor to mock.
package scheduler

import (
	"testing"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

func ptrTime(t time.Time) *time.Time { return &t }

func TestIsDue(t *testing.T) {
	t.Parallel()

	now := time.Now()

	cases := []struct {
		name string
		cfg  models.AutoInvestConfig
		want bool
	}{
		{
			name: "never_run_is_always_due",
			cfg:  models.AutoInvestConfig{IntervalHours: 24, LastRunAt: nil},
			want: true,
		},
		{
			name: "seconds_interval_elapsed_is_due",
			cfg:  models.AutoInvestConfig{IntervalSeconds: 30, LastRunAt: ptrTime(now.Add(-31 * time.Second))},
			want: true,
		},
		{
			name: "seconds_interval_not_elapsed_is_not_due",
			cfg:  models.AutoInvestConfig{IntervalSeconds: 30, LastRunAt: ptrTime(now.Add(-10 * time.Second))},
			want: false,
		},
		{
			name: "seconds_takes_priority_over_hours",
			// Hours would say "not due" (1h not elapsed) but seconds (1s) is elapsed and wins.
			cfg:  models.AutoInvestConfig{IntervalSeconds: 1, IntervalHours: 1, LastRunAt: ptrTime(now.Add(-2 * time.Second))},
			want: true,
		},
		{
			name: "hours_interval_elapsed_is_due",
			cfg:  models.AutoInvestConfig{IntervalHours: 6, LastRunAt: ptrTime(now.Add(-7 * time.Hour))},
			want: true,
		},
		{
			name: "hours_interval_not_elapsed_is_not_due",
			cfg:  models.AutoInvestConfig{IntervalHours: 6, LastRunAt: ptrTime(now.Add(-1 * time.Hour))},
			want: false,
		},
		{
			name: "hours_takes_priority_over_days",
			// Days (1d) would not be elapsed, but hours (1h) is and wins.
			cfg:  models.AutoInvestConfig{IntervalHours: 1, IntervalDays: 1, LastRunAt: ptrTime(now.Add(-2 * time.Hour))},
			want: true,
		},
		{
			name: "days_interval_elapsed_is_due",
			cfg:  models.AutoInvestConfig{IntervalDays: 1, LastRunAt: ptrTime(now.Add(-25 * time.Hour))},
			want: true,
		},
		{
			name: "days_interval_not_elapsed_is_not_due",
			cfg:  models.AutoInvestConfig{IntervalDays: 7, LastRunAt: ptrTime(now.Add(-24 * time.Hour))},
			want: false,
		},
		{
			name: "zero_days_defaults_to_one_day_not_elapsed",
			cfg:  models.AutoInvestConfig{IntervalDays: 0, LastRunAt: ptrTime(now.Add(-1 * time.Hour))},
			want: false,
		},
		{
			name: "zero_days_defaults_to_one_day_elapsed",
			cfg:  models.AutoInvestConfig{IntervalDays: 0, LastRunAt: ptrTime(now.Add(-25 * time.Hour))},
			want: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isDue(tc.cfg); got != tc.want {
				t.Errorf("isDue() = %v, want %v", got, tc.want)
			}
		})
	}
}
