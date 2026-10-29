package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/go-chi/chi/v5"
)

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unknown"
}

func Metrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := newStatusResponseWriter(w)

			next.ServeHTTP(sw, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(sw.statusCode)
			pattern := routePattern(r)

			m.HTTPRequestTotal.WithLabelValues(r.Method, pattern, status).Inc()
			m.HTTPRequestDuration.WithLabelValues(r.Method, pattern, status).Observe(duration)
		})
	}
}
