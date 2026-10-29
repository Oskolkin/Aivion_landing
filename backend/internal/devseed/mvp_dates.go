package devseed

import (
	"fmt"
	"time"
)

// MVPDateUTC returns the UTC calendar date at midnight for t.
func MVPDateUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// SeedStartDate is the first calendar day of a --days window ending at anchorDate (inclusive).
func SeedStartDate(anchorDate time.Time, days int) time.Time {
	if days <= 0 {
		return MVPDateUTC(anchorDate)
	}
	return MVPDateUTC(anchorDate).AddDate(0, 0, -(days - 1))
}

// SeedDateRange returns inclusive UTC calendar dates from start through anchorDate.
// When days <= 0, returns a single-element slice containing anchorDate only.
func SeedDateRange(anchorDate time.Time, days int) ([]time.Time, error) {
	anchor := MVPDateUTC(anchorDate)
	if days <= 0 {
		return []time.Time{anchor}, nil
	}
	start := SeedStartDate(anchor, days)
	out := make([]time.Time, 0, days)
	for d := start; !d.After(anchor); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	if len(out) != days {
		return nil, fmt.Errorf("seed date range: expected %d days, got %d (%s..%s)",
			days, len(out), start.Format("2006-01-02"), anchor.Format("2006-01-02"))
	}
	return out, nil
}

// MVPClampToCalendarDayUTC clamps t to the UTC calendar day of day (inclusive end 23:59:59).
func MVPClampToCalendarDayUTC(t, day time.Time) time.Time {
	dayStart := MVPDateUTC(day)
	dayEnd := dayStart.Add(24*time.Hour - time.Second)
	t = t.UTC()
	if t.Before(dayStart) {
		return dayStart
	}
	if t.After(dayEnd) {
		return dayEnd
	}
	return t
}

// mvpTimestampOnDay returns a UTC timestamp on the calendar day of day, hoursAfterMidnight in [0, 23].
func mvpTimestampOnDay(day time.Time, hoursAfterMidnight int) time.Time {
	dayStart := MVPDateUTC(day)
	if hoursAfterMidnight < 0 {
		hoursAfterMidnight = 0
	}
	if hoursAfterMidnight > 23 {
		hoursAfterMidnight = 23
	}
	return dayStart.Add(time.Duration(hoursAfterMidnight) * time.Hour)
}
