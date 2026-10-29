package jobs

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func LoggingMiddleware(log *zap.Logger, m *metrics.Metrics) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
			start := time.Now()

			taskID, _ := asynq.GetTaskID(ctx)
			queue, _ := asynq.GetQueueName(ctx)
			retryCount, _ := asynq.GetRetryCount(ctx)

			log.Info("job started",
				zap.String("task_id", taskID),
				zap.String("task_type", task.Type()),
				zap.String("queue", queue),
				zap.Int("retry_count", retryCount),
			)

			err := next.ProcessTask(ctx, task)

			duration := time.Since(start)

			if err != nil {
				m.JobsProcessedTotal.WithLabelValues(task.Type(), queue, "failed").Inc()

				log.Error("job failed",
					zap.String("task_id", taskID),
					zap.String("task_type", task.Type()),
					zap.String("queue", queue),
					zap.Int("retry_count", retryCount),
					zap.Int64("duration_ms", duration.Milliseconds()),
					zap.String("status", "failed"),
					zap.Error(err),
				)
				return err
			}

			m.JobsProcessedTotal.WithLabelValues(task.Type(), queue, "success").Inc()

			log.Info("job completed",
				zap.String("task_id", taskID),
				zap.String("task_type", task.Type()),
				zap.String("queue", queue),
				zap.Int("retry_count", retryCount),
				zap.Int64("duration_ms", duration.Milliseconds()),
				zap.String("status", "success"),
			)

			return nil
		})
	}
}
