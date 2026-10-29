package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/cleanup"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const TaskTypeCleanupMaintenance = "maintenance.cleanup"

type CleanupMaintenancePayload struct {
	RetentionDays int `json:"retention_days"`
}

func NewCleanupMaintenanceTask(retentionDays int) (*asynq.Task, error) {
	if retentionDays <= 0 {
		return nil, fmt.Errorf("retention_days must be > 0")
	}
	raw, err := json.Marshal(CleanupMaintenancePayload{RetentionDays: retentionDays})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeCleanupMaintenance, raw), nil
}

type CleanupMaintenanceHandler struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

func NewCleanupMaintenanceHandler(pool *pgxpool.Pool, log *zap.Logger) *CleanupMaintenanceHandler {
	return &CleanupMaintenanceHandler{pool: pool, log: log}
}

func (h *CleanupMaintenanceHandler) Handle(ctx context.Context, payload []byte) error {
	if h.pool == nil {
		return fmt.Errorf("postgres pool is required")
	}
	var p CleanupMaintenancePayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return err
		}
	}
	if p.RetentionDays <= 0 {
		return fmt.Errorf("retention_days must be > 0")
	}
	n, err := cleanup.ArchiveStaleActiveChatSessions(ctx, h.pool, p.RetentionDays)
	if err != nil {
		return err
	}
	if h.log != nil {
		h.log.Info("cleanup maintenance finished",
			zap.Int64("archived_chat_sessions", n),
			zap.Int("retention_days", p.RetentionDays),
		)
	}
	return nil
}
