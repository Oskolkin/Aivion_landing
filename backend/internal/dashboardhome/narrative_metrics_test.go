package dashboardhome

import "testing"

func TestNarrativeMetricsAcceptanceStatuses(t *testing.T) {
	cases := []struct {
		status   string
		accepted bool
	}{
		{status: "completed", accepted: true},
		{status: "completed_with_warnings", accepted: true},
		{status: "completed_repaired", accepted: true},
		{status: "completed_partial", accepted: true},
		{status: "failed", accepted: false},
		{status: "fallback_stale", accepted: false},
	}
	for _, tc := range cases {
		if got := narrativeGenerationStatusAccepted(tc.status); got != tc.accepted {
			t.Fatalf("status=%q accepted=%v want %v", tc.status, got, tc.accepted)
		}
	}
}

func TestRecordNarrativeGenerationOutcome_CompletedWithWarnings(t *testing.T) {
	resetNarrativeMetricsForTest()
	recordNarrativeGenerated()
	recordNarrativeGenerationOutcome("completed_with_warnings", nil)
	if narrativeMetrics.CompletedWithWarningsTotal != 1 {
		t.Fatalf("completed_with_warnings_total=%d", narrativeMetrics.CompletedWithWarningsTotal)
	}
	if narrativeAcceptanceRate() != 1 {
		t.Fatalf("acceptance_rate=%v want 1", narrativeAcceptanceRate())
	}
}
