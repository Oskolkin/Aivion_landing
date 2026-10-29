//go:build integration

package runtimeintegration

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func TestAIPipelineDispatch_NormalSuccess(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	var enqueueCalls atomic.Int32
	enqueuer := func(ctx context.Context, sellerID, syncJobID int64) error {
		enqueueCalls.Add(1)
		return nil
	}

	dispatchSvc := jobs.NewAIPipelineDispatchService(q, testDispatchConfig(), zap.NewNop())
	dispatch, err := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	if err != nil {
		t.Fatal(err)
	}
	updated, skip, err := dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	if err != nil || skip {
		t.Fatalf("begin recalc: skip=%v err=%v", skip, err)
	}
	if _, err := dispatchSvc.CompleteHeavyRecalculation(ctx, updated.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := dispatchSvc.EnqueuePipelineTask(ctx, updated, enqueuer); err != nil {
		t.Fatal(err)
	}
	if enqueueCalls.Load() != 1 {
		t.Fatalf("enqueue calls=%d", enqueueCalls.Load())
	}

	pipelineRunID := publishDispatchTestPipeline(t, ctx, pool, q, sellerID)
	if _, err := dispatchSvc.BeginPipelineExecution(ctx, syncJobID, pipelineRunID); err != nil {
		t.Fatal(err)
	}
	completed, err := dispatchSvc.CompleteDispatch(ctx, syncJobID, pipelineRunID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != jobs.DispatchStatusCompleted {
		t.Fatalf("status=%s", completed.Status)
	}
}

func TestAIPipelineDispatch_DuplicateTaskIsSuccess(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	enqueuer := func(ctx context.Context, sellerID, syncJobID int64) error {
		return asynq.ErrDuplicateTask
	}
	dispatchSvc := jobs.NewAIPipelineDispatchService(q, testDispatchConfig(), zap.NewNop())
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	row, err := dispatchSvc.EnqueuePipelineTask(ctx, dispatch, enqueuer)
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != jobs.DispatchStatusEnqueued {
		t.Fatalf("status=%s", row.Status)
	}
}

func TestAIPipelineDispatch_EnqueueErrorRetryPending(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	enqueuer := func(ctx context.Context, sellerID, syncJobID int64) error {
		return dispatchTestError("redis connection refused")
	}
	dispatchSvc := jobs.NewAIPipelineDispatchService(q, testDispatchConfig(), zap.NewNop())
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	_, err := dispatchSvc.EnqueuePipelineTask(ctx, dispatch, enqueuer)
	if err == nil {
		t.Fatal("expected enqueue error")
	}
	refreshed, err := q.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
		SyncJobID: syncJobID,
		Stage:     "ai_pipeline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != jobs.DispatchStatusRetryPending {
		t.Fatalf("status=%s", refreshed.Status)
	}
	if !refreshed.NextAttemptAt.Valid {
		t.Fatal("expected next_attempt_at")
	}
}

func TestAIPipelineDispatch_CrashAfterPublicationReconcile(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(q, testDispatchConfig(), zap.NewNop())
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })

	pipelineRunID := publishDispatchTestPipeline(t, ctx, pool, q, sellerID)
	started, err := dispatchSvc.BeginPipelineExecution(ctx, syncJobID, pipelineRunID)
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != jobs.DispatchStatusPipelineStarted {
		t.Fatalf("status=%s", started.Status)
	}

	reconciled, done, err := dispatchSvc.ReconcileStartedDispatch(ctx, started)
	if err != nil || !done {
		t.Fatalf("reconcile done=%v err=%v", done, err)
	}
	if reconciled.Status != jobs.DispatchStatusCompleted {
		t.Fatalf("status=%s", reconciled.Status)
	}
}

func TestAIPipelineDispatch_HeavyRecalcOnceOnRetry(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	var heavyCalls atomic.Int32
	acc := &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }}
	handler := jobs.NewRecalculateAfterSyncHandler(
		acc,
		&integrationPostSyncSKU{n: 1},
		nil,
		&integrationPostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}},
		q,
		nil,
		zap.NewNop(),
		nil,
	)
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	if err := handler.Handle(ctx, payload.Payload()); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(ctx, payload.Payload()); err != nil {
		t.Fatal(err)
	}
	if heavyCalls.Load() != 1 {
		t.Fatalf("heavy recalculation calls=%d want 1", heavyCalls.Load())
	}
}

func TestAIPipelineDispatch_MaxAttemptsFailed(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	cfg := testDispatchConfig()
	cfg.MaxAttempts = 1
	dispatchSvc := jobs.NewAIPipelineDispatchService(q, cfg, zap.NewNop())
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)

	failEnqueuer := func(ctx context.Context, sellerID, syncJobID int64) error {
		return dispatchTestError("enqueue down")
	}
	_, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, failEnqueuer)
	_, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, failEnqueuer)
	row, err := q.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
		SyncJobID: syncJobID,
		Stage:     "ai_pipeline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != jobs.DispatchStatusFailed {
		t.Fatalf("status=%s attempts=%d", row.Status, row.Attempts)
	}
}

func testDispatchConfig() jobs.DispatchConfig {
	return jobs.DispatchConfig{
		MaxAttempts:      10,
		BaseBackoff:      time.Millisecond,
		MaxBackoff:       time.Second,
		EnqueuedTimeout:  time.Hour,
		StartedTimeout:   time.Hour,
		LeaseDuration:    time.Hour,
		EnqueueStuckWait: time.Millisecond,
		RecoveryBatch:    100,
	}
}

func seedDispatchFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (sellerID, syncJobID int64) {
	t.Helper()
	h := &Harness{t: t, Pool: pool, Queries: dbgen.New(pool), MetricDate: time.Now().UTC()}
	sellerID = h.SeedSeller(t)
	var id int64
	if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, created_at, updated_at)
VALUES ($1, 'incremental_sync', 'completed', now(), now())
RETURNING id`, sellerID).Scan(&id); err != nil {
		t.Fatalf("sync job: %v", err)
	}
	return sellerID, id
}

func publishDispatchTestPipeline(t *testing.T, ctx context.Context, pool *pgxpool.Pool, q *dbgen.Queries, sellerID int64) int64 {
	t.Helper()
	metricDate := time.Now().UTC()
	runID := createPublishablePipelineRun(t, ctx, pool, q, sellerID, metricDate, time.Now().UTC())
	publisher := aipipeline.NewSQLCSnapshotPublisher(pool)
	if _, err := publisher.PublishPipelineSnapshot(ctx, runID); err != nil {
		t.Fatalf("publish pipeline: %v", err)
	}
	return runID
}

func insertPublishedPipelineRun(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64) int64 {
	t.Helper()
	return publishDispatchTestPipeline(t, ctx, pool, dbgen.New(pool), sellerID)
}

type countingPostSyncAccount struct {
	callback func()
}

func (c *countingPostSyncAccount) RebuildDailyAccountMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	if c.callback != nil {
		c.callback()
	}
	return 1, nil
}

type dispatchTestError string

func (e dispatchTestError) Error() string { return string(e) }

type integrationPostSyncSKU struct{ n int }

func (f *integrationPostSyncSKU) RebuildDailySKUMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	return f.n, nil
}

type integrationPostSyncAlerts struct {
	summary alerts.RunForAccountSummary
}

func (f *integrationPostSyncAlerts) RunForAccountWithType(ctx context.Context, sellerAccountID int64, asOfDate time.Time, runType alerts.RunType) (alerts.RunForAccountSummary, error) {
	return f.summary, nil
}
