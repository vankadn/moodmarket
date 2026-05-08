// application/scheduler/auto_invest_runner.go
package scheduler

import (
	"context"
	"fmt"
	"log"

	"github.com/krishnarajivvns/investiq/internal/application/services"
	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// runForUser executes the full investment pipeline for a single user.
// Returns the total dollar amount invested on success, or an error if the pipeline fails.
// Partial order failures (some tickers succeed, some fail) are not treated as errors here —
// they are logged inside InvestmentService and the user is notified of what did complete.
func runForUser(
	ctx context.Context,
	userID string,
	recommendSvc *services.RecommendationService,
	investSvc *services.InvestmentService,
	notifications ports.NotificationProvider,
) (float64, error) {
	log.Printf("[scheduler] user=%s pipeline start", userID)

	req := models.InvestmentRequest{BaseBudget: 100, ExtraMoney: 0}

	rec, err := recommendSvc.GetDailyRecommendation(ctx, userID, req)
	if err != nil {
		_ = notifications.SendInvestmentFailure(ctx, userID, fmt.Sprintf("recommendation failed: %v", err))
		return 0, fmt.Errorf("recommendation: %w", err)
	}

	receipts, _, err := investSvc.Execute(ctx, userID, rec.Allocations, rec.TotalBudget, rec.RiskLevel, rec.Summary)
	if err != nil {
		_ = notifications.SendInvestmentFailure(ctx, userID, fmt.Sprintf("execution failed: %v", err))
		return 0, fmt.Errorf("execution: %w", err)
	}

	var totalFilled float64
	for _, r := range receipts {
		totalFilled += r.FilledAmount
	}

	if err := notifications.SendInvestmentSummary(ctx, userID, receipts, totalFilled); err != nil {
		log.Printf("[scheduler] user=%s notification failed: %v", userID, err)
	}

	log.Printf("[scheduler] user=%s pipeline done — %d positions placed", userID, len(receipts))
	return totalFilled, nil
}
