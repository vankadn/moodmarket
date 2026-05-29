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
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
		return fmt.Errorf("resend: status %d: %v", resp.StatusCode, body)
	}

	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	log.Printf("[notify] resend accepted — email_id=%s to=%s subject=%q", result.ID, maskEmail(to), subject)
	return nil
}

func (r *resendEmailProvider) SendInvestmentSummary(ctx context.Context, to ports.NotificationTarget, receipts []models.TradeReceipt, totalInvested float64, overallReasoning string) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s investment summary skipped — no email configured", to.UserID)
		return nil
	}
	log.Printf("[notify] user=%s sending investment summary to %s (%d positions, $%.2f)", to.UserID, maskEmail(to.Email), len(receipts), totalInvested)
	var lines []string
	for _, rec := range receipts {
		lines = append(lines, fmt.Sprintf("<li><strong>%s</strong> — $%.2f</li>", rec.Ticker, rec.FilledAmount))
	}
	intro := "Your investment completed."
	if to.Source == "auto" {
		intro = "Your auto-invest ran today."
	}
	reasoningSection := ""
	if overallReasoning != "" {
		reasoningSection = fmt.Sprintf(`<p><strong>Why Claude invested:</strong> %s</p>`, overallReasoning)
	}
	html := fmt.Sprintf(
		`<p>%s</p><p><strong>$%.2f</strong> invested across %d position(s):</p><ul>%s</ul>%s<p>Open InvestIQ to review your portfolio.</p>`,
		intro, totalInvested, len(receipts), strings.Join(lines, ""), reasoningSection,
	)
	subject := fmt.Sprintf("Your InvestIQ summary — %s", time.Now().Format("January 2, 2006"))
	return r.send(ctx, to.Email, subject, html)
}

func (r *resendEmailProvider) SendInvestmentFailure(ctx context.Context, to ports.NotificationTarget, reason string) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s investment failure skipped — no email configured", to.UserID)
		return nil
	}
	log.Printf("[notify] user=%s sending investment failure to %s: %s", to.UserID, maskEmail(to.Email), reason)
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

func (r *resendEmailProvider) SendSkipSummary(ctx context.Context, to ports.NotificationTarget, configName, reason string) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s skip summary skipped — no email configured", to.UserID)
		return nil
	}
	log.Printf("[notify] user=%s sending skip summary to %s: config=%q reason=%s", to.UserID, maskEmail(to.Email), configName, reason)
	name := configName
	if name == "" {
		name = "your strategy"
	}
	html := fmt.Sprintf(`<p>Auto-invest skipped this run for <strong>%s</strong>.</p><p>Reason: %s</p><p>Open InvestIQ to review your settings.</p>`, name, reason)
	return r.send(ctx, to.Email, "InvestIQ: auto-invest skipped", html)
}

func (r *resendEmailProvider) SendRebalanceDigest(ctx context.Context, to ports.NotificationTarget, analysis *models.RebalanceAnalysis) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s rebalance digest skipped — no email configured", to.UserID)
		return nil
	}
	log.Printf("[notify] user=%s sending rebalance digest to %s (%d positions)", to.UserID, maskEmail(to.Email), len(analysis.Insights))

	actionLabel := map[string]string{
		"hold":      "Hold",
		"add":       "Add more",
		"trim":      "Trim",
		"reconsider": "Reconsider",
	}
	actionColor := map[string]string{
		"hold":      "#888888",
		"add":       "#27ae60",
		"trim":      "#e67e22",
		"reconsider": "#c0392b",
	}

	var rows []string
	for _, ins := range analysis.Insights {
		label := actionLabel[string(ins.SuggestedAction)]
		if label == "" {
			label = string(ins.SuggestedAction)
		}
		color := actionColor[string(ins.SuggestedAction)]
		if color == "" {
			color = "#888888"
		}
		name := ins.Name
		if name != "" {
			name = " — " + name
		}
		rows = append(rows, fmt.Sprintf(
			`<tr><td style="padding:8px 0;border-bottom:1px solid #f0f0f0"><strong>%s</strong><span style="color:#888;font-size:13px">%s</span></td><td style="padding:8px 0;border-bottom:1px solid #f0f0f0;text-align:right"><span style="color:%s;font-weight:600">%s</span></td></tr>`,
			ins.Ticker, name, color, label,
		))
		if ins.ClaudeAssessment != "" {
			rows = append(rows, fmt.Sprintf(
				`<tr><td colspan="2" style="padding:0 0 10px;font-size:13px;color:#555">%s</td></tr>`,
				ins.ClaudeAssessment,
			))
		}
	}

	date := time.Now().Format("January 2, 2006")
	html := fmt.Sprintf(`
<p><strong>Weekly portfolio review</strong></p>
<p>%s</p>
<table style="width:100%%;border-collapse:collapse;margin-top:12px">
%s
</table>
<p style="margin-top:16px;font-size:12px;color:#aaa">Suggestions only. InvestIQ never executes sells. Execute any changes yourself in your brokerage app.</p>
<p style="font-size:12px;color:#aaa">Generated %s</p>`,
		analysis.PortfolioHealthSummary,
		strings.Join(rows, ""),
		date,
	)

	return r.send(ctx, to.Email, fmt.Sprintf("InvestIQ: weekly rebalance digest — %s", date), html)
}

func (r *resendEmailProvider) SendRebalancingAlert(ctx context.Context, to ports.NotificationTarget, drifts []models.TickerDrift) error {
	if to.Email == "" {
		log.Printf("[notify] user=%s rebalancing alert skipped — no email configured", to.UserID)
		return nil
	}
	var rows []string
	for _, d := range drifts {
		direction := "overweight"
		if d.DriftPct < 0 {
			direction = "underweight"
		}
		rows = append(rows, fmt.Sprintf(
			`<li><strong>%s</strong> — target %.0f%%, actual %.0f%% (%+.1fpp %s)</li>`,
			d.Ticker, d.TargetPct, d.ActualPct, d.DriftPct, direction,
		))
	}
	html := fmt.Sprintf(
		`<p>Your portfolio has drifted from Claude's last recommendation. The following positions are outside your target allocation:</p><ul>%s</ul><p>Open InvestIQ to review and rebalance.</p>`,
		strings.Join(rows, ""),
	)
	return r.send(ctx, to.Email, "InvestIQ: portfolio rebalancing alert", html)
}
