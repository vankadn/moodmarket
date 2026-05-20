// infrastructure/brokerage/coinbase.go
package brokerage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const coinbaseBaseURL = "https://api.coinbase.com"

// CoinbaseProvider implements BrokerageProvider against the Coinbase Advanced Trade REST API.
// Auth uses HMAC-SHA256 signing: CB-ACCESS-KEY, CB-ACCESS-SIGN, CB-ACCESS-TIMESTAMP headers.
// Handles crypto-only; GetPortfolioHistory is unsupported by the Advanced Trade API.
type CoinbaseProvider struct {
	apiKey     string
	apiSecret  string
	httpClient *http.Client
}

func NewCoinbaseProvider(apiKey, apiSecret string) *CoinbaseProvider {
	return &CoinbaseProvider{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *CoinbaseProvider) sign(method, path, body string) (timestamp, sig string) {
	timestamp = strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(p.apiSecret))
	mac.Write([]byte(timestamp + method + path + body))
	return timestamp, hex.EncodeToString(mac.Sum(nil))
}

func (p *CoinbaseProvider) do(ctx context.Context, method, path string, payload []byte) ([]byte, int, error) {
	bodyStr := ""
	var reqBody io.Reader
	if payload != nil {
		bodyStr = string(payload)
		reqBody = bytes.NewReader(payload)
	}

	timestamp, sig := p.sign(method, path, bodyStr)

	req, err := http.NewRequestWithContext(ctx, method, coinbaseBaseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("coinbase: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CB-ACCESS-KEY", p.apiKey)
	req.Header.Set("CB-ACCESS-SIGN", sig)
	req.Header.Set("CB-ACCESS-TIMESTAMP", timestamp)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("coinbase: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("coinbase: read response: %w", err)
	}
	return raw, resp.StatusCode, nil
}

// productID converts a bare ticker to a Coinbase product ID: "BTC" → "BTC-USD".
func productID(ticker string) string {
	if strings.Contains(ticker, "-") {
		return ticker
	}
	return ticker + "-USD"
}

func (p *CoinbaseProvider) PlaceMarketOrder(ctx context.Context, order models.TradeOrder) (*models.TradeReceipt, error) {
	clientOrderID := fmt.Sprintf("investiq-%s-%d", order.Ticker, time.Now().UnixNano())

	payload, err := json.Marshal(map[string]any{
		"client_order_id": clientOrderID,
		"product_id":      productID(order.Ticker),
		"side":            "BUY",
		"order_configuration": map[string]any{
			"market_market_ioc": map[string]any{
				// quote_size is the USD notional amount for a buy order
				"quote_size": strconv.FormatFloat(order.Amount, 'f', 2, 64),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("coinbase: marshal order: %w", err)
	}

	raw, status, err := p.do(ctx, "POST", "/api/v3/brokerage/orders", payload)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("coinbase: order API %d: %s", status, raw)
	}

	var result struct {
		Success         bool   `json:"success"`
		OrderID         string `json:"order_id"`
		SuccessResponse *struct {
			OrderID string `json:"order_id"`
		} `json:"success_response"`
		ErrorResponse *struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		} `json:"error_response"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("coinbase: parse order response: %w", err)
	}
	if !result.Success {
		if result.ErrorResponse != nil {
			return nil, fmt.Errorf("coinbase: order failed: %s — %s", result.ErrorResponse.Error, result.ErrorResponse.Message)
		}
		return nil, fmt.Errorf("coinbase: order failed (no error detail)")
	}

	orderID := result.OrderID
	if result.SuccessResponse != nil && result.SuccessResponse.OrderID != "" {
		orderID = result.SuccessResponse.OrderID
	}

	return &models.TradeReceipt{
		OrderID:      orderID,
		Ticker:       order.Ticker,
		FilledAmount: order.Amount,
		Status:       "pending",
		Timestamp:    time.Now(),
	}, nil
}

func (p *CoinbaseProvider) GetPositions(ctx context.Context, _ string) ([]models.Position, error) {
	raw, status, err := p.do(ctx, "GET", "/api/v3/brokerage/accounts", nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("coinbase: accounts API %d: %s", status, raw)
	}

	var result struct {
		Accounts []struct {
			Name     string `json:"name"`
			Currency string `json:"currency"`
			AvailableBalance struct {
				Value string `json:"value"`
			} `json:"available_balance"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("coinbase: parse accounts response: %w", err)
	}

	var positions []models.Position
	for _, acc := range result.Accounts {
		if acc.Currency == "USD" {
			continue
		}
		qty, _ := strconv.ParseFloat(acc.AvailableBalance.Value, 64)
		if qty <= 0 {
			continue
		}
		price, err := p.GetCurrentPrice(ctx, acc.Currency)
		if err != nil || price <= 0 {
			continue
		}
		positions = append(positions, models.Position{
			Ticker:      acc.Currency,
			Name:        acc.Name,
			Quantity:    qty,
			MarketValue: qty * price,
		})
	}
	return positions, nil
}

func (p *CoinbaseProvider) GetOrder(ctx context.Context, orderID string) (*models.TradeReceipt, error) {
	path := "/api/v3/brokerage/orders/historical/" + orderID
	raw, status, err := p.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("coinbase: get order API %d: %s", status, raw)
	}

	var result struct {
		Order struct {
			OrderID             string `json:"order_id"`
			ProductID           string `json:"product_id"`
			Status              string `json:"status"`
			CreatedTime         string `json:"created_time"`
			TotalValueAfterFees string `json:"total_value_after_fees"`
			AverageFilledPrice  string `json:"average_filled_price"`
		} `json:"order"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("coinbase: parse order response: %w", err)
	}

	o := result.Order
	ticker := strings.TrimSuffix(o.ProductID, "-USD")
	filledAmt, _ := strconv.ParseFloat(o.TotalValueAfterFees, 64)
	filledPrice, _ := strconv.ParseFloat(o.AverageFilledPrice, 64)
	t, _ := time.Parse(time.RFC3339, o.CreatedTime)

	return &models.TradeReceipt{
		OrderID:      o.OrderID,
		Ticker:       ticker,
		FilledAmount: filledAmt,
		FilledPrice:  filledPrice,
		Status:       strings.ToLower(o.Status),
		Timestamp:    t,
	}, nil
}

// GetPortfolioHistory is not available in the Coinbase Advanced Trade API.
func (p *CoinbaseProvider) GetPortfolioHistory(_ context.Context, _, _, _ string) ([]models.HistoryPoint, error) {
	return nil, nil
}

func (p *CoinbaseProvider) GetCurrentPrice(ctx context.Context, ticker string) (float64, error) {
	pid := productID(ticker)
	path := "/api/v3/brokerage/best_bid_ask?product_ids=" + pid
	raw, status, err := p.do(ctx, "GET", path, nil)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("coinbase: price API %d for %s: %s", status, ticker, raw)
	}

	var result struct {
		Pricebooks []struct {
			ProductID string `json:"product_id"`
			Asks      []struct {
				Price string `json:"price"`
			} `json:"asks"`
		} `json:"pricebooks"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, fmt.Errorf("coinbase: parse price response for %s: %w", ticker, err)
	}

	for _, pb := range result.Pricebooks {
		if pb.ProductID == pid && len(pb.Asks) > 0 {
			price, _ := strconv.ParseFloat(pb.Asks[0].Price, 64)
			if price > 0 {
				return price, nil
			}
		}
	}
	return 0, fmt.Errorf("coinbase: no price available for %s", ticker)
}
