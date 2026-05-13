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

const (
	claudeAPIURL = "https://api.anthropic.com/v1/messages"
	claudeModel  = "claude-sonnet-4-6"
)

// maxAttempts caps total API calls per user request (1 initial + 2 retries).
const maxAttempts = 3

// maxToolIterations caps the tool-use loop to prevent runaway if Claude
// keeps requesting tools rather than returning a final answer.
const maxToolIterations = 10

// perCallTimeout is the deadline for a single HTTP round-trip to the Claude API.
// It is intentionally independent of the HTTP request context so that a short
// request deadline (e.g. a 30-second server write-timeout) does not abort a
// Claude call that started with sufficient time remaining.
const perCallTimeout = 45 * time.Second

const systemPrompt = `You are InvestIQ, a personal investment advisor. Your job is to recommend how to split a daily investment budget based on the user's complete financial picture.

AVAILABLE TOOLS:
- get_market_news: call this before making allocation decisions to retrieve today's top market news headlines.

ALLOCATION LOGIC (apply in order):
1. Risk + horizon + goal:
   - conservative + short horizon + emergency_fund/short_term_savings → 80%+ bonds/money market (BND, SHV, SGOV)
   - moderate + mid horizon + wealth_building/retirement → balanced: broad ETFs + some growth
   - aggressive + long horizon + wealth_building/retirement → growth-heavy: 20% broad ETFs, 80% growth
2. Immigration status: work_visa holders → ETFs only, no individual stocks, no complex instruments
3. Emergency fund: if missing, weight toward capital preservation until one exists
4. Existing positions: do NOT recommend any ticker where the new allocation would push the user above 40% concentration in that ticker across their total portfolio
5. Recent history: do NOT repeat the exact same allocation as the previous day — vary tickers or weights meaningfully

OUTPUT CONTRACT:
Return ONLY a raw JSON object. No markdown, no code fences, no text before or after.
All fields required. Allocations must sum exactly to total_budget.

{"total_budget":100.00,"allocations":[{"ticker":"VTI","name":"Vanguard Total Market ETF","type":"etf","amount":60.00,"percentage":60.0,"rationale":"broad US equity exposure"},{"ticker":"BND","name":"Vanguard Bond ETF","type":"etf","amount":40.00,"percentage":40.0,"rationale":"fixed income stability"}],"summary":"One sentence describing today's strategy.","risk_level":"medium"}

RULES:
- 3 to 5 allocations per recommendation
- real tickers only (SPY, VTI, BND, QQQ, AAPL, MSFT, NVDA, AMZN, SGOV, SHV, VXUS, XLE, XLF, XLV, etc.)
- risk_level: exactly one of low / medium / high
- rationale: under 12 words, specific to today's context — not generic`

// --- API request types ---

type claudeMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string for simple turns, []block for tool turns
}

type claudeCacheControl struct {
	Type string `json:"type"`
}

type claudeSystemBlock struct {
	Type         string              `json:"type"`
	Text         string              `json:"text"`
	CacheControl *claudeCacheControl `json:"cache_control,omitempty"`
}

type claudeTool struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	InputSchema claudeToolSchema `json:"input_schema"`
}

type claudeToolSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

type claudeAPIRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	System    []claudeSystemBlock `json:"system"`
	Messages  []claudeMessage     `json:"messages"`
	Tools     []claudeTool        `json:"tools,omitempty"`
}

// --- API response types ---

// claudeResponseBlock handles both "text" and "tool_use" blocks in a Claude response.
type claudeResponseBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type claudeAPIResponse struct {
	Content    []claudeResponseBlock `json:"content"`
	StopReason string                `json:"stop_reason"`
}

// claudeToolResultBlock is sent back to Claude after executing a tool call.
type claudeToolResultBlock struct {
	Type      string `json:"type"`        // always "tool_result"
	ToolUseID string `json:"tool_use_id"` //nolint:tagliatelle
	Content   string `json:"content"`
}

// --- Tool definitions ---

var marketNewsTools = []claudeTool{
	{
		Name:        "get_market_news",
		Description: "Fetch today's top SPY-tagged market news headlines from Polygon.io. Call this before recommending allocations so macro events inform your decision.",
		InputSchema: claudeToolSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
			Required:   []string{},
		},
	},
}

// --- Advisor ---

type claudeAdvisor struct {
	apiKey       string
	httpClient   *http.Client
	newsProvider ports.NewsProvider
}

func newClaudeAdvisor(newsProvider ports.NewsProvider) *claudeAdvisor {
	return &claudeAdvisor{
		apiKey:       os.Getenv("ANTHROPIC_API_KEY"),
		httpClient:   &http.Client{},
		newsProvider: newsProvider,
	}
}

// GetRecommendation builds the initial user message, then runs the tool-use loop with retries.
func (c *claudeAdvisor) GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot) (*models.Recommendation, error) {
	userMsg := buildUserMessage(req, profile, snapshot)

	log.Printf("[advisor] ══ START  budget=$%.2f  profile=%v  market=%v  positions=%d  decisions=%d",
		req.BaseBudget+req.ExtraMoney, profile != nil, snapshot != nil, len(req.Positions), len(req.RecentDecisions))
	log.Printf("[advisor]   user message built: %d chars", len(userMsg))

	messages := []claudeMessage{{Role: "user", Content: userMsg}}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			log.Printf("[advisor]   retry attempt %d/%d", attempt, maxAttempts)
		}
		rec, full, err := c.callClaudeWithTools(ctx, messages)
		if err == nil {
			log.Printf("[advisor] ══ DONE   %d allocations  risk=%s  total=$%.2f", len(rec.Allocations), rec.RiskLevel, rec.TotalBudget)
			log.Printf("[advisor]   summary: %q", rec.Summary)
			return rec, nil
		}
		lastErr = err
		if attempt < maxAttempts {
			if full != "" {
				// Claude responded but JSON was malformed — send a correction turn.
				log.Printf("[advisor]   parse error on attempt %d — adding correction turn", attempt)
				log.Printf("[advisor]   bad response (first 200 chars): %.200s", full)
				messages = []claudeMessage{
					{Role: "user", Content: userMsg},
					{Role: "assistant", Content: full},
					{Role: "user", Content: "Your previous response was not valid JSON. Return only the corrected JSON object with no other text, no markdown, no code fences."},
				}
			} else {
				// API/network error — back off before retrying.
				backoff := time.Duration(attempt) * 5 * time.Second
				log.Printf("[advisor]   API error on attempt %d: %v — backing off %s", attempt, err, backoff)
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

// callClaudeWithTools runs the multi-turn tool-use loop until Claude returns stop_reason="end_turn".
//
// Each iteration is one round-trip to Claude. The log stream shows the conversation as it builds:
//
//	TURN 1 →  Go sends user message
//	TURN 1 ←  Claude responds: wants a tool
//	          TOOL get_market_news →  Go executes locally
//	          TOOL get_market_news ←  result sent back
//	TURN 2 →  Go sends tool result (conversation now has 3 messages)
//	TURN 2 ←  Claude responds: final allocation JSON
//
// Each tool is removed from the available set after its first call, so Claude cannot
// request the same tool twice. This prevents a nil toolResults bug (nil stored in
// interface{} marshals as JSON null, not [], which the API rejects).
func (c *claudeAdvisor) callClaudeWithTools(ctx context.Context, messages []claudeMessage) (*models.Recommendation, string, error) {
	// Start with all tools available; remove each one after its first use.
	remainingTools := make([]claudeTool, len(marketNewsTools))
	copy(remainingTools, marketNewsTools)

	for turn := 1; turn <= maxToolIterations; turn++ {
		log.Printf("[advisor] TURN %d →  sending to Claude  (conversation: %d message(s), tools available: %d)", turn, len(messages), len(remainingTools))

		apiResp, err := c.doAPICall(ctx, messages, remainingTools)
		if err != nil {
			log.Printf("[advisor] TURN %d     API call failed: %v", turn, err)
			return nil, "", err
		}

		if apiResp.StopReason == "tool_use" {
			log.Printf("[advisor] TURN %d ←  stop_reason=tool_use  (%d block(s))", turn, len(apiResp.Content))

			// Echo the assistant's tool_use turn back into the conversation before executing.
			messages = append(messages, claudeMessage{Role: "assistant", Content: apiResp.Content})

			// Execute each requested tool and remove it from future turns.
			toolResults := make([]claudeToolResultBlock, 0)
			for _, block := range apiResp.Content {
				if block.Type != "tool_use" {
					continue
				}
				log.Printf("[advisor]           Claude wants: %s  (id=%s)", block.Name, block.ID)
				toolResults = append(toolResults, c.executeTool(ctx, block.Name, block.ID))
				remainingTools = removeToolByName(remainingTools, block.Name)
			}

			if len(toolResults) == 0 {
				// stop_reason=tool_use but no tool_use blocks in content — API spec violation.
				log.Printf("[advisor] TURN %d     stop_reason=tool_use but no tool_use blocks found — aborting turn", turn)
				return nil, "", fmt.Errorf("stop_reason=tool_use but no tool_use blocks in response")
			}

			// Return all tool results as a single user turn and loop.
			messages = append(messages, claudeMessage{Role: "user", Content: toolResults})
			continue
		}

		// stop_reason == "end_turn" — parse the allocation JSON from the text block.
		for _, block := range apiResp.Content {
			if block.Type != "text" {
				continue
			}
			full := strings.TrimSpace(block.Text)
			log.Printf("[advisor] TURN %d ←  stop_reason=end_turn  (%d chars)", turn, len(full))

			jsonText := extractJSON(full)
			var rec models.Recommendation
			if err := json.Unmarshal([]byte(jsonText), &rec); err != nil {
				log.Printf("[advisor]           JSON parse failed: %v", err)
				log.Printf("[advisor]           raw text: %.300s", full)
				return nil, full, fmt.Errorf("parse: %w (response: %.200s)", err, jsonText)
			}

			log.Printf("[advisor]           allocation received:")
			for _, a := range rec.Allocations {
				log.Printf("[advisor]             %-6s $%6.2f  %3.0f%%  %s", a.Ticker, a.Amount, a.Percentage, a.Rationale)
			}
			return &rec, full, nil
		}

		log.Printf("[advisor] TURN %d ←  end_turn but no text block in response", turn)
		return nil, "", fmt.Errorf("no text block in end_turn response")
	}
	return nil, "", fmt.Errorf("tool use loop exceeded %d iterations", maxToolIterations)
}

// removeToolByName returns a new slice with the named tool removed.
func removeToolByName(tools []claudeTool, name string) []claudeTool {
	out := tools[:0:0] // empty slice, no allocation if nothing to copy
	for _, t := range tools {
		if t.Name != name {
			out = append(out, t)
		}
	}
	log.Printf("[advisor]           tool %q used — removed from available tools (%d remaining)", name, len(out))
	return out
}

// doAPICall makes one HTTP round-trip to the Claude Messages API.
// It uses its own perCallTimeout context so that a tight HTTP request deadline
// (e.g. a 30-second server write-timeout) cannot abort a call that began with
// sufficient time. Parent cancellation (server shutdown, etc.) is still respected.
func (c *claudeAdvisor) doAPICall(parentCtx context.Context, messages []claudeMessage, tools []claudeTool) (*claudeAPIResponse, error) {
	callCtx, cancel := context.WithTimeout(context.Background(), perCallTimeout)
	defer cancel()
	// Propagate explicit parent cancellation without inheriting its deadline.
	go func() {
		select {
		case <-parentCtx.Done():
			cancel()
		case <-callCtx.Done():
		}
	}()

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
		Tools:    tools,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(callCtx, "POST", claudeAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("[advisor]           HTTP %d error: %s", resp.StatusCode, rawBody)
		return nil, fmt.Errorf("API %d: %s", resp.StatusCode, rawBody)
	}

	var apiResp claudeAPIResponse
	if err := json.Unmarshal(rawBody, &apiResp); err != nil {
		return nil, fmt.Errorf("envelope parse: %w", err)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	return &apiResp, nil
}

// executeTool dispatches a tool call by name and returns the result block.
// Logs use the same indentation as callClaudeWithTools so the stream reads as one story.
func (c *claudeAdvisor) executeTool(ctx context.Context, name, toolUseID string) claudeToolResultBlock {
	switch name {
	case "get_market_news":
		log.Printf("[advisor]           TOOL get_market_news →  fetching from news provider")
		items, err := c.newsProvider.GetDailyNews(ctx)
		if err != nil {
			log.Printf("[advisor]           TOOL get_market_news ←  ERROR: %v (sending degraded result)", err)
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "News unavailable today."}
		}
		if len(items) == 0 {
			log.Printf("[advisor]           TOOL get_market_news ←  0 items returned")
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "No news headlines available today."}
		}
		limit := 5
		if len(items) < limit {
			limit = len(items)
		}
		var sb strings.Builder
		log.Printf("[advisor]           TOOL get_market_news ←  %d headlines:", limit)
		for i, item := range items[:limit] {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", item.Source, item.Headline))
			log.Printf("[advisor]             [%d] [%s] %s", i+1, item.Source, item.Headline)
		}
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: sb.String()}
	default:
		log.Printf("[advisor]           TOOL %s →  unknown tool — returning error to Claude", name)
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: fmt.Sprintf("unknown tool: %s", name)}
	}
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

	if profile != nil && profile.IncludeCashContext && req.TransactionSummary != nil {
		ts := req.TransactionSummary
		msg += "\nSPENDING CONTEXT:\n"
		msg += fmt.Sprintf("- Spend last 7 days: $%.2f\n", ts.SpendLast7Days)
		msg += fmt.Sprintf("- Spend last 30 days: $%.2f\n", ts.SpendLast30Days)
		if req.BalanceSummary != nil && ts.SpendLast30Days > 0 {
			dailySpend := ts.SpendLast30Days / 30
			runway := int(req.BalanceSummary.TotalCash / dailySpend)
			msg += fmt.Sprintf("- Cash runway: ~%d days at current spend rate\n", runway)
		}
		msg += "Use as background context only — do not override the user's stated risk tolerance or investment amount.\n"
	}

	if len(req.Positions) > 0 {
		var totalValue float64
		for _, p := range req.Positions {
			totalValue += p.MarketValue
		}
		msg += "\nCURRENT BROKERAGE POSITIONS:\n"
		for _, p := range req.Positions {
			pct := 0.0
			if totalValue > 0 {
				pct = p.MarketValue / totalValue * 100
			}
			msg += fmt.Sprintf("- %s: $%.2f (%.0f%% of portfolio)", p.Ticker, p.MarketValue, pct)
			if pct >= 40 {
				msg += " ← already at concentration limit, do not add more"
			}
			msg += "\n"
		}
		msg += fmt.Sprintf("- Total portfolio value: $%.2f\n", totalValue)
		msg += "Do not recommend any ticker where the resulting position would exceed 40% of total portfolio value.\n"
	} else {
		msg += "\nBROKERAGE POSITIONS: No existing positions.\n"
	}

	if len(req.RecentDecisions) > 0 {
		limit := 5
		if len(req.RecentDecisions) < limit {
			limit = len(req.RecentDecisions)
		}
		msg += "\nRECENT INVESTMENT HISTORY (last 5):\n"
		for _, d := range req.RecentDecisions[:limit] {
			tickers := make([]string, len(d.Allocations))
			for i, a := range d.Allocations {
				tickers[i] = fmt.Sprintf("%s %.0f%%", a.Ticker, a.Percentage)
			}
			msg += fmt.Sprintf("- %s: $%.0f — %s\n", d.Timestamp.Format("Jan 2"), d.TotalAmount, strings.Join(tickers, ", "))
		}
		msg += "Vary today's allocation — do not repeat the exact same split as yesterday.\n"
	}

	if len(req.TaxDocuments) > 0 {
		msg += "\nTAX DOCUMENTS ON FILE:\n"
		for _, doc := range req.TaxDocuments {
			switch doc.DocumentType {
			case models.DocumentTypeW2:
				msg += fmt.Sprintf("- W2 %d: Gross wages $%s | Federal withheld $%s | State withheld $%s | Employer: %s\n",
					doc.TaxYear,
					doc.Fields["gross_wages"],
					doc.Fields["federal_withheld"],
					doc.Fields["state_withheld"],
					doc.Fields["employer_name"],
				)
			case models.DocumentType1099:
				msg += fmt.Sprintf("- 1099-%s %d: Income $%s | Federal withheld $%s | Payer: %s\n",
					strings.ToUpper(doc.Fields["income_type"]),
					doc.TaxYear,
					doc.Fields["gross_income"],
					doc.Fields["federal_withheld"],
					doc.Fields["payer_name"],
				)
			case models.DocumentType1098:
				msg += fmt.Sprintf("- 1098 %d: Mortgage interest paid $%s | Outstanding principal $%s | Lender: %s\n",
					doc.TaxYear,
					doc.Fields["mortgage_interest_paid"],
					doc.Fields["outstanding_principal"],
					doc.Fields["lender_name"],
				)
			}
		}
		msg += "Use tax data as additional context for income-adjusted allocation decisions.\n"
	}

	msg += "\nGive me today's investment allocation."
	return msg
}
