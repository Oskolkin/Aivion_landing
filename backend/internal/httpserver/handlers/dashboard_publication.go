package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboard"
	"github.com/jackc/pgx/v5"
)

type PublicationDashboardHandler struct {
	service *dashboard.Service
}

func NewPublicationDashboardHandler(service *dashboard.Service) *PublicationDashboardHandler {
	return &PublicationDashboardHandler{service: service}
}

func (h *PublicationDashboardHandler) GetHome(w http.ResponseWriter, r *http.Request) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	resp, err := h.service.Home(r.Context(), seller.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *PublicationDashboardHandler) GetStockSKUs(w http.ResponseWriter, r *http.Request) {
	h.listRows(w, r, true)
}

func (h *PublicationDashboardHandler) GetAdCampaigns(w http.ResponseWriter, r *http.Request) {
	h.listRows(w, r, false)
}

func (h *PublicationDashboardHandler) listRows(w http.ResponseWriter, r *http.Request, stock bool) {
	seller, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	pubID, err := strconv.ParseInt(r.URL.Query().Get("publication_id"), 10, 64)
	if err != nil || pubID <= 0 {
		writeJSONError(w, http.StatusBadRequest, "publication_id is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	var resp dashboard.RowsResponse
	if stock {
		var noSales *bool
		if r.URL.Query().Get("no_sales") == "true" {
			v := true
			noSales = &v
		}
		resp, err = h.service.StockSKUs(r.Context(), seller.ID, pubID, cursor, r.URL.Query().Get("coverage_band"), noSales, limit)
	} else {
		resp, err = h.service.AdCampaigns(r.Context(), seller.ID, pubID, cursor, limit)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "publication_not_found"})
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list rows")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}
