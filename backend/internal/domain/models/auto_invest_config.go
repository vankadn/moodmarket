// domain/models/auto_invest_config.go
package models

import "time"

// AutoInvestConfig is a first-class domain model representing a user's autonomous
// investment preferences. Stored in the auto_invest_configs collection, not on UserProfile.
type AutoInvestConfig struct {
	ID              string        `json:"id"`
	UserID          string        `json:"user_id"`
	Name            string        `json:"name,omitempty"`
	Enabled         bool          `json:"enabled"`
	Mode            string        `json:"mode,omitempty"` // "fixed" (default) | "agentic"
	Amount          float64       `json:"amount"`          // fixed mode: invest this amount each run
	DailyBudget     float64       `json:"daily_budget,omitempty"` // agentic mode: max to invest per calendar day
	Risk            RiskTolerance `json:"risk"`
	Strategy        string        `json:"strategy,omitempty"` // "long_term" | "short_term"
	IntervalDays    int           `json:"interval_days,omitempty"`      // deprecated — migrated to IntervalHours on read
	IntervalHours   int           `json:"interval_hours,omitempty"`     // when > 0, overrides interval_days; agentic hardcodes to 1
	IntervalSeconds int           `json:"interval_seconds,omitempty"`   // when > 0, overrides interval_hours; dev/sub-hour testing only
	EnabledAt       time.Time     `json:"enabled_at,omitempty"`
	UpdatedAt       time.Time     `json:"updated_at"`
	LastRunAt       *time.Time    `json:"last_run_at,omitempty"`
}
