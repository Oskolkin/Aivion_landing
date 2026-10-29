package publication

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

func TestHasCanonicalFunnelContractRequiresBothCompletedDatesAndSeparateWeeklyFacts(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{}
	if hasCanonicalFunnelContract(facts) {
		t.Fatal("empty facts must not be treated as rebuilt")
	}
	facts.Funnel.CurrentCompletedDate = "2026-07-07"
	facts.Funnel.PreviousCompletedDate = "2026-07-06"
	facts.Funnel.DailySeries = []dashboardfacts.FunnelDailyPoint{}
	if hasCanonicalFunnelContract(facts) {
		t.Fatal("missing overview_7d must not be treated as rebuilt")
	}
	facts.Funnel.Overview7D.Status = "available"
	facts.AdEfficiency.Period = &dashboardfacts.AdEfficiencyPeriod{MetricDate: "2026-07-08", CurrentCompletedDate: "2026-07-07", PreviousCompletedDate: "2026-07-06", Current7dDateFrom: "2026-07-01", Current7dDateTo: "2026-07-07", Previous7dDateFrom: "2026-06-24", Previous7dDateTo: "2026-06-30"}
	v := 1.0
	facts.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{PreviousDRR7dPct: &v, DeltaVsPrevious7dPp: &v}
	facts.AdEfficiency.Trend7D = []dashboardfacts.AdEfficiencyTrendPoint{}
	if !hasCanonicalFunnelContract(facts) {
		t.Fatal("complete D-1/D-2 + daily + weekly contract should be recognized")
	}
}

func TestDeterministicDashboardRebuildIsNotAnAIProviderRun(t *testing.T) {
	if deterministicDashboardRebuildTrigger == "" {
		t.Fatal("rebuild trigger must be explicit for auditability")
	}
	if deterministicDashboardRebuildTrigger == "unified_analysis" {
		t.Fatal("rebuild must not masquerade as a provider analysis")
	}
	if got := (&RebuildDashboardPublicationResult{}).ProviderCallsDelta; got != 0 {
		t.Fatalf("provider calls delta=%d, want 0", got)
	}
}
