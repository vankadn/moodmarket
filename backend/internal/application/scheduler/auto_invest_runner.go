// application/scheduler/auto_invest_runner.go
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

const defaultTimezone = "America/New_York"

// runForUser executes the full investment pipeline for a single user using their config.
// Returns the total dollar amount invested, or an error if the pipeline fails.
func runForUser(
	ctx context.Context,
	config models.AutoInvestConfig,
	target ports.NotificationTarget,
	recommendSvc Recommender,
	investSvc Investor,
	notifications ports.NotificationProvider,
	decisionRepo ports.DecisionRepository,
) (float64, error) {
	isAgentic := config.Mode == "agentic"
	log.Printf("[scheduler] user=%s pipeline start (mode=%s amount=%.0f dailyBudget=%.0f strategy=%s)", config.UserID, config.Mode, config.Amount, config.DailyBudget, config.Strategy)

	req := models.InvestmentRequest{
		BaseBudget:   config.Amount,
		ExtraMoney:   0,
		StrategyType: config.Strategy,
	}

	// Lifetime budget cap — applies to all modes; checked before daily budget to avoid unnecessary work.
	if config.LifetimeBudgetCap > 0 {
		spentAllTime, err := decisionRepo.SumAllTimeByConfig(ctx, config.UserID, config.ID)
		if err != nil {
			log.Printf("[scheduler] user=%s lifetime budget check failed (%v) — proceeding without cap", config.UserID, err)
		} else if spentAllTime >= config.LifetimeBudgetCap {
			log.Printf("[scheduler] user=%s lifetime budget cap reached (cap=%.2f spent=%.2f)", config.UserID, config.LifetimeBudgetCap, spentAllTime)
			saveSkipDecision(ctx, decisionRepo, config, "lifetime budget cap reached")
			_ = notifications.SendSkipSummary(ctx, target, config.Name, "Lifetime budget cap reached — this strategy has deployed its maximum configured amount.")
			return 0, nil
		}
	}

	if isAgentic {
		spentToday, err := decisionRepo.SumInvestedToday(ctx, config.UserID, config.ID, defaultTimezone)
		if err != nil {
			log.Printf("[scheduler] user=%s daily budget check failed (%v) — proceeding without cap", config.UserID, err)
		} else {
			remaining := config.DailyBudget - spentToday
			if remaining < 1.0 {
				log.Printf("[scheduler] user=%s daily budget exhausted (budget=%.2f spent=%.2f)", config.UserID, config.DailyBudget, spentToday)
				saveSkipDecision(ctx, decisionRepo, config, "daily budget exhausted for today")
				_ = notifications.SendSkipSummary(ctx, target, config.Name, "Daily budget exhausted for today.")
				return 0, nil
			}
			req.AgenticMode = true
			req.DailyBudget = config.DailyBudget
			req.SpentToday = spentToday
			req.Remaining = remaining
			req.BaseBudget = remaining
			log.Printf("[scheduler] user=%s agentic context: budget=%.2f spent=%.2f remaining=%.2f", config.UserID, config.DailyBudget, spentToday, remaining)
		}
	}

	rec, err := recommendSvc.GetDailyRecommendation(ctx, config.UserID, req)
	if err != nil {
		_ = notifications.SendInvestmentFailure(ctx, target, fmt.Sprintf("recommendation failed: %v", err))
		return 0, fmt.Errorf("recommendation: %w", err)
	}

	if rec.TotalBudget == 0 {
		// Critic block: the service's goroutine already persisted a "blocked" doc — no second write.
		if rec.WasBlocked {
			log.Printf("[scheduler] user=%s critic block already persisted — no skip doc written", config.UserID)
			return 0, nil
		}
		// Claude itself chose $0 (budget exhaustion or its own judgment).
		reason := rec.SkipReason
		if reason == "" {
			reason = rec.Summary
		}
		log.Printf("[scheduler] user=%s Claude returned $0 — skipping (reason: %s)", config.UserID, reason)
		saveSkipDecision(ctx, decisionRepo, config, reason)
		_ = notifications.SendSkipSummary(ctx, target, config.Name, reason)
		return 0, nil
	}

	// Enforce daily budget ceiling in case Claude exceeded the remaining amount.
	if isAgentic && req.AgenticMode && rec.TotalBudget > req.Remaining {
		log.Printf("[scheduler] user=%s Claude exceeded remaining (%.2f > %.2f) — capping", config.UserID, rec.TotalBudget, req.Remaining)
		scale := req.Remaining / rec.TotalBudget
		rec.TotalBudget = req.Remaining
		for i := range rec.Allocations {
			rec.Allocations[i].Amount = math.Round(rec.Allocations[i].Amount*scale*100) / 100
		}
	}

	receipts, _, err := investSvc.Execute(ctx, config.UserID, rec.Allocations, rec.TotalBudget, rec.RiskLevel, rec.Summary, rec.OverallReasoning, nil, config.ID)
	if err != nil {
		if errors.Is(err, ports.ErrBrokerageNotConnected) {
			log.Printf("[scheduler] user=%s skipping — no brokerage connected", config.UserID)
			return 0, nil
		}
		_ = notifications.SendInvestmentFailure(ctx, target, fmt.Sprintf("execution failed: %v", err))
		return 0, fmt.Errorf("execution: %w", err)
	}

	if len(receipts) == 0 {
		log.Printf("[scheduler] user=%s pipeline done — no positions placed, skipping notification", config.UserID)
		return 0, nil
	}

	allocByTicker := make(map[string]float64, len(rec.Allocations))
	for _, a := range rec.Allocations {
		allocByTicker[a.Ticker] = a.Amount
	}
	var totalFilled float64
	for i := range receipts {
		if receipts[i].FilledAmount == 0 {
			receipts[i].FilledAmount = allocByTicker[receipts[i].Ticker]
		}
		totalFilled += receipts[i].FilledAmount
	}
	if totalFilled == 0 {
		totalFilled = config.Amount
	}

	if err := notifications.SendInvestmentSummary(ctx, target, receipts, totalFilled, rec.OverallReasoning); err != nil {
		log.Printf("[scheduler] user=%s notification failed: %v", config.UserID, err)
	}

	log.Printf("[scheduler] user=%s pipeline done — %d positions placed", config.UserID, len(receipts))
	return totalFilled, nil
}

func saveSkipDecision(ctx context.Context, repo ports.DecisionRepository, config models.AutoInvestConfig, reason string) {
	d := &models.InvestmentDecision{
		UserID:       config.UserID,
		ConfigID:     config.ID,
		Timestamp:    time.Now(),
		DecisionType: "skip",
		SkipReason:   reason,
		TotalAmount:  0,
	}
	if err := repo.Save(ctx, d); err != nil {
		log.Printf("[scheduler] user=%s failed to save skip decision: %v", config.UserID, err)
	}
}

func currentTimeEST() string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Now().UTC().Format("3:04 PM UTC")
	}
	return time.Now().In(loc).Format("3:04 PM MST")
}
