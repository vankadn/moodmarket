// infrastructure/notifications/resend.go
package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type resendEmailProvider struct {
	apiKey string
	from   string
	client *http.Client
}

func newResendProvider(apiKey, from string) *resendEmailProvider {
	return &resendEmailProvider{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *resendEmailProvider) send(ctx context.Context, to, subject, html string) error {
	body, _ := json.Marshal(map[string]any{
		"from":    r.from,
		"to":      []string{to},
		"subject": subject,
		"html":    html,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (r *resendEmailProvider) SendInvestmentSummary(ctx context.Context, to ports.NotificationTarget, receipts []models.TradeReceipt, totalInvested float64) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s investment summary skipped — no email configured", to.UserID)
		return nil
	}
	log.Printf("[notify] user=%s sending investment summary to %s (%d positions, $%.2f)", to.UserID, to.Email, len(receipts), totalInvested)
	var lines []string
	for _, rec := range receipts {
		lines = append(lines, fmt.Sprintf("<li><strong>%s</strong> — $%.2f</li>", rec.Ticker, rec.FilledAmount))
	}
	html := fmt.Sprintf(
		`<p>Your auto-invest ran today.</p><p><strong>$%.2f</strong> invested across %d position(s):</p><ul>%s</ul><p>Open InvestIQ to review your portfolio.</p>`,
		totalInvested, len(receipts), strings.Join(lines, ""),
	)
	return r.send(ctx, to.Email, fmt.Sprintf("InvestIQ invested $%.2f today", totalInvested), html)
}

func (r *resendEmailProvider) SendInvestmentFailure(ctx context.Context, to ports.NotificationTarget, reason string) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s investment failure skipped — no email configured", to.UserID)
		return nil
	}
	log.Printf("[notify] user=%s sending investment failure to %s: %s", to.UserID, to.Email, reason)
	html := fmt.Sprintf(`<p>Your auto-invest could not run today.</p><p>Reason: %s</p><p>Open InvestIQ to check your settings.</p>`, reason)
	return r.send(ctx, to.Email, "InvestIQ: auto-invest could not run", html)
}

func (r *resendEmailProvider) SendMarketClosed(ctx context.Context, to ports.NotificationTarget, date string) error {
	if to.Email == "" {
		return nil
	}
	html := fmt.Sprintf(`<p>Auto-invest was skipped today — the market is closed (%s).</p><p>It will resume on the next trading day.</p>`, date)
	return r.send(ctx, to.Email, "InvestIQ: market closed today", html)
}
