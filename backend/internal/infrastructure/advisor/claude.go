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

	"github.com/krishnarajivvns/investiq/internal/domain/models"
)

const (
	claudeAPIURL = "https://api.anthropic.com/v1/messages"
	claudeModel  = "claude-sonnet-4-6"
)

// maxAttempts caps total API calls per user request (1 initial + 1 correction).
// Raise only with deliberate intent — each attempt burns tokens.
const maxAttempts = 2

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

type claudeAPIRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system"`
	Messages  []claudeMessage `json:"messages"`
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
func (c *claudeAdvisor) GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile) (*models.Recommendation, error) {
	userMsg := buildUserMessage(req, profile)

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
			log.Printf("advisor: attempt %d/%d failed (%v), sending correction", attempt, maxAttempts, err)
			messages = []claudeMessage{
				{Role: "user", Content: userMsg},
				{Role: "assistant", Content: full},
				{Role: "user", Content: "Your previous response was not valid JSON. Return only the corrected JSON object with no other text, no markdown, no code fences."},
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
		System:    systemPrompt,
		Messages:  messages,
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

func buildUserMessage(req models.InvestmentRequest, profile *models.UserProfile) string {
	total := req.BaseBudget + req.ExtraMoney
	msg := fmt.Sprintf("Total to invest today: $%.2f\n\n", total)

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

	msg += "\nGive me today's investment allocation."
	return msg
}
