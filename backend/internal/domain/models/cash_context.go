// domain/models/cash_context.go
package models

// CashContext is the pre-computed spending insight returned to the frontend
// before the user taps invest. All runway and label computation happens in the
// service layer — this struct is a pure data carrier.
type CashContext struct {
	HasData              bool    `json:"has_data"`
	RunwayDays           int     `json:"runway_days"`
	RunwayLabel          string  `json:"runway_label"`           // "healthy" | "moderate" | "tight"
	SpendLast7D          float64 `json:"spend_last_7d"`
	SpendLast30D         float64 `json:"spend_last_30d"`
	LargestPendingAmount float64 `json:"largest_pending_amount"`
	LargestPendingName   string  `json:"largest_pending_name"`
	Message              string  `json:"message"`
}
