package middleware

import (
	"net/http"
	"strings"
)

// MetricsAuth protects /metrics with bearer token unless explicitly allowed.
func MetricsAuth(token string, allowUnauthenticated bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if allowUnauthenticated {
				next.ServeHTTP(w, r)
				return
			}
			if strings.TrimSpace(token) == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			const prefix = "Bearer "
			if auth != prefix+token {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
