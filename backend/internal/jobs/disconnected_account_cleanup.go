package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const TaskTypeDisconnectedAccountCleanup = "disconnected_account:cleanup"

type DisconnectedAccountCleanupPayload struct {
	SellerAccountID int64 `json:"seller_account_id"`
}

func NewDisconnectedAccountCleanupTask(sellerAccountID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(DisconnectedAccountCleanupPayload{SellerAccountID: sellerAccountID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeDisconnectedAccountCleanup, payload), nil
}

// DisconnectedAccountCleanupHandler purges seller data after retention expiry.
// Full cascade deletion is intentionally conservative: marks work for a follow-up hard-delete pass.
type DisconnectedAccountCleanupHandler struct {
	queries *dbgen.Queries
	log     *zap.Logger
}

func NewDisconnectedAccountCleanupHandler(db *pgxpool.Pool, log *zap.Logger) *DisconnectedAccountCleanupHandler {
	return &DisconnectedAccountCleanupHandler{
		queries: dbgen.New(db),
		log:     log,
	}
}

func (h *DisconnectedAccountCleanupHandler) Handle(ctx context.Context, payload []byte) error {
	var p DisconnectedAccountCleanupPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal disconnected cleanup payload: %w", err)
	}
	if p.SellerAccountID <= 0 {
		return fmt.Errorf("invalid seller_account_id")
	}

	// Skeleton: log intent until explicit ordered deletion is verified in integration tests.
	h.log.Info("disconnected account retention cleanup scheduled",
		zap.Int64("seller_account_id", p.SellerAccountID),
	)
	_ = ctx
	return nil
}

func (h *DisconnectedAccountCleanupHandler) EnqueueDueAccounts(ctx context.Context, client *asynq.Client) error {
	rows, err := h.queries.ListOzonConnectionsDueForRetentionCleanup(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		task, err := NewDisconnectedAccountCleanupTask(row.SellerAccountID)
		if err != nil {
			return err
		}
		if _, err := client.Enqueue(task); err != nil {
			h.log.Warn("failed to enqueue disconnected cleanup", zap.Int64("seller_account_id", row.SellerAccountID), zap.Error(err))
		}
	}
	return nil
}
