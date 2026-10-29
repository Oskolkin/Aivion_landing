package analysisdata

import (
	"context"
	"fmt"
	"time"
)

// LegacyHistoryDays is the default Load() history window (inclusive calendar days).
const LegacyHistoryDays = 14

// UnifiedPackHistoryDays is the Analysis Pack v1 history window.
const UnifiedPackHistoryDays = 30

// MaxHistoryDays is the upper bound accepted by LoadWithOptions.
const MaxHistoryDays = 90

// LoadOptions configures snapshot loading.
type LoadOptions struct {
	SellerAccountID int64
	MetricDate      time.Time
	// HistoryDays is inclusive calendar days ending at MetricDate.
	// Legacy uses 14; Unified Analysis Pack uses 30.
	HistoryDays int
}

// OptionsLoader loads snapshots with an explicit history window.
type OptionsLoader interface {
	LoadWithOptions(ctx context.Context, options LoadOptions) (*AnalysisDataSnapshot, error)
}

// LoadedHistory records the actual SQL date range loaded into the snapshot.
// Legacy BuildPack ignores this field.
type LoadedHistory struct {
	From string
	To   string
	Days int
}

// HistoryDateRange returns inclusive [from, to] for historyDays ending at metricDate (UTC date-only).
func HistoryDateRange(metricDate time.Time, historyDays int) (from, to time.Time, err error) {
	if historyDays <= 0 {
		return time.Time{}, time.Time{}, fmt.Errorf("history_days must be > 0")
	}
	if historyDays > MaxHistoryDays {
		return time.Time{}, time.Time{}, fmt.Errorf("history_days must be <= %d", MaxHistoryDays)
	}
	to = DateOnlyUTC(metricDate)
	from = to.AddDate(0, 0, -(historyDays - 1))
	return from, to, nil
}
