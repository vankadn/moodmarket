# InvestIQ — Project Context & Master Reference

> Load this into your Claude Project so every new conversation starts with full context.
> Last updated: 2026-05-08 (Phase 8ca redesign complete)

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
  notifications/     ← log provider (dev), push (future)
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
- `BrokerageProvider` — trade execution (Alpaca or mock)
- `FinancialDataProvider` — bank + 401k data (Plaid or mock)
- `NotificationProvider` — push notifications (log provider now, push later)
- `SecretsProvider` — sensitive credential retrieval (pre go-live, Vault / AWS Secrets Manager)

### DEV_MODE pattern
`DEV_MODE=true` in `.env` auto-logs in as the hardcoded dev user. Zero login required during development. This logic lives in exactly ONE place — `infrastructure/auth/factory.go`. No DEV_MODE checks anywhere else.

### MOCK_ALL pattern
`MOCK_ALL=true` overrides all provider env vars to `mock` and sets `DEV_MODE=true` — zero external calls needed. Processed at startup in `main.go` before any factory runs. Factories are unaware of it.

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
| Notifications | Log provider (dev) | Behind NotificationProvider interface; logs to stdout |
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
| `Recommendation` | Full AI response: allocations + summary + risk level |
| `InvestmentDecision` | Persisted record: userId, timestamp, market snapshot, allocations, trade receipts |
| `MarketSnapshot` | Point-in-time market context: SPY/QQQ trend, sector ETF performance, sentiment |
| `RiskTolerance` | conservative / moderate / aggressive |
| `TimeHorizon` | under_1_year / one_to_five / five_to_ten / ten_plus |
| `InvestmentGoal` | wealth_building / retirement / emergency_fund / short_term_savings |
| `ImmigrationStatus` | us_citizen / permanent_resident / work_visa / other |
| `UserIdentity` | Authenticated user: UserID, Email, Name |
| `TradeOrder` | Value object: UserID, Ticker, Amount (dollar-based notional) |
| `TradeReceipt` | Value object: OrderID, Ticker, FilledAmount, FilledPrice, Status, Timestamp |
| `Position` | Brokerage holding: Ticker, Quantity, MarketValue |
| `BankAccount` | Plaid-sourced account: institution, type, balance |
| `BalanceSummary` | Aggregated view: total cash, total investments, net worth signal |
| `PlaidConnection` | Per-institution token record: institution name, access_token, item_id |
| `AutoInvestConfig` | First-class domain model (own collection): Enabled, Amount, Risk, EnabledAt, UpdatedAt |
| `SchedulerRun` | Audit record for one autonomous cycle: RunID, StartedAt, CompletedAt, UsersProcessed, TotalInvested, Errors |
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
- `immigration_status` enum
- `risk_tolerance` enum
- `investment_goal` enum
- `has_emergency_fund` boolean
- `include_cash_context` boolean — opt-in: share spending/runway data with Claude advisor (default false; no migration needed)

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
- Shows "polling for fill…" indicator while waiting

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

### Phase 9 — Planned

**Goal:** Deploy the app. Share a real URL with a friend. Real user, real account, real trades.

Full deployment plan covering:
- Backend: containerize Go server, deploy to cloud (Railway / Fly.io / Render TBD)
- Frontend: deploy React app (Vercel or same host TBD)
- MongoDB: Atlas (already used in dev, promote to production cluster)
- Secrets: move Plaid tokens from encrypted Mongo to Vault or AWS Secrets Manager
- Clerk: switch from dev instance to production instance
- Alpaca: evaluate paper → live switch for real user
- Environment config: production `.env` strategy, no secrets in repo
- Domain + HTTPS
- Smoke test checklist before sharing URL

### Phase 10 — TBD

Pick one item from Known Debt or Wishlist based on what matters most after Phase 9 real-user feedback.

---

## Backlog

Features defined but not yet scheduled. Reviewed after each phase — promoted when the time is right.

| Item | Notes |
|------|-------|
| Macro indicators (Fed rate, inflation) | News context covers this for now |
| Earnings calendar | Adds complexity, marginal value at current scale |
| Per-user scheduler interval | Get one real user working first (Phase 9) |
| Multiple auto-invest configs per user | Phase 9 first |
| Redis for Plaid balance cache | In-memory is fine until scale demands it |
| Finnhub news + sentiment scores | Revisit when individual stock recommendations make per-ticker sentiment worth a new dependency; structured sentiment ("2 bearish") is stronger signal than Claude inferring from text, but not worth the extra key at current ETF-only scope |

---

## The long-term vision

A unified financial operating system. The app knows:
- Bank accounts (checking, savings, cash position) via Plaid
- 401k and retirement accounts (Fidelity, Vanguard, etc.)
- Brokerage accounts (what you already own)
- Daily spending patterns

When the user taps "Invest today", Claude receives a prompt like:
> "User has $4,200 in checking, $180,000 in 401k (60/40 allocation), $12,000 in brokerage mostly QQQ and AAPL, a $1,200 credit card bill due in 8 days, H1B visa, 10-year horizon, moderate risk. Invest $100 today."

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
| Immigration status in profile | Visa status materially affects investment strategy |
| App fetches market data, not Claude | App owns the audit trail — decisions collection records what Claude saw |
| Polygon.io over Alpha Vantage | Alpha Vantage: 25 req/day cap. Polygon free tier: unlimited previous-day data |
| Daily cache on market provider | One Polygon call per day regardless of recommendation volume |
| Plaid product: transactions not auth | Auth requires separate Plaid approval; transactions covers balance reads |
| PLAID_CACHE_TTL (default 5m) | Sandbox rate limits are real during dev; production usage won't hit them at scale |
| Clerk for auth | 50k free MAU, dev instance unlocks Pro features, clean React SDK |
| Alpaca for brokerage | Visa-friendly, paper trading, paper → live is a config change not a code change |
| Notional (dollar-based) orders | Users think in dollars not shares |
| BROKERAGE_PROVIDER mock | Test full invest loop without touching Alpaca during dev |
| Partial order failure tolerance | One bad ticker shouldn't block the whole investment |
| Encrypted Mongo for tokens now, Vault before go-live | access_tokens are live credentials — DB compromise must not expose them |
| AutoInvestConfig as own collection | Decoupled from UserProfile; scales to multiple configs per user in Phase 9 |
| GetByUserID returns safe default | Frontend never handles not-found — safe defaults mean no extra error path |
| MOCK_ALL=true flag | Single env var for zero-external-calls dev setup — no need to set 5 provider vars |
| Mock advisor | Full end-to-end flow testable without Anthropic key or API calls |
| AUTO_INVEST_INTERVAL env var | Human-readable duration string; 1m for dev, 24h for prod, no code change |
| Log provider for notifications | No push infrastructure needed during development |
| Failure = skip not crash | One user's Plaid/Claude/Alpaca failure must not stop the scheduler for other users |
| CORS as outermost middleware | No handler can accidentally miss CORS — one place, universal coverage |
| Git workflow: write → verify → commit | No autonomous pushes — developer reviews before any commit |
| Polygon over Finnhub for news | Polygon already integrated — no new key, no new dependency. Claude infers sentiment from text. Finnhub backlogged for when individual stocks make per-ticker sentiment worth the extra dependency |
| Dependency principle: exhaust before adding | Before adding a new API/service, check if an existing one can do the job. New keys = new cost, new failure surface, new rotation burden |
| Phase 8 ordered 8a → 8b → 8c | 8a: zero risk, wires existing data; 8b: Polygon news, zero new deps; 8c: new Plaid scope, most complex — keep isolated |
| Cash context as opt-in stored preference (Phase 8ca redesign) | Per-session directives ("Factor this into allocation") caused Claude to override stated risk tolerance regardless of user intent. Fix: gate spending data behind a stored UserProfile preference; when shown, instruction is "background context only — do not override risk tolerance or amount." Removes the cognitive dissonance between user input and AI output. |

---

## Known debt

| Shortcut | Future fix |
|----------|-----------|
| Polygon market data is previous-day close | `/prev` endpoint always gives yesterday's prices — Claude recommends based on stale data if the market moves overnight. Acceptable for personal/dev use; becomes misleading in volatile sessions. Real-time quotes (e.g. Finnhub) fix this but add a new dependency — deferred until this meaningfully hurts recommendation quality |
| Paper trading only | Swap ALPACA_BASE_URL + keys for live |
| Log provider for notifications | Real push (FCM or APNs) |
| Per-user interval not supported | Wishlist — user sets own interval from settings |
| Market holiday awareness | Add NYSE calendar check before scheduler executes |
| Plaid balance cache is in-memory | Cache resets on restart; Redis for persistence at scale (Wishlist) |
| Encrypted Mongo for Plaid tokens | Vault / AWS Secrets Manager — Phase 9 deployment |
| One config per user only | Wishlist — multiple schedules with different risk levels |
| Receipt shows PENDING NEW | Fixed in Phase 7 — polls until terminal status |
| `[8a-debug]` log lines in `recommendation_service.go` | Remove after Phase 8a testing is verified; grep `[8a-debug]` |
| `[8b-debug]` log lines in `recommendation_service.go` | Remove after Phase 8b testing is verified; grep `[8b-debug]` |
| `[8c-debug]` log lines in `recommendation_service.go` | Remove after Phase 8c testing is verified; grep `[8c-debug]` |
| `[8ca-debug]` log lines in `recommendation_service.go` | Remove after Phase 8ca testing is verified; grep `[8ca-debug]` |
| `CashContext.UserOverride` field is dead | Field exists in the model but is never set; was part of the original Phase 8ca design before the redesign removed per-session override. Remove when cleaning up. |
| Prompt tests are white-box string assertions | `buildUserMessage` is package-private; tests in same package. Future: LLM-level assertion tests (does Claude actually respect the 40% rule?), fuzz tests on allocation sum, regression snapshot tests |
