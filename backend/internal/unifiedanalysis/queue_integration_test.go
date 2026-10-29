//go:build asynq_integration || integration

package unifiedanalysis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func createQueueSeller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	email := fmt.Sprintf("ua-asynq-%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status) VALUES ($1, $2, 'active') RETURNING id`,
		userID, "ua-asynq").Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	return sellerID
}

func asynqRedisConfig(t *testing.T) jobs.RedisConfig {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return jobs.RedisConfig{Addr: addr, DB: 14}
}

func asynqDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run asynq/unified analysis integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}
	return pool
}

type stubPackBuilder struct {
	result *pack.BuildResult
	key    string
	calls  int
}

func (s *stubPackBuilder) BuildAndStore(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*pack.BuildResult, string, error) {
	s.calls++
	if s.result == nil {
		return nil, s.key, nil
	}
	out := *s.result
	if out.Snapshot == nil {
		out.Snapshot = unifiedanalysis.MinimalTestSnapshot(sellerAccountID, metricDate)
	}
	return &out, s.key, nil
}

type memoryPackStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memoryPackStore) Get(ctx context.Context, objectKey string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.data[objectKey]...), nil
}

func (m *memoryPackStore) Put(key string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = make(map[string][]byte)
	}
	m.data[key] = append([]byte(nil), data...)
}

type recordingEnqueue struct {
	inner     jobs.UnifiedAnalysisEnqueue
	mu        sync.Mutex
	starts    int
	polls     int
	cleanups  int
	cancels   int
	validates int
	publishes int
}

func (r *recordingEnqueue) EnqueueStart(ctx context.Context, runID int64) error {
	r.mu.Lock()
	r.starts++
	r.mu.Unlock()
	return r.inner.EnqueueStart(ctx, runID)
}

func (r *recordingEnqueue) EnqueuePoll(ctx context.Context, runID int64, pollAttempt int, delay time.Duration) error {
	r.mu.Lock()
	r.polls++
	r.mu.Unlock()
	return r.inner.EnqueuePoll(ctx, runID, pollAttempt, delay)
}

func (r *recordingEnqueue) EnqueueCleanup(ctx context.Context, runID int64) error {
	r.mu.Lock()
	r.cleanups++
	r.mu.Unlock()
	return r.inner.EnqueueCleanup(ctx, runID)
}

func (r *recordingEnqueue) EnqueueCancel(ctx context.Context, runID int64) error {
	r.mu.Lock()
	r.cancels++
	r.mu.Unlock()
	return r.inner.EnqueueCancel(ctx, runID)
}

func (r *recordingEnqueue) EnqueueValidateOutput(ctx context.Context, runID int64) error {
	r.mu.Lock()
	r.validates++
	r.mu.Unlock()
	return r.inner.EnqueueValidateOutput(ctx, runID)
}

func (r *recordingEnqueue) EnqueuePublish(ctx context.Context, runID int64) error {
	r.mu.Lock()
	r.publishes++
	r.mu.Unlock()
	return r.inner.EnqueuePublish(ctx, runID)
}

func waitRun(t *testing.T, repo *unifiedanalysis.Repository, runID int64, timeout time.Duration, pred func(*unifiedanalysis.Run) bool) *unifiedanalysis.Run {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last *unifiedanalysis.Run
	for time.Now().Before(deadline) {
		run, err := repo.GetRun(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetRun: %v", err)
		}
		last = run
		if pred(run) {
			return run
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for run predicate; last status=%q cleanup=%q", last.Status, last.CleanupStatus)
	return last
}

func startEmbeddedWorker(t *testing.T, cfg jobs.RedisConfig, mux *asynq.ServeMux) (context.CancelFunc, <-chan error) {
	t.Helper()
	server := jobs.NewAsynqServer(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- jobs.RunAsynqUntilContext(ctx, server, mux)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errCh:
		case <-time.After(15 * time.Second):
		}
	})
	time.Sleep(300 * time.Millisecond)
	return cancel, errCh
}

func TestAsynqQueueHappyPathFakeProvider(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })

	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisCfg.Addr, Password: redisCfg.Password, DB: redisCfg.DB})
	t.Cleanup(func() { _ = inspector.Close() })
	_, _ = inspector.DeleteAllPendingTasks("default")
	_, _ = inspector.DeleteAllScheduledTasks("default")
	_, _ = inspector.DeleteAllRetryTasks("default")

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:2026-07-08:queuehappy", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: "aivion_analysis_pack_v1",
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: fmt.Sprintf("ai-analysis-packs/seller-%d/2026-07-08/abc/analysis_pack.zip", sellerID),
	}
	store := &memoryPackStore{}
	store.Put(stub.key, zip)

	fake := provider.NewFakeProvider(provider.FakeScenarioQueuedThenCompleted)
	fake.SetSmokePackID(packID)

	cfg := unifiedanalysis.Config{
		Enabled:                true,
		Provider:               unifiedanalysis.ProviderNameOpenAI,
		Model:                  "fake-model",
		MemoryLimit:            "4g",
		MaxOutputTokens:        1024,
		PollIntervalSeconds:    1,
		MaxPollDurationSeconds: 60,
		CleanupEnabled:         true,
		DebugArtifacts:         true,
	}
	svc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, cfg)
	inner := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	rec := &recordingEnqueue{inner: inner}
	handler := jobs.NewUnifiedAnalysisHandler(svc, rec, zap.NewNop())
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, handler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), true)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := inner.EnqueueStart(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueStart: %v", err)
	}

	final := waitRun(t, repo, run.ID, 45*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusProviderCompleted &&
			r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted
	})

	if final.ProviderResponseID == "" {
		t.Fatal("provider_response_id empty")
	}
	if final.CodeInterpreterCalls < 1 {
		t.Fatalf("code_interpreter_calls=%d", final.CodeInterpreterCalls)
	}
	if fake.CreateContainerCount != 1 {
		t.Fatalf("CreateContainer=%d", fake.CreateContainerCount)
	}
	if fake.UploadCount != 1 {
		t.Fatalf("Upload=%d", fake.UploadCount)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponse=%d", fake.StartResponseCount)
	}
	if fake.RetrieveCount < 2 {
		t.Fatalf("Retrieve=%d want >=2", fake.RetrieveCount)
	}
	if fake.DeleteFileCount != 1 || fake.DeleteContainerCount != 1 {
		t.Fatalf("cleanup deletes file=%d container=%d", fake.DeleteFileCount, fake.DeleteContainerCount)
	}
	rec.mu.Lock()
	polls, cleanups := rec.polls, rec.cleanups
	rec.mu.Unlock()
	if polls < 2 {
		t.Fatalf("poll tasks enqueued=%d want >=2", polls)
	}
	if cleanups < 1 {
		t.Fatalf("cleanup tasks enqueued=%d", cleanups)
	}
	t.Logf("queue evidence run_id=%d polls_enqueued=%d cleanups_enqueued=%d retrieve=%d",
		run.ID, polls, cleanups, fake.RetrieveCount)
}

func TestAsynqRecoveryProviderRunning(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisCfg.Addr, Password: redisCfg.Password, DB: redisCfg.DB})
	t.Cleanup(func() { _ = inspector.Close() })
	_, _ = inspector.DeleteAllPendingTasks("default")
	_, _ = inspector.DeleteAllScheduledTasks("default")

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:2026-07-08:recovery", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: "aivion_analysis_pack_v1",
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: fmt.Sprintf("ai-analysis-packs/seller-%d/recovery/analysis_pack.zip", sellerID),
	}
	store := &memoryPackStore{}
	store.Put(stub.key, zip)

	fake := provider.NewFakeProvider(provider.FakeScenarioInProgressThenCompleted)
	fake.SetSmokePackID(packID)
	cfg := unifiedanalysis.Config{
		Enabled: true, Provider: unifiedanalysis.ProviderNameOpenAI, Model: "fake-model",
		MemoryLimit: "4g", MaxOutputTokens: 1024, PollIntervalSeconds: 30,
		MaxPollDurationSeconds: 120, CleanupEnabled: true,
	}
	svc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, cfg)
	inner := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	handler := jobs.NewUnifiedAnalysisHandler(svc, inner, zap.NewNop())
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, handler)

	cancel, errCh := startEmbeddedWorker(t, redisCfg, mux)
	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), true)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := inner.EnqueueStart(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueStart: %v", err)
	}

	running := waitRun(t, repo, run.ID, 20*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusProviderRunning && r.ProviderResponseID != ""
	})
	responseBefore := running.ProviderResponseID
	containerBefore := running.ProviderContainerID

	// Stop worker before next successful poll can complete.
	cancel()
	select {
	case <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("worker did not stop")
	}
	_, _ = inspector.DeleteAllPendingTasks("default")
	_, _ = inspector.DeleteAllScheduledTasks("default")

	still, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if still.Status != unifiedanalysis.StatusProviderRunning {
		t.Fatalf("status while stopped=%q", still.Status)
	}

	// Restart worker with recovery enqueue (mirrors cmd/worker).
	mux2 := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux2, handler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux2)

	plans, err := svc.PlanRecovery(ctx, 50)
	if err != nil {
		t.Fatalf("PlanRecovery: %v", err)
	}
	recovered := false
	for _, plan := range plans {
		if plan.RunID != run.ID {
			continue
		}
		recovered = true
		if plan.Action != unifiedanalysis.RecoveryActionPoll {
			t.Fatalf("recovery action=%s want poll", plan.Action)
		}
		if err := inner.EnqueuePoll(ctx, plan.RunID, 1, 0); err != nil {
			t.Fatalf("EnqueuePoll recovery: %v", err)
		}
	}
	if !recovered {
		t.Fatal("PlanRecovery did not find provider_running run")
	}

	final := waitRun(t, repo, run.ID, 45*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusProviderCompleted &&
			r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted
	})
	if final.ProviderResponseID != responseBefore {
		t.Fatalf("response id changed %q -> %q", responseBefore, final.ProviderResponseID)
	}
	if final.ProviderContainerID != containerBefore {
		t.Fatalf("container id changed %q -> %q", containerBefore, final.ProviderContainerID)
	}
	if fake.CreateContainerCount != 1 || fake.StartResponseCount != 1 {
		t.Fatalf("duplicate provider resources create=%d start=%d", fake.CreateContainerCount, fake.StartResponseCount)
	}
	t.Logf("recovery evidence run_id=%d response_id=%s container_id=%s", run.ID, final.ProviderResponseID, final.ProviderContainerID)
}

func TestAsynqRecoveryCleanupPending(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:2026-07-08:cleanuprec", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: "aivion_analysis_pack_v1",
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: fmt.Sprintf("ai-analysis-packs/seller-%d/cleanup/analysis_pack.zip", sellerID),
	}
	store := &memoryPackStore{}
	store.Put(stub.key, zip)
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	cfg := unifiedanalysis.Config{
		Enabled: true, Provider: unifiedanalysis.ProviderNameOpenAI, Model: "fake-model",
		MemoryLimit: "4g", MaxOutputTokens: 1024, PollIntervalSeconds: 1,
		MaxPollDurationSeconds: 60, CleanupEnabled: true,
	}
	svc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, cfg)

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), true)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil || !poll.Completed {
		t.Fatalf("PollRun: err=%v", err)
	}
	mid, _ := repo.GetRun(ctx, run.ID)
	if mid.CleanupStatus != unifiedanalysis.CleanupStatusPending {
		t.Fatalf("cleanup_status=%q", mid.CleanupStatus)
	}

	plans, err := svc.PlanRecovery(ctx, 50)
	if err != nil {
		t.Fatalf("PlanRecovery: %v", err)
	}
	found := false
	for _, plan := range plans {
		if plan.RunID == run.ID && plan.Action == unifiedanalysis.RecoveryActionCleanup {
			found = true
		}
	}
	if !found {
		t.Fatal("expected cleanup recovery action")
	}

	inner := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	handler := jobs.NewUnifiedAnalysisHandler(svc, inner, zap.NewNop())
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, handler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	if err := inner.EnqueueCleanup(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueCleanup: %v", err)
	}
	final := waitRun(t, repo, run.ID, 30*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusProviderCompleted &&
			r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted
	})
	_ = final
	if fake.DeleteFileCount < 1 || fake.DeleteContainerCount < 1 {
		t.Fatalf("cleanup deletes file=%d container=%d", fake.DeleteFileCount, fake.DeleteContainerCount)
	}
}

func TestAsynqPollTimeout(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:2026-07-08:timeout", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: "aivion_analysis_pack_v1",
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: fmt.Sprintf("ai-analysis-packs/seller-%d/timeout/analysis_pack.zip", sellerID),
	}
	store := &memoryPackStore{}
	store.Put(stub.key, zip)
	fake := provider.NewFakeProvider(provider.FakeScenarioInProgressUntilTimeout)
	fake.SetSmokePackID(packID)
	cfg := unifiedanalysis.Config{
		Enabled: true, Provider: unifiedanalysis.ProviderNameOpenAI, Model: "fake-model",
		MemoryLimit: "4g", MaxOutputTokens: 1024, PollIntervalSeconds: 1,
		MaxPollDurationSeconds: 2, CleanupEnabled: true,
	}
	svc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, cfg)
	inner := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	handler := jobs.NewUnifiedAnalysisHandler(svc, inner, zap.NewNop())
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, handler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), true)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := inner.EnqueueStart(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueStart: %v", err)
	}

	final := waitRun(t, repo, run.ID, 45*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusFailed &&
			r.ErrorCode == unifiedanalysis.CodeProviderPollTimeout &&
			(r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted ||
				r.CleanupStatus == unifiedanalysis.CleanupStatusFailed ||
				r.ProviderContainerID == "")
	})
	if final.ErrorCode != unifiedanalysis.CodeProviderPollTimeout {
		t.Fatalf("error_code=%q status=%q cleanup=%q", final.ErrorCode, final.Status, final.CleanupStatus)
	}
	if fake.CancelCount < 1 {
		t.Fatalf("CancelCount=%d", fake.CancelCount)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
	if fake.RetrieveCount < 1 {
		t.Fatalf("RetrieveCount=%d", fake.RetrieveCount)
	}
}

func analysisQueueConfig() unifiedanalysis.Config {
	return unifiedanalysis.Config{
		Enabled:                 true,
		Provider:                unifiedanalysis.ProviderNameOpenAI,
		Model:                   "fake-model",
		MemoryLimit:             "4g",
		MaxOutputTokens:         1024,
		PollIntervalSeconds:     1,
		MaxPollDurationSeconds:  60,
		CleanupEnabled:          true,
		DebugArtifacts:          true,
		PromptProfile:           "analysis",
		PromptVersion:           unifiedanalysis.PromptVersionAnalyst,
		OutputValidationEnabled: true,
	}
}

func clearAsynqQueues(t *testing.T, redisCfg jobs.RedisConfig) {
	t.Helper()
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisCfg.Addr, Password: redisCfg.Password, DB: redisCfg.DB})
	t.Cleanup(func() { _ = inspector.Close() })
	_, _ = inspector.DeleteAllPendingTasks("default")
	_, _ = inspector.DeleteAllScheduledTasks("default")
	_, _ = inspector.DeleteAllRetryTasks("default")
}

func minimalReferencePackZip(t *testing.T, packID string) []byte {
	t.Helper()
	zipBytes, _ := unifiedanalysis.MustMinimalValidPackZip(packID, 1, "2026-07-08")
	return zipBytes
}

func loadNoActionsAnalysisOutput(t *testing.T, packID, packHash string) []byte {
	t.Helper()
	fixturePath := filepath.Join("contract", "testdata", "unified_output_v1.no_actions.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(fixture, &out); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	out.AnalysisMeta.PackID = packID
	out.AnalysisMeta.PackHash = packHash
	outBytes, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return outBytes
}

func setupAnalysisQueueHarness(
	t *testing.T,
	pool *pgxpool.Pool,
	redisCfg jobs.RedisConfig,
	scenario provider.FakeScenario,
) (int64, *unifiedanalysis.Service, *unifiedanalysis.Repository, *provider.FakeProvider, *jobs.AsynqUnifiedAnalysisEnqueue, *recordingEnqueue) {
	t.Helper()
	ctx := context.Background()

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })
	clearAsynqQueues(t, redisCfg)

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:2026-07-08:analysis", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stubKey := fmt.Sprintf("ai-analysis-packs/seller-%d/2026-07-08/abc/analysis_pack.zip", sellerID)
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: contract.AnalysisPackSchemaVersionV1,
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: stubKey,
	}
	store := &memoryPackStore{}
	store.Put(stubKey, zip)

	fake := provider.NewFakeProvider(scenario)
	fake.SetSmokePackID(packID)
	fake.SetAnalysisOutputJSON(loadNoActionsAnalysisOutput(t, packID, packHash))

	svc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, analysisQueueConfig())
	inner := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	rec := &recordingEnqueue{inner: inner}
	handler := jobs.NewUnifiedAnalysisHandler(svc, rec, zap.NewNop())
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, handler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	_ = ctx
	return sellerID, svc, repo, fake, inner, rec
}

func countRunArtifacts(t *testing.T, repo *unifiedanalysis.Repository, runID int64, kind string) int {
	t.Helper()
	arts, err := repo.ListArtifacts(context.Background(), runID)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	n := 0
	for _, art := range arts {
		if art.Kind == kind {
			n++
		}
	}
	return n
}

func validatedOutputHash(t *testing.T, repo *unifiedanalysis.Repository, runID int64) string {
	t.Helper()
	arts, err := repo.ListArtifacts(context.Background(), runID)
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	for _, art := range arts {
		if art.Kind != unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated {
			continue
		}
		if art.Hash.Valid && art.Hash.String != "" {
			return art.Hash.String
		}
	}
	t.Fatal("validated output artifact hash missing")
	return ""
}

func TestAsynqAnalysisQueueHappyPath(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	sellerID, svc, repo, fake, inner, rec := setupAnalysisQueueHarness(t, pool, redisCfg, provider.FakeScenarioQueuedThenCompleted)

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := inner.EnqueueStart(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueStart: %v", err)
	}

	final := waitRun(t, repo, run.ID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusPublicationReady &&
			r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted
	})

	if fake.CreateContainerCount != 1 {
		t.Fatalf("CreateContainer=%d", fake.CreateContainerCount)
	}
	if fake.UploadCount != 1 {
		t.Fatalf("Upload=%d", fake.UploadCount)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponse=%d", fake.StartResponseCount)
	}
	if countRunArtifacts(t, repo, run.ID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated) != 1 {
		t.Fatalf("validated artifact count=%d want 1", countRunArtifacts(t, repo, run.ID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated))
	}
	outputHash := validatedOutputHash(t, repo, run.ID)
	if outputHash == "" {
		t.Fatal("output hash empty")
	}

	rec.mu.Lock()
	validates, cleanups := rec.validates, rec.cleanups
	rec.mu.Unlock()
	if validates < 1 {
		t.Fatalf("validate tasks enqueued=%d want >=1", validates)
	}
	if cleanups < 1 {
		t.Fatalf("cleanup tasks enqueued=%d want >=1", cleanups)
	}
	_ = final
	t.Logf("analysis queue happy path run_id=%d output_hash=%s validates_enqueued=%d", run.ID, outputHash, validates)
}

func TestAsynqAnalysisRecoveryOutputValidating(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })
	clearAsynqQueues(t, redisCfg)

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:2026-07-08:recovery-validate", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stubKey := fmt.Sprintf("ai-analysis-packs/seller-%d/recovery-validate/analysis_pack.zip", sellerID)
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: contract.AnalysisPackSchemaVersionV1,
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: stubKey,
	}
	store := &memoryPackStore{}
	store.Put(stubKey, zip)

	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	fake.SetAnalysisOutputJSON(loadNoActionsAnalysisOutput(t, packID, packHash))

	svc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, analysisQueueConfig())

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if poll == nil || !poll.Completed || poll.Run.Status != unifiedanalysis.StatusOutputValidating {
		t.Fatalf("expected output_validating, got status=%q completed=%v", poll.Run.Status, poll.Completed)
	}
	startCountBefore := fake.StartResponseCount

	plans, err := svc.PlanRecovery(ctx, 50)
	if err != nil {
		t.Fatalf("PlanRecovery: %v", err)
	}
	foundValidate := false
	for _, plan := range plans {
		if plan.RunID != run.ID {
			continue
		}
		if plan.Action == unifiedanalysis.RecoveryActionValidate {
			foundValidate = true
		}
	}
	if !foundValidate {
		t.Fatal("PlanRecovery did not find validate action for output_validating run")
	}

	inner := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	rec := &recordingEnqueue{inner: inner}
	handler := jobs.NewUnifiedAnalysisHandler(svc, rec, zap.NewNop())
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, handler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	if err := inner.EnqueueValidateOutput(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueValidateOutput: %v", err)
	}
	if err := inner.EnqueueCleanup(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueCleanup: %v", err)
	}

	final := waitRun(t, repo, run.ID, 45*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusPublicationReady &&
			r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted
	})
	if fake.StartResponseCount != startCountBefore {
		t.Fatalf("duplicate StartResponse=%d before=%d", fake.StartResponseCount, startCountBefore)
	}
	if countRunArtifacts(t, repo, run.ID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated) != 1 {
		t.Fatal("expected one validated artifact after recovery validate")
	}
	_ = final
	t.Logf("analysis recovery validate run_id=%d output_hash=%s", run.ID, validatedOutputHash(t, repo, run.ID))
}

func TestAsynqAnalysisDuplicateValidate(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	sellerID, svc, repo, fake, inner, _ := setupAnalysisQueueHarness(t, pool, redisCfg, provider.FakeScenarioQueuedThenCompleted)

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := inner.EnqueueStart(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueStart: %v", err)
	}

	waitRun(t, repo, run.ID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusPublicationReady &&
			r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted
	})
	firstHash := validatedOutputHash(t, repo, run.ID)

	// Asynq deduplicates validate tasks by run_id (Unique TTL); re-enqueue is a no-op.
	if err := inner.EnqueueValidateOutput(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueValidateOutput duplicate: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	still, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if still.Status != unifiedanalysis.StatusPublicationReady {
		t.Fatalf("status after duplicate enqueue=%q", still.Status)
	}

	result, err := svc.ValidateOutput(ctx, run.ID)
	if err != nil {
		t.Fatalf("ValidateOutput first direct: %v", err)
	}
	if result == nil || !result.Valid || result.OutputHash != firstHash {
		t.Fatalf("first direct revalidate valid=%v hash=%q want=%q", result != nil && result.Valid, result.OutputHash, firstHash)
	}

	again, err := svc.ValidateOutput(ctx, run.ID)
	if err != nil {
		t.Fatalf("ValidateOutput second direct: %v", err)
	}
	if again == nil || !again.Valid || again.OutputHash != firstHash {
		t.Fatalf("second direct revalidate valid=%v hash=%q want=%q", again != nil && again.Valid, again.OutputHash, firstHash)
	}
	if countRunArtifacts(t, repo, run.ID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated) != 1 {
		t.Fatal("duplicate validate must not create extra validated artifacts")
	}

	_ = fake
	t.Logf("duplicate validate idempotent run_id=%d hash=%s", run.ID, firstHash)
}

func TestAsynqAnalysisValidateRefusalFailed(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	sellerID, svc, repo, fake, inner, _ := setupAnalysisQueueHarness(t, pool, redisCfg, provider.FakeScenarioQueuedThenCompleted)
	fake.SetRefusal("policy refusal for integration test")

	run, err := svc.CreateRun(ctx, sellerID, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := inner.EnqueueStart(ctx, run.ID); err != nil {
		t.Fatalf("EnqueueStart: %v", err)
	}

	final := waitRun(t, repo, run.ID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusFailed &&
			r.ErrorCode == unifiedanalysis.CodeProviderRefusal &&
			(r.CleanupStatus == unifiedanalysis.CleanupStatusCompleted ||
				r.CleanupStatus == unifiedanalysis.CleanupStatusPending ||
				r.CleanupStatus == unifiedanalysis.CleanupStatusFailed)
	})
	if final.Status == unifiedanalysis.StatusPublicationBlocked {
		t.Fatalf("refusal must not block publication, status=%q", final.Status)
	}
	if countRunArtifacts(t, repo, run.ID, unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated) != 0 {
		t.Fatal("refusal must not produce validated output artifact")
	}
	t.Logf("analysis refusal failed run_id=%d error_code=%q", run.ID, final.ErrorCode)
}

func TestAsynqPublishE2E(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()
	clearAsynqQueues(t, redisCfg)

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)
	if _, err := q.UpsertSellerAnalysisSettings(ctx, dbgen.UpsertSellerAnalysisSettingsParams{
		SellerAccountID:       sellerID,
		AnalysisMode:          "unified",
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
		UpdatedBy:             "test",
	}); err != nil {
		t.Fatalf("seller settings: %v", err)
	}

	// Seed a publication_ready run the same way Stage 4 leaves it: validated artifact present.
	policy := publication.DefaultPublicationPolicy()
	fixturePath := filepath.Join("contract", "testdata", "unified_output_v1.no_actions.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	packID := fmt.Sprintf("pack:%d:2026-07-08:pub-e2e", sellerID)
	packHash := "pub_e2e_hash_0123456789abcdef0123456789abcdef0123456789abcdef"
	out.AnalysisMeta.PackID = packID
	out.AnalysisMeta.PackHash = packHash
	out.AnalysisMeta.MetricDate = "2026-07-08"
	canonical, err := outputvalidator.CanonicalJSON(&out)
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	outputHash := outputvalidator.OutputHash(canonical)

	runRow, err := q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:               "asynq_publish_e2e",
		Status:                unifiedanalysis.StatusPublicationReady,
		DataPackSchemaVersion: pgtype.Text{String: contract.AnalysisPackSchemaVersionV1, Valid: true},
		DataPackHash:          packHash,
		PipelineVersion:       pgtype.Text{String: "test", Valid: true},
		PromptVersion:         pgtype.Text{String: policy.AcceptedPromptVersion, Valid: true},
		Provider:              pgtype.Text{String: "openai", Valid: true},
		ProviderModel:         pgtype.Text{String: "gpt-test", Valid: true},
		ProviderAttemptCount:  1,
		StartedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateUnifiedAnalysisRun: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_runs SET pack_id=$2, pack_hash=$3, code_interpreter_calls=1, cleanup_status='completed' WHERE id=$1`,
		runRow.ID, packID, packHash); err != nil {
		t.Fatalf("stamp pack: %v", err)
	}
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runRow.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   canonical,
	}); err != nil {
		t.Fatalf("validated artifact: %v", err)
	}
	valPayload, _ := json.Marshal(map[string]any{
		"prompt_version": policy.AcceptedPromptVersion,
		"prompt_hash":    policy.AcceptedPromptHash,
		"schema_version": policy.AcceptedSchemaVersion,
		"schema_hash":    policy.AcceptedSchemaHash,
		"output_hash":    outputHash,
		"valid":          true,
	})
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runRow.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidation,
		Hash:          pgtype.Text{String: "validation", Valid: true},
		Storage:       "db",
		PayloadJson:   valPayload,
	}); err != nil {
		t.Fatalf("validation artifact: %v", err)
	}
	facts := unifiedanalysis.BuildFixtureDashboardFacts(sellerID, metricDate, packHash, &out)
	if err := artifacts.SaveDashboardFacts(ctx, runRow.ID, facts); err != nil {
		t.Fatalf("SaveDashboardFacts: %v", err)
	}

	pubSvc := publication.NewService(pool, q, artifacts, policy, publication.GlobalFlags{
		RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true,
	})
	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })
	enqueue := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisPublishHandler(mux, jobs.NewUnifiedAnalysisPublishHandler(pubSvc, zap.NewNop()))
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	if err := enqueue.EnqueuePublish(ctx, runRow.ID); err != nil {
		t.Fatalf("EnqueuePublish: %v", err)
	}
	published := waitRun(t, repo, runRow.ID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusPublished
	})

	var pubID int64
	var visibility, status string
	if err := pool.QueryRow(ctx, `SELECT id, visibility, status FROM unified_analysis_publications WHERE run_id=$1`, runRow.ID).
		Scan(&pubID, &visibility, &status); err != nil {
		t.Fatalf("load publication: %v", err)
	}
	if status != "published" || visibility != "visible" {
		t.Fatalf("publication status=%s visibility=%s", status, visibility)
	}
	var currentPub, compatRun int64
	if err := pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_unified_publications WHERE seller_account_id=$1`, sellerID).Scan(&currentPub); err != nil {
		t.Fatalf("unified pointer: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&compatRun); err != nil {
		t.Fatalf("compat pointer: %v", err)
	}
	if currentPub != pubID || compatRun != runRow.ID {
		t.Fatalf("pointer mismatch currentPub=%d pubID=%d compat=%d run=%d", currentPub, pubID, compatRun, runRow.ID)
	}
	t.Logf("asynq publish e2e run_id=%d publication_id=%d status=%s", published.ID, pubID, published.Status)
}
