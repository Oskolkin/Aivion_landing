//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt11ExpandEvidenceNonEmptyStages1to10(t *testing.T) {
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt11-exec", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
		st, err := f.repo.GetStage(ctx, run.ID, key)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != "SUCCEEDED" && st.Status != "SUCCEEDED_WITH_LIMITATIONS" {
			t.Fatalf("stage %s status=%s err=%s", key, st.Status, st.ErrorMessage)
		}
	}
	st10, err := f.repo.GetStage(ctx, run.ID, string(stages.ExpandEvidence))
	if err != nil {
		t.Fatal(err)
	}
	if st10.AcceptedAttemptID == nil {
		t.Fatal("stage 10 accepted attempt missing")
	}
	for _, kind := range []string{
		string(artifacts.EvidenceRequestSet), string(artifacts.EvidenceBundle),
		string(artifacts.AnalysisFactHandleRegistry), string(artifacts.EvidenceExpansionReport),
	} {
		_, _, _, err = f.repo.ArtifactByStageAttemptAndKind(ctx, run.ID, *st10.AcceptedAttemptID, kind)
		if err != nil {
			t.Fatalf("artifact %s: %v", kind, err)
		}
	}
	body := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.EvidenceBundle))
	var bundle evidence.BundleV1
	if err = json.Unmarshal(body, &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Summary.Unique < 2 {
		t.Fatalf("expected deduped unique>=2 got %+v", bundle.Summary)
	}
	if len(bundle.Items) == 0 {
		t.Fatal("expected non-empty evidence items")
	}
	complete := 0
	for _, it := range bundle.Items {
		if it.MaterializationStatus == evidence.StatusComplete || it.MaterializationStatus == evidence.StatusEmptyValid || it.MaterializationStatus == evidence.StatusPartial {
			complete++
		}
	}
	if complete == 0 {
		t.Fatalf("no materialized items: %+v", bundle.Items)
	}
	reqBody := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.EvidenceRequestSet))
	var rs evidence.RequestSetV1
	if err = json.Unmarshal(reqBody, &rs); err != nil {
		t.Fatal(err)
	}
	if rs.RawRequestCount < rs.UniqueRequestCount {
		t.Fatalf("raw < unique %+v", rs)
	}
	priceRefs := 0
	for _, rel := range rs.ThreadRelations {
		for _, n := range rs.Requests {
			if n.RequestKey == rel.RequestKey && n.EvidenceType == "SKU_PRICE_HISTORY" {
				priceRefs++
			}
		}
	}
	if priceRefs < 2 && rs.RawRequestCount >= 4 {
		t.Fatalf("expected SKU_PRICE_HISTORY cross-thread relations, got %d rels=%+v", priceRefs, rs.ThreadRelations)
	}
	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status == "READY" {
			t.Fatal("stage 14 READY")
		}
	}
	var nAI int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_ai_attempts a JOIN analysis_stage_attempts s ON s.id=a.stage_attempt_id
		JOIN analysis_run_stages st ON st.id=s.stage_id WHERE st.run_id=$1 AND st.stage_key=$2`, run.ID, string(stages.ExpandEvidence)).Scan(&nAI)
	if nAI != 0 {
		t.Fatalf("stage 10 ai attempts=%d", nAI)
	}
	var frontier bool
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM analysis_run_events WHERE run_id=$1 AND event_type='IMPLEMENTED_FRONTIER_REACHED' AND payload_json->>'last_stage'='POST_PUBLISH')`, run.ID).Scan(&frontier)
	if err != nil || !frontier {
		t.Fatalf("frontier: err=%v found=%v", err, frontier)
	}
	got, err := f.repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SellerID != f.seller {
		t.Fatal("seller")
	}
}
