//go:build integration

package runtimeintegration

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func TestAIPipelineSingleExecution_TwoHandlersConcurrent(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)
	seedEnqueuedDispatchForPipeline(t, ctx, pool, sellerID, syncJobID)

	runner := newBlockingPipelineRunner()
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)

	var wg sync.WaitGroup
	var secondDone atomic.Bool
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = handler.Handle(ctx, task.Payload())
	}()
	time.Sleep(200 * time.Millisecond)
	go func() {
		defer wg.Done()
		_ = handler.Handle(ctx, task.Payload())
		secondDone.Store(true)
	}()
	deadline := time.After(2 * time.Second)
	for !secondDone.Load() {
		select {
		case <-deadline:
			t.Fatal("second handler did not finish")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	close(runner.release)
	wg.Wait()

	if got := runner.calls.Load(); got != 1 {
		t.Fatalf("provider calls=%d want 1", got)
	}
	var dispatchID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM ai_pipeline_dispatches WHERE sync_job_id=$1 AND stage='ai_pipeline'`, syncJobID).Scan(&dispatchID); err != nil {
		t.Fatal(err)
	}
	var pipelineCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_runs WHERE dispatch_id=$1`, dispatchID).Scan(&pipelineCount); err != nil {
		t.Fatal(err)
	}
	if pipelineCount != 1 {
		t.Fatalf("pipeline runs=%d want 1", pipelineCount)
	}
}

func TestAIPipelineSingleExecution_FreshStartedDuplicateNoRetry(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })

	lease := uuid.NewString()
	claim, err := dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, lease, time.Now().UTC(), jobs.ClaimSourceHandler)
	if err != nil || !claim.Claimed {
		t.Fatalf("claim: %+v err=%v", claim, err)
	}

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 0 {
		t.Fatalf("duplicate handler calls=%d want 0", runner.calls.Load())
	}

	var status string
	var pipelineAttempts int32
	if err := pool.QueryRow(ctx, `SELECT status, pipeline_attempts FROM ai_pipeline_dispatches WHERE id=$1`, dispatch.ID).
		Scan(&status, &pipelineAttempts); err != nil {
		t.Fatal(err)
	}
	if status != jobs.DispatchStatusPipelineStarted {
		t.Fatalf("status=%s", status)
	}
	if pipelineAttempts != 1 {
		t.Fatalf("pipeline_attempts=%d want 1", pipelineAttempts)
	}
}

func TestAIPipelineSingleExecution_StaleLeaseRecoveryReenqueue(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })
	claim, err := dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, uuid.NewString(), time.Now().UTC(), jobs.ClaimSourceHandler)
	if err != nil || !claim.Claimed {
		t.Fatalf("claim: %+v err=%v", claim, err)
	}

	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_dispatches
SET lease_expires_at = now() - interval '1 minute'
WHERE id = $1`, dispatch.ID); err != nil {
		t.Fatal(err)
	}

	recovery := jobs.NewAIPipelineDispatchRecoveryHandler(pool, dbgen.New(pool), 10, func(ctx context.Context, sellerID, syncJobID int64) error {
		return nil
	}, zap.NewNop())
	if err := recovery.Handle(ctx, nil); err != nil {
		t.Fatal(err)
	}

	var status string
	var leaseToken *string
	if err := pool.QueryRow(ctx, `SELECT status, execution_lease_token FROM ai_pipeline_dispatches WHERE id=$1`, dispatch.ID).
		Scan(&status, &leaseToken); err != nil {
		t.Fatal(err)
	}
	if status != jobs.DispatchStatusEnqueued && status != jobs.DispatchStatusRetryPending {
		t.Fatalf("status=%s want enqueued or retry_pending", status)
	}
	if leaseToken != nil && *leaseToken != "" {
		t.Fatalf("lease should be cleared, got %q", *leaseToken)
	}
}

func TestAIPipelineSingleExecution_PublishedBookkeepingRecovery(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	pipelineRunID := publishDispatchTestPipeline(t, ctx, pool, dbgen.New(pool), sellerID)

	lease := uuid.NewString()
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_dispatches
SET status='pipeline_started',
    pipeline_run_id=$2,
    execution_lease_token=$3,
    lease_acquired_at=$4::timestamptz,
    lease_expires_at=$4::timestamptz + interval '1 hour',
    pipeline_started_at=$4::timestamptz
WHERE id=$1`, dispatch.ID, pipelineRunID, lease, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_runs SET dispatch_id=$1 WHERE id=$2`, dispatch.ID, pipelineRunID); err != nil {
		t.Fatal(err)
	}

	refreshed, err := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	if err != nil {
		t.Fatal(err)
	}
	reconciled, done, err := dispatchSvc.ReconcileStartedDispatch(ctx, refreshed)
	if err != nil || !done {
		t.Fatalf("reconcile done=%v err=%v", done, err)
	}
	if reconciled.Status != jobs.DispatchStatusCompleted {
		t.Fatalf("status=%s", reconciled.Status)
	}
}

func TestAIPipelineSingleExecution_ConcurrentClaimOneWinner(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })

	var wg sync.WaitGroup
	var claims [2]jobs.PipelineExecutionClaim
	var errs [2]error
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			claims[i], errs[i] = dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, uuid.NewString(), time.Now().UTC(), jobs.ClaimSourceHandler)
		}()
	}
	wg.Wait()
	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Fatalf("claim %d err=%v", i, errs[i])
		}
	}

	claimed := 0
	idempotent := 0
	for _, c := range claims {
		if c.Claimed {
			claimed++
		}
		if c.AlreadyRunning {
			idempotent++
		}
	}
	if claimed != 1 || claimed+idempotent != 2 {
		t.Fatalf("claimed=%d already_running=%d claims=%+v %+v", claimed, idempotent, claims[0], claims[1])
	}

	var pipelineCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_runs WHERE dispatch_id=$1`, dispatch.ID).Scan(&pipelineCount); err != nil {
		t.Fatal(err)
	}
	if pipelineCount != 1 {
		t.Fatalf("pipeline runs=%d want 1", pipelineCount)
	}
}

func TestAIPipelineSingleExecution_CrashAfterPipelineRowNoSecondRun(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })

	first, err := dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, uuid.NewString(), time.Now().UTC(), jobs.ClaimSourceHandler)
	if err != nil || !first.Claimed {
		t.Fatalf("first claim: %+v err=%v", first, err)
	}

	second, err := dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, uuid.NewString(), time.Now().UTC(), jobs.ClaimSourceHandler)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AlreadyRunning {
		t.Fatalf("second claim=%+v want already_running", second)
	}
	if second.PipelineRunID != first.PipelineRunID {
		t.Fatalf("pipeline_run_id changed %d -> %d", first.PipelineRunID, second.PipelineRunID)
	}

	var pipelineCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_runs WHERE dispatch_id=$1`, dispatch.ID).Scan(&pipelineCount); err != nil {
		t.Fatal(err)
	}
	if pipelineCount != 1 {
		t.Fatalf("pipeline runs=%d want 1", pipelineCount)
	}
}

func TestAIPipelineSingleExecution_MaxAttemptsTerminalNoProviderCalls(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	cfg := testDispatchConfig()
	cfg.MaxAttempts = 1
	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), cfg, zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })
	claim, err := dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, uuid.NewString(), time.Now().UTC(), jobs.ClaimSourceHandler)
	if err != nil || !claim.Claimed {
		t.Fatalf("claim: %+v err=%v", claim, err)
	}
	refreshed, err := dbgen.New(pool).GetAIPipelineDispatchByID(ctx, dispatch.ID)
	if err != nil {
		t.Fatal(err)
	}
	leaseToken := ""
	if refreshed.ExecutionLeaseToken.Valid {
		leaseToken = refreshed.ExecutionLeaseToken.String
	}
	if _, err := dispatchSvc.MarkPipelineRetry(ctx, refreshed, fmt.Errorf("boom"), "pipeline_retryable", leaseToken); err != nil {
		t.Fatal(err)
	}

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d want 0", runner.calls.Load())
	}
}

func TestAIPipelineSingleExecution_ConcurrentRecoveryOneRelease(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	dispatch, _, _ = dispatchSvc.BeginHeavyRecalculation(ctx, dispatch)
	dispatch, _ = dispatchSvc.CompleteHeavyRecalculation(ctx, dispatch.ID)
	dispatch, _ = dispatchSvc.EnqueuePipelineTask(ctx, dispatch, func(ctx context.Context, sellerID, syncJobID int64) error { return nil })
	claim, err := dispatchSvc.ClaimPipelineExecution(ctx, dispatch.ID, uuid.NewString(), time.Now().UTC(), jobs.ClaimSourceHandler)
	if err != nil || !claim.Claimed {
		t.Fatalf("claim: %+v err=%v", claim, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_dispatches SET lease_expires_at = now() - interval '1 minute' WHERE id=$1`, dispatch.ID); err != nil {
		t.Fatal(err)
	}

	recovery := jobs.NewAIPipelineDispatchRecoveryHandler(pool, dbgen.New(pool), 10, func(ctx context.Context, sellerID, syncJobID int64) error {
		return nil
	}, zap.NewNop())
	var wg sync.WaitGroup
	var errs [2]error
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			errs[i] = recovery.Handle(ctx, nil)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("recovery %d: %v", i, err)
		}
	}

	var pipelineCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_runs WHERE dispatch_id=$1`, dispatch.ID).Scan(&pipelineCount); err != nil {
		t.Fatal(err)
	}
	if pipelineCount != 1 {
		t.Fatalf("pipeline runs=%d want 1", pipelineCount)
	}
}

func TestAIPipelineSingleExecution_LeaseLossStopsExecution(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)
	seedEnqueuedDispatchForPipeline(t, ctx, pool, sellerID, syncJobID)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)

	runner := &leaseLossRunner{pool: pool, dispatchID: dispatch.ID}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 1 {
		t.Fatalf("provider calls=%d want 1", runner.calls.Load())
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_dispatches WHERE id=$1`, dispatch.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != jobs.DispatchStatusRetryPending && status != jobs.DispatchStatusPipelineStarted {
		t.Fatalf("status=%s", status)
	}
}

func TestAIPipelineSingleExecution_HandlerPublishedNoDuplicateAI(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)
	seedEnqueuedDispatchForPipeline(t, ctx, pool, sellerID, syncJobID)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	pipelineRunID := publishDispatchTestPipeline(t, ctx, pool, dbgen.New(pool), sellerID)
	lease := uuid.NewString()
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_dispatches
SET status='pipeline_started',
    pipeline_run_id=$2,
    execution_lease_token=$3,
    lease_acquired_at=$4::timestamptz,
    lease_expires_at=$4::timestamptz + interval '1 hour',
    pipeline_started_at=$4::timestamptz
WHERE id=$1`, dispatch.ID, pipelineRunID, lease, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_runs SET dispatch_id=$1 WHERE id=$2`, dispatch.ID, pipelineRunID); err != nil {
		t.Fatal(err)
	}

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d want 0", runner.calls.Load())
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_dispatches WHERE id=$1`, dispatch.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != jobs.DispatchStatusCompleted {
		t.Fatalf("status=%s want completed", status)
	}
}

func TestAIPipelineSingleExecution_PipelineFailureMarksRetryWithLease(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)
	seedEnqueuedDispatchForPipeline(t, ctx, pool, sellerID, syncJobID)

	runner := &errorPipelineRunner{err: fmt.Errorf("provider unavailable")}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}

	var status string
	var leaseToken *string
	if err := pool.QueryRow(ctx, `
SELECT status, execution_lease_token
FROM ai_pipeline_dispatches
WHERE sync_job_id=$1 AND stage='ai_pipeline'`, syncJobID).Scan(&status, &leaseToken); err != nil {
		t.Fatal(err)
	}
	if status != jobs.DispatchStatusRetryPending {
		t.Fatalf("status=%s want retry_pending", status)
	}
	if leaseToken != nil && *leaseToken != "" {
		t.Fatalf("lease should be cleared after retry, got %q", *leaseToken)
	}
}

func TestAIPipelineSingleExecution_CompleteDispatchFailureRetryable(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	ctx := context.Background()
	sellerID, syncJobID := seedDispatchFixture(t, ctx, pool)

	dispatchSvc := jobs.NewAIPipelineDispatchService(dbgen.New(pool), testDispatchConfig(), zap.NewNop()).WithPool(pool)
	dispatch, _ := dispatchSvc.EnsureDispatch(ctx, sellerID, syncJobID)
	pipelineRunID := publishDispatchTestPipeline(t, ctx, pool, dbgen.New(pool), sellerID)
	lease := uuid.NewString()
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_dispatches
SET status='pipeline_started',
    pipeline_run_id=$2,
    execution_lease_token=$3,
    lease_acquired_at=$4::timestamptz,
    lease_expires_at=$4::timestamptz + interval '1 hour',
    pipeline_started_at=$4::timestamptz
WHERE id=$1`, dispatch.ID, pipelineRunID, lease, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_runs SET dispatch_id=$1 WHERE id=$2`, dispatch.ID, pipelineRunID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_dispatches SET stage='broken_stage' WHERE id=$1`, dispatch.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := dispatchSvc.CompleteDispatch(ctx, syncJobID, pipelineRunID); err == nil {
		t.Fatal("expected complete dispatch error")
	}

	runner := &countingPipelineRunner{}
	handler := jobs.NewRunSellerAIPipelineHandler(runner, dbgen.New(pool), pool, zap.NewNop(), nil)
	task, _ := jobs.NewRunSellerAIPipelineTask(sellerID, syncJobID, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_dispatches SET stage='ai_pipeline' WHERE id=$1`, dispatch.ID); err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatal(err)
	}
	if runner.calls.Load() != 0 {
		t.Fatalf("provider calls=%d want 0", runner.calls.Load())
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_dispatches WHERE id=$1`, dispatch.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != jobs.DispatchStatusCompleted {
		t.Fatalf("status=%s want completed", status)
	}
}

type blockingPipelineRunner struct {
	calls   atomic.Int32
	release chan struct{}
}

func newBlockingPipelineRunner() *blockingPipelineRunner {
	return &blockingPipelineRunner{release: make(chan struct{})}
}

func (b *blockingPipelineRunner) RunPostSync(ctx context.Context, in aipipeline.PostSyncPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	return b.RunExistingPipeline(ctx, aipipeline.ExistingPipelineInput{
		SellerAccountID: in.SellerAccountID,
		MetricDate:      in.MetricDate,
		PipelineRunID:   1,
	})
}

func (b *blockingPipelineRunner) RunExistingPipeline(ctx context.Context, in aipipeline.ExistingPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	b.calls.Add(1)
	select {
	case <-b.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &aipipeline.RunSellerAIPipelineResult{
		SellerAccountID: in.SellerAccountID,
		PipelineRunID:   in.PipelineRunID,
		Status:          aipipeline.PipelineStatusCompleted,
		Published:       true,
	}, nil
}

type errorPipelineRunner struct {
	err error
}

func (e *errorPipelineRunner) RunPostSync(ctx context.Context, in aipipeline.PostSyncPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	return nil, e.err
}

func (e *errorPipelineRunner) RunExistingPipeline(ctx context.Context, in aipipeline.ExistingPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	return nil, e.err
}

type leaseLossRunner struct {
	pool       *pgxpool.Pool
	dispatchID int64
	calls      atomic.Int32
}

func (r *leaseLossRunner) RunPostSync(ctx context.Context, in aipipeline.PostSyncPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	return r.RunExistingPipeline(ctx, aipipeline.ExistingPipelineInput{
		SellerAccountID: in.SellerAccountID,
		MetricDate:      in.MetricDate,
		PipelineRunID:   1,
	})
}

func (r *leaseLossRunner) RunExistingPipeline(ctx context.Context, in aipipeline.ExistingPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	r.calls.Add(1)
	if in.LeaseGuard != nil {
		if err := in.LeaseGuard.ValidateLease(ctx, in.DispatchID, in.PipelineRunID); err != nil {
			return nil, err
		}
	}
	if _, err := r.pool.Exec(ctx, `
UPDATE ai_pipeline_dispatches
SET execution_lease_token = NULL, lease_expires_at = now() - interval '1 minute'
WHERE id = $1`, r.dispatchID); err != nil {
		return nil, err
	}
	if in.LeaseGuard != nil {
		if err := in.LeaseGuard.ValidateLease(ctx, in.DispatchID, in.PipelineRunID); err != nil {
			return nil, err
		}
	}
	return &aipipeline.RunSellerAIPipelineResult{
		SellerAccountID: in.SellerAccountID,
		PipelineRunID:   in.PipelineRunID,
		Published:       true,
		Status:          aipipeline.PipelineStatusCompleted,
	}, nil
}
