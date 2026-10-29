package readiness

import (
	"testing"
	"time"
)

func prompt98Cutoff() time.Time {
	return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
}

func prompt98Day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func prompt98Ptr(s string) *time.Time {
	d := prompt98Day(s)
	return &d
}

func prompt98CoreCaps(state, reason string) (Capability, Capability, Capability) {
	cat := Capability{Key: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "READY", SemanticState: "VALIDATED"}
	sales := Capability{Key: "SALES.ACCOUNT_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: state, ReasonCode: reason, SemanticState: "VALIDATED"}
	sku := Capability{Key: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: state, ReasonCode: reason, SemanticState: "VALIDATED"}
	return cat, sales, sku
}

func prompt98Stats(days []string, mature map[string]bool, recon map[string]string) evidenceStats {
	sales := map[string]bool{}
	for _, d := range days {
		sales[d] = true
	}
	max := prompt98Day(days[len(days)-1])
	return evidenceStats{
		skuCount:       1,
		salesDays:      len(days),
		salesDates:     sales,
		matureDates:    mature,
		reconciliation: recon,
		maxAccountDate: &max,
	}
}

func TestZeroSalesHistoryStillBlocksAnalysis(t *testing.T) {
	stop := analysisDateHardStop(evidenceStats{skuCount: 1, salesDays: 0})
	if stop == nil || stop.Status != GateInsufficient || len(stop.BlockingReasons) != 1 || stop.BlockingReasons[0] != "NO_SALES_HISTORY" {
		t.Fatalf("empty sales truth must block: %+v", stop)
	}
}

func TestEmptySKUUniverseStillBlocksAnalysis(t *testing.T) {
	stop := analysisDateHardStop(evidenceStats{skuCount: 0, maxAccountDate: prompt98Ptr("2026-08-24")})
	if stop == nil || stop.Status != GateEmpty || stop.BlockingReasons[0] != "EMPTY_SKU_UNIVERSE" {
		t.Fatalf("empty catalog must block: %+v", stop)
	}
}

func TestKnownZeroSalesIsNotMissingHistory(t *testing.T) {
	// A present mature PASS day with business-zero sales is history, not absence.
	st := prompt98Stats([]string{"2026-08-24"}, map[string]bool{"2026-08-24": true}, map[string]string{"2026-08-24": "PASS"})
	if analysisDateHardStop(st) != nil {
		t.Fatal("known sales day must not trip NO_SALES_HISTORY")
	}
	cat, account, sku := prompt98CoreCaps("PARTIAL", "INSUFFICIENT_HISTORY")
	_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected == nil || selected.Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("known-zero day must still be selectable, got %v", selected)
	}
}

func TestShortHistoryDoesNotBlockAnalysisDate(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"1 day", 1},
		{"6 days", 6},
		{"7 days", 7},
		{"13 days", 13},
		{"14 days", 14},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			days := make([]string, 0, tc.n)
			mature := map[string]bool{}
			recon := map[string]string{}
			for i := 0; i < tc.n; i++ {
				d := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i-tc.n+1).Format("2006-01-02")
				days = append(days, d)
				mature[d] = true
				recon[d] = "PASS"
			}
			st := prompt98Stats(days, mature, recon)
			reason := "INSUFFICIENT_HISTORY"
			state := "PARTIAL"
			if tc.n >= 14 {
				state, reason = "READY", ""
			}
			cat, account, sku := prompt98CoreCaps(state, reason)
			_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
			if selected == nil || selected.Format("2006-01-02") != "2026-08-24" {
				t.Fatalf("history depth %d blocked analysis_date: %v", tc.n, selected)
			}
		})
	}
}

func TestCandidateDateStillRequiresMaturity(t *testing.T) {
	st := prompt98Stats([]string{"2026-08-24"}, map[string]bool{}, map[string]string{"2026-08-24": "PASS"})
	cat, account, sku := prompt98CoreCaps("PARTIAL", "INSUFFICIENT_HISTORY")
	out, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected != nil {
		t.Fatal("immature sole candidate must not be selected")
	}
	if len(out) != 1 || out[0].Status != GateBlocked {
		t.Fatalf("expected blocked immature candidate, got %+v", out)
	}
	found := false
	for _, r := range out[0].BlockingReasons {
		if r == "SALES_NOT_MATURE:2026-08-24" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing maturity reason: %+v", out[0].BlockingReasons)
	}
}

func TestCandidateDateStillRequiresReconciliation(t *testing.T) {
	st := prompt98Stats([]string{"2026-08-24"}, map[string]bool{"2026-08-24": true}, map[string]string{"2026-08-24": "NOT_COMPARABLE"})
	cat, account, sku := prompt98CoreCaps("PARTIAL", "INSUFFICIENT_HISTORY")
	_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected != nil {
		t.Fatal("NOT_COMPARABLE candidate must not be selected")
	}
}

func TestResolverFallsBackFromImmatureCandidate(t *testing.T) {
	st := prompt98Stats(
		[]string{"2026-08-23", "2026-08-24"},
		map[string]bool{"2026-08-23": true},
		map[string]string{"2026-08-23": "PASS", "2026-08-24": "PASS"},
	)
	cat, account, sku := prompt98CoreCaps("PARTIAL", "INSUFFICIENT_HISTORY")
	_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected == nil || selected.Format("2006-01-02") != "2026-08-23" {
		t.Fatalf("want fallback to mature D-1, got %v", selected)
	}
}

func TestResolverFallsBackFromUnexplainedGap(t *testing.T) {
	st := prompt98Stats(
		[]string{"2026-08-23", "2026-08-24"},
		map[string]bool{"2026-08-23": true, "2026-08-24": true},
		map[string]string{"2026-08-23": "PASS", "2026-08-24": "UNEXPLAINED_GAP"},
	)
	cat, account, sku := prompt98CoreCaps("PARTIAL", "INSUFFICIENT_HISTORY")
	_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected == nil || selected.Format("2006-01-02") != "2026-08-23" {
		t.Fatalf("want fallback from UNEXPLAINED_GAP, got %v", selected)
	}
}

func TestExplainedGapDoesNotBlockCandidateDay(t *testing.T) {
	st := prompt98Stats(
		[]string{"2026-08-24"},
		map[string]bool{"2026-08-24": true},
		map[string]string{"2026-08-24": "EXPLAINED_GAP"},
	)
	cat, account, sku := prompt98CoreCaps("PARTIAL", "SALES_RECONCILIATION_EXPLAINED_GAP")
	_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected == nil || selected.Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("EXPLAINED_GAP candidate must remain selectable, got %v", selected)
	}
}

func TestHistoricalHoleDoesNotInvalidateReadyCandidateDay(t *testing.T) {
	days := make([]string, 0, 30)
	mature := map[string]bool{}
	recon := map[string]string{}
	for i := 0; i < 30; i++ {
		d := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format("2006-01-02")
		if d == "2026-08-21" {
			continue // hole at D-3
		}
		days = append(days, d)
		mature[d] = true
		recon[d] = "PASS"
	}
	st := prompt98Stats(days, mature, recon)
	st.missingSalesDays = 1
	cat, account, sku := prompt98CoreCaps("READY", "")
	_, selected := selectAnalysisDateCandidates(prompt98Cutoff(), st, cat, account, sku)
	if selected == nil || selected.Format("2006-01-02") != "2026-08-24" {
		t.Fatalf("historical hole must not invalidate ready D, got %v", selected)
	}
}

func TestPrompt98HistoryDQDoesNotBlockAnalysisDate(t *testing.T) {
	ds := []domainEvidence{{domain: "CATALOG"}, {domain: "SALES_DEMAND"}, {domain: "POSTINGS"}, {domain: "RETURNS"}}
	st := evidenceStats{
		skuCount:         1,
		salesDays:        6,
		missingSalesDays: 2,
		matureDates:      map[string]bool{"2026-08-24": true},
		salesDates:       map[string]bool{"2026-08-24": true},
		reconciliation:   map[string]string{},
	}
	out := (*Service)(nil).issues(ds, st, prompt98Cutoff())
	want := map[string]string{
		"SALES_HISTORY_INSUFFICIENT":  "DEGRADE_TO_PARTIAL",
		"SALES_MATURITY_INSUFFICIENT": "DEGRADE_TO_PARTIAL",
		"SALES_DAILY_COVERAGE_HOLE":   "DEGRADE_TO_PARTIAL",
	}
	found := map[string]string{}
	for _, issue := range out {
		if _, ok := want[issue.Code]; ok {
			found[issue.Code] = issue.ReadinessEffect
			if issue.ReadinessEffect == "BLOCK_ANALYSIS_DATE" {
				t.Fatalf("%s still blocks analysis date", issue.Code)
			}
		}
	}
	for code, effect := range want {
		if found[code] != effect {
			t.Fatalf("%s effect=%q want %q", code, found[code], effect)
		}
	}
}
