package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/go-chi/chi/v5"
)

func (h *ActionsHandler) AdminListGenerationRuns(w http.ResponseWriter, r *http.Request) {
	filters, err := parseAdminGenerationRunsFilters(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.service.ListGenerationRunsForAdmin(r.Context(), filters)
	if err != nil {
		if errors.Is(err, actions.ErrInvalidMetricDate) {
			writeJSONError(w, http.StatusBadRequest, "invalid metric_date, expected YYYY-MM-DD")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to list generation runs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  filters.Limit,
		"offset": filters.Offset,
	})
}

func (h *ActionsHandler) AdminGetGenerationRun(w http.ResponseWriter, r *http.Request) {
	runID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "run_id")), 10, 64)
	if err != nil || runID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid run_id")
		return
	}
	run, categoryRuns, err := h.service.GetGenerationRunDetailForAdmin(r.Context(), runID)
	if err != nil {
		if errors.Is(err, actions.ErrActionNotFound) {
			writeJSONError(w, http.StatusNotFound, "generation run not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get generation run")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"run":           run,
		"category_runs": categoryRuns,
	})
}

func parseAdminGenerationRunsFilters(r *http.Request) (actions.AdminListGenerationRunsFilters, error) {
	filters := actions.AdminListGenerationRunsFilters{
		Limit:  50,
		Offset: 0,
	}
	q := r.URL.Query()
	if raw := strings.TrimSpace(q.Get("seller_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return filters, errors.New("invalid seller_id")
		}
		filters.SellerAccountID = &id
	}
	filters.Status = strings.TrimSpace(q.Get("status"))
	filters.FinalStatus = strings.TrimSpace(q.Get("final_status"))
	filters.MetricDate = strings.TrimSpace(q.Get("metric_date"))
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return filters, errors.New("invalid limit")
		}
		if v > 200 {
			v = 200
		}
		filters.Limit = v
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return filters, errors.New("invalid offset")
		}
		filters.Offset = v
	}
	return filters, nil
}

func (h *ActionsHandler) AdminRerunActions(w http.ResponseWriter, r *http.Request) {
	if h.rerun == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "actions generation unavailable")
		return
	}
	var body struct {
		SellerID   int64  `json:"seller_id"`
		MetricDate string `json:"metric_date"`
		AIEnabled  *bool  `json:"ai_enabled"`
		Force      bool   `json:"force"`
	}
	if err := decodeJSONBody(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.SellerID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "seller_id is required")
		return
	}
	var metricDate *time.Time
	if strings.TrimSpace(body.MetricDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(body.MetricDate))
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid metric_date, expected YYYY-MM-DD")
			return
		}
		t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		metricDate = &t
	}
	result, err := h.rerun.RerunActions(r.Context(), actions.RerunActionsRequest{
		SellerAccountID: body.SellerID,
		MetricDate:      metricDate,
		AIEnabled:       body.AIEnabled,
		Force:           body.Force,
	})
	if err != nil {
		if errors.Is(err, actions.ErrInvalidSellerID) {
			writeJSONError(w, http.StatusBadRequest, "invalid seller_id")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to rerun actions generation")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
