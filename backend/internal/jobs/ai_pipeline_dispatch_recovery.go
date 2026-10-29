package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AIPipelineDispatchRecoveryHandler struct {
	pool      *pgxpool.Pool
	queries   *dbgen.Queries
	dispatch  *AIPipelineDispatchService
	enqueuer  func(ctx context.Context, sellerAccountID, syncJobID int64) error
	batchSize int32
	log       *zap.Logger
}

func NewAIPipelineDispatchRecoveryHandler(pool *pgxpool.Pool, q *dbgen.Queries, batchSize int32, enqueuer func(ctx context.Context, sellerAccountID, syncJobID int64) error, log *zap.Logger) *AIPipelineDispatchRecoveryHandler {
	cfg := LoadDispatchConfig()
	if batchSize <= 0 {
		batchSize = cfg.RecoveryBatch
	}
	return &AIPipelineDispatchRecoveryHandler{
		pool:      pool,
		queries:   q,
		dispatch:  NewAIPipelineDispatchService(q, cfg, log),
		enqueuer:  enqueuer,
		batchSize: batchSize,
		log:       log,
	}
}

func (h *AIPipelineDispatchRecoveryHandler) Handle(ctx context.Context, _ []byte) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	qtx := h.queries.WithTx(tx)
	now := time.Now().UTC()
	rows, err := qtx.ListRecoverableAIPipelineDispatches(ctx, dbgen.ListRecoverableAIPipelineDispatchesParams{
		Stage:                postSyncDispatchStageAIPipeline,
		NowTs:                pgTimestamptz(now),
		EnqueueStuckInterval: durationInterval(h.dispatch.cfg.EnqueueStuckWait),
		EnqueuedTimeout:      durationInterval(h.dispatch.cfg.EnqueuedTimeout),
		StartedTimeout:       durationInterval(h.dispatch.cfg.StartedTimeout),
		LimitCount:           h.batchSize,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.Attempts >= h.dispatch.cfg.MaxAttempts {
			if _, ferr := qtx.MarkAIPipelineDispatchFailed(ctx, dbgen.MarkAIPipelineDispatchFailedParams{
				ID:            row.ID,
				LastError:     pgtype.Text{String: "dispatch recovery max attempts exceeded", Valid: true},
				LastErrorCode: pgtype.Text{String: "max_attempts_exceeded", Valid: true},
			}); ferr != nil {
				return fmt.Errorf("mark failed %d: %w", row.ID, ferr)
			}
			continue
		}
		switch row.Status {
		case DispatchStatusPipelineStarted, DispatchStatusEnqueued:
			dispatchSvc := NewAIPipelineDispatchService(qtx, h.dispatch.cfg, h.log)
			reconciled, done, rerr := dispatchSvc.ReconcileStartedDispatch(ctx, row)
			if rerr != nil {
				return fmt.Errorf("dispatch reconcile %d: %w", row.ID, rerr)
			}
			if done {
				continue
			}
			row = reconciled
			if row.Status == DispatchStatusPipelineStarted {
				if row.PipelineAttempts >= h.dispatch.cfg.MaxAttempts {
					if _, ferr := qtx.MarkAIPipelineDispatchFailed(ctx, dbgen.MarkAIPipelineDispatchFailedParams{
						ID:            row.ID,
						LastError:     pgtype.Text{String: "dispatch recovery max pipeline attempts exceeded", Valid: true},
						LastErrorCode: pgtype.Text{String: "max_attempts_exceeded", Valid: true},
					}); ferr != nil {
						return fmt.Errorf("mark failed %d: %w", row.ID, ferr)
					}
					continue
				}
				released, rerr := dispatchSvc.ReleaseStaleExecution(ctx, row.ID, now)
				if rerr != nil {
					if errors.Is(rerr, pgx.ErrNoRows) {
						continue
					}
					return fmt.Errorf("release stale execution %d: %w", row.ID, rerr)
				}
				row = released
			}
		}
		dispatchSvc := NewAIPipelineDispatchService(qtx, h.dispatch.cfg, h.log)
		parentGuard := NewSQLCParentSyncGuard(qtx, h.log, nil, "recovery")
		if _, err := parentGuard.RequireCompletedParentSync(ctx, row.SyncJobID, row.SellerAccountID, nil); err != nil {
			if IsTerminalParentSyncGuardError(err) {
				if _, ferr := qtx.MarkAIPipelineDispatchFailed(ctx, dbgen.MarkAIPipelineDispatchFailedParams{
					ID:            row.ID,
					LastError:     pgtype.Text{String: err.Error(), Valid: true},
					LastErrorCode: pgtype.Text{String: "parent_sync_blocked", Valid: true},
				}); ferr != nil {
					return fmt.Errorf("mark failed %d: %w", row.ID, ferr)
				}
			}
			h.log.Warn("dispatch recovery blocked by parent sync guard",
				zap.Int64("dispatch_id", row.ID),
				zap.Int64("sync_job_id", row.SyncJobID),
				zap.Error(err),
			)
			continue
		}
		if _, err := dispatchSvc.EnqueuePipelineTask(ctx, row, h.enqueuer); err != nil {
			h.log.Warn("dispatch recovery enqueue failed",
				zap.Int64("dispatch_id", row.ID),
				zap.Int64("sync_job_id", row.SyncJobID),
				zap.Error(err),
			)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	h.log.Debug("ai pipeline dispatch recovery completed", zap.Int("rows", len(rows)))
	return nil
}

func durationInterval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

func EnqueueAIPipelineDispatchRecovery(ctx context.Context, client *asynq.Client) error {
	task, err := NewAIPipelineDispatchRecoveryTask()
	if err != nil {
		return err
	}
	_, err = client.Enqueue(task, asynq.TaskID("ai-pipeline-dispatch-recovery"), asynq.Unique(60))
	if err != nil && !isDuplicateTask(err) {
		return err
	}
	return nil
}
