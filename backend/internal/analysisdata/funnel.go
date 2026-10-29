package analysisdata

import (
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// SelectAccountFunnelRows filters account-scope funnel rows for Analysis Pack use.
//
// Rules:
//   - only scope_type = "account"
//   - scope_id may be "" or "account"; other scope IDs are excluded
//   - for a given date, prefer the row with empty scope_id
//   - result is deterministically sorted by metric date ascending
func SelectAccountFunnelRows(rows []dbgen.ProductAnalyticsDaily) []dbgen.ProductAnalyticsDaily {
	byDate := map[string]dbgen.ProductAnalyticsDaily{}
	for _, row := range rows {
		if row.ScopeType != "account" {
			continue
		}
		if row.ScopeID != "" && row.ScopeID != "account" {
			continue
		}
		d := FormatDate(row.MetricDate.Time)
		existing, ok := byDate[d]
		if !ok || (existing.ScopeID != "" && row.ScopeID == "") {
			byDate[d] = row
		}
	}
	out := make([]dbgen.ProductAnalyticsDaily, 0, len(byDate))
	for _, row := range byDate {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		return FormatDate(out[i].MetricDate.Time) < FormatDate(out[j].MetricDate.Time)
	})
	return out
}
