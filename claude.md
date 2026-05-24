# Claude Code Instructions — InvestIQ

> For project phases, tech stack, and product decisions — read `InvestIQ_Project_Context.md`

---

## Commands

```bash
# Backend
cd backend
go run ./cmd/server/main.go          # start API (port 8080)
go test ./...                        # all tests
go test ./internal/application/...   # single package
go build ./...                       # compile check
go run ./cmd/dbcheck [collection]    # inspect MongoDB without mongosh

# Frontend
cd frontend
npm run dev       # Vite dev server → http://localhost:5173
npm run build     # tsc + vite build
npm test          # vitest (single pass)
npx vitest        # vitest watch mode
```

Minimum dev setup — set in `backend/.env`:
```
MOCK_ALL=true
DEV_USER_ID=local
DEV_USER_EMAIL=local@dev.com
DEV_USER_NAME=Local
MONGODB_URI=mongodb://localhost:27017
AUTO_INVEST_INTERVAL=24h
NOTIFICATION_PROVIDER=log
VERDICT_MIN_AGE=0s
```
`MOCK_ALL=true` mocks every provider and enables `DEV_MODE`. MongoDB is the only external dependency. Set `VITE_DEV_MODE=true` in `frontend/.env` to skip the Clerk login screen.

---

## Provider env vars

| What | Env var | Values |
|------|---------|--------|
| Auth | `DEV_MODE` | `true` (stub) / `false` (Clerk + `CLERK_SECRET_KEY`) |
| AI advisor | `AI_PROVIDER` | `mock` / `claude` (+ `ANTHROPIC_API_KEY`) |
| Market data | `MARKET_PROVIDER` | `mock` / `polygon` (+ `POLYGON_API_KEY`) |
| Banking | `FINANCIAL_DATA_PROVIDER` | `mock` / `plaid` (+ `PLAID_CLIENT_ID`, `PLAID_SECRET`, `PLAID_ENV`) |
| Brokerage | `BROKERAGE_PROVIDER` | `mock` / `alpaca` / `coinbase` |
| Notifications | `NOTIFICATION_PROVIDER` | `log` / `resend` / `twilio` |
| Document extraction | `DOCUMENT_EXTRACTOR` | `mock` / `claude` |
| Rebalancing drift | `REBALANCE_DRIFT_THRESHOLD` | float, default `10.0` (percentage points) |

---

## Onion Architecture — strictly enforced

Dependency direction: outer layers depend on inner. Never the reverse.

```
domain/          ← innermost: entities, value objects, interfaces (domain/ports/)
application/     ← use cases, orchestration; imports domain only
infrastructure/  ← implements domain/ports/; imports domain only
api/             ← HTTP handlers; imports application + domain only
```

Any import that violates this direction is a hard stop — redesign before proceeding.

## Interface-first

Every external dependency (AI, database, auth, market data, brokerage, banking) must be hidden behind an interface in `domain/ports/`. Application layer talks to interfaces only. Swapping any provider requires one new file + one env var change. Nothing else.

## Brokerage providers — hard rules

- All brokerage providers implement `BrokerageProvider` (defined in `domain/ports/`) — no SDK imported into domain, application, or handler layers
- DEV_MODE and sandbox switching live in factory functions only (`infrastructure/*/factory.go`) — never in handlers or application services
- **Banned:** `robin_stocks`, any unofficial Robinhood client, any reverse-engineered Fidelity endpoint, any library that scrapes or mimics a broker's private API
- Approved providers: Alpaca (equity execution), Coinbase Advanced Trade (crypto execution), SnapTrade (read-only portfolio aggregation)

## Credential storage — per-user vs per-app

| Type | Example | Where it lives |
|------|---------|----------------|
| Per-app | Polygon, Anthropic API keys | `.env` → Railway env var |
| Per-user | Alpaca API key, Plaid access token, SnapTrade secrets | AES-256-GCM encrypted in MongoDB per user document |

Per-user credentials in env vars = every user shares one account. Always wrong for financial data or trade execution.

## Testing

Table-driven, parallel, in the same package as the function under test. Test pure functions and domain logic — not repository implementations, HTTP handlers, or provider infrastructure.

```go
func TestFoo(t *testing.T) {
    t.Parallel()
    cases := []struct{ name string; ... }{ ... }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
        })
    }
}
```

## DEV_MODE pattern

`DEV_MODE=true` bypasses auth. That branch lives in exactly one place: `infrastructure/auth/factory.go`. No DEV_MODE checks anywhere else in the codebase.

## Go rules

- Standard library only — no Gin, Echo, GORM, or any third-party framework
- `fmt.Errorf("context: %w", err)` everywhere — no silent failures, no bare `return err`
- Names read like English: `GetDailyRecommendation`, not `getData`. Abbreviations: `ctx err db req res` only
- One primary responsibility per file — no god files
- userId comes from context only — always `identityProvider.GetCurrentUser(ctx)`, never a global or hardcoded value
- Business logic lives in the domain layer — never in handlers or infrastructure

## Git workflow

Before any commit or push, read `skills/pre-commit-checklist.md` and verify every item.

1. Write and build the code
2. You verify it works
3. You say "commit" or "push" — then do it

## Project context

`InvestIQ_Project_Context.md` is the source of truth for phases, decisions, and known debt. Keep it current — update it as part of the same session that completes a feature, not after the fact.

Update when:
- A phase or sub-phase is marked complete
- A new architectural or product decision is made
- Known debt is resolved or added
- A domain model, collection, or interface changes

## Skills — load when relevant

- Adding or reviewing logs → read `skills/logging-rules.md`
- Touching frontend → read `skills/react-rules.md`
- Starting a new feature implementation → read `skills/new-feature-checklist.md`
- Committing or pushing → read `skills/pre-commit-checklist.md`
- Adding or changing business logic → read `skills/testing-rules.md`
- Integrating any third-party service or credential → read `skills/credential-storage-rules.md`
- Adding, changing, or removing any API endpoint → read `skills/postman-update-rules.md`
