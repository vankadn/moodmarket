// infrastructure/calendar/mock_calendar.go
package calendar

import "time"

// MockCalendar always reports every day as a trading day — used in MOCK_ALL mode
// so the scheduler runs during tests regardless of the actual date.
type MockCalendar struct{}

func (m *MockCalendar) IsTradingDay(_ time.Time) bool { return true }
