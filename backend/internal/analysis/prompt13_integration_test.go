//go:build integration

package analysis_test

import (
	"context"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func (f prompt81Fixture) prodRun(t *testing.T) analysispg.Run {
	t.Helper()
	run, err := f.repo.CreateProductionRun(context.Background(), analysispg.Run{
		SellerID: f.seller, AnalysisDate: f.cut.AddDate(0, 0, -1), CutoffAt: f.cut,
		ContractHash: f.bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.repo.EnsureStages(context.Background(), run.ID, stages.ProductionImplementedStageNames()); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestPrompt13FirstPublicationStages1to19(t *testing.T) {
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.prodRun(t)
	if run.BasePublicationID != nil {
		t.Fatal("first run must have null base")
	}
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt13-p1", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
	}
	cur, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil || cur.ID == 0 || cur.Generation != 1 {
		t.Fatalf("current %+v %v", cur, err)
	}
	var nPub, nOutbox int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, run.ID).Scan(&nPub)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE event_type='publication.current_changed'`).Scan(&nOutbox)
	if nPub != 1 || nOutbox < 1 {
		t.Fatalf("pub=%d outbox=%d", nPub, nOutbox)
	}
	ov, err := f.repo.GetProjection(ctx, cur.ID, "OVERVIEW")
	if err != nil || len(ov) == 0 {
		t.Fatalf("overview %v", err)
	}
	var frontier bool
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM analysis_run_events WHERE run_id=$1 AND event_type='IMPLEMENTED_FRONTIER_REACHED' AND payload_json->>'last_stage'='POST_PUBLISH')`, run.ID).Scan(&frontier)
	if err != nil || !frontier {
		t.Fatalf("frontier %v %v", err, frontier)
	}
}

func TestPrompt13EvaluationCannotPublish(t *testing.T) {
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt13-eval", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
	}
	var n int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, run.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("evaluation published %d", n)
	}
}

func TestPrompt13SecondRunContinuity(t *testing.T) {
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	f := prompt91Seed(t)
	ctx := context.Background()
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt13-p2", nil)
	if err != nil {
		t.Fatal(err)
	}
	r1 := f.prodRun(t)
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, r1.ID, key); err != nil {
			t.Fatalf("p1 stage %s: %v", key, err)
		}
	}
	p1, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	r2 := f.prodRun(t)
	if r2.BasePublicationID == nil || *r2.BasePublicationID != p1.ID {
		t.Fatalf("base=%v want %d", r2.BasePublicationID, p1.ID)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, r2.ID, key); err != nil {
			t.Fatalf("p2 stage %s: %v", key, err)
		}
	}
	p2, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil || p2.ID == p1.ID || p2.Generation != 2 {
		t.Fatalf("p2 %+v p1 %+v %v", p2, p1, err)
	}
}
