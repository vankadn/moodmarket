// domain/models/auto_invest_config.go
package models

import "time"

// AutoInvestConfig is a first-class domain model representing a user's autonomous
// investment preferences. Stored in the auto_invest_configs collection, not on UserProfile.
type AutoInvestConfig struct {
	ID           string        `json:"id"`
	UserID       string        `json:"user_id"`
	Enabled      bool          `json:"enabled"`
	Amount       float64       `json:"amount"`
	Risk         RiskTolerance `json:"risk"`
	IntervalDays int           `json:"interval_days,omitempty"`
	EnabledAt    time.Time     `json:"enabled_at,omitempty"`
	UpdatedAt    time.Time     `json:"updated_at"`
	LastRunAt    *time.Time    `json:"last_run_at,omitempty"`
}
