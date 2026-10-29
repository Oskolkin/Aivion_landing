package admin

import (
	"errors"
	"time"
)

const metricsRerunDefaultInclusiveDays = 30

func metricsCalendarDayUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

func inclusiveDaySpan(from, to time.Time) int {
	if to.Before(from) {
		return 0
	}
	return int(to.Sub(from)/(24*time.Hour)) + 1
}

func resolveRerunMetricsDateRange(in RerunMetricsInput) (from, to time.Time, err error) {
	if in.DateFrom.IsZero() && in.DateTo.IsZero() {
		to = metricsCalendarDayUTC(time.Now())
		from = to.AddDate(0, 0, -(metricsRerunDefaultInclusiveDays - 1))
		return from, to, nil
	}
	if in.DateFrom.IsZero() || in.DateTo.IsZero() {
		return time.Time{}, time.Time{}, errors.New("date_from and date_to must both be set or both omitted")
	}
	from = metricsCalendarDayUTC(in.DateFrom)
	to = metricsCalendarDayUTC(in.DateTo)
	if from.After(to) {
		return time.Time{}, time.Time{}, errors.New("date_from must be before or equal to date_to")
	}
	if inclusiveDaySpan(from, to) > 366 {
		return time.Time{}, time.Time{}, errors.New("date range is too large")
	}
	return from, to, nil
}
