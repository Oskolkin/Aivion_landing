package readiness

import (
	"testing"
	"time"
)

func TestMaturityLatestTwoDistinctRunsAndCorrectionReset(t *testing.T) {
	type rec struct {
		hash string
		run  int64
	}
	mature := func(v []rec) bool { return len(v) >= 2 && v[0].run != v[1].run && v[0].hash == v[1].hash }
	if mature([]rec{{"H1", 1}}) {
		t.Fatal("one observation mature")
	}
	if !mature([]rec{{"H1", 2}, {"H1", 1}}) {
		t.Fatal("two confirming runs not mature")
	}
	if mature([]rec{{"H2", 3}, {"H1", 2}, {"H1", 1}}) {
		t.Fatal("correction did not reset maturity")
	}
	if !mature([]rec{{"H2", 4}, {"H2", 3}, {"H1", 2}}) {
		t.Fatal("correction reconfirmation not mature")
	}
}
func TestLastCompletedMoscowNeverCurrentDay(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	if got := lastCompletedMoscow(cut).Format("2006-01-02"); got != "2026-08-24" {
		t.Fatalf("got %s", got)
	}
}
func TestCapabilityPrecedenceAndEmptyCatalog(t *testing.T) {
	if got := finalCapability("ACTIVE", "ACCESSIBLE", "SUCCESS", "PENDING_ACCEPTANCE", "FULL", "AVAILABLE"); got != "PARTIAL" {
		t.Fatalf("semantic pending=%s", got)
	}
	if got := finalCapability("ACTIVE", "ACCESSIBLE", "FAILED", "VALIDATED", "FULL", "AVAILABLE"); got != "FAILED" {
		t.Fatalf("failed source=%s", got)
	}
}
func TestCoreSalesCapabilityEligibility(t *testing.T) {
	if !coreSalesCapabilityEligible(Capability{FinalState: "READY"}) ||
		!coreSalesCapabilityEligible(Capability{FinalState: "PARTIAL", ReasonCode: "SALES_RECONCILIATION_EXPLAINED_GAP"}) ||
		!coreSalesCapabilityEligible(Capability{FinalState: "PARTIAL", ReasonCode: "INSUFFICIENT_HISTORY"}) {
		t.Fatal("eligible Core Sales capability was blocked")
	}
	for _, c := range []Capability{
		{FinalState: "UNAVAILABLE"},
		{FinalState: "FAILED"},
		{FinalState: "PARTIAL", ReasonCode: "SOURCE_SEMANTICS_PENDING_ACCEPTANCE"},
	} {
		if coreSalesCapabilityEligible(c) {
			t.Fatalf("ineligible Core Sales capability accepted: %+v", c)
		}
	}
}
