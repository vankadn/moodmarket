// infrastructure/notifications/log_provider.go
package notifications

import (
	"context"
	"log"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type logNotificationProvider struct{}

func (l *logNotificationProvider) SendInvestmentSummary(_ context.Context, to ports.NotificationTarget, receipts []models.TradeReceipt, total float64, overallReasoning string) error {
	log.Printf("[notify] user=%s investment complete: %d positions, $%.2f", to.UserID, len(receipts), total)
	if overallReasoning != "" {
		log.Printf("[notify] user=%s reasoning: %s", to.UserID, overallReasoning)
	}
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

func (l *logNotificationProvider) SendSkipSummary(_ context.Context, to ports.NotificationTarget, configName, reason string) error {
	log.Printf("[notify] user=%s auto-invest skipped: config=%q reason=%s", to.UserID, configName, reason)
	return nil
}

func (l *logNotificationProvider) SendRebalancingAlert(_ context.Context, to ports.NotificationTarget, drifts []models.TickerDrift) error {
	for _, d := range drifts {
		log.Printf("[notify] user=%s rebalancing alert: %s target=%.1f%% actual=%.1f%% drift=%+.1f%%",
			to.UserID, d.Ticker, d.TargetPct, d.ActualPct, d.DriftPct)
	}
	return nil
}

func (l *logNotificationProvider) SendRebalanceDigest(_ context.Context, to ports.NotificationTarget, analysis *models.RebalanceAnalysis) error {
	log.Printf("[notify] user=%s rebalance digest: %d positions", to.UserID, len(analysis.Insights))
	for _, ins := range analysis.Insights {
		log.Printf("[notify] user=%s  %s → %s", to.UserID, ins.Ticker, ins.SuggestedAction)
	}
	return nil
}
