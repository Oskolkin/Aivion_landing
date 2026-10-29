package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type ActionViewHandler struct {
	service *actionview.Service
}

func NewActionViewHandler(service *actionview.Service) *ActionViewHandler {
	return &ActionViewHandler{service: service}
}

func (h *ActionViewHandler) List(w http.ResponseWriter, r *http.Request) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	f := actionview.ListFilter{
		Status: r.URL.Query().Get("status"),
		Family: r.URL.Query().Get("family"),
		Query:  r.URL.Query().Get("q"),
		Cursor: r.URL.Query().Get("cursor"),
		Limit:  limit,
	}
	resp, err := h.service.List(r.Context(), seller.ID, f)
	if errors.Is(err, actionview.ErrCursorMismatch) {
		writeJSONError(w, http.StatusBadRequest, "cursor_mismatch")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list actions")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ActionViewHandler) Get(w http.ResponseWriter, r *http.Request) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	viewID := chi.URLParam(r, "action_view_id")
	resp, err := h.service.Get(r.Context(), seller.ID, viewID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "action_not_found"})
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *ActionViewHandler) mutate(w http.ResponseWriter, r *http.Request, eventType string) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	caseID := chi.URLParam(r, "case_id")
	var userID *int64
	if u, ok := auth.UserFromContext(r.Context()); ok {
		id := u.ID
		userID = &id
	}
	if err := h.service.Mutate(r.Context(), seller.ID, userID, caseID, eventType); err != nil {
		writeJSONError(w, http.StatusBadRequest, "mutation_failed")
		return
	}
	writeJSON(w, http.StatusOK, actionview.MutationResponse{CaseID: caseID, UserState: eventType})
}

func (h *ActionViewHandler) Done(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "DONE")
}
func (h *ActionViewHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "DISMISSED")
}
func (h *ActionViewHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "REOPENED")
}

func (h *ActionViewHandler) Bulk(w http.ResponseWriter, r *http.Request) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req actionview.BulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid body")
		return
	}
	var userID *int64
	if u, ok := auth.UserFromContext(r.Context()); ok {
		id := u.ID
		userID = &id
	}
	if err := h.service.Bulk(r.Context(), seller.ID, userID, req.CaseIDs, req.EventType); err != nil {
		writeJSONError(w, http.StatusBadRequest, "bulk_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(req.CaseIDs)})
}
