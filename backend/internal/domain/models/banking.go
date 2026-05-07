// domain/models/banking.go
package models

import "time"

// PlaidConnection is the full connection record stored per user.
// AccessToken holds the encrypted value — it is decrypted only in memory, never logged or sent over HTTP.
type PlaidConnection struct {
	Institution string
	AccessToken string // AES-256-GCM encrypted; decrypted by infrastructure layer before use
	ItemID      string
}

// PlaidConnectionSummary is the safe subset returned to callers — institution and item_id only.
// AccessToken is intentionally absent so it can never leak to the API response or frontend.
type PlaidConnectionSummary struct {
	Institution string `json:"institution"`
	ItemID      string `json:"item_id"`
}

// BankAccount is a single account returned by the balance endpoint.
type BankAccount struct {
	AccountID   string
	Institution string
	Name        string
	Type        string  // depository, investment, credit, loan
	Subtype     string  // checking, savings, 401k, brokerage, etc.
	Balance     float64
	Currency    string
}

// BalanceSummary aggregates account balances across all connected institutions.
// It is built fresh on every recommendation request so Claude sees current data.
type BalanceSummary struct {
	TotalCash        float64  // sum of depository account balances
	TotalInvestments float64  // sum of investment account balances
	Institutions     []string // unique institution names
	AccountCount     int
	PulledAt         time.Time
}
