// domain/ports/market_calendar.go
package ports

import "time"

// MarketCalendar reports whether a given day is an NYSE trading day.
type MarketCalendar interface {
	IsTradingDay(t time.Time) bool
}
