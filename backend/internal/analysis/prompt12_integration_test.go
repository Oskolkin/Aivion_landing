//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt12DeepNonEmptyStages1to13(t *testing.T) {
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt12-exec", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
	}
	st11, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildDeepPack))
	if err != nil || st11.AcceptedAttemptID == nil {
		t.Fatalf("stage11 %+v %v", st11, err)
	}
	for _, kind := range []string{string(artifacts.DeepPack), string(artifacts.DeepRequestManifest), string(artifacts.DeepActionMenu), string(artifacts.DeepRelationshipPolicy)} {
		if _, _, _, err = f.repo.ArtifactByStageAttemptAndKind(ctx, run.ID, *st11.AcceptedAttemptID, kind); err != nil {
			t.Fatalf("artifact %s: %v", kind, err)
		}
	}
	st12, err := f.repo.GetStage(ctx, run.ID, string(stages.DeepAI))
	if err != nil || st12.AcceptedAttemptID == nil {
		t.Fatal(err)
	}
	if _, _, _, err = f.repo.ArtifactByStageAttemptAndKind(ctx, run.ID, *st12.AcceptedAttemptID, string(artifacts.DeepOutput)); err != nil {
		t.Fatal(err)
	}
	var nAI int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_ai_attempts a JOIN analysis_stage_attempts s ON s.id=a.stage_attempt_id
		JOIN analysis_run_stages st ON st.id=s.stage_id WHERE st.run_id=$1 AND st.stage_key=$2`, run.ID, string(stages.DeepAI)).Scan(&nAI)
	if nAI < 1 {
		t.Fatalf("ai attempts=%d", nAI)
	}
	st13, err := f.repo.GetStage(ctx, run.ID, string(stages.ValidateDeep))
	if err != nil || st13.Status != "SUCCEEDED" {
		t.Fatalf("stage13 %+v %v", st13, err)
	}
	body := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.DeepValidatedOutput))
	var validated deep.ValidatedOutputV1
	if err = json.Unmarshal(body, &validated); err != nil {
		t.Fatal(err)
	}
	if len(validated.Output.ThreadResolutions) < 1 || len(validated.Output.Findings) < 1 || len(validated.Output.Recommendations) < 1 {
		t.Fatalf("semantic graph %#v", validated.Output)
	}
	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status != "SUCCEEDED" && s.Status != "SUCCEEDED_WITH_LIMITATIONS" {
			t.Fatalf("stage 14 status=%s", s.Status)
		}
	}
	var frontier bool
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM analysis_run_events WHERE run_id=$1 AND event_type='IMPLEMENTED_FRONTIER_REACHED' AND payload_json->>'last_stage'='POST_PUBLISH')`, run.ID).Scan(&frontier)
	if err != nil || !frontier {
		t.Fatalf("frontier err=%v found=%v", err, frontier)
	}
}

func TestPrompt12NoMeaningfulThreadsDeep(t *testing.T) {
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "0")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt12-empty", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
	}
	body := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.DeepValidatedOutput))
	var validated deep.ValidatedOutputV1
	if err = json.Unmarshal(body, &validated); err != nil {
		t.Fatal(err)
	}
	if len(validated.Output.ThreadResolutions) != 0 || len(validated.Output.ActionProposals) != 0 {
		t.Fatalf("no meaningful invariants %#v", validated.Output)
	}
	if validated.Output.ExecutiveSummary.Headline == "" {
		t.Fatal("account summary missing")
	}
}
