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
	"strconv"
	"sort"
	"strings"
	"sync"
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
- get_earnings_calendar: call this for any individual stock (not ETFs) you are considering to check if it reports earnings in the next 7 days.
- get_fundamentals: [bargain_hunter] fetch PE, ForwardPE, ForwardPEG, 52-week range with dates, debt ratios, and current ratio for a stock ticker.
- get_earnings_surprises: [bargain_hunter] fetch last N quarters of EPS actual vs estimate for a stock ticker.
- get_insider_activity: [bargain_hunter] fetch monthly insider sentiment (MSPR, net share changes, ConsecutiveNegativeMonths) for a stock ticker.

EARNINGS AWARENESS:
- Always call get_earnings_calendar before recommending any individual stock ticker (e.g. AAPL, CRM, NVDA). Never call it for ETFs (SPY, QQQ, VTI, BND, XLE, etc.).
- If a stock reports earnings within 3 days: reduce its allocation by at least 50% or substitute a sector ETF instead. Add "reports [date]" to its rationale field.
- If a stock reports earnings 4–7 days out: you may still include it but note the date in the rationale.
- If no earnings are found in the next 7 days: proceed normally.

ALLOCATION LOGIC (apply in order):
1. Risk + horizon + goal:
   - conservative + short horizon + emergency_fund/short_term_savings → 80%+ bonds/money market (BND, SHV, SGOV)
   - moderate + mid horizon + wealth_building/retirement → balanced: broad ETFs + some growth
   - aggressive + long horizon + wealth_building/retirement → growth-heavy: 20% broad ETFs, 80% growth
2. Immigration status: work_visa holders → ETFs only, no individual stocks, no complex instruments
3. Emergency fund: if missing, weight toward capital preservation until one exists
4. Existing positions: do NOT recommend any ticker where the new allocation would push the user above 40% concentration in that ticker across their total portfolio
5. Recent history: do NOT repeat the exact same allocation as the previous day — vary tickers or weights meaningfully
6. CONCENTRATION RULE: Do not recommend any allocation that would push an asset class already above 40% concentration higher. If all asset classes are below 40%, allocate freely based on risk profile and market context.
7. TICKER RULE: Prefer tickers not already held in the portfolio unless the position is below 5% and adding more is justified by today's market context.
8. RECENT BLOCKS: if a RECENT BLOCKS section appears in the user message, do not repeat the same risk_level or asset-class concentration that triggered those blocks.

OUTPUT CONTRACT:
Return ONLY a raw JSON object. No markdown, no code fences, no text before or after.
All fields required. Allocations must sum exactly to total_budget.

{"total_budget":100.00,"allocations":[{"ticker":"VTI","asset_class":"US Equity","name":"Vanguard Total Market ETF","type":"etf","amount":60.00,"percentage":60.0,"rationale":"broad US equity exposure","reasoning":"US equities show resilience amid rate stability; VTI gives broadest market coverage."},{"ticker":"BND","asset_class":"Bonds","name":"Vanguard Bond ETF","type":"etf","amount":40.00,"percentage":40.0,"rationale":"fixed income stability","reasoning":"Bonds hedge equity risk given mixed macro signals today."}],"summary":"One sentence describing today's strategy.","risk_level":"medium","overall_reasoning":"Balanced split captures equity upside while bonds cushion against today's rate uncertainty."}

RULES:
- 3 to 5 allocations per recommendation
- real tickers only (SPY, VTI, BND, QQQ, AAPL, MSFT, NVDA, AMZN, SGOV, SHV, VXUS, XLE, XLF, XLV, etc.)
- asset_class: the asset class of the ticker (US Equity, International, Bonds, Real Estate, Commodities, or Other)
- risk_level: exactly one of low / medium / high
- rationale: under 12 words, specific to today's context — not generic
- reasoning: one sentence per ticker explaining the specific pick in today's market context
- overall_reasoning: 1-2 sentences explaining why this allocation makes sense right now`

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
	{
		Name:        "get_earnings_calendar",
		Description: "Fetch upcoming earnings dates for a specific stock ticker from Finnhub. Call this for any individual stock you are considering recommending (not ETFs) to check if it reports within the next 7 days.",
		InputSchema: claudeToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"ticker": map[string]interface{}{
					"type":        "string",
					"description": "The stock ticker symbol to look up, e.g. AAPL, CRM, NVDA.",
				},
			},
			Required: []string{"ticker"},
		},
	},
}

// earningsCacheEntry holds a single ticker's earnings result for up to earningsCacheTTL.
type earningsCacheEntry struct {
	result string
	exp    time.Time
}

const earningsCacheTTL = 1 * time.Hour
const finnhubEarningsBaseURL = "https://finnhub.io/api/v1/calendar/earnings"

// fundamentalsTools are registered alongside marketNewsTools for bargain_hunter strategy calls.
// All three are multi-use (one call per candidate ticker, same pattern as get_earnings_calendar).
var fundamentalsTools = []claudeTool{
	{
		Name:        "get_fundamentals",
		Description: "Fetch key value metrics for a stock ticker from Finnhub: PE (trailing), ForwardPE, ForwardPEG, 52-week high/low with dates, LongTermDebtToEquity, TotalDebtToEquity, CurrentRatio. Call this for each individual stock candidate in a bargain_hunter strategy.",
		InputSchema: claudeToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"ticker": map[string]interface{}{
					"type":        "string",
					"description": "Stock ticker symbol, e.g. MU, AAPL, NVDA.",
				},
			},
			Required: []string{"ticker"},
		},
	},
	{
		Name:        "get_earnings_surprises",
		Description: "Fetch the last N quarters of EPS actuals vs analyst estimates for a stock ticker from Finnhub. Validates whether a forward-PE-cheap thesis is supported by recent earnings delivery.",
		InputSchema: claudeToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"ticker": map[string]interface{}{
					"type":        "string",
					"description": "Stock ticker symbol.",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Number of quarters to return (default 4, max 8).",
				},
			},
			Required: []string{"ticker"},
		},
	},
	{
		Name:        "get_insider_activity",
		Description: "Fetch insider sentiment data for a stock ticker from Finnhub: monthly MSPR display data, and ConsecutiveNegativeMonths (months since last genuine open-market purchase, code P, price > $0 — grants and awards at $0 do NOT reset this counter). Also returns LastGenuinePurchase date. Required for bargain_hunter to surface sustained insider selling.",
		InputSchema: claudeToolSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"ticker": map[string]interface{}{
					"type":        "string",
					"description": "Stock ticker symbol.",
				},
			},
			Required: []string{"ticker"},
		},
	},
}

// multiUseTools lists tool names that are NOT removed after their first call —
// Claude may invoke them once per ticker candidate in the same recommendation.
var multiUseTools = map[string]bool{
	"get_earnings_calendar": true,
	"get_fundamentals":      true,
	"get_earnings_surprises": true,
	"get_insider_activity":  true,
}

// --- Advisor ---

type claudeAdvisor struct {
	apiKey               string
	httpClient           *http.Client
	newsProvider         ports.NewsProvider
	fundamentalsProvider ports.FundamentalsProvider // nil when not configured; tools return degraded responses
	classifier           ports.Classifier
	classRepo            ports.ClassificationRepository

	earningsMu    sync.RWMutex
	earningsCache map[string]earningsCacheEntry
}

func newClaudeAdvisor(newsProvider ports.NewsProvider, fundamentalsProvider ports.FundamentalsProvider, classifier ports.Classifier, classRepo ports.ClassificationRepository) *claudeAdvisor {
	return &claudeAdvisor{
		apiKey:               os.Getenv("ANTHROPIC_API_KEY"),
		httpClient:           &http.Client{},
		newsProvider:         newsProvider,
		fundamentalsProvider: fundamentalsProvider,
		classifier:           classifier,
		classRepo:            classRepo,
		earningsCache:        make(map[string]earningsCacheEntry),
	}
}

// GetRecommendation builds the initial user message, then runs the tool-use loop with retries.
func (c *claudeAdvisor) GetRecommendation(ctx context.Context, req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot) (*models.Recommendation, error) {
	userMsg := buildUserMessage(req, profile, snapshot, c.classifier)

	log.Printf("[advisor] ══ START  budget=$%.2f  profile=%v  market=%v  positions=%d  decisions=%d",
		req.BaseBudget+req.ExtraMoney, profile != nil, snapshot != nil, len(req.Positions), len(req.RecentDecisions))
	log.Printf("[advisor]   user message built: %d chars", len(userMsg))

	// Resolve strategy prompt: explicit StrategyPrompt wins; fall back to StrategyType lookup.
	resolvedStrategyPrompt := req.StrategyPrompt
	if resolvedStrategyPrompt == "" && req.StrategyType != "" {
		resolvedStrategyPrompt = strategySystemPrompt(req.StrategyType)
	}

	fullSystem := systemPrompt
	if resolvedStrategyPrompt != "" {
		fullSystem = "[STRATEGY PREFIX]\n" + resolvedStrategyPrompt + "\n\n[BASE SYSTEM PROMPT]\n" + systemPrompt
	}
	log.Printf("[advisor] ── prompt sizes: system=%d chars  user=%d chars  strategy=%s", len(fullSystem), len(userMsg), req.StrategyType)

	messages := []claudeMessage{{Role: "user", Content: userMsg}}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			log.Printf("[advisor]   retry attempt %d/%d", attempt, maxAttempts)
		}
		rec, full, err := c.callClaudeWithTools(ctx, messages, resolvedStrategyPrompt)
		if err == nil {
			c.classifyAndStore(rec.Allocations)
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
func (c *claudeAdvisor) callClaudeWithTools(ctx context.Context, messages []claudeMessage, strategyPrompt string) (*models.Recommendation, string, error) {
	// Start with all tools available; remove single-use tools after their first call.
	// Multi-use tools (earnings calendar, fundamentals family) stay available for per-ticker calls.
	allTools := append(marketNewsTools, fundamentalsTools...)
	remainingTools := make([]claudeTool, len(allTools))
	copy(remainingTools, allTools)

	for turn := 1; turn <= maxToolIterations; turn++ {
		log.Printf("[advisor] TURN %d →  sending to Claude  (conversation: %d message(s), tools available: %d)", turn, len(messages), len(remainingTools))

		apiResp, err := c.doAPICall(ctx, messages, remainingTools, strategyPrompt)
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
				toolResults = append(toolResults, c.executeTool(ctx, block.Name, block.ID, block.Input))
				// Multi-use tools stay available for per-ticker calls; single-use tools are removed after first call.
				if !multiUseTools[block.Name] {
					remainingTools = removeToolByName(remainingTools, block.Name)
				}
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
func (c *claudeAdvisor) doAPICall(parentCtx context.Context, messages []claudeMessage, tools []claudeTool, strategyPrompt string) (*claudeAPIResponse, error) {
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

	system := []claudeSystemBlock{}
	if strategyPrompt != "" {
		system = append(system, claudeSystemBlock{Type: "text", Text: strategyPrompt})
	}
	system = append(system, claudeSystemBlock{
		Type:         "text",
		Text:         systemPrompt,
		CacheControl: &claudeCacheControl{Type: "ephemeral"},
	})

	body := claudeAPIRequest{
		Model:     claudeModel,
		MaxTokens: 1024,
		System:    system,
		Messages:  messages,
		Tools:     tools,
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
		if resp.StatusCode == 529 {
			return nil, fmt.Errorf("%w: API 529: %s", ports.ErrAdvisorOverloaded, rawBody)
		}
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
func (c *claudeAdvisor) executeTool(ctx context.Context, name, toolUseID string, rawInput json.RawMessage) claudeToolResultBlock {
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
		limit := 15
		if v := os.Getenv("NEWS_ARTICLE_LIMIT"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				limit = n
			}
		}
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

	case "get_earnings_calendar":
		var input struct {
			Ticker string `json:"ticker"`
		}
		if err := json.Unmarshal(rawInput, &input); err != nil || strings.TrimSpace(input.Ticker) == "" {
			log.Printf("[advisor]           TOOL get_earnings_calendar ←  missing or invalid ticker")
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Error: ticker is required for get_earnings_calendar."}
		}
		ticker := strings.ToUpper(strings.TrimSpace(input.Ticker))
		log.Printf("[advisor]           TOOL get_earnings_calendar →  fetching earnings for %s", ticker)
		result := c.fetchEarnings(ctx, ticker)
		log.Printf("[advisor]           TOOL get_earnings_calendar ←  %s", result)
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: result}

	case "get_fundamentals":
		var input struct {
			Ticker string `json:"ticker"`
		}
		if err := json.Unmarshal(rawInput, &input); err != nil || strings.TrimSpace(input.Ticker) == "" {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Error: ticker is required for get_fundamentals."}
		}
		ticker := strings.ToUpper(strings.TrimSpace(input.Ticker))
		log.Printf("[advisor]           TOOL get_fundamentals →  %s", ticker)
		if c.fundamentalsProvider == nil {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Fundamentals data unavailable (FUNDAMENTALS_PROVIDER not configured)."}
		}
		fund, err := c.fundamentalsProvider.GetFundamentals(ctx, ticker)
		if err != nil {
			log.Printf("[advisor]           TOOL get_fundamentals ←  ERROR: %v", err)
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: fmt.Sprintf("Error fetching fundamentals for %s: %v", ticker, err)}
		}
		var fcfStr, peAvgStr, evAvgStr string
		if fund.FCFYieldPct > 0 {
			fcfStr = fmt.Sprintf("%.2f%%", fund.FCFYieldPct)
		} else {
			fcfStr = "n/a (negative/missing FCF)"
		}
		if fund.PEVsOwnFiveYearAvg > 0 {
			peAvgStr = fmt.Sprintf("%.2fx own 5yr avg", fund.PEVsOwnFiveYearAvg)
		} else {
			peAvgStr = "n/a (insufficient history)"
		}
		if fund.EVEBITDAVsOwnAvg > 0 {
			evAvgStr = fmt.Sprintf("%.2fx own 5yr avg", fund.EVEBITDAVsOwnAvg)
		} else {
			evAvgStr = "n/a (insufficient history)"
		}
		content := fmt.Sprintf(
			"%s: PE=%.2f ForwardPE=%.2f ForwardPEG=%.4f 52wkHigh=%.2f(%s) 52wkLow=%.2f(%s) LT-D/E=%.2f Tot-D/E=%.2f CurrentRatio=%.2f EV/EBITDA=%.2f(%s) FCFYield=%s P/B=%.2f",
			fund.Ticker, fund.PE, fund.ForwardPE, fund.ForwardPEG,
			fund.FiftyTwoWeekHigh, fund.FiftyTwoWeekHighDate,
			fund.FiftyTwoWeekLow, fund.FiftyTwoWeekLowDate,
			fund.DebtToEquity, fund.TotalDebtToEquity, fund.CurrentRatio,
			fund.EVToEBITDA, evAvgStr, fcfStr, fund.PriceToBook,
		)
		if fund.PEVsOwnFiveYearAvg > 0 {
			content += fmt.Sprintf(" PE-vs-5yrAvg=%s", peAvgStr)
		}
		log.Printf("[advisor]           TOOL get_fundamentals ←  %s", content)
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: content}

	case "get_earnings_surprises":
		var input struct {
			Ticker string `json:"ticker"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(rawInput, &input); err != nil || strings.TrimSpace(input.Ticker) == "" {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Error: ticker is required for get_earnings_surprises."}
		}
		ticker := strings.ToUpper(strings.TrimSpace(input.Ticker))
		limit := input.Limit
		if limit <= 0 || limit > 8 {
			limit = 4
		}
		log.Printf("[advisor]           TOOL get_earnings_surprises →  %s limit=%d", ticker, limit)
		if c.fundamentalsProvider == nil {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Earnings surprise data unavailable."}
		}
		surprises, err := c.fundamentalsProvider.GetEarningsSurprises(ctx, ticker, limit)
		if err != nil {
			log.Printf("[advisor]           TOOL get_earnings_surprises ←  ERROR: %v", err)
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: fmt.Sprintf("Error: %v", err)}
		}
		if len(surprises) == 0 {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: fmt.Sprintf("%s: no earnings surprise data available.", ticker)}
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%s earnings surprises (last %d quarters):\n", ticker, len(surprises))
		for _, s := range surprises {
			label := "BEAT"
			if s.SurprisePct < 0 {
				label = "MISS"
			}
			fmt.Fprintf(&sb, "- %s: actual=%.2f estimate=%.2f surprise=%+.2f%% [%s]\n",
				s.Period, s.ActualEPS, s.EstimateEPS, s.SurprisePct, label)
		}
		result := strings.TrimSpace(sb.String())
		log.Printf("[advisor]           TOOL get_earnings_surprises ←  %d quarters for %s", len(surprises), ticker)
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: result}

	case "get_insider_activity":
		var input struct {
			Ticker string `json:"ticker"`
		}
		if err := json.Unmarshal(rawInput, &input); err != nil || strings.TrimSpace(input.Ticker) == "" {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Error: ticker is required for get_insider_activity."}
		}
		ticker := strings.ToUpper(strings.TrimSpace(input.Ticker))
		log.Printf("[advisor]           TOOL get_insider_activity →  %s", ticker)
		if c.fundamentalsProvider == nil {
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: "Insider activity data unavailable."}
		}
		activity, err := c.fundamentalsProvider.GetInsiderActivity(ctx, ticker)
		if err != nil {
			log.Printf("[advisor]           TOOL get_insider_activity ←  ERROR: %v", err)
			return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: fmt.Sprintf("Error: %v", err)}
		}
		var sb strings.Builder
		lastBuy := activity.LastGenuinePurchaseDate
		if lastBuy == "" {
			lastBuy = "none in lookback window"
		}
		fmt.Fprintf(&sb, "%s insider activity: ConsecutiveNegativeMonths=%d LastGenuinePurchase=%s\n",
			ticker, activity.ConsecutiveNegativeMonths, lastBuy)
		shown := activity.RecentMonths
		if len(shown) > 6 {
			shown = shown[:6]
		}
		if len(shown) > 0 {
			sb.WriteString("Recent months (most recent first; SELLING/BUYING reflects MSPR sign, not purchase intent):\n")
			for _, m := range shown {
				direction := "BUYING"
				if m.MSPR < 0 {
					direction = "SELLING"
				}
				fmt.Fprintf(&sb, "- %d-%02d: MSPR=%.1f change=%d [%s]\n", m.Year, m.Month, m.MSPR, m.Change, direction)
			}
		}
		result := strings.TrimSpace(sb.String())
		log.Printf("[advisor]           TOOL get_insider_activity ←  %s ConsecutiveNegativeMonths=%d lastBuy=%q",
			ticker, activity.ConsecutiveNegativeMonths, activity.LastGenuinePurchaseDate)
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: result}

	default:
		log.Printf("[advisor]           TOOL %s →  unknown tool — returning error to Claude", name)
		return claudeToolResultBlock{Type: "tool_result", ToolUseID: toolUseID, Content: fmt.Sprintf("unknown tool: %s", name)}
	}
}

// fetchEarnings looks up upcoming earnings for a single ticker via Finnhub,
// caching results for earningsCacheTTL (1 hour) per ticker.
func (c *claudeAdvisor) fetchEarnings(ctx context.Context, ticker string) string {
	apiKey := os.Getenv("FINNHUB_API_KEY")
	if apiKey == "" {
		return "Earnings calendar unavailable (FINNHUB_API_KEY not configured)."
	}

	c.earningsMu.RLock()
	if entry, ok := c.earningsCache[ticker]; ok && time.Now().Before(entry.exp) {
		c.earningsMu.RUnlock()
		log.Printf("[advisor]             earnings cache hit for %s", ticker)
		return entry.result
	}
	c.earningsMu.RUnlock()

	today := time.Now().Format("2006-01-02")
	toDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	url := fmt.Sprintf("%s?symbol=%s&from=%s&to=%s&token=%s", finnhubEarningsBaseURL, ticker, today, toDate, apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Sprintf("Earnings lookup failed for %s.", ticker)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("Earnings lookup failed for %s.", ticker)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("Earnings lookup failed for %s.", ticker)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("Earnings lookup failed for %s (HTTP %d).", ticker, resp.StatusCode)
	}

	var parsed struct {
		EarningsCalendar []struct {
			Date   string `json:"date"`
			Hour   string `json:"hour"`
			Symbol string `json:"symbol"`
		} `json:"earningsCalendar"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Sprintf("Earnings data parse failed for %s.", ticker)
	}

	var result string
	if len(parsed.EarningsCalendar) == 0 {
		result = fmt.Sprintf("No earnings found for %s in the next 7 days.", ticker)
	} else {
		e := parsed.EarningsCalendar[0]
		timing := "during market hours"
		switch e.Hour {
		case "amc":
			timing = "after market close"
		case "bmo":
			timing = "before market open"
		}
		daysLabel := ""
		if earningsDate, err := time.Parse("2006-01-02", e.Date); err == nil {
			days := int(time.Until(earningsDate).Hours()/24) + 1
			switch {
			case days <= 0:
				daysLabel = " (today)"
			case days == 1:
				daysLabel = " (tomorrow)"
			default:
				daysLabel = fmt.Sprintf(" (%d days away)", days)
			}
		}
		result = fmt.Sprintf("%s earnings: %s%s (%s).", ticker, e.Date, daysLabel, timing)
	}

	c.earningsMu.Lock()
	c.earningsCache[ticker] = earningsCacheEntry{result: result, exp: time.Now().Add(earningsCacheTTL)}
	c.earningsMu.Unlock()

	return result
}

// extractJSON strips markdown code fences and trims to the outermost { ... }.
func currentTimeEST() string {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.Now().UTC().Format("3:04 PM UTC")
	}
	return time.Now().In(loc).Format("3:04 PM MST")
}

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

// classifyAndStore fires a goroutine for each allocation whose ticker is not in the
// approved cache. Claude's suggested asset_class is stored as approved:true immediately
// and the in-memory cache is updated so subsequent recommendations see it without restart.
func (c *claudeAdvisor) classifyAndStore(allocations []models.Allocation) {
	if c.classifier == nil || c.classRepo == nil {
		return
	}
	for _, alloc := range allocations {
		_, known := c.classifier.Classify(alloc.Ticker)
		if known {
			continue
		}
		ticker, assetClass := alloc.Ticker, alloc.AssetClass
		go func() {
			if err := c.classRepo.StoreClassification(context.Background(), ticker, assetClass); err != nil {
				log.Printf("[classification] ERROR storing %s: %v", ticker, err)
				return
			}
			c.classifier.Store(ticker, assetClass)
			log.Printf("[classification] NEW: %s → %s (Claude-classified)", ticker, assetClass)
		}()
	}
}

// buildConcentrationBlock groups positions by asset class and formats the
// CURRENT PORTFOLIO CONCENTRATION section. Returns "" when classifier is nil.
func buildConcentrationBlock(positions []models.Position, classifier ports.Classifier, totalValue float64) string {
	if classifier == nil || len(positions) == 0 || totalValue == 0 {
		return ""
	}

	type tickerEntry struct {
		ticker string
		value  float64
	}
	type classGroup struct {
		name    string
		value   float64
		tickers []tickerEntry
	}

	groupMap := map[string]*classGroup{}
	var classOrder []string

	for _, p := range positions {
		ac, _ := classifier.Classify(p.Ticker)
		if _, exists := groupMap[ac]; !exists {
			groupMap[ac] = &classGroup{name: ac}
			classOrder = append(classOrder, ac)
		}
		groupMap[ac].value += p.MarketValue
		groupMap[ac].tickers = append(groupMap[ac].tickers, tickerEntry{p.Ticker, p.MarketValue})
	}

	groups := make([]*classGroup, 0, len(groupMap))
	for _, ac := range classOrder {
		groups = append(groups, groupMap[ac])
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].value > groups[j].value
	})

	var sb strings.Builder
	sb.WriteString("\nCURRENT PORTFOLIO CONCENTRATION:\n")
	for _, g := range groups {
		classPct := g.value / totalValue * 100
		sort.Slice(g.tickers, func(i, j int) bool {
			return g.tickers[i].value > g.tickers[j].value
		})
		parts := make([]string, len(g.tickers))
		for i, t := range g.tickers {
			parts[i] = fmt.Sprintf("%s %.0f%%", t.ticker, t.value/totalValue*100)
		}
		sb.WriteString(fmt.Sprintf("- %s: %.0f%% ($%.0f) — %s\n", g.name, classPct, g.value, strings.Join(parts, ", ")))
	}
	return sb.String()
}

func buildUserMessage(req models.InvestmentRequest, profile *models.UserProfile, snapshot *models.MarketSnapshot, classifier ports.Classifier) string {
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

		if prefs := profile.AllocationPreferences; prefs != nil && (len(prefs.AssetClassLimits) > 0 || prefs.MaxSingleTickerPct > 0) {
			msg += "\nPORTFOLIO CONSTRAINTS (hard rules — never violate these):\n"
			for _, l := range prefs.AssetClassLimits {
				switch {
				case l.MinPct > 0 && l.MaxPct > 0:
					msg += fmt.Sprintf("- %s: min %.0f%%, max %.0f%%\n", l.AssetClass, l.MinPct, l.MaxPct)
				case l.MinPct > 0:
					msg += fmt.Sprintf("- %s: minimum %.0f%%\n", l.AssetClass, l.MinPct)
				case l.MaxPct > 0:
					msg += fmt.Sprintf("- %s: maximum %.0f%%\n", l.AssetClass, l.MaxPct)
				}
			}
			if prefs.MaxSingleTickerPct > 0 {
				msg += fmt.Sprintf("- Single ticker cap: maximum %.0f%% in any one ticker\n", prefs.MaxSingleTickerPct)
			}
		}
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

		if block := buildConcentrationBlock(req.Positions, classifier, totalValue); block != "" {
			msg += block
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
		hasInvestDecision := false
		for _, d := range req.RecentDecisions[:limit] {
			switch d.DecisionType {
			case "blocked":
				reason := d.BlockedReason
				if reason == "" {
					reason = "no reason recorded"
				}
				msg += fmt.Sprintf("- %s: [BLOCKED] %s\n", d.Timestamp.Format("Jan 2"), reason)
			case "skip":
				reason := d.SkipReason
				if reason == "" {
					reason = "no specific reason"
				}
				msg += fmt.Sprintf("- %s: [SKIPPED] %s\n", d.Timestamp.Format("Jan 2"), reason)
			default: // "invest" or legacy empty string
				tickers := make([]string, len(d.Allocations))
				for i, a := range d.Allocations {
					tickers[i] = fmt.Sprintf("%s %.0f%%", a.Ticker, a.Percentage)
				}
				msg += fmt.Sprintf("- %s: $%.0f — %s\n", d.Timestamp.Format("Jan 2"), d.TotalAmount, strings.Join(tickers, ", "))
				hasInvestDecision = true
			}
		}
		if hasInvestDecision {
			msg += "Vary today's allocation — do not repeat the exact same split as yesterday.\n"
		}
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

	// PORTFOLIO INTELLIGENCE — injected only when a recent rebalance analysis exists.
	// Blocked tickers are a hard constraint; underweight tickers are a soft hint.
	if ra := req.RebalanceAnalysis; ra != nil {
		var blocked, underweight []string
		for _, ins := range ra.Insights {
			switch ins.SuggestedAction {
			case models.ActionTrim, models.ActionReconsider:
				blocked = append(blocked, ins.Ticker)
			case models.ActionAdd:
				underweight = append(underweight, ins.Ticker)
			}
		}
		if len(blocked) > 0 || len(underweight) > 0 {
			msg += fmt.Sprintf("\nPORTFOLIO INTELLIGENCE (from latest rebalance analysis, generated %s):\n", ra.GeneratedAt.Format("Jan 2, 2006"))
			if len(blocked) > 0 {
				sort.Strings(blocked)
				msg += fmt.Sprintf("Hard constraint — do not allocate to these tickers today: %s\n", strings.Join(blocked, ", "))
			}
			if len(underweight) > 0 {
				sort.Strings(underweight)
				msg += fmt.Sprintf("Consider adding to these underweighted positions if thesis is strong: %s\n", strings.Join(underweight, ", "))
			}
		}
	}

	// POSITION HISTORY — injected only when per-ticker purchase history is available.
	// Helps Claude understand cost basis and holding period context.
	if len(req.PositionContext) > 0 {
		tickers := make([]string, 0, len(req.PositionContext))
		for t := range req.PositionContext {
			tickers = append(tickers, t)
		}
		sort.Strings(tickers)
		msg += "\nPOSITION HISTORY (your prior purchases):\n"
		for _, ticker := range tickers {
			tc := req.PositionContext[ticker]
			taxLabel := "short-term"
			if tc.MonthsHeld >= 12 {
				taxLabel = "long-term"
			}
			msg += fmt.Sprintf("- %s: avg cost $%.2f, %d purchase(s), held %d month(s) (%s), $%.2f total invested\n",
				ticker, tc.AverageCostBasis, tc.PurchaseCount, tc.MonthsHeld, taxLabel, tc.TotalInvested)
		}
	}

	// Performance feedback — injected only when >= 5 verdicts exist (set by recommendation service).
	// Gives Claude a signal on whether its past picks have beaten the market for this user.
	if ps := req.PerformanceSummary; ps != nil {
		wins := int(ps.WinRate * float64(ps.VerdictedDecisions))
		msg += fmt.Sprintf("\nPAST PERFORMANCE (%d evaluated decisions):\n", ps.VerdictedDecisions)
		msg += fmt.Sprintf("- Win rate vs SPY: %d%% (%d/%d beat the market)\n", int(ps.WinRate*100), wins, ps.VerdictedDecisions)
		msg += fmt.Sprintf("- Avg return: %+.1f%% vs SPY avg %+.1f%%\n", ps.AvgReturnPct, ps.AvgSPYReturnPct)
		if ps.BestDecision != nil && ps.WorstDecision != nil {
			msg += fmt.Sprintf("- Best decision: %+.1f%% | Worst: %+.1f%%\n", ps.BestDecision.ReturnPct, ps.WorstDecision.ReturnPct)
		}
		msg += "Use as context only — do not override the user's stated risk tolerance or investment amount.\n"
	}

	if len(req.RecentBlockedDecisions) > 0 {
		msg += "\nRECENT BLOCKS (recommendations rejected by risk review):\n"
		for _, d := range req.RecentBlockedDecisions {
			msg += fmt.Sprintf("- %s: %s\n", d.Timestamp.Format("Jan 2"), d.BlockedReason)
		}
		msg += "Do not repeat the same risk_level or asset-class concentration that triggered these blocks.\n"
	}

	if req.AgenticMode {
		msg += "\nAGENTIC BUDGET CONTEXT\n"
		msg += fmt.Sprintf("Daily budget: $%.2f\n", req.DailyBudget)
		msg += fmt.Sprintf("Already invested today: $%.2f\n", req.SpentToday)
		msg += fmt.Sprintf("Remaining today: $%.2f\n", req.Remaining)
		msg += fmt.Sprintf("Current time: %s\n\n", currentTimeEST())
		msg += fmt.Sprintf("You must decide how much of the $%.2f remaining to invest RIGHT NOW based on current market conditions.\n", req.Remaining)
		msg += fmt.Sprintf("Set total_budget to a value between 0 and %.2f.\n", req.Remaining)
		msg += "If total_budget is 0 (skip), include a \"skip_reason\" field in your JSON with one sentence explaining why.\n"
		msg += fmt.Sprintf("Minimum $1 per ticker if investing. Never exceed $%.2f.\n", req.Remaining)
	}

	msg += "\nGive me today's investment allocation."
	return msg
}
