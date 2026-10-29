package publicationaudit

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
)

// assertNoNotChecked is the core invariant: every finding must carry a real
// ok/warning/error verdict, never a skipped/not_checked placeholder.
func assertNoNotChecked(t *testing.T, findings []Finding) {
	t.Helper()
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding, got none (would leave an empty CSV)")
	}
	for i, f := range findings {
		switch f.Status {
		case StatusOK, StatusWarning, StatusError:
		default:
			t.Fatalf("finding[%d] %q has non-verdict status %q", i, f.Check, f.Status)
		}
	}
}

func testPackIndex() *outputvalidator.PackReferenceIndex {
	return &outputvalidator.PackReferenceIndex{
		SKUIds:      map[string]struct{}{"801000100001": {}},
		CampaignIds: map[string]struct{}{"555": {}},
	}
}

func TestSourceReconciliationValidSnapshot(t *testing.T) {
	rep := sourcerecon.Finalize(sourcerecon.Report{})
	findings := SourceReconciliation(SourceInputs{
		Live:         rep,
		LiveLoaded:   true,
		Stored:       rep,
		StoredLoaded: true,
	})
	assertNoNotChecked(t, findings)
	if s := summarize(findings); !s.Valid {
		t.Fatalf("expected valid source layer, got %+v", s)
	}
}

func TestSourceReconciliationMissingArtifactIsError(t *testing.T) {
	findings := SourceReconciliation(SourceInputs{
		Live:       sourcerecon.Finalize(sourcerecon.Report{}),
		LiveLoaded: true,
		// StoredLoaded intentionally false.
	})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "source_recon_artifact_present") {
		t.Fatalf("expected missing-artifact error, got %+v", findings)
	}
}

func TestAIClaimsMissingOutputIsError(t *testing.T) {
	findings := AIClaimsReconciliation(AIClaimInputs{IndexLoaded: true, PackIndex: testPackIndex()})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "ai_output_present") {
		t.Fatalf("expected missing-output error, got %+v", findings)
	}
}

func TestAIClaimsMissingPackIndexIsError(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{}
	findings := AIClaimsReconciliation(AIClaimInputs{Output: out, IndexLoaded: false})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "pack_index_loadable") {
		t.Fatalf("expected pack-index error, got %+v", findings)
	}
}

func TestDashboardReconciliationCountMismatchIsError(t *testing.T) {
	rev := 10.0
	orders := int32(2)
	facts := dashboardfacts.DashboardFactsV1{
		PackHash:   "hash-1",
		MetricDate: "2026-07-08",
		SellerPulse: dashboardfacts.SellerPulseFacts{
			KPIs:    []dashboardfacts.KPICardFact{{Key: "revenue"}},
			Trend7D: []dashboardfacts.TrendDayPoint{{Date: "2026-07-08", Revenue: &rev, Orders: &orders}},
		},
	}
	frozen := dashboardhome.FrozenDashboardData{MetricDate: "2026-07-08"}
	frozen.SellerPulse.KPIs = nil // mismatch vs facts (1 vs 0)

	findings := DashboardReconciliation(DashboardInputs{
		Facts:        facts,
		FactsLoaded:  true,
		Frozen:       frozen,
		FrozenLoaded: true,
		PackHash:     "hash-1",
		MetricDate:   "2026-07-08",
	})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "dashboard_count_seller_pulse_kpis") {
		t.Fatalf("expected kpi count mismatch error, got %+v", findings)
	}
	if !hasOKCheck(findings, "facts_pack_hash_matches_run") {
		t.Fatalf("expected pack hash match ok, got %+v", findings)
	}
}

func TestDashboardReconciliationMissingFactsIsError(t *testing.T) {
	findings := DashboardReconciliation(DashboardInputs{FrozenLoaded: true, PackHash: "h", MetricDate: "2026-07-08"})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "dashboard_facts_present") {
		t.Fatalf("expected missing facts error, got %+v", findings)
	}
}

func TestActionsReconciliationEmptyEvidenceIsError(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{{AnalysisActionID: "action_001"}},
	}
	findings := ActionsReconciliation(ActionsInputs{
		Output:      out,
		Persisted:   []PersistedAction{{ActionKey: "unified:pack:action_001", EvidenceFactIDs: []string{"", "  "}}},
		PackIndex:   testPackIndex(),
		IndexLoaded: true,
	})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "action_evidence_fact_ids_non_empty") {
		t.Fatalf("expected empty-evidence error, got %+v", findings)
	}
}

func TestActionsReconciliationUnresolvedFactIsError(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{{AnalysisActionID: "action_001"}},
	}
	findings := ActionsReconciliation(ActionsInputs{
		Output:      out,
		Persisted:   []PersistedAction{{ActionKey: "k", EvidenceFactIDs: []string{"sku:999999999999:orders_7d"}}},
		PackIndex:   testPackIndex(),
		IndexLoaded: true,
	})
	assertNoNotChecked(t, findings)
	if !hasErrorCheck(findings, "action_evidence_facts_exist") {
		t.Fatalf("expected unresolved-fact error, got %+v", findings)
	}
}

func TestActionsReconciliationGroundedActionPasses(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{{AnalysisActionID: "action_001"}},
	}
	findings := ActionsReconciliation(ActionsInputs{
		Output:      out,
		Persisted:   []PersistedAction{{ActionKey: "k", EvidenceFactIDs: []string{"sku:801000100001:orders_7d"}}},
		PackIndex:   testPackIndex(),
		IndexLoaded: true,
	})
	assertNoNotChecked(t, findings)
	if s := summarize(findings); !s.Valid {
		t.Fatalf("expected valid actions layer, got %+v (%+v)", s, findings)
	}
}

func TestActionsReconciliationNoActionsOutcomeIsOK(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{}
	findings := ActionsReconciliation(ActionsInputs{Output: out, IndexLoaded: true, PackIndex: testPackIndex()})
	assertNoNotChecked(t, findings)
	if s := summarize(findings); !s.Valid {
		t.Fatalf("expected valid empty-actions layer, got %+v", s)
	}
}

func TestBuildReportAggregatesValidity(t *testing.T) {
	ok := []Finding{{Check: "x", Status: StatusOK}}
	bad := []Finding{{Check: "y", Status: StatusError}}
	if r := BuildReport(ok, ok, ok, ok); !r.Valid {
		t.Fatalf("expected valid aggregate, got %+v", r.Layers)
	}
	if r := BuildReport(ok, bad, ok, ok); r.Valid {
		t.Fatalf("expected invalid aggregate when a layer errors")
	}
}

func hasErrorCheck(findings []Finding, check string) bool {
	for _, f := range findings {
		if f.Check == check && f.Status == StatusError {
			return true
		}
	}
	return false
}

func hasOKCheck(findings []Finding, check string) bool {
	for _, f := range findings {
		if f.Check == check && f.Status == StatusOK {
			return true
		}
	}
	return false
}
