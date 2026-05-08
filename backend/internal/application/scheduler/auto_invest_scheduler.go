// application/scheduler/auto_invest_scheduler.go
package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/application/services"
	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// AutoInvestScheduler ticks on a configurable interval and runs the full
// investment pipeline for every user who has opted into autonomous investing.
type AutoInvestScheduler struct {
	autoInvestRepo ports.AutoInvestRepository
	recommendSvc   *services.RecommendationService
	investSvc      *services.InvestmentService
	schedulerRepo  ports.SchedulerRepository
	notifications  ports.NotificationProvider
	interval       time.Duration
}

func NewAutoInvestScheduler(
	autoInvestRepo ports.AutoInvestRepository,
	recommendSvc *services.RecommendationService,
	investSvc *services.InvestmentService,
	schedulerRepo ports.SchedulerRepository,
	notifications ports.NotificationProvider,
) *AutoInvestScheduler {
	interval := parseInterval()
	log.Printf("[scheduler] auto-invest interval: %s", interval)
	return &AutoInvestScheduler{
		autoInvestRepo: autoInvestRepo,
		recommendSvc:   recommendSvc,
		investSvc:      investSvc,
		schedulerRepo:  schedulerRepo,
		notifications:  notifications,
		interval:       interval,
	}
}

// Start runs the ticker loop in the calling goroutine. Call via go scheduler.Start(ctx).
func (s *AutoInvestScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	log.Printf("[scheduler] started — first tick in %s", s.interval)

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
	log.Printf("[scheduler] cycle %s — running for %d user(s)", runID, len(configs))

	var (
		mu            sync.Mutex
		totalInvested float64
		errs          []string
		wg            sync.WaitGroup
	)

	for _, cfg := range configs {
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
			}
		}(cfg)
	}
	wg.Wait()

	run := &models.SchedulerRun{
		RunID:          runID,
		StartedAt:      startedAt,
		CompletedAt:    time.Now(),
		UsersProcessed: len(configs),
		TotalInvested:  totalInvested,
		Errors:         errs,
	}
	if err := s.schedulerRepo.Save(ctx, run); err != nil {
		log.Printf("[scheduler] cycle %s — failed to save audit record: %v", runID, err)
	}

	log.Printf("[scheduler] cycle %s done — %d users, %d errors", runID, len(configs), len(errs))
}

func parseInterval() time.Duration {
	raw := os.Getenv("AUTO_INVEST_INTERVAL")
	if raw == "" {
		return 24 * time.Hour
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Printf("[scheduler] invalid AUTO_INVEST_INTERVAL=%q, defaulting to 24h: %v", raw, err)
		return 24 * time.Hour
	}
	return d
}
