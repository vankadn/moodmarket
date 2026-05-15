// infrastructure/notifications/log_provider.go
package notifications

import (
	"context"
	"log"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type logNotificationProvider struct{}

func (l *logNotificationProvider) SendInvestmentSummary(_ context.Context, to ports.NotificationTarget, receipts []models.TradeReceipt, total float64) error {
	log.Printf("[notify] user=%s investment complete: %d positions, $%.2f", to.UserID, len(receipts), total)
	return nil
}

func (l *logNotificationProvider) SendInvestmentFailure(_ context.Context, to ports.NotificationTarget, reason string) error {
	log.Printf("[notify] user=%s investment failed: %s", to.UserID, reason)
	return nil
}

func (l *logNotificationProvider) SendMarketClosed(_ context.Context, to ports.NotificationTarget, date string) error {
	log.Printf("[notify] user=%s market closed: %s", to.UserID, date)
	return nil
}
