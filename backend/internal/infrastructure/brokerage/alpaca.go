// infrastructure/brokerage/alpaca.go
package brokerage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

// AlpacaProvider implements BrokerageProvider against the Alpaca paper trading REST API.
// One API key = one paper account — userID in TradeOrder is for logging only.
type AlpacaProvider struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client
}

func NewAlpacaProvider(apiKey, apiSecret, baseURL string) *AlpacaProvider {
	return &AlpacaProvider{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// PlaceMarketOrder sends a notional market buy order to Alpaca.
// Notional orders are dollar-based; Alpaca computes fractional shares internally.
// time_in_force must be "day" for notional orders per Alpaca's API contract.
func (p *AlpacaProvider) PlaceMarketOrder(ctx context.Context, order models.TradeOrder) (*models.TradeReceipt, error) {
	payload := struct {
		Symbol      string `json:"symbol"`
		Notional    string `json:"notional"`
		Side        string `json:"side"`
		Type        string `json:"type"`
		TimeInForce string `json:"time_in_force"`
	}{
		Symbol:      order.Ticker,
		Notional:    strconv.FormatFloat(order.Amount, 'f', 2, 64),
		Side:        "buy",
		Type:        "market",
		TimeInForce: "day",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("alpaca: marshal order: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v2/orders", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("alpaca: build order request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: place order %s: %w", order.Ticker, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alpaca: read order response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: order API %d: %s", resp.StatusCode, rawBody)
	}

	var alpacaOrder struct {
		ID              string  `json:"id"`
		Symbol          string  `json:"symbol"`
		Status          string  `json:"status"`
		CreatedAt       string  `json:"created_at"`
		FilledNotional  *string `json:"filled_notional"`
		FilledAvgPrice  *string `json:"filled_avg_price"`
	}
	if err := json.Unmarshal(rawBody, &alpacaOrder); err != nil {
		return nil, fmt.Errorf("alpaca: parse order response: %w", err)
	}

	receipt := &models.TradeReceipt{
		OrderID:   alpacaOrder.ID,
		Ticker:    alpacaOrder.Symbol,
		Status:    alpacaOrder.Status,
		Timestamp: parseAlpacaTime(alpacaOrder.CreatedAt),
	}
	if alpacaOrder.FilledNotional != nil {
		receipt.FilledAmount, _ = strconv.ParseFloat(*alpacaOrder.FilledNotional, 64)
	}
	if alpacaOrder.FilledAvgPrice != nil {
		receipt.FilledPrice, _ = strconv.ParseFloat(*alpacaOrder.FilledAvgPrice, 64)
	}
	return receipt, nil
}

// GetPositions returns all open positions from the Alpaca account.
func (p *AlpacaProvider) GetPositions(ctx context.Context, _ string) ([]models.Position, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v2/positions", nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build positions request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: fetch positions: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alpaca: read positions response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: positions API %d: %s", resp.StatusCode, rawBody)
	}

	var alpacaPositions []struct {
		Symbol      string `json:"symbol"`
		Qty         string `json:"qty"`
		MarketValue string `json:"market_value"`
	}
	if err := json.Unmarshal(rawBody, &alpacaPositions); err != nil {
		return nil, fmt.Errorf("alpaca: parse positions response: %w", err)
	}

	positions := make([]models.Position, 0, len(alpacaPositions))
	for _, ap := range alpacaPositions {
		qty, _ := strconv.ParseFloat(ap.Qty, 64)
		mv, _ := strconv.ParseFloat(ap.MarketValue, 64)
		positions = append(positions, models.Position{
			Ticker:      ap.Symbol,
			Quantity:    qty,
			MarketValue: mv,
		})
	}
	return positions, nil
}

// GetOrder fetches the current status of a single order from Alpaca.
func (p *AlpacaProvider) GetOrder(ctx context.Context, orderID string) (*models.TradeReceipt, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/v2/orders/"+orderID, nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build get-order request: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: get order %s: %w", orderID, err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("alpaca: read order response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alpaca: get order API %d: %s", resp.StatusCode, rawBody)
	}

	var alpacaOrder struct {
		ID             string  `json:"id"`
		Symbol         string  `json:"symbol"`
		Status         string  `json:"status"`
		CreatedAt      string  `json:"created_at"`
		FilledNotional *string `json:"filled_notional"`
		FilledAvgPrice *string `json:"filled_avg_price"`
	}
	if err := json.Unmarshal(rawBody, &alpacaOrder); err != nil {
		return nil, fmt.Errorf("alpaca: parse order response: %w", err)
	}

	receipt := &models.TradeReceipt{
		OrderID:   alpacaOrder.ID,
		Ticker:    alpacaOrder.Symbol,
		Status:    alpacaOrder.Status,
		Timestamp: parseAlpacaTime(alpacaOrder.CreatedAt),
	}
	if alpacaOrder.FilledNotional != nil {
		receipt.FilledAmount, _ = strconv.ParseFloat(*alpacaOrder.FilledNotional, 64)
	}
	if alpacaOrder.FilledAvgPrice != nil {
		receipt.FilledPrice, _ = strconv.ParseFloat(*alpacaOrder.FilledAvgPrice, 64)
	}
	return receipt, nil
}

func (p *AlpacaProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("APCA-API-KEY-ID", p.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", p.apiSecret)
}

func parseAlpacaTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Now()
	}
	return t
}
