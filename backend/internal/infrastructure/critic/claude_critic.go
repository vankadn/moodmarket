package critic

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
	claudeAPIURL   = "https://api.anthropic.com/v1/messages"
	claudeModel    = "claude-sonnet-4-6"
	criticTimeout  = 45 * time.Second
	criticMaxTokens = 512
	criticMaxAttempts = 2
)

const criticSystemPrompt = `You are an adversarial risk reviewer for InvestIQ. Your job is to find the strongest reason NOT to execute a specific investment recommendation — not to re-justify it.

You are skeptical by default. You are NOT the advisor who made this recommendation; you are checking their work.

BLOCK the recommendation (verdict="block") only when you find a genuinely high-risk concern, such as:
- Concentration blowup: the new allocation would push a single ticker or asset class above 40% of the user's total portfolio
- Contradicts stated risk tolerance: a conservative user being recommended high-volatility single stocks; an aggressive user being pushed into all bonds
- Earnings event missed: a single stock is included with a same-day or next-day earnings event that was not flagged in the recommendation
- Leverage or inappropriate instrument: anything that could cause a loss exceeding the invested amount
- Internal inconsistency: the recommended risk_level contradicts the actual allocations

APPROVE (verdict="approve") when the concerns are:
- Theoretical ("this could underperform")
- Already acknowledged in the recommendation's reasoning
- Minor diversification preferences with no hard rule violated
- Present but not high-severity

OUTPUT CONTRACT:
Return ONLY a raw JSON object. No markdown, no code fences, no text before or after.

{"verdict":"approve","concerns":[],"risk_level":"low","reasoning":"Allocations are consistent with the user's moderate risk tolerance and no concentration limits are violated."}
{"verdict":"block","concerns":["NVDA allocation would push Nasdaq exposure to 52%, above the 40% cap"],"risk_level":"high","reasoning":"Concentration rule violated: NVDA pushes tech exposure beyond 40% given existing positions."}

Fields:
- verdict: exactly "approve" or "block"
- concerns: array of specific findings; empty array on approve
- risk_level: exactly "low", "medium", or "high" — severity of your findings regardless of verdict
- reasoning: one sentence explaining your verdict`

// --- minimal Claude API types (no tool use needed for the critic) ---

type criticSystemBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *criticCacheControl `json:"cache_control,omitempty"`
}

type criticCacheControl struct {
	Type string `json:"type"`
}

type criticMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type criticAPIRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    []criticSystemBlock `json:"system"`
	Messages  []criticMessage     `json:"messages"`
}

type criticResponseBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type criticAPIResponse struct {
	Content    []criticResponseBlock `json:"content"`
	StopReason string                `json:"stop_reason"`
}

// claudeRecommendationCritic implements ports.RecommendationCritic via direct Claude API calls.
type claudeRecommendationCritic struct {
	apiKey     string
	httpClient *http.Client
}

func newClaudeRecommendationCritic() *claudeRecommendationCritic {
	return &claudeRecommendationCritic{
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		httpClient: &http.Client{},
	}
}

// ReviewRecommendation sends the recommendation and relevant request context to Claude
// and returns a CriticReview. Non-JSON responses trigger one correction retry.
func (c *claudeRecommendationCritic) ReviewRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, rec *models.Recommendation) (*models.CriticReview, error) {
	userMsg := buildCriticUserMessage(req, profile, rec)
	log.Printf("[critic] ══ START  allocations=%d  risk=%s  budget=$%.2f", len(rec.Allocations), rec.RiskLevel, rec.TotalBudget)

	messages := []criticMessage{{Role: "user", Content: userMsg}}

	var lastErr error
	for attempt := 1; attempt <= criticMaxAttempts; attempt++ {
		if attempt > 1 {
			log.Printf("[critic]   retry attempt %d/%d", attempt, criticMaxAttempts)
		}

		raw, err := c.doAPICall(ctx, messages)
		if err != nil {
			lastErr = err
			if attempt < criticMaxAttempts {
				backoff := time.Duration(attempt) * 3 * time.Second
				log.Printf("[critic]   API error on attempt %d: %v — backing off %s", attempt, err, backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return nil, fmt.Errorf("critic: context cancelled: %w", ctx.Err())
				}
			}
			continue
		}

		review, err := parseCriticResponse(raw)
		if err != nil {
			lastErr = err
			if attempt < criticMaxAttempts {
				log.Printf("[critic]   parse error on attempt %d — adding correction turn", attempt)
				messages = []criticMessage{
					{Role: "user", Content: userMsg},
					{Role: "assistant", Content: raw},
					{Role: "user", Content: "Your previous response was not valid JSON. Return only the corrected JSON object with no other text, no markdown, no code fences."},
				}
			}
			continue
		}

		log.Printf("[critic] ══ DONE  verdict=%s  risk=%s  reasoning=%q", review.Verdict, review.RiskLevel, review.Reasoning)
		return review, nil
	}
	return nil, fmt.Errorf("critic: all %d attempts failed: %w", criticMaxAttempts, lastErr)
}

func (c *claudeRecommendationCritic) doAPICall(parentCtx context.Context, messages []criticMessage) (string, error) {
	callCtx, cancel := context.WithTimeout(context.Background(), criticTimeout)
	defer cancel()
	go func() {
		select {
		case <-parentCtx.Done():
			cancel()
		case <-callCtx.Done():
		}
	}()

	body := criticAPIRequest{
		Model:     claudeModel,
		MaxTokens: criticMaxTokens,
		System: []criticSystemBlock{{
			Type:         "text",
			Text:         criticSystemPrompt,
			CacheControl: &criticCacheControl{Type: "ephemeral"},
		}},
		Messages: messages,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(callCtx, "POST", claudeAPIURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[critic]   HTTP %d: %s", resp.StatusCode, rawBody)
		return "", fmt.Errorf("API %d: %s", resp.StatusCode, rawBody)
	}

	var apiResp criticAPIResponse
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		return "", fmt.Errorf("envelope parse: %w", err)
	}
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", fmt.Errorf("no text block in response")
}

func parseCriticResponse(raw string) (*models.CriticReview, error) {
	text := extractCriticJSON(raw)
	var review models.CriticReview
	if err := json.Unmarshal([]byte(text), &review); err != nil {
		return nil, fmt.Errorf("parse: %w (response: %.200s)", err, text)
	}
	if review.Verdict != "approve" && review.Verdict != "block" {
		return nil, fmt.Errorf("parse: invalid verdict %q", review.Verdict)
	}
	if review.RiskLevel != "low" && review.RiskLevel != "medium" && review.RiskLevel != "high" {
		return nil, fmt.Errorf("parse: invalid risk_level %q", review.RiskLevel)
	}
	return &review, nil
}

// extractCriticJSON strips markdown fences and trims to the outermost { ... }.
func extractCriticJSON(s string) string {
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

// buildCriticUserMessage formats the recommendation, profile, and position context for the critic.
func buildCriticUserMessage(req models.InvestmentRequest, profile *models.UserProfile, rec *models.Recommendation) string {
	var sb strings.Builder
	sb.WriteString("Review this investment recommendation adversarially. Find the strongest reason NOT to execute it.\n\n")

	sb.WriteString("RECOMMENDATION TO REVIEW:\n")
	sb.WriteString(fmt.Sprintf("- Risk level: %s\n", rec.RiskLevel))
	sb.WriteString(fmt.Sprintf("- Total budget: $%.2f\n", rec.TotalBudget))
	sb.WriteString(fmt.Sprintf("- Summary: %s\n", rec.Summary))
	if rec.OverallReasoning != "" {
		sb.WriteString(fmt.Sprintf("- Advisor reasoning: %s\n", rec.OverallReasoning))
	}
	sb.WriteString("\nALLOCATIONS:\n")
	for _, a := range rec.Allocations {
		sb.WriteString(fmt.Sprintf("- %s (%s, %s): $%.2f (%.1f%%) — %s\n",
			a.Ticker, a.Name, a.AssetClass, a.Amount, a.Percentage, a.Rationale))
	}

	// Profile — critical for risk tolerance contradiction checks
	sb.WriteString("\nUSER PROFILE:\n")
	if profile != nil {
		sb.WriteString(fmt.Sprintf("- Risk tolerance: %s\n", profile.RiskTolerance))
		sb.WriteString(fmt.Sprintf("- Investment goal: %s\n", profile.InvestmentGoal))
		sb.WriteString(fmt.Sprintf("- Time horizon: %s\n", profile.TimeHorizon))
		sb.WriteString(fmt.Sprintf("- Immigration status: %s\n", profile.ImmigrationStatus))
		if profile.ExistingPortfolioValue > 0 {
			sb.WriteString(fmt.Sprintf("- Existing portfolio value: $%.0f\n", profile.ExistingPortfolioValue))
		}
	} else {
		sb.WriteString("- No profile on file — assume moderate risk tolerance.\n")
	}

	// Current positions — critical for concentration limit checks
	if len(req.Positions) > 0 {
		var totalValue float64
		for _, p := range req.Positions {
			totalValue += p.MarketValue
		}
		sb.WriteString(fmt.Sprintf("\nCURRENT PORTFOLIO POSITIONS (total value: $%.2f):\n", totalValue))
		for _, p := range req.Positions {
			pct := 0.0
			if totalValue > 0 {
				pct = p.MarketValue / totalValue * 100
			}
			sb.WriteString(fmt.Sprintf("- %s: $%.2f (%.1f%% of portfolio)\n", p.Ticker, p.MarketValue, pct))
		}
	} else {
		sb.WriteString("\nCURRENT PORTFOLIO: No existing positions.\n")
	}

	sb.WriteString("\nReturn your JSON verdict.")
	return sb.String()
}
