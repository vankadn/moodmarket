// application/scheduler/rebalance_digest_scheduler.go
package scheduler

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// RebalanceDigestScheduler sends a weekly email digest of Claude's rebalance
// analysis to every auto-invest user who has a notification email configured.
// Interval is read from REBALANCE_DIGEST_INTERVAL (default 168h = 7 days).
type RebalanceDigestScheduler struct {
	autoInvestRepo  ports.AutoInvestRepository
	profileRepo     ports.ProfileRepository
	aggregationSvc  RebalanceRequestBuilder
	rebalanceAdvisor ports.RebalanceAdvisor
	notifications   ports.NotificationProvider
}

func NewRebalanceDigestScheduler(
	autoInvestRepo ports.AutoInvestRepository,
	profileRepo ports.ProfileRepository,
	aggregationSvc RebalanceRequestBuilder,
	rebalanceAdvisor ports.RebalanceAdvisor,
	notifications ports.NotificationProvider,
) *RebalanceDigestScheduler {
	return &RebalanceDigestScheduler{
		autoInvestRepo:   autoInvestRepo,
		profileRepo:      profileRepo,
		aggregationSvc:   aggregationSvc,
		rebalanceAdvisor: rebalanceAdvisor,
		notifications:    notifications,
	}
}

// Start runs the ticker loop in the calling goroutine. Call via go scheduler.Start(ctx).
func (s *RebalanceDigestScheduler) Start(ctx context.Context) {
	tick := 168 * time.Hour
	if raw := os.Getenv("REBALANCE_DIGEST_INTERVAL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			tick = d
		} else {
			log.Printf("[rebalance-digest] invalid REBALANCE_DIGEST_INTERVAL %q — defaulting to 168h", raw)
		}
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	log.Printf("[rebalance-digest] scheduler started — interval=%s", tick)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[rebalance-digest] scheduler stopped")
			return
		case <-ticker.C:
			s.runCycle(ctx)
		}
	}
}

func (s *RebalanceDigestScheduler) runCycle(ctx context.Context) {
	configs, err := s.autoInvestRepo.GetAllEnabled(ctx)
	if err != nil {
		log.Printf("[rebalance-digest] fetch configs failed: %v", err)
		return
	}
	if len(configs) == 0 {
		return
	}

	// Deduplicate — multiple configs per user should only produce one digest.
	seen := make(map[string]struct{})
	var wg sync.WaitGroup
	for _, cfg := range configs {
		if _, ok := seen[cfg.UserID]; ok {
			continue
		}
		seen[cfg.UserID] = struct{}{}

		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			s.sendDigest(ctx, userID)
		}(cfg.UserID)
	}
	wg.Wait()
	log.Printf("[rebalance-digest] cycle done — sent to %d user(s)", len(seen))
}

func (s *RebalanceDigestScheduler) sendDigest(ctx context.Context, userID string) {
	profile, err := s.profileRepo.GetByUserID(ctx, userID)
	if err != nil {
		log.Printf("[rebalance-digest] user=%s load profile failed: %v", userID, err)
		return
	}
	if profile.NotificationEmail == "" {
		log.Printf("[rebalance-digest] user=%s no notification email — skipping", userID)
		return
	}

	req, err := s.aggregationSvc.BuildRequest(ctx, userID)
	if err != nil {
		log.Printf("[rebalance-digest] user=%s build request failed: %v", userID, err)
		return
	}
	if len(req.Positions) == 0 {
		log.Printf("[rebalance-digest] user=%s no positions — skipping", userID)
		return
	}

	analysis, err := s.rebalanceAdvisor.AnalyzePortfolio(ctx, *req, profile)
	if err != nil {
		log.Printf("[rebalance-digest] user=%s analyze failed: %v", userID, err)
		return
	}

	target := ports.NotificationTarget{
		UserID: userID,
		Email:  profile.NotificationEmail,
		Source: "auto",
	}
	if err := s.notifications.SendRebalanceDigest(ctx, target, analysis); err != nil {
		log.Printf("[rebalance-digest] user=%s send failed: %v", userID, err)
	}
}
