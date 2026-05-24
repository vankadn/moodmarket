// application/scheduler/rebalancing_scheduler.go
package scheduler

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/application/services"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// RebalancingScheduler ticks daily and sends a rebalancing alert to any auto-invest
// user whose portfolio has drifted past the configured threshold.
// Threshold is read from REBALANCE_DRIFT_THRESHOLD (default 10 percentage points).
// Tick interval is read from REBALANCE_TICK (default 24h).
type RebalancingScheduler struct {
	autoInvestRepo   ports.AutoInvestRepository
	profileRepo      ports.ProfileRepository
	rebalancingSvc   *services.RebalancingService
	notifications    ports.NotificationProvider
	calendar         ports.MarketCalendar
}

func NewRebalancingScheduler(
	autoInvestRepo ports.AutoInvestRepository,
	profileRepo ports.ProfileRepository,
	rebalancingSvc *services.RebalancingService,
	notifications ports.NotificationProvider,
	calendar ports.MarketCalendar,
) *RebalancingScheduler {
	return &RebalancingScheduler{
		autoInvestRepo: autoInvestRepo,
		profileRepo:    profileRepo,
		rebalancingSvc: rebalancingSvc,
		notifications:  notifications,
		calendar:       calendar,
	}
}

// Start runs the ticker loop in the calling goroutine. Call via go scheduler.Start(ctx).
func (s *RebalancingScheduler) Start(ctx context.Context) {
	tick := 24 * time.Hour
	if raw := os.Getenv("REBALANCE_TICK"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			tick = d
		} else {
			log.Printf("[rebalance] invalid REBALANCE_TICK %q — defaulting to 24h", raw)
		}
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	log.Printf("[rebalance] scheduler started — tick=%s", tick)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[rebalance] scheduler stopped")
			return
		case <-ticker.C:
			s.runCycle(ctx)
		}
	}
}

func (s *RebalancingScheduler) runCycle(ctx context.Context) {
	now := time.Now()
	if !s.calendar.IsTradingDay(now) {
		log.Printf("[rebalance] market closed today (%s) — skipping", now.Format("Mon Jan 2"))
		return
	}

	configs, err := s.autoInvestRepo.GetAllEnabled(ctx)
	if err != nil {
		log.Printf("[rebalance] fetch configs failed: %v", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, cfg := range configs {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()

			drifts, err := s.rebalancingSvc.CheckDrift(ctx, userID)
			if err != nil {
				log.Printf("[rebalance] user=%s drift check failed: %v", userID, err)
				return
			}
			if len(drifts) == 0 {
				return
			}

			log.Printf("[rebalance] user=%s drift detected in %d ticker(s)", userID, len(drifts))

			target := ports.NotificationTarget{UserID: userID, Source: "auto"}
			if profile, err := s.profileRepo.GetByUserID(ctx, userID); err == nil {
				target.Email = profile.NotificationEmail
				target.Phone = profile.Phone
			}

			if err := s.notifications.SendRebalancingAlert(ctx, target, drifts); err != nil {
				log.Printf("[rebalance] user=%s send alert failed: %v", userID, err)
			}
		}(cfg.UserID)
	}
	wg.Wait()
	log.Printf("[rebalance] cycle done — checked %d user(s)", len(configs))
}
