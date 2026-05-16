// domain/ports/notification_provider.go
package ports

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// NotificationTarget holds contact details for one delivery.
// Empty Email or Phone means that channel is skipped — no error.
// Source distinguishes "manual" (user-triggered) from "auto" (scheduler-triggered).
type NotificationTarget struct {
	UserID string
	Email  string
	Phone  string
	Source string // "manual" | "auto"
}

// NotificationProvider sends user-facing notifications for investment events.
// Implementations must be non-fatal: channel failures are logged and swallowed,
// never propagated to the investment pipeline.
type NotificationProvider interface {
	SendInvestmentSummary(ctx context.Context, to NotificationTarget, receipts []models.TradeReceipt, totalInvested float64) error
	SendInvestmentFailure(ctx context.Context, to NotificationTarget, reason string) error
	SendMarketClosed(ctx context.Context, to NotificationTarget, date string) error
}
