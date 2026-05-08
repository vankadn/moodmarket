// infrastructure/banking/plaid.go
package banking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// plaidProvider implements FinancialDataProvider using Plaid's REST API.
// All calls use net/http only — no Plaid SDK.
type plaidProvider struct {
	clientID       string
	secret         string
	baseURL        string
	httpClient     *http.Client
	cacheTTL       time.Duration
	cachedSummary  *models.BalanceSummary
	cacheExpiresAt time.Time
}

func NewPlaidProvider(clientID, secret, environment, cacheTTL string) *plaidProvider {
	baseURL := plaidBaseURL(environment)
	ttl, err := time.ParseDuration(cacheTTL)
	if err != nil || ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &plaidProvider{
		clientID:   clientID,
		secret:     secret,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		cacheTTL:   ttl,
	}
}

func plaidBaseURL(env string) string {
	switch strings.ToLower(env) {
	case "production":
		return "https://production.plaid.com"
	case "development":
		return "https://development.plaid.com"
	default:
		return "https://sandbox.plaid.com"
	}
}

// post makes an authenticated POST to a Plaid endpoint and decodes the response into out.
func (p *plaidProvider) post(ctx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("plaid: marshal request for %s: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("plaid: build request for %s: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("plaid: call %s: %w", path, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("plaid: read response from %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plaid: %s returned %d: %s", path, resp.StatusCode, rawBody)
	}
	if err := json.Unmarshal(rawBody, out); err != nil {
		return fmt.Errorf("plaid: parse response from %s: %w", path, err)
	}
	return nil
}

// authBody returns a map pre-populated with client_id and secret — all Plaid calls require these.
func (p *plaidProvider) authBody() map[string]any {
	return map[string]any{
		"client_id": p.clientID,
		"secret":    p.secret,
	}
}

// CreateLinkToken generates a Plaid Link initialization token for the given user.
func (p *plaidProvider) CreateLinkToken(ctx context.Context, userID string) (string, error) {
	body := p.authBody()
	body["user"] = map[string]string{"client_user_id": userID}
	body["client_name"] = "InvestIQ"
	body["products"] = []string{"transactions"}
	body["country_codes"] = []string{"US"}
	body["language"] = "en"

	var resp struct {
		LinkToken string `json:"link_token"`
	}
	if err := p.post(ctx, "/link/token/create", body, &resp); err != nil {
		return "", fmt.Errorf("plaid: create link token: %w", err)
	}
	return resp.LinkToken, nil
}

// ExchangePublicToken exchanges a short-lived public_token for a permanent access_token,
// then resolves the institution name via two additional Plaid calls.
func (p *plaidProvider) ExchangePublicToken(ctx context.Context, publicToken string) (string, string, string, error) {
	// Step 1: exchange public_token → access_token + item_id
	exchangeBody := p.authBody()
	exchangeBody["public_token"] = publicToken

	var exchangeResp struct {
		AccessToken string `json:"access_token"`
		ItemID      string `json:"item_id"`
	}
	if err := p.post(ctx, "/item/public_token/exchange", exchangeBody, &exchangeResp); err != nil {
		return "", "", "", fmt.Errorf("plaid: exchange token: %w", err)
	}

	// Step 2: get institution_id from item
	itemBody := p.authBody()
	itemBody["access_token"] = exchangeResp.AccessToken

	var itemResp struct {
		Item struct {
			InstitutionID string `json:"institution_id"`
		} `json:"item"`
	}
	if err := p.post(ctx, "/item/get", itemBody, &itemResp); err != nil {
		return "", "", "", fmt.Errorf("plaid: get item: %w", err)
	}

	// Step 3: resolve institution name from institution_id
	instBody := p.authBody()
	instBody["institution_id"] = itemResp.Item.InstitutionID
	instBody["country_codes"] = []string{"US"}

	var instResp struct {
		Institution struct {
			Name string `json:"name"`
		} `json:"institution"`
	}
	if err := p.post(ctx, "/institutions/get_by_id", instBody, &instResp); err != nil {
		return "", "", "", fmt.Errorf("plaid: get institution: %w", err)
	}

	return exchangeResp.AccessToken, exchangeResp.ItemID, instResp.Institution.Name, nil
}

// GetAccounts returns all accounts for a given access token.
func (p *plaidProvider) GetAccounts(ctx context.Context, accessToken string) ([]models.BankAccount, error) {
	body := p.authBody()
	body["access_token"] = accessToken

	var resp struct {
		Accounts []struct {
			AccountID string `json:"account_id"`
			Name      string `json:"name"`
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			Balances  struct {
				Current         *float64 `json:"current"`
				ISOCurrencyCode string   `json:"iso_currency_code"`
			} `json:"balances"`
		} `json:"accounts"`
		Item struct {
			InstitutionID string `json:"institution_id"`
		} `json:"item"`
	}
	if err := p.post(ctx, "/accounts/balance/get", body, &resp); err != nil {
		return nil, fmt.Errorf("plaid: get accounts: %w", err)
	}

	accounts := make([]models.BankAccount, 0, len(resp.Accounts))
	for _, a := range resp.Accounts {
		var balance float64
		if a.Balances.Current != nil {
			balance = *a.Balances.Current
		}
		currency := a.Balances.ISOCurrencyCode
		if currency == "" {
			currency = "USD"
		}
		accounts = append(accounts, models.BankAccount{
			AccountID: a.AccountID,
			Name:      a.Name,
			Type:      a.Type,
			Subtype:   a.Subtype,
			Balance:   balance,
			Currency:  currency,
		})
	}
	return accounts, nil
}

// GetBalanceSummary aggregates balances across all connected institutions.
// Failures on individual connections are logged and skipped — partial results are returned.
// Results are cached for cacheTTL to avoid hammering Plaid during development.
func (p *plaidProvider) GetBalanceSummary(ctx context.Context, connections []models.PlaidConnection) (models.BalanceSummary, error) {
	if p.cachedSummary != nil && time.Now().Before(p.cacheExpiresAt) {
		log.Printf("[plaid] balance cache hit (expires in %s)", time.Until(p.cacheExpiresAt).Round(time.Second))
		return *p.cachedSummary, nil
	}

	summary := models.BalanceSummary{
		PulledAt: time.Now(),
	}

	seenInstitutions := make(map[string]bool)

	for _, conn := range connections {
		accounts, err := p.GetAccounts(ctx, conn.AccessToken)
		if err != nil {
			log.Printf("[plaid] balance fetch failed for %s (skipped): %v", conn.Institution, err)
			continue
		}

		if !seenInstitutions[conn.Institution] {
			summary.Institutions = append(summary.Institutions, conn.Institution)
			seenInstitutions[conn.Institution] = true
		}

		for _, a := range accounts {
			summary.AccountCount++
			switch a.Type {
			case "depository":
				summary.TotalCash += a.Balance
			case "investment":
				summary.TotalInvestments += a.Balance
			}
		}
	}

	p.cachedSummary = &summary
	p.cacheExpiresAt = time.Now().Add(p.cacheTTL)
	return summary, nil
}

// GetTransactionSummary fetches up to 30 days of transactions for all connections and
// returns aggregated spend signals for the Claude prompt.
func (p *plaidProvider) GetTransactionSummary(ctx context.Context, connections []models.PlaidConnection) (models.TransactionSummary, error) {
	now := time.Now()
	endDate := now.Format("2006-01-02")
	startDate := now.AddDate(0, 0, -30).Format("2006-01-02")
	cutoff7 := now.AddDate(0, 0, -7)

	summary := models.TransactionSummary{PulledAt: now}

	for _, conn := range connections {
		body := p.authBody()
		body["access_token"] = conn.AccessToken
		body["start_date"] = startDate
		body["end_date"] = endDate
		body["options"] = map[string]any{"count": 500}

		var resp struct {
			Transactions []struct {
				Amount  float64 `json:"amount"` // positive = debit (spending)
				Date    string  `json:"date"`
				Name    string  `json:"name"`
				Pending bool    `json:"pending"`
			} `json:"transactions"`
		}
		if err := p.post(ctx, "/transactions/get", body, &resp); err != nil {
			if strings.Contains(err.Error(), "ADDITIONAL_CONSENT_REQUIRED") {
				log.Printf("[plaid] transaction fetch skipped for %s — re-authorization needed (disconnect and reconnect in app to grant transaction consent)", conn.Institution)
			} else {
				log.Printf("[plaid] transaction fetch failed for %s (skipped): %v", conn.Institution, err)
			}
			continue
		}

		log.Printf("[plaid-txn] %s: %d transaction(s) returned for %s to %s", conn.Institution, len(resp.Transactions), startDate, endDate)
		var debits, credits int
		for i, tx := range resp.Transactions {
			// Log first 5 raw transactions so we can diagnose amount signs and date format
			if i < 5 {
				log.Printf("[plaid-txn] raw[%d]: date=%s amount=%.2f pending=%v name=%q", i, tx.Date, tx.Amount, tx.Pending, tx.Name)
			}
			if tx.Amount <= 0 {
				credits++
				continue // credit, refund, or transfer — skip
			}
			debits++
			summary.SpendLast30Days += tx.Amount

			txDate, err := time.Parse("2006-01-02", tx.Date)
			if err == nil && !txDate.Before(cutoff7) {
				summary.SpendLast7Days += tx.Amount
			}

			if tx.Pending && tx.Amount > summary.LargestPendingAmount {
				summary.LargestPendingAmount = tx.Amount
				summary.LargestPendingName = tx.Name
			}
		}
		log.Printf("[plaid-txn] %s: %d debits counted, %d credits/transfers skipped", conn.Institution, debits, credits)
	}

	return summary, nil
}

// RevokeToken permanently removes an item from Plaid (item/remove).
func (p *plaidProvider) RevokeToken(ctx context.Context, accessToken string) error {
	body := p.authBody()
	body["access_token"] = accessToken

	var resp struct {
		Removed bool `json:"removed"`
	}
	if err := p.post(ctx, "/item/remove", body, &resp); err != nil {
		return fmt.Errorf("plaid: revoke token: %w", err)
	}
	return nil
}
