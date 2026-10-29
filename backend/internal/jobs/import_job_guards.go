package jobs

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
)

func isLastAsynqRetry(ctx context.Context) bool {
	r, ok := asynq.GetRetryCount(ctx)
	if !ok {
		return true
	}
	max, ok := asynq.GetMaxRetry(ctx)
	if !ok {
		return true
	}
	return r >= max
}

func guardedUpdateImportJobToFetching(ctx context.Context, q *dbgen.Queries, importJobID int64) (dbgen.ImportJob, bool, error) {
	resolved, err := resolveImportJobExecutionTarget(ctx, q, importJobID)
	if err != nil {
		return dbgen.ImportJob{}, false, err
	}
	importJobID = resolved.ID
	job := resolved
	if job.Status == syncdomain.ImportStatusCompleted.String() {
		return job, true, nil
	}
	updated, err := q.UpdateImportJobToFetching(ctx, importJobID)
	if err != nil {
		if ingestion.IsNoRows(err) {
			job, rerr := q.GetImportJobByID(ctx, importJobID)
			if rerr != nil {
				return dbgen.ImportJob{}, false, rerr
			}
			if job.Status == syncdomain.ImportStatusCompleted.String() {
				return job, true, nil
			}
			return dbgen.ImportJob{}, false, err
		}
		return dbgen.ImportJob{}, false, err
	}
	return updated, false, nil
}

func markImportRetryingOrFailed(
	ctx context.Context,
	q *dbgen.Queries,
	importJobID int64,
	cause error,
	lastRetry bool,
) error {
	msg := pgtype.Text{String: cause.Error(), Valid: true}
	if !lastRetry {
		_, err := q.UpdateImportJobToRetrying(ctx, dbgen.UpdateImportJobToRetryingParams{
			ID:           importJobID,
			ErrorMessage: msg,
		})
		return err
	}
	_, err := q.UpdateImportJobToFailed(ctx, dbgen.UpdateImportJobToFailedParams{
		ID:           importJobID,
		ErrorMessage: msg,
	})
	return err
}

func runPostSyncContinuation(ctx context.Context, h *OzonImportHandler, payload OzonImportJobPayload) error {
	return runParentSyncJobFinalizationAndHook(ctx, h, payload)
}
