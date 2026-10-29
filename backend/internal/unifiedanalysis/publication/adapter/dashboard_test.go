package adapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/healthpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func readContractFixture(t *testing.T, name string) *contract.UnifiedAnalysisOutputV1 {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "contract", "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	return &out
}

func i64(v int64) *int64     { return &v }
func i32(v int32) *int32     { return &v }
func f64(v float64) *float64 { return &v }
func str(v string) *string   { return &v }

// skuFactRecord is a small helper constructor for dashboardfacts.SKUFactRecord
// test fixtures.
func skuFactRecord(ozonProductID int64, sku int64, offerID, productName string, revenue float64, orders, stock int32, daysOfCover float64) dashboardfacts.SKUFactRecord {
	rec := dashboardfacts.SKUFactRecord{
		OzonProductID:  ozonProductID,
		SKU:            i64(sku),
		ProductName:    str(productName),
		Revenue:        revenue,
		Orders:         orders,
		Revenue7d:      revenue,
		Orders7d:       int64(orders),
		StockAvailable: stock,
		DaysOfCover:    f64(daysOfCover),
		// Default multi-day sparkline so focus cards pass the ≥2 observed-points
		// publish contract used by bundlevalidator.checkSKUTrend.
		Sparkline7D: []dashboardfacts.SparklinePoint{
			{Date: "2026-07-07", Value: f64(float64(orders)), Revenue: f64(revenue * 0.9), HasData: true},
			{Date: "2026-07-08", Value: f64(float64(orders)), Revenue: f64(revenue), HasData: true},
		},
	}
	if offerID != "" {
		rec.OfferID = str(offerID)
	}
	return rec
}

// factsForValidFixture matches the SKUs/keys referenced by
// unified_output_v1.valid.json so ProjectDashboard's facts join succeeds.
func factsForValidFixture() dashboardfacts.DashboardFactsV1 {
	rec1 := skuFactRecord(1, 801000100001, "offer-1", "SKU One", 12000, 10, 5, 3)
	rec2 := skuFactRecord(2, 801000100002, "offer-2", "SKU Two", 8000, 5, 2, 2)
	rec3 := skuFactRecord(3, 801000100003, "", "SKU Three", 0, 0, 20, 90)
	rec4 := skuFactRecord(4, 801000100004, "", "SKU Four", 100, 1, 50, 120)
	// 12000+8000+100 = 20100; cumulative reaches ≥80% after SKU One + SKU Two.
	cohort := dashboardfacts.KeySKUCohortFacts{
		Status:                    "available",
		StoreTotalRevenue7d:       20100,
		KeySKUsTotalRevenue7d:     20000,
		CumulativeRevenueSharePct: 99.5,
		ThresholdPct:              dashboardfacts.KeySKUCohortThresholdPct,
		PolicyVersion:             dashboardfacts.KeySKUCohortPolicyVersion,
		Items: []dashboardfacts.KeySKUCohortItem{
			{
				OzonProductID: 1, SKU: rec1.SKU, OfferID: rec1.OfferID, ProductName: rec1.ProductName,
				Revenue7d: 12000, Orders7d: 10, RevenueSharePct: 59.7, CumulativeRevenueSharePct: 59.7, Rank: 1,
			},
			{
				OzonProductID: 2, SKU: rec2.SKU, OfferID: rec2.OfferID, ProductName: rec2.ProductName,
				Revenue7d: 8000, Orders7d: 5, RevenueSharePct: 39.8, CumulativeRevenueSharePct: 99.5, Rank: 2,
			},
		},
	}
	return dashboardfacts.DashboardFactsV1{
		SchemaVersion:   dashboardfacts.SchemaVersion,
		SellerAccountID: 42,
		MetricDate:      "2026-07-08",
		SellerPulse: dashboardfacts.SellerPulseFacts{
			KPIs: []dashboardfacts.KPICardFact{
				{Key: "orders", Label: "Заказы", Value: "120"},
				{Key: "revenue", Label: "Выручка", Value: "430 000 ₽"},
			},
			Trend7D: []dashboardfacts.TrendDayPoint{
				{Date: "2026-07-08", Revenue: f64(66000), Orders: i32(15)},
			},
		},
		StockCoverage: dashboardfacts.StockCoverageFacts{
			Status:    "available",
			TotalSKUs: 12,
			Segments: []dashboardfacts.StockCoverageSegment{
				{Key: "deficit", Label: "Дефицит", SKUCount: 3, Severity: "critical"},
			},
		},
		AdEfficiency: dashboardfacts.AdEfficiencyFacts{
			Status: "available",
			Summary: &dashboardfacts.AdEfficiencySummary{
				CurrentDRRPct:     f64(28.0),
				TotalSpendToday:   12000,
				TotalRevenueToday: 43000,
				TotalOrdersToday:  18,
			},
		},
		Funnel: dashboardfacts.FunnelFacts{
			Status: "available",
			Stages: []dashboardfacts.FunnelStage{
				{Key: "impressions", Label: "Показы", Value: i64(50000)},
				{Key: "orders", Label: "Заказы", Value: i64(120)},
			},
		},
		SKUFactsIndex: dashboardfacts.SKUFactsIndex{
			ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{
				"1": rec1, "2": rec2, "3": rec3, "4": rec4,
			},
		},
		KeySKUCohort: cohort,
		DataCoverage: dashboardfacts.DataCoverage{
			AccountMetricsDays: 7,
			FunnelAvailable:    true,
			AdMetricsDays:      7,
			StockSnapshotRows:  12,
		},
	}
}

func TestProjectDashboard_MaterialActionsFixture(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()

	proj, warnings := ProjectDashboard(facts, out, 42, "2026-07-08")
	foundHealthMismatch := false
	for _, w := range warnings {
		switch {
		case w.Field == "account_summary.health_score" && w.Reason == "backend_score_overrides_ai":
			foundHealthMismatch = true
		case w.Field == "account_summary.risk_drivers" && w.Reason == "ai_risk_driver_unknown_id":
			t.Fatalf("canonical fixture driver must not be unknown: %+v", w)
		default:
			t.Fatalf("unexpected mismatch warning: %+v", w)
		}
	}
	if !foundHealthMismatch {
		t.Fatal("expected backend_score_overrides_ai warning for AI health_score 62")
	}

	if proj.Frozen.MetricDate != "2026-07-08" {
		t.Fatalf("expected metric date 2026-07-08, got %q", proj.Frozen.MetricDate)
	}
	if proj.Frozen.DashboardMetrics.SellerAccountID != 42 {
		t.Fatalf("expected seller account id 42, got %d", proj.Frozen.DashboardMetrics.SellerAccountID)
	}
	// The score is now grounded purely in the backend candidate registry
	// built from facts (key_sku_stock_risk + no_sales_with_stock in this
	// fixture) — independent of the AI's own (unknown-id) risk_drivers.
	if proj.HealthScore == nil || *proj.HealthScore != 85 {
		got := any(nil)
		if proj.HealthScore != nil {
			got = *proj.HealthScore
		}
		t.Fatalf("expected backend health score 85 from the facts-grounded registry, got %v", got)
	}
	if proj.Narrative.AccountHealth == nil || proj.Narrative.AccountHealth.HealthScoreBreakdown == nil {
		t.Fatal("expected health_score_breakdown on account health")
	}
	if proj.Narrative.AccountHealth.HealthScore == nil || *proj.Narrative.AccountHealth.HealthScore != 85 {
		t.Fatalf("expected account health score 85, got %v", proj.Narrative.AccountHealth.HealthScore)
	}
	if proj.Narrative.AccountHealth.HealthPolicyVersion != healthpolicy.HealthPolicyVersion {
		t.Fatalf("expected health_policy_version %q, got %q", healthpolicy.HealthPolicyVersion, proj.Narrative.AccountHealth.HealthPolicyVersion)
	}
	if proj.Narrative.AccountHealth.HealthFactsHash != facts.FactsHash {
		t.Fatalf("expected health_facts_hash to match facts.FactsHash %q, got %q", facts.FactsHash, proj.Narrative.AccountHealth.HealthFactsHash)
	}
	// Registry-derived risk_drivers must only ever surface known backend
	// driver ids (fixture AI id is now the canonical key_sku_stock_risk).
	for _, d := range proj.Narrative.AccountHealth.RiskDrivers {
		if !healthpolicy.IsKnownDriverID(d.ID) {
			t.Fatalf("displayed risk driver %q is not a known backend driver id", d.ID)
		}
	}
	if proj.Severity != "low" {
		t.Fatalf("expected mapped severity low (from score band), got %q", proj.Severity)
	}
	if proj.Narrative.Headline == "" {
		t.Fatal("expected non-empty narrative headline")
	}
	if len(proj.Narrative.HealthReasons) != 1 {
		t.Fatalf("expected 1 health reason, got %d", len(proj.Narrative.HealthReasons))
	}
	if len(proj.Narrative.TopActions) != 3 {
		t.Fatalf("expected 3 top actions (a1,a2,a3), got %d", len(proj.Narrative.TopActions))
	}
	if proj.Narrative.TopActions[0].DriverID != "a1" {
		t.Fatalf("expected first top action to be a1, got %q", proj.Narrative.TopActions[0].DriverID)
	}

	// Facts always win for the seller_pulse KPIs, regardless of what the AI
	// output says.
	if len(proj.Frozen.SellerPulse.KPIs) != 2 {
		t.Fatalf("expected 2 seller_pulse KPIs from facts, got %d", len(proj.Frozen.SellerPulse.KPIs))
	}
	foundRevenueKPI := false
	for _, kpi := range proj.Frozen.SellerPulse.KPIs {
		if kpi.Key == "revenue" {
			foundRevenueKPI = true
			if kpi.Value != "430 000 ₽" {
				t.Fatalf("expected revenue KPI value from facts, got %q", kpi.Value)
			}
		}
	}
	if !foundRevenueKPI {
		t.Fatal("expected a revenue KPI sourced from facts")
	}

	// SKU focus: AI selects entities, facts supply factual fields; the
	// selected SKU must resolve against facts.SKUFactsIndex.
	if len(proj.Frozen.SKUFocus.OrderDrop) != 1 {
		t.Fatalf("expected 1 order_drop sku, got %d", len(proj.Frozen.SKUFocus.OrderDrop))
	}
	orderDrop := proj.Frozen.SKUFocus.OrderDrop[0]
	if orderDrop.OzonProductID != 1 {
		t.Fatalf("expected order_drop joined to ozon_product_id 1, got %d", orderDrop.OzonProductID)
	}
	if orderDrop.CurrentStock == nil || *orderDrop.CurrentStock != 5 {
		t.Fatalf("expected order_drop current_stock from facts (5), got %v", orderDrop.CurrentStock)
	}
	if orderDrop.Problem != "Падение заказов" {
		t.Fatalf("expected AI-owned problem text preserved, got %q", orderDrop.Problem)
	}

	if len(proj.Narrative.DataQualityNotes) != 2 {
		t.Fatalf("expected 2 data quality notes (from limitations), got %d", len(proj.Narrative.DataQualityNotes))
	}

	// Must round-trip through JSON cleanly since this is what gets persisted
	// into ai_dashboard_narratives.validated_payload.
	if _, err := json.Marshal(proj.Narrative); err != nil {
		t.Fatalf("marshal narrative payload: %v", err)
	}
	if _, err := json.Marshal(proj.Frozen); err != nil {
		t.Fatalf("marshal frozen dashboard data: %v", err)
	}
}

func TestProjectDashboard_AdEfficiencyChecklistProjected(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	narrative := proj.Frozen.AdEfficiency.Narrative
	if narrative == nil {
		t.Fatal("expected ad_efficiency narrative to be projected")
	}
	if len(narrative.Checklist) != 2 {
		t.Fatalf("expected 2 checklist items copied from AI output, got %d", len(narrative.Checklist))
	}
	if narrative.Checklist[0].Title == "" || narrative.Checklist[0].Text == "" {
		t.Fatalf("expected checklist item title/text to be copied, got %+v", narrative.Checklist[0])
	}
	if narrative.Checklist[0].Target != "high_drr_campaigns" {
		t.Fatalf("expected checklist item target copied, got %q", narrative.Checklist[0].Target)
	}
	// The narrative payload (persisted separately) must carry the same
	// checklist, since it's the same pointer projected onto both.
	if proj.Narrative.AdEfficiencyNarrative == nil || len(proj.Narrative.AdEfficiencyNarrative.Checklist) != 2 {
		t.Fatalf("expected narrative payload to carry the checklist, got %+v", proj.Narrative.AdEfficiencyNarrative)
	}
}

func TestProjectDashboard_AdEfficiencyClaimsPreserved(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()
	storeShare := 16.8
	attr := 91.8
	out.Dashboard.AdEfficiency.Narrative.MetricClaims = []contract.UnifiedMetricClaimV1{
		{
			FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_7d",
			MetricCode: "ad_spend_share_of_total_store_revenue_pct", Value: &storeShare,
			PeriodType: "current_7d", PeriodStart: "2026-07-02", PeriodEnd: "2026-07-08",
		},
		{
			FactID:     "account:attributed_campaign_drr_pct:current_7d",
			MetricCode: "attributed_campaign_drr_pct", Value: &attr,
			PeriodType: "current_7d", PeriodStart: "2026-07-02", PeriodEnd: "2026-07-08",
		},
	}
	out.Dashboard.AdEfficiency.Narrative.ThresholdClaims = []contract.UnifiedThresholdClaimV1{{
		PolicyID: "high_drr_campaign_threshold", MetricCode: "campaign_drr_pct",
		Scope: "campaign", Operator: "gt", Value: 80,
	}}

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	n := proj.Frozen.AdEfficiency.Narrative
	if n == nil {
		t.Fatal("expected narrative")
	}
	if len(n.MetricClaims) != 2 {
		t.Fatalf("metric_claims dropped in projection: got %d", len(n.MetricClaims))
	}
	if len(n.ThresholdClaims) != 1 {
		t.Fatalf("threshold_claims dropped in projection: got %d", len(n.ThresholdClaims))
	}
	if n.MetricClaims[0].MetricCode != "ad_spend_share_of_total_store_revenue_pct" {
		t.Fatalf("unexpected claim: %+v", n.MetricClaims[0])
	}
	if proj.Narrative.AdEfficiencyNarrative == nil || len(proj.Narrative.AdEfficiencyNarrative.MetricClaims) != 2 {
		t.Fatal("narrative payload must retain metric_claims for audit/bundle validation")
	}
}

func TestProjectDashboard_AdEfficiencyChecklistEmptyWhenAINarrativeOmitsIt(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.AdEfficiency.Narrative.Checklist = nil
	facts := factsForValidFixture()

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	narrative := proj.Frozen.AdEfficiency.Narrative
	if narrative == nil {
		t.Fatal("expected ad_efficiency narrative to be projected")
	}
	if len(narrative.Checklist) != 0 {
		t.Fatalf("expected no checklist items when AI output has none, got %+v", narrative.Checklist)
	}
}

// TestProjectDashboard_AdEfficiencyNarrativeNeverRewritten locks in the
// Wave 2 dashboard audit outcome: projectAdEfficiencyNarrative is pure
// passthrough of the AI headline/summary/checklist/limitations text.
// Claim-based grounding (outputvalidator) blocks publication on a
// percentage/denominator mismatch instead — the adapter must never
// silently rewrite the text to match backend facts, even when the AI's
// text and dashboardfacts.AdEfficiency.Summary genuinely disagree (as they
// do here: AI text cites 16,8%/91,8% while facts.Summary.CurrentDRRPct is
// 28.0, matching the old ground-test fixture's deliberate mismatch).
func TestProjectDashboard_AdEfficiencyNarrativeNeverRewritten(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	aiHeadline := out.Dashboard.AdEfficiency.Narrative.Headline
	aiSummary := out.Dashboard.AdEfficiency.Narrative.Summary
	if aiHeadline == "" || aiSummary == "" {
		t.Fatal("fixture must carry non-empty AI headline/summary to exercise no-rewrite behavior")
	}
	facts := factsForValidFixture()
	if facts.AdEfficiency.Summary == nil || facts.AdEfficiency.Summary.CurrentDRRPct == nil {
		t.Fatal("fixture facts must carry a conflicting current_drr_pct to exercise no-rewrite behavior")
	}

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")

	narrative := proj.Frozen.AdEfficiency.Narrative
	if narrative == nil {
		t.Fatal("expected ad_efficiency narrative to be projected")
	}
	if narrative.Headline != aiHeadline {
		t.Fatalf("headline was rewritten: got %q want %q", narrative.Headline, aiHeadline)
	}
	if narrative.Summary != aiSummary {
		t.Fatalf("summary was rewritten: got %q want %q", narrative.Summary, aiSummary)
	}
	if proj.Narrative.AdEfficiencyNarrative == nil ||
		proj.Narrative.AdEfficiencyNarrative.Headline != aiHeadline ||
		proj.Narrative.AdEfficiencyNarrative.Summary != aiSummary {
		t.Fatalf("narrative payload copy was rewritten: %+v", proj.Narrative.AdEfficiencyNarrative)
	}
}

func TestProjectDashboard_NoActionsFixture(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.no_actions.json")
	facts := factsForValidFixture()
	facts.SellerAccountID = 7

	proj, _ := ProjectDashboard(facts, out, 7, "2026-07-08")

	if len(proj.Narrative.TopActions) != 0 {
		t.Fatalf("expected no top actions, got %d", len(proj.Narrative.TopActions))
	}
	// Same facts as TestProjectDashboard_MaterialActionsFixture (a
	// richly-detailed material_actions_found AI output) -> the score must be
	// identical here even though this fixture's AI output is a minimal
	// no_actions response with different (or no) risk_drivers. This is the
	// Wave 2 invariant: health_score depends only on facts, never on the AI's
	// driver selection.
	if proj.HealthScore == nil || *proj.HealthScore != 85 {
		got := int32(-1)
		if proj.HealthScore != nil {
			got = *proj.HealthScore
		}
		t.Fatalf("expected backend health score 85 (facts-grounded registry, same as the material_actions fixture), got %d", got)
	}
	if proj.Narrative.AccountHealth == nil {
		t.Fatal("expected account health to still be set for a healthy account")
	}
}

func TestProjectDashboard_InsufficientDataFixture(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.insufficient_data.json")
	facts := dashboardfacts.DashboardFactsV1{
		SchemaVersion:   dashboardfacts.SchemaVersion,
		SellerAccountID: 9,
		MetricDate:      "2026-07-08",
	}

	proj, _ := ProjectDashboard(facts, out, 9, "2026-07-08")

	if proj.HealthScore != nil {
		t.Fatalf("expected nil health score for insufficient_data fixture, got %v", *proj.HealthScore)
	}
	// account_summary.severity is empty and health_score is nil in this
	// fixture, so no AccountHealth block should be synthesized.
	if proj.Narrative.AccountHealth != nil {
		t.Fatalf("expected no account health block, got %+v", proj.Narrative.AccountHealth)
	}
	if len(proj.Narrative.DataQualityNotes) == 0 {
		t.Fatal("expected data quality notes to be populated from limitations")
	}

	if _, err := json.Marshal(proj.Narrative); err != nil {
		t.Fatalf("marshal narrative payload: %v", err)
	}
}

func TestProjectDashboard_PreservesCanonicalFunnelDailyContract(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()
	value := func(v int64) *int64 { return &v }
	change := 4.62
	facts.Funnel = dashboardfacts.FunnelFacts{
		Status:                "available",
		CurrentCompletedDate:  "2026-07-07",
		PreviousCompletedDate: "2026-07-06",
		Stages: []dashboardfacts.FunnelStage{
			{
				Key: "impressions", Label: "Показы", Value: value(12345), PreviousDayValue: value(11800),
				ChangeVsPreviousDayPct: &change, PeriodType: "completed_day",
			},
		},
		DailySeries: []dashboardfacts.FunnelDailyPoint{
			{Date: "2026-07-06", Impressions: value(11800)},
			{Date: "2026-07-07", Impressions: value(12345)},
		},
		Overview7D: dashboardfacts.FunnelWeeklyFacts{
			Status: "available",
			Stages: []dashboardfacts.FunnelWeeklyStage{{Key: "impressions", Label: "Показы", Value: value(70000)}},
		},
	}

	projection, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	funnel := projection.Frozen.SalesFunnel
	if funnel.CurrentCompletedDate != "2026-07-07" || funnel.PreviousCompletedDate != "2026-07-06" {
		t.Fatalf("completed dates lost in projection: %+v", funnel)
	}
	if len(funnel.DailySeries) != 2 || funnel.DailySeries[1].Impressions == nil || *funnel.DailySeries[1].Impressions != 12345 {
		t.Fatalf("daily_series lost in projection: %+v", funnel.DailySeries)
	}
	apiResponse := dashboardhome.Response{}
	if err := dashboardhome.ApplyFrozenDashboardDataUnified(&apiResponse, projection.Frozen); err != nil {
		t.Fatalf("apply production dashboard projection: %v", err)
	}
	raw, err := json.Marshal(apiResponse)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	aiDiagnostics, ok := payload["ai_diagnostics"].(map[string]any)
	if !ok {
		t.Fatal("ai_diagnostics JSON object missing")
	}
	salesFunnel, ok := aiDiagnostics["sales_funnel"].(map[string]any)
	if !ok {
		t.Fatal("sales_funnel JSON object missing")
	}
	for _, key := range []string{"current_completed_date", "previous_completed_date", "stages", "daily_series", "overview_7d"} {
		if _, ok := salesFunnel[key]; !ok {
			t.Fatalf("canonical sales_funnel.%s missing from production DTO JSON", key)
		}
	}
	stages, ok := salesFunnel["stages"].([]any)
	if !ok || len(stages) != 1 {
		t.Fatalf("stages JSON=%T %#v", salesFunnel["stages"], salesFunnel["stages"])
	}
	stage := stages[0].(map[string]any)
	for _, key := range []string{"previous_day_value", "change_vs_previous_day_pct", "previous_conversion_from_previous", "conversion_delta_pp"} {
		if _, ok := stage[key]; !ok {
			t.Fatalf("nullable canonical stage field %s must be present", key)
		}
	}
}

// TestProjectDashboard_AIWrongRevenueIgnored verifies that when the AI
// output's seller_pulse KPI numbers disagree with backend facts, the
// published projection always uses the facts value (never the AI number),
// and the disagreement is surfaced only as an informational
// MismatchWarning.
func TestProjectDashboard_AIWrongRevenueIgnored(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	// Sabotage the AI's revenue KPI so it disagrees with facts.
	for i := range out.Dashboard.SellerPulse.KPIs {
		if out.Dashboard.SellerPulse.KPIs[i].Key == "revenue" {
			out.Dashboard.SellerPulse.KPIs[i].Value = "999 999 999 ₽"
		}
	}
	facts := factsForValidFixture()

	proj, warnings := ProjectDashboard(facts, out, 42, "2026-07-08")

	var publishedRevenue string
	for _, kpi := range proj.Frozen.SellerPulse.KPIs {
		if kpi.Key == "revenue" {
			publishedRevenue = kpi.Value
		}
	}
	if publishedRevenue != "430 000 ₽" {
		t.Fatalf("expected published revenue KPI to come from facts (430 000 ₽), got %q", publishedRevenue)
	}

	foundWarning := false
	for _, w := range warnings {
		if w.Field == "seller_pulse.kpis.revenue" {
			foundWarning = true
			if w.FactValue != "430 000 ₽" || w.AIValue != "999 999 999 ₽" {
				t.Fatalf("unexpected mismatch warning values: %+v", w)
			}
		}
	}
	if !foundWarning {
		t.Fatalf("expected a mismatch warning for seller_pulse.kpis.revenue, got %+v", warnings)
	}
}

// TestProjectDashboard_EmptyAIKPIsUsesFacts verifies that when the AI
// output's seller_pulse.kpis array is empty but backend facts have KPIs,
// the published projection uses the facts KPIs (valid merge outcome; never
// falls back to an empty/AI-derived array).
func TestProjectDashboard_EmptyAIKPIsUsesFacts(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.SellerPulse.KPIs = nil
	facts := factsForValidFixture()

	proj, warnings := ProjectDashboard(facts, out, 42, "2026-07-08")
	for _, w := range warnings {
		if w.Field == "account_summary.health_score" && w.Reason == "backend_score_overrides_ai" {
			continue
		}
		if w.Field == "account_summary.risk_drivers" && w.Reason == "ai_risk_driver_unknown_id" {
			continue
		}
		t.Fatalf("unexpected mismatch warning when AI KPIs are empty: %+v", w)
	}
	if len(proj.Frozen.SellerPulse.KPIs) != 2 {
		t.Fatalf("expected facts KPIs to be used when AI KPIs are empty, got %d", len(proj.Frozen.SellerPulse.KPIs))
	}
}

// TestProjectDashboard_NeverFallsBackWhenFactsEmpty verifies that when
// backend facts have no seller_pulse KPIs, ProjectDashboard does NOT fall
// back to the AI-reported KPI numbers — the published block stays empty.
func TestProjectDashboard_NeverFallsBackWhenFactsEmpty(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()
	facts.SellerPulse.KPIs = nil

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	if len(proj.Frozen.SellerPulse.KPIs) != 0 {
		t.Fatalf("expected no seller_pulse KPIs published (facts empty, no AI fallback), got %+v", proj.Frozen.SellerPulse.KPIs)
	}
}

// TestProjectDashboard_SKUFocusCardCarriesSparkline verifies that
// skuFocusCardFromFacts joins SKUFactRecord.Sparkline7D onto the projected
// SKUFocusCard, including missing days (HasData=false, Value/Revenue nil).
func TestProjectDashboard_SKUFocusCardCarriesSparkline(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()
	rec := facts.SKUFactsIndex.ByOzonProductID["1"]
	rec.Sparkline7D = []dashboardfacts.SparklinePoint{
		{Date: "2026-07-02", HasData: false},
		{Date: "2026-07-03", Value: f64(4), Revenue: f64(400), HasData: true},
		{Date: "2026-07-04", Value: f64(5), Revenue: f64(500), HasData: true},
	}
	facts.SKUFactsIndex.ByOzonProductID["1"] = rec

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	if len(proj.Frozen.SKUFocus.OrderDrop) != 1 {
		t.Fatalf("expected 1 order_drop sku, got %d", len(proj.Frozen.SKUFocus.OrderDrop))
	}
	card := proj.Frozen.SKUFocus.OrderDrop[0]
	if len(card.Sparkline7D) != 3 {
		t.Fatalf("expected 3 sparkline points joined onto the card, got %d", len(card.Sparkline7D))
	}
	if card.Sparkline7D[0].HasData {
		t.Fatalf("expected missing day to stay HasData=false, got %+v", card.Sparkline7D[0])
	}
	if card.Sparkline7D[0].Value != nil || card.Sparkline7D[0].Revenue != nil {
		t.Fatalf("expected missing day to have nil value/revenue (never coerced to zero), got %+v", card.Sparkline7D[0])
	}
	if !card.Sparkline7D[1].HasData || card.Sparkline7D[1].Value == nil || *card.Sparkline7D[1].Value != 4 {
		t.Fatalf("expected observed day to carry orders value 4, got %+v", card.Sparkline7D[1])
	}
}

// TestProjectDashboard_KeySkuCardCarriesRevenueTrend verifies that
// keySkuCardFromCohortItem joins SKUFactRecord.Sparkline7D onto
// KeySkuCard.RevenueTrend7D.
func TestProjectDashboard_KeySkuCardCarriesRevenueTrend(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	if len(out.Dashboard.SKUFocus.KeySKUs) == 0 {
		t.Fatal("fixture has no key_skus to exercise")
	}
	facts := factsForValidFixture()
	keySku := out.Dashboard.SKUFocus.KeySKUs[0]
	rec, ok := buildSKUFactsLookup(facts.SKUFactsIndex).find(keySku.SKU, keySku.OfferID)
	if !ok {
		t.Fatal("expected fixture key sku to resolve against facts")
	}
	rec.Sparkline7D = []dashboardfacts.SparklinePoint{
		{Date: "2026-07-07", HasData: false},
		{Date: "2026-07-08", Value: f64(3), Revenue: f64(9000), HasData: true},
	}
	facts.SKUFactsIndex.ByOzonProductID[fmt.Sprintf("%d", rec.OzonProductID)] = rec

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	var card *dashboardhome.KeySkuCard
	for i := range proj.Frozen.SKUFocus.KeySKUs {
		if proj.Frozen.SKUFocus.KeySKUs[i].OzonProductID == rec.OzonProductID {
			card = &proj.Frozen.SKUFocus.KeySKUs[i]
			break
		}
	}
	if card == nil {
		t.Fatal("expected cohort key sku in the projection")
	}
	if len(card.RevenueTrend7D) != 2 {
		t.Fatalf("expected 2 revenue trend points, got %d", len(card.RevenueTrend7D))
	}
	if card.RevenueTrend7D[0].HasData != nil {
		t.Fatalf("expected missing day HasData=nil, got %+v", card.RevenueTrend7D[0])
	}
	if card.RevenueTrend7D[1].HasData == nil || !*card.RevenueTrend7D[1].HasData {
		t.Fatalf("expected observed day HasData=true, got %+v", card.RevenueTrend7D[1])
	}
	if card.RevenueTrend7D[1].Orders != 3 || card.RevenueTrend7D[1].Revenue != 9000 {
		t.Fatalf("expected observed day orders=3 revenue=9000, got %+v", card.RevenueTrend7D[1])
	}
}

// TestProjectDashboard_KeySKUCohortMembershipOwnsSelection verifies that
// backend KeySKUCohort is the membership source: AI-only key_skus outside the
// cohort are dropped, and cohort members appear even without AI text.
func TestProjectDashboard_KeySKUCohortMembershipOwnsSelection(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	facts := factsForValidFixture()

	// AI nominates an outside-cohort SKU (product 4) and omits cohort member
	// product 2 (which has no AI text).
	out.Dashboard.SKUFocus.KeySKUs = []contract.UnifiedKeySKUCardV1{
		{SKU: i64(801000100001), ProductName: "SKU One", Reason: "AI reason for cohort member"},
		{SKU: i64(801000100004), ProductName: "SKU Four outside", Reason: "should be dropped"},
	}

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	got := proj.Frozen.SKUFocus.KeySKUs
	if len(got) != 2 {
		t.Fatalf("expected 2 cohort members, got %d: %+v", len(got), got)
	}
	if got[0].OzonProductID != 1 || got[1].OzonProductID != 2 {
		t.Fatalf("expected cohort order [1,2], got [%d,%d]", got[0].OzonProductID, got[1].OzonProductID)
	}
	for _, c := range got {
		if c.OzonProductID == 4 {
			t.Fatal("AI-only SKU outside cohort must be dropped")
		}
	}
	if got[0].RevenueSharePct != 59.7 || got[1].CumulativeRevenueSharePct != 99.5 {
		t.Fatalf("expected backend cohort shares, got %+v / %+v", got[0], got[1])
	}
	if got[0].Revenue7d != 12000 || got[1].Revenue7d != 8000 {
		t.Fatalf("expected cohort revenue_7d from facts, got %.0f / %.0f", got[0].Revenue7d, got[1].Revenue7d)
	}
	if proj.Frozen.SKUFocus.KeySKUsTotalCount != 2 {
		t.Fatalf("KeySKUsTotalCount=%d want 2", proj.Frozen.SKUFocus.KeySKUsTotalCount)
	}
}

// TestProjectDashboard_UnknownSKUDropped verifies that an AI-selected
// sku_focus entity that cannot be resolved against facts.SKUFactsIndex is
// dropped from the published projection rather than substituted with
// AI-provided numbers.
func TestProjectDashboard_UnknownSKUDropped(t *testing.T) {
	out := readContractFixture(t, "unified_output_v1.valid.json")
	out.Dashboard.SKUFocus.OrderDrop = append(out.Dashboard.SKUFocus.OrderDrop, contract.UnifiedSKUFocusCardV1{
		SKU:         i64(999999999999),
		ProductName: "Unknown SKU",
		Problem:     "Не найден в фактах",
		KeyMetric:   "n/a",
	})
	facts := factsForValidFixture()

	proj, _ := ProjectDashboard(facts, out, 42, "2026-07-08")
	if len(proj.Frozen.SKUFocus.OrderDrop) != 1 {
		t.Fatalf("expected unresolved SKU to be dropped, still have %d order_drop entries", len(proj.Frozen.SKUFocus.OrderDrop))
	}
}
