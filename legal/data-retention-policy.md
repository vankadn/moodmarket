# Data Retention and Disposal Policy

**InvestIQ**
Last updated: May 7, 2026

---

## Purpose

This policy defines how InvestIQ retains, manages, and disposes of user data. It applies to all personal and financial data collected through the InvestIQ application.

---

## Data categories and retention periods

| Data category | Retention period | Basis |
|--------------|-----------------|-------|
| User financial profile (salary, goals, risk tolerance, etc.) | Duration of active account | Required for app function |
| Plaid access tokens | Until account disconnected or deleted | Required for bank account access |
| Investment decision history (recommendations, allocations, receipts) | Duration of active account | User audit trail |
| Market snapshot data (SPY, QQQ, sector ETFs at time of decision) | Duration of active account | Part of decision audit trail |
| Authentication data (managed by Clerk) | Governed by Clerk's retention policy | Delegated to Clerk |
| Server logs | 30 days | Operational debugging |

---

## Account deletion

When a user deletes their account, the following occurs immediately and permanently:

1. Financial profile deleted from MongoDB
2. All investment decision records deleted from MongoDB
3. All Plaid access tokens revoked via the Plaid `/item/remove` API endpoint
4. All Plaid connection records deleted from MongoDB
5. Authentication record deletion requested from Clerk
6. No data is retained in backups beyond the next scheduled backup rotation (maximum 7 days)

Account deletion is irreversible. Users are informed of this before confirming deletion.

---

## Plaid account disconnection

When a user disconnects a specific bank account (without deleting their full account):

1. The Plaid access token for that institution is immediately revoked via Plaid's `/item/remove` API
2. The connection record is deleted from MongoDB
3. No further balance data can be retrieved for that institution
4. Historical decision records that included balance data from that institution are retained as part of the investment audit trail but no new data is pulled

Disconnection is immediate. The access token is revoked at Plaid before the database record is deleted — this order is enforced in code to prevent orphaned live tokens.

---

## Background data access

When auto-invest is enabled, InvestIQ accesses connected account balances once daily. Every background access is logged with:

- Timestamp of access
- User ID
- Which institutions were queried
- What balance data was returned
- What investment decision was made as a result

These logs are retained for the duration of the active account and deleted with the account.

When auto-invest is disabled, all background account access stops immediately. No data is accessed outside of explicit user-initiated actions.

---

## Data minimization

InvestIQ does not persist data it does not need:

- Account balance figures retrieved from Plaid are used at request time to build investment context and are not stored as standalone records (the decision record stores a snapshot for audit purposes only)
- Transaction history is not requested from Plaid even if the connected account has it — only balance data is accessed
- No cookies, advertising identifiers, or behavioral tracking data are collected

---

## Disposal method

All data is stored in MongoDB Atlas. Deletion operations use MongoDB's standard delete commands which remove documents from the database. Plaid tokens are revoked via API before database deletion to ensure credentials are invalidated at the source.

MongoDB Atlas automated backups are retained for 7 days. Deleted user data may persist in backups for up to 7 days after deletion, after which it is permanently unrecoverable.

---

## Policy review

This policy is reviewed and updated when:
- A new data category is introduced
- A new third-party integration is added
- Applicable privacy laws change
- The app moves from development to production

---

## Contact

For questions about this policy or to request data deletion:

Email: krishnarajivvns@gmail.com
