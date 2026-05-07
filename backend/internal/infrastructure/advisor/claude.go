package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const (
	claudeAPIURL = "https://api.anthropic.com/v1/messages"
	claudeModel  = "claude-sonnet-4-6"
)

// maxAttempts caps total API calls per user request (1 initial + 2 retries).
// Raise only with deliberate intent — each attempt burns tokens.
const maxAttempts = 3

const systemPrompt = `You are InvestIQ, a personal investment advisor. Recommend how to split a daily investment budget based on the user's complete financial profile.

PROFILE-DRIVEN LOGIC:
- conservative + short horizon + emergency_fund/short_term_savings: 80%+ bonds/money market (BND, SHV, SGOV)
- moderate + mid horizon + wealth_building/retirement: 50% broad ETFs, 50% growth stocks
- aggressive + long horizon + wealth_building/retirement: 20% broad ETFs, 80% growth stocks
- work_visa holders: prefer ETFs over individual stocks, avoid complex instruments

Always factor in existing portfolio size and whether the user has an emergency fund.

Return ONLY a raw JSON object. Do not use markdown, code fences, or any text before or after the JSON.
Use standard ASCII colons as key-value separators. The response must be parseable by Go's encoding/json.

Example of the exact format required:
{"total_budget":100.00,"allocations":[{"ticker":"VTI","name":"Vanguard Total Market ETF","type":"etf","amount":60.00,"percentage":60.0,"rationale":"broad US equity exposure"},{"ticker":"BND","name":"Vanguard Bond ETF","type":"etf","amount":40.00,"percentage":40.0,"rationale":"fixed income stability"}],"summary":"Balanced split between equities and bonds.","risk_level":"medium"}

Rules:
- allocations must sum exactly to total_budget
- 3 to 5 allocations per recommendation
- real tickers only (SPY, VTI, BND, QQQ, AAPL, MSFT, NVDA, AMZN, SGOV, SHV, etc.)
- risk_level must be exactly one of: low, medium, high
- rationale under 12 words each`

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type claudeSystemBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *claudeCacheControl `json:"cache_control,omitempty"`
}

type claudeAPIRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    []claudeSystemBlock `json:"system"`
	Messages  []claudeMessage     `json:"messages"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeAPIResponse struct {
	Content []claudeContentBlock `json:"content"`
}

type claudeAdvisor struct {
	apiKey     string
	httpClient *http.Client
}

func newClaudeAdvisor() *claudeAdvisor {
	return &claudeAdvisor{
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		httpClient: &http.Client{},
	}
}

// GetRecommendation calls Claude with a JSON prefill and retries once on parse failure.
func (c *claudeAdvisor) GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot) (*models.Recommendation, error) {
	userMsg := buildUserMessage(req, profile, snapshot)
	log.Printf("[claude] ── USER MESSAGE ──────────────────────────────────────────\n%s\n[claude] ────────────────────────────────────────────────────────────", userMsg)

	messages := []claudeMessage{
		{Role: "user", Content: userMsg},
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		rec, full, err := c.callClaude(ctx, messages)
		if err == nil {
			return rec, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			if full != "" {
				// Claude responded but the JSON was malformed — send a correction turn.
				log.Printf("advisor: attempt %d/%d failed (parse error), sending correction", attempt, maxAttempts)
				messages = []claudeMessage{
					{Role: "user", Content: userMsg},
					{Role: "assistant", Content: full},
					{Role: "user", Content: "Your previous response was not valid JSON. Return only the corrected JSON object with no other text, no markdown, no code fences."},
				}
			} else {
				// API/network error — back off before retrying so we don't immediately
				// hammer an already-overloaded server (529 is the common case here).
				backoff := time.Duration(attempt) * 5 * time.Second
				log.Printf("advisor: attempt %d/%d failed (API error: %v), retrying in %s", attempt, maxAttempts, err, backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return nil, fmt.Errorf("advisor: context cancelled during backoff: %w", ctx.Err())
				}
			}
		}
	}
	return nil, fmt.Errorf("advisor: all %d attempts failed: %w", maxAttempts, lastErr)
}

// callClaude makes one API round-trip. Returns (rec, fullResponseText, error).
func (c *claudeAdvisor) callClaude(ctx context.Context, messages []claudeMessage) (*models.Recommendation, string, error) {
	body := claudeAPIRequest{
		Model:     claudeModel,
		MaxTokens: 1024,
		System: []claudeSystemBlock{
			{
				Type:         "text",
				Text:         systemPrompt,
				CacheControl: &claudeCacheControl{Type: "ephemeral"},
			},
		},
		Messages: messages,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", claudeAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("API %d: %s", resp.StatusCode, rawBody)
	}

	var apiResp claudeAPIResponse
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		return nil, "", fmt.Errorf("envelope parse: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return nil, "", fmt.Errorf("empty response")
	}

	full := strings.TrimSpace(apiResp.Content[0].Text)
	jsonText := extractJSON(full)

	var rec models.Recommendation
	if err := json.Unmarshal([]byte(jsonText), &rec); err != nil {
		return nil, full, fmt.Errorf("parse: %w (response: %.200s)", err, jsonText)
	}
	return &rec, full, nil
}

// extractJSON strips markdown code fences and trims to the outermost { ... }.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl != -1 {
			s = s[nl+1:]
		}
		if end := strings.LastIndex(s, "```"); end != -1 {
			s = s[:end]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}

func buildUserMessage(req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot) string {
	total := req.BaseBudget + req.ExtraMoney
	msg := fmt.Sprintf("Total to invest today: $%.2f\n\n", total)

	if snapshot != nil {
		msg += fmt.Sprintf("Today's market context (%s):\n", snapshot.Date)
		msg += fmt.Sprintf("- Overall sentiment: %s\n", snapshot.MarketSentiment)
		msg += fmt.Sprintf("- SPY (S&P 500 ETF): %+.2f%%\n", snapshot.SPYChangePercent)
		msg += fmt.Sprintf("- QQQ (Nasdaq ETF): %+.2f%%\n", snapshot.QQQChangePercent)
		if len(snapshot.TopMovers) > 0 {
			msg += "- Top moving sectors today:\n"
			for _, m := range snapshot.TopMovers {
				msg += fmt.Sprintf("  • %s: %+.2f%%\n", m.Symbol, m.ChangePercent)
			}
		}
		msg += "\n"
	}

	if profile != nil {
		msg += "User financial profile:\n"
		msg += fmt.Sprintf("- Name: %s\n", profile.FullName)
		msg += fmt.Sprintf("- Annual salary: $%.0f\n", profile.Salary)
		msg += fmt.Sprintf("- Monthly savings: $%.0f\n", profile.MonthlySavings)
		msg += fmt.Sprintf("- Retirement contribution: %.0f%%\n", profile.RetirementContributionPct)
		msg += fmt.Sprintf("- Existing portfolio value: $%.0f\n", profile.ExistingPortfolioValue)
		msg += fmt.Sprintf("- Time horizon: %s\n", profile.TimeHorizon)
		msg += fmt.Sprintf("- Immigration status: %s\n", profile.ImmigrationStatus)
		msg += fmt.Sprintf("- Risk tolerance: %s\n", profile.RiskTolerance)
		msg += fmt.Sprintf("- Investment goal: %s\n", profile.InvestmentGoal)
		msg += fmt.Sprintf("- Has emergency fund: %v\n", profile.HasEmergencyFund)
	} else {
		msg += "No profile on file. Use a balanced moderate strategy.\n"
	}

	if req.BalanceSummary != nil {
		s := req.BalanceSummary
		msg += "\nCONNECTED FINANCIAL ACCOUNTS (live Plaid data):\n"
		msg += fmt.Sprintf("- Total cash (checking/savings): $%.2f\n", s.TotalCash)
		msg += fmt.Sprintf("- Total investments (brokerage/retirement): $%.2f\n", s.TotalInvestments)
		if len(s.Institutions) > 0 {
			msg += fmt.Sprintf("- Connected institutions: %s\n", strings.Join(s.Institutions, ", "))
		}
		msg += fmt.Sprintf("- Accounts connected: %d\n", s.AccountCount)
		msg += fmt.Sprintf("- Data pulled at: %s\n", s.PulledAt.Format(time.RFC3339))
	} else {
		msg += "\nFINANCIAL ACCOUNTS: No bank accounts connected. Using profile estimates only.\n"
	}

	msg += "\nGive me today's investment allocation."
	return msg
}
