package pack

import (
	"sort"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// skuFacts holds every derived (non-CSV-formatted) fact needed for both
// sku_summary.csv rows and account-level SKU health counters.
type skuFacts struct {
	SKU string

	Revenue7D, RevenuePrev7D  float64
	Orders7D, OrdersPrev7D    int64
	UnitsSold7D, UnitsSold30D int64

	HasStockData   bool
	AvailableStock int64
	IncomingStock  *int64

	AvgDailySales7D  *float64
	AvgDailySales30D *float64
	Coverage7D       *float64
	Coverage30D      *float64

	// TurnoverGrade is the raw Ozon signal (never used as final stock_status).
	TurnoverGrade string
	StockStatus   string

	HasDaysWithoutSales bool
	DaysWithoutSales    int64

	StockConflictFlags []string
}

// computeSKUFacts derives windowed sums and stock health for one SKU.
func computeSKUFacts(idx *Indexes, sku string, periods Periods, metricDate string) skuFacts {
	f := skuFacts{SKU: sku}
	byDate := idx.SKUMetricsByDate[sku]

	sumWindow := func(r analysisdata.DateRange) (revenue float64, orders, units int64) {
		for _, d := range dateRangeInclusive(r.From, r.To) {
			day, ok := byDate[d]
			if !ok {
				continue
			}
			revenue += day.Revenue
			orders += day.Orders
			units += day.UnitsSold
		}
		return
	}

	f.Revenue7D, f.Orders7D, f.UnitsSold7D = sumWindow(periods.Current7D)
	f.RevenuePrev7D, f.OrdersPrev7D, _ = sumWindow(periods.Previous7D)
	_, ordersTrend, unitsTrend := sumWindow(periods.Trend)
	f.UnitsSold30D = unitsTrend

	f.AvgDailySales7D = analytics.AverageDailyVelocity(f.Orders7D, 7)
	f.AvgDailySales30D = analytics.AverageDailyVelocity(ordersTrend, contract.AnalysisPackDailyHistoryDaysV1)

	stock, hasStock := skuAvailableStock(idx, sku, metricDate)
	f.HasStockData = hasStock
	f.AvailableStock = stock

	var rawOzonDays *int32
	if agg, ok := idx.StockBySKU[sku]; ok && agg.HasData {
		f.IncomingStock = ptrInt64(agg.TransitStockCount)
		f.TurnoverGrade = agg.TurnoverGrade
		if agg.HasDaysWithoutSales {
			v := agg.DaysWithoutSales
			rawOzonDays = &v
		}
	}

	f.Coverage7D = CoverageDays(f.AvailableStock, f.AvgDailySales7D)
	f.Coverage30D = CoverageDays(f.AvailableStock, f.AvgDailySales30D)

	// Canonical days_without_sales always comes from daily history relative to metric_date.
	f.DaysWithoutSales = daysWithoutSalesFromMetrics(byDate, metricDate, periods.Trend, f.Orders7D)
	f.HasDaysWithoutSales = f.HasStockData || len(byDate) > 0
	if rawOzonDays != nil && int64(*rawOzonDays) != f.DaysWithoutSales {
		f.StockConflictFlags = append(f.StockConflictFlags, "ozon_days_without_sales_vs_calculated")
	}

	coverage := f.Coverage7D
	if coverage == nil {
		coverage = f.Coverage30D
	}
	dws := f.DaysWithoutSales
	status := stockpolicy.ComputeCanonicalStockStatus(stockpolicy.Input{
		AvailableStock:    f.AvailableStock,
		HasStockData:      f.HasStockData,
		CoverageDays:      coverage,
		Orders7D:          f.Orders7D,
		DaysWithoutSales:  &dws,
		HasSalesHistory:   len(byDate) > 0,
		OzonTurnoverGrade: f.TurnoverGrade,
	})
	f.StockStatus = status.Status
	f.StockConflictFlags = append(f.StockConflictFlags, status.ConflictFlags...)
	return f
}

// skuAvailableStock resolves the current-day available stock, preferring the
// stock_analytics_snapshots aggregate and falling back to daily_sku_metrics.
func skuAvailableStock(idx *Indexes, sku, metricDate string) (int64, bool) {
	if agg, ok := idx.StockBySKU[sku]; ok && agg.HasData {
		return agg.AvailableStockCount, true
	}
	if byDate, ok := idx.SKUMetricsByDate[sku]; ok {
		if day, ok := byDate[metricDate]; ok && day.HasStock {
			return int64(day.StockAvailable), true
		}
	}
	return 0, false
}

// daysWithoutSalesFromMetrics counts consecutive observed zero-order days walking
// back from metricDate. A missing calendar day is a coverage gap and ends the streak
// (absence is not treated as a confirmed zero-sales day).
// When orders7d > 0 the result is clamped to ≤ 6.
func daysWithoutSalesFromMetrics(byDate map[string]skuMetricDay, metricDate string, trend analysisdata.DateRange, orders7d int64) int64 {
	dates := dateRangeInclusive(trend.From, trend.To)
	ordersByDate := make(map[string]int64, len(byDate))
	observedNewest := make([]string, 0, len(dates))
	for i := len(dates) - 1; i >= 0; i-- {
		d := dates[i]
		if d > metricDate {
			continue
		}
		day, ok := byDate[d]
		if !ok {
			// Sparse gap ends the streak; do not treat missing days as zero sales.
			break
		}
		ordersByDate[d] = day.Orders
		observedNewest = append(observedNewest, d)
	}
	return stockpolicy.DaysWithoutSalesFromObserved(ordersByDate, observedNewest, orders7d)
}

func ptrInt64(v int64) *int64 {
	return &v
}

// ComputeAllSKUFacts derives skuFacts for every SKU in the universe once, so
// sku_summary.csv and account-level SKU health counters stay consistent.
func ComputeAllSKUFacts(idx *Indexes, periods Periods, metricDate string) map[string]skuFacts {
	facts := make(map[string]skuFacts, len(idx.SKUUniverse))
	for _, sku := range idx.SKUUniverse {
		facts[sku] = computeSKUFacts(idx, sku, periods, metricDate)
	}
	return facts
}

// BuildSKUSummaryTable renders sku_summary.csv rows in SKU-ascending order.
func BuildSKUSummaryTable(idx *Indexes, facts map[string]skuFacts, limitations *limitationCollector) csvTable {
	rows := make([][]string, 0, len(idx.SKUUniverse))

	var totalRevenue7D float64
	for _, sku := range idx.SKUUniverse {
		totalRevenue7D += facts[sku].Revenue7D
	}

	sawSKUData := len(idx.SKUUniverse) > 0
	if sawSKUData {
		limitations.Add(LimitationSKUFunnelUnavailable)
		limitations.Add(LimitationSKUOrderQualityUnavailable)
		limitations.Add(LimitationSKUReservedStockUnknown)
		limitations.Add(LimitationSKUAdSpendNotAttributed)
		limitations.Add(LimitationContentScoreUnavailable)
		limitations.Add(LimitationAttributeCompletionUnknown)
	}

	for _, sku := range idx.SKUUniverse {
		f := facts[sku]
		m := idx.SKUMeta[sku]
		category, categoryName := "", ""
		if m.CategoryID != "" {
			category = "category:" + m.CategoryID
			if cat, ok := idx.CategoryByID[atoi64(m.CategoryID)]; ok {
				categoryName = cat.CategoryName
			}
		}

		ordersAbs := DeltaAbs(float64(f.Orders7D), float64(f.OrdersPrev7D))
		ordersPct := DeltaPct(float64(f.Orders7D), float64(f.OrdersPrev7D))
		revAbs := DeltaAbs(f.Revenue7D, f.RevenuePrev7D)
		revPct := DeltaPct(f.Revenue7D, f.RevenuePrev7D)
		revShare := RevenueSharePct(f.Revenue7D, totalRevenue7D)

		var incoming string
		if f.IncomingStock != nil {
			incoming = FormatIntCSVPtr(f.IncomingStock)
		}

		var daysWithoutSales string
		if f.HasDaysWithoutSales {
			daysWithoutSales = strconv.FormatInt(f.DaysWithoutSales, 10)
		}

		var availableStock string
		if f.HasStockData {
			availableStock = strconv.FormatInt(f.AvailableStock, 10)
		}

		price, oldPrice, marketPriceIndex := skuPriceFacts(idx, sku)
		enrichmentStatus, hasWarnings, imageCount := skuEnrichmentFacts(idx, sku)

		row := []string{
			"sku:" + sku,
			sku,
			m.OfferID,
			m.ProductID,
			m.Title,
			category,
			categoryName,
			FormatIntCSV(f.Orders7D),
			FormatIntCSV(f.OrdersPrev7D),
			FormatFloatCSV(&ordersAbs),
			FormatPctCSV(ordersPct),
			FormatMoneyCSV(f.Revenue7D),
			FormatMoneyCSV(f.RevenuePrev7D),
			FormatMoneyCSV(revAbs),
			FormatPctCSV(revPct),
			FormatPctCSV(revShare),
			"", // delivered_7d
			"", // cancelled_7d
			"", // returns_7d
			"", // impressions_7d
			"", // views_7d
			"", // add_to_cart_7d
			"", // funnel_orders_7d
			"", // impression_to_view_pct
			"", // view_to_cart_pct
			"", // cart_to_order_pct
			"", // order_to_delivered_pct
			availableStock,
			"", // reserved_stock_units
			incoming,
			FormatFloatCSV(f.AvgDailySales7D),
			FormatFloatCSV(f.AvgDailySales30D),
			FormatFloatCSV(f.Coverage7D),
			FormatFloatCSV(f.Coverage30D),
			f.StockStatus,
			daysWithoutSales,
			"", // ad_spend_7d
			"", // ad_revenue_7d
			"", // ad_orders_7d
			"", // drr_pct
			FormatFloatCSV(price),
			FormatFloatCSV(oldPrice),
			FormatFloatCSV(marketPriceIndex),
			enrichmentStatus,
			"", // content_score
			FormatBoolCSVPtr(hasWarnings),
			FormatIntCSVPtr(imageCount),
			"", // attribute_completion_pct
		}
		rows = append(rows, row)
	}

	return csvTable{Name: "sku_summary.csv", Columns: contract.SKUSummaryColumnsV1, Rows: rows}
}

// BuildSKUDailyTable renders sku_daily.csv rows sorted by (sku, date) ascending.
// Only dates with an observed daily_sku_metrics row are emitted (sparse daily series).
func BuildSKUDailyTable(idx *Indexes) csvTable {
	var rows [][]string
	for _, sku := range idx.SKUUniverse {
		byDate := idx.SKUMetricsByDate[sku]
		if len(byDate) == 0 {
			continue
		}
		dates := make([]string, 0, len(byDate))
		for d := range byDate {
			dates = append(dates, d)
		}
		sort.Strings(dates)
		for _, d := range dates {
			day := byDate[d]
			rows = append(rows, []string{
				"sku:" + sku + ":" + d,
				sku,
				d,
				FormatIntCSV(day.Orders),
				FormatMoneyCSV(day.Revenue),
				"", // delivered
				"", // cancelled
				"", // returns
				"", // impressions
				"", // views
				"", // add_to_cart
				strconv.FormatInt(int64(day.StockAvailable), 10),
				"", // reserved_stock
				"", // incoming_stock
				"", // ad_spend
				"", // ad_revenue
				"", // ad_orders
			})
		}
	}
	return csvTable{Name: "sku_daily.csv", Columns: contract.SKUDailyColumnsV1, Rows: rows}
}

func skuPriceFacts(idx *Indexes, sku string) (price, oldPrice, marketPriceIndex *float64) {
	if snap, ok := idx.ProductInfoBySKU[sku]; ok {
		price = numericToFloatPtr(snap.Price)
		oldPrice = numericToFloatPtr(snap.OldPrice)
		if v := numericToFloatPtr(snap.OzonPriceIndexValue); v != nil {
			marketPriceIndex = v
		} else {
			marketPriceIndex = numericToFloatPtr(snap.ExternalPriceIndexValue)
		}
		return
	}
	if p, ok := idx.ProductBySKU[sku]; ok {
		price = numericToFloatPtr(p.ReferencePrice)
		oldPrice = numericToFloatPtr(p.OldPrice)
	}
	return
}

func skuEnrichmentFacts(idx *Indexes, sku string) (status string, hasWarnings *bool, imageCount *int64) {
	snap, ok := idx.ProductInfoBySKU[sku]
	if !ok {
		return "not_available", nil, nil
	}
	warnings := len(parseJSONElements(snap.ErrorsJson)) > 0
	hasWarnings = &warnings
	if n := len(parseJSONElements(snap.ImagesJson)); n > 0 || snap.PrimaryImage.Valid {
		count := int64(n)
		if count == 0 && snap.PrimaryImage.Valid {
			count = 1
		}
		imageCount = &count
	}
	return "available", hasWarnings, imageCount
}

func atoi64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}
