# Credential Storage Rules — User-Facing Integrations

Before wiring any third-party credential, ask:
**"Is this credential per-user or per-app?"**

| Type | Example | Where it lives |
|------|---------|---------------|
| Per-app | Polygon API key, Anthropic API key | `.env` → Railway env var |
| Per-user | Alpaca API key, Plaid access token | AES-256-GCM encrypted in Mongo, per user document |

**If per-user credentials live in env vars, every user shares one account. That is always wrong for financial data or trade execution.**

Anti-pattern to call out immediately:
- "That Alpaca key is in `.env` — every user trades against the same account. Move to per-user encrypted Mongo storage, same pattern as Plaid."

This check belongs at the moment of integration design, not after the feature ships.
