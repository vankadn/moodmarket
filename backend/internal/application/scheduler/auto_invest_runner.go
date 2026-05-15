// application/scheduler/auto_invest_runner.go
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/krishnarajivvns/investiq/internal/application/services"
	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// runForUser executes the full investment pipeline for a single user using their config.
// Returns the total dollar amount invested, or an error if the pipeline fails.
func runForUser(
	ctx context.Context,
	config models.AutoInvestConfig,
	target ports.NotificationTarget,
	recommendSvc *services.RecommendationService,
	investSvc *services.InvestmentService,
	notifications ports.NotificationProvider,
) (float64, error) {
	log.Printf("[scheduler] user=%s pipeline start (amount=%.0f)", config.UserID, config.Amount)

	req := models.InvestmentRequest{BaseBudget: config.Amount, ExtraMoney: 0}

	rec, err := recommendSvc.GetDailyRecommendation(ctx, config.UserID, req)
	if err != nil {
		_ = notifications.SendInvestmentFailure(ctx, target, fmt.Sprintf("recommendation failed: %v", err))
		return 0, fmt.Errorf("recommendation: %w", err)
	}

	receipts, _, err := investSvc.Execute(ctx, config.UserID, rec.Allocations, rec.TotalBudget, rec.RiskLevel, rec.Summary, nil)
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

	var totalFilled float64
	for _, r := range receipts {
		totalFilled += r.FilledAmount
	}
	if totalFilled == 0 {
		totalFilled = config.Amount // Alpaca paper orders have nil FilledNotional until async fill
	}

	if err := notifications.SendInvestmentSummary(ctx, target, receipts, totalFilled); err != nil {
		log.Printf("[scheduler] user=%s notification failed: %v", config.UserID, err)
	}

	log.Printf("[scheduler] user=%s pipeline done — %d positions placed", config.UserID, len(receipts))
	return totalFilled, nil
}
