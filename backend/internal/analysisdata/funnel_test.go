package analysisdata

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSelectAccountFunnelRowsPrefersEmptyScopeID(t *testing.T) {
	rows := []dbgen.ProductAnalyticsDaily{
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "account", ScopeID: "account", Impressions: pgInt8(1)},
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "account", ScopeID: "", Impressions: pgInt8(99)},
	}
	picked := SelectAccountFunnelRows(rows)
	if len(picked) != 1 {
		t.Fatalf("len=%d", len(picked))
	}
	if picked[0].ScopeID != "" || picked[0].Impressions.Int64 != 99 {
		t.Fatalf("picked wrong row: scope_id=%q impressions=%d", picked[0].ScopeID, picked[0].Impressions.Int64)
	}
}

func TestSelectAccountFunnelRowsExcludesOtherScopes(t *testing.T) {
	rows := []dbgen.ProductAnalyticsDaily{
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "sku", ScopeID: "", Impressions: pgInt8(1)},
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "account", ScopeID: "other", Impressions: pgInt8(2)},
		{MetricDate: pgDate(2026, 6, 26), ScopeType: "account", ScopeID: "account", Impressions: pgInt8(3)},
	}
	picked := SelectAccountFunnelRows(rows)
	if len(picked) != 1 {
		t.Fatalf("len=%d", len(picked))
	}
	if FormatDate(picked[0].MetricDate.Time) != "2026-06-26" {
		t.Fatalf("date=%s", FormatDate(picked[0].MetricDate.Time))
	}
}

func TestSelectAccountFunnelRowsSortedAndOnePerDate(t *testing.T) {
	rows := []dbgen.ProductAnalyticsDaily{
		{MetricDate: pgDate(2026, 6, 27), ScopeType: "account", ScopeID: "account", Impressions: pgInt8(27)},
		{MetricDate: pgDate(2026, 6, 25), ScopeType: "account", ScopeID: "", Impressions: pgInt8(25)},
		{MetricDate: pgDate(2026, 6, 26), ScopeType: "account", ScopeID: "account", Impressions: pgInt8(26)},
		{MetricDate: pgDate(2026, 6, 26), ScopeType: "account", ScopeID: "", Impressions: pgInt8(260)},
	}
	picked := SelectAccountFunnelRows(rows)
	if len(picked) != 3 {
		t.Fatalf("len=%d", len(picked))
	}
	wantDates := []string{"2026-06-25", "2026-06-26", "2026-06-27"}
	for i, want := range wantDates {
		got := FormatDate(picked[i].MetricDate.Time)
		if got != want {
			t.Fatalf("index %d date=%s want=%s", i, got, want)
		}
	}
	if picked[1].Impressions.Int64 != 260 || picked[1].ScopeID != "" {
		t.Fatalf("expected empty scope preferred on 2026-06-26: %+v", picked[1])
	}
}

func TestSelectAccountFunnelRowsEmpty(t *testing.T) {
	if got := SelectAccountFunnelRows(nil); len(got) != 0 {
		t.Fatalf("len=%d", len(got))
	}
	if got := SelectAccountFunnelRows([]dbgen.ProductAnalyticsDaily{}); len(got) != 0 {
		t.Fatalf("len=%d", len(got))
	}
}

func pgDate(y int, m time.Month, d int) pgtype.Date {
	return pgtype.Date{Time: DateOnlyUTC(time.Date(y, m, d, 0, 0, 0, 0, time.UTC)), Valid: true}
}

func pgInt8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}
