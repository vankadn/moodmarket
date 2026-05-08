# Logging Rules

Logs are for understanding flow, not recording data. Never commit a log line that contains PII or sensitive user data.

**Never log:**
- Financial amounts (salary, balances, budget, portfolio value)
- Institution or account names
- Personal profile fields (name, email, immigration status)
- Full prompts or API payloads sent to external services

**Safe to log:**
- User ID (opaque identifier, not personal data)
- Step completion and counts (`3 allocations`, `2 accounts`)
- Categorical values that don't identify a person (`risk=medium`, `sentiment=bullish`)
- Error messages and latency
