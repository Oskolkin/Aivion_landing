package outputvalidator_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/healthstatus"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

func minimalAction(id, tier, priority string) contract.UnifiedActionV1 {
	return contract.UnifiedActionV1{
		AnalysisActionID:   id,
		ExecutionTier:      tier,
		Priority:           priority,
		Type:               "stock_replenishment",
		Category:           "stock",
		Confidence:         "high",
		OperationalGoal:    "goal-" + id,
		EvidenceSummary:    "evidence-" + id,
		CompletionCriteria: "done-" + id,
		Steps:              []contract.UnifiedActionStepV1{{Order: 1, Title: "step", Description: "do it"}},
		Scope: contract.UnifiedActionScopeV1{
			ScopeType: "account",
		},
	}
}

func assertTopRequiredOnly(t *testing.T, out *contract.UnifiedAnalysisOutputV1) {
	t.Helper()
	byID := map[string]contract.UnifiedActionV1{}
	for _, a := range out.Actions {
		byID[a.AnalysisActionID] = a
	}
	if len(out.Dashboard.TopActionIDs) > contract.MaxTopActionsV1 {
		t.Fatalf("top_action_ids len=%d > %d", len(out.Dashboard.TopActionIDs), contract.MaxTopActionsV1)
	}
	for _, id := range out.Dashboard.TopActionIDs {
		a, ok := byID[id]
		if !ok {
			t.Fatalf("top id %q missing from actions", id)
		}
		if a.ExecutionTier != contract.ExecutionTierRequired {
			t.Fatalf("top id %q tier=%q want required", id, a.ExecutionTier)
		}
	}
}

func TestFinalizeTopActions_TwoRequiredOneRecommendedInTop(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "critical"),
			minimalAction("action_002", contract.ExecutionTierRequired, "important"),
			minimalAction("action_003", contract.ExecutionTierRecommended, "watch"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs:         []string{"action_001", "action_002", "action_003"},
			RecommendedActionIDs: nil,
		},
	}
	res := outputvalidator.FinalizeTopActionIDs(out)
	if res.DroppedNonRequired != 1 {
		t.Fatalf("DroppedNonRequired=%d want 1", res.DroppedNonRequired)
	}
	want := []string{"action_001", "action_002"}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, want) {
		t.Fatalf("top=%v want=%v", out.Dashboard.TopActionIDs, want)
	}
	if !reflect.DeepEqual(out.Dashboard.RecommendedActionIDs, []string{"action_003"}) {
		t.Fatalf("recommended=%v want [action_003]", out.Dashboard.RecommendedActionIDs)
	}
	assertTopRequiredOnly(t, out)

	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{}, nil)
	if hasIssueCode(report.Errors, outputvalidator.CodeQuickActionRequiredReferenceMissing) {
		t.Fatalf("tier error must be cleared after finalization: %+v", report.Errors)
	}
}

func TestFinalizeTopActions_OneRequiredTwoRecommended(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "critical"),
			minimalAction("action_002", contract.ExecutionTierRecommended, "important"),
			minimalAction("action_003", contract.ExecutionTierRecommended, "watch"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_001", "action_002", "action_003"},
		},
	}
	_ = outputvalidator.FinalizeTopActionIDs(out)
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, []string{"action_001"}) {
		t.Fatalf("top=%v want [action_001]", out.Dashboard.TopActionIDs)
	}
	assertTopRequiredOnly(t, out)
}

func TestFinalizeTopActions_FourRequiredRanksByMateriality(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "watch"),
			minimalAction("action_002", contract.ExecutionTierRequired, "watch"),
			minimalAction("action_003", contract.ExecutionTierRequired, "watch"),
			minimalAction("action_004", contract.ExecutionTierRequired, "critical"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_001", "action_002", "action_003", "action_004"},
		},
	}
	_ = outputvalidator.FinalizeTopActionIDs(out)
	// All are stock_replenishment; critical priority wins first slot.
	if out.Dashboard.TopActionIDs[0] != "action_004" {
		t.Fatalf("top[0]=%v want action_004 (critical materiality)", out.Dashboard.TopActionIDs)
	}
	if len(out.Dashboard.TopActionIDs) != 3 {
		t.Fatalf("top len=%d want 3", len(out.Dashboard.TopActionIDs))
	}
}

func TestFinalizeTopActions_StockOutranksNoSales(t *testing.T) {
	stock := minimalAction("action_stock", contract.ExecutionTierRequired, "important")
	stock.Type = "stock_replenishment"
	nosales := minimalAction("action_nosales", contract.ExecutionTierRequired, "watch")
	nosales.Type = "no_sales_with_stock"
	ads := minimalAction("action_ads", contract.ExecutionTierRequired, "important")
	ads.Type = "ad_high_drr_review"
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{nosales, ads, stock},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_nosales", "action_ads", "action_stock"},
		},
	}
	_ = outputvalidator.FinalizeTopActionIDs(out)
	want := []string{"action_stock", "action_ads", "action_nosales"}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, want) {
		t.Fatalf("top=%v want=%v", out.Dashboard.TopActionIDs, want)
	}
}

func TestFinalizeTopActions_BackfillUsesPriorityThenOrder(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "watch"),
			minimalAction("action_002", contract.ExecutionTierRequired, "critical"),
			minimalAction("action_003", contract.ExecutionTierRequired, "important"),
			minimalAction("action_004", contract.ExecutionTierRequired, "critical"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			// AI listed only one required; backfill must use priority then order.
			TopActionIDs: []string{"action_001"},
		},
	}
	res := outputvalidator.FinalizeTopActionIDs(out)
	if res.Backfilled != 3 {
		t.Fatalf("Backfilled=%d want 3 (all remaining required enter the materiality pool)", res.Backfilled)
	}
	// Materiality re-ranks: critical actions before important/watch.
	want := []string{"action_002", "action_004", "action_003"}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, want) {
		t.Fatalf("top=%v want=%v", out.Dashboard.TopActionIDs, want)
	}
	assertTopRequiredOnly(t, out)
}

func TestFinalizeTopActions_BackfillRequiredOutsideTop(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "important"),
			minimalAction("action_002", contract.ExecutionTierRecommended, "watch"),
			minimalAction("action_003", contract.ExecutionTierRequired, "critical"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_001", "action_002"},
		},
	}
	_ = outputvalidator.FinalizeTopActionIDs(out)
	want := []string{"action_003", "action_001"}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, want) {
		t.Fatalf("top=%v want=%v", out.Dashboard.TopActionIDs, want)
	}
	if !reflect.DeepEqual(out.Dashboard.RecommendedActionIDs, []string{"action_002"}) {
		t.Fatalf("recommended=%v", out.Dashboard.RecommendedActionIDs)
	}
}

func TestFinalizeTopActions_DuplicateRequiredRemoved(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "critical"),
			minimalAction("action_002", contract.ExecutionTierRequired, "important"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_001", "action_001", "action_002"},
		},
	}
	res := outputvalidator.FinalizeTopActionIDs(out)
	if res.DroppedDuplicates != 1 {
		t.Fatalf("DroppedDuplicates=%d want 1", res.DroppedDuplicates)
	}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, []string{"action_001", "action_002"}) {
		t.Fatalf("top=%v", out.Dashboard.TopActionIDs)
	}
}

func TestFinalizeTopActions_UnknownIDNotSilentlyReplaced(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "critical"),
			minimalAction("action_002", contract.ExecutionTierRequired, "important"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_missing", "action_001"},
		},
	}
	res := outputvalidator.FinalizeTopActionIDs(out)
	if res.DroppedUnknown != 1 {
		t.Fatalf("DroppedUnknown=%d want 1", res.DroppedUnknown)
	}
	// Unknown is dropped; remaining required kept; free slot backfilled with action_002.
	want := []string{"action_001", "action_002"}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, want) {
		t.Fatalf("top=%v want=%v (unknown must not become another invented id)", out.Dashboard.TopActionIDs, want)
	}
	for _, id := range out.Dashboard.TopActionIDs {
		if id == "action_missing" {
			t.Fatal("unknown id must not remain after finalization")
		}
	}
}

func TestFinalizeTopActions_NoRequiredLeavesEmptyTop(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		AnalysisOutcome: contract.AnalysisOutcomeMaterialActionsFound,
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRecommended, "important"),
			minimalAction("action_002", contract.ExecutionTierRecommended, "watch"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_001", "action_002"},
		},
	}
	_ = outputvalidator.FinalizeTopActionIDs(out)
	if len(out.Dashboard.TopActionIDs) != 0 {
		t.Fatalf("top must stay empty when no required actions, got %v", out.Dashboard.TopActionIDs)
	}
	// Strict validator must still reject material_actions_found without required.
	report := outputvalidator.Validate(out, outputvalidator.ExpectedRunMeta{}, nil)
	if !hasIssueCode(report.Errors, outputvalidator.CodeRequiredActionsCountBelowMinimum) &&
		!hasIssueCode(report.Errors, outputvalidator.CodeQuickActionRequiredReferenceMissing) {
		// Prefer required-count error; either way must not invent tops.
		if report.Valid {
			t.Fatal("expected contract error when material_actions_found has zero required")
		}
	}
}

func TestFinalizeTopActions_Idempotent(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			minimalAction("action_001", contract.ExecutionTierRequired, "critical"),
			minimalAction("action_002", contract.ExecutionTierRequired, "important"),
			minimalAction("action_003", contract.ExecutionTierRecommended, "watch"),
		},
		Dashboard: contract.UnifiedDashboardV1{
			TopActionIDs: []string{"action_001", "action_002", "action_003"},
		},
	}
	_ = outputvalidator.FinalizeTopActionIDs(out)
	firstTop := append([]string{}, out.Dashboard.TopActionIDs...)
	firstRec := append([]string{}, out.Dashboard.RecommendedActionIDs...)
	res2 := outputvalidator.FinalizeTopActionIDs(out)
	if res2.DroppedNonRequired != 0 || res2.Backfilled != 0 || res2.DroppedUnknown != 0 {
		t.Fatalf("second pass must be no-op, got %+v", res2)
	}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, firstTop) {
		t.Fatalf("top changed on second pass: %v -> %v", firstTop, out.Dashboard.TopActionIDs)
	}
	if !reflect.DeepEqual(out.Dashboard.RecommendedActionIDs, firstRec) {
		t.Fatalf("recommended changed on second pass")
	}
}

func TestAcceptanceRun201_FinalizeAndValidate(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "acceptance_run201_output.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read run201 fixture: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Dashboard.TopActionIDs[2] != "action_003" {
		t.Fatalf("fixture drift: top[2]=%q want action_003", out.Dashboard.TopActionIDs[2])
	}

	before := outputvalidator.Validate(&out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		PackHash:   out.AnalysisMeta.PackHash,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !hasIssueCode(before.Errors, outputvalidator.CodeQuickActionRequiredReferenceMissing) {
		t.Fatalf("run201 fixture must reproduce tier error before finalization, got %+v", before.Errors)
	}

	res := outputvalidator.FinalizeTopActionIDs(&out)
	if res.DroppedNonRequired != 1 {
		t.Fatalf("DroppedNonRequired=%d want 1", res.DroppedNonRequired)
	}
	wantTop := []string{"action_001", "action_002"}
	if !reflect.DeepEqual(out.Dashboard.TopActionIDs, wantTop) {
		t.Fatalf("top=%v want=%v", out.Dashboard.TopActionIDs, wantTop)
	}
	foundRec := false
	for _, id := range out.Dashboard.RecommendedActionIDs {
		if id == "action_003" {
			foundRec = true
			break
		}
	}
	if !foundRec {
		t.Fatalf("action_003 must move to recommended_action_ids, got %v", out.Dashboard.RecommendedActionIDs)
	}
	assertTopRequiredOnly(t, &out)

	_ = outputvalidator.FinalizeDashboardRiskDrivers(&out, nil)

	// Historical fixture predates healthstatus band remapping; align severity
	// to the canonical FromScore mapping before the hard contract check.
	if out.Dashboard.AccountSummary.HealthScore != nil {
		out.Dashboard.AccountSummary.Severity = healthstatus.FromScore(out.Dashboard.AccountSummary.HealthScore).Severity
	}

	after := outputvalidator.Validate(&out, outputvalidator.ExpectedRunMeta{
		PackID:     out.AnalysisMeta.PackID,
		PackHash:   out.AnalysisMeta.PackHash,
		MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if hasIssueCode(after.Errors, outputvalidator.CodeQuickActionRequiredReferenceMissing) {
		t.Fatalf("tier error must be gone after finalization: %+v", after.Errors)
	}
	if !after.Valid {
		t.Fatalf("run201 after finalization must validate, errors=%+v warnings=%+v", after.Errors, after.Warnings)
	}
}

func hasIssueCode(items []outputvalidator.Issue, code string) bool {
	for _, it := range items {
		if it.Code == code {
			return true
		}
	}
	return false
}
