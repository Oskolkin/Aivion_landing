package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const syncJobFailedDueToImportJobsMessage = "one or more import jobs failed"

// ParentSyncFinalizeResult is returned by runParentSyncJobFinalization.
type ParentSyncFinalizeResult struct {
	// SyncJobJustCompleted is true when this invocation transitioned the parent sync_job to status completed.
	SyncJobJustCompleted bool
}

// parentSyncJobFinalizerQueries is implemented by *dbgen.Queries.
type parentSyncJobFinalizerQueries interface {
	TryFinalizeSyncJobFailedIfNonTerminal(ctx context.Context, arg dbgen.TryFinalizeSyncJobFailedIfNonTerminalParams) (dbgen.SyncJob, error)
	TryFinalizeSyncJobCompletedIfNonTerminal(ctx context.Context, id int64) (dbgen.SyncJob, error)
}

// runParentSyncJobFinalization marks the parent sync_job completed or failed when every
// import_job for that sync has reached a terminal state. It is safe to call after each
// import_job finishes and is idempotent for terminal sync_jobs.
func runParentSyncJobFinalization(ctx context.Context, q parentSyncJobFinalizerQueries, syncJobID int64) (ParentSyncFinalizeResult, error) {
	var res ParentSyncFinalizeResult
	if syncJobID <= 0 {
		return res, nil
	}
	_, err := q.TryFinalizeSyncJobFailedIfNonTerminal(ctx, dbgen.TryFinalizeSyncJobFailedIfNonTerminalParams{
		ID: syncJobID,
		ErrorMessage: pgtype.Text{
			String: syncJobFailedDueToImportJobsMessage,
			Valid:  true,
		},
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return res, fmt.Errorf("try finalize sync job as failed: %w", err)
	}
	sj, err := q.TryFinalizeSyncJobCompletedIfNonTerminal(ctx, syncJobID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return res, fmt.Errorf("try finalize sync job as completed: %w", err)
	}
	if err == nil && sj.Status == "completed" {
		res.SyncJobJustCompleted = true
	}
	return res, nil
}
