package analytics

import (
	"math/bits"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type dashboardMetricsAssemblyInput struct {
	SellerAccountID int64
	AsOf            time.Time
	AsOfSource      string
	TodayKey        string

	RevenueToday     float64
	RevenueYesterday float64
	Last7Revenue     float64
	Previous7Revenue float64
	OrdersToday      int32
	OrdersYesterday  int32

	TodayAccount   dbgen.DailyAccountMetric
	AccountAdSpend float64
	OrdersByStatus []OrdersByStatusCount
	SKUItems       []DashboardSKUMetricsItem
	MaxUpdatedAt   pgtype.Timestamptz
}

func assembleDashboardMetricsDTO(in dashboardMetricsAssemblyInput) DashboardMetricsDTO {
	return DashboardMetricsDTO{
		SellerAccountID: in.SellerAccountID,
		AsOfDate:        in.TodayKey,
		AsOfDateSource:  in.AsOfSource,
		LastUpdatedAt:   timestamptzToRFC3339(in.MaxUpdatedAt),
		Summary: DashboardSummaryMeta{
			PeriodUsed:              buildPeriodUsed(in.TodayKey),
			DataFreshness:           resolveDataFreshness(in.TodayKey),
			KPISemantics:            "Revenue and orders are aligned to sales-based metric_date for dashboard consistency; returns/cancels are order-status counts for the same metric_date.",
			SKUOrdersSemantics:      "SKU orders_count is sales-operation count on SKU/day grain.",
			StocksSemantics:         "Stocks endpoint and stock fields represent current-state snapshot from latest warehouse rows, not historical stock flow.",
			ContributionSemantics:   "contribution_to_revenue_change = sku_revenue(day) - sku_revenue(day-1).",
			ShareOfRevenueSemantics: "share_of_revenue = sku_revenue(day) / account_revenue(day).",
		},
		Account: buildDashboardAccountMetrics(
			in.RevenueToday, in.RevenueYesterday, in.Last7Revenue, in.Previous7Revenue,
			in.OrdersToday, in.OrdersYesterday,
			in.TodayAccount, in.AccountAdSpend, in.OrdersByStatus,
		),
		SKUs: in.SKUItems,
	}
}

func buildDashboardSKUItems(
	asOf time.Time,
	todayKey string,
	yesterdayKey string,
	revenueToday float64,
	skuRows []dbgen.DailySkuMetric,
	enrichCtx skuEnrichmentContext,
) ([]DashboardSKUMetricsItem, pgtype.Timestamptz) {
	from7 := asOf.AddDate(0, 0, -6)

	type sku7dAgg struct {
		revenue       float64
		orders        int64
		activeDayMask uint8
	}
	agg7d := make(map[int64]*sku7dAgg, len(skuRows))
	skusToday := make([]dbgen.DailySkuMetric, 0)
	skusYesterdayByProduct := make(map[int64]dbgen.DailySkuMetric)
	for _, row := range skuRows {
		key := row.MetricDate.Time.Format("2006-01-02")
		if key == todayKey {
			skusToday = append(skusToday, row)
		}
		if key == yesterdayKey {
			skusYesterdayByProduct[row.OzonProductID] = row
		}
		if !row.MetricDate.Valid {
			continue
		}
		d := row.MetricDate.Time.UTC().Truncate(24 * time.Hour)
		f7 := from7.UTC().Truncate(24 * time.Hour)
		a0 := asOf.UTC().Truncate(24 * time.Hour)
		if d.Before(f7) || d.After(a0) {
			continue
		}
		dayIndex := int(d.Sub(f7) / (24 * time.Hour))
		if dayIndex < 0 || dayIndex > 6 {
			continue
		}
		a := agg7d[row.OzonProductID]
		if a == nil {
			a = &sku7dAgg{}
			agg7d[row.OzonProductID] = a
		}
		rev := numericToFloat(row.Revenue)
		ord := int64(row.OrdersCount)
		a.revenue += rev
		a.orders += ord
		if rev > 0 || ord > 0 {
			a.activeDayMask |= 1 << dayIndex
		}
	}

	skuItems := make([]DashboardSKUMetricsItem, 0, len(skusToday))
	var maxUpdatedAt pgtype.Timestamptz
	for _, todayRow := range skusToday {
		yesterdayRow := skusYesterdayByProduct[todayRow.OzonProductID]
		todayRevenue := numericToFloat(todayRow.Revenue)
		yesterdayRevenue := numericToFloat(yesterdayRow.Revenue)
		revenueDelta := round2(todayRevenue - yesterdayRevenue)
		share := safeRatio(todayRevenue, revenueToday)

		var rev7 float64
		var ord7 int64
		var activeDays int
		if a := agg7d[todayRow.OzonProductID]; a != nil {
			rev7 = a.revenue
			ord7 = a.orders
			activeDays = bits.OnesCount8(a.activeDayMask)
		}
		item := DashboardSKUMetricsItem{
			OzonProductID:               todayRow.OzonProductID,
			OfferID:                     textPtr(todayRow.OfferID),
			SKU:                         int8Ptr(todayRow.Sku),
			ProductName:                 textPtr(todayRow.ProductName),
			Revenue:                     todayRevenue,
			OrdersCount:                 todayRow.OrdersCount,
			StockAvailable:              todayRow.StockAvailable,
			DaysOfCover:                 numericToFloatPtr(todayRow.DaysOfCover),
			RevenueDeltaDayToDay:        revenueDelta,
			OrdersDeltaDayToDay:         todayRow.OrdersCount - yesterdayRow.OrdersCount,
			ShareOfRevenue:              share,
			ContributionToRevenueChange: revenueDelta,
			RevenueLast7Days:            rev7,
			OrdersLast7Days:             ord7,
			ActiveSalesDays7D:           activeDays,
		}
		if units, ok := int32FromPgInt4(todayRow.UnitsSold); ok {
			item.UnitsSold = &units
		}
		enrichSKUMetricsItem(&item, enrichCtx, yesterdayRow.OrdersCount)
		skuItems = append(skuItems, item)
		maxUpdatedAt = maxTimestamptz(maxUpdatedAt, todayRow.UpdatedAt)
	}

	sort.Slice(skuItems, func(i, j int) bool {
		if skuItems[i].Revenue == skuItems[j].Revenue {
			return skuItems[i].OzonProductID < skuItems[j].OzonProductID
		}
		return skuItems[i].Revenue > skuItems[j].Revenue
	})
	return skuItems, maxUpdatedAt
}
