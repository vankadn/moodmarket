# InvestIQ — Project Context & Master Reference

> Load this into your Claude Project so every new conversation starts with full context.
> Last updated: 2026-05-08 (Phase 6 complete + post-phase fixes, Phase 7 current)

---

## Who I am

- Background: React + Go developer (professional experience)
- AI experience: New to Claude and AI development — learning by building
- Goal: Build a real product while earning the Anthropic CPN certification simultaneously
- Dev environment: MacBook Pro (Intel), VS Code + Claude Code extension, terminal

---

## The two goals running in parallel

### Goal 1 — CPN Certification
Complete the Anthropic Claude Partner Network certification via Anthropic Academy (anthropic.skilljar.com). Access was granted through a company CPN application. The certification is called **Claude Certified Architect, Foundations** — multiple choice exam, passing score 720/1000, free for CPN members.

**Course order:**
1. Claude 101 (~30 mins) — start here
2. Building with the Claude API (~8 hrs) — maps directly to what we are building
3. Introduction to Agent Skills
4. Introduction to Model Context Protocol (MCP)
5. Claude Code in Action

The strategy: every CPN module studied maps to a feature built in InvestIQ. Learning and building together.

### Goal 2 — InvestIQ app
A smart daily investment assistant. Described in full below.

---

## What InvestIQ is

A personal financial operating system. Not a mood-based gimmick — a real tool that knows the user's complete financial life and makes intelligent daily investment decisions based on actual data.

**The core loop:**
1. User sets up full financial profile once — never asked again
2. Every day: open app, optionally add extra money on top of $100 base, tap one button
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

infrastructure/      ← implementations of domain ports
  advisor/           ← AI provider implementations
  db/                ← MongoDB
  auth/              ← Auth providers (DEV + Clerk)
  market/            ← stock price APIs (Phase 3)
  banking/           ← Plaid (Phase 5)
  brokerage/         ← Alpaca (Phase 4)
  secrets/           ← Vault / secrets manager (pre go-live)

api/                 ← outermost: HTTP handlers, middleware
  handlers/
  middleware/
```

Dependency rule: outer layers depend on inner. Never the reverse. Domain layer never imports infrastructure.

### Provider Abstraction Pattern
Every external dependency is hidden behind an interface defined in `domain/ports/`. Swapping any provider (AI model, database, auth, brokerage, banking) requires one new file + one config change. Nothing else changes.

Key interfaces:
- `InvestmentAdvisor` — AI recommendation engine (Claude today, Deepseek tomorrow)
- `ProfileRepository` — user profile persistence (MongoDB today)
- `AuthProvider` — identity (DevAuth for local dev, Clerk in production ✓)
- `IdentityProvider` — userId in request context
- `MarketDataProvider` — live prices (Phase 3 ✓)
- `DecisionRepository` — investment decision persistence (Phase 3 ✓)
- `BrokerageProvider` — trade execution (Phase 4 ✓, Alpaca paper trading)
- `FinancialDataProvider` — bank + 401k data (Phase 5 ✓, Plaid production)
- `NotificationProvider` — push notifications (Phase 6, provider TBD)
- `SecretsProvider` — sensitive credential retrieval (pre go-live, Vault / AWS Secrets Manager)

### DEV_MODE pattern
`DEV_MODE=true` in `.env` auto-logs in as the hardcoded dev user. Zero login required during development. This logic lives in exactly ONE place — `infrastructure/auth/factory.go`. No DEV_MODE checks anywhere else in the codebase. Flip to `false` and real auth kicks in.

---

## Tech stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Backend | Go, standard library only | No Gin, Echo, or any HTTP framework |
| Frontend | React + TypeScript + Vite | No UI component libraries |
| Database | MongoDB | Behind ProfileRepository interface |
| AI Provider | Claude (current) | Behind InvestmentAdvisor interface |
| Auth | Clerk (email + Google SSO) | Behind AuthProvider interface; DEV_MODE=true still works locally |
| Market Data | Polygon.io (previous-day, free tier) | Behind MarketDataProvider interface; mock available for dev |
| Brokerage | Alpaca (paper trading) | Behind BrokerageProvider interface; mock available for dev |
| Banking | Plaid (production, 10 free connections) | Behind FinancialDataProvider interface ✓ |
| Secrets | Encrypted Mongo now, Vault pre go-live | Behind SecretsProvider interface |
| Scheduler | Go time.Ticker | Drives autonomous investment cycle, interval from env var |
| Notifications | TBD Phase 6 | Behind NotificationProvider interface |

---

## Domain language — use these terms consistently

| Term | Meaning |
|------|---------|
| `UserProfile` | Complete financial picture of the user |
| `InvestmentRequest` | Request to generate a daily allocation |
| `Allocation` | Single position recommendation (ticker, amount, %) |
| `Recommendation` | Full AI response: allocations + summary + risk level |
| `InvestmentDecision` | Persisted record: userId, timestamp, market snapshot seen, allocations returned |
| `MarketSnapshot` | Point-in-time market context: SPY/QQQ trend, sector ETF performance, sentiment |
| `RiskTolerance` | conservative / moderate / aggressive |
| `TimeHorizon` | under_1_year / one_to_five / five_to_ten / ten_plus |
| `InvestmentGoal` | wealth_building / retirement / emergency_fund / short_term |
| `ImmigrationStatus` | us_citizen / permanent_resident / work_visa / other |
| `UserIdentity` | Authenticated user: UserID, Email, Name |
| `InvestmentAdvisor` | Interface for any AI recommendation engine |
| `AuthProvider` | Interface for any authentication system |
| `DecisionRepository` | Interface for persisting investment decisions |
| `FinancialDataProvider` | Interface for bank/account data (Phase 5) |
| `BrokerageProvider` | Interface for trade execution (Phase 4 ✓) |
| `TradeOrder` | Value object: UserID, Ticker, Amount (dollar-based notional) |
| `TradeReceipt` | Value object: OrderID, Ticker, FilledAmount, FilledPrice, Status, Timestamp |
| `Position` | Brokerage holding: Ticker, Quantity, MarketValue |
| `BankAccount` | Plaid-sourced account: institution, type, balance |
| `BalanceSummary` | Aggregated view: total cash, total investments, net worth signal |
| `PlaidConnection` | Per-institution token record: institution name, access_token, item_id |
| `NotificationProvider` | Interface for push notifications (Phase 6) |
| `AutoInvestConfig` | Value object on UserProfile: Enabled bool + EnabledAt time.Time — consent record |
| `SchedulerRun` | Audit record for one autonomous cycle: RunID, StartedAt, CompletedAt, UsersProcessed, TotalInvested, Errors |

---

## User profile fields (full onboarding, collected once)

- `full_name` string
- `salary` number (annual)
- `monthly_savings` number
- `retirement_contribution_percent` number (0–100)
- `existing_portfolio_value` number
- `time_horizon` enum: under_1_year / one_to_five / five_to_ten / ten_plus
- `immigration_status` enum: us_citizen / permanent_resident / work_visa / other
- `risk_tolerance` enum: conservative / moderate / aggressive
- `investment_goal` enum: wealth_building / retirement / emergency_fund / short_term_savings
- `has_emergency_fund` boolean
- `auto_invest_enabled` boolean (Phase 6 — consent to autonomous trading)
- `auto_invest_enabled_at` timestamp (Phase 6 — legal record of when consent was given)

---

## MongoDB collections

| Collection | Purpose |
|-----------|---------|
| `users` | User profile + financial goals + Plaid connections array + auto-invest config |
| `decisions` | Every daily investment decision logged (userId, timestamp, market snapshot, allocations, trade receipts, Plaid snapshot) |
| `portfolio` | Simulated holdings tracker |
| `market_snapshots` | Daily price context (Phase 3) |
| `scheduler_runs` | One document per autonomous cycle run — runID, timestamp, users processed, total invested, errors (Phase 6) |

---

## Phase completion log

### Phase 1 — Complete
- Go backend with `/recommend` endpoint calling Claude API
- React + TypeScript frontend with Vite
- `InvestmentAdvisor` interface with Claude implementation
- Provider abstraction pattern established
- App renamed from MoodMarket (mood-based concept scrapped)
- CLAUDE.md created in repo root with DDD + onion architecture rules

### Phase 2 — Complete
- MongoDB connected, `.env` auto-loaded at startup
- Full `UserProfile` schema with 10 fields
- `POST /users/profile` and `GET /users/profile` endpoints
- `ProfileRepository` interface with MongoDB implementation
- `IdentityProvider` interface in `domain/ports/identity.go`
- `ContextIdentityProvider` middleware in `infrastructure/middleware/auth.go`
- All endpoints scoped to `userId` via context — no hardcoded user IDs anywhere
- Auth foundation: `AuthProvider` interface with `DevAuthProvider` and `ClerkAuthProvider` stub
- `DEV_MODE=true` auto-logins as `krishna_local` — logic only in `factory.go`
- Frontend login skeleton with "Dev login" button (renders only when `VITE_DEV_MODE=true`)
- Basic routing: unauthenticated → Login page, authenticated → Home

### Phase 3 — Complete
- `MarketDataProvider` interface in `domain/ports/`
- Polygon.io implementation using `/v2/aggs/ticker/{ticker}/prev` (free tier compatible)
- Sector ETF coverage: SPY, QQQ, XLE, XLF, XLV, XLI
- Market sentiment derived from SPY % change: bullish / neutral / bearish
- Daily in-memory cache on the Polygon provider — one API fetch per day, all `/recommend` calls hit cache
- Mock provider (`MARKET_PROVIDER=mock`) for local dev — zero API calls during development
- `DecisionRepository` interface in `domain/ports/decision_repository.go`
- MongoDB `DecisionRepository` implementation — every recommendation persisted with userId, timestamp, market snapshot, and allocations
- Claude advisor hardened: prompt caching + 3-attempt retry + exponential backoff (5s/10s) for 529 overload errors
- Retry logic split: parse errors get correction prompt, API errors get clean retry
- Claude prompt enriched with full market context + complete user profile

**Issues hit and resolved in Phase 3:**

| Issue | Fix |
|-------|-----|
| Polygon 403 on `/v2/snapshot` | Switched to individual `/prev` calls per ETF (free tier compatible) |
| Polygon 429 rate limiting | Daily cache absorbs it — only hits Polygon once per server start |
| Claude retry sending nonsense correction on 529 | Split retry paths: parse errors get correction, API errors get clean retry |
| Consistent Claude 529s | Prompt caching + 5s/10s backoff + 3 attempts |

**Known debt carried into Phase 4:**

| Shortcut | Future fix |
|----------|-----------|
| `/prev` = yesterday's data, not live intraday | Polygon paid plan with intraday endpoint |
| QQQ fetched twice | Reuse value already fetched in `fetch()` |
| 5 concurrent sector calls → 429s, partial sectors | Serialize or rate-limit sector calls |
| In-memory cache resets on server restart | Persist cache to MongoDB |
| Sector price always 0 | Read `c` (close) from the `/prev` response already in hand |

### Phase 4 — Complete

**Phase 4a — Clerk Auth**
- `ClerkAuthProvider` stub filled in — JWT verification via Clerk backend API (Go standard library only)
- `infrastructure/auth/factory.go` routes DEV_MODE=false → Clerk, DEV_MODE=true → DevAuth (no change)
- Frontend: `@clerk/clerk-react` installed, app wrapped in `<ClerkProvider>`
- Login supports email + password and Google SSO
- When `VITE_DEV_MODE=false`, Clerk `<SignIn>` component replaces dev login button
- All API requests attach Clerk session token as `Authorization: Bearer <token>`
- Domain layer untouched — handlers still just see `UserIdentity` from context

**Phase 4b — Alpaca Paper Trading**
- `BrokerageProvider` interface in `domain/ports/brokerage.go` — `PlaceMarketOrder`, `GetPositions`
- `TradeOrder` + `TradeReceipt` + `Position` value objects in `domain/models/trade.go`
- Alpaca implementation in `infrastructure/brokerage/alpaca.go` — notional (dollar-based) market orders via paper API
- Mock provider in `infrastructure/brokerage/mock.go` — zero API calls, hardcoded receipts
- `infrastructure/brokerage/factory.go` — routes via `BROKERAGE_PROVIDER` env var
- `application/services/investment_service.go` — orchestrates: allocations → orders → decision → persist
- `POST /invest` handler — accepts recommendation payload, returns receipts + decisionId
- `InvestmentDecision` updated to include `Receipts []TradeReceipt`
- MongoDB decision repository updated with receipts field + conversion helpers
- Frontend: `ConfirmScreen.tsx` — allocations table (ticker, amount, %), risk level badge, total, confirm/cancel
- Frontend: `ReceiptScreen.tsx` — per-order receipt display (order ID, filled price, status)
- `Home.tsx` state machine: idle → confirming → investing → receipt

**Full invest loop now works:**
```
User taps "Invest today"
→ GET /recommend → Claude returns allocations
→ ConfirmScreen: allocations + risk level + total
→ User taps "Confirm & Invest"
→ POST /invest → Alpaca places paper orders → MongoDB saves full decision
→ ReceiptScreen: order IDs, filled prices, status per ticker
```

**Known debt carried into Phase 5:**

| Shortcut | Future fix |
|----------|-----------|
| Paper trading only | Swap ALPACA_BASE_URL + keys for live trading |
| Auth tied to Clerk free tier | Monitor MAU — paid plan at 50k users |
| No portfolio position tracking in UI | Phase 6 history view |
| Partial order failures logged but not surfaced in UI | Show partial failure state in ReceiptScreen |
| Receipt screen shows order status at placement time (PENDING NEW) | Poll Alpaca for final fill status and update ReceiptScreen |

### Phase 5 — Complete
- `FinancialDataProvider` interface in `domain/ports/`
- Plaid sandbox integration: connect real bank accounts + 401k via Plaid Link popup
- Per-user `plaid_connections` array stored on user document in MongoDB (institution, access_token, item_id)
- Auto-pull account balances into investment context on every `/recommend` call
- Claude prompt enriched with real cash position, account balances, connected institution summary
- Token revocation endpoint: `DELETE /plaid/accounts/:item_id` — calls Plaid revoke API + removes from Mongo
- `decisions` collection updated to snapshot Plaid balance data at time of each decision

**Plaid token flow:**
```
Frontend calls POST /plaid/link-token → backend creates Plaid link token
→ Plaid Link popup opens in browser
→ User connects bank → Plaid returns public_token to frontend
→ Frontend calls POST /plaid/exchange → backend exchanges for access_token
→ access_token stored in MongoDB on user document (encrypted at field level)
→ All future /recommend calls pull live balances silently using stored token
```

### Phase 5 — Complete

- `FinancialDataProvider` interface in `domain/ports/financial_data_provider.go`
- `PlaidConnection`, `BankAccount`, `BalanceSummary` value objects in `domain/models/banking.go`
- Plaid REST client in `infrastructure/banking/plaid.go` — net/http only, no Plaid SDK
- Mock provider in `infrastructure/banking/mock.go` — `FINANCIAL_DATA_PROVIDER=mock` for dev
- `ProfileRepository` extended: `SavePlaidConnection` ($push), `GetPlaidConnections` (decrypt on read), `RemovePlaidConnection` ($pull)
- AES-256-GCM encryption in `infrastructure/db/encryption.go` — key from `PLAID_TOKEN_ENCRYPTION_KEY` env var, loaded once via `sync.Once`
- `GET /users/profile` returns `connected_accounts` (institution + item_id only — access token never exposed to frontend)
- `RecommendationService` fetches live Plaid balances before every recommendation — fetch failure is non-fatal (logs and continues)
- Claude prompt enriched with: total cash, total investments, institution names, account count, data pull timestamp
- `POST /plaid/link-token`, `POST /plaid/exchange`, `DELETE /plaid/accounts/{item_id}` handlers
- Frontend: `usePlaidSetup` hook, `PlaidLinkButton` (react-plaid-link), `ConnectedAccounts` list with disconnect
- Frontend: `Profile.tsx` — connected account management + add new account
- Plaid production approved — free trial, 10 real connections, real BofA account connected and tested
- `PLAID_ENV=production` in `.env`

**Known debt carried into Phase 6:**

| Shortcut | Future fix |
|----------|-----------|
| Encrypted Mongo for tokens | Vault / AWS Secrets Manager before any real user beyond developer |
| No transaction history in prompt | Add Plaid Transactions scope to enrich spending context in Phase 8 |
| Balance fetch on every `/recommend` | Cache with short TTL (5 min) to reduce Plaid API calls at scale |

### Phase 6 — Complete

**Goal:** Fully autonomous investment agent. The app runs the investment cycle on a configurable interval without any user interaction. Users opt in explicitly. Every run is fully logged.

**Scheduler design:**
- Go `time.Ticker` in `application/scheduler/auto_invest_scheduler.go`
- Interval driven entirely by env var — no hardcoded times anywhere
- Uses Go standard library `time.ParseDuration` — any valid duration string works with zero code changes
- Started as a goroutine in `main.go` alongside the HTTP server

**Env vars:**
```
AUTO_INVEST_INTERVAL=24h       ← production default
AUTO_INVEST_INTERVAL=1m        ← dev/testing (fires every minute)
AUTO_INVEST_INTERVAL=5m        ← dev with real APIs (reduce rate limit risk)
```

Valid examples: `1m`, `5m`, `30m`, `3h`, `12h`, `24h` — anything `time.ParseDuration` accepts. No code change needed for any value.

**Scheduler flow on each tick:**
1. Query MongoDB for all users where `auto_invest_enabled: true`
2. For each user, run the full investment pipeline concurrently (goroutine per user)
3. Fetch Plaid balances → fetch market snapshot → call Claude → place Alpaca orders → log decision
4. Send push notification with summary
5. Log every step — timestamp, userId, what was fetched, what was invested, any errors

**Failure handling:**
- Plaid fetch fails → log error, skip user for this cycle, do not invest
- Claude call fails → retry 3 times (existing retry logic), then log and skip
- Alpaca order fails → log partial failure, continue with other tickers, notify user of partial execution
- Full cycle failure → log with full error context, send failure notification to user

**New domain models:**
- `AutoInvestConfig` on `UserProfile`: `{ Enabled bool, EnabledAt time.Time }` — consent record
- `SchedulerRun` — audit record: `{ RunID, StartedAt, CompletedAt, UsersProcessed, TotalInvested, Errors []string }`

**New files:**
- `application/scheduler/auto_invest_scheduler.go` — ticker loop, user fan-out
- `application/scheduler/auto_invest_runner.go` — single-user investment pipeline (reuses existing services)
- `infrastructure/db/scheduler_repository.go` — persists `SchedulerRun` records to `scheduler_runs` collection
- `domain/ports/notification_provider.go` — `NotificationProvider` interface: `SendInvestmentSummary`, `SendInvestmentFailure`
- `infrastructure/notifications/log_provider.go` — dev implementation: logs to stdout instead of sending real push
- `infrastructure/notifications/factory.go` — routes via `NOTIFICATION_PROVIDER` env var

**New MongoDB collection:**
- `scheduler_runs` — one document per cycle run: runID, timestamp, users processed, total invested, errors

**User profile changes:**
- Add `auto_invest_enabled bool` and `auto_invest_enabled_at time.Time` to `UserProfile` in `domain/models/`
- Add `PUT /users/auto-invest` handler — body: `{ enabled: bool }` — writes consent record with timestamp

**Frontend changes:**
- Auto-invest toggle on Home screen — clearly labeled, shows last run time when enabled
- When enabled: shows "Next run in X" countdown and "Last invested: $100 at 9:02am" summary
- When a run completes: decision appears in home screen without user doing anything
- Disable toggle immediately stops background access — calls `PUT /users/auto-invest` with `{ enabled: false }`

**CPN relevance:** This is the Agent Skills module in practice. An autonomous agent needs: explicit user consent (opt-in), defined scope (investment only, nothing else), audit trail (every run logged), graceful failure (never silently fail), and a way to stop (disable toggle). InvestIQ Phase 6 implements all five.

**What was built:**
- `domain/models/scheduler.go` — `SchedulerRun` audit model
- `domain/ports/notification_provider.go` — `NotificationProvider` interface
- `domain/ports/scheduler_repository.go` — `SchedulerRepository` interface
- `domain/models/user_profile.go` — `auto_invest_enabled` + `auto_invest_enabled_at` (consent record)
- `domain/ports/profile_repository.go` — `GetAutoInvestUsers` + `SetAutoInvest`
- `infrastructure/notifications/` — log provider (stdout); factory routes via `NOTIFICATION_PROVIDER`
- `infrastructure/db/mongo_scheduler_repository.go` — `scheduler_runs` collection
- `infrastructure/db/mongo_profile_repository.go` — auto_invest fields with omitempty (Upsert never resets flag)
- `application/scheduler/auto_invest_scheduler.go` — ticker loop, concurrent user fan-out, audit save
- `application/scheduler/auto_invest_runner.go` — single-user pipeline reusing existing services
- `api/handlers/auto_invest_handler.go` — `PUT /users/auto-invest`
- Frontend: animated toggle in Home.tsx, `updateAutoInvest()` in api.ts
- `AUTO_INVEST_INTERVAL=1m` in `.env` (change to `24h` for production)
- Verified: scheduler fires on interval, handles zero opted-in users cleanly

**Post-Phase 6 fixes:**
- CORS moved from per-handler `setCORSHeaders()` calls to a single `middleware.CORS` wrapper in `main.go` — covers all routes universally, no handler can miss it
- Git workflow rule added to CLAUDE.md: Claude writes code, you verify, you trigger commit/push

**Known debt carried into Phase 7:**

| Shortcut | Future fix |
|----------|-----------|
| Log provider for notifications | Real push notifications (FCM or APNs) |
| Per-user interval not supported | Phase 9 — user sets their own interval from profile settings |
| Market holiday awareness | Scheduler fires on holidays — add NYSE calendar check before executing |
| Concurrent user runs not rate-limited | Add semaphore to limit concurrent Plaid/Alpaca calls at scale |
| Plaid `BALANCE_LIMIT` 429 on rapid calls | Cache balance summary with 5-min TTL; free tier throttles `/accounts/balance/get` per item |

### Phase 6b — Auto-Invest Config (Planned)

**Goal:** Promote AutoInvestConfig from fields on UserProfile to a first-class domain model with its own collection and a dedicated settings screen.

**Why:** Auto-invest needs its own amount and risk tolerance (separate from profile defaults). Future phases will support multiple configs with different risk levels and schedules.

**Backend changes:**
- New domain model: `domain/models/auto_invest_config.go` — fields: ID, UserID, Enabled, Amount (float64), Risk (RiskTolerance), EnabledAt, UpdatedAt
- New port: `domain/ports/auto_invest_repository.go` — GetByUserID, Upsert, GetAllEnabled
- New infrastructure: `infrastructure/db/mongo_auto_invest_repository.go` — collection: `auto_invest_configs`, upsert by user_id
- Update scheduler: use `AutoInvestRepository.GetAllEnabled()` instead of querying users collection; pass config.Amount and config.Risk into runner
- New handler: `api/handlers/auto_invest_config_handler.go` — GET and PUT `/users/auto-invest/config`
- Remove `auto_invest_enabled` and `auto_invest_enabled_at` from UserProfile
- Remove `SetAutoInvest` / `GetAutoInvestUsers` from ProfileRepository
- Remove old `PUT /users/auto-invest` handler

**Frontend changes:**
- New page: `AutoInvestSettings.tsx` — toggle, dollar amount input, risk pill selector (3 buttons, not dropdown), save button
- `api.ts`: add `AutoInvestConfig` type, `getAutoInvestConfig()`, `saveAutoInvestConfig()`
- `Home.tsx`: auto-invest row navigates to settings instead of toggling directly; show "Enabled — $100/day" when active
- New route in `App.tsx`: `/auto-invest/settings`

**Design decision:** Home toggle is enable/disable only. All config lives in the settings screen. This pattern scales to multiple configs in Phase 9.

**Known debt to carry forward:**
- Only one config per user supported — Phase 9 adds multiple schedules with different risk levels
- Frequency is daily only — weekly/monthly scheduling is a Phase 9 addition

### Phase 7 — Current Phase
- Dashboard: total dollars invested via InvestIQ, number of decisions made
- Dashboard: allocation breakdown pie chart by ticker and sector (from decisions collection)
- Dashboard: investment timeline — when and how much per decision
- Receipt screen: poll Alpaca for final fill status instead of showing PENDING NEW

### Phase 8 — Planned
- `NewsProvider` interface in `domain/ports/`
- Polygon news endpoint implementation — fetch top relevant headlines daily
- Inject news context into Claude prompt alongside market snapshot
- Claude reasons across market data + user profile + current events together

---

## The long-term vision

A unified financial operating system. The app knows:
- Bank accounts (checking, savings, cash position) via Plaid
- 401k and retirement accounts (Fidelity, Vanguard, etc.)
- Brokerage accounts (what you already own)
- Daily spending patterns

When the user taps "Invest today", Claude receives a prompt like:
> "User has $4,200 in checking, $180,000 in 401k (60/40 allocation), $12,000 in brokerage mostly QQQ and AAPL, a $1,200 credit card bill due in 8 days, H1B visa, 10-year horizon, moderate risk. Invest $100 today."

That context is what makes recommendations genuinely intelligent — not mood, not guesses.

Eventually: fully autonomous. App runs at 9am, invests, sends a notification. User never opens it unless they want to review.

---

## Security & Compliance Decisions

### Plaid access token model
Plaid `access_token` values are permanent credentials — treat like passwords. A stolen token grants read access to all account balances, transaction history, account/routing numbers, and investment holdings for that institution. They cannot initiate transfers on their own, but the data exposure alone is severe.

**Per-user Mongo structure on the `users` document:**
```json
"plaid_connections": [
  { "institution": "Bank of America", "access_token": "access-sandbox-xxx", "item_id": "xxx" },
  { "institution": "Robinhood",        "access_token": "access-sandbox-yyy", "item_id": "yyy" }
]
```
One entry per connected institution. Users can connect multiple banks and brokerages.

**Current (sandbox/dev):** field-level encryption on `access_token` in MongoDB is acceptable.

**Before any real user connects a real account (non-negotiable):** move tokens to HashiCorp Vault or AWS Secrets Manager. Store only a vault reference key in Mongo (`plaid_vault_ref`). App calls Vault at request time to retrieve the live token. If the database is compromised, attacker gets useless reference keys — not live credentials.

Infrastructure hook is ready: `infrastructure/secrets/vault.go` behind a `SecretsProvider` interface in `domain/ports/`. Swapping encrypted Mongo for Vault is one new file + one config change.

### Regulatory obligations
- **GLBA** — must protect financial data and disclose data sharing practices to users
- **CCPA** — if any user is in California, they have rights to access, deletion, and opt-out of data sale
- **Plaid developer agreement** — legally binding on data handling standards; Plaid can and does terminate API access for violations
- **FTC enforcement** — primary body for fintech apps in the US; has pursued companies for plaintext credential storage, undisclosed data sharing, and retaining data beyond stated policy
- **SOC 2 Type II** — not a law but increasingly required by enterprise customers and app stores; certifies security controls are real and tested over time

### Principle of least privilege
Only request the Plaid scopes actually used by the feature. If the feature only needs account balances, do not request transaction history scope. Over-requesting scope is both a legal risk and a user trust problem. Plaid logs every scope request and every API call — regulators and Plaid's own trust team can audit these.

### Autonomous background access (Phase 6)
Background data access without the user being logged in is legitimate when all of the following are true:
1. User explicitly opted into auto-invest — `auto_invest_enabled: true` + consent timestamp stored in MongoDB
2. Privacy policy explicitly states "when auto-invest is enabled, we access your connected accounts daily at 9am to execute your investment"
3. Every background Plaid call is logged — timestamp, which accounts were read, what balances were seen, what decision was made
4. User can disable auto-invest at any time, with a clear explanation of what stops

The `decisions` collection records what Plaid returned alongside what Claude recommended — full audit trail per investment run. This protects both the user and InvestIQ if access is ever questioned.

### Token revocation
Users must be able to disconnect accounts. Disconnecting must call Plaid's token revocation API — not just delete the MongoDB record. A deleted Mongo record with a still-active Plaid token is a compliance violation. The token remains live at Plaid until explicitly revoked.

Endpoint: `DELETE /plaid/accounts/:item_id` → calls Plaid `/item/remove` → removes connection from MongoDB.

### Audit trail requirement
Every Plaid data pull — whether triggered by a user action or a background autonomous run — must be logged with: timestamp, userId, which accounts were read, what data was returned, and what downstream action (recommendation, investment) it informed. This is the evidence that access was scoped to its stated purpose.

---

## Key product decisions made

| Decision | Why |
|---------|-----|
| Scrapped mood-based concept (cold/warm/hot) | Mood is a gimmick. Financial state drives real decisions |
| App owns intelligence, AI is swappable tool | Don't be locked into Claude — could be Deepseek, fine-tuned model, anything |
| MongoDB first, interfaces from day one | Swapping to Postgres later should be one file + one env var |
| DEV_MODE for dev auth | Save time during development, never touch login flow while building features |
| Full profile collected upfront | App should never ask the same question twice |
| Immigration status in profile | Visa status materially affects investment strategy (H1B holders have different constraints) |
| Onion arch + DDD from Phase 1 | Decisions made in Phase 1 make Phase 5 easy or painful |
| CLAUDE.md in repo root | Claude Code reads it automatically — acts as senior engineer on the team every session |
| App fetches market data, not Claude | App owns the audit trail — decisions collection stores what Claude saw when it recommended |
| Polygon.io over Alpha Vantage | Alpha Vantage free tier: 25 req/day cap, unreliable after-hours responses. Polygon free tier: unlimited previous-day data |
| Daily cache on market provider | One Polygon API call per day regardless of how many `/recommend` calls are made |
| Mock provider for dev | Zero API calls during development — swap via MARKET_PROVIDER env var |
| Clerk for auth | 50k free MAU, dev instance unlocks all Pro features, clean React SDK, Go backend just verifies JWT |
| Alpaca for brokerage | Visa-friendly, paper trading requires no brokerage account, paper → live is a config change not a code change |
| Notional (dollar-based) orders | User thinks in dollars not shares — $60 into VTI is clearer than 0.27 shares |
| BROKERAGE_PROVIDER mock for dev | Test full invest loop without touching Alpaca API during development |
| Partial order failure tolerance | One bad ticker shouldn't block the whole investment — log and continue, return partial receipts |
| Plaid sandbox before production | Test full token flow with real institutions without real credentials |
| Encrypted Mongo for tokens now, Vault before go-live | access_tokens are live credentials — DB compromise must not expose them in production |
| Consent timestamp for auto-invest | Legal record of when user opted into background data access — required for autonomous Phase 6 |
| Log every background Plaid call | Audit trail protects both user and InvestIQ if access is ever questioned |
| Token revocation on disconnect | Deleting from Mongo without revoking at Plaid is a compliance violation — token stays live at Plaid until explicitly revoked |
| Principle of least privilege on Plaid scopes | Only request what the feature uses — over-requesting is a legal and trust risk |
| AUTO_INVEST_INTERVAL env var for scheduler | Human readable, configurable without code changes — 1m for dev, 24h for prod, 3h just works |
| time.ParseDuration for interval parsing | Any valid Go duration string works — no translation layer, no code change needed for new values |
| Log provider for notifications in dev | No push infrastructure needed during development — stdout logs confirm the pipeline works |
| Consent timestamp on auto-invest opt-in | Legal record of when user authorized background access and autonomous trading |
| Scheduler run audit collection | Every autonomous cycle logged — protects InvestIQ if a user questions an automatic trade |
| Failure = skip not crash | A user's Plaid/Claude/Alpaca failure should not stop the scheduler for other users |

---

## CLAUDE.md engineering rules summary

- Domain-Driven Design: business logic in domain layer only
- Onion Architecture: outer layers depend on inner, never reverse
- Interface-first: every third party behind a `domain/ports/` interface
- Go standard library only — no frameworks
- React + TypeScript + Vite — no UI libraries
- Explicit error handling with `fmt.Errorf("context: %w", err)`
- No abbreviations except ctx, err, db, req, res
- No god files — one primary responsibility per file
- DEV_MODE logic only in factory functions
- Sensitive credentials never stored in plaintext — field-level encryption minimum, Vault before go-live

---

## How to use this document

Load this into a Claude Project as a project file. Start every new conversation with:

> "Read the project context document first, then help me with [what you need]."

Or for VS Code Claude Code, reference it directly:

> "Read InvestIQ_Project_Context.md and CLAUDE.md, then [task]."

This document should be updated at the end of each phase as a record of what was built and why.
