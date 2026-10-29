package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardService struct {
	queries *dbgen.Queries
}

func NewDashboardService(db *pgxpool.Pool) *DashboardService {
	return &DashboardService{
		queries: dbgen.New(db),
	}
}

type DashboardMetricsDTO struct {
	SellerAccountID int64                     `json:"seller_account_id"`
	AsOfDate        string                    `json:"as_of_date"`
	AsOfDateSource  string                    `json:"as_of_date_source"`
	LastUpdatedAt   *string                   `json:"last_updated_at"`
	Summary         DashboardSummaryMeta      `json:"summary"`
	Account         DashboardAccountMetrics   `json:"account"`
	SKUs            []DashboardSKUMetricsItem `json:"skus"`
}

type DashboardSummaryMeta struct {
	PeriodUsed              string `json:"period_used"`
	DataFreshness           string `json:"data_freshness"`
	KPISemantics            string `json:"kpi_semantics"`
	SKUOrdersSemantics      string `json:"sku_orders_semantics"`
	StocksSemantics         string `json:"stocks_semantics"`
	ContributionSemantics   string `json:"contribution_semantics"`
	ShareOfRevenueSemantics string `json:"share_of_revenue_semantics"`
}

type OrdersByStatusCount struct {
	Status      string `json:"status"`
	OrdersCount int64  `json:"orders_count"`
}

type DashboardAccountMetrics struct {
	RevenueToday            float64               `json:"revenue_today"`
	RevenueYesterday        float64               `json:"revenue_yesterday"`
	RevenueLast7Days        float64               `json:"revenue_last_7d"`
	RevenueDayDelta         DashboardDelta        `json:"revenue_day_to_day_delta"`
	RevenueWeekDelta        DashboardDelta        `json:"revenue_week_to_week_delta"`
	OrdersToday             int32                 `json:"orders_today"`
	OrdersYesterday         int32                 `json:"orders_yesterday"`
	ReturnsToday            int32                 `json:"returns_today"`
	CancelsToday            int32                 `json:"cancels_today"`
	PreviousWeekTotal       float64               `json:"previous_week_revenue"`
	AverageOrderValue       *float64              `json:"average_order_value,omitempty"`
	UnitsSoldToday          *int32                `json:"units_sold_today,omitempty"`
	AccountAdSpendToRevenue *float64              `json:"account_ad_spend_to_revenue,omitempty"`
	CancellationRate        *float64              `json:"cancellation_rate,omitempty"`
	OrdersByStatus          []OrdersByStatusCount `json:"orders_by_status,omitempty"`
}

type DashboardDelta struct {
	Abs float64  `json:"abs"`
	Pct *float64 `json:"pct"`
}

type DashboardSKUMetricsItem struct {
	OzonProductID               int64    `json:"ozon_product_id"`
	OfferID                     *string  `json:"offer_id"`
	SKU                         *int64   `json:"sku"`
	ProductName                 *string  `json:"product_name"`
	Revenue                     float64  `json:"revenue"`
	OrdersCount                 int32    `json:"orders_count"`
	UnitsSold                   *int32   `json:"units_sold,omitempty"`
	StockAvailable              int32    `json:"stock_available"`
	DaysOfCover                 *float64 `json:"days_of_cover"`
	RevenueDeltaDayToDay        float64  `json:"revenue_delta_day_to_day"`
	OrdersDeltaDayToDay         int32    `json:"orders_delta_day_to_day"`
	ShareOfRevenue              *float64 `json:"share_of_revenue"`
	ContributionToRevenueChange float64  `json:"contribution_to_revenue_change"`
	SKUOrdersShare              *float64 `json:"sku_orders_share,omitempty"`
	SKURevenueShare             *float64 `json:"sku_revenue_share,omitempty"`
	SKUSalesVelocityOrders7D    *float64 `json:"sku_sales_velocity_orders_7d,omitempty"`
	SKUSalesVelocityOrders14D   *float64 `json:"sku_sales_velocity_orders_14d,omitempty"`
	SKUSalesVelocityUnits7D     *float64 `json:"sku_sales_velocity_units_7d,omitempty"`
	SKUSalesVelocityUnits14D    *float64 `json:"sku_sales_velocity_units_14d,omitempty"`
	SKUOrdersDeltaPercent       *float64 `json:"sku_orders_delta_percent,omitempty"`
	DiscountDepth               *float64 `json:"discount_depth,omitempty"`
	OzonIDC                     *int32   `json:"ozon_idc,omitempty"`
	OzonADS                     *float64 `json:"ozon_ads,omitempty"`
	StockTurnoverGrade          *string  `json:"stock_turnover_grade,omitempty"`
	ExcessStockShare            *float64 `json:"excess_stock_share,omitempty"`
	DaysWithoutSales            *int32   `json:"days_without_sales,omitempty"`
	StockStatusBucket           *string  `json:"stock_status_bucket,omitempty"`
	StockStatusBucketSource     *string  `json:"stock_status_bucket_source,omitempty"`
	// RevenueLast7Days / OrdersLast7Days / ActiveSalesDays7D are aggregated from daily_sku_metrics for metric_date in [asOf-6, asOf].
	RevenueLast7Days  float64 `json:"revenue_last_7d"`
	OrdersLast7Days   int64   `json:"orders_last_7d"`
	ActiveSalesDays7D int     `json:"active_sales_days_7d"`
}

func (s *DashboardService) BuildDashboardMetrics(ctx context.Context, sellerAccountID int64, asOfDate *time.Time) (DashboardMetricsDTO, error) {
	asOf, asOfSource, err := s.resolveAsOfDate(ctx, sellerAccountID, asOfDate)
	if err != nil {
		return DashboardMetricsDTO{}, err
	}
	from := asOf.AddDate(0, 0, -13)
	yesterday := asOf.AddDate(0, 0, -1)

	accountRows, err := s.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(asOf),
	})
	if err != nil {
		return DashboardMetricsDTO{}, fmt.Errorf("list account metrics by date range: %w", err)
	}

	accountByDate := make(map[string]dbgen.DailyAccountMetric, len(accountRows))
	for _, row := range accountRows {
		accountByDate[row.MetricDate.Time.Format("2006-01-02")] = row
	}

	todayKey := asOf.Format("2006-01-02")
	yesterdayKey := yesterday.Format("2006-01-02")
	todayAccount := accountByDate[todayKey]
	yesterdayAccount := accountByDate[yesterdayKey]

	revenueToday := numericToFloat(todayAccount.Revenue)
	revenueYesterday := numericToFloat(yesterdayAccount.Revenue)
	last7Revenue := sumRevenueForRange(accountRows, asOf.AddDate(0, 0, -6), asOf)
	previous7Revenue := sumRevenueForRange(accountRows, asOf.AddDate(0, 0, -13), asOf.AddDate(0, 0, -7))

	from7 := asOf.AddDate(0, 0, -6)
	skuRows, err := s.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(from7),
		MetricDate_2:    dateValue(asOf),
	})
	if err != nil {
		return DashboardMetricsDTO{}, fmt.Errorf("list sku metrics by date range: %w", err)
	}

	enrichCtx, err := loadSKUEnrichmentContext(ctx, s.queries, sellerAccountID, asOf, todayAccount)
	if err != nil {
		return DashboardMetricsDTO{}, err
	}

	accountAdSpend, err := s.sumAdSpendForDate(ctx, sellerAccountID, asOf)
	if err != nil {
		return DashboardMetricsDTO{}, err
	}

	ordersByStatus, err := s.queries.CountOrdersByStatusForDateRange(ctx, dbgen.CountOrdersByStatusForDateRangeParams{
		SellerAccountID: sellerAccountID,
		Column2:         dateValue(asOf),
		Column3:         dateValue(asOf),
	})
	if err != nil {
		return DashboardMetricsDTO{}, fmt.Errorf("count orders by status: %w", err)
	}

	skuItems, skuUpdatedAt := buildDashboardSKUItems(asOf, todayKey, yesterdayKey, revenueToday, skuRows, enrichCtx)
	// Account-level orders KPI is canonical from daily_account_metrics, not a
	// sum of SKU orders (which excludes cancels and long-tail rows).
	ordersToday := todayAccount.OrdersCount
	ordersYesterday := yesterdayAccount.OrdersCount

	var maxUpdatedAt pgtype.Timestamptz
	maxUpdatedAt = maxTimestamptz(maxUpdatedAt, skuUpdatedAt)
	for _, row := range accountRows {
		maxUpdatedAt = maxTimestamptz(maxUpdatedAt, row.UpdatedAt)
	}

	return assembleDashboardMetricsDTO(dashboardMetricsAssemblyInput{
		SellerAccountID:  sellerAccountID,
		AsOf:             asOf,
		AsOfSource:       asOfSource,
		TodayKey:         todayKey,
		RevenueToday:     revenueToday,
		RevenueYesterday: revenueYesterday,
		Last7Revenue:     last7Revenue,
		Previous7Revenue: previous7Revenue,
		OrdersToday:      ordersToday,
		OrdersYesterday:  ordersYesterday,
		TodayAccount:     todayAccount,
		AccountAdSpend:   accountAdSpend,
		OrdersByStatus:   mapOrdersByStatus(ordersByStatus),
		SKUItems:         skuItems,
		MaxUpdatedAt:     maxUpdatedAt,
	}), nil
}

func (s *DashboardService) resolveAsOfDate(ctx context.Context, sellerAccountID int64, asOfDate *time.Time) (time.Time, string, error) {
	if asOfDate != nil {
		return normalizeDate(*asOfDate), "request", nil
	}

	latestDate, err := s.queries.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("get latest available dashboard metric date: %w", err)
	}
	if latestDate.Valid && latestDate.Time.Year() > 1970 {
		return normalizeDate(latestDate.Time), "latest_available", nil
	}

	return normalizeDate(time.Now().UTC()), "fallback_today", nil
}

func buildDashboardAccountMetrics(
	revenueToday, revenueYesterday, last7Revenue, previous7Revenue float64,
	ordersToday, ordersYesterday int32,
	todayAccount dbgen.DailyAccountMetric,
	accountAdSpend float64,
	ordersByStatus []OrdersByStatusCount,
) DashboardAccountMetrics {
	metrics := DashboardAccountMetrics{
		RevenueToday:            revenueToday,
		RevenueYesterday:        revenueYesterday,
		RevenueLast7Days:        last7Revenue,
		RevenueDayDelta:         buildDelta(revenueToday, revenueYesterday),
		RevenueWeekDelta:        buildDelta(last7Revenue, previous7Revenue),
		OrdersToday:             ordersToday,
		OrdersYesterday:         ordersYesterday,
		ReturnsToday:            todayAccount.ReturnsCount,
		CancelsToday:            todayAccount.CancelCount,
		PreviousWeekTotal:       previous7Revenue,
		AverageOrderValue:       RatioOrNil(revenueToday, float64(todayAccount.OrdersCount)),
		CancellationRate:        CancellationRate(todayAccount.CancelCount, todayAccount.OrdersCount),
		AccountAdSpendToRevenue: AccountAdSpendToRevenue(accountAdSpend, revenueToday),
	}
	if u, ok := int32FromPgInt4(todayAccount.UnitsSold); ok {
		metrics.UnitsSoldToday = &u
	}
	if len(ordersByStatus) > 0 {
		metrics.OrdersByStatus = ordersByStatus
	}
	return metrics
}

func mapOrdersByStatus(rows []dbgen.CountOrdersByStatusForDateRangeRow) []OrdersByStatusCount {
	if len(rows) == 0 {
		return nil
	}
	out := make([]OrdersByStatusCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, OrdersByStatusCount{
			Status:      fmt.Sprint(row.Status),
			OrdersCount: row.OrdersCount,
		})
	}
	return out
}

func (s *DashboardService) sumAdSpendForDate(ctx context.Context, sellerAccountID int64, asOf time.Time) (float64, error) {
	rows, err := s.queries.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        dateValue(asOf),
		DateTo:          dateValue(asOf),
	})
	if err != nil {
		return 0, fmt.Errorf("list ad metrics daily: %w", err)
	}
	var total float64
	for _, row := range rows {
		total += numericToFloat(row.Spend)
	}
	return total, nil
}

func buildPeriodUsed(asOfDate string) string {
	return asOfDate + " (d) / last 7 days for WoW"
}

func resolveDataFreshness(asOfDate string) string {
	asOf, err := time.Parse("2006-01-02", asOfDate)
	if err != nil {
		return "unknown"
	}

	daysLag := int(time.Since(asOf.UTC()).Hours() / 24)
	switch {
	case daysLag <= 0:
		return "fresh"
	case daysLag <= 1:
		return "stale_1d"
	default:
		return "stale"
	}
}

func maxTimestamptz(current pgtype.Timestamptz, next pgtype.Timestamptz) pgtype.Timestamptz {
	if !next.Valid {
		return current
	}
	if !current.Valid || next.Time.After(current.Time) {
		return next
	}
	return current
}

func buildDelta(current float64, previous float64) DashboardDelta {
	abs := round2(current - previous)
	return DashboardDelta{
		Abs: abs,
		Pct: safeRatio(abs, previous),
	}
}

func sumRevenueForRange(rows []dbgen.DailyAccountMetric, from time.Time, to time.Time) float64 {
	fromDate := normalizeDate(from)
	toDate := normalizeDate(to)
	var sum float64
	for _, row := range rows {
		metricDate := normalizeDate(row.MetricDate.Time)
		if metricDate.Before(fromDate) || metricDate.After(toDate) {
			continue
		}
		sum += numericToFloat(row.Revenue)
	}
	return round2(sum)
}

func safeRatio(numerator float64, denominator float64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := round2(numerator / denominator)
	return &value
}

func numericToFloat(n pgtype.Numeric) float64 {
	if value := numericToFloatPtr(n); value != nil {
		return *value
	}
	return 0
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	value := round2(v.Float64)
	return &value
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
