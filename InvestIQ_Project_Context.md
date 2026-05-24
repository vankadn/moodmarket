# InvestIQ — Project Context & Master Reference

> Load this into your Claude Project so every new conversation starts with full context.
> Last updated: 2026-05-24 — Claude reasoning stored on decisions (OverallReasoning + TickerReasoning); "Why Claude invested" section in email; Allocation.Reasoning + Recommendation.OverallReasoning fields

---

## Who I am

- Background: React + Go developer (professional experience)
- Dev environment: MacBook Pro (Intel), VS Code + Claude Code extension, terminal

---

## What InvestIQ is

A personal financial operating system. Not a mood-based gimmick — a real tool that knows the user's complete financial life and makes intelligent daily investment decisions based on actual data.

**The core loop:**
1. User sets up full financial profile once — never asked again
2. Every day: open app, optionally adjust the dollar amount, tap one button
3. App reads user profile from MongoDB + live market context
4. Claude (or any AI model) generates a structured investment allocation
5. Trade executes via brokerage API
6. Decision logged to MongoDB

**What makes it different from a mood app:**
- The AI reads real financial data — salary, portfolio, goals, risk tolerance, immigration status
- User never enters data twice — it is stored and auto-loaded
- The AI model is a swappable reasoning tool, not the brain of the app
- The app owns the intelligence, the data, and the decisions

---

## Architecture principles — non-negotiable

### Onion Architecture
```
domain/              ← innermost: entities, value objects, interfaces
  models/
  ports/             ← interfaces the domain defines

application/         ← use cases, orchestration
  services/
  scheduler/

infrastructure/      ← implementations of domain ports
  advisor/           ← AI provider implementations (Claude + mock)
  classification/    ← in-memory ticker→asset-class cache
  db/                ← MongoDB
  auth/              ← Auth providers (DEV + Clerk)
  market/            ← stock price APIs (Polygon + mock)
  news/              ← market headlines (Polygon + mock)
  banking/           ← Plaid
  brokerage/         ← Alpaca
  notifications/     ← log provider (dev), Resend email (prod)
  secrets/           ← Vault / secrets manager (pre go-live)

api/                 ← outermost: HTTP handlers, middleware
  handlers/
  middleware/
```

Dependency rule: outer layers depend on inner. Never the reverse. Domain layer never imports infrastructure.

### Provider Abstraction Pattern
Every external dependency is hidden behind an interface defined in `domain/ports/`. Swapping any provider requires one new file + one config change. Nothing else changes.

Key interfaces:
- `InvestmentAdvisor` — AI recommendation engine (Claude or mock)
- `ProfileRepository` — user profile persistence (MongoDB)
- `AutoInvestRepository` — auto-invest config persistence (MongoDB)
- `AuthProvider` — identity (DevAuth for local dev, Clerk in production)
- `IdentityProvider` — userId in request context
- `MarketDataProvider` — live prices (Polygon or mock)
- `NewsProvider` — market headlines (Polygon or mock)
- `DecisionRepository` — investment decision persistence
- `BrokerageProvider` — trade execution + position reads + portfolio history (Alpaca or mock)
- `BrokerageProviderFactory` — constructs a `BrokerageProvider` from a per-user `BrokerageConnection`; decouples application layer from credential decryption and Alpaca constructor details
- `FinancialDataProvider` — bank + 401k data (Plaid or mock)
- `NotificationProvider` — post-invest notifications; `SendInvestmentSummary(…, overallReasoning string)`, `SendInvestmentFailure`, `SendMarketClosed`, `SendSkipSummary`; log provider (dev), Resend email provider (prod)
- `SecretsProvider` — sensitive credential retrieval (pre go-live, Vault / AWS Secrets Manager)
- `Classifier` — in-memory ticker→asset-class lookup; `ClassificationCache` implements this; never hits Mongo during a recommendation
- `ClassificationRepository` — ticker classification persistence: `LoadAll`, `StoreClassification`
- `PortfolioAggregator` — read-only external holdings: `GetHoldings(ctx, providerUserID, providerUserSecret) []Position`; SnapTrade implements this
- `PortfolioConnector` — OAuth lifecycle for external broker linking: `RegisterUser`, `GenerateConnectURL`, `DeleteUser`; SnapTrade implements this

### DEV_MODE pattern
`DEV_MODE=true` in `.env` auto-logs in as the hardcoded dev user. Zero login required during development. This logic lives in exactly ONE place — `infrastructure/auth/factory.go`. No DEV_MODE checks anywhere else.

### MOCK_ALL pattern
`MOCK_ALL=true` overrides all provider env vars to `mock` and sets `DEV_MODE=true` — zero external calls needed. Processed at startup in `main.go` before any factory runs. Factories are unaware of it.

---

## Brokerage strategy

Three providers, three distinct roles — never conflated:

| Provider | Role | Auth | Status |
|----------|------|------|--------|
| **Alpaca** | Trade execution — stocks, options, crypto subset | Per-user API key + secret (AES-256-GCM encrypted in Mongo) | Active (paper → live is a config change) |
| **SnapTrade** | Portfolio aggregation — read positions from Robinhood, Fidelity, and other linked brokers | OAuth; per-user `providerUserID` + `providerUserSecret` (AES-256-GCM encrypted in Mongo) | Active — read-only; execution only after SnapTrade confirms Robinhood write access |
| **Coinbase Advanced Trade** | Crypto execution | API key auth (per-user, encrypted at rest) | Planned — see What's next |

**SnapTrade trade execution via Robinhood:** Do not build this path until SnapTrade's Robinhood integration explicitly confirms write/order access. Read-only aggregation first — execution only after verified.

**Banned providers:**
- `robin_stocks` or any library that reverse-engineers Robinhood's private API
- Any unofficial Fidelity client
- Any scraping-based or undocumented endpoint from any broker

Every brokerage provider implements `BrokerageProvider` in `domain/ports/`. No broker SDK is imported into domain, application, or handler layers.

---

## Tech stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Backend | Go, standard library only | No Gin, Echo, or any HTTP framework |
| Frontend | React + TypeScript + Vite | No UI component libraries |
| Database | MongoDB | Behind repository interfaces |
| AI Provider | Claude / mock | Behind InvestmentAdvisor interface; mock returns fixed three-fund portfolio |
| Auth | Clerk (email + Google SSO) | Behind AuthProvider interface; DEV_MODE=true bypasses for local dev |
| Market Data | Polygon.io (previous-day, free tier) / mock | Behind MarketDataProvider interface |
| Brokerage | Alpaca paper trading / mock | Behind BrokerageProvider interface; paper → live is a config change only |
| Banking | Plaid (production, 10 free connections) / mock | Behind FinancialDataProvider interface; 5-min balance cache via PLAID_CACHE_TTL |
| Secrets | Encrypted Mongo now, Vault pre go-live | Behind SecretsProvider interface |
| Scheduler | Go time.Ticker | Drives autonomous investment cycle; interval from AUTO_INVEST_INTERVAL env var |
| Notifications | Log provider (dev) / Resend (prod) | Behind NotificationProvider interface; `NOTIFICATION_PROVIDER=resend` + `RESEND_API_KEY` + `RESEND_FROM` activates email; SMS via Twilio backlogged |
| News | Polygon.io `/v2/reference/news` / mock | Behind NewsProvider interface; daily cache; top 5 SPY-tagged headlines injected into Claude prompt |

---

## Running with no external accounts

Set `MOCK_ALL=true` in `.env`. Only MongoDB is required. No Anthropic, Plaid, Alpaca, Polygon, or Clerk keys needed.

To use real Claude but mock everything else: set `AI_PROVIDER=claude` + `ANTHROPIC_API_KEY`.

See README.md for the full env var reference and provider swap table.

---

## Domain language — use these terms consistently

| Term | Meaning |
|------|---------|
| `UserProfile` | Complete financial picture of the user |
| `InvestmentRequest` | Request to generate a daily allocation |
| `Allocation` | Single position recommendation (ticker, amount, %, `Reasoning` — one-sentence per-ticker Claude explanation; `omitempty`) |
| `Recommendation` | Full AI response: allocations + summary + risk level + `OverallReasoning` (1-2 sentence thesis) + `FromCache bool` (true when returned from MongoDB fallback due to advisor overload) |
| `InvestmentDecision` | Persisted record: userId, timestamp, market snapshot, allocations, trade receipts, DecisionType ("invest"/"skip"), SkipReason, OverallReasoning, TickerReasoning (map[ticker]reason) |
| `MarketSnapshot` | Point-in-time market context: SPY/QQQ trend, sector ETF performance, sentiment |
| `RiskTolerance` | conservative / moderate / aggressive |
| `TimeHorizon` | under_1_year / one_to_five / five_to_ten / ten_plus |
| `InvestmentGoal` | wealth_building / retirement / emergency_fund / short_term_savings |
| `UserIdentity` | Authenticated user: UserID, Email, Name |
| `TradeOrder` | Value object: UserID, Ticker, Amount (dollar-based notional) |
| `TradeReceipt` | Value object: OrderID, Ticker, FilledAmount, FilledPrice, Status, Timestamp, BrokerageID (optional), BrokerageName (optional) |
| `Position` | Brokerage holding: Ticker, Quantity, MarketValue |
| `BankAccount` | Plaid-sourced account: institution, type, balance |
| `BalanceSummary` | Aggregated view: total cash, total investments, net worth signal |
| `PlaidConnection` | Per-institution token record: institution name, access_token, item_id |
| `BrokerageConnection` | Internal per-user Alpaca credential record: ID, Name, AssetCategories, APIKey, SecretKey, BaseURL, Connected, ConnectedAt — encrypted at rest, never exposed to frontend |
| `BrokerageStatus` | JSON-safe API subset: ID, Name, AssetCategories, connected, base_url, connected_at — returned in `UserProfile.Brokerages` array |
| `AssetCategory` | `equity` / `bond` / `default` — controls which connection an allocation is routed to |
| `AutoInvestConfig` | First-class domain model (own collection): Enabled, Amount, DailyBudget, Risk, IntervalHours, EnabledAt, UpdatedAt |
| `SchedulerRun` | Audit record for one autonomous cycle: RunID, StartedAt, CompletedAt, UsersProcessed, TotalInvested, Errors |
| `NotificationTarget` | Delivery target for one notification: UserID, Email, Phone, Source (`"manual"` or `"auto"`) |
| `HistoryPoint` | One data point in a portfolio value time series: Timestamp (Unix epoch seconds), Equity, ProfitLoss, ProfitLossPct |
| `NewsItem` | One market headline: Headline, Summary, Source, PublishedAt |
| `PortfolioConnection` | Per-user SnapTrade credential record: Provider, ProviderUserID, ProviderUserSecret (both AES-256-GCM encrypted), ConnectedAt — never serialized to JSON; never logged |
| `PortfolioConnectionStatus` | Safe API subset returned to callers: Provider, Connected, ConnectedAt — secrets never included |
| `TransactionSummary` | Aggregated spending signals: SpendLast7Days, SpendLast30Days, LargestPendingAmount, LargestPendingName, PulledAt |
| `DecisionVerdict` | Performance result stamped on a decision: StampedAt, OverallReturnPct, SPYReturnPct, BeatMarket, TickerVerdicts |
| `TickerVerdict` | Per-ticker performance: Ticker, EntryPrice, PrevDayPrice, PrevDayTimestamp, CurrentPrice, CurrentTimestamp, ReturnPct, TodayChangePct |
| `EvalSummary` | Aggregated verdict stats: TotalDecisions, VerdictedDecisions, WinRate, AvgReturnPct, AvgSPYReturnPct, BestDecision, WorstDecision, ByStrategy |
| `ClassificationEntry` | One ticker record: ticker, asset_class, approved, suggested_by_claude, first_seen_at |
| `Allocation.AssetClass` | Asset class on each allocation — returned by Claude in JSON, verified against cache; `omitempty` so old decisions are unaffected |

---

## User profile fields (collected once at onboarding)

- `full_name` string
- `salary` number (annual)
- `monthly_savings` number
- `retirement_contribution_percent` number (0–100)
- `existing_portfolio_value` number
- `time_horizon` enum
- `risk_tolerance` enum
- `investment_goal` enum
- `has_emergency_fund` boolean
- `include_cash_context` boolean — opt-in: share spending/runway data with Claude advisor (default false; no migration needed)
- `notification_email` string (optional) — email address for post-invest notifications; empty = no email sent
- `phone` string (optional) — E.164 phone number for SMS notifications; stored but SMS not yet implemented (Twilio backlogged)
- `brokerage_connection` embedded doc (in `users` collection) — encrypted APIKey, SecretKey, BaseURL, ConnectedAt; never serialized to JSON; exposed only as `BrokerageStatus` in profile response
- `portfolio_connection` embedded doc (in `users` collection) — encrypted ProviderUserID, ProviderUserSecret, Provider, ConnectedAt; never serialized to JSON; exposed only as `PortfolioConnectionStatus` in profile response

Auto-invest config (amount, risk, enabled) lives in `AutoInvestConfig` — its own collection, not on UserProfile.

---

## MongoDB collections

| Collection | Purpose |
|-----------|---------|
| `users` | User profile + financial goals + Plaid connections array |
| `auto_invest_configs` | One document per user: enabled, amount, risk, EnabledAt, UpdatedAt |
| `decisions` | Every daily investment decision: userId, timestamp, market snapshot, allocations, receipts, Plaid snapshot |
| `scheduler_runs` | One document per autonomous cycle: runID, timestamp, users processed, total invested, errors |
| `ticker_classifications` | Ticker→asset-class map; all entries loaded into memory at startup via ClassificationCache regardless of approved field |
| `tax_documents` | Extracted tax form fields (W2/1099/1098); form-specific upsert keys |

---

## What's built

### Core investment loop
- `/recommend` — Claude generates a structured allocation (ticker, amount, %, asset_class) using: user profile, live market snapshot, current positions, recent decision history, market news (via tool use), tax documents, spending context (opt-in), and portfolio concentration by asset class
- `/invest` — places Alpaca market orders (notional dollar-based), routes by asset category across multiple brokerage accounts, logs full decision + receipts to MongoDB
- Advisor overload fallback: on HTTP 529, returns last decision rescaled to today's budget (`FromCache: true`) — no hard error
- Claude prompt retries: 3 attempts, exponential backoff; parse errors get a correction turn, API errors get a clean retry

### Auth & dev tooling
- Clerk (email + Google SSO) in production; `DEV_MODE=true` bypass lives only in `infrastructure/auth/factory.go`
- `MOCK_ALL=true` overrides all providers to mock + sets `DEV_MODE=true` — MongoDB is the only external dependency in dev
- `cmd/dbcheck` — Go CLI to inspect any Mongo collection without mongosh; `go run ./cmd/dbcheck [collection]`
- OpenAPI 3.0.3 spec + Swagger UI served at `/docs/` (embedded at compile time)
- Postman collection with all endpoints + environment files for local and production

### Brokerage & portfolio
- Alpaca paper trading; per-user AES-256-GCM encrypted credentials; paper → live is a config change
- Multi-brokerage: connect multiple accounts by name + asset category; allocations routed by category with default fallback
- `TradeReceipt` stamps `brokerage_id` + `brokerage_name` for full audit trail
- Per-allocation brokerage override on `/invest` (frontend confirm screen)
- Portfolio P&L: positions with cost basis, unrealized P&L, per-account + combined totals
- Portfolio history chart: 1D/5D/1M/1Y/5Y period selector, SVG polyline (no charting library)
- `GET /portfolio`, `GET /portfolio/history`, `POST/DELETE /brokerage/connections`
- `ErrBrokerageNotConnected` sentinel: fatal for invest (400), non-fatal for recommend (skip positions), clean skip in scheduler

### External portfolio aggregation (SnapTrade)
- `POST /portfolio/connect` — registers user with SnapTrade, returns broker OAuth redirect URL; idempotent (re-uses existing registration for a fresh URL)
- `DELETE /portfolio/connect` — de-registers user from SnapTrade; best-effort (local clear proceeds even if provider call fails)
- Credentials (`providerUserID`, `providerUserSecret`) stored AES-256-GCM encrypted in `users.portfolio_connection`; never logged
- `PortfolioConnector` interface: `RegisterUser`, `GenerateConnectURL`, `DeleteUser` — lifecycle for linking external brokers
- `PortfolioAggregator` interface: `GetHoldings` — fetches all positions from linked external brokers (two-step: list accounts → positions per account)
- HMAC-SHA256 signed SnapTrade API client in `infrastructure/portfolio/snaptrade.go`; per-call credentials, no state stored on client
- `RecommendationService` step 5b: external holdings fetched and merged into `InvestmentRequest.Positions`; Alpaca positions take precedence (first-seen-wins deduplication by ticker)
- `UserProfile.PortfolioAggregator` field: `PortfolioConnectionStatus` (provider + connected + connected_at) returned in profile response
- Frontend: `PortfolioAggregatorConnect` component; "Ext. accounts" nav button; `portfolio-connect` app state; `connectPortfolioAggregator` / `disconnectPortfolioAggregator` API functions

### Banking & spending
- Plaid (production, 10 free connections) — balance + transaction history; `PLAID_CACHE_TTL` (default 5m) reduces API calls
- Spending context (opt-in via `UserProfile.IncludeCashContext`): 7d + 30d spend, cash runway, largest pending charge — injected as background context only, never overrides risk tolerance
- `GET /users/cash-context`, `POST /plaid/link-token`, `POST /plaid/exchange`, `DELETE /plaid/accounts/{item_id}`
- Token revocation calls Plaid `/item/remove` before MongoDB delete

### Autonomous investing
- Scheduler: per-user `isDue()` check on hourly tick; priority: `IntervalSeconds` > `IntervalHours` > `IntervalDays` (0 = daily)
- Agentic daily budget: when `DailyBudget > 0`, scheduler sums today's spend via `SumTodayByConfig`, injects remaining into Claude prompt; Claude returns `total_budget: 0` to skip or an amount ≤ remaining; budget-exhausted and Claude-skip paths both save a `DecisionType: "skip"` decision and notify via `SendSkipSummary`
- Multi-config strategies per user: named, with `long_term` or `short_term` strategy prompt injected before base system prompt; CRUD at `/users/auto-invest/configs`
- Market holiday awareness: algorithmic NYSE calendar (no external API); `MARKET_CALENDAR=nyse|mock`
- Per-strategy activity: decision count, total invested, last run — `GET /users/activity/by-strategy`
- Per-strategy P&L: proportional attribution of live Alpaca unrealized P&L by ticker cost basis — `GET /users/activity/by-strategy/pnl`
- All auto-invest decisions tagged with `config_id`; manual invest tagged `"manual"`

### Intelligence & context
- Claude tool use: fetches market news itself via `get_market_news` (Polygon) — app never pre-fetches or injects headlines
- Tax document intelligence: PDF upload → Claude extracts structured fields (W2/1099/1098) → injected into every recommendation; `POST /documents/upload`, `GET /documents`, `DELETE /documents/:id`
- Portfolio concentration block: positions grouped by asset class, sorted by %, injected into Claude prompt; CONCENTRATION RULE + TICKER RULE in system prompt
- Ticker classification: 29 base tickers seeded in `ticker_classifications`; unknown tickers classified by Claude at recommendation time and stored immediately via `StoreClassification`; in-memory `ClassificationCache` means zero Mongo reads per request
- Decision verdicts: stamped inline (goroutine per recommendation); `OverallReturnPct`, `SPYReturnPct`, `BeatMarket`, per-ticker verdicts
- Feedback loop: when `VerdictedDecisions ≥ 5`, Claude receives its own win rate + avg return vs SPY in every prompt (`PAST PERFORMANCE` section)
- Claude reasoning stored on every invest decision: `OverallReasoning` (1-2 sentence thesis) and `TickerReasoning` map (per-ticker explanation); both `omitempty` — old decisions unaffected; system prompt requires `overall_reasoning` + `allocations[].reasoning` in Claude's JSON response
- Activity dashboard: total invested, verdict stats, win rate vs SPY, best/worst decision, per-strategy breakdown, full decision history with verdicts overlaid — `GET /users/activity`, `/eval/summary`, `/eval/decisions`

### Infrastructure & deployment
- www.investiq.fit (Vercel), api.investiq.fit (Railway); TLS via Let's Encrypt; DNS on Namecheap
- Docker: two-stage distroless image (`golang:1.23-alpine` builder → `gcr.io/distroless/static-debian12` runner); static binary required
- Email notifications via Resend (`NOTIFICATION_PROVIDER=resend`); source-aware copy (manual vs auto-invest); "Why Claude invested" section included when `OverallReasoning` is non-empty; `GET/PATCH /users/notifications`
- `GET /health` on top-level mux (no auth) for Railway/Docker healthchecks

---

## What's next

### P1 — Pre go-live / blockers

| Item | Why |
|------|-----|
| LLC formation | Required before signing Alpaca Broker API or SnapTrade commercial agreements; one LLC covers InvestIQ + other ventures |
| Vault / AWS Secrets Manager for Plaid tokens | Plaid `access_token` values are live credentials — DB compromise must not expose them. Hook is ready: `infrastructure/secrets/vault.go` behind `SecretsProvider` interface |
| Atlas IP allowlist | Replace 0.0.0.0/0 with Railway static IP; requires Railway Pro plan |
| Paper → live Alpaca trading | Per-user credentials already wired — UI adds account type selector (Paper/Live); user supplies live keys; no backend code change; requires LLC |
| Remove debug log lines | `[8a-debug]`, `[8b-debug]`, `[8c-debug]`, `[8ca-debug]` in `recommendation_service.go` — grep each tag; marked with `// TODO: remove` comments |

### P2 — Near-term features

| Item | Notes |
|------|-------|
| Coinbase Advanced Trade | Crypto execution via official API key auth. Implements `BrokerageProvider` — zero application layer changes. Same per-user AES-256-GCM credential pattern as Alpaca |
| SMS notifications via Twilio | `Phone` field already on `UserProfile`. Needs `infrastructure/notifications/twilio.go` implementing `NotificationProvider`; factory wired on `NOTIFICATION_PROVIDER=twilio` with `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM` |

### P3 — Longer-term / conditional

| Item | Notes |
|------|-------|
| SnapTrade trade execution via Robinhood | Conditional on SnapTrade confirming Robinhood write/order access. Route equity allocations to Robinhood when connected; fallback to Alpaca |
| Real-time market data | `/prev` endpoint gives yesterday's prices. Real-time (Finnhub) deferred until stale data meaningfully hurts recommendation quality |
| Verdict entry price accuracy | When `FilledPrice = 0` (async Alpaca), Polygon prev-day close is used as proxy. True fix: re-fetch Alpaca order after a delay and re-stamp with real fill price |
| Redis for Plaid balance cache | In-memory cache resets on restart. Redis for persistence at scale |
| Rebalancing alerts | Notify when portfolio drifts past target allocation |
| Tax optimization | Tax-loss harvesting, asset location strategy |
| Household / family accounts | Multi-user households sharing one financial picture |
| Earnings calendar, macro indicators | News context covers macros for now; earnings adds complexity for marginal value at current scale |

---

## Business Notes

- InvestIQ is free access for friends/users — not paid product initially
- LLC needed before signing Alpaca Broker API or SnapTrade commercial agreements
- Claude making recommendations vs executing trades = compliance distinction
- One LLC can cover InvestIQ + other business ventures
- See PERSONAL.md (not committed) for LLC formation and account migration details

---

## The long-term vision

A unified financial operating system. The app knows:
- Bank accounts (checking, savings, cash position) via Plaid
- 401k and retirement accounts (Fidelity, Vanguard, etc.)
- Brokerage accounts (what you already own)
- Daily spending patterns

When the user taps "Invest today", Claude receives a prompt like:
> "User has $4,200 in checking, $180,000 in 401k (60/40 allocation), $12,000 in brokerage mostly QQQ and AAPL, a $1,200 credit card bill due in 8 days, 10-year horizon, moderate risk. Invest $100 today."

Eventually: fully autonomous. App runs at 9am, invests, sends a notification. User never opens it unless they want to review.

---

## Security & Compliance Decisions

### Plaid access token model
Plaid `access_token` values are permanent credentials — treat like passwords. A stolen token grants read access to all account balances, transaction history, and investment holdings.

**Current:** field-level AES-256-GCM encryption on `access_token` in MongoDB.

**Before any real user beyond developer:** move tokens to HashiCorp Vault or AWS Secrets Manager. Store only a vault reference key in Mongo. Infrastructure hook is ready: `infrastructure/secrets/vault.go` behind `SecretsProvider` interface.

### Regulatory obligations
- **GLBA** — protect financial data, disclose data sharing practices
- **CCPA** — California users have access, deletion, and opt-out rights
- **Plaid developer agreement** — legally binding on data handling; Plaid can terminate API access for violations
- **FTC enforcement** — primary body for fintech in the US

### Principle of least privilege
Only request the Plaid scopes the feature actually uses. Plaid product used: `transactions` (not `auth` — auth requires separate approval and is not needed for balance reads).

### Autonomous background access
Background data access is legitimate when:
1. User explicitly opted in — `AutoInvestConfig.Enabled = true` + `EnabledAt` timestamp stored
2. Privacy policy states the access schedule and what it does
3. Every background Plaid call is logged with timestamp, balances seen, decision made
4. User can disable at any time

### Token revocation
`DELETE /plaid/accounts/{item_id}` calls Plaid `/item/remove` first, then removes from MongoDB. Deleting the Mongo record without revoking at Plaid is a compliance violation — token stays live at Plaid until explicitly revoked.

---

## Key product decisions

| Decision | Why |
|---------|-----|
| Scrapped mood-based concept | Mood is a gimmick — financial state drives real decisions |
| App owns intelligence, AI is swappable | Not locked into Claude — Deepseek, fine-tuned model, or any model works |
| Onion arch + DDD from day one | Early architecture decisions make later features easy or painful — set them wrong and every new layer pays for it |
| CLAUDE.md in repo root | Claude Code reads it automatically — consistent engineering rules every session |
| Skills files for rarely-needed rules | CLAUDE.md loads every session; skills load on demand — saves tokens |
| Full profile collected upfront | App never asks the same question twice |
| App fetches market data, not Claude | App owns the audit trail — decisions collection records what Claude saw |
| Polygon.io over Alpha Vantage | Alpha Vantage: 25 req/day cap. Polygon free tier: unlimited previous-day data |
| Daily cache on market provider | One Polygon call per day regardless of recommendation volume |
| Plaid product: transactions not auth | Auth requires separate Plaid approval; transactions covers balance reads |
| PLAID_CACHE_TTL (default 5m) | Sandbox rate limits are real during dev; production usage won't hit them at scale |
| Clerk for auth | 50k free MAU, dev instance unlocks Pro features, clean React SDK |
| Alpaca for brokerage | Paper trading, paper → live is a config change not a code change |
| Notional (dollar-based) orders | Users think in dollars not shares |
| BROKERAGE_PROVIDER mock | Test full invest loop without touching Alpaca during dev |
| Partial order failure tolerance | One bad ticker shouldn't block the whole investment |
| Encrypted Mongo for tokens now, Vault before go-live | access_tokens are live credentials — DB compromise must not expose them |
| AutoInvestConfig as own collection | Decoupled from UserProfile; scales to multiple configs per user |
| GetByUserID returns safe default | Frontend never handles not-found — safe defaults mean no extra error path |
| MOCK_ALL=true flag | Single env var for zero-external-calls dev setup — no need to set 5 provider vars |
| Mock advisor | Full end-to-end flow testable without Anthropic key or API calls |
| AUTO_INVEST_INTERVAL env var | Human-readable duration string; 1m for dev, 24h for prod, no code change |
| MarketCalendar interface over Alpaca clock API | Alpaca's `/v2/clock` requires per-user credentials — can't be called at the system level before user fan-out. Static algorithmic NYSE calendar in `infrastructure/calendar/` needs no credentials, no network call, and is correct for any year. |
| AppShell `keepPageOnRefresh` over callback prop | A callback prop for refresh would require AppShell to expose its internal `setState` + `setProfile` externally. A boolean flag keeps state fully internal and cleanly encodes the only behavioral difference between Dev and Clerk modes. |
| Log provider for notifications | No push infrastructure needed during development |
| Failure = skip not crash | One user's Plaid/Claude/Alpaca failure must not stop the scheduler for other users |
| CORS as outermost middleware | No handler can accidentally miss CORS — one place, universal coverage |
| Git workflow: write → verify → commit | No autonomous pushes — developer reviews before any commit |
| Polygon over Finnhub for news | Polygon already integrated — no new key, no new dependency. Claude infers sentiment from text. Finnhub backlogged for when individual stocks make per-ticker sentiment worth the extra dependency |
| Dependency principle: exhaust before adding | Before adding a new API/service, check if an existing one can do the job. New keys = new cost, new failure surface, new rotation burden |
| Cash context as opt-in stored preference | Per-session directives caused Claude to override stated risk tolerance regardless of user intent. Fix: gate spending data behind a stored UserProfile preference; when shown, instruction is "background context only — do not override risk tolerance or amount." |
| Per-user Alpaca credentials over env-var keys | Single shared env-var key means every user trades the same account — always wrong for financial data or trade execution. Per-user AES-256-GCM encrypted credentials in MongoDB, same pattern as Plaid access tokens. |
| `BrokerageProviderFactory` interface to avoid arch violation | Services decrypt keys + construct Alpaca client directly would require application layer importing infrastructure packages. Factory interface in `domain/ports/` inverts the dependency. |
| `ErrBrokerageNotConnected` sentinel error | Allows callers to distinguish "not connected" (user action needed, non-fatal for recommendations) from real errors. RecommendationService skips positions; InvestmentService returns 400; scheduler skips user silently. |
| distroless runner image | `gcr.io/distroless/static-debian12` has no shell, no libc, no package manager — smaller attack surface. Requires static binary (`CGO_ENABLED=0`). |
| `/health` on top-level mux, not inside `UserIdentity` | `UserIdentity` blocks all non-`/auth/` routes. Top-level mux registers `/health` first; everything else falls through to the protected mux. |
| Per-allocation brokerage override over global override | Strictly more expressive than a global override — can still route everything to one account, or split individual allocations. Stamped on each `TradeReceipt` for full audit trail. |
| 529 fallback to cached recommendation | Hard-failing the recommendation with a 500 is bad UX for a daily-use app. Pull last decision, rescale to today's budget, set `FromCache: true`. No new storage needed. |
| P&L data from Alpaca not MongoDB | Alpaca already computes avg_entry_price, cost_basis, unrealized_pl per position. Computing from receipts would require reconstructing position state across all historical orders. |
| SVG chart, no charting library | Dependency principle: a plain SVG polyline is sufficient for a sparkline chart. Recharts/Chart.js would add ~100KB+ for one line and a few labels. |
| Polygon deduplication in verdict stamping | Naive approach: one Polygon call per decision per ticker = rapid 429 exhaustion on free tier. Fix: collect unique tickers across all decisions for one user, fetch each once, share the price cache. |
| Polygon prev-day as entry price fallback | Alpaca market orders are async — `FilledPrice = 0` at submission. Polygon prev-day close is a reasonable proxy; return will be slightly off but directionally correct. |
| Equal-weight fallback when FilledAmount = 0 | Same Alpaca async issue — `FilledAmount` is 0 at submission. Equal weighting (1.0 per ticker) gives a correct unweighted average until Alpaca updates the order. |
| `safeFloat()` at handler boundary | MongoDB can store `+Infinity` from a divide-by-zero bug. `json.Encoder` panics on Inf/NaN. `safeFloat()` converts non-finite to 0 at the serialization boundary. |
| Split age gate in `ListUnverdicted` | Age gate applies only to "no verdict yet" branch — bad-verdict decisions are re-stamped regardless of age. |
| Merged Activity + Eval into single "Activity" page | Two separate tabs require users to switch to see related data. One page with three parallel API calls gives a unified view. |
| `config_id = ""` normalized to `"manual"` in MongoDB aggregation | Legacy decisions have no `config_id` field — reads as `""`. Fix: `$addFields` + `$cond` normalizes both to `"manual"` before grouping. |
| Mongo as single source of truth for ticker classifications | Static maps rot. DB-backed seed with `$setOnInsert` lets the classification set grow without code changes. `StoreClassification` writes Claude's suggested class directly as approved — no review queue, no restart needed; the in-memory cache is updated in the same goroutine. |
| In-memory cache for classifier, zero Mongo reads per request | Classification is called in the hot path (every recommendation, every position). DB reads per request would add latency and Mongo load for a dataset that changes at most once per session. Cache hydrated at startup, refreshed only on restart. |
| `cmd/dbcheck` over mongosh for diagnostics | `mongosh` requires a separate install. A Go script using the existing `go.mongodb.org/mongo-driver` runs immediately with `go run ./cmd/dbcheck`, lives in the repo, and is available to any team member without setup. |
| Two separate SnapTrade ports (`PortfolioAggregator` + `PortfolioConnector`) | A single combined interface would force every caller to implement or mock methods it doesn't use. `RecommendationService` depends only on `PortfolioAggregator`; the handler depends only on `PortfolioConnector`. `SnapTradeClient` implements both in one struct — one file, one HTTP client. |
| Save-before-URL-generate in portfolio connect flow | If credentials are persisted after URL generation, a URL-step failure leaves a registered SnapTrade user with no local record — orphaned forever. Persist first; rollback with `DeleteUser` + `ClearPortfolioConnection` if the URL step fails. |
| Best-effort disconnect for SnapTrade | Failing the `DeleteUser` provider call and returning 500 would leave the user stuck in "connected" UI state. Local clear proceeds regardless; provider-side cleanup is logged and monitored but not user-blocking. |
| Alpaca positions take precedence over SnapTrade in deduplication | Alpaca positions are held and traded in this app — they are the ground truth for cost basis, entry price, and P&L. SnapTrade read-only positions are context for Claude, not authoritative. First-seen-wins with Alpaca fetched first enforces this without special-casing. |

---

## Known debt

| Shortcut | Future fix |
|----------|-----------|
| Polygon market data is previous-day close | `/prev` endpoint always gives yesterday's prices — Claude recommends based on stale data if the market moves overnight. Acceptable for personal/dev use; becomes misleading in volatile sessions. Real-time quotes (e.g. Finnhub) fix this but add a new dependency — deferred until this meaningfully hurts recommendation quality |
| Paper trading only | Swap `ALPACA_BASE_URL` + keys for live. Per-user credentials already wired — no code change, just a UI account type selector + real keys |
| Log provider for notifications | Real push (FCM or APNs) for mobile; Resend covers email |
| Plaid balance cache is in-memory | Cache resets on restart; Redis for persistence at scale |
| Encrypted Mongo for Plaid tokens | Move to Vault / AWS Secrets Manager before go-live |
| `[8a-debug]` log lines in `recommendation_service.go` | Temporary prompt-debugging logs; remove when no longer needed — grep `[8a-debug]` |
| `[8b-debug]` log lines in `recommendation_service.go` | Temporary news-debugging logs; remove when no longer needed — grep `[8b-debug]` |
| `[8c-debug]` log lines in `recommendation_service.go` | Temporary spending-debugging logs; remove when no longer needed — grep `[8c-debug]` |
| `[8ca-debug]` log lines in `recommendation_service.go` | Temporary cash-context-debugging logs; remove when no longer needed — grep `[8ca-debug]` |
| `CashContext.UserOverride` field is dead | Field exists in the model but is never set; left over from an abandoned per-session override design. Remove when cleaning up. |
| Prompt tests are white-box string assertions | `buildUserMessage` is package-private; tests in same package. Future: LLM-level assertion tests (does Claude actually respect the 40% rule?), fuzz tests on allocation sum, regression snapshot tests |
| Verdict entry price uses Polygon proxy | When `FilledPrice = 0` (async Alpaca order), Polygon prev-day close is used as entry price. Understates/overstates return vs actual fill. True fix: re-fetch Alpaca order after a delay and re-stamp with real fill price |
| Classification bad data has no correction UI | If Claude mis-classifies a ticker (e.g. calls a bond ETF "US Equity"), the fix is a manual Mongo update. Acceptable for now — `cmd/dbcheck` makes it easy to spot. No admin UI planned until mis-classifications become a real pattern |

---

## Retrospectives

### Build the pipeline without tests, pay for it when you surface it in the UI

The verdict data pipeline (verdict stamping, Polygon/Alpaca price fetching, MongoDB stamping) was built with no unit tests — every requirement existed only in the code, nothing machine-verifiable. When verdict data was surfaced in the Activity UI for the first time, five separate bugs appeared that were only visible at runtime:

| Bug | Root cause | What it cost |
|-----|------------|-------------|
| Polygon 429 rate limit | Per-ticker fetch in a loop; one API call per decision × N tickers = rapid exhaustion | Extra deploy cycle to add deduplication |
| `FilledPrice = 0` → `Infinity` | Alpaca async orders have `FilledPrice=0` at submission; dividing by zero produced Inf | Extra deploy cycle to add equal-weight fallback |
| `json.Encoder` crash on Inf/NaN | JSON cannot encode IEEE Infinity; the `/eval/summary` endpoint panicked for affected users | Extra deploy cycle to add `safeFloat()` guard |
| Age gate blocked re-stamping of bad verdicts | `minAge` cutoff applied to ALL decisions including already-stamped-wrong ones | Extra deploy cycle to split age gate: new decisions only |
| `config_id = ""` broke strategy grouping | Legacy decisions (before config_id was introduced) have no `config_id`; MongoDB `$group` treated `""` and `"manual"` as separate buckets | Extra deploy cycle + frontend merge logic |

**Rule reinforced:** write unit tests before or alongside the pipeline, not after. `verdict_stamper_test.go`, `eval_handler_test.go`, and `Eval.test.tsx` were added retroactively — all five bugs are now machine-verifiable and any regression fails immediately in CI rather than in the UI.

---

### Verdict retry condition too broad → infinite re-queue loop

The `badVerdict` mongo condition `{"verdict.ticker_verdicts": {"$size": 0}}` was added to handle a legitimate case: Alpaca async orders where `FilledPrice=0` at submission get stamped with zero tickers, then corrected when brokerage data arrives. That condition means "empty ticker list = retry later."

Decisions made before the verdict pipeline existed also produce empty ticker lists — but for a permanent reason: no SPY entry price and no brokerage receipts were ever recorded. After being stamped with `tickers=[]`, they matched the same retry condition and were re-queued indefinitely. On a short dev tick (e.g. `5m`) this produced continuous log noise and pointless DB reads.

**Fix:** both `ListUnverdicted` and `GetUsersWithPendingVerdicts` now require `market_snapshot.spy_price > 0`. Old decisions with `spy_price=0` (or the field absent) are permanently excluded from the verdict queue.

**Rule reinforced:** when adding a "retry on bad state" condition, enumerate the states that can actually be corrected vs. states that are permanently terminal. Terminal states need a separate filter or a `skipped` flag — otherwise they re-queue forever.

---
