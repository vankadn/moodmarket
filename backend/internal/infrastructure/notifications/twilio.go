// infrastructure/notifications/twilio.go
package notifications

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

type twilioProvider struct {
	accountSID string
	authToken  string
	from       string
	client     *http.Client
}

func newTwilioProvider(accountSID, authToken, from string) *twilioProvider {
	return &twilioProvider{
		accountSID: accountSID,
		authToken:  authToken,
		from:       from,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *twilioProvider) send(ctx context.Context, to, body string) error {
	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.accountSID)

	data := url.Values{}
	data.Set("To", to)
	data.Set("From", t.from)
	data.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("twilio: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio: status %d", resp.StatusCode)
	}

	log.Printf("[notify] twilio SMS sent to %s", to)
	return nil
}

func (t *twilioProvider) SendInvestmentSummary(ctx context.Context, to ports.NotificationTarget, receipts []models.TradeReceipt, totalInvested float64) error {
	if to.Phone == "" {
		log.Printf("[notify] user=%s investment SMS skipped — no phone configured", to.UserID)
		return nil
	}
	tickers := make([]string, 0, len(receipts))
	for _, r := range receipts {
		tickers = append(tickers, r.Ticker)
	}
	prefix := "InvestIQ"
	if to.Source == "auto" {
		prefix = "InvestIQ auto-invest"
	}
	body := fmt.Sprintf("%s: $%.2f invested across %s.", prefix, totalInvested, strings.Join(tickers, ", "))
	return t.send(ctx, to.Phone, body)
}

func (t *twilioProvider) SendInvestmentFailure(ctx context.Context, to ports.NotificationTarget, reason string) error {
	if to.Phone == "" {
		return nil
	}
	body := fmt.Sprintf("InvestIQ: auto-invest could not run today. Reason: %s", reason)
	return t.send(ctx, to.Phone, body)
}

func (t *twilioProvider) SendMarketClosed(ctx context.Context, to ports.NotificationTarget, date string) error {
	if to.Phone == "" {
		return nil
	}
	body := fmt.Sprintf("InvestIQ: market closed today (%s). Auto-invest resumes on the next trading day.", date)
	return t.send(ctx, to.Phone, body)
}
