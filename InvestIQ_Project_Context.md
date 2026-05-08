# InvestIQ — Project Context & Master Reference

> Load this into your Claude Project so every new conversation starts with full context.
> Last updated: 2026-05-08 (Phase 6b complete, Phase 7 current)

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
  market/            ← stock price APIs
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

### Phase 7 — Current

**Goal:** Activity dashboard showing what the user has done through InvestIQ. No profit/loss — activity only.

**Dashboard:**
- Total decisions made + total dollars invested — both filtered by selected time range
- Time range filter: three numeric inputs (hours / days / months) — hours accepts decimals (e.g. 0.5 = 30 min), useful for local scheduler testing; user fills any combination, app combines into a date range; includes a reset button that defaults back to 30 days
- Investment timeline: list of decisions within selected period showing date + amount invested
- All data aggregated from `decisions` collection

**Receipt screen:**
- Poll Alpaca for final fill status instead of showing PENDING NEW

### Phase 8 — Planned
- `NewsProvider` interface in `domain/ports/`
- Polygon news endpoint — fetch top headlines daily
- Inject news context into Claude prompt alongside market snapshot

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

---

## Known debt

| Shortcut | Future fix |
|----------|-----------|
| Paper trading only | Swap ALPACA_BASE_URL + keys for live |
| Log provider for notifications | Real push (FCM or APNs) |
| Per-user interval not supported | Phase 9 — user sets own interval from settings |
| Market holiday awareness | Add NYSE calendar check before scheduler executes |
| Plaid balance cache is in-memory | Cache resets on restart; Redis for persistence at scale |
| Encrypted Mongo for Plaid tokens | Vault / AWS Secrets Manager before any real user beyond developer |
| No transaction history in prompt | Add Plaid Transactions scope in Phase 8 |
| One config per user only | Phase 9 — multiple schedules with different risk levels |
| Receipt shows PENDING NEW | Poll Alpaca for final fill status |
