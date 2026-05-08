// infrastructure/notifications/log_provider.go
package notifications

import (
	"context"
	"log"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

type logNotificationProvider struct{}

func (l *logNotificationProvider) SendInvestmentSummary(_ context.Context, userID string, receipts []models.TradeReceipt, _ float64) error {
	log.Printf("[notify] user=%s auto-invest complete: %d positions placed", userID, len(receipts))
	return nil
}

func (l *logNotificationProvider) SendInvestmentFailure(_ context.Context, userID string, reason string) error {
	log.Printf("[notify] user=%s auto-invest failed: %s", userID, reason)
	return nil
}
