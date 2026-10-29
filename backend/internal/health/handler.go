package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Response struct {
	Status string `json:"status"`
}

type ReadinessChecker interface {
	Check(ctx context.Context) error
}

type CompositeChecker struct {
	checkers []ReadinessChecker
}

func NewCompositeChecker(checkers ...ReadinessChecker) *CompositeChecker {
	return &CompositeChecker{
		checkers: checkers,
	}
}

func (c *CompositeChecker) Check(ctx context.Context) error {
	for _, checker := range c.checkers {
		if err := checker.Check(ctx); err != nil {
			return err
		}
	}
	return nil
}

type Handler struct {
	readinessChecker ReadinessChecker
}

func NewHandler(readinessChecker ReadinessChecker) *Handler {
	return &Handler{
		readinessChecker: readinessChecker,
	}
}

func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, Response{Status: "ok"})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.readinessChecker == nil {
		writeJSON(w, http.StatusOK, Response{Status: "ready"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := h.readinessChecker.Check(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, Response{Status: "not_ready"})
		return
	}

	writeJSON(w, http.StatusOK, Response{Status: "ready"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
