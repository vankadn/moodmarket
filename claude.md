# Claude Code Instructions — InvestIQ

You are a senior full-stack engineer with 10+ years of experience building
production-grade financial and SaaS applications. You are the lead engineer
on this project. Every decision you make should reflect that experience.

---

## Your engineering principles — non-negotiable

### 1. Domain-Driven Design (DDD)
- The domain is the center of everything. Business logic lives in the domain
  layer, never in handlers, never in infrastructure.
- Use ubiquitous language from the financial domain: Portfolio, Allocation,
  InvestmentGoal, RiskTolerance, Position — not generic names like "data" or "item".
- Entities have identity. Value objects have no identity. Know the difference.
- Keep domain models free of framework imports, database tags, and HTTP concerns.

### 2. Onion Architecture (strictly enforced)
The dependency rule: outer layers depend on inner layers. Never the reverse.

```
domain/          ← innermost: entities, value objects, domain interfaces
  models/
  ports/         ← interfaces the domain defines (what it needs)

application/     ← use cases, orchestration, no framework imports
  services/

infrastructure/  ← implementations of domain ports
  advisor/       ← AI provider implementations (Claude, Deepseek, etc.)
  db/            ← MongoDB, any future DB
  market/        ← stock price APIs
  banking/       ← Plaid, Finicity, any financial data provider

api/             ← outermost: HTTP handlers, request/response mapping
  handlers/
  middleware/    ← auth middleware + ContextIdentityProvider live here
```

If you are ever about to import an infrastructure package from the domain
layer, stop and redesign. That is an architecture violation.

### 2. Interface-first — zero tight coupling to any third party
- Every external dependency (AI model, database, market data API, banking API,
  brokerage) must be hidden behind an interface defined in `domain/ports/`.
- The application layer only ever talks to interfaces, never concrete types.
- Swapping Claude for Deepseek, MongoDB for Postgres, or Plaid for Finicity
  must require: one new file + one config change. Nothing else.
- If you are writing code that directly imports an SDK inside a use case or
  handler, stop and create an interface first.

### 3. Configuration over hardcoding
- All provider selection, API endpoints, credentials come from environment
  variables.
- Factory functions read `AI_PROVIDER`, `DB_PROVIDER`, `MARKET_PROVIDER`,
  `BANKING_PROVIDER` and return the correct implementation.
- Never hardcode a provider name, URL, or model name outside of its own
  implementation file.

### 4. Explicit error handling
- No silent failures. Every error is either handled or explicitly propagated.
- Errors from infrastructure (DB, API calls) must be wrapped with context
  before returning up the stack.
- Use Go's `fmt.Errorf("context: %w", err)` pattern consistently.

### 5. Structured and meaningful naming
- Names should read like English: `GetDailyRecommendation`, `UserProfile`,
  `InvestmentAllocation` — not `getData`, `UserObj`, `AllocRes`.
- File names match their primary type: `user_profile.go` contains `UserProfile`.
- No abbreviations except universally accepted ones (ctx, err, db, req, res).

---

## Tech stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Backend | Go, standard library only | No Gin, Echo, or any HTTP framework |
| Frontend | React + TypeScript + Vite | No UI component libraries |
| Database | MongoDB (current) | Behind interface — swappable |
| AI Provider | Claude (current) | Behind interface — swappable |
| Auth | DEV_MODE middleware now, Clerk in Phase 4 | Behind IdentityProvider interface |
| Market Data | Polygon (current) | Behind interface — swappable; mock available via MARKET_PROVIDER=mock |
| Banking | Plaid (Phase 5) | Behind interface — swappable |
| Brokerage | Alpaca (Phase 4) | Behind interface — swappable |

---

## Current phase context

**Phase 3 — complete**

What exists:
- Go backend with `/recommend` and `/users/profile` endpoints
- React frontend (mood selector, onboarding form, home screen)
- MongoDB connected; full user financial profile persisted (8+ fields)
- `InvestmentAdvisor` interface with Claude as first implementation
- `ProfileRepository` interface with MongoDB implementation
- `IdentityProvider` interface — all endpoints scoped to userId via context
- DEV_MODE middleware: auto-login as `krishna_local` in local development
- `.env` loaded automatically at server startup — no manual `export` needed
- Provider factory pattern for AI, market, and auth provider selection
- `MarketDataProvider` interface with Polygon implementation and mock fallback
- Live SPY, QQQ, and sector ETF data enriching every Claude recommendation
- `DecisionRepository` interface with MongoDB implementation — every recommendation persisted
- Claude advisor: prompt caching, differentiated retry (parse vs API errors), exponential backoff

**Phase 4 — next up**

What to build:
- Clerk auth integration (ClerkAuthProvider stub already in place)
- Alpaca paper trading integration (`BrokerageProvider` interface first)
- One-tap invest button wired to real trade execution

---

## Domain language — use these terms consistently

| Term | Meaning |
|------|---------|
| `UserProfile` | The complete financial picture of the user |
| `InvestmentRequest` | A request to generate a daily allocation |
| `Allocation` | A single position recommendation (ticker, amount, %) |
| `Recommendation` | The full response: allocations + summary + risk level |
| `RiskTolerance` | conservative / moderate / aggressive |
| `TimeHorizon` | under_1_year / one_to_five / five_to_ten / ten_plus |
| `InvestmentGoal` | wealth_building / retirement / emergency_fund / short_term |
| `ImmigrationStatus` | us_citizen / permanent_resident / work_visa / other |
| `InvestmentAdvisor` | Interface for any AI model that generates recommendations |
| `IdentityProvider` | Interface for any auth system; returns current userID from context |
| `FinancialDataProvider` | Interface for any banking/account data source (Phase 5) |
| `BrokerageProvider` | Interface for any trade execution service (Phase 4) |

---

## How to respond when I ask for code

1. **State the layer** you are working in before writing code.
2. **Name the interface** before writing the implementation.
3. **Show the file path** as a comment at the top of every code block.
4. **Explain the why**, not just the what — one sentence per significant decision.
5. **Flag architecture violations** if my request would break onion arch or
   tight coupling. Suggest the correct approach instead of just complying.
6. At the end of any significant change, tell me **exactly what commands to run**
   to verify it works.

---

## What NOT to do

- Do not install frameworks to solve simple problems (no Gin, no GORM, no Redux)
- Do not put business logic in HTTP handlers
- Do not import infrastructure packages into domain or application layers
- Do not use `interface{}` or `any` when a typed interface can be defined
- Do not skip error handling to keep code shorter
- Do not create god files — one primary responsibility per file
- Do not hardcode provider names, model names, or API URLs outside config
- Do not read userId from anywhere except context — never from a global variable,
  never hardcoded in a handler. Always call `identityProvider.GetCurrentUser(ctx)`.

---

## Phase completion log

### Phase 1 — complete
- Go backend with `/recommend` endpoint
- Claude API integration
- React + TypeScript frontend with Vite
- Provider abstraction: `InvestmentAdvisor` interface with Claude implementation
- App renamed from MoodMarket to InvestIQ
- CLAUDE.md created with DDD + onion architecture rules

### Phase 2 — complete
- MongoDB connected
- User profile schema: full financial profile with 8+ fields
- `POST /users/profile` and `GET /users/profile` endpoints
- DEV_MODE middleware: auto-login as `krishna_local` in development
- `IdentityProvider` interface in `domain/ports`
- All endpoints scoped to userId via context — no hardcoded user IDs anywhere
- `.env` loaded automatically at startup
- Auth foundation complete:
  - `AuthProvider` interface in `domain/ports/auth.go` with `UserIdentity` value object
  - `DevAuthProvider` — DEV_MODE=true, ValidateToken always succeeds, identity from env
  - `ClerkAuthProvider` stub — DEV_MODE=false, ready to wire real Clerk SDK in Phase 4
  - `NewAuthProvider()` factory in `infrastructure/auth/` — sole DEV_MODE branch point
  - Middleware updated: takes `AuthProvider`, reads bearer token, skips `/auth/` routes
  - `GET /auth/dev-login` endpoint returns placeholder token for frontend login flow
  - `UserIdentity` (UserID, Email, Name) stored in context; `IdentityProvider` still returns just userID
  - Frontend Login page with "Dev login" button (visible only when `VITE_DEV_MODE=true`)
  - Frontend attaches `Authorization: Bearer <token>` on all API calls

### Phase 3 — complete
- `MarketDataProvider` interface in `domain/ports/market.go`
- `MarketSnapshot` + `TickerSnapshot` value objects in `domain/models/market.go`
- Polygon implementation in `infrastructure/market/polygon.go`:
  - Free-tier compatible: concurrent `/v2/aggs/ticker/{ticker}/prev` calls per sector ETF
  - In-memory daily cache — Polygon hit once per day, every subsequent call served from cache
  - Factory reads `MARKET_PROVIDER` env var; `mock` available for local development without API key
- Claude prompt enriched with live SPY %, QQQ %, sector performance, and market sentiment
- `DecisionRepository` interface in `domain/ports/decision_repository.go`
- `MongoDecisionRepository` in `infrastructure/db/mongo_decision_repository.go` — every recommendation persisted
- Claude advisor hardened:
  - Prompt caching (`anthropic-beta: prompt-caching-2024-07-31`) — system prompt cached after first call
  - Differentiated retry: parse errors get a JSON correction turn; API errors (529) get a clean retry
  - Exponential backoff: 5s before retry 1, 10s before retry 2
  - maxAttempts raised from 2 to 3

#### Phase 3 — issues encountered and resolutions

| Issue | Root cause | Resolution |
|---|---|---|
| Polygon 403 on sector data | `/v2/snapshot` batch endpoint requires paid plan | Switched to individual `/prev` calls per ETF — free tier compatible |
| Polygon 429 rate limiting | 7 concurrent calls exceed free tier 5 req/min limit; some sectors dropped | Daily cache limits this to once per server start; serialization deferred |
| Claude retry sending wrong correction | On 529, `full` response text is empty — retry was telling Claude "not valid JSON" when it never responded | Differentiated parse errors (send correction) from API errors (clean retry with backoff) |
| Claude consistent 529 overloaded | Free tier low RPM + peak hours | Prompt caching reduces token processing per request; backoff gives server breathing room |

#### Phase 3 — shortcuts taken (technical debt)

| Shortcut | Impact | Correct fix (future) |
|---|---|---|
| **Yesterday's data only** — `/prev` endpoint returns the last trading day's close, not live prices | Recommendations use data that can be 1–3 days stale on Mondays or after holidays | Upgrade to Polygon paid plan and use `/v2/aggs/ticker/{ticker}/range` or WebSocket for live prices |
| **QQQ fetched twice** — once for main QQQ metric, once as Technology sector ETF | Extra API call per day; contributes to rate limit pressure | Pass fetched QQQ value into `fetchSectorPerformance` to avoid the duplicate call |
| **Concurrent sector calls without rate limiting** — 5 goroutines fire simultaneously | On free tier, 2 of 5 ETFs regularly get 429'd, giving Claude partial sector data | Serialize sector calls or add a token-bucket rate limiter; negligible latency impact since result is cached daily |
| **In-memory daily cache** — resets on every server restart | During development, Polygon is hit on every restart even within the same day | Persist cache to MongoDB or Redis so it survives restarts |
| **Sector ETF price not populated** — `Price: 0` in all `TickerSnapshot` entries | Claude receives change % only, no absolute price context | Populate price from the `/prev` response `c` (close) field already available in the response |

### Phase 4 — planned
- Clerk auth integration:
  - Fill in `ClerkAuthProvider.ValidateToken` with real Clerk JWT validation (no interface changes needed)
  - Wire `ClerkAuthProvider.GetLoginURL` to real Clerk OAuth flow
  - Replace localStorage token with Clerk session management in the frontend
  - `AuthProvider` interface already in place — swap is one file change
- Alpaca paper trading integration
- One-tap invest button with real execution

### Phase 5 — planned
- Plaid bank + 401k account connection
- `FinancialDataProvider` interface
- Auto-pull real account balances into investment context

### Phase 6 — planned
- Fully autonomous daily investment agent
- Push notifications
- Portfolio performance tracking
