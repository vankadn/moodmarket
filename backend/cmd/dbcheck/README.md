# dbcheck

A lightweight diagnostic CLI for inspecting InvestIQ's MongoDB collections without
installing mongosh or any external tools. Uses the same driver already in `go.mod`.

## Usage

Run from the `backend/` directory:

```bash
# Default — inspects ticker_classifications
go run ./cmd/dbcheck

# Inspect a specific collection
go run ./cmd/dbcheck decisions
go run ./cmd/dbcheck profiles
go run ./cmd/dbcheck auto_invest_configs
go run ./cmd/dbcheck scheduler_runs
go run ./cmd/dbcheck tax_documents
```

Reads `MONGODB_URI` from the environment or `.env` file (same as the server).
Falls back to `mongodb://localhost:27017` if neither is set.

Prints up to 200 documents as pretty-printed JSON with `_id` omitted.

## Common checks

```bash
# How many tickers are seeded and approved
go run ./cmd/dbcheck | grep '"approved": true' | wc -l

# Claude-suggested tickers not yet reviewed (approved: false)
go run ./cmd/dbcheck | grep -A8 '"approved": false'

# Count all documents in a collection
go run ./cmd/dbcheck decisions | head -1
```

## ticker_classifications

This is the primary collection managed by dbcheck. Fields:

| Field               | Type    | Description                                              |
|---------------------|---------|----------------------------------------------------------|
| `ticker`            | string  | Unique symbol (e.g. `VTI`)                               |
| `asset_class`       | string  | `US Equity`, `International`, `Bonds`, `Real Estate`, `Commodities`, or `Other` |
| `approved`          | bool    | `true` = in the in-memory cache; `false` = pending review |
| `suggested_by_claude` | bool  | `true` = Claude recommended this ticker during a live recommendation |
| `first_seen_at`     | time    | When the record was first inserted                       |

### Approving a Claude-suggested ticker

When `approved: false` entries appear, it means Claude recommended a ticker not in the
seed list. To approve one, update it directly in Mongo and restart the server:

```
db.ticker_classifications.updateOne(
  { ticker: "GOOGL" },
  { $set: { approved: true, asset_class: "US Equity" } }
)
```

The in-memory cache is refreshed only at startup via `RefreshCache`, so a server
restart is required for the change to take effect.

## Seed list (29 tickers)

Seeded automatically on every server startup using `$setOnInsert` — restarts never
overwrite existing records.

| Asset Class   | Tickers                                              |
|---------------|------------------------------------------------------|
| US Equity     | VTI, SPY, QQQ, XLK, XLF, XLE, XLV, XLI, AAPL, MSFT, NVDA, AMZN, TSLA |
| International | VXUS, EFA, EEM, VEA, VWO                            |
| Bonds         | BND, AGG, SGOV, SHV, TLT, IEF                       |
| Real Estate   | VNQ, SCHH                                            |
| Commodities   | GLD, SLV, USO                                        |
