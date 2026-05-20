// domain/models/portfolio_connection.go
package models

import "time"

// PortfolioConnection holds provider credentials for a connected portfolio aggregator.
// ProviderUserID and ProviderUserSecret are stored AES-256-GCM encrypted in MongoDB.
// ProviderUserSecret is a long-lived HMAC signing key — never log it.
type PortfolioConnection struct {
	Provider           string    // e.g. "snaptrade"
	ProviderUserID     string    // encrypted at rest; decrypted by infrastructure layer before use
	ProviderUserSecret string    // AES-256-GCM encrypted HMAC signing key; never log
	ConnectedAt        time.Time
}

// PortfolioConnectionStatus is the safe subset returned to API callers — secrets are never included.
type PortfolioConnectionStatus struct {
	Provider    string `json:"provider"`
	Connected   bool   `json:"connected"`
	ConnectedAt string `json:"connected_at,omitempty"`
}
