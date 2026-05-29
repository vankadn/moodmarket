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
	"github.com/krishnarajivvns/investiq/internal/domain/ports"
)

const rebalanceSystemPrompt = `You are InvestIQ, a portfolio review advisor. Your job is to assess the user's current holdings and suggest what to do with each position.

AVAILABLE ACTIONS — exactly one per position:
- hold: current weight is appropriate and original thesis remains intact
- add: position is underweighted or thesis has strengthened — user should consider buying more
- trim: position has grown oversized relative to its thesis but core thesis still holds — user should consider reducing
- reconsider: thesis has materially changed, risk has elevated, or position no longer fits the user's stated goals — evaluate whether to exit

RULES:
- Never instruct the user to sell anything. Suggest only. The user decides and acts in their own brokerage app.
- SnapTrade (external) positions: you may comment but must note the user must act in their external brokerage app.
- Tax sensitivity: when SHORT-TERM is flagged, factor the higher tax cost into the urgency of any trim or reconsider suggestion.
- If an original buy thesis is available, compare the position's current situation against it and explain specifically what has or hasn't changed.
- Keep each claude_assessment to 1-2 sentences. Be specific — no generic platitudes like "strong fundamentals."
- portfolio_health_summary: 1-2 sentences on overall portfolio balance and the single highest-priority action.

OUTPUT CONTRACT:
Return ONLY a raw JSON object. No markdown, no code fences, no text before or after.
Every position in the input must appear in insights, identified by its exact ticker symbol.

{"insights":[{"ticker":"AAPL","suggested_action":"hold","claude_assessment":"Position is 12% of portfolio and iPhone cycle momentum supports the original thesis."},{"ticker":"MSFT","suggested_action":"trim","claude_assessment":"Grown to 28% — overweight vs original thesis; consider reducing when tax treatment allows (note: short-term gains if sold now)."}],"portfolio_health_summary":"Portfolio is 60% tech; trimming MSFT would meaningfully reduce concentration risk."}`

const (
	rebalancePerCallTimeout = 90 * time.Second
	rebalanceMaxAttempts    = 3
	rebalanceMaxTokens      = 2048
)

// claudeRebalanceJSON is the JSON structure Claude must return.
type claudeRebalanceJSON struct {
	Insights []struct {
		Ticker           string `json:"ticker"`
		SuggestedAction  string `json:"suggested_action"`
		ClaudeAssessment string `json:"claude_assessment"`
	} `json:"insights"`
	PortfolioHealthSummary string `json:"portfolio_health_summary"`
}

type claudeRebalanceAdvisor struct {
	apiKey     string
	httpClient *http.Client
}

func newClaudeRebalanceAdvisor() *claudeRebalanceAdvisor {
	return &claudeRebalanceAdvisor{
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
		httpClient: &http.Client{},
	}
}

// AnalyzePortfolio sends the assembled portfolio to Claude and returns structured insights.
func (c *claudeRebalanceAdvisor) AnalyzePortfolio(ctx context.Context, req models.RebalanceRequest, profile *models.UserProfile) (*models.RebalanceAnalysis, error) {
	if len(req.Positions) == 0 {
		return &models.RebalanceAnalysis{
			PortfolioHealthSummary: "No positions to analyze.",
			GeneratedAt:            time.Now(),
		}, nil
	}

	userMsg := buildRebalanceUserMessage(req, profile)
	alpaca, snap := countBySource(req.Positions)
	log.Printf("[rebalance] ══ START  positions=%d  alpaca=%d  snaptrade=%d  reasoning=%d",
		len(req.Positions), alpaca, snap, len(req.BuyReasoningByTicker))
	log.Printf("[rebalance]   user message: %d chars", len(userMsg))

	messages := []claudeMessage{{Role: "user", Content: userMsg}}

	var lastErr error
	for attempt := 1; attempt <= rebalanceMaxAttempts; attempt++ {
		if attempt > 1 {
			log.Printf("[rebalance]   retry attempt %d/%d", attempt, rebalanceMaxAttempts)
		}

		raw, err := c.doAPICall(ctx, messages)
		if err != nil {
			lastErr = err
			if attempt < rebalanceMaxAttempts {
				if strings.Contains(err.Error(), "529") {
					return nil, fmt.Errorf("%w", ports.ErrAdvisorOverloaded)
				}
				backoff := time.Duration(attempt) * 5 * time.Second
				log.Printf("[rebalance]   API error on attempt %d: %v — backing off %s", attempt, err, backoff)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return nil, fmt.Errorf("rebalance advisor: context cancelled during backoff: %w", ctx.Err())
				}
			}
			continue
		}

		analysis, err := c.parseResponse(raw, req)
		if err != nil {
			lastErr = err
			if attempt < rebalanceMaxAttempts {
				log.Printf("[rebalance]   parse error on attempt %d — adding correction turn", attempt)
				log.Printf("[rebalance]   bad response (first 200 chars): %.200s", raw)
				messages = []claudeMessage{
					{Role: "user", Content: userMsg},
					{Role: "assistant", Content: raw},
					{Role: "user", Content: "Your previous response was not valid JSON. Return only the corrected JSON object with no other text, no markdown, no code fences."},
				}
			}
			continue
		}

		log.Printf("[rebalance] ══ DONE  %d insights", len(analysis.Insights))
		log.Printf("[rebalance]   summary: %q", analysis.PortfolioHealthSummary)
		for _, ins := range analysis.Insights {
			log.Printf("[rebalance]   %-6s  %-11s  tax=%-10s  %s", ins.Ticker, ins.SuggestedAction, ins.TaxFlag, ins.ClaudeAssessment)
		}
		return analysis, nil
	}
	return nil, fmt.Errorf("rebalance advisor: all %d attempts failed: %w", rebalanceMaxAttempts, lastErr)
}

func (c *claudeRebalanceAdvisor) doAPICall(parentCtx context.Context, messages []claudeMessage) (string, error) {
	callCtx, cancel := context.WithTimeout(context.Background(), rebalancePerCallTimeout)
	defer cancel()
	go func() {
		select {
		case <-parentCtx.Done():
			cancel()
		case <-callCtx.Done():
		}
	}()

	system := []claudeSystemBlock{{
		Type:         "text",
		Text:         rebalanceSystemPrompt,
		CacheControl: &claudeCacheControl{Type: "ephemeral"},
	}}

	body := claudeAPIRequest{
		Model:     claudeModel,
		MaxTokens: rebalanceMaxTokens,
		System:    system,
		Messages:  messages,
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
		log.Printf("[rebalance]   HTTP %d: %s", resp.StatusCode, rawBody)
		if resp.StatusCode == 529 {
			return "", fmt.Errorf("%w: API 529: %s", ports.ErrAdvisorOverloaded, rawBody)
		}
		return "", fmt.Errorf("API %d: %s", resp.StatusCode, rawBody)
	}

	var apiResp claudeAPIResponse
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		return "", fmt.Errorf("envelope parse: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response")
	}
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			return strings.TrimSpace(block.Text), nil
		}
	}
	return "", fmt.Errorf("no text block in response")
}

func (c *claudeRebalanceAdvisor) parseResponse(raw string, req models.RebalanceRequest) (*models.RebalanceAnalysis, error) {
	jsonText := extractJSON(raw) // reuses extractJSON from claude.go
	var parsed claudeRebalanceJSON
	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return nil, fmt.Errorf("parse: %w (response: %.200s)", err, jsonText)
	}
	if parsed.PortfolioHealthSummary == "" {
		return nil, fmt.Errorf("parse: missing portfolio_health_summary")
	}

	// Index Claude's output by uppercase ticker for fast lookup.
	type insightData struct{ action, assessment string }
	byTicker := make(map[string]insightData, len(parsed.Insights))
	for _, ins := range parsed.Insights {
		byTicker[strings.ToUpper(ins.Ticker)] = insightData{
			action:     ins.SuggestedAction,
			assessment: ins.ClaudeAssessment,
		}
	}

	insights := make([]models.PositionInsight, 0, len(req.Positions))
	for _, p := range req.Positions {
		data, ok := byTicker[strings.ToUpper(p.Ticker)]
		if !ok {
			log.Printf("[rebalance]   ticker %s absent from Claude response — defaulting to hold", p.Ticker)
			data = insightData{action: string(models.ActionHold), assessment: "No assessment provided."}
		}

		action := models.SuggestedAction(data.action)
		switch action {
		case models.ActionHold, models.ActionAdd, models.ActionTrim, models.ActionReconsider:
		default:
			log.Printf("[rebalance]   unrecognized action %q for %s — defaulting to hold", data.action, p.Ticker)
			action = models.ActionHold
		}

		insights = append(insights, models.PositionInsight{
			Ticker:            p.Ticker,
			Name:              p.Name,
			Source:            p.Source,
			AccountName:       p.AccountName,
			CurrentValue:      p.MarketValue,
			UnrealizedPL:      p.UnrealizedPL,
			UnrealizedPLPct:   p.UnrealizedPLPercent,
			OriginalBuyThesis: req.BuyReasoningByTicker[p.Ticker],
			ClaudeAssessment:  data.assessment,
			SuggestedAction:   action,
			TaxFlag:           computeTaxFlag(p.Ticker, req.FirstPurchaseByTicker),
		})
	}

	return &models.RebalanceAnalysis{
		Insights:               insights,
		PortfolioHealthSummary: parsed.PortfolioHealthSummary,
		GeneratedAt:            time.Now(),
	}, nil
}

// buildRebalanceUserMessage formats the full position list + profile for Claude.
func buildRebalanceUserMessage(req models.RebalanceRequest, profile *models.UserProfile) string {
	var sb strings.Builder
	sb.WriteString("Review my current portfolio and assess each position.\n\n")

	if profile != nil {
		sb.WriteString("USER PROFILE:\n")
		sb.WriteString(fmt.Sprintf("- Risk tolerance: %s\n", profile.RiskTolerance))
		sb.WriteString(fmt.Sprintf("- Investment goal: %s\n", profile.InvestmentGoal))
		sb.WriteString(fmt.Sprintf("- Time horizon: %s\n", profile.TimeHorizon))
		if profile.ExistingPortfolioValue > 0 {
			sb.WriteString(fmt.Sprintf("- Portfolio value on file: $%.0f\n", profile.ExistingPortfolioValue))
		}
	} else {
		sb.WriteString("USER PROFILE: Not available — use a balanced moderate approach.\n")
	}
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("CURRENT POSITIONS (%d):\n", len(req.Positions)))
	for i, p := range req.Positions {
		sourceLabel := "Alpaca (InvestIQ-managed)"
		if p.Source == "snaptrade" {
			if p.AccountName != "" {
				sourceLabel = p.AccountName + " — read-only, must act externally"
			} else {
				sourceLabel = "external account — read-only, must act externally"
			}
		}

		sb.WriteString(fmt.Sprintf("%d. %s", i+1, p.Ticker))
		if p.Name != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", p.Name))
		}
		sb.WriteString(fmt.Sprintf(" — %s\n", sourceLabel))
		sb.WriteString(fmt.Sprintf("   Value: $%.2f", p.MarketValue))
		if p.UnrealizedPL != 0 {
			sb.WriteString(fmt.Sprintf(" | Unrealized P&L: %+.2f (%.2f%%)", p.UnrealizedPL, p.UnrealizedPLPercent))
		}
		sb.WriteString("\n")

		flag := computeTaxFlag(p.Ticker, req.FirstPurchaseByTicker)
		switch flag {
		case models.TaxFlagShortTerm:
			t := req.FirstPurchaseByTicker[p.Ticker]
			sb.WriteString(fmt.Sprintf("   Tax: SHORT-TERM (held since %s — selling triggers higher capital gains rate)\n", t.Format("2006-01-02")))
		case models.TaxFlagLongTerm:
			t := req.FirstPurchaseByTicker[p.Ticker]
			sb.WriteString(fmt.Sprintf("   Tax: LONG-TERM (held since %s — lower capital gains rate applies)\n", t.Format("2006-01-02")))
		default:
			sb.WriteString("   Tax: unknown (no purchase history in InvestIQ)\n")
		}

		if thesis := req.BuyReasoningByTicker[p.Ticker]; thesis != "" {
			sb.WriteString(fmt.Sprintf("   Original buy thesis: %q\n", thesis))
		} else {
			sb.WriteString("   Original buy thesis: not available\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Assess each position and return your JSON analysis.")
	return sb.String()
}

// computeTaxFlag determines the likely capital gains treatment for a ticker.
// Uses AddDate(1,0,0) to match the IRS "more than 1 year" long-term threshold exactly.
func computeTaxFlag(ticker string, firstPurchase map[string]time.Time) models.TaxFlag {
	t, ok := firstPurchase[ticker]
	if !ok || t.IsZero() {
		return models.TaxFlagUnknown
	}
	if time.Now().After(t.AddDate(1, 0, 0)) {
		return models.TaxFlagLongTerm
	}
	return models.TaxFlagShortTerm
}

// countBySource returns (alpacaCount, snaptradeCount) for logging.
func countBySource(positions []models.RebalancePosition) (int, int) {
	var a, s int
	for _, p := range positions {
		if p.Source == "alpaca" {
			a++
		} else {
			s++
		}
	}
	return a, s
}
