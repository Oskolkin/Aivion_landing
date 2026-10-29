package aidatapack

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
)

// ComputeDataWindows derives rolling windows from metricDate (inclusive last_7d / last_14d).
func ComputeDataWindows(metricDate time.Time) DataWindows {
	return analysisdata.ComputeDataWindows(metricDate)
}

func dateOnlyUTC(t time.Time) time.Time {
	return analysisdata.DateOnlyUTC(t)
}

func formatDate(t time.Time) string {
	return analysisdata.FormatDate(t)
}

func parseDate(s string) time.Time {
	return analysisdata.ParseDate(s)
}

func windowContains(windows DataWindows, window, dateStr string) bool {
	d := parseDate(dateStr)
	switch window {
	case "current_day":
		return dateStr == windows.CurrentDay
	case "yesterday":
		return dateStr == windows.Yesterday
	case "last_7d":
		return !d.Before(parseDate(windows.Last7d.From)) && !d.After(parseDate(windows.Last7d.To))
	case "previous_7d":
		return !d.Before(parseDate(windows.Previous7d.From)) && !d.After(parseDate(windows.Previous7d.To))
	case "last_14d":
		return !d.Before(parseDate(windows.Last14d.From)) && !d.After(parseDate(windows.Last14d.To))
	default:
		return false
	}
}
