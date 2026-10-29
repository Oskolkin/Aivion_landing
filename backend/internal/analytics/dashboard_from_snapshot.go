package analytics

import (
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// BuildDashboardMetricsFromSnapshot assembles dashboard metrics purely from an AnalysisDataSnapshot.
func BuildDashboardMetricsFromSnapshot(snap *analysisdata.AnalysisDataSnapshot) (DashboardMetricsDTO, error) {
	if snap == nil {
		return DashboardMetricsDTO{}, fmt.Errorf("snapshot is nil")
	}
	if snap.SellerAccountID <= 0 {
		return DashboardMetricsDTO{}, fmt.Errorf("snapshot seller_account_id is invalid")
	}
	if !snap.MetricDate.Valid {
		return DashboardMetricsDTO{}, fmt.Errorf("snapshot metric_date is invalid")
	}

	asOf := normalizeDate(snap.MetricDate.Time)
	from14 := asOf.AddDate(0, 0, -13)
	from7 := asOf.AddDate(0, 0, -6)
	yesterday := asOf.AddDate(0, 0, -1)
	todayKey := asOf.Format("2006-01-02")
	yesterdayKey := yesterday.Format("2006-01-02")

	accountRows := filterAccountMetricsInRange(snap.AccountMetrics, from14, asOf)
	accountByDate := make(map[string]dbgen.DailyAccountMetric, len(accountRows))
	for _, row := range accountRows {
		accountByDate[row.MetricDate.Time.Format("2006-01-02")] = row
	}

	todayAccount := accountByDate[todayKey]
	yesterdayAccount := accountByDate[yesterdayKey]

	revenueToday := numericToFloat(todayAccount.Revenue)
	revenueYesterday := numericToFloat(yesterdayAccount.Revenue)
	last7Revenue := sumRevenueForRange(accountRows, asOf.AddDate(0, 0, -6), asOf)
	previous7Revenue := sumRevenueForRange(accountRows, asOf.AddDate(0, 0, -13), asOf.AddDate(0, 0, -7))

	skuRows := filterSKUMetricsInRange(snap.SKUMetrics, from7, asOf)
	enrichCtx := buildSKUEnrichmentContextFromSnapshot(snap, asOf, todayAccount)
	skuItems, skuUpdatedAt := buildDashboardSKUItems(asOf, todayKey, yesterdayKey, revenueToday, skuRows, enrichCtx)

	// Account-level orders KPI is canonical from daily_account_metrics, not a
	// sum of SKU orders (which excludes cancels and long-tail rows).
	ordersToday := todayAccount.OrdersCount
	ordersYesterday := yesterdayAccount.OrdersCount
	accountAdSpend := sumAdSpendForDateFromSnapshot(snap.AdMetrics, asOf)

	var maxUpdatedAt pgtype.Timestamptz
	maxUpdatedAt = maxTimestamptz(maxUpdatedAt, skuUpdatedAt)
	for _, row := range accountRows {
		maxUpdatedAt = maxTimestamptz(maxUpdatedAt, row.UpdatedAt)
	}

	return assembleDashboardMetricsDTO(dashboardMetricsAssemblyInput{
		SellerAccountID:  snap.SellerAccountID,
		AsOf:             asOf,
		AsOfSource:       "snapshot",
		TodayKey:         todayKey,
		RevenueToday:     revenueToday,
		RevenueYesterday: revenueYesterday,
		Last7Revenue:     last7Revenue,
		Previous7Revenue: previous7Revenue,
		OrdersToday:      ordersToday,
		OrdersYesterday:  ordersYesterday,
		TodayAccount:     todayAccount,
		AccountAdSpend:   accountAdSpend,
		OrdersByStatus:   nil,
		SKUItems:         skuItems,
		MaxUpdatedAt:     maxUpdatedAt,
	}), nil
}

func filterAccountMetricsInRange(rows []dbgen.DailyAccountMetric, from, to time.Time) []dbgen.DailyAccountMetric {
	fromDate := normalizeDate(from)
	toDate := normalizeDate(to)
	out := make([]dbgen.DailyAccountMetric, 0, len(rows))
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := normalizeDate(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func filterSKUMetricsInRange(rows []dbgen.DailySkuMetric, from, to time.Time) []dbgen.DailySkuMetric {
	fromDate := normalizeDate(from)
	toDate := normalizeDate(to)
	out := make([]dbgen.DailySkuMetric, 0, len(rows))
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := normalizeDate(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func sumAdSpendForDateFromSnapshot(rows []dbgen.AdMetricsDaily, asOf time.Time) float64 {
	key := asOf.Format("2006-01-02")
	var total float64
	for _, row := range rows {
		if !row.MetricDate.Valid || row.MetricDate.Time.Format("2006-01-02") != key {
			continue
		}
		total += numericToFloat(row.Spend)
	}
	return total
}
