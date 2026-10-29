package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// TaskTypeUnifiedAnalysisPublish is the Stage 5 publication task: it projects a
// publication_ready unified analysis run into the seller-facing dashboard/actions
// tables via publication.Service.PublishRun.
const TaskTypeUnifiedAnalysisPublish = "unified_analysis.publish"

// Asynq option defaults for the Stage 5 publish task.
const (
	UnifiedAnalysisPublishTimeout    = 10 * time.Minute
	UnifiedAnalysisPublishUniqueTTL  = 15 * time.Minute
	UnifiedAnalysisMaxPublishRetries = 5
)

// PublishUnifiedAnalysisPayload is the payload for TaskTypeUnifiedAnalysisPublish.
type PublishUnifiedAnalysisPayload struct {
	RunID int64 `json:"run_id"`
}

// NewUnifiedAnalysisPublishTask builds the Stage 5 publish task for runID.
func NewUnifiedAnalysisPublishTask(runID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(PublishUnifiedAnalysisPayload{RunID: runID})
	if err != nil {
		return nil, fmt.Errorf("marshal unified analysis publish payload: %w", err)
	}
	return asynq.NewTask(TaskTypeUnifiedAnalysisPublish, payload), nil
}

// EnqueuePublish schedules TaskTypeUnifiedAnalysisPublish for runID. Deduplicated per
// run_id (TaskID) with a 15m uniqueness window so auto-publish-after-validate and
// recovery-triggered republish never double-enqueue a concurrent publish attempt.
func (e *AsynqUnifiedAnalysisEnqueue) EnqueuePublish(_ context.Context, runID int64) error {
	if e == nil || e.Client == nil {
		return fmt.Errorf("asynq client is not configured")
	}
	task, err := NewUnifiedAnalysisPublishTask(runID)
	if err != nil {
		return err
	}
	_, err = e.Client.Enqueue(task,
		asynq.TaskID(fmt.Sprintf("unified-analysis-publish:%d", runID)),
		asynq.Unique(UnifiedAnalysisPublishUniqueTTL),
		asynq.Timeout(UnifiedAnalysisPublishTimeout),
		asynq.MaxRetry(UnifiedAnalysisMaxPublishRetries),
	)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

// UnifiedAnalysisPublishHandler processes Stage 5 publication tasks. It never talks to
// the AI provider: publication.Service.PublishRun only projects an already-validated
// unified analysis run's output into dashboard/actions.
type UnifiedAnalysisPublishHandler struct {
	svc *publication.Service
	log *zap.Logger
}

// NewUnifiedAnalysisPublishHandler builds a Stage 5 publish task handler.
func NewUnifiedAnalysisPublishHandler(svc *publication.Service, log *zap.Logger) *UnifiedAnalysisPublishHandler {
	return &UnifiedAnalysisPublishHandler{svc: svc, log: log}
}

// RegisterUnifiedAnalysisPublishHandler registers the Stage 5 publish handler on mux.
func RegisterUnifiedAnalysisPublishHandler(mux *asynq.ServeMux, handler *UnifiedAnalysisPublishHandler) {
	if mux == nil || handler == nil {
		return
	}
	mux.HandleFunc(TaskTypeUnifiedAnalysisPublish, func(ctx context.Context, t *asynq.Task) error {
		return handler.Handle(ctx, t.Payload())
	})
}

// Handle processes one TaskTypeUnifiedAnalysisPublish task. Visibility (shadow vs
// visible) is resolved from seller_analysis_settings by PublishRun itself, so the
// task payload only needs run_id.
func (h *UnifiedAnalysisPublishHandler) Handle(ctx context.Context, taskPayload []byte) error {
	started := time.Now()
	var payload PublishUnifiedAnalysisPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal unified analysis publish: %w", err)
	}
	if payload.RunID <= 0 {
		return fmt.Errorf("invalid run_id")
	}
	if h.svc == nil {
		return fmt.Errorf("publication service is not configured")
	}
	if h.log != nil {
		h.log.Info("unified_analysis.publish",
			zap.String("event", "unified_analysis.publish"),
			zap.Int64("run_id", payload.RunID),
		)
	}
	result, err := h.svc.PublishRun(ctx, publication.PublishRequest{
		RunID: payload.RunID,
		Actor: "system",
	})
	if err != nil {
		if h.log != nil {
			h.log.Warn("unified_analysis.publish_error", zap.Int64("run_id", payload.RunID), zap.Error(err))
		}
		return err
	}
	if h.log != nil {
		fields := []zap.Field{
			zap.String("event", "unified_analysis.publish_done"),
			zap.Int64("run_id", payload.RunID),
			zap.Duration("duration_ms", time.Since(started)),
		}
		if result != nil {
			fields = append(fields,
				zap.String("outcome", string(result.Outcome)),
				zap.String("visibility", string(result.Visibility)),
				zap.String("status", string(result.Status)),
			)
		}
		h.log.Info("unified_analysis.publish_done", fields...)
	}
	return nil
}
