// infrastructure/notifications/composite.go
package notifications

import (
	"context"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

// compositeNotificationProvider fans every call out to all providers in order.
// Non-fatal: a failure from one provider is logged and does not stop the rest.
type compositeNotificationProvider struct {
	providers []ports.NotificationProvider
}

func (c *compositeNotificationProvider) SendInvestmentSummary(ctx context.Context, to ports.NotificationTarget, receipts []models.TradeReceipt, totalInvested float64, overallReasoning string) error {
	for _, p := range c.providers {
		p.SendInvestmentSummary(ctx, to, receipts, totalInvested, overallReasoning) //nolint:errcheck
	}
	return nil
}

func (c *compositeNotificationProvider) SendInvestmentFailure(ctx context.Context, to ports.NotificationTarget, reason string) error {
	for _, p := range c.providers {
		p.SendInvestmentFailure(ctx, to, reason) //nolint:errcheck
	}
	return nil
}

func (c *compositeNotificationProvider) SendMarketClosed(ctx context.Context, to ports.NotificationTarget, date string) error {
	for _, p := range c.providers {
		p.SendMarketClosed(ctx, to, date) //nolint:errcheck
	}
	return nil
}

func (c *compositeNotificationProvider) SendSkipSummary(ctx context.Context, to ports.NotificationTarget, configName, reason string) error {
	for _, p := range c.providers {
		p.SendSkipSummary(ctx, to, configName, reason) //nolint:errcheck
	}
	return nil
}

func (c *compositeNotificationProvider) SendRebalancingAlert(ctx context.Context, to ports.NotificationTarget, drifts []models.TickerDrift) error {
	for _, p := range c.providers {
		p.SendRebalancingAlert(ctx, to, drifts) //nolint:errcheck
	}
	return nil
}
