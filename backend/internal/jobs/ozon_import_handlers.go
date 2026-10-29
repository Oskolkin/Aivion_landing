package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ordersync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stocksync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncstate"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type OzonImportHandler struct {
	queries          *dbgen.Queries
	cursorService    *syncstate.SyncCursorService
	productsImporter *productsync.Service
	ordersImporter   *ordersync.Service
	stocksImporter   *stocksync.Service
	adsImporter      *adsync.Service
	log              *zap.Logger
	domain           string
	// onSyncJobCompleted is optional; invoked once when a parent sync_job transitions to completed.
	onSyncJobCompleted func(ctx context.Context, sellerAccountID, syncJobID int64) error
}

// OzonImportHandlerOption configures OzonImportHandler.
type OzonImportHandlerOption func(*OzonImportHandler)

// WithSyncJobCompletedHook registers a callback after the parent sync_job reaches status completed
// (e.g. enqueue post-sync recalculation). If the hook returns an error, the import task fails and can retry.
func WithSyncJobCompletedHook(fn func(ctx context.Context, sellerAccountID, syncJobID int64) error) OzonImportHandlerOption {
	return func(h *OzonImportHandler) {
		h.onSyncJobCompleted = fn
	}
}

func NewOzonImportHandler(
	db *pgxpool.Pool,
	log *zap.Logger,
	domain string,
	productsImporter *productsync.Service,
	ordersImporter *ordersync.Service,
	stocksImporter *stocksync.Service,
	adsImporter *adsync.Service,
	opts ...OzonImportHandlerOption,
) *OzonImportHandler {
	h := &OzonImportHandler{
		queries:          dbgen.New(db),
		cursorService:    syncstate.NewSyncCursorService(db),
		productsImporter: productsImporter,
		ordersImporter:   ordersImporter,
		stocksImporter:   stocksImporter,
		adsImporter:      adsImporter,
		log:              log,
		domain:           domain,
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

func (h *OzonImportHandler) Handle(ctx context.Context, taskPayload []byte) error {
	var payload OzonImportJobPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal import job payload: %w", err)
	}

	guardOutcome, guardErr := guardImportParentSync(ctx, h.queries, payload.SyncJobID, payload.ImportJobID)
	if guardErr != nil {
		return guardErr
	}
	if guardOutcome == syncGuardSkipSuccess {
		h.log.Info("ozon import skipped: parent sync terminal",
			zap.String("domain", h.domain),
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.Int64("import_job_id", payload.ImportJobID),
		)
		return runPostSyncContinuation(ctx, h, payload)
	}

	importJob, alreadyCompleted, err := guardedUpdateImportJobToFetching(ctx, h.queries, payload.ImportJobID)
	if err != nil {
		return fmt.Errorf("update import job to fetching: %w", err)
	}
	if alreadyCompleted {
		return runPostSyncContinuation(ctx, h, payload)
	}

	h.log.Info("ozon import job fetching started",
		zap.String("domain", h.domain),
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int64("import_job_id", payload.ImportJobID),
		zap.Bool("source_cursor_present", importJob.SourceCursor.Valid),
		zap.String("source_cursor", importJob.SourceCursor.String),
	)

	if _, err := h.queries.UpdateImportJobToImporting(ctx, payload.ImportJobID); err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("update import job to importing: %w", err)
	}

	switch h.domain {
	case "products":
		return h.handleProducts(ctx, payload, importJob)

	case "orders":
		return h.handleOrders(ctx, payload, importJob)

	case "stocks":
		return h.handleStocks(ctx, payload, importJob)

	case "ads":
		return h.handleAds(ctx, payload, importJob)

	default:
		err := fmt.Errorf("unsupported domain: %s", h.domain)
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return err
	}
}

func (h *OzonImportHandler) handleProducts(
	ctx context.Context,
	payload OzonImportJobPayload,
	importJob dbgen.ImportJob,
) error {
	_ = importJob // pagination cursor is read from sync_cursors (see ResolveSourceCursor) so retries resume correctly
	if h.productsImporter == nil {
		err := fmt.Errorf("products importer is nil")
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return err
	}

	sc, err := h.cursorService.ResolveSourceCursor(ctx, payload.SellerAccountID, "products")
	if err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("resolve products source cursor: %w", err)
	}
	sourceCursor := ""
	if sc.Valid {
		sourceCursor = sc.String
	}

	result, err := h.productsImporter.Run(ctx, productsync.RunInput{
		SellerAccountID: payload.SellerAccountID,
		ImportJobID:     payload.ImportJobID,
		SourceCursor:    sourceCursor,
	})
	if err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("run products import: %w", err)
	}

	if err := h.completeImport(
		ctx,
		payload,
		result.NextCursorValue,
		result.RecordsReceived,
		result.RecordsImported,
		result.RecordsFailed,
	); err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("complete products import: %w", err)
	}

	h.log.Info("ozon products import completed",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int64("import_job_id", payload.ImportJobID),
		zap.Int32("pages_fetched", result.PagesFetched),
		zap.Int32("records_received", result.RecordsReceived),
		zap.Int32("records_imported", result.RecordsImported),
		zap.String("final_page_last_id", result.FinalPageLastID),
		zap.String("next_cursor_value", result.NextCursorValue),
	)

	return nil
}

func (h *OzonImportHandler) handleOrders(
	ctx context.Context,
	payload OzonImportJobPayload,
	importJob dbgen.ImportJob,
) error {
	if h.ordersImporter == nil {
		err := fmt.Errorf("orders importer is nil")
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return err
	}

	result, err := h.ordersImporter.Run(ctx, ordersync.RunInput{
		SellerAccountID: payload.SellerAccountID,
		ImportJobID:     payload.ImportJobID,
		SourceCursor:    importJob.SourceCursor.String,
	})
	if err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("run orders import: %w", err)
	}

	if err := h.completeImport(
		ctx,
		payload,
		result.NextCursorValue,
		result.RecordsReceived,
		result.RecordsImported,
		result.RecordsFailed,
	); err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("complete orders import: %w", err)
	}

	h.log.Info("ozon orders import completed",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int64("import_job_id", payload.ImportJobID),
		zap.Int32("records_received", result.RecordsReceived),
		zap.Int32("records_imported", result.RecordsImported),
		zap.String("next_cursor_value", result.NextCursorValue),
	)

	return nil
}

func (h *OzonImportHandler) handleStocks(
	ctx context.Context,
	payload OzonImportJobPayload,
	importJob dbgen.ImportJob,
) error {
	if h.stocksImporter == nil {
		err := fmt.Errorf("stocks importer is nil")
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return err
	}

	result, err := h.stocksImporter.Run(ctx, stocksync.RunInput{
		SellerAccountID: payload.SellerAccountID,
		ImportJobID:     payload.ImportJobID,
		SourceCursor:    importJob.SourceCursor.String,
	})
	if err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("run stocks import: %w", err)
	}

	if err := h.completeImport(
		ctx,
		payload,
		result.NextCursorValue,
		result.RecordsReceived,
		result.RecordsImported,
		result.RecordsFailed,
	); err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("complete stocks import: %w", err)
	}

	h.log.Info("ozon stocks import completed",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int64("import_job_id", payload.ImportJobID),
		zap.Int32("records_received", result.RecordsReceived),
		zap.Int32("records_imported", result.RecordsImported),
		zap.String("next_cursor_value", result.NextCursorValue),
	)

	return nil
}

func (h *OzonImportHandler) handleAds(
	ctx context.Context,
	payload OzonImportJobPayload,
	importJob dbgen.ImportJob,
) error {
	if h.adsImporter == nil {
		err := fmt.Errorf("ads importer is nil")
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return err
	}

	result, err := h.adsImporter.Run(ctx, adsync.RunInput{
		SellerAccountID: payload.SellerAccountID,
		ImportJobID:     payload.ImportJobID,
		SourceCursor:    importJob.SourceCursor.String,
	})
	if err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("run ads import: %w", err)
	}

	if err := h.completeImport(
		ctx,
		payload,
		result.NextCursorValue,
		result.RecordsReceived,
		result.RecordsImported,
		result.RecordsFailed,
	); err != nil {
		if ferr := h.failImport(ctx, payload, err); ferr != nil {
			return ferr
		}
		return fmt.Errorf("complete ads import: %w", err)
	}

	h.log.Info("ozon ads import placeholder completed",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int64("import_job_id", payload.ImportJobID),
		zap.Int32("records_received", result.RecordsReceived),
		zap.Int32("records_imported", result.RecordsImported),
		zap.String("next_cursor_value", result.NextCursorValue),
	)

	return nil
}

func (h *OzonImportHandler) completeImport(
	ctx context.Context,
	payload OzonImportJobPayload,
	nextCursorValue string,
	recordsReceived int32,
	recordsImported int32,
	recordsFailed int32,
) error {
	if nextCursorValue != "" {
		if _, err := h.cursorService.AdvanceCursor(
			ctx,
			payload.SellerAccountID,
			payload.Domain,
			nextCursorValue,
		); err != nil {
			return fmt.Errorf("advance cursor: %w", err)
		}
	}

	if _, err := h.queries.UpdateImportJobToCompleted(ctx, dbgen.UpdateImportJobToCompletedParams{
		ID:              payload.ImportJobID,
		RecordsReceived: recordsReceived,
		RecordsImported: recordsImported,
		RecordsFailed:   recordsFailed,
	}); err != nil {
		return fmt.Errorf("update import job to completed: %w", err)
	}

	if err := runParentSyncJobFinalizationAndHook(ctx, h, payload); err != nil {
		return fmt.Errorf("finalize parent sync job: %w", err)
	}

	return nil
}

func (h *OzonImportHandler) failImport(ctx context.Context, payload OzonImportJobPayload, cause error) error {
	lastRetry := isLastAsynqRetry(ctx)
	if err := markImportRetryingOrFailed(ctx, h.queries, payload.ImportJobID, cause, lastRetry); err != nil {
		return fmt.Errorf("update import job status after failure: %w", err)
	}
	if lastRetry {
		if err := runParentSyncJobFinalizationAndHook(ctx, h, payload); err != nil {
			return fmt.Errorf("finalize parent sync job: %w", err)
		}
	}
	return cause
}

func runParentSyncJobFinalizationAndHook(ctx context.Context, h *OzonImportHandler, payload OzonImportJobPayload) error {
	res, err := runParentSyncJobFinalization(ctx, h.queries, payload.SyncJobID)
	if err != nil {
		return err
	}
	if h.onSyncJobCompleted == nil {
		return nil
	}
	shouldRunHook := res.SyncJobJustCompleted
	if !shouldRunHook {
		imports, listErr := h.queries.ListCanonicalImportJobsBySyncJobID(ctx, payload.SyncJobID)
		if listErr == nil && len(imports) > 0 {
			allTerminal := true
			for _, ij := range imports {
				if !syncdomain.IsTerminalImportStatus(ij.Status) {
					allTerminal = false
					break
				}
			}
			shouldRunHook = allTerminal
		}
	}
	if shouldRunHook {
		guard := NewSQLCParentSyncGuard(h.queries, h.log, nil, "import_hook")
		if _, err := guard.RequireCompletedParentSync(ctx, payload.SyncJobID, payload.SellerAccountID, nil); err != nil {
			if IsRetryableParentSyncGuardError(err) {
				return MapParentSyncGuardErrorToAsynq(err)
			}
			return nil
		}
		if hookErr := h.onSyncJobCompleted(ctx, payload.SellerAccountID, payload.SyncJobID); hookErr != nil {
			h.log.Error("sync_job completed hook failed",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
				zap.Error(hookErr),
			)
			return hookErr
		}
	}
	return nil
}
