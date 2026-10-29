//go:build integration

package analysis_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/hibiken/asynq"
)

func TestPrompt81AsynqStageExecute(t *testing.T) {
	redisURL := strings.TrimSpace(os.Getenv("TEST_REDIS_URL"))
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL")
	}
	f := prompt81Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt81-asynq", runtime.AsynqEnqueueFunc(client))
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
	waitStageSucceeded(t, f, run.ID, string(stages.ResolveContext), 30*time.Second)
	waitStageSucceeded(t, f, run.ID, string(stages.SealUniverse), 30*time.Second)

	got, err := f.repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UniverseID == "" {
		t.Fatal("asynq path did not bind factual universe")
	}

	// Duplicate delivery of already-succeeded RESOLVE must be a safe no-op.
	if err = runtime.EnqueueStageExecute(ctx, client, run.ID, string(stages.ResolveContext)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	st, err := f.repo.GetStage(ctx, run.ID, string(stages.ResolveContext))
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "SUCCEEDED" && st.Status != "SUCCEEDED_WITH_LIMITATIONS" {
		t.Fatalf("duplicate delivery corrupted resolve stage: %s", st.Status)
	}
}

func TestPrompt81AsynqRecoveryDispatch(t *testing.T) {
	redisURL := strings.TrimSpace(os.Getenv("TEST_REDIS_URL"))
	if redisURL == "" {
		t.Skip("set TEST_REDIS_URL")
	}
	f := prompt81Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt81-recovery", runtime.AsynqEnqueueFunc(client))
	if err != nil {
		t.Fatal(err)
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

	// Simulate commit-before-enqueue gap: stage READY, no prior enqueue.
	// Recovery scan must observe it; then dispatch that exact candidate through Asynq
	// (global Scan may see older READY rows from other tests first).
	disp := runtime.RecoveryDispatcher{Enabled: true, Executor: exec, Enqueue: runtime.AsynqEnqueueFunc(client)}
	items, err := disp.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var hit *runtime.WorkItem
	for i := range items {
		if items[i].RunID == run.ID && items[i].StageKey == string(stages.ResolveContext) {
			hit = &items[i]
			break
		}
	}
	if hit == nil {
		t.Fatalf("recovery scan missed READY RESOLVE_CONTEXT for run %d; items=%+v", run.ID, items)
	}
	if err = runtime.EnqueueStageExecute(ctx, client, hit.RunID, hit.StageKey); err != nil {
		t.Fatal(err)
	}
	waitStageSucceeded(t, f, run.ID, string(stages.ResolveContext), 30*time.Second)
	waitStageSucceeded(t, f, run.ID, string(stages.SealUniverse), 30*time.Second)
}

func waitStageSucceeded(t *testing.T, f prompt81Fixture, runID int64, stageKey string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := f.repo.GetStage(context.Background(), runID, stageKey)
		if err == nil && (st.Status == "SUCCEEDED" || st.Status == "SUCCEEDED_WITH_LIMITATIONS") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	st, _ := f.repo.GetStage(context.Background(), runID, stageKey)
	t.Fatalf("timeout waiting for %s SUCCEEDED; last=%+v", stageKey, st)
}
