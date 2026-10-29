package outputvalidator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func sectionFloatPtr(v float64) *float64 { return &v }

func TestSectionFactPolicy_SellerPulseAllowsAccountOnly(t *testing.T) {
	if !outputvalidator.FactScopeAllowedInSection(outputvalidator.SectionSellerPulse, outputvalidator.FactScopeAccount) {
		t.Fatal("account must be allowed in seller_pulse")
	}
	for _, scope := range []outputvalidator.FactScope{
		outputvalidator.FactScopeCampaign,
		outputvalidator.FactScopeSKU,
		outputvalidator.FactScopeCategory,
	} {
		if outputvalidator.FactScopeAllowedInSection(outputvalidator.SectionSellerPulse, scope) {
			t.Fatalf("scope %s must be forbidden in seller_pulse", scope)
		}
	}
	if !outputvalidator.FactScopeAllowedInSection(outputvalidator.SectionAdvertising, outputvalidator.FactScopeCampaign) {
		t.Fatal("campaign must remain allowed in advertising")
	}
	if outputvalidator.AllowedFactScopesForSection(outputvalidator.SectionUnknown) != nil {
		t.Fatal("unknown section must deny-all (nil allowlist)")
	}
}

func TestFinalizeSectionFacts_PASS_AccountRevenueKept(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					MetricClaims: []contract.UnifiedMetricClaimV1{{
						FactID: "account:revenue_7d:current_7d", MetricCode: "revenue_7d", Value: sectionFloatPtr(100), PeriodType: "current_7d",
					}},
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{{
						UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
							Title: "Revenue", Explanation: "Недельная выручка выросла", FactIDs: []string{"account:revenue_7d:current_7d"},
						},
						Claims: []contract.UnifiedMetricClaimV1{{
							FactID: "account:revenue_7d:current_7d", MetricCode: "revenue_7d", Value: sectionFloatPtr(100), PeriodType: "current_7d",
						}},
					}},
				},
			},
		},
	}
	res := outputvalidator.FinalizeDashboardSectionFactReferences(out)
	if res.DroppedPulseItems != 0 || res.DroppedMetricClaims != 0 {
		t.Fatalf("unexpected drops: %+v", res)
	}
	if out.Dashboard.SellerPulse.Narrative.MetricClaims[0].FactID != "account:revenue_7d:current_7d" {
		t.Fatal("account revenue claim must be kept")
	}
}

func TestFinalizeSectionFacts_PASS_AccountAdShareKept(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{{
						UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
							Title: "Ad load", Explanation: "Доля рекламы в общей выручке 16%",
							FactIDs: []string{"account:ad_spend_share_of_total_store_revenue_pct:current_7d"},
						},
						Claims: []contract.UnifiedMetricClaimV1{{
							FactID:     "account:ad_spend_share_of_total_store_revenue_pct:current_7d",
							MetricCode: "ad_spend_share_of_total_store_revenue_pct", Value: sectionFloatPtr(16.8), PeriodType: "current_7d",
						}},
					}},
				},
			},
		},
	}
	_ = outputvalidator.FinalizeDashboardSectionFactReferences(out)
	if len(out.Dashboard.SellerPulse.Narrative.ImportantFacts) != 1 {
		t.Fatal("account ad share item must remain")
	}
}

func TestFinalizeSectionFacts_PASS_MixedAccountAndCampaignDropsCampaign(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{{
						UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
							Title: "Stock and ads", Explanation: "12 SKU без продаж при остатке",
							FactIDs:   []string{"account:no_sales_with_stock_count:current_7d", "campaign:882000021528:DRR"},
							EntityIDs: []string{"882000021528"},
						},
						Claims: []contract.UnifiedMetricClaimV1{
							{FactID: "account:no_sales_with_stock_count:current_7d", MetricCode: "no_sales_with_stock_count", Value: sectionFloatPtr(12), PeriodType: "current_7d"},
							{FactID: "campaign:882000021528:DRR", MetricCode: "campaign_drr_pct", Value: sectionFloatPtr(90.1), PeriodType: "current_7d"},
						},
					}},
				},
			},
		},
	}
	res := outputvalidator.FinalizeDashboardSectionFactReferences(out)
	if res.DroppedPulseClaims < 1 || res.DroppedPulseFactIDs < 1 {
		t.Fatalf("expected campaign refs dropped: %+v", res)
	}
	got := out.Dashboard.SellerPulse.Narrative.ImportantFacts
	if len(got) != 1 {
		t.Fatalf("item must remain with account evidence, got %d", len(got))
	}
	if len(got[0].FactIDs) != 1 || got[0].FactIDs[0] != "account:no_sales_with_stock_count:current_7d" {
		t.Fatalf("fact_ids=%v", got[0].FactIDs)
	}
	if len(got[0].Claims) != 1 {
		t.Fatalf("claims=%v", got[0].Claims)
	}
	if len(got[0].EntityIDs) != 0 {
		t.Fatalf("campaign entity_ids must be cleared, got %v", got[0].EntityIDs)
	}
}

func TestFinalizeSectionFacts_REPAIR_CampaignOnlyItemDropped(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{{
						UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
							Title: "2 кампании >80% ДРР", Explanation: "ДРР превысил 80%",
							FactIDs: []string{"campaign:882000021528:DRR"},
						},
						MetricCode: "campaign_drr_pct",
						Claims: []contract.UnifiedMetricClaimV1{{
							FactID: "campaign:882000021528:DRR", MetricCode: "campaign_drr_pct", Value: sectionFloatPtr(90.1), PeriodType: "current_7d",
						}},
					}},
				},
			},
		},
	}
	res := outputvalidator.FinalizeDashboardSectionFactReferences(out)
	if res.DroppedPulseItems != 1 {
		t.Fatalf("DroppedPulseItems=%d want 1", res.DroppedPulseItems)
	}
	if len(out.Dashboard.SellerPulse.Narrative.ImportantFacts) != 0 {
		t.Fatal("campaign-only item must be removed")
	}
}

func TestFinalizeSectionFacts_FAIL_SKUStockInPulseDropped(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{{
						UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
							Title: "SKU stock", Explanation: "покрытие 3 дня",
							FactIDs: []string{"sku:801000100001:stock_coverage_days_7d"},
						},
						Claims: []contract.UnifiedMetricClaimV1{{
							FactID: "sku:801000100001:stock_coverage_days_7d", MetricCode: "stock_coverage_days_7d", Value: sectionFloatPtr(3), PeriodType: "current_7d",
						}},
					}},
				},
			},
		},
	}
	res := outputvalidator.FinalizeDashboardSectionFactReferences(out)
	if res.DroppedPulseItems != 1 {
		t.Fatalf("sku fact item must be dropped: %+v", res)
	}
}

func TestFinalizeSectionFacts_Idempotent(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			SellerPulse: contract.UnifiedSellerPulseV1{
				Narrative: &contract.UnifiedSellerPulseNarrativeV1{
					ImportantFacts: []contract.UnifiedSellerPulsePointV1{{
						UnifiedDiagnosticSignalV2: contract.UnifiedDiagnosticSignalV2{
							Title: "x", Explanation: "y", FactIDs: []string{"account:revenue_7d:current_7d", "campaign:1:DRR"},
						},
						Claims: []contract.UnifiedMetricClaimV1{
							{FactID: "account:revenue_7d:current_7d", MetricCode: "revenue_7d", Value: sectionFloatPtr(1), PeriodType: "current_7d"},
							{FactID: "campaign:1:DRR", MetricCode: "campaign_drr_pct", Value: sectionFloatPtr(90), PeriodType: "current_7d"},
						},
					}},
				},
			},
		},
	}
	_ = outputvalidator.FinalizeDashboardSectionFactReferences(out)
	first, _ := json.Marshal(out.Dashboard.SellerPulse.Narrative)
	res2 := outputvalidator.FinalizeDashboardSectionFactReferences(out)
	if res2.DroppedPulseClaims != 0 || res2.DroppedPulseItems != 0 || res2.DroppedPulseFactIDs != 0 {
		t.Fatalf("second pass must be no-op: %+v", res2)
	}
	second, _ := json.Marshal(out.Dashboard.SellerPulse.Narrative)
	if string(first) != string(second) {
		t.Fatal("Finalize(Finalize(x)) must equal Finalize(x)")
	}
}

func TestParseFactIDScope_AndForbiddenMetrics(t *testing.T) {
	if outputvalidator.ParseFactIDScope("campaign:882000021528:DRR") != outputvalidator.FactScopeCampaign {
		t.Fatal("expected campaign scope")
	}
	if outputvalidator.ParseFactIDScope("account:revenue_7d:current_7d") != outputvalidator.FactScopeAccount {
		t.Fatal("expected account scope")
	}
	if !outputvalidator.IsSellerPulseForbiddenMetric("campaign_drr_pct") {
		t.Fatal("campaign_drr_pct must be forbidden in Seller Pulse")
	}
	if outputvalidator.IsSellerPulseForbiddenMetric("ad_spend_share_of_total_store_revenue_pct") {
		t.Fatal("store ad share must be allowed")
	}
}

func TestAcceptanceRun204_FinalizeSectionFacts(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "acceptance_run204_output.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n := out.Dashboard.SellerPulse.Narrative
	if n == nil || len(n.ImportantFacts) < 2 {
		t.Fatal("fixture missing important_facts")
	}
	before := n.ImportantFacts[1]
	if before.Claims[0].FactID != "campaign:882000021528:DRR" {
		t.Fatalf("fixture drift fact_id=%q", before.Claims[0].FactID)
	}
	if before.Claims[0].MetricCode != "campaign_drr_pct" {
		t.Fatalf("fixture drift metric=%q", before.Claims[0].MetricCode)
	}

	beforeCount := len(n.ImportantFacts)
	res := outputvalidator.FinalizeDashboardSectionFactReferences(&out)
	if res.DroppedPulseItems != 1 {
		t.Fatalf("DroppedPulseItems=%d want 1; res=%+v", res.DroppedPulseItems, res)
	}
	after := out.Dashboard.SellerPulse.Narrative.ImportantFacts
	if len(after) != beforeCount-1 {
		t.Fatalf("important_facts %d → %d", beforeCount, len(after))
	}
	for _, item := range after {
		for _, id := range item.FactIDs {
			if strings.HasPrefix(id, "campaign:") {
				t.Fatalf("campaign fact leaked: %s", id)
			}
		}
		for _, c := range item.Claims {
			if strings.HasPrefix(c.FactID, "campaign:") || c.MetricCode == "campaign_drr_pct" {
				t.Fatalf("campaign claim leaked: %+v", c)
			}
		}
	}
	res2 := outputvalidator.FinalizeDashboardSectionFactReferences(&out)
	if res2.DroppedPulseItems != 0 {
		t.Fatalf("idempotent second pass failed: %+v", res2)
	}
	// Contract depth: 2 remaining important_facts → strict depth fails (Variant C).
	report := outputvalidator.Validate(&out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, PackHash: out.AnalysisMeta.PackHash, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !hasIssueCode(report.Errors, outputvalidator.CodeSellerPulseImportantFactsMissing) && report.Valid {
		t.Fatalf("expected depth failure after dropping required item, errors=%+v valid=%v", report.Errors, report.Valid)
	}
}
