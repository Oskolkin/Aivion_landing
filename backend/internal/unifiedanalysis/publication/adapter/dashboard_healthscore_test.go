package adapter

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/healthpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// TestProjectDashboard_SameFactsSameScore is the core Wave 2 invariant:
// health_score depends only on Dashboard Facts (via the backend risk
// candidate registry), never on the AI output. Two structurally very
// different AI outputs (a rich material_actions_found response and a
// minimal no_actions response) over the SAME facts must publish the exact
// same health_score/breakdown.
func TestProjectDashboard_SameFactsSameScore(t *testing.T) {
	facts := factsForValidFixture()

	material := readContractFixture(t, "unified_output_v1.valid.json")
	projMaterial, _ := ProjectDashboard(facts, material, 42, "2026-07-08")

	noActions := readContractFixture(t, "unified_output_v1.no_actions.json")
	projNoActions, _ := ProjectDashboard(facts, noActions, 42, "2026-07-08")

	if projMaterial.HealthScore == nil || projNoActions.HealthScore == nil {
		t.Fatalf("expected both projections to have a health score: material=%v no_actions=%v", projMaterial.HealthScore, projNoActions.HealthScore)
	}
	if *projMaterial.HealthScore != *projNoActions.HealthScore {
		t.Fatalf("same facts produced different scores for different AI outputs: material=%d no_actions=%d",
			*projMaterial.HealthScore, *projNoActions.HealthScore)
	}

	bdMaterial := projMaterial.Narrative.AccountHealth.HealthScoreBreakdown
	bdNoActions := projNoActions.Narrative.AccountHealth.HealthScoreBreakdown
	if bdMaterial == nil || bdNoActions == nil {
		t.Fatal("expected both projections to carry a health_score_breakdown")
	}
	if bdMaterial.FinalScore != bdNoActions.FinalScore || bdMaterial.RawPenalty != bdNoActions.RawPenalty {
		t.Fatalf("breakdowns differ despite identical facts: %+v vs %+v", bdMaterial, bdNoActions)
	}
}

// TestProjectDashboard_DifferentAIDriverSelectionSameScore verifies that
// which risk_drivers the AI chooses to report (a different subset/order,
// or none at all) never changes health_score for the same facts.
func TestProjectDashboard_DifferentAIDriverSelectionSameScore(t *testing.T) {
	facts := factsForValidFixture()
	out := readContractFixture(t, "unified_output_v1.valid.json")

	baseline, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	// AI selects a completely different (still-unknown, to keep the fixture
	// simple) subset/order of risk_drivers.
	out.Dashboard.AccountSummary.RiskDrivers = []contract.UnifiedRiskDriverV1{
		{ID: "some_other_ai_id", Category: "advertising", Severity: "low", ImpactScore: 5, Title: "T1", Reason: "R1"},
	}
	altSelection, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	// AI reports zero risk_drivers at all.
	out.Dashboard.AccountSummary.RiskDrivers = nil
	noSelection, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	if baseline.HealthScore == nil || altSelection.HealthScore == nil || noSelection.HealthScore == nil {
		t.Fatalf("expected non-nil scores: baseline=%v alt=%v none=%v", baseline.HealthScore, altSelection.HealthScore, noSelection.HealthScore)
	}
	if *baseline.HealthScore != *altSelection.HealthScore || *baseline.HealthScore != *noSelection.HealthScore {
		t.Fatalf("driver selection changed health_score: baseline=%d alt=%d none=%d",
			*baseline.HealthScore, *altSelection.HealthScore, *noSelection.HealthScore)
	}
}

// TestProjectDashboard_AIExplanationTextDoesNotChangeScore verifies that
// the AI may freely rewrite Title/Reason copy for a registry driver it
// references without moving health_score, while the rewritten copy is still
// what gets displayed.
func TestProjectDashboard_AIExplanationTextDoesNotChangeScore(t *testing.T) {
	facts := factsForValidFixture()
	out := readContractFixture(t, "unified_output_v1.valid.json")

	// Reference a real registry driver_id for this fixture's facts
	// (key_sku_stock_risk — see TestProjectDashboard_MaterialActionsFixture)
	// with one piece of copy.
	out.Dashboard.AccountSummary.RiskDrivers = []contract.UnifiedRiskDriverV1{
		{ID: healthpolicy.DriverKeySKUStockRisk, Category: "stock", Severity: "high", ImpactScore: 36, Title: "Первый вариант заголовка", Reason: "Первый вариант объяснения."},
	}
	first, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	out.Dashboard.AccountSummary.RiskDrivers[0].Title = "Совершенно другой заголовок про остатки"
	out.Dashboard.AccountSummary.RiskDrivers[0].Reason = "Полностью переписанное объяснение с другими словами."
	second, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	if first.HealthScore == nil || second.HealthScore == nil || *first.HealthScore != *second.HealthScore {
		t.Fatalf("rewriting AI explanation text changed health_score: first=%v second=%v", first.HealthScore, second.HealthScore)
	}

	view := findRiskDriverView(t, second.Narrative.AccountHealth.RiskDrivers, healthpolicy.DriverKeySKUStockRisk)
	if view.Title != "Совершенно другой заголовок про остатки" {
		t.Fatalf("expected AI title overlay to be displayed, got %q", view.Title)
	}
	if view.Reason != "Полностью переписанное объяснение с другими словами." {
		t.Fatalf("expected AI reason overlay to be displayed, got %q", view.Reason)
	}
	// Severity/impact must stay backend-owned regardless of the AI's copy.
	if view.Severity != "high" || view.ImpactScore != 36 {
		t.Fatalf("AI overlay must not change severity/impact_score, got severity=%q impact=%d", view.Severity, view.ImpactScore)
	}
}

// TestProjectDashboard_BreakdownAlwaysMatchesScore verifies FinalScore in
// the persisted breakdown always equals the top-level HealthScore.
func TestProjectDashboard_BreakdownAlwaysMatchesScore(t *testing.T) {
	facts := factsForValidFixture()
	out := readContractFixture(t, "unified_output_v1.valid.json")
	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	if proj.HealthScore == nil {
		t.Fatal("expected a health score")
	}
	bd := proj.Narrative.AccountHealth.HealthScoreBreakdown
	if bd == nil {
		t.Fatal("expected a health_score_breakdown")
	}
	if bd.FinalScore != *proj.HealthScore {
		t.Fatalf("breakdown.final_score=%d does not match HealthScore=%d", bd.FinalScore, *proj.HealthScore)
	}
}

// TestProjectDashboard_MissingAdvertisingDoesNotTankScore verifies that a
// seller whose ONLY signal is "advertising data unavailable" scores exactly
// like one where advertising is simply not present as a source at all
// (i.e. the backend never treats "no ads source" as a business failure).
func TestProjectDashboard_MissingAdvertisingDoesNotTankScore(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.no_actions.json")

	withAds := dashboardfacts.DashboardFactsV1{
		SchemaVersion:   dashboardfacts.SchemaVersion,
		SellerAccountID: 55,
		MetricDate:      "2026-07-08",
		StockCoverage:   dashboardfacts.StockCoverageFacts{Status: "available"},
		AdEfficiency:    dashboardfacts.AdEfficiencyFacts{Status: "available"},
		Funnel:          dashboardfacts.FunnelFacts{Status: "available"},
		DataCoverage:    dashboardfacts.DataCoverage{AccountMetricsDays: 7},
	}
	withoutAds := withAds
	withoutAds.AdEfficiency = dashboardfacts.AdEfficiencyFacts{Status: "source_not_available"}

	projWithAds, _ := ProjectDashboard(withAds, out, 55, "2026-07-08")
	projWithoutAds, _ := ProjectDashboard(withoutAds, out, 55, "2026-07-08")

	if projWithAds.HealthScore == nil || projWithoutAds.HealthScore == nil {
		t.Fatalf("expected non-nil scores: with_ads=%v without_ads=%v", projWithAds.HealthScore, projWithoutAds.HealthScore)
	}
	if *projWithAds.HealthScore != *projWithoutAds.HealthScore {
		t.Fatalf("missing advertising source changed health_score: with_ads=%d without_ads=%d",
			*projWithAds.HealthScore, *projWithoutAds.HealthScore)
	}
}

// TestProjectDashboard_InsufficientCoreDataScoreUnavailable verifies that
// when Dashboard Facts carry no core coverage at all, health_score stays
// nil (unavailable) rather than defaulting to the "healthy, no risks found"
// score — this matches the pre-existing pattern exercised by
// TestProjectDashboard_InsufficientDataFixture.
func TestProjectDashboard_InsufficientCoreDataScoreUnavailable(t *testing.T) {
	facts := dashboardfacts.DashboardFactsV1{
		SchemaVersion:   dashboardfacts.SchemaVersion,
		SellerAccountID: 9,
		MetricDate:      "2026-07-08",
	}
	if healthpolicy.HasSufficientCoreData(facts) {
		t.Fatal("test fixture should have insufficient core data")
	}
	out := readContractFixture(t, "unified_output_v1.insufficient_data.json")

	proj, _ := ProjectDashboard(facts, out, 9, "2026-07-08")
	if proj.HealthScore != nil {
		t.Fatalf("expected nil health score for insufficient core data, got %v", *proj.HealthScore)
	}
}

func findRiskDriverView(t *testing.T, views []NarrativeRiskDriverView, id string) NarrativeRiskDriverView {
	t.Helper()
	for _, v := range views {
		if v.ID == id {
			return v
		}
	}
	t.Fatalf("expected risk driver view %q, got %+v", id, views)
	return NarrativeRiskDriverView{}
}
