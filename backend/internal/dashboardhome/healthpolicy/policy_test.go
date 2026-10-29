package healthpolicy

import (
	"math"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func f64(v float64) *float64 { return &v }

// factsWithStockRisk returns facts with a triggered key_sku_stock_risk
// candidate (2 critical-coverage key SKUs) plus a no_sales_with_stock
// candidate — used across several invariance tests below.
func factsWithStockRisk() dashboardfacts.DashboardFactsV1 {
	daysOfCover1 := 2.0
	daysOfCover2 := 3.0
	return dashboardfacts.DashboardFactsV1{
		SchemaVersion: dashboardfacts.SchemaVersion,
		MetricDate:    "2026-07-08",
		StockCoverage: dashboardfacts.StockCoverageFacts{Status: "available"},
		AdEfficiency:  dashboardfacts.AdEfficiencyFacts{Status: "available"},
		Funnel:        dashboardfacts.FunnelFacts{Status: "available"},
		DataCoverage:  dashboardfacts.DataCoverage{AccountMetricsDays: 7, StockSnapshotRows: 10},
		SKUFactsIndex: dashboardfacts.SKUFactsIndex{
			ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{
				"1": {OzonProductID: 1, DaysOfCover: &daysOfCover1, StockAvailable: 2, Orders7d: 5},
				"2": {OzonProductID: 2, DaysOfCover: &daysOfCover2, StockAvailable: 1, Orders7d: 3},
				"3": {OzonProductID: 3, StockAvailable: 20, Orders7d: 0, Orders: 0},
			},
		},
		KeySKUCohort: dashboardfacts.KeySKUCohortFacts{
			Status: "available",
			Items: []dashboardfacts.KeySKUCohortItem{
				{OzonProductID: 1},
				{OzonProductID: 2},
			},
		},
	}
}

func TestBuildCandidateRegistry_DeterministicForSameFacts(t *testing.T) {
	facts := factsWithStockRisk()
	a := BuildCandidateRegistry(facts)
	b := BuildCandidateRegistry(facts)
	scoreA := ComputeHealthScore(a).FinalScore
	scoreB := ComputeHealthScore(b).FinalScore
	if scoreA != scoreB {
		t.Fatalf("same facts produced different scores: %d vs %d", scoreA, scoreB)
	}
	if len(a) == 0 {
		t.Fatal("expected at least one candidate for facts with stock risk")
	}
}

// TestComputeHealthScore_IndependentOfAISelection is the core Wave 2
// invariant: health_score is always computed from the FULL backend
// candidate registry, never from a subset. Scoring the full registry vs. an
// arbitrarily AI-truncated subset must differ (proving the score does not
// silently key off whatever the AI happened to pick), while scoring the
// full registry twice (simulating two different AI outputs referencing the
// same facts) always agrees.
func TestComputeHealthScore_IndependentOfAISelection(t *testing.T) {
	facts := factsWithStockRisk()
	registry := BuildCandidateRegistry(facts)
	if len(registry) < 2 {
		t.Fatalf("expected >=2 candidates, got %d", len(registry))
	}

	scoreFull1 := ComputeHealthScore(registry).FinalScore
	// A second, differently-ordered/rebuilt registry from the SAME facts
	// (simulating a different AI run that only explains a different
	// candidate first) must produce the identical score.
	registryAgain := BuildCandidateRegistry(facts)
	scoreFull2 := ComputeHealthScore(registryAgain).FinalScore
	if scoreFull1 != scoreFull2 {
		t.Fatalf("expected identical score for identical facts, got %d vs %d", scoreFull1, scoreFull2)
	}

	// Scoring only the subset an AI might have chosen to explain must NOT be
	// how health_score is computed — the full registry must be used.
	subset := registry[:1]
	scoreSubset := ComputeHealthScore(subset).FinalScore
	if scoreSubset == scoreFull1 {
		t.Fatalf("scoring a proper subset should not usually match the full registry score in this fixture; got %d for both", scoreFull1)
	}
}

func TestComputeHealthScore_AIExplanationTextNeverAffectsScore(t *testing.T) {
	facts := factsWithStockRisk()
	registry := BuildCandidateRegistry(facts)
	breakdown := ComputeHealthScore(registry)

	// Overlaying different Title/Reason copy (as the AI would) must never
	// change Severity/ImpactScore/Penalty, and therefore never change score.
	drivers := ToRiskDrivers(registry)
	for i := range drivers {
		drivers[i].Title = "AI-authored title variant"
		drivers[i].Reason = "AI-authored reason variant, completely different wording."
	}
	relabeled := dashboardhome.ComputeHealthScoreBreakdown(drivers)
	if relabeled.FinalScore != breakdown.FinalScore {
		t.Fatalf("changing Title/Reason changed score: %d vs %d", relabeled.FinalScore, breakdown.FinalScore)
	}
}

func TestComputeHealthScore_BreakdownMatchesFinalScore(t *testing.T) {
	facts := factsWithStockRisk()
	breakdown := ComputeHealthScore(BuildCandidateRegistry(facts))
	if breakdown.FinalScore <= 0 {
		t.Fatalf("expected positive final score, got %d", breakdown.FinalScore)
	}
	if len(breakdown.Drivers) == 0 {
		t.Fatal("expected breakdown drivers to be populated")
	}
	saturated := 90 * (1 - math.Exp(-breakdown.RawPenalty/60))
	wantScore := int32(clampInt(int(math.Round(100-saturated)), 10, 95))
	// Growing cabinets without sales/revenue decline recover toward attention
	// (+22), matching computeHealthScoreBreakdown in dashboardhome.
	hasSalesDecline := false
	hasRevenueDecline := false
	for _, d := range breakdown.Drivers {
		if d.DriverID == "sales_decline" {
			hasSalesDecline = true
		}
		if d.DriverID == "revenue_decline" {
			hasRevenueDecline = true
		}
	}
	if !hasSalesDecline && !hasRevenueDecline {
		wantScore = int32(clampInt(int(wantScore)+22, 10, 95))
	}
	if breakdown.FinalScore != wantScore {
		t.Fatalf("breakdown does not match recomputed final score: got %d want %d", breakdown.FinalScore, wantScore)
	}
}

// TestComputeHealthScore_MissingAdvertisingDoesNotTankScore verifies that a
// facts snapshot whose ONLY signal is a missing/unavailable advertising
// source never scores worse than the same facts with advertising marked
// available (i.e. data_limitation never contributes to score).
func TestComputeHealthScore_MissingAdvertisingDoesNotTankScore(t *testing.T) {
	base := dashboardfacts.DashboardFactsV1{
		SchemaVersion: dashboardfacts.SchemaVersion,
		MetricDate:    "2026-07-08",
		StockCoverage: dashboardfacts.StockCoverageFacts{Status: "available"},
		Funnel:        dashboardfacts.FunnelFacts{Status: "available"},
		DataCoverage:  dashboardfacts.DataCoverage{AccountMetricsDays: 7},
	}
	withAds := base
	withAds.AdEfficiency = dashboardfacts.AdEfficiencyFacts{Status: "available"}
	withoutAds := base
	withoutAds.AdEfficiency = dashboardfacts.AdEfficiencyFacts{Status: "not_enough_data"}

	scoreWithAds := ComputeHealthScore(BuildCandidateRegistry(withAds)).FinalScore
	scoreWithoutAds := ComputeHealthScore(BuildCandidateRegistry(withoutAds)).FinalScore

	if scoreWithoutAds != scoreWithAds {
		t.Fatalf("missing optional advertising source changed score: with_ads=%d without_ads=%d", scoreWithAds, scoreWithoutAds)
	}
	if scoreWithoutAds != 88 {
		t.Fatalf("expected the healthy no-risk default score (88) when only data_limitation is present, got %d", scoreWithoutAds)
	}

	registry := BuildCandidateRegistry(withoutAds)
	foundDataLimitation := false
	for _, c := range registry {
		if c.DriverID == DriverDataLimitation {
			foundDataLimitation = true
			if c.ImpactScore != 0 {
				t.Fatalf("expected data_limitation to carry zero impact score, got %d", c.ImpactScore)
			}
		}
	}
	if !foundDataLimitation {
		t.Fatal("expected a data_limitation candidate when advertising is not_enough_data")
	}
}

func TestHasSufficientCoreData(t *testing.T) {
	empty := dashboardfacts.DashboardFactsV1{SchemaVersion: dashboardfacts.SchemaVersion, MetricDate: "2026-07-08"}
	if HasSufficientCoreData(empty) {
		t.Fatal("expected insufficient core data for a fully empty facts snapshot")
	}
	withCoverage := empty
	withCoverage.DataCoverage = dashboardfacts.DataCoverage{AccountMetricsDays: 7}
	if !HasSufficientCoreData(withCoverage) {
		t.Fatal("expected sufficient core data once account metrics coverage is present")
	}
}

func TestIsKnownDriverID(t *testing.T) {
	for _, id := range KnownDriverIDs {
		if !IsKnownDriverID(id) {
			t.Fatalf("expected %q to be a known driver id", id)
		}
	}
	if IsKnownDriverID("stock_deficit") {
		t.Fatal("expected an AI-invented id outside the registry to be unknown")
	}
	unknown := UnknownDriverIDs([]string{DriverStockout, "invented_driver", "", "another_invented"})
	if len(unknown) != 2 || unknown[0] != "invented_driver" || unknown[1] != "another_invented" {
		t.Fatalf("unexpected unknown ids: %#v", unknown)
	}
}

// TestComputePenalty_MatchesDashboardhomeFormula pins healthpolicy's
// duplicated severity/impact constants to dashboardhome's private
// computeHealthScoreBreakdown formula (accessed indirectly through the
// exported ComputeHealthScoreBreakdown with a single driver, where weight=1
// and the per-driver cap of 28 does not engage). If dashboardhome's table
// changes without updating healthpolicy, this test must fail.
func TestComputePenalty_MatchesDashboardhomeFormula(t *testing.T) {
	cases := []struct {
		severity    string
		impactScore int
	}{
		{"critical", 90},
		{"high", 53},
		{"medium", 26},
		{"low", 10},
	}
	for _, tc := range cases {
		driver := dashboardhome.RiskDriver{
			ID: "single", Category: "sales", Severity: tc.severity, ImpactScore: tc.impactScore,
			Title: "t", Reason: "r",
		}
		breakdown := dashboardhome.ComputeHealthScoreBreakdown([]dashboardhome.RiskDriver{driver})
		got := ComputePenalty(tc.severity, tc.impactScore)
		want := breakdown.Drivers[0].RawDriverPenalty // weight=1 at rank 0
		if math.Abs(got-want) > 0.01 {
			t.Fatalf("severity=%s impact=%d: ComputePenalty=%.4f dashboardhome_raw=%.4f", tc.severity, tc.impactScore, got, want)
		}
	}
}

func TestBuildCandidateRegistry_NoSKUSignalsAreNoOps(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{
		SchemaVersion: dashboardfacts.SchemaVersion,
		MetricDate:    "2026-07-08",
		StockCoverage: dashboardfacts.StockCoverageFacts{Status: "available"},
		AdEfficiency:  dashboardfacts.AdEfficiencyFacts{Status: "available"},
		Funnel:        dashboardfacts.FunnelFacts{Status: "available"},
		DataCoverage:  dashboardfacts.DataCoverage{AccountMetricsDays: 7},
	}
	registry := BuildCandidateRegistry(facts)
	if len(registry) != 0 {
		t.Fatalf("expected no candidates for a clean facts snapshot, got %#v", registry)
	}
	score := ComputeHealthScore(registry).FinalScore
	if score != 88 {
		t.Fatalf("expected healthy default score 88, got %d", score)
	}
}

func TestBuildRevenueDeclineCandidate(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{
		MetricDate: "2026-07-08",
		DashboardMetrics: analytics.DashboardMetricsDTO{
			Account: analytics.DashboardAccountMetrics{
				RevenueToday:     50000,
				RevenueDayDelta:  analytics.DashboardDelta{Pct: f64(-0.22)},
				RevenueWeekDelta: analytics.DashboardDelta{Pct: f64(-0.15)},
			},
		},
	}
	c, ok := buildRevenueDeclineCandidate(facts)
	if !ok {
		t.Fatal("expected revenue_decline candidate")
	}
	if c.DriverID != DriverRevenueDecline || c.Domain != "sales" {
		t.Fatalf("candidate=%#v", c)
	}
	if c.Severity == "" || c.ImpactScore <= 0 {
		t.Fatalf("candidate=%#v", c)
	}
}

func TestBuildSalesDeclineCandidate_RequiresPriorDayBaseline(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{
		DashboardMetrics: analytics.DashboardMetricsDTO{
			Account: analytics.DashboardAccountMetrics{OrdersToday: 5, OrdersYesterday: 0},
		},
	}
	if _, ok := buildSalesDeclineCandidate(facts); ok {
		t.Fatal("expected no sales_decline candidate without a prior-day baseline")
	}
}

func TestBuildStockoutCandidate_IgnoresNeverSellingSKUs(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{
		SKUFactsIndex: dashboardfacts.SKUFactsIndex{
			ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{
				"1": {OzonProductID: 1, StockAvailable: 0, Orders7d: 0},
			},
		},
	}
	if _, ok := buildStockoutCandidate(facts); ok {
		t.Fatal("expected no stockout candidate for a SKU with no recent sales")
	}
}

func TestBuildDataLimitationCandidate_NeverScored(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{
		AdEfficiency: dashboardfacts.AdEfficiencyFacts{Status: "source_not_available"},
		DataCoverage: dashboardfacts.DataCoverage{AccountMetricsDays: 7},
	}
	c, ok := buildDataLimitationCandidate(facts)
	if !ok {
		t.Fatal("expected data_limitation candidate")
	}
	scored := ScoringCandidates([]RiskCandidate{c})
	if len(scored) != 0 {
		t.Fatalf("expected data_limitation to be excluded from scoring, got %#v", scored)
	}
}

func TestDefaultTitleAndReason_CoverAllKnownDrivers(t *testing.T) {
	for _, id := range KnownDriverIDs {
		title := DefaultTitle(id)
		if title == "" || title == "Риск по кабинету" {
			t.Fatalf("driver %q should have a specific default title, got %q", id, title)
		}
		reason := DefaultReason(RiskCandidate{DriverID: id, ReasonTemplateData: map[string]any{}})
		if reason == "" {
			t.Fatalf("driver %q should have a non-empty default reason", id)
		}
	}
}
