package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

func Logging(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := newStatusResponseWriter(w)

			next.ServeHTTP(sw, r)

			duration := time.Since(start)

			log.Info("http request completed",
				zap.String("request_id", GetRequestID(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", sw.statusCode),
				zap.Int64("duration_ms", duration.Milliseconds()),
			)
		})
	}
}
