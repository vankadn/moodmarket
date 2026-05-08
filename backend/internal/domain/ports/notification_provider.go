// domain/ports/notification_provider.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// NotificationProvider sends user-facing notifications after autonomous investment runs.
// The dev implementation logs to stdout; swap for FCM/APNs before go-live.
type NotificationProvider interface {
	SendInvestmentSummary(ctx context.Context, userID string, receipts []models.TradeReceipt, totalInvested float64) error
	SendInvestmentFailure(ctx context.Context, userID string, reason string) error
}
