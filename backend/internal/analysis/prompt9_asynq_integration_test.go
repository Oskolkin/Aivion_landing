//go:build integration

package analysis_test

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/hibiken/asynq"
)

func TestPrompt9AsynqStages1to6(t *testing.T) {
	redisURL := integrationRedisURL(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt9-asynq", runtime.AsynqEnqueueFunc(client))
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

	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status != "SUCCEEDED" {
			t.Fatalf("stage 11 must succeed; status=%s", s.Status)
		}
	}
	var frontier bool
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM analysis_run_events WHERE run_id=$1 AND event_type='IMPLEMENTED_FRONTIER_REACHED'
		AND payload_json->>'last_stage'='POST_PUBLISH')`, run.ID).Scan(&frontier)
	if err != nil || !frontier {
		t.Fatalf("expected POST_PUBLISH frontier: err=%v found=%v", err, frontier)
	}

	// Duplicate delivery of Detail must remain idempotent.
	detailBefore, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeDetail))
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.EnqueueStageExecute(ctx, client, run.ID, string(stages.BuildLandscapeDetail)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	detailAfter, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeDetail))
	if err != nil {
		t.Fatal(err)
	}
	if detailAfter.Status != detailBefore.Status || detailAfter.OutputHash != detailBefore.OutputHash {
		t.Fatalf("duplicate Detail mutated stage: before=%+v after=%+v", detailBefore, detailAfter)
	}
}

func TestPrompt9AsynqRecoveryLandscapeCore(t *testing.T) {
	redisURL := integrationRedisURL(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt9-recovery", runtime.AsynqEnqueueFunc(client))
	if err != nil {
		t.Fatal(err)
	}
	// Drive stages 1–3 directly so Core is READY without enqueue chain.
	for _, key := range []string{
		string(stages.ResolveContext), string(stages.SealUniverse), string(stages.PrepareContinuity),
	} {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	if err = exec.EnsureExecutableInputHashes(ctx, run.ID); err != nil {
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

	disp := runtime.RecoveryDispatcher{Enabled: true, Executor: exec, Enqueue: runtime.AsynqEnqueueFunc(client)}
	items, err := disp.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var hit *runtime.WorkItem
	for i := range items {
		if items[i].RunID == run.ID && items[i].StageKey == string(stages.BuildLandscapeCore) {
			hit = &items[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("recovery missed READY BUILD_LANDSCAPE_CORE for run %d; items=%+v", run.ID, items)
	}
	if err = runtime.EnqueueStageExecute(ctx, client, hit.RunID, hit.StageKey); err != nil {
		t.Fatal(err)
	}
	waitStageSucceeded(t, f, run.ID, string(stages.BuildLandscapeCore), 90*time.Second)
	waitStageSucceeded(t, f, run.ID, string(stages.RunMateriality), 90*time.Second)
	waitStageSucceeded(t, f, run.ID, string(stages.BuildLandscapeDetail), 90*time.Second)
}
