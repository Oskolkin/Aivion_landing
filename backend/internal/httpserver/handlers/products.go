package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productcatalog"
)

type ProductsHandler struct {
	service *productcatalog.Service
}

func NewProductsHandler(service *productcatalog.Service) *ProductsHandler {
	return &ProductsHandler{service: service}
}

func (h *ProductsHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	params := parseProductsListParams(r)
	result, err := h.service.ListForSeller(r.Context(), sellerAccount.ID, params)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list products")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func parseProductsListParams(r *http.Request) productcatalog.ListParams {
	limit, offset := parseProductsPagination(r)
	sortBy := strings.TrimSpace(r.URL.Query().Get("sort"))
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	stockFilter := strings.TrimSpace(r.URL.Query().Get("stock_status"))

	var hasSales *bool
	switch strings.TrimSpace(r.URL.Query().Get("has_sales_last_day")) {
	case "true", "1":
		v := true
		hasSales = &v
	case "false", "0":
		v := false
		hasSales = &v
	}

	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	keySKUsOnly := filter == "key-skus" || filter == "key_skus"

	return productcatalog.ListParams{
		Search:            search,
		Limit:             limit,
		Offset:            offset,
		SortBy:            sortBy,
		HasSalesLastDay:   hasSales,
		StockStatusFilter: stockFilter,
		KeySKUsOnly:       keySKUsOnly,
	}
}

func parseProductsPagination(r *http.Request) (int, int) {
	limit := productcatalog.DefaultLimit
	offset := 0

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			if parsed > productcatalog.MaxLimit {
				parsed = productcatalog.MaxLimit
			}
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return limit, offset
}
