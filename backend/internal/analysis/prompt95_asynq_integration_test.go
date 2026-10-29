//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/hibiken/asynq"
)

func TestPrompt95AsynqStages1To6V3(t *testing.T) {
	redisURL := integrationRedisURL(t)
	f := prompt93Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt95-asynq", runtime.AsynqEnqueueFunc(client))
	if err != nil {
		t.Fatal(err)
	}

	mux := asynq.NewServeMux()
	runtime.RegisterStageExecuteHandler(mux, exec)
	srv := asynq.NewServer(opt, asynq.Config{Concurrency: 1})
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(mux) }()
	t.Cleanup(func() {
		srv.Shutdown()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	})

	if err = runtime.EnqueueStageExecute(ctx, client, run.ID, string(stages.ResolveContext)); err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		waitStageSucceeded(t, f, run.ID, key, 90*time.Second)
	}

	coreBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeCore))
	var core landscape.LandscapeSet
	if err = json.Unmarshal(coreBody, &core); err != nil {
		t.Fatal(err)
	}
	if core.Context.LandscapeSchemaVersion != landscape.LandscapeSchemaV5 {
		t.Fatalf("core schema=%s", core.Context.LandscapeSchemaVersion)
	}
	if core.Context.LandscapeProfileVersion != landscape.CoreProfileVersionSealV5() {
		t.Fatalf("core profile seal=%s want %s", core.Context.LandscapeProfileVersion, landscape.CoreProfileVersionSealV5())
	}

	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Context.LandscapeProfileVersion != landscape.DetailProfileVersionSealV5() {
		t.Fatalf("detail profile seal mismatch")
	}

	policy := materiality.CurrentPolicy()
	if policy.Version != materiality.MaterialityPolicyV1_4 {
		t.Fatalf("unexpected materiality policy version %s", policy.Version)
	}

	coreBefore, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeCore))
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.EnqueueStageExecute(ctx, client, run.ID, string(stages.BuildLandscapeCore)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	coreAfter, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeCore))
	if err != nil {
		t.Fatal(err)
	}
	if coreAfter.OutputHash != coreBefore.OutputHash {
		t.Fatalf("duplicate Core delivery mutated hash: before=%s after=%s", coreBefore.OutputHash, coreAfter.OutputHash)
	}

	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status != "SUCCEEDED" {
			t.Fatalf("stage 11 must succeed; status=%s", s.Status)
		}
	}
}
