---
name: project-snaptrade-context
description: SnapTrade portfolio aggregation plan — architectural constraints, auth model, and key decisions to remember for implementation sessions
metadata:
  type: project
---

SnapTrade planned as P2 feature: read-only portfolio aggregation (Robinhood, Fidelity) via OAuth. Trade execution blocked until SnapTrade confirms Robinhood write access.

Auth model: app-level clientId + consumerKey (env vars), per-user snaptrade_user_id + snaptrade_user_secret. The user_secret is long-lived and used to SIGN every API request (HMAC), not just as a bearer token. This means it is a signing key, not an access token — rotation is destructive and requires re-registration.

**Why:** SnapTrade's signing model means losing the user_secret = user must re-register and re-connect all brokerages. Encryption at rest is critical; secret must never be logged.

**How to apply:** When designing the SnapTrade infrastructure, always treat user_secret as a signing key. Loss = full reconnection. Encrypt with AES-256-GCM same as Alpaca/Plaid pattern. Never log it. Flag any code path that could expose it.

Key architectural decision log (from plan review session 2026-05-20):
- Interface should be split: PortfolioAggregator (holdings) + PortfolioConnector (OAuth flow) — two distinct responsibilities
- portfolio_connections embedded in users collection (same pattern as plaid_connections) — not a separate collection
- Encryption uses existing EncryptToken/DecryptToken in infrastructure/db — same PLAID_TOKEN_ENCRYPTION_KEY
- Factory pattern: SNAPTRADE_PROVIDER=snaptrade|mock, same shape as BROKERAGE_PROVIDER and FINANCIAL_DATA_PROVIDER
- App-level credentials (clientId, consumerKey) stored in factory, not per-user model
- SnapTrade holdings injected into req.Positions alongside Alpaca in RecommendationService step 5
- New sentinel: ErrPortfolioNotConnected (same pattern as ErrBrokerageNotConnected)
- ProfileRepository needs 4 new methods: SaveSnapTradeConnection, GetSnapTradeConnection, ClearSnapTradeConnection, and the aggregator needs GetHoldings + GenerateConnectURL + DisconnectBrokerage

[[project-architecture-constraints]]
