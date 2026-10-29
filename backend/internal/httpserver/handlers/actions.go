package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/go-chi/chi/v5"
)

type ActionsHandler struct {
	service *actions.Service
	rerun   actions.ActionsRerunRunner
}

func NewActionsHandler(service *actions.Service, rerun actions.ActionsRerunRunner) *ActionsHandler {
	return &ActionsHandler{service: service, rerun: rerun}
}

func (h *ActionsHandler) ListActions(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	filters, err := parseActionsFilters(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.ListActions(r.Context(), sellerAccount.ID, filters)
	if err != nil {
		if errors.Is(err, actions.ErrInvalidMetricDate) {
			writeJSONError(w, http.StatusBadRequest, "invalid metric_date, expected YYYY-MM-DD or latest")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to list actions")
		return
	}
	setAnalysisSourceHeaders(w, resp.AnalysisSource, resp.PublicationID, resp.PipelineRunID, resp.MetricDate, resp.FallbackUsed, "")
	writeJSON(w, http.StatusOK, resp)
}

func (h *ActionsHandler) GetAction(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	actionKey, err := actionKeyFromRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	item, err := h.service.GetAction(r.Context(), sellerAccount.ID, actionKey)
	if err != nil {
		if errors.Is(err, actions.ErrActionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get action")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *ActionsHandler) GetActionByID(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	actionID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || actionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid action id")
		return
	}
	item, err := h.service.GetActionByID(r.Context(), sellerAccount.ID, actionID)
	if err != nil {
		if errors.Is(err, actions.ErrActionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get action")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *ActionsHandler) GetHistoricalActionByID(w http.ResponseWriter, r *http.Request) {
	rawID := strings.TrimSpace(chi.URLParam(r, "id"))
	actionID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || actionID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid action id")
		return
	}
	sellerID, err := strconv.ParseInt(strings.TrimSpace(chi.URLParam(r, "seller_account_id")), 10, 64)
	if err != nil || sellerID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid seller_account_id")
		return
	}
	item, err := h.service.GetHistoricalActionByID(r.Context(), sellerID, actionID)
	if err != nil {
		if errors.Is(err, actions.ErrActionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to get historical action")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *ActionsHandler) CheckAction(w http.ResponseWriter, r *http.Request) {
	h.handleActionState(w, r, true)
}

func (h *ActionsHandler) UncheckAction(w http.ResponseWriter, r *http.Request) {
	h.handleActionState(w, r, false)
}

func (h *ActionsHandler) DismissAction(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	actionKey, err := actionKeyFromRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.DismissAction(r.Context(), sellerAccount.ID, actionKey); err != nil {
		if errors.Is(err, actions.ErrActionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update action state")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ActionsHandler) ExportAction(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	actionKey, err := actionKeyFromRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	payload, mime, filename, err := h.service.ExportAction(r.Context(), sellerAccount.ID, actionKey, format)
	if err != nil {
		switch {
		case errors.Is(err, actions.ErrActionNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
		case errors.Is(err, actions.ErrUnsupportedExportFormat):
			writeJSONError(w, http.StatusBadRequest, "unsupported export format")
		case errors.Is(err, actions.ErrUnsupportedActionExport):
			writeJSONError(w, http.StatusUnprocessableEntity, "export for this action is not supported")
		default:
			writeJSONError(w, http.StatusInternalServerError, "failed to export action")
		}
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	_, _ = w.Write(payload)
}

func (h *ActionsHandler) handleActionState(w http.ResponseWriter, r *http.Request, check bool) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	actionKey, err := actionKeyFromRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var svcErr error
	if check {
		svcErr = h.service.CheckAction(r.Context(), sellerAccount.ID, actionKey)
	} else {
		svcErr = h.service.UncheckAction(r.Context(), sellerAccount.ID, actionKey)
	}
	if svcErr != nil {
		if errors.Is(svcErr, actions.ErrActionNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update action state")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func actionKeyFromRequest(r *http.Request) (string, error) {
	raw := chi.URLParam(r, "action_key")
	if raw == "" {
		return "", errors.New("action_key is required")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", errors.New("invalid action_key")
	}
	if err := actions.ValidateActionKey(decoded); err != nil {
		return "", err
	}
	return decoded, nil
}

func parseActionsFilters(r *http.Request) (actions.ListActionsFilters, error) {
	filters := actions.ListActionsFilters{
		Status:     string(actions.ActionStatusOpen),
		MetricDate: "latest",
		Limit:      50,
		Offset:     0,
	}
	q := r.URL.Query()

	if raw := strings.TrimSpace(q.Get("status")); raw != "" {
		switch actions.ActionStatus(raw) {
		case actions.ActionStatusOpen,
			actions.ActionStatusChecked,
			actions.ActionStatusSnoozed,
			actions.ActionStatusDismissed:
			filters.Status = raw
		case "all":
			filters.Status = "all"
		default:
			return actions.ListActionsFilters{}, errors.New("invalid status")
		}
	}

	if raw := strings.TrimSpace(q.Get("priority")); raw != "" {
		priority := actions.ActionPriority(raw)
		switch priority {
		case actions.ActionPriorityCritical,
			actions.ActionPriorityImportant,
			actions.ActionPriorityWatch:
			filters.Priority = &priority
		default:
			return actions.ListActionsFilters{}, errors.New("invalid priority")
		}
	}

	if raw := strings.TrimSpace(q.Get("category")); raw != "" {
		category := actions.ActionCategory(raw)
		switch category {
		case actions.ActionCategoryStock,
			actions.ActionCategorySlowTurnover,
			actions.ActionCategorySKU,
			actions.ActionCategoryAdvertising,
			actions.ActionCategoryFunnel,
			actions.ActionCategorySales,
			actions.ActionCategoryCategory:
			filters.Category = &category
		default:
			return actions.ListActionsFilters{}, errors.New("invalid category")
		}
	}

	if raw := strings.TrimSpace(q.Get("metric_date")); raw != "" {
		if raw != "latest" {
			if _, err := time.Parse("2006-01-02", raw); err != nil {
				return actions.ListActionsFilters{}, errors.New("invalid metric_date, expected YYYY-MM-DD or latest")
			}
		}
		filters.MetricDate = raw
	}

	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return actions.ListActionsFilters{}, errors.New("invalid limit")
		}
		if v > 100 {
			v = 100
		}
		filters.Limit = v
	}

	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return actions.ListActionsFilters{}, errors.New("invalid offset")
		}
		filters.Offset = v
	}

	return filters, nil
}
