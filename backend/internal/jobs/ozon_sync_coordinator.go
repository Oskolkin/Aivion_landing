package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncstate"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type OzonSyncCoordinatorHandler struct {
	queries       *dbgen.Queries
	enqueuer      ingestion.TaskEnqueuer
	cursorService *syncstate.SyncCursorService
	log           *zap.Logger
}

func NewOzonSyncCoordinatorHandler(db *pgxpool.Pool, enqueuer ingestion.TaskEnqueuer, log *zap.Logger) *OzonSyncCoordinatorHandler {
	return &OzonSyncCoordinatorHandler{
		queries:       dbgen.New(db),
		enqueuer:      enqueuer,
		cursorService: syncstate.NewSyncCursorService(db),
		log:           log,
	}
}

func (h *OzonSyncCoordinatorHandler) Handle(ctx context.Context, taskPayload []byte) error {
	var payload OzonSyncCoordinatorPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal sync coordinator payload: %w", err)
	}

	syncJob, err := h.queries.GetSyncJobByID(ctx, payload.SyncJobID)
	if err != nil {
		return fmt.Errorf("load sync job: %w", err)
	}
	switch guardParentSyncJob(syncJob) {
	case syncGuardSkipSuccess:
		h.log.Info("ozon sync coordinator skipped: parent sync terminal",
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.String("status", syncJob.Status),
		)
		return nil
	case syncGuardPermanentFailure:
		return fmt.Errorf("sync job %d not found", payload.SyncJobID)
	}

	if _, err := h.queries.UpdateSyncJobToRunning(ctx, payload.SyncJobID); err != nil {
		return fmt.Errorf("update sync job to running: %w", err)
	}

	importJobs, err := ingestion.EnsureImportJobsForSync(
		ctx,
		h.queries,
		h.cursorService.ResolveSourceCursor,
		payload.SellerAccountID,
		payload.SyncJobID,
	)
	if err != nil {
		return h.failCoordinator(ctx, payload, err)
	}

	var enqueueErr error
	for _, importJob := range importJobs {
		task, taskErr := newImportTaskForDomain(payload.SellerAccountID, payload.SyncJobID, importJob.ID, importJob.Domain)
		if taskErr != nil {
			enqueueErr = taskErr
			break
		}
		taskID := importTaskID(payload.SyncJobID, importJob.Domain)
		if _, err := h.enqueuer.Enqueue(task, asynq.TaskID(taskID)); err != nil && !isDuplicateTask(err) {
			enqueueErr = fmt.Errorf("enqueue import task for %s: %w", importJob.Domain, err)
			break
		}
		h.log.Info("ozon import job ensured",
			zap.Int64("seller_account_id", payload.SellerAccountID),
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.Int64("import_job_id", importJob.ID),
			zap.String("domain", importJob.Domain),
			zap.String("status", importJob.Status),
		)
	}

	if enqueueErr != nil {
		return h.failCoordinator(ctx, payload, enqueueErr)
	}

	h.log.Info("ozon sync coordinator dispatched import jobs",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.String("sync_type", payload.SyncType),
	)
	return nil
}

func (h *OzonSyncCoordinatorHandler) failCoordinator(ctx context.Context, payload OzonSyncCoordinatorPayload, cause error) error {
	msg := pgtype.Text{String: cause.Error(), Valid: true}
	_ = h.queries.FailPendingImportJobsForSync(ctx, dbgen.FailPendingImportJobsForSyncParams{
		SyncJobID:    payload.SyncJobID,
		ErrorMessage: msg,
	})
	_, _ = h.queries.MarkSyncJobFailedIfNonTerminal(ctx, dbgen.MarkSyncJobFailedIfNonTerminalParams{
		ID:           payload.SyncJobID,
		ErrorMessage: msg,
	})
	return cause
}

func newImportTaskForDomain(sellerAccountID, syncJobID, importJobID int64, domain string) (*asynq.Task, error) {
	switch domain {
	case syncdomain.ImportDomainProducts.String():
		return NewOzonImportProductsTask(sellerAccountID, syncJobID, importJobID)
	case syncdomain.ImportDomainOrders.String():
		return NewOzonImportOrdersTask(sellerAccountID, syncJobID, importJobID)
	case syncdomain.ImportDomainStocks.String():
		return NewOzonImportStocksTask(sellerAccountID, syncJobID, importJobID)
	case syncdomain.ImportDomainAds.String():
		return NewOzonImportAdsTask(sellerAccountID, syncJobID, importJobID)
	default:
		return nil, fmt.Errorf("unsupported domain: %s", domain)
	}
}

func isDuplicateTask(err error) bool {
	return err != nil && err.Error() == asynq.ErrDuplicateTask.Error()
}

func importTaskID(syncJobID int64, domain string) string {
	return "ozon-import:" + strconv.FormatInt(syncJobID, 10) + ":" + domain
}

func coordinatorTaskID(syncJobID int64) string {
	return "ozon-sync:" + strconv.FormatInt(syncJobID, 10)
}
