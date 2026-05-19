package models

import "time"

type ClassificationEntry struct {
	Ticker            string    `json:"ticker"`
	AssetClass        string    `json:"asset_class"`
	Approved          bool      `json:"approved"`
	SuggestedByClaude bool      `json:"suggested_by_claude"`
	FirstSeenAt       time.Time `json:"first_seen_at"`
}
