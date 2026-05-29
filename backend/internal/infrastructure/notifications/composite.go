// infrastructure/notifications/composite.go
package notifications

import (
	"context"
	"log"

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
		if err := p.SendInvestmentSummary(ctx, to, receipts, totalInvested, overallReasoning); err != nil {
			log.Printf("[notify] user=%s SendInvestmentSummary provider error: %v", to.UserID, err)
		}
	}
	return nil
}

func (c *compositeNotificationProvider) SendInvestmentFailure(ctx context.Context, to ports.NotificationTarget, reason string) error {
	for _, p := range c.providers {
		if err := p.SendInvestmentFailure(ctx, to, reason); err != nil {
			log.Printf("[notify] user=%s SendInvestmentFailure provider error: %v", to.UserID, err)
		}
	}
	return nil
}

func (c *compositeNotificationProvider) SendMarketClosed(ctx context.Context, to ports.NotificationTarget, date string) error {
	for _, p := range c.providers {
		if err := p.SendMarketClosed(ctx, to, date); err != nil {
			log.Printf("[notify] user=%s SendMarketClosed provider error: %v", to.UserID, err)
		}
	}
	return nil
}

func (c *compositeNotificationProvider) SendSkipSummary(ctx context.Context, to ports.NotificationTarget, configName, reason string) error {
	for _, p := range c.providers {
		if err := p.SendSkipSummary(ctx, to, configName, reason); err != nil {
			log.Printf("[notify] user=%s SendSkipSummary provider error: %v", to.UserID, err)
		}
	}
	return nil
}

func (c *compositeNotificationProvider) SendRebalancingAlert(ctx context.Context, to ports.NotificationTarget, drifts []models.TickerDrift) error {
	for _, p := range c.providers {
		if err := p.SendRebalancingAlert(ctx, to, drifts); err != nil {
			log.Printf("[notify] user=%s SendRebalancingAlert provider error: %v", to.UserID, err)
		}
	}
	return nil
}

func (c *compositeNotificationProvider) SendRebalanceDigest(ctx context.Context, to ports.NotificationTarget, analysis *models.RebalanceAnalysis) error {
	for _, p := range c.providers {
		if err := p.SendRebalanceDigest(ctx, to, analysis); err != nil {
			log.Printf("[notify] user=%s SendRebalanceDigest provider error: %v", to.UserID, err)
		}
	}
	return nil
}
