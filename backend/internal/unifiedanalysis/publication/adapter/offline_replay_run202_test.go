package adapter_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/healthpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
)

func TestOfflineReplayRun202_RiskDriverFinalizeBundlePASS(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	base := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "outputvalidator", "testdata"))
	outRaw, err := os.ReadFile(filepath.Join(base, "acceptance_run202_output.json"))
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	factsRaw, err := os.ReadFile(filepath.Join(base, "acceptance_run202_facts.json"))
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(outRaw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	var facts dashboardfacts.DashboardFactsV1
	if err := json.Unmarshal(factsRaw, &facts); err != nil {
		t.Fatalf("unmarshal facts: %v", err)
	}

	beforeIDs := []string{
		out.Dashboard.AccountSummary.RiskDrivers[0].ID,
		out.Dashboard.AccountSummary.RiskDrivers[1].ID,
	}
	if beforeIDs[0] != "high_drr_882000021528" || beforeIDs[1] != "low_stock_801000100001" {
		t.Fatalf("fixture drift: %v", beforeIDs)
	}

	_, beforeWarnings := adapter.ProjectDashboard(facts, &out, 1, out.AnalysisMeta.MetricDate)
	if !hasUnknownDriverWarning(beforeWarnings) {
		t.Fatalf("before finalize expected unknown driver warning, got %+v", beforeWarnings)
	}

	idx := &outputvalidator.PackReferenceIndex{
		SKUIds:          map[string]struct{}{"801000100001": {}},
		CampaignIds:     map[string]struct{}{"882000021528": {}},
		HasSKUsAny:      true,
		HasCampaignsAny: true,
	}
	for _, a := range out.Actions {
		for _, e := range a.Scope.EntityIDs {
			idx.SKUIds[e] = struct{}{}
		}
	}

	_ = outputvalidator.FinalizeTopActionIDs(&out)
	res := outputvalidator.FinalizeDashboardRiskDrivers(&out, idx)
	if res.RewrittenComposite != 2 {
		t.Fatalf("RewrittenComposite=%d want 2", res.RewrittenComposite)
	}

	got := map[string]bool{}
	for _, d := range out.Dashboard.AccountSummary.RiskDrivers {
		got[d.ID] = true
		if !healthpolicy.IsKnownDriverID(d.ID) {
			t.Fatalf("non-canonical after finalize: %q", d.ID)
		}
	}
	if !got[healthpolicy.DriverHighDRRCampaign] || !got[healthpolicy.DriverKeySKUStockRisk] {
		t.Fatalf("expected canonical codes, got keys=%v", got)
	}

	report := outputvalidator.Validate(&out, outputvalidator.ExpectedRunMeta{
		PackID: out.AnalysisMeta.PackID, PackHash: out.AnalysisMeta.PackHash, MetricDate: out.AnalysisMeta.MetricDate,
	}, nil)
	if !report.Valid {
		t.Fatalf("output validation must PASS after finalize: %+v", report.Errors)
	}

	proj, warnings := adapter.ProjectDashboard(facts, &out, 1, out.AnalysisMeta.MetricDate)
	if hasUnknownDriverWarning(warnings) {
		t.Fatalf("after finalize must not warn unknown drivers: %+v", warnings)
	}
	if proj.Narrative.AccountHealth == nil {
		t.Fatal("expected account health after projection")
	}
	for _, d := range proj.Narrative.AccountHealth.RiskDrivers {
		if !healthpolicy.IsKnownDriverID(d.ID) {
			t.Fatalf("displayed driver %q not canonical", d.ID)
		}
	}
}

func hasUnknownDriverWarning(ws []adapter.MismatchWarning) bool {
	for _, w := range ws {
		if w.Reason == adapter.ReasonAIRiskDriverUnknownID {
			return true
		}
	}
	return false
}
