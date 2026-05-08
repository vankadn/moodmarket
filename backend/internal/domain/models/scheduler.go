// domain/models/scheduler.go
package models

import "time"

// SchedulerRun is an audit record for one autonomous investment cycle.
// One document is written to the scheduler_runs collection after every tick.
type SchedulerRun struct {
	RunID          string    `json:"run_id"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	UsersProcessed int       `json:"users_processed"`
	TotalInvested  float64   `json:"total_invested"`
	Errors         []string  `json:"errors,omitempty"`
}
