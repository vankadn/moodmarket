# InvestIQ — Project Context & Master Reference

> Load this into your Claude Project so every new conversation starts with full context.
> Last updated: 2026-05-06

---

## Who I am

- Name: Krishna
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

The strategy: every CPN module studied maps to a feature built in InvestIQ. Learning and building happen together.

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
- `AuthProvider` — identity (DevAuth today, Clerk in Phase 4)
- `IdentityProvider` — userId in request context
- `MarketDataProvider` — live prices (Phase 3 ✓)
- `DecisionRepository` — investment decision persistence (Phase 3 ✓)
- `BrokerageProvider` — trade execution (Phase 4, Alpaca)
- `FinancialDataProvider` — bank + 401k data (Phase 5, Plaid)

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
| Auth | DEV_MODE now, Clerk Phase 4 | Behind AuthProvider interface |
| Market Data | Polygon.io (previous-day, free tier) | Behind MarketDataProvider interface; mock available for dev |
| Brokerage | Alpaca Phase 4 | Paper trading first, then live |
| Banking | Plaid Phase 5 | Behind FinancialDataProvider interface |

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
| `BrokerageProvider` | Interface for trade execution (Phase 4) |

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

---

## MongoDB collections

| Collection | Purpose |
|-----------|---------|
| `users` | User profile + financial goals |
| `decisions` | Every daily investment decision logged (userId, timestamp, market snapshot, allocations) |
| `portfolio` | Simulated holdings tracker |
| `market_snapshots` | Daily price context (Phase 3) |

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

### Phase 4 — Planned
- Clerk auth: fill in `ClerkAuthProvider` stub with real Clerk SDK
- Replace DEV_MODE middleware with Clerk session management
- `BrokerageProvider` interface in `domain/ports/`
- Alpaca paper trading implementation
- One-tap invest button: Claude recommends → Alpaca executes
- Trade confirmation screen + receipt saved to MongoDB

### Phase 5 — Planned
- `FinancialDataProvider` interface in `domain/ports/`
- Plaid integration: connect real bank accounts + 401k
- Auto-pull account balances into investment context
- User connects accounts once via Plaid Link popup
- Claude sees real portfolio, real cash, real spending before recommending

### Phase 6 — Planned
- Fully autonomous daily investment agent
- Runs every morning, invests automatically
- Push notifications with daily summary
- Portfolio performance tracking + history view

---

## The long-term vision

A unified financial operating system. The app knows:
- Bank accounts (checking, savings, cash position) via Plaid
- 401k and retirement accounts (Fidelity, Vanguard, etc.)
- Brokerage accounts (what you already own)
- Daily spending patterns

When the user taps "Invest today", Claude receives a prompt like:
> "Krishna has $4,200 in checking, $180,000 in 401k (60/40 allocation), $12,000 in brokerage mostly QQQ and AAPL, a $1,200 credit card bill due in 8 days, H1B visa, 10-year horizon, moderate risk. Invest $100 today."

That context is what makes recommendations genuinely intelligent — not mood, not guesses.

Eventually: fully autonomous. App runs at 9am, invests, sends a notification. User never opens it unless they want to review.

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

---

## How to use this document

Load this into a Claude Project as a project file. Start every new conversation with:

> "Read the project context document first, then help me with [what you need]."

Or for VS Code Claude Code, reference it directly:

> "Read InvestIQ_Project_Context.md and CLAUDE.md, then [task]."

This document should be updated at the end of each phase as a record of what was built and why.
