package outputvalidator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func riskDriver(id string, entityIDs, factIDs []string, category string) contract.UnifiedRiskDriverV1 {
	return contract.UnifiedRiskDriverV1{
		ID:            id,
		Category:      category,
		CategoryLabel: category,
		Severity:      "high",
		ImpactScore:   70,
		Title:         "title-" + id,
		Reason:        "reason-" + id,
		EntityIDs:     entityIDs,
		FactIDs:       factIDs,
	}
}

func packIdxWith(skus, campaigns []string) *outputvalidator.PackReferenceIndex {
	idx := &outputvalidator.PackReferenceIndex{
		SKUIds:      map[string]struct{}{},
		CampaignIds: map[string]struct{}{},
	}
	for _, s := range skus {
		idx.SKUIds[s] = struct{}{}
		idx.HasSKUsAny = true
	}
	for _, c := range campaigns {
		idx.CampaignIds[c] = struct{}{}
		idx.HasCampaignsAny = true
	}
	return idx
}

func TestFinalizeRiskDrivers_HighDRRComposite(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("high_drr_882000021528", []string{"882000021528"},
						[]string{"campaign:882000021528:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"882000021528"})
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if res.RewrittenComposite != 1 {
		t.Fatalf("RewrittenComposite=%d want 1", res.RewrittenComposite)
	}
	got := out.Dashboard.AccountSummary.RiskDrivers[0]
	if got.ID != "high_drr_campaign" {
		t.Fatalf("id=%q want high_drr_campaign", got.ID)
	}
	if !reflect.DeepEqual(got.EntityIDs, []string{"882000021528"}) {
		t.Fatalf("entity_ids=%v", got.EntityIDs)
	}
}

func TestFinalizeRiskDrivers_LowStockComposite(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("low_stock_801000100001", []string{"801000100001"},
						[]string{"sku:801000100001:stock_coverage_days_7d"}, "stock"),
				},
			},
		},
	}
	idx := packIdxWith([]string{"801000100001"}, nil)
	_ = outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	got := out.Dashboard.AccountSummary.RiskDrivers[0]
	if got.ID != "key_sku_stock_risk" {
		t.Fatalf("id=%q want key_sku_stock_risk", got.ID)
	}
}

func TestFinalizeRiskDrivers_CanonicalIdempotent(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("high_drr_campaign", []string{"882000021528"},
						[]string{"campaign:882000021528:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"882000021528"})
	_ = outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	first := out.Dashboard.AccountSummary.RiskDrivers[0].ID
	res2 := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if res2.RewrittenComposite != 0 || res2.MergedDuplicates != 0 {
		t.Fatalf("second pass must be no-op: %+v", res2)
	}
	if out.Dashboard.AccountSummary.RiskDrivers[0].ID != first {
		t.Fatal("canonical id changed on second pass")
	}
}

func TestFinalizeRiskDrivers_MultipleEntitiesSameCode(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("high_drr_111", []string{"111"}, []string{"campaign:111:drr_pct"}, "advertising"),
					riskDriver("high_drr_222", []string{"222"}, []string{"campaign:222:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"111", "222"})
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if res.RewrittenComposite != 2 {
		t.Fatalf("RewrittenComposite=%d want 2", res.RewrittenComposite)
	}
	if res.MergedDuplicates != 0 {
		t.Fatalf("MergedDuplicates=%d want 0 (distinct entities stay separate)", res.MergedDuplicates)
	}
	if len(out.Dashboard.AccountSummary.RiskDrivers) != 2 {
		t.Fatalf("drivers=%d want 2 instances of one registry code", len(out.Dashboard.AccountSummary.RiskDrivers))
	}
	for _, got := range out.Dashboard.AccountSummary.RiskDrivers {
		if got.ID != "high_drr_campaign" {
			t.Fatalf("id=%q want high_drr_campaign", got.ID)
		}
	}
	if out.Dashboard.AccountSummary.RiskDrivers[0].EntityIDs[0] == out.Dashboard.AccountSummary.RiskDrivers[1].EntityIDs[0] {
		t.Fatal("entity instances must remain distinct")
	}
}

func TestFinalizeRiskDrivers_DuplicateInstanceMerged(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("high_drr_111", []string{"111"}, []string{"campaign:111:drr_pct"}, "advertising"),
					riskDriver("high_drr_campaign", []string{"111"}, []string{"campaign:111:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"111"})
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if res.MergedDuplicates != 1 {
		t.Fatalf("MergedDuplicates=%d want 1", res.MergedDuplicates)
	}
	if len(out.Dashboard.AccountSummary.RiskDrivers) != 1 {
		t.Fatalf("drivers=%d want 1", len(out.Dashboard.AccountSummary.RiskDrivers))
	}
}

func TestFinalizeRiskDrivers_UnknownPrefixFailsValidation(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				HealthScore: intPtr(50),
				Severity:    "high",
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("very_bad_ads_882000021528", []string{"882000021528"},
						[]string{"campaign:882000021528:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"882000021528"})
	_ = outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{}, idx)
	if !hasIssueCode(report.Errors, outputvalidator.CodeRiskDriverUnknownID) {
		t.Fatalf("expected %s, got %+v", outputvalidator.CodeRiskDriverUnknownID, report.Errors)
	}
}

func TestFinalizeRiskDrivers_UnknownEntityUnresolved(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("high_drr_999999999999", []string{"999999999999"},
						[]string{"campaign:999999999999:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"882000021528"}) // entity not present
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if len(res.Unresolved) != 1 {
		t.Fatalf("Unresolved=%v want composite left unresolved", res.Unresolved)
	}
	if out.Dashboard.AccountSummary.RiskDrivers[0].ID != "high_drr_999999999999" {
		t.Fatal("must not silently rewrite to another entity")
	}
}

func TestFinalizeRiskDrivers_WrongEntityTypeUnresolved(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					// low_stock alias → key_sku_stock_risk, but entity is a campaign.
					riskDriver("low_stock_882000021528", []string{"882000021528"},
						[]string{"campaign:882000021528:drr_pct"}, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"882000021528"})
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if len(res.Unresolved) != 1 {
		t.Fatalf("wrong entity type must stay unresolved, got %+v / drivers=%+v", res, out.Dashboard.AccountSummary.RiskDrivers)
	}
}

func TestFinalizeRiskDrivers_MissingSupportingFactUnresolved(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{
					riskDriver("high_drr_882000021528", []string{"882000021528"}, nil, "advertising"),
				},
			},
		},
	}
	idx := packIdxWith(nil, []string{"882000021528"})
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if len(res.Unresolved) != 1 {
		t.Fatalf("missing fact must leave unresolved, got %+v", res)
	}
}

func TestFinalizeRiskDrivers_AmbiguousCodesUnresolved(t *testing.T) {
	// Two different entity suffixes that would map to different codes —
	// construct a pathological id that matches both suffixes if both entities listed.
	out := &contract.UnifiedAnalysisOutputV1{
		Dashboard: contract.UnifiedDashboardV1{
			AccountSummary: contract.UnifiedAccountSummaryV1{
				RiskDrivers: []contract.UnifiedRiskDriverV1{{
					ID:        "high_drr_111_low_stock_222",
					Category:  "advertising",
					Severity:  "high",
					Title:     "ambiguous",
					Reason:    "ambiguous",
					EntityIDs: []string{"111", "222"},
					FactIDs:   []string{"campaign:111:drr_pct", "sku:222:stock_coverage_days_7d"},
				}},
			},
		},
	}
	idx := packIdxWith([]string{"222"}, []string{"111"})
	res := outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	if len(res.Unresolved) != 1 {
		t.Fatalf("ambiguous composite must stay unresolved, got %+v / id=%q", res, out.Dashboard.AccountSummary.RiskDrivers[0].ID)
	}
}

func TestAcceptanceRun202_FinalizeRiskDriversAndValidate(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "acceptance_run202_output.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	beforeIDs := []string{
		out.Dashboard.AccountSummary.RiskDrivers[0].ID,
		out.Dashboard.AccountSummary.RiskDrivers[1].ID,
	}
	if beforeIDs[0] != "high_drr_882000021528" || beforeIDs[1] != "low_stock_801000100001" {
		t.Fatalf("fixture drift: %v", beforeIDs)
	}

	idx := packIdxWith([]string{"801000100001"}, []string{"882000021528"})
	// Also include other SKUs referenced by actions so validation is not distracted.
	for _, a := range out.Actions {
		for _, e := range a.Scope.EntityIDs {
			idx.SKUIds[e] = struct{}{}
		}
	}
	idx.HasSKUsAny = true
	idx.HasCampaignsAny = true

	before := outputvalidator.Validate(&out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, PackHash: out.AnalysisMeta.PackHash, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !hasIssueCode(before.Errors, outputvalidator.CodeRiskDriverUnknownID) {
		t.Fatalf("before finalize expected unknown driver ids, got %+v", before.Errors)
	}

	res := outputvalidator.FinalizeDashboardRiskDrivers(&out, idx)
	if res.RewrittenComposite != 2 {
		t.Fatalf("RewrittenComposite=%d want 2", res.RewrittenComposite)
	}
	ids := make([]string, 0, len(out.Dashboard.AccountSummary.RiskDrivers))
	for _, d := range out.Dashboard.AccountSummary.RiskDrivers {
		ids = append(ids, d.ID)
		if !outputvalidator.IsCanonicalRiskDriverID(d.ID) {
			t.Fatalf("non-canonical id after finalize: %q", d.ID)
		}
	}
	if !containsAll(ids, "high_drr_campaign", "key_sku_stock_risk") {
		t.Fatalf("ids=%v", ids)
	}

	after := outputvalidator.Validate(&out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, PackHash: out.AnalysisMeta.PackHash, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if hasIssueCode(after.Errors, outputvalidator.CodeRiskDriverUnknownID) {
		t.Fatalf("unknown driver must be gone: %+v", after.Errors)
	}
	if !after.Valid {
		t.Fatalf("run202 after risk finalize must validate: %+v", after.Errors)
	}

	res2 := outputvalidator.FinalizeDashboardRiskDrivers(&out, idx)
	if res2.RewrittenComposite != 0 {
		t.Fatalf("idempotent finalize failed: %+v", res2)
	}
}

func containsAll(have []string, want ...string) bool {
	set := map[string]struct{}{}
	for _, h := range have {
		set[h] = struct{}{}
	}
	for _, w := range want {
		if _, ok := set[w]; !ok {
			return false
		}
	}
	return true
}

func intPtr(v int32) *int32 { return &v }
