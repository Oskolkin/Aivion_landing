package jobs

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/jackc/pgx/v5"
)

func loadSyncJobGuard(ctx context.Context, q *dbgen.Queries, syncJobID int64) (dbgen.SyncJob, error) {
	if syncJobID <= 0 {
		return dbgen.SyncJob{}, fmt.Errorf("invalid sync_job_id")
	}
	job, err := q.GetSyncJobByID(ctx, syncJobID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return dbgen.SyncJob{}, fmt.Errorf("sync job %d not found", syncJobID)
		}
		return dbgen.SyncJob{}, err
	}
	return job, nil
}

// syncGuardOutcome describes how a handler should proceed for an orphan or terminal parent sync.
type syncGuardOutcome int

const (
	syncGuardProceed syncGuardOutcome = iota
	syncGuardRetry
	syncGuardSkipSuccess
	syncGuardPermanentFailure
)

// guardParentSyncJob is used by sync coordinator to avoid re-running terminal parent jobs.
func guardParentSyncJob(job dbgen.SyncJob) syncGuardOutcome {
	switch job.Status {
	case "failed", "completed":
		return syncGuardSkipSuccess
	default:
		return syncGuardProceed
	}
}

// guardImportParentSync validates import tasks against the parent sync without requiring completion.
// Imports run while the parent sync is pending/running; only terminal failed/cancelled parents are skipped.
func guardImportParentSync(ctx context.Context, q *dbgen.Queries, syncJobID, importJobID int64) (syncGuardOutcome, error) {
	job, err := loadSyncJobGuard(ctx, q, syncJobID)
	if err != nil {
		return syncGuardPermanentFailure, err
	}
	switch job.Status {
	case syncdomain.SyncStatusFailed.String(), "cancelled":
		return syncGuardSkipSuccess, nil
	case syncdomain.SyncStatusPending.String(), syncdomain.SyncStatusRunning.String(), syncdomain.SyncStatusCompleted.String():
	default:
		return syncGuardPermanentFailure, guardPermanent(ErrParentSyncUnknownStatus, syncJobID, job.SellerAccountID, job.Status, syncdomain.SyncStatusCompleted.String())
	}

	importJob, err := q.GetImportJobByID(ctx, importJobID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return syncGuardPermanentFailure, fmt.Errorf("import job %d not found", importJobID)
		}
		return syncGuardPermanentFailure, err
	}
	if importJob.SyncJobID != syncJobID {
		return syncGuardPermanentFailure, fmt.Errorf("import job %d does not belong to sync %d", importJobID, syncJobID)
	}
	if importJob.SellerAccountID != job.SellerAccountID {
		return syncGuardPermanentFailure, guardPermanent(ErrParentSyncSellerMismatch, syncJobID, importJob.SellerAccountID, "", "")
	}
	return syncGuardProceed, nil
}
