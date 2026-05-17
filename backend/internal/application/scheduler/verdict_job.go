// application/scheduler/verdict_job.go
package scheduler

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// VerdictJob runs daily and stamps a performance verdict on every InvestmentDecision
// that is at least 24 hours old and has not yet been evaluated.
// It is append-only — existing verdicts are never overwritten.
type VerdictJob struct {
	decisionRepo     ports.DecisionRepository
	profileRepo      ports.ProfileRepository
	brokerageFactory ports.BrokerageProviderFactory
	marketProvider   ports.MarketDataProvider
}

func NewVerdictJob(
	decisionRepo ports.DecisionRepository,
	profileRepo ports.ProfileRepository,
	brokerageFactory ports.BrokerageProviderFactory,
	marketProvider ports.MarketDataProvider,
) *VerdictJob {
	return &VerdictJob{
		decisionRepo:     decisionRepo,
		profileRepo:      profileRepo,
		brokerageFactory: brokerageFactory,
		marketProvider:   marketProvider,
	}
}

// Start runs the ticker loop in the calling goroutine. Call via go job.Start(ctx).
// Tick interval: VERDICT_JOB_TICK env var (default 24h).
// Minimum decision age: VERDICT_MIN_AGE env var (default 24h).
// Set both to a short duration in .env to test immediately against existing MongoDB decisions.
func (j *VerdictJob) Start(ctx context.Context) {
	tick := 24 * time.Hour
	if raw := os.Getenv("VERDICT_JOB_TICK"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			tick = d
		} else {
			log.Printf("[verdict-job] invalid VERDICT_JOB_TICK %q — defaulting to 24h", raw)
		}
	}

	minAge := 24 * time.Hour
	if raw := os.Getenv("VERDICT_MIN_AGE"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d >= 0 {
			minAge = d
		} else {
			log.Printf("[verdict-job] invalid VERDICT_MIN_AGE %q — defaulting to 24h", raw)
		}
	}

	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	log.Printf("[verdict-job] started — tick=%s minAge=%s", tick, minAge)

	// Run immediately on startup — don't wait for first tick
	j.run(ctx, minAge)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[verdict-job] stopped")
			return
		case <-ticker.C:
			j.run(ctx, minAge)
		}
	}
}

func (j *VerdictJob) run(ctx context.Context, minAge time.Duration) {
	log.Printf("[verdict-job] cycle started (minAge=%s)", minAge)

	userIDs, err := j.decisionRepo.GetUsersWithPendingVerdicts(ctx, minAge)
	if err != nil {
		log.Printf("[verdict-job] fetch pending users: %v", err)
		return
	}
	if len(userIDs) == 0 {
		log.Printf("[verdict-job] no pending verdicts")
		return
	}

	log.Printf("[verdict-job] processing %d user(s)", len(userIDs))
	for _, userID := range userIDs {
		stampVerdicts(ctx, userID, minAge, j.decisionRepo, j.profileRepo, j.brokerageFactory, j.marketProvider)
	}
	log.Printf("[verdict-job] cycle done — %d user(s) processed", len(userIDs))
}
