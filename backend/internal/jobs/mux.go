package jobs

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

func NewServeMux(handler *Handler, log *zap.Logger, m *metrics.Metrics) *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.Use(LoggingMiddleware(log, m))
	mux.HandleFunc(TypeSystemPing, handler.HandleSystemPingTask)
	return mux
}
