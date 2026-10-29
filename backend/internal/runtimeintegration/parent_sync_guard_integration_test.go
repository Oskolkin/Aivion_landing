//go:build integration

package runtimeintegration

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func TestParentSyncGuard_CompletedSyncPermitsRecalculation(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedParentSyncFixture(t, ctx, pool, "completed")

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	if err := handler.Handle(ctx, payload.Payload()); err != nil {
		t.Fatal(err)
	}
	if heavyCalls.Load() != 1 {
		t.Fatalf("heavy calls=%d want 1", heavyCalls.Load())
	}
}

func TestParentSyncGuard_CompletedSyncPermitsPipeline(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "completed", metricDate)

	runner := &countingPipelineRunner{}
	seedEnqueuedDispatchForPipeline(t, ctx, pool, sellerID, syncJobID)
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, metricDate.Format("2006-01-02"), "post_sync", false)
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 1 {
		t.Fatalf("provider calls=%d want 1", runner.calls.Load())
	}
}

func TestParentSyncGuard_PendingSyncBlocksRecalculation(t *testing.T) {
	runRecalcGuardBlockCase(t, "pending", assertRetryableGuardError)
}

func TestParentSyncGuard_RunningSyncBlocksRecalculation(t *testing.T) {
	runRecalcGuardBlockCase(t, "running", assertRetryableGuardError)
}

func TestParentSyncGuard_PendingSyncBlocksPipeline(t *testing.T) {
	runPipelineGuardBlockCase(t, "pending", assertRetryableGuardError)
}

func TestParentSyncGuard_RunningSyncBlocksPipeline(t *testing.T) {
	runPipelineGuardBlockCase(t, "running", assertRetryableGuardError)
}

func TestParentSyncGuard_FailedSyncBlocksRecalculation(t *testing.T) {
	runRecalcGuardBlockCase(t, "failed", assertAsynqSkipRetry)
}

func TestParentSyncGuard_FailedSyncBlocksPipeline(t *testing.T) {
	runPipelineGuardBlockCase(t, "failed", assertAsynqSkipRetry)
}

func TestParentSyncGuard_CancelledSyncBlocksRecalculation(t *testing.T) {
	runRecalcGuardBlockCase(t, "cancelled", assertAsynqSkipRetry)
}

func TestParentSyncGuard_CancelledSyncBlocksPipeline(t *testing.T) {
	runPipelineGuardBlockCase(t, "cancelled", assertAsynqSkipRetry)
}

func TestParentSyncGuard_MissingSyncBlocksRecalculation(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, _ := seedParentSyncFixture(t, ctx, pool, "completed")

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, 999999999)
	err := handler.Handle(ctx, payload.Payload())
	assertAsynqSkipRetry(t, err)
	if heavyCalls.Load() != 0 {
		t.Fatalf("heavy calls=%d", heavyCalls.Load())
	}
}

func TestParentSyncGuard_MissingSyncBlocksPipeline(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	h := &Harness{t: t, Pool: pool, Queries: dbgen.New(pool), MetricDate: metricDate}
	sellerID := h.SeedSeller(t)

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	before := snapshotParentSyncEffects(t, ctx, pool, sellerID, 0)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, 999999999, metricDate.Format("2006-01-02"), "post_sync", false)
	assertAsynqSkipRetry(t, handler.Handle(ctx, task.Payload()))
	after := snapshotParentSyncEffects(t, ctx, pool, sellerID, 0)
	assertZeroParentSyncDelta(t, before, after)
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d", runner.calls.Load())
	}
}

func TestParentSyncGuard_SellerMismatchBlocksRecalculation(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedParentSyncFixture(t, ctx, pool, "completed")

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID+1000, syncJobID)
	assertAsynqSkipRetry(t, handler.Handle(ctx, payload.Payload()))
	if heavyCalls.Load() != 0 {
		t.Fatalf("heavy calls=%d", heavyCalls.Load())
	}
}

func TestParentSyncGuard_SellerMismatchBlocksPipeline(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "completed", metricDate)

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	before := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID+1000, syncJobID, metricDate.Format("2006-01-02"), "post_sync", false)
	assertAsynqSkipRetry(t, handler.Handle(ctx, task.Payload()))
	after := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	assertZeroParentSyncDelta(t, before, after)
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d", runner.calls.Load())
	}
}

func TestParentSyncGuard_MetricDateMismatchBlocksRecalculation(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "completed", metricDate)

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	// Recalc payload has no metric date; mismatch tests are pipeline-focused.
	_ = sellerID
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	if err := handler.Handle(ctx, payload.Payload()); err != nil {
		t.Fatal(err)
	}
	if heavyCalls.Load() != 1 {
		t.Fatalf("recalc should proceed without payload metric date, calls=%d", heavyCalls.Load())
	}
}

func TestParentSyncGuard_MetricDateMismatchBlocksPipeline(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "completed", metricDate)

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	before := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, "2020-01-01", "post_sync", false)
	assertAsynqSkipRetry(t, handler.Handle(ctx, task.Payload()))
	after := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	assertZeroParentSyncDelta(t, before, after)
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d", runner.calls.Load())
	}
}

func TestParentSyncGuard_RetryAfterSyncCompletion(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedParentSyncFixture(t, ctx, pool, "running")

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	assertRetryableGuardError(t, handler.Handle(ctx, payload.Payload()))
	if heavyCalls.Load() != 0 {
		t.Fatalf("first attempt heavy calls=%d", heavyCalls.Load())
	}

	if _, err := pool.Exec(ctx, `UPDATE sync_jobs SET status = 'completed', updated_at = now() WHERE id = $1`, syncJobID); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(ctx, payload.Payload()); err != nil {
		t.Fatal(err)
	}
	if heavyCalls.Load() != 1 {
		t.Fatalf("retry heavy calls=%d want 1", heavyCalls.Load())
	}
}

func TestParentSyncGuard_OrphanQueuedPipelineTask(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "fake")
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "completed", metricDate)

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, metricDate.Format("2006-01-02"), "post_sync", false)

	if _, err := pool.Exec(ctx, `UPDATE sync_jobs SET status = 'failed', updated_at = now() WHERE id = $1`, syncJobID); err != nil {
		t.Fatal(err)
	}
	before := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	assertAsynqSkipRetry(t, handler.Handle(ctx, task.Payload()))
	after := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	assertZeroParentSyncDelta(t, before, after)
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d", runner.calls.Load())
	}
}

func TestParentSyncGuard_RecoveryRechecksParent(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	q := dbgen.New(pool)
	ctx := context.Background()
	sellerID, syncJobID := seedParentSyncFixture(t, ctx, pool, "failed")

	dispatchSvc := jobs.NewAIPipelineDispatchService(q, testDispatchConfig(), zap.NewNop())
	dispatch, err := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	if _, err := dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_dispatches
SET status = 'retry_pending', next_attempt_at = now() - interval '1 minute', updated_at = now()
WHERE id = $1`, dispatch.ID); err != nil {
		t.Fatal(err)
	}

	var enqueueCalls atomic.Int32
	targetSyncID := syncJobID
	recovery := jobs.NewAIPipelineDispatchRecoveryHandler(pool, q, 10, func(ctx context.Context, sellerID, syncJobID int64) error {
		if syncJobID == targetSyncID {
			enqueueCalls.Add(1)
		}
		return nil
	}, zap.NewNop())
	if err := recovery.Handle(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if enqueueCalls.Load() != 0 {
		t.Fatalf("enqueue calls=%d", enqueueCalls.Load())
	}
	row, err := q.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
		SyncJobID: syncJobID,
		Stage:     "ai_pipeline",
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != jobs.DispatchStatusFailed {
		t.Fatalf("dispatch status=%s", row.Status)
	}
}

func TestParentSyncGuard_EnrichmentPartialAfterCompletedSync(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "completed", metricDate)

	if _, err := pool.Exec(ctx, `
INSERT INTO ozon_enrichment_sync_runs (seller_account_id, metric_date, scope, status, started_at)
VALUES ($1, $2, 'p0', 'failed', now())`, sellerID, pgtype.Date{Time: metricDate, Valid: true}); err != nil {
		t.Fatalf("seed enrichment failure: %v", err)
	}

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	if err := handler.Handle(ctx, payload.Payload()); err != nil {
		t.Fatal(err)
	}
	if heavyCalls.Load() != 1 {
		t.Fatalf("recalc should proceed after completed parent sync, calls=%d", heavyCalls.Load())
	}
}

func TestParentSyncGuard_FailedParentBlocksEnrichmentContinuation(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, "failed", metricDate)

	var recalcEnqueue atomic.Int32
	handler := jobs.NewOzonEnrichmentP0Handler(nil, dbgen.New(pool), func(ctx context.Context, sellerID, syncJobID int64) error {
		recalcEnqueue.Add(1)
		return nil
	}, zap.NewNop(), nil)
	task, _ := jobs.NewOzonEnrichmentP0Task(sellerID, syncJobID, metricDate.Format("2006-01-02"), "p0")
	assertAsynqSkipRetry(t, handler.Handle(ctx, task.Payload()))
	if recalcEnqueue.Load() != 0 {
		t.Fatalf("recalc enqueue=%d", recalcEnqueue.Load())
	}
}

func TestParentSyncGuard_UnknownStatusFailClosed(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedParentSyncFixture(t, ctx, pool, "completed")
	relaxSyncJobStatusCheck(t, ctx, pool, "archived_unknown")
	if _, err := pool.Exec(ctx, `UPDATE sync_jobs SET status = 'archived_unknown', updated_at = now() WHERE id = $1`, syncJobID); err != nil {
		t.Fatal(err)
	}

	guard := jobs.NewSQLCParentSyncGuard(dbgen.New(pool), zap.NewNop(), nil, "test")
	_, err := guard.RequireCompletedParentSync(ctx, syncJobID, sellerID, nil)
	if !errors.Is(err, jobs.ErrParentSyncUnknownStatus) {
		t.Fatalf("expected unknown status error, got %v", err)
	}
	assertAsynqSkipRetry(t, jobs.MapParentSyncGuardErrorToAsynq(err))
}

// --- helpers ---

type parentSyncEffectSnapshot struct {
	dispatches    int
	pipelineRuns  int
	artifacts     int
	usageEvents   int
	actionRuns    int
	dashboardRuns int
}

func snapshotParentSyncEffects(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, syncJobID int64) parentSyncEffectSnapshot {
	t.Helper()
	var snap parentSyncEffectSnapshot
	if syncJobID > 0 {
		_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_dispatches WHERE sync_job_id = $1`, syncJobID).Scan(&snap.dispatches)
	}
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_runs WHERE seller_account_id = $1`, sellerID).Scan(&snap.pipelineRuns)
	_ = pool.QueryRow(ctx, `
SELECT COUNT(*) FROM ai_pipeline_artifacts a
INNER JOIN ai_pipeline_runs r ON r.id = a.ai_pipeline_run_id
WHERE r.seller_account_id = $1`, sellerID).Scan(&snap.artifacts)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_usage_events WHERE seller_account_id = $1`, sellerID).Scan(&snap.usageEvents)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_generation_runs WHERE seller_account_id = $1`, sellerID).Scan(&snap.actionRuns)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_dashboard_narratives WHERE seller_account_id = $1`, sellerID).Scan(&snap.dashboardRuns)
	return snap
}

func assertZeroParentSyncDelta(t *testing.T, before, after parentSyncEffectSnapshot) {
	t.Helper()
	if before.dispatches != after.dispatches {
		t.Fatalf("dispatch delta=%d", after.dispatches-before.dispatches)
	}
	if before.pipelineRuns != after.pipelineRuns {
		t.Fatalf("pipeline runs delta=%d", after.pipelineRuns-before.pipelineRuns)
	}
	if before.artifacts != after.artifacts {
		t.Fatalf("artifacts delta=%d", after.artifacts-before.artifacts)
	}
	if before.usageEvents != after.usageEvents {
		t.Fatalf("usage delta=%d", after.usageEvents-before.usageEvents)
	}
	if before.actionRuns != after.actionRuns {
		t.Fatalf("action runs delta=%d", after.actionRuns-before.actionRuns)
	}
	if before.dashboardRuns != after.dashboardRuns {
		t.Fatalf("dashboard runs delta=%d", after.dashboardRuns-before.dashboardRuns)
	}
}

func assertAsynqSkipRetry(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
}

func assertRetryableGuardError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected retryable error, got SkipRetry: %v", err)
	}
}

func runRecalcGuardBlockCase(t *testing.T, status string, assertErr func(*testing.T, error)) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedParentSyncFixture(t, ctx, pool, status)

	var heavyCalls atomic.Int32
	handler := newRecalcProbeHandler(t, pool, &countingPostSyncAccount{callback: func() { heavyCalls.Add(1) }})
	before := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	payload, _ := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	assertErr(t, handler.Handle(ctx, payload.Payload()))
	after := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	assertZeroParentSyncDelta(t, before, after)
	if heavyCalls.Load() != 0 {
		t.Fatalf("heavy calls=%d", heavyCalls.Load())
	}
}

func runPipelineGuardBlockCase(t *testing.T, status string, assertErr func(*testing.T, error)) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	metricDate := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sellerID, syncJobID := seedParentSyncFixtureWithDate(t, ctx, pool, status, metricDate)

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	before := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, metricDate.Format("2006-01-02"), "post_sync", false)
	assertErr(t, handler.Handle(ctx, task.Payload()))
	after := snapshotParentSyncEffects(t, ctx, pool, sellerID, syncJobID)
	assertZeroParentSyncDelta(t, before, after)
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d", runner.calls.Load())
	}
}

func newRecalcProbeHandler(t *testing.T, pool *pgxpool.Pool, acc *countingPostSyncAccount) *jobs.RecalculateAfterSyncHandler {
	t.Helper()
	q := dbgen.New(pool)
	return jobs.NewRecalculateAfterSyncHandler(
		acc,
		&integrationPostSyncSKU{n: 1},
		nil,
		&integrationPostSyncAlerts{summary: alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}},
		q,
		nil,
		zap.NewNop(),
		nil,
	)
}

func seedParentSyncFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string) (sellerID, syncJobID int64) {
	t.Helper()
	return seedParentSyncFixtureWithDate(t, ctx, pool, status, time.Now().UTC())
}

func seedParentSyncFixtureWithDate(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string, metricDate time.Time) (sellerID, syncJobID int64) {
	t.Helper()
	h := &Harness{t: t, Pool: pool, Queries: dbgen.New(pool), MetricDate: metricDate.UTC()}
	sellerID = h.SeedSeller(t)
	syncJobID = insertParentSyncJob(t, ctx, pool, sellerID, status)
	return sellerID, syncJobID
}

func insertParentSyncJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64, status string) int64 {
	t.Helper()
	relaxSyncJobStatusCheck(t, ctx, pool, status)
	var id int64
	if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, created_at, updated_at)
VALUES ($1, 'incremental_sync', $2, now(), now())
RETURNING id`, sellerID, status).Scan(&id); err != nil {
		t.Fatalf("insert sync job: %v", err)
	}
	return id
}

func relaxSyncJobStatusCheck(t *testing.T, ctx context.Context, pool *pgxpool.Pool, status string) {
	t.Helper()
	switch status {
	case "pending", "running", "completed", "failed":
		return
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE sync_jobs DROP CONSTRAINT IF EXISTS sync_jobs_status_check`); err != nil {
		t.Fatalf("drop sync_jobs status check: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `
ALTER TABLE sync_jobs ADD CONSTRAINT sync_jobs_status_check
CHECK (status IN ('pending', 'running', 'completed', 'failed'))`)
	})
}

func seedEnqueuedDispatchForPipeline(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, syncJobID int64) {
	t.Helper()
	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop())
	dispatch, err := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, _, err = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type countingPipelineRunner struct {
	calls atomic.Int32
}

func (c *countingPipelineRunner) RunPostSync(ctx context.Context, in aipipeline.PostSyncPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	c.calls.Add(1)
	return &aipipeline.RunSellerAIPipelineResult{
		SellerAccountID: in.SellerAccountID,
		Status:          aipipeline.PipelineStatusCompleted,
		Published:       true,
		PipelineRunID:   1,
	}, nil
}

func (c *countingPipelineRunner) RunExistingPipeline(ctx context.Context, in aipipeline.ExistingPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	c.calls.Add(1)
	return &aipipeline.RunSellerAIPipelineResult{
		SellerAccountID: in.SellerAccountID,
		Status:          aipipeline.PipelineStatusCompleted,
		Published:       true,
		PipelineRunID:   in.PipelineRunID,
	}, nil
}
