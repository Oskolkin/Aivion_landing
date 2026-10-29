package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
)

type recoveryResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id"`
}

func Recovery(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					requestID := GetRequestID(r.Context())

					sentry.WithScope(func(scope *sentry.Scope) {
						scope.SetTag("request_id", requestID)
						scope.SetTag("method", r.Method)
						scope.SetTag("path", r.URL.Path)
						scope.SetLevel(sentry.LevelError)
						scope.SetContext("http", map[string]interface{}{
							"method":     r.Method,
							"path":       r.URL.Path,
							"request_id": requestID,
						})
						sentry.CurrentHub().Recover(rec)
						sentry.Flush(2 * time.Second)
					})

					log.Error("panic recovered",
						zap.String("request_id", requestID),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.Any("panic", rec),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					_ = json.NewEncoder(w).Encode(recoveryResponse{
						Error:     "internal server error",
						RequestID: requestID,
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
