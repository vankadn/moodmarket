# InvestIQ — Project Context & Master Reference

> Load this into your Claude Project so every new conversation starts with full context.
> Last updated: 2026-05-16 (Phase 16b — Custom domain + Resend domain verified)

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
- `NotificationProvider` — post-invest notifications; `SendInvestmentSummary`, `SendInvestmentFailure`, `SendMarketClosed`; log provider (dev), Resend email provider (prod)
- `SecretsProvider` — sensitive credential retrieval (pre go-live, Vault / AWS Secrets Manager)

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
| **SnapTrade** | Portfolio aggregation — read positions, balances, holdings from Robinhood and Fidelity | OAuth, per-user token | Planned (Phase 16) |
| **Coinbase Advanced Trade** | Crypto execution | API key auth (per-user, encrypted at rest) | Planned (Phase 17) |

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
| `Allocation` | Single position recommendation (ticker, amount, %) |
| `Recommendation` | Full AI response: allocations + summary + risk level + `FromCache bool` (true when returned from MongoDB fallback due to advisor overload) |
| `InvestmentDecision` | Persisted record: userId, timestamp, market snapshot, allocations, trade receipts |
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
| `AutoInvestConfig` | First-class domain model (own collection): Enabled, Amount, Risk, EnabledAt, UpdatedAt |
| `SchedulerRun` | Audit record for one autonomous cycle: RunID, StartedAt, CompletedAt, UsersProcessed, TotalInvested, Errors |
| `NotificationTarget` | Delivery target for one notification: UserID, Email, Phone, Source (`"manual"` or `"auto"`) |
| `HistoryPoint` | One data point in a portfolio value time series: Timestamp (Unix epoch seconds), Equity, ProfitLoss, ProfitLossPct |
| `NewsItem` | One market headline: Headline, Summary, Source, PublishedAt |
| `TransactionSummary` | Aggregated spending signals: SpendLast7Days, SpendLast30Days, LargestPendingAmount, LargestPendingName, PulledAt |

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

Auto-invest config (amount, risk, enabled) lives in `AutoInvestConfig` — its own collection, not on UserProfile.

---

## MongoDB collections

| Collection | Purpose |
|-----------|---------|
| `users` | User profile + financial goals + Plaid connections array |
| `auto_invest_configs` | One document per user: enabled, amount, risk, EnabledAt, UpdatedAt |
| `decisions` | Every daily investment decision: userId, timestamp, market snapshot, allocations, receipts, Plaid snapshot |
| `scheduler_runs` | One document per autonomous cycle: runID, timestamp, users processed, total invested, errors |

---

## Phase completion log

### Phase 1 — Complete
- Go backend with `/recommend` endpoint calling Claude API
- React + TypeScript frontend with Vite
- `InvestmentAdvisor` interface with Claude implementation
- Provider abstraction pattern established
- App renamed from MoodMarket (mood-based concept scrapped)
- CLAUDE.md created in repo root

### Phase 2 — Complete
- MongoDB connected, `.env` auto-loaded at startup
- Full `UserProfile` schema with 10 fields
- `POST /users/profile` and `GET /users/profile` endpoints
- `ProfileRepository` interface with MongoDB implementation
- `IdentityProvider` interface — userId from context only, never hardcoded
- `AuthProvider` interface with `DevAuthProvider` and `ClerkAuthProvider` stub
- `DEV_MODE=true` auto-logins as dev user — logic only in `factory.go`
- Frontend login skeleton with dev login button

### Phase 3 — Complete
- `MarketDataProvider` interface in `domain/ports/`
- Polygon.io implementation using `/v2/aggs/ticker/{ticker}/prev` (free tier)
- Sector ETF coverage: SPY, QQQ, XLE, XLF, XLV, XLI
- Market sentiment from SPY % change: bullish / neutral / bearish
- Daily in-memory cache on Polygon provider — one fetch per day
- Mock provider (`MARKET_PROVIDER=mock`) — zero API calls in dev
- `DecisionRepository` — every recommendation persisted with full context
- Claude hardened: prompt caching + 3-attempt retry + 5s/10s exponential backoff for 529 errors
- Retry split: parse errors get correction prompt, API errors get clean retry
- Claude prompt enriched with full market context + user profile

### Phase 4 — Complete

**Phase 4a — Clerk Auth**
- `ClerkAuthProvider` filled in — JWT verification via Clerk backend API (Go stdlib only)
- Frontend: `@clerk/clerk-react`, app wrapped in `<ClerkProvider>`
- Login: email + password and Google SSO
- All API requests attach Clerk session JWT as `Authorization: Bearer`

**Phase 4b — Alpaca Paper Trading**
- `BrokerageProvider` interface — `PlaceMarketOrder`, `GetPositions`
- `TradeOrder` + `TradeReceipt` + `Position` value objects
- Alpaca implementation — notional (dollar-based) market orders via paper API
- Mock provider — zero API calls, hardcoded receipts
- `application/services/investment_service.go` — orchestrates allocations → orders → persist
- `POST /invest` handler — returns receipts + decisionId
- Frontend: `ConfirmScreen.tsx`, `ReceiptScreen.tsx`, Home state machine: idle → confirming → investing → receipt

### Phase 5 — Complete
- `FinancialDataProvider` interface in `domain/ports/`
- Plaid REST client — net/http only, no SDK
- `PlaidConnection`, `BankAccount`, `BalanceSummary` value objects
- Per-user `plaid_connections` array on user document; AES-256-GCM encryption on access_token
- `GET /users/profile` returns institution + item_id only — access token never exposed to frontend
- `RecommendationService` fetches live balances before every recommendation — fetch failure is non-fatal
- Claude prompt enriched with cash position, total investments, institution summary
- `POST /plaid/link-token`, `POST /plaid/exchange`, `DELETE /plaid/accounts/{item_id}` handlers
- Token revocation calls Plaid `/item/remove` before removing from Mongo
- Frontend: `Profile.tsx` with connected account management, Plaid Link popup
- Plaid product: `transactions` (not `auth` — auth requires separate Plaid approval)
- `PLAID_ENV=production` — 10 free real connections

### Phase 6 — Complete

**Goal:** Fully autonomous investment agent running on a configurable interval without user interaction.

**Scheduler:**
- `application/scheduler/auto_invest_scheduler.go` — `time.Ticker` loop, concurrent user fan-out with `sync.WaitGroup`
- `application/scheduler/auto_invest_runner.go` — single-user pipeline reusing existing services
- `AUTO_INVEST_INTERVAL` env var parsed via `time.ParseDuration` — any valid duration, no code change needed
- Started as goroutine in `main.go` alongside HTTP server

**Failure handling:**
- Plaid fetch fails → log and skip user for this cycle
- Claude fails → 3 retries, then skip
- Alpaca order fails → log partial failure, continue other tickers
- One user's failure never blocks others

**Infrastructure:**
- `domain/ports/notification_provider.go` — `NotificationProvider` interface
- `infrastructure/notifications/log_provider.go` — logs to stdout (dev); factory routes via `NOTIFICATION_PROVIDER`
- `infrastructure/db/mongo_scheduler_repository.go` — `scheduler_runs` collection
- CORS moved to outermost `middleware.CORS()` wrapper in `main.go` — covers all routes universally

### Phase 6b — Complete

**Goal:** Promote AutoInvestConfig to a first-class domain model with its own collection and a dedicated settings screen.

**Why:** Auto-invest needs its own amount and risk (separate from profile defaults). Own collection scales to multiple configs per user in Phase 9.

**Backend:**
- `domain/models/auto_invest_config.go` — Enabled, Amount (float64), Risk (RiskTolerance), EnabledAt, UpdatedAt
- `domain/ports/auto_invest_repository.go` — GetByUserID, Upsert, GetAllEnabled
- `infrastructure/db/mongo_auto_invest_repository.go` — `auto_invest_configs` collection; GetByUserID returns safe default (disabled, $100, moderate) if no doc exists — never errors on not-found
- Scheduler updated: uses `AutoInvestRepository.GetAllEnabled()` + passes `config.Amount` and `config.Risk` into runner
- `api/handlers/auto_invest_config_handler.go` — `GET /users/auto-invest/config`, `PUT /users/auto-invest/config`
- Removed `auto_invest_enabled` / `auto_invest_enabled_at` from `UserProfile`
- Removed `SetAutoInvest` / `GetAutoInvestUsers` from `ProfileRepository`

**Frontend:**
- `AutoInvestSettings.tsx` — toggle, dollar amount input, risk pill selector (3 buttons), save button
- `Home.tsx` — auto-invest row navigates to settings; shows "Enabled — $X/day" or "Off"
- `api.ts` — `AutoInvestConfig` type, `getAutoInvestConfig()`, `saveAutoInvestConfig()`
- Home layout: Auto-invest row → Today's investment input + Get recommendation (inline, same row)

### Phase 6 infra cleanup — Complete

- Mock advisor (`AI_PROVIDER=mock`) — fixed three-fund portfolio (VTI 60% / VXUS 30% / BND 10%), no Anthropic key needed
- `MOCK_ALL=true` — single flag overrides all providers to mock + sets DEV_MODE=true; processed in `main.go` before factories
- `PLAID_CACHE_TTL` env var (default `5m`) — caches `GetBalanceSummary` result to reduce Plaid API calls during development
- Skills system: logging rules, React rules, new-feature checklist, pre-commit checklist moved to `skills/` files — loaded on demand to save tokens
- README rewritten: zero-external-calls setup, provider swap table, Plaid and Alpaca config sections

### Phase 7 — Complete

**Goal:** Activity dashboard showing what the user has done through InvestIQ. No profit/loss — activity only.

**Dashboard:**
- Total decisions made + total dollars invested — both filtered by selected time range
- Time range filter: three numeric inputs (hours / days / months) — hours accepts decimals (e.g. 0.5 = 30 min), useful for local scheduler testing; user fills any combination, app combines into a date range; includes a reset button that defaults back to 30 days
- Investment timeline: list of decisions within selected period showing date + amount invested
- All data aggregated from `decisions` collection
- `GET /users/activity?since=<RFC3339>` endpoint; `ListByUserSince` on `DecisionRepository`
- Frontend: `Activity.tsx` with stats cards + timeline list

**Receipt screen:**
- Polls Alpaca every 3s for non-terminal orders; terminal set: filled, canceled, expired, rejected, replaced
- Per-order polling with two stop conditions (independent per order):
  - After-hours: 5 consecutive `accepted` polls → stop, show "Orders accepted. Will fill when market opens (Mon–Fri 9:30am–4pm ET)."
  - Max attempts: 20 polls (~60 s) → stop, show "Market may be closed — check back when market opens."
- Stop state tracked via refs (readable in closure) + `stopNotes` state (drives UI and `allSettled`)
- Next poll scheduled inside `.then()` — prevents request pile-up on slow connections
- "polling for fill…" indicator only visible while at least one order is still actively polling

### Phase 8 — Complete

**Goal:** Make the Claude prompt as strong as possible with maximum real context. Sub-phases ordered by benefit — do 8a before touching news or Plaid.

---

### Phase 8a — Complete

**Prompt strengthening with existing data (zero new APIs)**

- `RecommendationService` now has 6 steps: profile → market snapshot → Plaid balances → Alpaca positions → decision history → Claude
- `BrokerageProvider.GetPositions()` called before building prompt; injected as `req.Positions`
- `DecisionRepository.ListByUser(ctx, userID, 10)` called; last 5 decisions injected as `req.RecentDecisions`
- Concentration rule: any position at ≥ 40% of portfolio value gets "← already at concentration limit, do not add more" appended to its line; system prompt instructs Claude not to push any ticker above 40%
- Diversity rule: last 5 decision allocations shown in prompt with "Vary today's allocation — do not repeat the exact same split as yesterday"
- `[8a-debug]` temporary logs added with `// TODO: remove after 8a testing` markers — grep for `[8a-debug]` to find them
- System prompt rewritten: numbered rules, explicit output contract, tighter rationale constraint (under 12 words)
- Prompt caching: system prompt sent as `[]claudeSystemBlock` with `cache_control: ephemeral`; `anthropic-beta: prompt-caching-2024-07-31` header added
- Retry: 3 attempts, 5s/10s exponential backoff; parse errors get correction turn, API errors get clean retry
- Basic prompt tests: `infrastructure/advisor/prompt_test.go` — 12 table-driven cases covering concentration warning boundary, history section presence, balance fallback, profile inclusion, budget math; marked as tech debt for LLM-level assertion tests

**Concentration boundary (documented from test writing):** The `>= 40` check fires at exactly 40%, consistent with the rule "do not push above 40%" — any addition to a position already at 40% would exceed the limit. Test data for the "no warning" case must use positions where all are strictly under 40%.

---

### Phase 8b — Complete

**Polygon news — zero new dependencies, existing key**

**Why Polygon over Finnhub:** Already using Polygon for market data (same free-tier key). Adding Finnhub would be a new key, new dependency, new failure surface. Claude infers sentiment from headline text well enough at this stage. Finnhub backlogged for when per-ticker sentiment on individual stocks becomes genuinely valuable.

- `domain/models/news.go` — `NewsItem`: Headline, Summary, Source, PublishedAt
- `domain/ports/news.go` — `NewsProvider.GetDailyNews(ctx) ([]NewsItem, error)`
- `infrastructure/news/polygon.go` — Polygon `/v2/reference/news?ticker=SPY&limit=10`; daily cache; reuses `POLYGON_API_KEY`
- `infrastructure/news/mock.go` — 3 hardcoded headlines (Fed, S&P, Oil)
- `infrastructure/news/factory.go` — `NEWS_PROVIDER=polygon|mock`; defaults to mock
- `MOCK_ALL=true` sets `NEWS_PROVIDER=mock`
- `RecommendationService` now 7 steps: profile → market → Plaid → positions → decisions → news → Claude
- News failure is non-fatal — recommendation proceeds without headlines
- Claude prompt: `TODAY'S MARKET NEWS` section (top 5) with source + headline; instruction to factor in macro events
- New env var: `NEWS_PROVIDER=polygon` (POLYGON_API_KEY already required by market data)
- Prompt tests updated: 3 new cases added (`no_news_omits_section`, `news_present_shows_section_and_macro_instruction`, `news_capped_at_five_headlines`) — total now 15 cases
- `[8b-debug]` temporary logs added with `// TODO: remove after 8b testing` markers — grep `[8b-debug]` to find them

---

### Phase 8c + 8ca — Complete

**Plaid transaction history**

- `domain/models/banking.go` — `TransactionSummary`: SpendLast7Days, SpendLast30Days, LargestPendingAmount, LargestPendingName, PulledAt
- `domain/ports/financial_data_provider.go` — `GetTransactionSummary(ctx, connections) (TransactionSummary, error)` added to existing interface
- `infrastructure/banking/plaid.go` — `/transactions/get` with 30-day window; sums positive amounts (debits only, skip credits/refunds); tracks largest pending charge
- `infrastructure/banking/mock.go` — realistic fixture: $342.50/7d, $1240/30d, $189 Netflix pending
- `RecommendationService` now 8 steps — transactions (step 4) inserted right after balances (step 3); both Plaid, failures non-fatal
- Claude prompt: `SPENDING HISTORY` section — 7d spend, 30d spend, largest pending charge, estimated cash runway (TotalCash ÷ daily avg); instruction to consider smaller investment if runway is short
- Cash runway computed in `buildUserMessage` using `req.BalanceSummary.TotalCash` ÷ `(SpendLast30Days/30)` — requires both sections present
- `[8c-debug]` temporary log added — grep to remove after testing
- Prompt tests: 4 new cases (absent, spend figures, pending charge, runway calculation) — total now 19

**Phase 8ca — Cash context surface (redesign: preference-gated, FYI-only)**

**Why redesigned:** Original implementation sent per-session cash directives to Claude ("Factor this into allocation — consider more conservative positions"). This caused Claude to override the user's stated risk tolerance even when the user had confirmed the amount. Root cause was the conditional directive logic in `buildUserMessage`, not the data itself.

**What was removed:**
- `CashOverride bool` from `InvestmentRequest` — per-session override gone entirely
- "Invest anyway" and "Adjust amount" buttons from `CashContextCard`
- `cashOverride` state and `amountInputRef` from `Home.tsx`
- The three-way directive conditional from `buildUserMessage` ("Respect this decision" / "Factor into allocation" / silent)

**What was added / kept:**
- `domain/models/cash_context.go` — `CashContext`: HasData, RunwayDays, RunwayLabel, SpendLast7D, SpendLast30D, LargestPendingAmount, LargestPendingName, Message (kept; UserOverride field is dead code — never set)
- `domain/models/user_profile.go` — `IncludeCashContext bool` (`json:"include_cash_context"`) — stored opt-in preference, default false; no DB migration needed
- `application/services/recommendation_service.go` — `GetCashContext(ctx, userID)` method unchanged; `runwayLabelAndMessage`: >30d=healthy, 14-30d=moderate, <14d=tight
- `infrastructure/advisor/claude.go` — `buildUserMessage` SPENDING HISTORY section replaced with SPENDING CONTEXT; gated on `profile != nil && profile.IncludeCashContext && req.TransactionSummary != nil`; if true: emits 7d spend, 30d spend, cash runway (if BalanceSummary present), and "Use as background context only — do not override the user's stated risk tolerance or investment amount."; if false: section omitted entirely
- `api/handlers/cash_context_handler.go` — `GET /users/cash-context` unchanged
- `frontend/src/components/CashContextCard.tsx` — redesigned: FYI-only, no buttons, auto-dismiss after 5s or tap; amber tint only (tight runway only)
- `frontend/src/pages/Home.tsx` — localStorage daily gate on mount (`cash_card_shown_date` = ISO date string); card only shown when `runway_label === "tight"` AND not already shown today; `handleCashCardDismiss` writes localStorage and clears card; `cash_override` removed from recommendation request
- `frontend/src/pages/AutoInvestSettings.tsx` — "Include cash balance context" toggle; loads profile on mount alongside auto-invest config; saves both on save button
- `frontend/src/services/api.ts` — `include_cash_context` added to `UserProfile`; `cash_override` removed from `InvestmentRequest`
- Prompt tests: replaced 5 old spending/override cases with 5 new opt-in gate cases; total now 20 cases
  - `no_transactions_omits_section` — mustNotContain SPENDING CONTEXT
  - `spending_context_omitted_without_opt_in` — TransactionSummary present but IncludeCashContext=false → no section
  - `spending_context_shown_when_opted_in` — IncludeCashContext=true → SPENDING CONTEXT, "background context only", spend figures
  - `spending_context_with_runway_when_opted_in` — IncludeCashContext=true + BalanceSummary → Cash runway shown
  - `spending_context_omitted_no_profile` — nil profile → no section (can't opt in without profile)
- `[8ca-debug]` temporary log in recommendation_service.go — grep to remove after testing

### Phase 8d — Complete

**Per-user Alpaca brokerage credentials**

**Why:** Every user was sharing a single Alpaca account via env-var keys. Each user must trade their own account. Credential storage follows the same AES-256-GCM pattern established for Plaid access tokens.

**New domain models (`domain/models/user_profile.go`):**
- `BrokerageConnection` — internal encrypted record (APIKey, SecretKey, BaseURL, Connected, ConnectedAt `time.Time`); no JSON tags; never leaves the backend
- `BrokerageStatus` — JSON-safe projection (connected, base_url, connected_at); embedded as `Brokerage *BrokerageStatus` in `UserProfile`

**New port (`domain/ports/brokerage_factory.go`):**
- `BrokerageProviderFactory` interface — `ForUser(conn *models.BrokerageConnection) (BrokerageProvider, error)`
- `ErrBrokerageNotConnected` sentinel — fatal in InvestmentService (400), non-fatal in RecommendationService (skip positions), clean skip in scheduler

**Profile repository (`domain/ports/profile_repository.go` + `infrastructure/db/mongo_profile_repository.go`):**
- 3 new repo methods: `GetBrokerageConnection`, `SaveBrokerageConnection`, `ClearBrokerageConnection`
- `GetBrokerageConnection` decrypts both keys; `SaveBrokerageConnection` encrypts both keys using existing `EncryptToken`/`DecryptToken` from `encryption.go`
- `ClearBrokerageConnection` uses MongoDB `$unset` to remove the field entirely — mirrors Plaid pattern
- Bug fix: `include_cash_context` was missing from `profileDocument`, `fromProfile`, and `toProfile` — silently dropped on every profile save; fixed in the same pass

**Factory rewrite (`infrastructure/brokerage/factory.go`):**
- Replaced `NewBrokerageProvider()` (global, env-var keys) with `NewBrokerageFactory()` returning `BrokerageProviderFactory`
- Mock implementation: `ForUser` ignores conn, always returns mock provider
- Alpaca implementation: `ForUser` returns `ErrBrokerageNotConnected` if conn nil or not connected; otherwise constructs `AlpacaProvider` from decrypted per-user keys

**Service changes:**
- `RecommendationService` — `brokerageFactory` replaces `brokerageProvider`; loads connection from repo per request; missing brokerage = skip positions (non-fatal, logs)
- `InvestmentService` — `profileRepo` added; loads connection per request; missing brokerage = fatal 400
- `auto_invest_runner.go` — `errors.Is(err, ErrBrokerageNotConnected)` check added; clean skip with log instead of error

**New endpoints:**
- `POST /brokerage/connect` — validates non-empty keys, defaults base_url to paper, calls `SaveBrokerageConnection`; never logs credentials
- `DELETE /brokerage/connect` — calls `ClearBrokerageConnection`

**Handler update (`api/handlers/order_handler.go`):**
- Changed from injected `BrokerageProvider` to `profileRepo + brokerageFactory`; per-request credential load; returns 400 for `ErrBrokerageNotConnected`

**Frontend (`BrokerageConnect.tsx` new, `Home.tsx`, `App.tsx`, `api.ts`):**
- `BrokerageConnect.tsx` — full-page component; connected state: green card + disconnect; not-connected: API key input, secret key (password type), account type pill selector (Paper / Live)
- `Home.tsx` — "Brokerage" nav button (red when not connected); invest button disabled when not connected; red nudge banner with "Set up now →" link
- `App.tsx` — `"brokerage"` state added to `DevAppState`; routing to `BrokerageConnect` component
- `api.ts` — `BrokerageStatus` interface; `brokerage?: BrokerageStatus` on `UserProfile`; `connectBrokerage()` and `disconnectBrokerage()`

**Credential storage skill (`skills/credential-storage-rules.md`):**
- Per-user vs per-app credential table with enforcement rule
- Anti-pattern callout: "That Alpaca key is in `.env` — every user trades against the same account."
- Wired into `CLAUDE.md` under Skills — loads when integrating any third-party service

---

### Phase 9 — Complete

- Railway backend deployed: ~~moodmarket-production.up.railway.app~~ → now api.investiq.fit (see Phase 16b)
- Vercel frontend deployed: ~~moodmarket-mu.vercel.app~~ → now www.investiq.fit (see Phase 16b)
- MongoDB Atlas free tier connected
- Per-user Alpaca credentials (encrypted, per-user in Mongo)
- CORS fixed via ALLOWED_ORIGIN env var
- BrokerageConnect reachable in production (ClerkApp.tsx fix)
- Real trades placing on Alpaca live account
- Atlas IP allowlist: 0.0.0.0/0 (temporary — tighten when Railway Pro)
- Favicon added (SVG + PNG)
- Polling fix: stop after 20 attempts or 5 consecutive ACCEPTED (after-hours)
- Alpaca live account funding pending — support ticket filed

---

### Phase 9a — Complete

**Backend containerization**

- `Dockerfile` (repo root) — two-stage build: `golang:1.23-alpine` builder → `gcr.io/distroless/static-debian12` runner
- Builder: copies `go.mod` + `go.sum` first for layer caching, then source; builds with `CGO_ENABLED=0 GOOS=linux` for a fully static binary (required — distroless has no libc)
- Runner: copies only the binary; no shell, no package manager, minimal attack surface
- `.dockerignore` (repo root) — excludes `.env`, `.env.*`, `frontend/`, `*.md`, `.git`, `.gitignore`; keeps build context small and prevents secrets from reaching the Docker daemon
- `GET /health` endpoint added — registered on a top-level mux before the `UserIdentity` middleware so Railway / Docker healthchecks work without a bearer token; all other routes still require auth
- PORT was already read from `os.Getenv("PORT")` with `"8080"` fallback — no change needed

### Phase 10 — Complete

**Goal:** Claude fetches market news itself via tool use instead of Go pre-fetching and injecting it into the prompt.

- `get_market_news` tool defined in `infrastructure/advisor/claude.go` with Polygon as the backing provider
- `claudeAdvisor` receives `NewsProvider` via constructor injection — infrastructure imports domain port (correct direction)
- Tool-use loop in `callClaudeWithTools`: TURN N → Claude API → if `stop_reason=tool_use`, execute tool locally, append result, loop; if `end_turn`, parse JSON
- Each tool removed from `remainingTools` after first call — prevents Claude requesting the same tool twice (nil-slice-in-interface → JSON null bug fixed)
- `doAPICall` uses `context.WithTimeout(context.Background(), 45s)` per call with goroutine propagating parent cancellation — decouples individual Claude calls from short HTTP request deadlines
- `RecommendationService` drops `news ports.NewsProvider` — 7 steps now (was 8); news owned by advisor
- Mock guard added to all provider factories: if provider=mock and `DEV_MODE != "true"`, startup fails — production can never silently use mock data
- Mock defaults removed from all factories — missing env var now fails fast with a helpful message instead of silently using fake data
- `NEWS_PROVIDER=polygon` added to `.env` (shares existing `POLYGON_API_KEY`)
- Structured logs tell the agentic story: `TURN N →` / `TURN N ←` / `TOOL name →` / `TOOL name ←` with consistent indentation
- Prompt tests: 3 old news-in-prompt cases replaced with 1 `news_absent_from_prompt_claude_fetches_via_tool` — 18 cases total, all passing

### Phase 11 — Complete

**Goal:** RAG document intelligence — users upload tax PDFs; Claude extracts structured fields; extracted data is injected into every recommendation prompt.

**Backend:**
- `domain/models/tax_document.go` — `TaxDocument`: ID, UserID, DocumentType (w2/1099/1098), TaxYear, Fields (map[string]string), Verified, UploadedAt, VerifiedAt
- `domain/ports/document_extractor.go` — `DocumentExtractor.ExtractTaxDocument(ctx, bytes, type)` interface
- `domain/ports/document_repository.go` — `DocumentRepository`: Save, GetByUserID, GetByID, DeleteByID
- `infrastructure/extractor/claude_extractor.go` — Claude document API (base64 PDF); type-specific prompts per form; 3-attempt retry; 60s timeout; no PDF library needed
- `infrastructure/extractor/mock_extractor.go` — realistic fixtures for W2, 1099, 1098
- `infrastructure/extractor/factory.go` — `DOCUMENT_EXTRACTOR=claude|mock`
- `infrastructure/db/mongo_document_repository.go` — `tax_documents` collection; form-specific upsert keys (W2: user+type+year+employer, 1099: user+type+year+payer, 1098: user+type only)
- `application/services/document_service.go` — orchestrates extract → persist; PDF bytes never stored
- `api/handlers/document_handler.go` — `POST /documents/upload` (multipart, PDF only, 10MB max), `GET /documents`, `DELETE /documents/:id`
- `infrastructure/advisor/claude.go` — `buildUserMessage` extended: TAX DOCUMENTS section injected if docs exist; per-type field display (employer+wages+withholding, payer+income+type, lender+interest+principal)
- `RecommendationService` extended: `ListDocuments` called before Claude; non-fatal on failure

**Frontend:**
- `services/api.ts` — `TaxDocument` interface, `DocumentType` type, `listDocuments()`, `uploadDocument(file, type)`, `deleteDocument(id)`
- `pages/Documents.tsx` — type selector (W2/1099/1098 pills), PDF file picker, upload + extract button with "Claude is reading…" indicator; document list with per-type key field grid; two-step delete confirmation; empty state with explanation
- `App.tsx` + `ClerkApp.tsx` — `"documents"` state added to both app shells; routed to `<Documents onBack />` component
- `pages/Home.tsx` — "Tax docs" nav button added; `onDocuments` prop wired through both shells

### Phase 12a — KTLO (Complete)

**Goal:** Make the backend easier to test and maintain before adding new brokerages.

**Router refactor:**
- `internal/api/router/routes.go` — 17 URI constants (`HealthURI`, `ProfileURI`, `DocumentsUploadURI`, etc.); single source of truth for all paths
- `internal/api/router/router.go` — `Build(Handlers, AuthProvider) http.Handler`; all `mux.Handle()` calls live here using the URI constants; two-tier mux (health + docs without auth, everything else behind CORS + UserIdentity)
- `cmd/server/main.go` — now only constructs handlers and calls `router.Build()`; no path strings

**Swagger UI:**
- `internal/api/handlers/openapi.yaml` — full OpenAPI 3.0.3 spec: all 19 endpoints with schemas, enums, request/response bodies, auth scheme (bearerAuth)
- `internal/api/handlers/docs_handler.go` — serves Swagger UI (CDN-loaded) at `/docs/` and raw spec at `/docs/openapi.yaml`; spec embedded at compile time via `//go:embed`; no auth required, registered on top-level mux

**Postman collection:**
- `postman/InvestIQ.postman_collection.json` — v2.1 collection; 20 requests in 7 folders (System, Auth, Users, Investment, Plaid, Brokerage, Documents); Dev Login test script auto-sets `authToken` collection variable
- `postman/InvestIQ_Local.postman_environment.json` — local dev environment; all variables pre-declared
- `postman/InvestIQ_Production.postman_environment.json` — production environment pointing at Railway URL

**Skills:**
- `skills/postman-update-rules.md` — rules for keeping Postman + OpenAPI in sync after any endpoint change; wired into `CLAUDE.md`

### Phase 12b — Multi-Brokerage Routing (Complete)

**Goal:** Connect multiple brokerage accounts and route allocations by asset type (e.g. bonds → one account, stocks → another).

**Domain changes:**
- `AssetCategory` type + constants (`equity`, `bond`, `default`) in `models/user_profile.go`
- `BrokerageConnection` gains `ID`, `Name`, `AssetCategories`; `BrokerageStatus` same
- `UserProfile.Brokerage *BrokerageStatus` → `Brokerages []BrokerageStatus` (breaking — frontend updated simultaneously)

**Application layer:**
- `services/brokerage_router.go` (new) — `NormalizeAssetCategory()` maps Claude's free-form type strings; `RouteAllocation()` selects connection by asset category with default fallback
- `services/investment_service.go` — groups allocations by routed connection; per-group provider construction; accepts `perAllocBrokerage map[string]string` (ticker → connectionID) — non-nil map overrides auto-routing per allocation; stamps `BrokerageID` and `BrokerageName` on each `TradeReceipt` after order placement
- `services/recommendation_service.go` — loops all connections for positions fetch; deduplicates by ticker (first-seen wins)

**Repository:**
- `ports/profile_repository.go` — replaced `GetBrokerageConnection` with `GetBrokerageConnections`; old save/clear renamed to `SaveLegacySingleBrokerageConnection` / `ClearLegacySingleBrokerageConnection`; added `UpsertBrokerageConnection` (upsert by ID), `RemoveBrokerageConnection`
- `infrastructure/db/mongo_profile_repository.go` — `brokerage_connections` array field; backward-compat read synthesizes `ID="default"` from legacy `brokerage_connection` field on first read without auto-save

**API:**
- `POST /brokerage/connections` — add named connection with asset categories; ID auto-generated if omitted
- `DELETE /brokerage/connections/{id}` — remove by ID; returns 204
- `POST /invest` body gains optional `per_allocation_brokerage: map[ticker → connectionID]`; nil = full auto-route
- `TradeReceipt` gains `brokerage_id` and `brokerage_name` (omitempty) — stored in MongoDB `decisions.receipts`

**Frontend:**
- `BrokerageConnect.tsx` — redesigned: broker dropdown (Alpaca available, Fidelity/Robinhood/Schwab/E*TRADE "(not ready)"), connection list with category pills + two-step remove + credential form
- `ConfirmScreen.tsx` — gains `brokerages`, `perAllocBrokerage`, `onPerAllocChange` props; always shows "Via" column when `brokerages.length > 0`; single connection: `<select>` pre-selected and disabled; multiple connections: `<select>` with "Auto" + all options, user can change per row
- `Home.tsx` — `perAllocBrokerage` state (ticker → connID); `handlePerAllocChange`; reset on cancel/done; passes map to `invest()` as `per_allocation_brokerage`; removed global brokerage switcher dropdown (superseded)
- `api.ts` — `AssetCategory` type, updated `BrokerageStatus` / `UserProfile`, `addBrokerageConnection()`, `removeBrokerageConnection()`, `InvestRequest.per_allocation_brokerage`, `TradeReceipt.brokerage_id` + `brokerage_name`

**Backward compatibility:**
- Legacy `/brokerage/connect` endpoint still works; writes to old `brokerage_connection` field
- Users with existing single connection see identical behavior — synthesized `ID="default"` routes all allocations; "Via" column shows the account name pre-selected (non-interactive)

### Phase 12c — Advisor Overload Fallback (Complete)

**Goal:** When Anthropic's API returns HTTP 529 (overloaded), return the user's last recommendation scaled to today's budget instead of surfacing a hard error.

- `domain/ports/advisor.go` — `ErrAdvisorOverloaded` sentinel error
- `infrastructure/advisor/claude.go` — HTTP 529 response wraps `ErrAdvisorOverloaded` via `fmt.Errorf("%w: ...")` so `errors.Is` works up the call chain
- `domain/models/investment.go` — `FromCache bool` added to `Recommendation` (json: `from_cache,omitempty`; absent on normal responses)
- `application/services/recommendation_service.go` — on `ErrAdvisorOverloaded`, calls `decisionRepo.ListByUser(ctx, userID, 1)`; rescales allocation amounts proportionally to today's budget (`amount = pct * budget`); returns `Recommendation{FromCache: true}`; if no prior decision exists, the original error propagates unchanged
- `frontend/src/services/api.ts` — `from_cache?: boolean` on `Recommendation`
- `frontend/src/components/ConfirmScreen.tsx` — amber banner when `rec.from_cache` is true: "AI advisor is temporarily unavailable. Showing your last recommendation — amounts scaled to today's budget."

### Phase 13 — Portfolio P&L Dashboard (Complete)

**Goal:** Show users what they own, what it's worth, and whether they're up or down — across all connected brokerage accounts.

**Backend:**
- `domain/models/trade.go` — `Position` enriched: added `Name`, `CostBasis`, `AvgEntryPrice`, `UnrealizedPL`, `UnrealizedPLPercent`
- `infrastructure/brokerage/alpaca.go` — `GetPositions` now parses `cost_basis`, `avg_entry_price`, `unrealized_pl`, `unrealized_plpc` from Alpaca's `/v2/positions` response (already returned by the API, just not extracted); `unrealized_plpc` converted from decimal to percent
- `infrastructure/brokerage/mock.go` — realistic mock positions: VTI +13%, QQQ +8%, BND -2%
- `api/handlers/portfolio_handler.go` (new) — `GET /portfolio`: fetches positions from all connections via `BrokerageProviderFactory`, groups by brokerage, computes per-account and combined totals (`total_value`, `total_cost`, `total_unrealized_pl`, `total_unrealized_pl_percent`)
- `api/router/routes.go` + `router.go` — `PortfolioURI = "/portfolio"` registered

**Frontend:**
- `services/api.ts` — `PortfolioPosition`, `PortfolioAccount`, `Portfolio` interfaces + `getPortfolio()`
- `pages/Portfolio.tsx` (new) — summary header (value / cost / gain+loss in green/red), per-account sections when multiple brokerages connected, flat list when one; empty state when no brokerage; ← Back nav
- `pages/Home.tsx` — "Portfolio" nav button + `onPortfolio` prop
- `App.tsx` + `ClerkApp.tsx` — `"portfolio"` state added, routed to `<Portfolio onBack />`

### Phase 13b — Portfolio History Chart (Complete)

**Goal:** Google Finance-style period selector (1D / 5D / 1M / 1Y / 5Y) with SVG line chart showing portfolio equity over time.

**Backend:**
- `domain/models/trade.go` — `HistoryPoint` struct: `Timestamp int64`, `Equity`, `ProfitLoss`, `ProfitLossPct float64`
- `domain/ports/brokerage.go` — `GetPortfolioHistory(ctx, userID, period, timeframe string) ([]HistoryPoint, error)` added to `BrokerageProvider` interface
- `infrastructure/brokerage/alpaca.go` — `GetPortfolioHistory` calls Alpaca `GET /v2/account/portfolio/history?period=X&timeframe=Y&extended_hours=false`; skips zero-equity points (market-closed nulls); `profit_loss_pct` converted from decimal to percent
- `infrastructure/brokerage/mock.go` — 30-point sine-wave uptrend with period-appropriate step intervals
- `api/handlers/portfolio_handler.go` — `GET /portfolio/history?period=1D|5D|1M|1Y|5Y`; maps UI label to Alpaca period+timeframe; aggregates equity across all connections by summing parallel arrays; dispatched via path check in `ServeHTTP`
- `api/router/routes.go` + `router.go` — `PortfolioHistoryURI = "/portfolio/history"` registered before `PortfolioURI`

**Alpaca period/timeframe mapping:**

| UI | Alpaca period | timeframe |
|----|--------------|-----------|
| 1D | 1D | 5Min |
| 5D | 5D | 1H |
| 1M | 1M | 1D |
| 1Y | 1A | 1D |
| 5Y | 5A | 1D |

**Frontend:**
- `services/api.ts` — `HistoryPoint`, `PortfolioHistory`, `HistoryPeriod` type + `getPortfolioHistory(period)`
- `pages/Portfolio.tsx` — `PortfolioChart` SVG component: polyline + filled area, green/red based on first→last direction, 5 X-axis time labels, muted placeholder while loading; period pill selector (active pill = dark, others transparent); history fetches on mount + on period change

---

### Phase 14 — Per-User Auto-Invest Frequency (Complete)

- `AutoInvestConfig` gains `IntervalDays int` (0 = daily) and `LastRunAt *time.Time`
- Scheduler shifts from global env-var clock to hourly tick + per-user `isDue()` check
- `isDue`: `IntervalDays=0 → 1`; `LastRunAt=nil → run immediately`; backward-compatible — existing docs with missing fields read as zero values and run daily
- `StampLastRunAt` method added to `AutoInvestRepository` interface + Mongo impl; stamps after each successful user run
- UI: frequency pill selector (Daily / Every 2 days / Weekly) added to `AutoInvestSettings.tsx`
- Label "Daily investment amount" → "Investment amount" to match variable frequency

### Phase 15 — Reliability & Maintainability (Complete)

**Phase 15a — Market Holiday Awareness**
- `domain/ports/market_calendar.go` — `MarketCalendar` interface: `IsTradingDay(t time.Time) bool`
- `infrastructure/calendar/nyse_calendar.go` — algorithmic NYSE calendar; no external API; all rules: weekends, New Year's (with Sat→Dec 31 edge case), MLK Day, Presidents Day, Good Friday (Easter via Meeus/Jones/Butcher), Memorial Day, Juneteenth (since 2022), Independence Day, Labor Day, Thanksgiving, Christmas; all in America/New_York timezone
- `infrastructure/calendar/mock_calendar.go` — always returns true; used by MOCK_ALL
- `infrastructure/calendar/factory.go` — `MARKET_CALENDAR=nyse|mock`
- Scheduler `runCycle`: early return with log if `!calendar.IsTradingDay(now)`
- `MOCK_ALL=true` sets `MARKET_CALENDAR=mock`

**Phase 15b — App Shell Unification**
- `frontend/src/AppShell.tsx` (new) — single source of truth for all 9-state post-auth routing; props: `signOut?: () => void`, `keepPageOnRefresh?: boolean`
- `keepPageOnRefresh=false` (Dev): account/brokerage change → `setState("loading")` → useEffect → home
- `keepPageOnRefresh=true` (Clerk): fetch profile inline, stay on current page (no double fetch)
- `App.tsx` DevApp reduced to ~5 lines: localStorage token check → `<Login />` or `<AppShell />`
- `ClerkApp.tsx` reduced to ~25 lines: Clerk auth gate + token fetcher wiring → `<AppShell signOut keepPageOnRefresh />`
- Adding any new page now requires editing only `AppShell.tsx`

### Phase 16 — Email Notifications (Complete)

**Goal:** Send users an email after every investment — both manual and auto-invest — using Resend.

**Domain:**
- `NotificationTarget` gains `Source string` (`"manual"` | `"auto"`) — used to vary email copy
- `UserProfile` gains `NotificationEmail string` and `Phone string` — stored in MongoDB `users` collection; both optional, empty = channel skipped

**Backend:**
- `infrastructure/notifications/resend.go` — Resend HTTP API client; source-aware copy ("Your investment completed." vs "Your auto-invest ran today."); per-position amount backfilled from allocation when `FilledAmount == 0` (Alpaca paper orders are async); recipient + email_id logged on every send
- `infrastructure/notifications/factory.go` — `NOTIFICATION_PROVIDER=resend` + `RESEND_API_KEY` + `RESEND_FROM` activates Resend; missing keys fall back to log with warning
- `api/handlers/notification_settings_handler.go` — `GET /users/notifications` + `PATCH /users/notifications`; reads/writes only email + phone; profile-level fields, not a new collection
- `infrastructure/db/mongo_profile_repository.go` — `notification_email` and `phone` bson fields added to `profileDocument`, `fromProfile`, `toProfile` (were silently dropped before)
- `api/handlers/invest_handler.go` — `InvestHandler` gains `profileRepo` + `notifications`; fires `sendSummary` in goroutine with `recover()` after successful Execute; backfills `FilledAmount` from allocations before notifying
- `application/scheduler/auto_invest_scheduler.go` — gains `profileRepo`; loads user profile in `runCycle` to build fully-populated `NotificationTarget` (email + phone) before each user's goroutine
- `application/scheduler/auto_invest_runner.go` — accepts pre-built `NotificationTarget`; backfills `FilledAmount` from allocations; guards: skips notification when receipts == 0
- `middleware/auth.go` + `handlers/cors.go` — `PATCH` added to `Access-Control-Allow-Methods` (was missing, caused preflight failure)

**API:**
- `GET /users/notifications` — returns `{notification_email, phone}`
- `PATCH /users/notifications` — updates email + phone only; does not touch other profile fields

**Frontend:**
- `NotificationSettingsPage.tsx` (new) — email + phone inputs; on Save calls `onSaved()` which refreshes profile and returns to home
- `Home.tsx` — Notifications row below Auto-invest; shows current email or "Not configured"
- `AppShell.tsx` — `"notifications"` state added; `onSaved` wired to `refreshAndReturn("home")` so home row updates immediately
- `api.ts` — `notification_email?` + `phone?` on `UserProfile`; `NotificationSettings` interface; `getNotificationSettings()` + `updateNotificationSettings()`

**Env vars:**
```
NOTIFICATION_PROVIDER=resend
RESEND_API_KEY=re_...
RESEND_FROM=InvestIQ <noreply@investiq.fit>
```

### Phase 16b — Domain & Deployment (Complete)

No code changes. Infrastructure and configuration only.

**Custom domain — investiq.fit (Namecheap)**
- Frontend: https://www.investiq.fit (Vercel — was moodmarket-mu.vercel.app)
- Backend: https://api.investiq.fit (Railway — was moodmarket-production.up.railway.app)
- DNS records on Namecheap: A record (@) → Vercel, CNAME (www) → Vercel, CNAME (api) → Railway, TXT (_railway-verify.api) for Railway domain verification
- TLS: Let's Encrypt via Railway (api subdomain), Vercel (apex + www)
- `VITE_API_URL` updated to https://api.investiq.fit in Vercel env vars
- `ALLOWED_ORIGIN` updated to https://www.investiq.fit in Railway env vars

**Email sending — Resend domain verified on investiq.fit**
- DKIM (TXT), SPF (TXT), DMARC (TXT), MX (send → Amazon SES) records added to Namecheap
- Domain verified in Resend dashboard — real emails can now be sent to any recipient
- From address in use: `noreply@investiq.fit`

### Phase 17 — Alpaca Real Trading (Planned)

Switch from paper to live Alpaca trading. Per-user credentials already wired (Phase 8d) — this is a UI change (account type selector) + user supplying live keys. No backend code change required. Requires LLC entity before signing Alpaca Broker API agreement.

### Phase 18 — SnapTrade Portfolio Read (Planned)

Connect existing Robinhood and Fidelity accounts read-only via SnapTrade. Goals:
- Aggregate positions, balances, and holdings from both accounts into a unified portfolio view
- Inject aggregated holdings context into Claude recommendation prompt
- OAuth-based connection flow (no copy-paste API keys)

Do not implement SnapTrade trade execution until SnapTrade confirms Robinhood write/order access is supported.

### Phase 19 — Coinbase Advanced Trade (Planned)

Crypto execution via Coinbase Advanced Trade API (official, API key auth). Same per-user encrypted-credential pattern as Alpaca. Implements `BrokerageProvider` interface — zero application layer changes.

### Phase 20 — SnapTrade Trade Execution via Robinhood (Conditional)

Only if Phase 18 SnapTrade integration confirms Robinhood write access. Route crypto/equity allocations to Robinhood via SnapTrade when user has it connected. Falls back to Alpaca if not available.

---

## Business Notes

- InvestIQ is free access for friends/users — not paid product initially
- LLC needed before signing Alpaca Broker API or SnapTrade commercial agreements
- Claude making recommendations vs executing trades = compliance distinction
- One LLC can cover InvestIQ + other business ventures
- See PERSONAL.md (not committed) for LLC formation and account migration details

---

## Backlog

Features defined but not yet scheduled. Reviewed after each phase — promoted when the time is right.

| Item | Notes |
|------|-------|
| Macro indicators (Fed rate, inflation) | News context covers this for now |
| Earnings calendar | Adds complexity, marginal value at current scale |
| Per-user scheduler interval | Complete — Phase 14 |
| Multiple auto-invest configs per user | Deferred |
| Redis for Plaid balance cache | In-memory is fine until scale demands it |
| Finnhub news + sentiment scores | Revisit when individual stock recommendations make per-ticker sentiment worth a new dependency; structured sentiment ("2 bearish") is stronger signal than Claude inferring from text, but not worth the extra key at current ETF-only scope |
| Refactor: single app shell | Complete — Phase 15b |
| Household/family accounts | Phase 13+ |
| Portfolio P&L dashboard | Gains per stock, total return — Phase 11 |
| Tax optimization | Tax-loss harvesting, asset location strategy |
| Rebalancing alerts | — |
| SMS notifications via Twilio | `Phone` field already stored on UserProfile and populated into `NotificationTarget`. Needs: `infrastructure/notifications/twilio.go` implementing `NotificationProvider`, factory wired on `NOTIFICATION_PROVIDER=twilio` with `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN`, `TWILIO_FROM` env vars. Consider a multi-channel provider that wraps both Resend + Twilio so email and SMS fire together. |
| Atlas IP allowlist | Replace 0.0.0.0/0 with Railway static IP when on Pro plan |
| LLC formation | Required before signing Alpaca Broker API or SnapTrade commercial agreements |

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
| Onion arch + DDD from Phase 1 | Decisions made in Phase 1 make Phase 5 easy or painful |
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
| AutoInvestConfig as own collection | Decoupled from UserProfile; scales to multiple configs per user in Phase 9 |
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
| Phase 8 ordered 8a → 8b → 8c | 8a: zero risk, wires existing data; 8b: Polygon news, zero new deps; 8c: new Plaid scope, most complex — keep isolated |
| Cash context as opt-in stored preference (Phase 8ca redesign) | Per-session directives ("Factor this into allocation") caused Claude to override stated risk tolerance regardless of user intent. Fix: gate spending data behind a stored UserProfile preference; when shown, instruction is "background context only — do not override risk tolerance or amount." Removes the cognitive dissonance between user input and AI output. |
| Per-user Alpaca credentials over env-var keys (Phase 8d) | Single shared env-var key means every user trades the same account — always wrong for financial data or trade execution. Per-user AES-256-GCM encrypted credentials in MongoDB, same pattern as Plaid access tokens. |
| `BrokerageProviderFactory` interface to avoid arch violation | Spec's approach (services decrypt keys + construct Alpaca client directly) would require application layer importing infrastructure packages. Factory interface in `domain/ports/` inverts the dependency — services never touch encryption or Alpaca constructors. |
| `ErrBrokerageNotConnected` sentinel error | Allows callers to distinguish "not connected" (user action needed, non-fatal for recommendations) from real errors. RecommendationService skips positions; InvestmentService returns 400; scheduler skips user silently. |
| distroless runner image | `gcr.io/distroless/static-debian12` has no shell, no libc, no package manager — smaller attack surface and smaller image than alpine. Requires static binary (`CGO_ENABLED=0`). |
| `/health` on top-level mux, not inside `UserIdentity` | `UserIdentity` blocks all non-`/auth/` routes. A health endpoint inside the mux would require a bearer token, breaking Docker/Railway healthchecks. Top-level mux registers `/health` first; everything else falls through to the protected mux. |
| Per-allocation brokerage override over global override | Global `brokerage_override_id` was the initial Phase 12b design. Per-allocation `per_allocation_brokerage map[ticker→connID]` is strictly more expressive — you can still route everything to one account (just set all tickers to the same ID) but you can also split individual allocations. Stamped on each `TradeReceipt` so MongoDB has a full audit trail of which account executed each trade. |
| 529 fallback to cached recommendation | Anthropic occasionally returns HTTP 529 (overloaded). Hard-failing the recommendation with a 500 is a bad UX for a daily-use app. The `decisions` collection already has everything needed — pull the last decision, rescale amounts to today's budget, set `FromCache: true`. If no prior decision exists the original error still propagates. No new storage, no cache layer needed. |
| P&L data from Alpaca not MongoDB | Alpaca already computes avg_entry_price, cost_basis, unrealized_pl per position. Computing this from MongoDB trade receipts would require reconstructing position state across all historical orders (buys/sells, partial fills, splits). Alpaca's live data is simpler and more accurate. |
| SVG chart, no charting library | Dependency principle: no charting library added. A plain SVG polyline with filled area is sufficient for a sparkline-style chart. Recharts/Chart.js would add ~100KB+ for a feature that needs one line and a few labels. |

---

## Known debt

| Shortcut | Future fix |
|----------|-----------|
| Polygon market data is previous-day close | `/prev` endpoint always gives yesterday's prices — Claude recommends based on stale data if the market moves overnight. Acceptable for personal/dev use; becomes misleading in volatile sessions. Real-time quotes (e.g. Finnhub) fix this but add a new dependency — deferred until this meaningfully hurts recommendation quality |
| Paper trading only | Swap ALPACA_BASE_URL + keys for live |
| Log provider for notifications | Real push (FCM or APNs) |
| Per-user interval not supported | Wishlist — user sets own interval from settings |
| Market holiday awareness | Complete — Phase 15a |
| Plaid balance cache is in-memory | Cache resets on restart; Redis for persistence at scale (Wishlist) |
| Encrypted Mongo for Plaid tokens | Vault / AWS Secrets Manager — Phase 9 deployment |
| One config per user only | Wishlist — multiple schedules with different risk levels |
| Receipt shows PENDING NEW | Fixed in Phase 7 — polls until terminal status; per-order max-attempts and after-hours stop added later |
| `[8a-debug]` log lines in `recommendation_service.go` | Remove after Phase 8a testing is verified; grep `[8a-debug]` |
| `[8b-debug]` log lines in `recommendation_service.go` | Remove after Phase 8b testing is verified; grep `[8b-debug]` |
| `[8c-debug]` log lines in `recommendation_service.go` | Remove after Phase 8c testing is verified; grep `[8c-debug]` |
| `[8ca-debug]` log lines in `recommendation_service.go` | Remove after Phase 8ca testing is verified; grep `[8ca-debug]` |
| `CashContext.UserOverride` field is dead | Field exists in the model but is never set; was part of the original Phase 8ca design before the redesign removed per-session override. Remove when cleaning up. |
| Prompt tests are white-box string assertions | `buildUserMessage` is package-private; tests in same package. Future: LLM-level assertion tests (does Claude actually respect the 40% rule?), fuzz tests on allocation sum, regression snapshot tests |
| Alpaca still paper trading only | Per-user credentials are wired — switching to live is a UI change (user selects "Live trading" account type) + using real Alpaca keys. No code change needed. |
