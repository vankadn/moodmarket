// infrastructure/portfolio/snaptrade.go
package portfolio

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const snapTradeBaseURL = "https://api.snaptrade.com"

// SnapTradeClient implements both PortfolioAggregator and PortfolioConnector.
// clientID identifies the app; consumerKey is used to sign every request via HMAC-SHA256.
// Per-user credentials (providerUserID, providerUserSecret) are passed per-call — never stored here.
type SnapTradeClient struct {
	clientID    string
	consumerKey string // used only for HMAC signing; never logged or exposed
	httpClient  *http.Client
}

func NewSnapTradeClient(clientID, consumerKey string) *SnapTradeClient {
	return &SnapTradeClient{
		clientID:    clientID,
		consumerKey: consumerKey,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

// RegisterUser registers a new user with SnapTrade and returns per-user provider credentials.
func (c *SnapTradeClient) RegisterUser(ctx context.Context, userID string) (string, string, error) {
	path := "/api/v1/snapTrade/registerUser"
	bodyObj := struct {
		UserID string `json:"userId"`
	}{UserID: userID}

	reqURL, queryStr := c.buildURL(path, nil)

	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return "", "", fmt.Errorf("snaptrade: register user: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("snaptrade: register user: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.signRequest(req, path, queryStr, bodyObj); err != nil {
		return "", "", fmt.Errorf("snaptrade: register user: sign: %w", err)
	}

	var resp struct {
		UserID     string `json:"userId"`
		UserSecret string `json:"userSecret"`
	}
	if err := c.do(req, &resp); err != nil {
		return "", "", fmt.Errorf("snaptrade: register user: %w", err)
	}
	if resp.UserSecret == "" {
		return "", "", fmt.Errorf("snaptrade: register user: empty userSecret in response")
	}
	return resp.UserID, resp.UserSecret, nil
}

// GenerateConnectURL returns a SnapTrade OAuth portal URL for the user to link a broker.
func (c *SnapTradeClient) GenerateConnectURL(ctx context.Context, providerUserID, providerUserSecret string) (string, error) {
	path := "/api/v1/snapTrade/login"
	extra := map[string]string{
		"userId":     providerUserID,
		"userSecret": providerUserSecret,
	}
	reqURL, queryStr := c.buildURL(path, extra)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", fmt.Errorf("snaptrade: generate connect url: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := c.signRequest(req, path, queryStr, nil); err != nil {
		return "", fmt.Errorf("snaptrade: generate connect url: sign: %w", err)
	}

	var resp struct {
		RedirectURI string `json:"redirectURI"`
	}
	if err := c.do(req, &resp); err != nil {
		return "", fmt.Errorf("snaptrade: generate connect url: %w", err)
	}
	if resp.RedirectURI == "" {
		return "", fmt.Errorf("snaptrade: generate connect url: empty redirectURI in response")
	}
	return resp.RedirectURI, nil
}

// DeleteUser de-registers the user from SnapTrade, invalidating all their linked broker connections.
func (c *SnapTradeClient) DeleteUser(ctx context.Context, providerUserID, providerUserSecret string) error {
	path := "/api/v1/snapTrade/deleteUser"
	extra := map[string]string{
		"userId":     providerUserID,
		"userSecret": providerUserSecret,
	}
	reqURL, queryStr := c.buildURL(path, extra)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("snaptrade: delete user: build request: %w", err)
	}
	if err := c.signRequest(req, path, queryStr, nil); err != nil {
		return fmt.Errorf("snaptrade: delete user: sign: %w", err)
	}

	if err := c.do(req, nil); err != nil {
		return fmt.Errorf("snaptrade: delete user: %w", err)
	}
	return nil
}

// listAccounts fetches all SnapTrade account metadata for the given user.
// Shared by GetHoldings and ListAccounts.
func (c *SnapTradeClient) listAccounts(ctx context.Context, providerUserID, providerUserSecret string) ([]snapTradeAccountMeta, error) {
	path := "/api/v1/accounts"
	extra := map[string]string{
		"userId":     providerUserID,
		"userSecret": providerUserSecret,
	}
	reqURL, queryStr := c.buildURL(path, extra)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("snaptrade: list accounts: build request: %w", err)
	}
	if err := c.signRequest(req, path, queryStr, nil); err != nil {
		return nil, fmt.Errorf("snaptrade: list accounts: sign: %w", err)
	}

	var accounts []snapTradeAccountMeta
	if err := c.do(req, &accounts); err != nil {
		return nil, fmt.Errorf("snaptrade: list accounts: %w", err)
	}
	for _, a := range accounts {
		log.Printf("[snaptrade] account id=%s name=%q institution_name=%q", a.ID, a.Name, a.InstitutionName)
	}
	return accounts, nil
}

// ListAccounts returns all brokerage accounts linked via SnapTrade for the given user.
func (c *SnapTradeClient) ListAccounts(ctx context.Context, providerUserID, providerUserSecret string) ([]models.LinkedAccount, error) {
	accounts, err := c.listAccounts(ctx, providerUserID, providerUserSecret)
	if err != nil {
		return nil, fmt.Errorf("snaptrade: ListAccounts: %w", err)
	}
	out := make([]models.LinkedAccount, len(accounts))
	for i, a := range accounts {
		displayName := a.Name
		if displayName == "" || displayName == "Default" {
			if a.InstitutionName != "" {
				displayName = a.InstitutionName
			} else if displayName == "" {
				displayName = a.ID
			}
		}
		out[i] = models.LinkedAccount{ID: a.ID, Name: displayName}
	}
	return out, nil
}

// GetHoldings fetches all positions across all linked brokerages for the given user.
// Uses two-step approach: list accounts, then fetch positions per account.
// The aggregate /holdings endpoint is not available on all plans.
func (c *SnapTradeClient) GetHoldings(ctx context.Context, providerUserID, providerUserSecret string) ([]models.Position, error) {
	// Step 1: list all accounts for this user.
	extra := map[string]string{
		"userId":     providerUserID,
		"userSecret": providerUserSecret,
	}
	accounts, err := c.listAccounts(ctx, providerUserID, providerUserSecret)
	if err != nil {
		return nil, fmt.Errorf("snaptrade: get holdings: %w", err)
	}
	log.Printf("[snaptrade] get holdings: %d account(s) found", len(accounts))

	// Step 2: fetch positions for each account.
	var positions []models.Position
	for _, account := range accounts {
		posPath := "/api/v1/accounts/" + account.ID + "/positions"
		posURL, posQuery := c.buildURL(posPath, extra)

		posReq, err := http.NewRequestWithContext(ctx, http.MethodGet, posURL, nil)
		if err != nil {
			log.Printf("[snaptrade] get holdings: positions for account %s: build request failed (%v) — skipped", account.ID, err)
			continue
		}
		if err := c.signRequest(posReq, posPath, posQuery, nil); err != nil {
			log.Printf("[snaptrade] get holdings: positions for account %s: sign failed (%v) — skipped", account.ID, err)
			continue
		}

		var raw []snapTradePosition
		if err := c.do(posReq, &raw); err != nil {
			log.Printf("[snaptrade] get holdings: positions for account %s failed (%v) — skipped", account.ID, err)
			continue
		}
		log.Printf("[snaptrade] get holdings: account %s — %d raw position(s)", account.ID, len(raw))

		for _, p := range raw {
			ticker := p.Symbol.Symbol.Symbol
			if ticker == "" {
				ticker = p.Symbol.Symbol.RawSymbol
			}
			if ticker == "" {
				log.Printf("[snaptrade] get holdings: skipping position with empty ticker (account %s)", account.ID)
				continue
			}
			mv := p.Units * p.Price
			var costBasis, avgEntry, upl float64
			if p.AveragePurchasePrice != nil {
				avgEntry = *p.AveragePurchasePrice
				costBasis = avgEntry * p.Units
			}
			if p.OpenPnl != nil {
				upl = *p.OpenPnl
			}
			var uplPct float64
			if costBasis > 0 {
				uplPct = (upl / costBasis) * 100
			}
			log.Printf("[snaptrade] get holdings: ticker=%s qty=%.4f source=account/%s", ticker, p.Units, account.ID)
			positions = append(positions, models.Position{
				Ticker:              ticker,
				Name:                p.Symbol.Symbol.Description,
				Quantity:            p.Units,
				MarketValue:         mv,
				CostBasis:           costBasis,
				AvgEntryPrice:       avgEntry,
				UnrealizedPL:        upl,
				UnrealizedPLPercent: uplPct,
			})
		}
	}
	log.Printf("[snaptrade] get holdings: returning %d position(s) across %d account(s)", len(positions), len(accounts))
	return positions, nil
}

// buildURL constructs a SnapTrade API URL with clientId and timestamp as query params.
// Returns (fullURL, queryString) — queryString is used in signature computation.
func (c *SnapTradeClient) buildURL(path string, extra map[string]string) (string, string) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	params := url.Values{}
	params.Set("clientId", c.clientID)
	params.Set("timestamp", timestamp)
	for k, v := range extra {
		params.Set(k, v)
	}
	queryStr := params.Encode() // sorted alphabetically by key
	return snapTradeBaseURL + path + "?" + queryStr, queryStr
}

// signRequest computes and sets the SnapTrade Signature header.
//
// SnapTrade signs: JSON({"content":<body>,"path":<path>,"query":<querystring>})
// using HMAC-SHA256(consumerKey, json_string) → base64.
// Keys are sorted alphabetically (content < path < query).
// SetEscapeHTML(false) is required so & in the query string is NOT escaped to &.
// body must be the parsed request body object; pass nil for bodyless requests (GET/DELETE) → JSON null.
func (c *SnapTradeClient) signRequest(req *http.Request, path, queryStr string, body any) error {
	// Keys sorted alphabetically to match SDK canonical form.
	sigObject := map[string]any{
		"content": body, // nil → JSON null for bodyless requests
		"path":    path,
		"query":   queryStr,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // keep & as-is in query string; json.Marshal would escape to &
	if err := enc.Encode(sigObject); err != nil {
		return fmt.Errorf("marshal signed content: %w", err)
	}
	// json.Encoder.Encode appends a newline; trim it before signing.
	msg := strings.TrimSuffix(buf.String(), "\n")

	mac := hmac.New(sha256.New, []byte(c.consumerKey))
	mac.Write([]byte(msg))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set("Signature", sig)
	return nil
}

// do executes the request and JSON-decodes the response into dst.
// Pass nil for dst to skip decoding (e.g. DELETE with empty response body).
func (c *SnapTradeClient) do(req *http.Request, dst any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API %d: %s", resp.StatusCode, body)
	}
	if dst == nil {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// SnapTrade response types — internal to this package.

type snapTradeAccountMeta struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	InstitutionName string `json:"institution_name"`
}

type snapTradePosition struct {
	Symbol               snapTradePositionSymbol `json:"symbol"`
	Units                float64                 `json:"units"`
	Price                float64                 `json:"price"`
	OpenPnl              *float64                `json:"open_pnl"`
	AveragePurchasePrice *float64                `json:"average_purchase_price"`
}

type snapTradePositionSymbol struct {
	Symbol snapTradeUniversalSymbol `json:"symbol"`
}

type snapTradeUniversalSymbol struct {
	Symbol      string `json:"symbol"`
	RawSymbol   string `json:"raw_symbol"`
	Description string `json:"description"`
}
