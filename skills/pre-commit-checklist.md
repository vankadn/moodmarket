# Pre-Commit / Pre-Push Checklist

## Hard Rules
- Never commit or push unless explicitly told "commit" or "push" in this conversation
- Never commit "to save progress" — never push "as a backup"
- If not explicitly told to commit: say "Changes are done. Review with `git status`, then tell me to commit."

## Pre-Commit Checklist

### Architecture
- No business logic in `api/handlers/`
- No `infrastructure/` imports in `domain/` or `application/`
- Every new third-party dependency behind a `domain/ports/` interface
- DEV_MODE logic only in factory functions — nowhere else

### Logging / Privacy
- No financial amounts in logs (salary, balance, portfolio value)
- No institution or account names in logs
- No personal fields in logs (name, email, immigration status)
- No full Claude prompts or API payloads logged

### Secrets — hard stop if any of these are staged
- `.env` / `.env.local` / `.env.*` not staged — confirm in `.gitignore`
- No API keys, tokens, or secrets hardcoded in any staged file
- No Anthropic, Clerk, Plaid, Alpaca, Polygon keys in source
- No MongoDB connection strings with credentials
- No encryption keys or salts (e.g. PLAID_TOKEN_ENCRYPTION_KEY values)
- No private SSH or TLS keys
- Run: `git diff --staged | grep -iE "(sk-ant|sk_test|access-|apca-|pk[a-z0-9]{20}|mongodb\+srv)" ` — must return empty

### Personal data — hard stop if any of these are staged
- No real names, emails, or user IDs hardcoded in source (DEV_USER_* belong in .env only)
- No real account numbers, institution names, or financial data in source
- No real Plaid access tokens or item IDs
- No test fixtures containing real PII — use obviously fake data (e.g. "Jane Smith", "test@example.com")

### Go
- All errors wrapped: `fmt.Errorf("context: %w", err)` — no bare `return err`
- No third-party frameworks imported (no Gin, Echo, GORM)
- `userId` always from `identityProvider.GetCurrentUser(ctx)` — never hardcoded

### React
- No UI component libraries (no MUI, Chakra, Shadcn)
- No Redux

### Staged Files
- `package-lock.json` not staged
- `backend/server` and `frontend/dist/` not staged
- `.env` / `.env.local` not staged

## Pre-Push (in addition to above)
- You have confirmed it works
- No debug code or stray `fmt.Println` left in
- Commit message is descriptive — not "fix" or "update"

## Commit Message Format
[phase/area] Short description of what changed

- Key change 1
- Key change 2

Examples:
- `[phase7] Add dashboard total invested and decision count`
- `[infra] Move CORS to middleware wrapper in main.go`
