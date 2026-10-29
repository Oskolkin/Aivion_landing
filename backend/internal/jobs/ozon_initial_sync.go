package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type OzonInitialSyncHandler struct {
	queries *dbgen.Queries
	log     *zap.Logger
}

func NewOzonInitialSyncHandler(db *pgxpool.Pool, log *zap.Logger) *OzonInitialSyncHandler {
	return &OzonInitialSyncHandler{
		queries: dbgen.New(db),
		log:     log,
	}
}

func (h *OzonInitialSyncHandler) Handle(ctx context.Context, taskPayload []byte) error {
	var payload OzonInitialSyncPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal ozon initial sync payload: %w", err)
	}

	_, err := h.queries.UpdateSyncJobToRunning(ctx, payload.SyncJobID)
	if err != nil {
		return fmt.Errorf("update sync job to running: %w", err)
	}

	_, err = h.queries.UpdateOzonConnectionStatus(ctx, dbgen.UpdateOzonConnectionStatusParams{
		SellerAccountID: payload.SellerAccountID,
		Status:          "sync_in_progress",
	})
	if err != nil {
		return fmt.Errorf("update ozon connection status to sync_in_progress: %w", err)
	}

	// MVP bootstrap workflow
	time.Sleep(2 * time.Second)

	_, err = h.queries.UpdateSyncJobToCompleted(ctx, payload.SyncJobID)
	if err != nil {
		return fmt.Errorf("update sync job to completed: %w", err)
	}

	_, err = h.queries.UpdateOzonConnectionStatus(ctx, dbgen.UpdateOzonConnectionStatusParams{
		SellerAccountID: payload.SellerAccountID,
		Status:          "valid",
	})
	if err != nil {
		return fmt.Errorf("update ozon connection status back to valid: %w", err)
	}

	h.log.Info("ozon initial sync completed",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
	)

	return nil
}
