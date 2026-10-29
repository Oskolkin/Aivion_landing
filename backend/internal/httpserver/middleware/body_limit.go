package middleware

import (
	"encoding/json"
	"net/http"
)

// MaxBytes returns middleware that limits request body size.
func MaxBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func writePayloadTooLarge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "payload too large"})
}

// RecoverMaxBytesError maps http.MaxBytesReader errors to 413 responses.
func RecoverMaxBytesError(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if err, ok := rec.(error); ok && err.Error() == "http: request body too large" {
					writePayloadTooLarge(w)
					return
				}
				panic(rec)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
