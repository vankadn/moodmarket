# InvestIQ

Mood-driven daily investment advisor powered by Claude.

---

## Running locally

### 1. Start MongoDB

```
mongod --dbpath ~/data/db
```

### 2. Configure backend/.env

The minimum `.env` to run with **no external accounts**:

```env
# Mocks everything — no Plaid, Alpaca, Polygon, Anthropic, or Clerk needed
MOCK_ALL=true

# Local dev user (used when MOCK_ALL=true sets DEV_MODE=true)
DEV_USER_ID=local
DEV_USER_EMAIL=local@dev.com
DEV_USER_NAME=Local

# Local MongoDB
MONGODB_URI=mongodb://localhost:27017

# Scheduler
AUTO_INVEST_INTERVAL=24h
NOTIFICATION_PROVIDER=log
```

To use real Claude recommendations but keep everything else mocked, drop `MOCK_ALL` and add:

```env
AI_PROVIDER=claude
ANTHROPIC_API_KEY=your_key_here
```

### 3. Start the backend

```
cd backend
go run cmd/server/main.go
```

### 4. Start the frontend

```
cd frontend
npm install
npm run dev
```

Open http://localhost:5173

---

## Switching to real providers

Each provider is swapped independently — change one at a time.

| What | Env var | Values | Required keys |
|------|---------|--------|---------------|
| Auth | `DEV_MODE` | `true` (stub) / `false` (Clerk) | `CLERK_SECRET_KEY` when false |
| AI advisor | `AI_PROVIDER` | `mock` / `claude` | `ANTHROPIC_API_KEY` when claude |
| Market data | `MARKET_PROVIDER` | `mock` / `polygon` | `POLYGON_API_KEY` |
| Bank accounts | `FINANCIAL_DATA_PROVIDER` | `mock` / `plaid` | `PLAID_CLIENT_ID`, `PLAID_SECRET`, `PLAID_ENV` |
| Brokerage | `BROKERAGE_PROVIDER` | `mock` / `alpaca` | `ALPACA_API_KEY`, `ALPACA_API_SECRET`, `ALPACA_BASE_URL` |

### Plaid-specific

```env
PLAID_ENV=sandbox          # sandbox | development | production
PLAID_CACHE_TTL=5m         # how long to cache balance fetches (reduces API calls)
PLAID_TOKEN_ENCRYPTION_KEY= # generate with: openssl rand -base64 32
```

### Alpaca-specific

Use `https://paper-api.alpaca.markets` as `ALPACA_BASE_URL` for paper trading.

### Frontend

```env
VITE_DEV_MODE=true         # true = skip Clerk login screen
```
