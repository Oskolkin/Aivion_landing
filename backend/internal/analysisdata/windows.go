package analysisdata

import "time"

// DateRange is an inclusive calendar date range in YYYY-MM-DD form.
type DateRange struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// DataWindows are the rolling analysis windows derived from metric date.
type DataWindows struct {
	CurrentDay string    `json:"current_day"`
	Yesterday  string    `json:"yesterday"`
	Last7d     DateRange `json:"last_7d"`
	Previous7d DateRange `json:"previous_7d"`
	Last14d    DateRange `json:"last_14d"`
}

// ComputeDataWindows derives rolling windows from metricDate (inclusive last_7d / last_14d).
//
//	current_day  = metricDate
//	yesterday    = metricDate - 1 day
//	last_7d      = metricDate - 6 days ... metricDate
//	previous_7d  = metricDate - 13 days ... metricDate - 7 days
//	last_14d     = metricDate - 13 days ... metricDate
func ComputeDataWindows(metricDate time.Time) DataWindows {
	d := DateOnlyUTC(metricDate)
	yesterday := d.AddDate(0, 0, -1)
	last7From := d.AddDate(0, 0, -6)
	prev7To := d.AddDate(0, 0, -7)
	prev7From := d.AddDate(0, 0, -13)
	last14From := d.AddDate(0, 0, -13)

	return DataWindows{
		CurrentDay: FormatDate(d),
		Yesterday:  FormatDate(yesterday),
		Last7d: DateRange{
			From: FormatDate(last7From),
			To:   FormatDate(d),
		},
		Previous7d: DateRange{
			From: FormatDate(prev7From),
			To:   FormatDate(prev7To),
		},
		Last14d: DateRange{
			From: FormatDate(last14From),
			To:   FormatDate(d),
		},
	}
}

// DateOnlyUTC truncates t to a UTC calendar date at midnight.
func DateOnlyUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// FormatDate formats t as YYYY-MM-DD in UTC.
func FormatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// ParseDate parses YYYY-MM-DD into a UTC date-only time. Invalid input yields zero time.
func ParseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return DateOnlyUTC(t)
}
