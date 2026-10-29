package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestFrozenDashboardData_CanonicalHashStable(t *testing.T) {
	dto := analytics.DashboardMetricsDTO{
		SellerAccountID: 42,
		AsOfDate:        "2026-06-24",
		Account: analytics.DashboardAccountMetrics{
			RevenueToday: 1200.5,
			OrdersToday:  8,
		},
	}
	resp := newDashboardResponseShellForMetrics()
	resp.MetricDate = "2026-06-24"
	resp.SellerPulse.KPIs = []KPICard{{Key: "revenue", Label: "Выручка", Value: "1200"}}

	frozen := NewFrozenDashboardData("2026-06-24", dto, &resp)
	h1, err := frozen.CanonicalHash()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := frozen.CanonicalHash()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("hash not stable: %s vs %s", h1, h2)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestDateFreshnessNote_HistoricalNotStale(t *testing.T) {
	got := DateFreshnessNote(true, false, 0, "2026-07-08")
	want := "Исторический анализ по данным на 08.07.2026"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDateFreshnessNote_CurrentStaleFromSourceLag(t *testing.T) {
	got := DateFreshnessNote(false, true, 10, "2026-07-08")
	want := "Данные не обновлялись 10 дн."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDateFreshnessNote_HistoricalWithStaleSources(t *testing.T) {
	// Genuine source lag still surfaces stale copy even for fixed/historical.
	got := DateFreshnessNote(true, true, 10, "2026-07-08")
	want := "Данные не обновлялись 10 дн."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestApplyFrozenDashboardData_RejectsMissingSchema(t *testing.T) {
	resp := Response{}
	err := ApplyFrozenDashboardData(&resp, FrozenDashboardData{})
	if err == nil {
		t.Fatal("expected schema error")
	}
}
