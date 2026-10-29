package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
)

type AnalyticsDashboardHandler struct {
	dashboardService     *analytics.DashboardService
	stocksService        *analytics.StocksViewService
	criticalSKUService   *analytics.CriticalSKUService
	replenishmentService *analytics.ReplenishmentService
	advertisingService   *analytics.AdvertisingService
}

func NewAnalyticsDashboardHandler(
	dashboardService *analytics.DashboardService,
	stocksService *analytics.StocksViewService,
	criticalSKUService *analytics.CriticalSKUService,
	replenishmentService *analytics.ReplenishmentService,
	advertisingService *analytics.AdvertisingService,
) *AnalyticsDashboardHandler {
	return &AnalyticsDashboardHandler{
		dashboardService:     dashboardService,
		stocksService:        stocksService,
		criticalSKUService:   criticalSKUService,
		replenishmentService: replenishmentService,
		advertisingService:   advertisingService,
	}
}

type dashboardSummaryResponse struct {
	KPI struct {
		RevenueCurrent          float64                         `json:"revenue_current"`
		RevenueDayToDayDelta    analytics.DashboardDelta        `json:"revenue_day_to_day_delta"`
		RevenueWeekToWeekDelta  analytics.DashboardDelta        `json:"revenue_week_to_week_delta"`
		OrdersCurrent           int32                           `json:"orders_current"`
		OrdersDayToDayDelta     int32                           `json:"orders_day_to_day_delta"`
		ReturnsCurrent          int32                           `json:"returns_current"`
		CancelsCurrent          int32                           `json:"cancels_current"`
		AverageOrderValue       *float64                        `json:"average_order_value,omitempty"`
		UnitsSoldToday          *int32                          `json:"units_sold_today,omitempty"`
		AccountAdSpendToRevenue *float64                        `json:"account_ad_spend_to_revenue,omitempty"`
		CancellationRate        *float64                        `json:"cancellation_rate,omitempty"`
		OrdersByStatus          []analytics.OrdersByStatusCount `json:"orders_by_status,omitempty"`
	} `json:"kpi"`
	Summary struct {
		LastSuccessfulUpdate *string `json:"last_successful_update"`
		PeriodUsed           string  `json:"period_used"`
		DataFreshness        string  `json:"data_freshness"`
		AsOfDate             string  `json:"as_of_date"`
		AsOfDateSource       string  `json:"as_of_date_source"`
		KPISemantics         string  `json:"kpi_semantics"`
		SKUOrdersSemantics   string  `json:"sku_orders_semantics"`
	} `json:"summary"`
	TopSKUs []analytics.DashboardSKUMetricsItem `json:"top_skus"`
}

type skuTableResponse struct {
	Items  []analytics.DashboardSKUMetricsItem `json:"items"`
	Total  int                                 `json:"total"`
	Limit  int                                 `json:"limit"`
	Offset int                                 `json:"offset"`
}

type stockTableWarehouseRow struct {
	OzonProductID     int64   `json:"ozon_product_id"`
	OfferID           *string `json:"offer_id"`
	SKU               *int64  `json:"sku"`
	ProductName       *string `json:"product_name"`
	Warehouse         string  `json:"warehouse"`
	QuantityTotal     int32   `json:"quantity_total"`
	QuantityReserved  int32   `json:"quantity_reserved"`
	QuantityAvailable int32   `json:"quantity_available"`
	SnapshotAt        *string `json:"snapshot_at"`
}

type stocksTableResponse struct {
	Items []stockTableWarehouseRow `json:"items"`
	Total int                      `json:"total"`
	Meta  struct {
		Semantics string `json:"semantics"`
	} `json:"meta"`
}

type criticalSKUsResponse struct {
	Items []analytics.CriticalSKUCard `json:"items"`
	Meta  struct {
		AsOfDate            string            `json:"as_of_date"`
		ScoringSemantics    map[string]string `json:"scoring_semantics"`
		LatestDataTimestamp *string           `json:"latest_data_timestamp"`
		Total               int               `json:"total"`
		Limit               int               `json:"limit"`
		Offset              int               `json:"offset"`
		SortBy              string            `json:"sort_by"`
		SortOrder           string            `json:"sort_order"`
	} `json:"meta"`
}

type stocksReplenishmentResponse struct {
	Items []analytics.ReplenishmentSKUItem `json:"items"`
	Meta  struct {
		StockSemantics  string  `json:"stock_semantics"`
		AsOfDate        string  `json:"as_of_date"`
		LastStockUpdate *string `json:"last_stock_update"`
	} `json:"meta"`
}

type advertisingAnalyticsResponse struct {
	Summary   analytics.AdvertisingSummaryDTO      `json:"summary"`
	Campaigns []analytics.AdvertisingCampaignRow   `json:"campaigns"`
	SKURisks  []analytics.AdvertisingSKURiskRowDTO `json:"sku_risks"`
}

func (h *AnalyticsDashboardHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	asOf, err := parseAsOfDate(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
		return
	}

	dto, err := h.dashboardService.BuildDashboardMetrics(r.Context(), sellerAccount.ID, asOf)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build dashboard summary")
		return
	}

	response := mapDashboardSummaryResponse(dto)
	writeJSON(w, http.StatusOK, response)
}

func (h *AnalyticsDashboardHandler) GetSKUTable(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	asOf, err := parseAsOfDate(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
		return
	}

	dto, err := h.dashboardService.BuildDashboardMetrics(r.Context(), sellerAccount.ID, asOf)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build sku table")
		return
	}

	limit, offset := parsePaginationParams(r)
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))
	rows := append([]analytics.DashboardSKUMetricsItem(nil), dto.SKUs...)
	sortSKURows(rows, sortBy, sortOrder)

	total := len(rows)
	start := minInt(offset, total)
	end := minInt(start+limit, total)

	writeJSON(w, http.StatusOK, skuTableResponse{
		Items:  rows[start:end],
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func (h *AnalyticsDashboardHandler) GetStocksTable(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := h.stocksService.ListCurrentStocksBySellerAccount(r.Context(), sellerAccount.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build stocks table")
		return
	}

	flatRows := make([]stockTableWarehouseRow, 0)
	for _, product := range rows {
		for _, warehouse := range product.Warehouses {
			flatRows = append(flatRows, stockTableWarehouseRow{
				OzonProductID:     product.OzonProductID,
				OfferID:           product.OfferID,
				SKU:               product.SKU,
				ProductName:       product.ProductName,
				Warehouse:         warehouse.WarehouseExternalID,
				QuantityTotal:     warehouse.TotalStock,
				QuantityReserved:  warehouse.ReservedStock,
				QuantityAvailable: warehouse.AvailableStock,
				SnapshotAt:        warehouse.SnapshotAt,
			})
		}
	}

	sort.Slice(flatRows, func(i, j int) bool {
		if flatRows[i].QuantityAvailable == flatRows[j].QuantityAvailable {
			if flatRows[i].OzonProductID == flatRows[j].OzonProductID {
				return flatRows[i].Warehouse < flatRows[j].Warehouse
			}
			return flatRows[i].OzonProductID < flatRows[j].OzonProductID
		}
		return flatRows[i].QuantityAvailable > flatRows[j].QuantityAvailable
	})

	writeJSON(w, http.StatusOK, stocksTableResponse{
		Items: flatRows,
		Total: len(flatRows),
		Meta: struct {
			Semantics string `json:"semantics"`
		}{
			Semantics: "Current-state snapshot view from latest row per product+warehouse; not historical stock timeline.",
		},
	})
}

func (h *AnalyticsDashboardHandler) GetCriticalSKUs(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	asOfDate := time.Now().UTC()
	asOf, err := parseAsOfDate(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
		return
	}
	if asOf != nil {
		asOfDate = *asOf
	}

	result, err := h.criticalSKUService.ListCriticalSKUsForSellerAccount(r.Context(), sellerAccount.ID, asOfDate)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build critical skus")
		return
	}

	rows := append([]analytics.CriticalSKUCard(nil), result.Items...)
	limit, offset := parsePaginationParams(r)
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))
	sortCriticalSKURows(rows, sortBy, sortOrder)

	total := len(rows)
	start := minInt(offset, total)
	end := minInt(start+limit, total)

	writeJSON(w, http.StatusOK, mapCriticalSKUsResponse(result, rows[start:end], total, limit, offset, sortBy, sortOrder))
}

func (h *AnalyticsDashboardHandler) GetStocksReplenishment(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	asOfDate := time.Now().UTC()
	asOf, err := parseAsOfDate(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid as_of_date, expected YYYY-MM-DD")
		return
	}
	if asOf != nil {
		asOfDate = *asOf
	}

	result, err := h.replenishmentService.ListReplenishmentForSellerAccount(r.Context(), sellerAccount.ID, asOfDate)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build stocks replenishment")
		return
	}

	writeJSON(w, http.StatusOK, mapStocksReplenishmentResponse(result))
}

func (h *AnalyticsDashboardHandler) GetAdvertisingAnalytics(w http.ResponseWriter, r *http.Request) {
	sellerAccount, ok := auth.SellerAccountFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	dateFrom, dateTo, err := parseAdvertisingDateRange(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid date range, expected YYYY-MM-DD")
		return
	}
	limit, offset := parsePaginationParams(r)
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))

	result, err := h.advertisingService.BuildAdvertisingAnalytics(r.Context(), sellerAccount.ID, analytics.AdvertisingQuery{
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		Limit:     limit,
		Offset:    offset,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build advertising analytics")
		return
	}

	writeJSON(w, http.StatusOK, advertisingAnalyticsResponse{
		Summary:   result.Summary,
		Campaigns: result.Campaign,
		SKURisks:  result.SKURisks,
	})
}

func mapDashboardSummaryResponse(dto analytics.DashboardMetricsDTO) dashboardSummaryResponse {
	response := dashboardSummaryResponse{}
	response.KPI.RevenueCurrent = dto.Account.RevenueToday
	response.KPI.RevenueDayToDayDelta = dto.Account.RevenueDayDelta
	response.KPI.RevenueWeekToWeekDelta = dto.Account.RevenueWeekDelta
	response.KPI.OrdersCurrent = dto.Account.OrdersToday
	response.KPI.OrdersDayToDayDelta = dto.Account.OrdersToday - dto.Account.OrdersYesterday
	response.KPI.ReturnsCurrent = dto.Account.ReturnsToday
	response.KPI.CancelsCurrent = dto.Account.CancelsToday
	response.KPI.AverageOrderValue = dto.Account.AverageOrderValue
	response.KPI.UnitsSoldToday = dto.Account.UnitsSoldToday
	response.KPI.AccountAdSpendToRevenue = dto.Account.AccountAdSpendToRevenue
	response.KPI.CancellationRate = dto.Account.CancellationRate
	response.KPI.OrdersByStatus = dto.Account.OrdersByStatus

	response.Summary.LastSuccessfulUpdate = dto.LastUpdatedAt
	response.Summary.PeriodUsed = dto.Summary.PeriodUsed
	response.Summary.DataFreshness = dto.Summary.DataFreshness
	response.Summary.AsOfDate = dto.AsOfDate
	response.Summary.AsOfDateSource = dto.AsOfDateSource
	response.Summary.KPISemantics = dto.Summary.KPISemantics
	response.Summary.SKUOrdersSemantics = dto.Summary.SKUOrdersSemantics

	top := dto.SKUs
	if len(top) > 5 {
		top = top[:5]
	}
	response.TopSKUs = top
	return response
}

func mapCriticalSKUsResponse(
	result analytics.CriticalSKUResult,
	items []analytics.CriticalSKUCard,
	total int,
	limit int,
	offset int,
	sortBy string,
	sortOrder string,
) criticalSKUsResponse {
	response := criticalSKUsResponse{
		Items: items,
	}
	response.Meta.AsOfDate = result.AsOfDate
	response.Meta.ScoringSemantics = result.ScoringSemantics
	response.Meta.LatestDataTimestamp = result.LatestDataTimestamp
	response.Meta.Total = total
	response.Meta.Limit = limit
	response.Meta.Offset = offset
	response.Meta.SortBy = sortBy
	if response.Meta.SortBy == "" {
		response.Meta.SortBy = "problem_score"
	}
	response.Meta.SortOrder = sortOrder
	if response.Meta.SortOrder == "" {
		response.Meta.SortOrder = "desc"
	}
	return response
}

func mapStocksReplenishmentResponse(result analytics.ReplenishmentResult) stocksReplenishmentResponse {
	response := stocksReplenishmentResponse{
		Items: result.Items,
	}
	response.Meta.StockSemantics = result.StockSemantics
	response.Meta.AsOfDate = result.AsOfDate
	response.Meta.LastStockUpdate = result.LastStockUpdate
	return response
}

func parseAsOfDate(r *http.Request) (*time.Time, error) {
	asOfRaw := r.URL.Query().Get("as_of_date")
	if strings.TrimSpace(asOfRaw) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", asOfRaw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parsePaginationParams(r *http.Request) (int, int) {
	limit := 20
	offset := 0

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
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

func parseAdvertisingDateRange(r *http.Request) (time.Time, time.Time, error) {
	dateTo := normalizeDateValue(time.Now().UTC())
	dateFrom := dateTo.AddDate(0, 0, -29)

	if raw := strings.TrimSpace(r.URL.Query().Get("date_to")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		dateTo = normalizeDateValue(parsed)
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("date_from")); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		dateFrom = normalizeDateValue(parsed)
	}
	if dateFrom.After(dateTo) {
		return time.Time{}, time.Time{}, fmt.Errorf("date_from is after date_to")
	}
	return dateFrom, dateTo, nil
}

func normalizeDateValue(v time.Time) time.Time {
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
}

func sortSKURows(rows []analytics.DashboardSKUMetricsItem, sortBy string, sortOrder string) {
	desc := sortOrder != "asc"
	if sortBy == "" {
		sortBy = "revenue"
	}

	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]

		switch sortBy {
		case "orders_count":
			if left.OrdersCount == right.OrdersCount {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.OrdersCount > right.OrdersCount
			}
			return left.OrdersCount < right.OrdersCount
		case "share_of_revenue":
			lv := safeFloat(left.ShareOfRevenue)
			rv := safeFloat(right.ShareOfRevenue)
			if lv == rv {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return lv > rv
			}
			return lv < rv
		case "contribution_to_revenue_change":
			if left.ContributionToRevenueChange == right.ContributionToRevenueChange {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.ContributionToRevenueChange > right.ContributionToRevenueChange
			}
			return left.ContributionToRevenueChange < right.ContributionToRevenueChange
		case "stock_available":
			if left.StockAvailable == right.StockAvailable {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.StockAvailable > right.StockAvailable
			}
			return left.StockAvailable < right.StockAvailable
		case "days_of_cover":
			lv := safeFloat(left.DaysOfCover)
			rv := safeFloat(right.DaysOfCover)
			if lv == rv {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return lv > rv
			}
			return lv < rv
		case "product_name":
			ln := strings.ToLower(safeString(left.ProductName))
			rn := strings.ToLower(safeString(right.ProductName))
			if ln == rn {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return ln > rn
			}
			return ln < rn
		default:
			if left.Revenue == right.Revenue {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.Revenue > right.Revenue
			}
			return left.Revenue < right.Revenue
		}
	})
}

func sortCriticalSKURows(rows []analytics.CriticalSKUCard, sortBy string, sortOrder string) {
	desc := sortOrder != "asc"
	if sortBy == "" {
		sortBy = "problem_score"
	}

	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		switch sortBy {
		case "importance":
			if left.Importance == right.Importance {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.Importance > right.Importance
			}
			return left.Importance < right.Importance
		case "out_of_stock_risk":
			if left.OutOfStockRisk == right.OutOfStockRisk {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.OutOfStockRisk > right.OutOfStockRisk
			}
			return left.OutOfStockRisk < right.OutOfStockRisk
		case "revenue_delta_day":
			if left.RevenueDeltaDay == right.RevenueDeltaDay {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.RevenueDeltaDay > right.RevenueDeltaDay
			}
			return left.RevenueDeltaDay < right.RevenueDeltaDay
		case "stock_available":
			if left.StockAvailable == right.StockAvailable {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.StockAvailable > right.StockAvailable
			}
			return left.StockAvailable < right.StockAvailable
		default:
			if left.ProblemScore == right.ProblemScore {
				return left.OzonProductID < right.OzonProductID
			}
			if desc {
				return left.ProblemScore > right.ProblemScore
			}
			return left.ProblemScore < right.ProblemScore
		}
	})
}

func safeFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func safeString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
