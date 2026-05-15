// application/scheduler/auto_invest_scheduler.go
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/application/services"
	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// AutoInvestScheduler ticks hourly and runs the full investment pipeline for
// every user whose per-user interval has elapsed since their last run.
type AutoInvestScheduler struct {
	autoInvestRepo ports.AutoInvestRepository
	recommendSvc   *services.RecommendationService
	investSvc      *services.InvestmentService
	schedulerRepo  ports.SchedulerRepository
	notifications  ports.NotificationProvider
}

func NewAutoInvestScheduler(
	autoInvestRepo ports.AutoInvestRepository,
	recommendSvc *services.RecommendationService,
	investSvc *services.InvestmentService,
	schedulerRepo ports.SchedulerRepository,
	notifications ports.NotificationProvider,
) *AutoInvestScheduler {
	return &AutoInvestScheduler{
		autoInvestRepo: autoInvestRepo,
		recommendSvc:   recommendSvc,
		investSvc:      investSvc,
		schedulerRepo:  schedulerRepo,
		notifications:  notifications,
	}
}

// Start runs the ticker loop in the calling goroutine. Call via go scheduler.Start(ctx).
// Ticks every hour; per-user interval_days controls actual run frequency.
func (s *AutoInvestScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	log.Printf("[scheduler] started — hourly check, per-user interval applies")

	for {
		select {
		case <-ctx.Done():
			log.Printf("[scheduler] stopped")
			return
		case <-ticker.C:
			s.runCycle(ctx)
		}
	}
}

// isDue returns true when a user's run interval has elapsed.
// IntervalDays == 0 is treated as 1 (daily). LastRunAt == nil means never run.
func isDue(cfg models.AutoInvestConfig) bool {
	days := cfg.IntervalDays
	if days <= 0 {
		days = 1
	}
	if cfg.LastRunAt == nil {
		return true
	}
	return time.Since(*cfg.LastRunAt) >= time.Duration(days)*24*time.Hour
}

func (s *AutoInvestScheduler) runCycle(ctx context.Context) {
	startedAt := time.Now()
	runID := fmt.Sprintf("run-%d", startedAt.UnixNano())
	log.Printf("[scheduler] cycle %s started", runID)

	configs, err := s.autoInvestRepo.GetAllEnabled(ctx)
	if err != nil {
		log.Printf("[scheduler] cycle %s — failed to fetch configs: %v", runID, err)
		return
	}
	if len(configs) == 0 {
		log.Printf("[scheduler] cycle %s — no users with auto-invest enabled", runID)
		return
	}

	// Filter to users whose interval has elapsed.
	var due []models.AutoInvestConfig
	for _, c := range configs {
		if isDue(c) {
			due = append(due, c)
		}
	}
	if len(due) == 0 {
		log.Printf("[scheduler] cycle %s — no users due yet (%d enabled)", runID, len(configs))
		return
	}
	log.Printf("[scheduler] cycle %s — running for %d/%d user(s)", runID, len(due), len(configs))

	var (
		mu            sync.Mutex
		totalInvested float64
		errs          []string
		wg            sync.WaitGroup
	)

	for _, cfg := range due {
		wg.Add(1)
		go func(c models.AutoInvestConfig) {
			defer wg.Done()
			invested, err := runForUser(ctx, c, s.recommendSvc, s.investSvc, s.notifications)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Sprintf("user=%s: %v", c.UserID, err))
			} else {
				totalInvested += invested
				if stampErr := s.autoInvestRepo.StampLastRunAt(ctx, c.UserID, startedAt); stampErr != nil {
					log.Printf("[scheduler] cycle %s — failed to stamp last_run_at for user=%s: %v", runID, c.UserID, stampErr)
				}
			}
		}(cfg)
	}
	wg.Wait()

	run := &models.SchedulerRun{
		RunID:          runID,
		StartedAt:      startedAt,
		CompletedAt:    time.Now(),
		UsersProcessed: len(due),
		TotalInvested:  totalInvested,
		Errors:         errs,
	}
	if err := s.schedulerRepo.Save(ctx, run); err != nil {
		log.Printf("[scheduler] cycle %s — failed to save audit record: %v", runID, err)
	}

	log.Printf("[scheduler] cycle %s done — %d users, %d errors", runID, len(due), len(errs))
}

