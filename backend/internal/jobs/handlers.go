package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

type Handler struct {
	log *zap.Logger
}

func NewHandler(log *zap.Logger) *Handler {
	return &Handler{
		log: log,
	}
}

func (h *Handler) HandleSystemPingTask(ctx context.Context, task *asynq.Task) error {
	var payload SystemPingPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal system ping payload: %w", err)
	}

	h.log.Info("system.ping business logic executed",
		zap.String("message", payload.Message),
	)

	return nil
}
