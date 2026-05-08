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
	profileRepo   ports.ProfileRepository
	recommendSvc  *services.RecommendationService
	investSvc     *services.InvestmentService
	schedulerRepo ports.SchedulerRepository
	notifications ports.NotificationProvider
	interval      time.Duration
}

func NewAutoInvestScheduler(
	profileRepo ports.ProfileRepository,
	recommendSvc *services.RecommendationService,
	investSvc *services.InvestmentService,
	schedulerRepo ports.SchedulerRepository,
	notifications ports.NotificationProvider,
) *AutoInvestScheduler {
	interval := parseInterval()
	log.Printf("[scheduler] auto-invest interval: %s", interval)
	return &AutoInvestScheduler{
		profileRepo:   profileRepo,
		recommendSvc:  recommendSvc,
		investSvc:     investSvc,
		schedulerRepo: schedulerRepo,
		notifications: notifications,
		interval:      interval,
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

	users, err := s.profileRepo.GetAutoInvestUsers(ctx)
	if err != nil {
		log.Printf("[scheduler] cycle %s — failed to fetch users: %v", runID, err)
		return
	}
	if len(users) == 0 {
		log.Printf("[scheduler] cycle %s — no users with auto-invest enabled", runID)
		return
	}
	log.Printf("[scheduler] cycle %s — running for %d user(s)", runID, len(users))

	var (
		mu            sync.Mutex
		totalInvested float64
		errs          []string
		wg            sync.WaitGroup
	)

	for _, user := range users {
		wg.Add(1)
		go func(u models.UserProfile) {
			defer wg.Done()
			invested, err := runForUser(ctx, u.UserID, s.recommendSvc, s.investSvc, s.notifications)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Sprintf("user=%s: %v", u.UserID, err))
			} else {
				totalInvested += invested
			}
		}(user)
	}
	wg.Wait()

	run := &models.SchedulerRun{
		RunID:          runID,
		StartedAt:      startedAt,
		CompletedAt:    time.Now(),
		UsersProcessed: len(users),
		TotalInvested:  totalInvested,
		Errors:         errs,
	}
	if err := s.schedulerRepo.Save(ctx, run); err != nil {
		log.Printf("[scheduler] cycle %s — failed to save audit record: %v", runID, err)
	}

	log.Printf("[scheduler] cycle %s done — %d users, %d errors", runID, len(users), len(errs))
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
