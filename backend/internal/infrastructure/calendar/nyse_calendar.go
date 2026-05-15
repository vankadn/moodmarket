// infrastructure/calendar/nyse_calendar.go
package calendar

import "time"

// NYSECalendar reports trading days for the NYSE using the standard holiday schedule.
// All checks are performed in America/New_York time.
type NYSECalendar struct{}

var easternTZ *time.Location

func init() {
	var err error
	easternTZ, err = time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback: UTC-5 (EST) — acceptable if tzdata not available
		easternTZ = time.FixedZone("EST", -5*60*60)
	}
}

func (c *NYSECalendar) IsTradingDay(t time.Time) bool {
	d := t.In(easternTZ)
	if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	return !isNYSEHoliday(d)
}

func isNYSEHoliday(d time.Time) bool {
	y, m, day := d.Year(), d.Month(), d.Day()
	target := date(y, m, day)

	for _, h := range nysHolidaysForYear(y) {
		if h.Equal(target) {
			return true
		}
	}

	// Special case: if Jan 1 of next year falls on Saturday, NYSE closes Dec 31 this year.
	nextJan1 := time.Date(y+1, time.January, 1, 0, 0, 0, 0, time.UTC)
	if nextJan1.Weekday() == time.Saturday && m == time.December && day == 31 {
		return true
	}

	return false
}

func nysHolidaysForYear(y int) []time.Time {
	var h []time.Time

	// New Year's Day — Jan 1 (Sat→observed Fri prior year handled separately; Sun→Mon Jan 2)
	jan1 := date(y, time.January, 1)
	switch jan1.Weekday() {
	case time.Sunday:
		h = append(h, date(y, time.January, 2))
	case time.Saturday:
		// observed Dec 31 of prior year — handled via the special-case check
	default:
		h = append(h, jan1)
	}

	// MLK Day — 3rd Monday of January
	h = append(h, nthWeekday(y, time.January, time.Monday, 3))

	// Presidents Day — 3rd Monday of February
	h = append(h, nthWeekday(y, time.February, time.Monday, 3))

	// Good Friday — Easter Sunday − 2 days
	h = append(h, easterSunday(y).AddDate(0, 0, -2))

	// Memorial Day — last Monday of May
	h = append(h, lastWeekday(y, time.May, time.Monday))

	// Juneteenth — June 19, observed; NYSE added in 2022
	if y >= 2022 {
		h = append(h, observedFixed(y, time.June, 19))
	}

	// Independence Day — July 4, observed
	h = append(h, observedFixed(y, time.July, 4))

	// Labor Day — 1st Monday of September
	h = append(h, nthWeekday(y, time.September, time.Monday, 1))

	// Thanksgiving — 4th Thursday of November
	h = append(h, nthWeekday(y, time.November, time.Thursday, 4))

	// Christmas — December 25, observed
	h = append(h, observedFixed(y, time.December, 25))

	return h
}

// date constructs a midnight UTC time for comparison purposes.
func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// observedFixed returns the observed date for a fixed holiday:
// Saturday → prior Friday; Sunday → next Monday.
func observedFixed(y int, m time.Month, d int) time.Time {
	t := date(y, m, d)
	switch t.Weekday() {
	case time.Saturday:
		return t.AddDate(0, 0, -1)
	case time.Sunday:
		return t.AddDate(0, 0, 1)
	}
	return t
}

// nthWeekday returns the nth occurrence of weekday wd in month m of year y.
func nthWeekday(y int, m time.Month, wd time.Weekday, n int) time.Time {
	first := date(y, m, 1)
	diff := int(wd) - int(first.Weekday())
	if diff < 0 {
		diff += 7
	}
	return first.AddDate(0, 0, diff+(n-1)*7)
}

// lastWeekday returns the last occurrence of weekday wd in month m of year y.
func lastWeekday(y int, m time.Month, wd time.Weekday) time.Time {
	// time.Date with day=0 gives the last day of the prior month, so month+1 day 0 = last day of month m
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
	diff := int(last.Weekday()) - int(wd)
	if diff < 0 {
		diff += 7
	}
	return last.AddDate(0, 0, -diff)
}

// easterSunday computes Easter Sunday via the Meeus/Jones/Butcher algorithm.
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	hh := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - hh - k) % 7
	mm := (a + 11*hh + 22*l) / 451
	month := (hh + l - 7*mm + 114) / 31
	day := ((hh + l - 7*mm + 114) % 31) + 1
	return date(year, time.Month(month), day)
}
