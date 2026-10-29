package dashboardfacts

import (
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// buildSKUFactsIndex assembles the per-SKU factual index for metricDate,
// joining each metrics.SKUs entry (today's row) with its metric_date-6..
// metric_date sparkline built directly from snapshot.SKUMetrics.
//
// Products that appear in the 7-day SKUMetrics window but not in today's
// metrics.SKUs (e.g. no row on metric_date) are still indexed so AI-selected
// pack SKUs retain sparkline_7d / identity facts and are not dropped at merge.
func buildSKUFactsIndex(snap *analysisdata.AnalysisDataSnapshot, metrics analytics.DashboardMetricsDTO, metricDate time.Time) SKUFactsIndex {
	from := metricDate.AddDate(0, 0, -6)
	sparklines := buildSKUSparklines(snap.SKUMetrics, from, metricDate)
	prevFrom := metricDate.AddDate(0, 0, -13)
	prevTo := metricDate.AddDate(0, 0, -7)
	prevSparklines := buildSKUSparklines(snap.SKUMetrics, prevFrom, prevTo)
	latestMeta := latestSKUMetricByProduct(snap.SKUMetrics, from, metricDate)

	byID := make(map[string]SKUFactRecord, len(metrics.SKUs)+len(sparklines))
	for _, sku := range metrics.SKUs {
		spark := sparklines[sku.OzonProductID]
		rev7, ord7 := sku.RevenueLast7Days, sku.OrdersLast7Days
		// Prefer analytics 7d aggregates when present; otherwise sum sparkline.
		if rev7 == 0 && ord7 == 0 {
			if sRev, sOrd := sumSparklineRevenueOrders7d(spark); sRev > 0 || sOrd > 0 {
				rev7, ord7 = sRev, sOrd
			}
		}
		prevRev, prevOrd := sumSparklineRevenueOrders7d(prevSparklines[sku.OzonProductID])
		bucket := analytics.StockCoverageBucketKey(sku)
		rec := SKUFactRecord{
			OzonProductID:        sku.OzonProductID,
			OfferID:              sku.OfferID,
			SKU:                  sku.SKU,
			ProductName:          sku.ProductName,
			Revenue:              sku.Revenue,
			Orders:               sku.OrdersCount,
			Revenue7d:            rev7,
			Orders7d:             ord7,
			RevenuePrev7d:        prevRev,
			OrdersPrev7d:         prevOrd,
			OrdersDeltaDayToDay:  sku.OrdersDeltaDayToDay,
			RevenueDeltaDayToDay: sku.RevenueDeltaDayToDay,
			StockAvailable:       sku.StockAvailable,
			DaysOfCover:          sku.DaysOfCover,
			CoverageBucket:       bucket,
			DaysWithoutSales:     sku.DaysWithoutSales,
			Sparkline7D:          spark,
		}
		byID[fmt.Sprintf("%d", sku.OzonProductID)] = rec
	}
	for productID, spark := range sparklines {
		key := fmt.Sprintf("%d", productID)
		if _, exists := byID[key]; exists {
			continue
		}
		observed := 0
		for _, pt := range spark {
			if pt.HasData {
				observed++
			}
		}
		if observed == 0 {
			continue
		}
		meta := latestMeta[productID]
		rev7, ord7 := sumSparklineRevenueOrders7d(spark)
		prevRev, prevOrd := sumSparklineRevenueOrders7d(prevSparklines[productID])
		rec := SKUFactRecord{
			OzonProductID: productID,
			Revenue7d:     rev7,
			Orders7d:      ord7,
			RevenuePrev7d: prevRev,
			OrdersPrev7d:  prevOrd,
			Sparkline7D:   spark,
		}
		if meta.OfferID != "" {
			offer := meta.OfferID
			rec.OfferID = &offer
		}
		if meta.SKU != 0 {
			sku := meta.SKU
			rec.SKU = &sku
		}
		if meta.ProductName != "" {
			name := meta.ProductName
			rec.ProductName = &name
		}
		rec.Revenue = meta.Revenue
		rec.Orders = meta.Orders
		rec.StockAvailable = meta.StockAvailable
		if meta.DaysOfCover != nil {
			rec.DaysOfCover = meta.DaysOfCover
		}
		byID[key] = rec
	}
	return SKUFactsIndex{ByOzonProductID: byID}
}

// sumSparklineRevenueOrders7d sums observed sparkline points. Missing days
// (HasData=false) contribute nothing — they are never treated as zero sales.
func sumSparklineRevenueOrders7d(spark []SparklinePoint) (revenue float64, orders int64) {
	for _, pt := range spark {
		if !pt.HasData {
			continue
		}
		if pt.Revenue != nil {
			revenue += *pt.Revenue
		}
		if pt.Value != nil {
			orders += int64(*pt.Value)
		}
	}
	return revenue, orders
}

type skuMetricMeta struct {
	OfferID        string
	SKU            int64
	ProductName    string
	Revenue        float64
	Orders         int32
	StockAvailable int32
	DaysOfCover    *float64
	MetricDate     time.Time
}

// latestSKUMetricByProduct returns, per ozon_product_id, identity/metrics from
// the newest DailySkuMetric row inside [from, to].
func latestSKUMetricByProduct(rows []dbgen.DailySkuMetric, from, to time.Time) map[int64]skuMetricMeta {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	out := make(map[int64]skuMetricMeta)
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		prev, ok := out[row.OzonProductID]
		if ok && !d.After(prev.MetricDate) {
			continue
		}
		meta := skuMetricMeta{
			OfferID:        strings.TrimSpace(pgTextString(row.OfferID)),
			SKU:            pgInt8Value(row.Sku),
			ProductName:    strings.TrimSpace(pgTextString(row.ProductName)),
			Revenue:        pgNumericToFloat(row.Revenue),
			Orders:         row.OrdersCount,
			StockAvailable: row.StockAvailable,
			DaysOfCover:    pgNumericToFloatPtr(row.DaysOfCover),
			MetricDate:     d,
		}
		out[row.OzonProductID] = meta
	}
	return out
}

type skuSparklineDayMetrics struct {
	orders   float64
	revenue  float64
	stock    int32
	hasStock bool
}

// buildSKUSparklines returns, per ozon_product_id, a calendar sparkline
// covering [from, to] built from snapshot.SKUMetrics rows. A day with no
// daily_sku_metrics row for that product stays HasData=false with
// Value/Revenue left nil — missing days are never coerced to zero.
func buildSKUSparklines(rows []dbgen.DailySkuMetric, from, to time.Time) map[int64][]SparklinePoint {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)

	byProductDate := make(map[int64]map[string]skuSparklineDayMetrics)
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		key := d.Format("2006-01-02")
		if byProductDate[row.OzonProductID] == nil {
			byProductDate[row.OzonProductID] = make(map[string]skuSparklineDayMetrics)
		}
		byProductDate[row.OzonProductID][key] = skuSparklineDayMetrics{
			orders:   float64(row.OrdersCount),
			revenue:  pgNumericToFloat(row.Revenue),
			stock:    row.StockAvailable,
			hasStock: true,
		}
	}

	out := make(map[int64][]SparklinePoint, len(byProductDate))
	for productID, byDate := range byProductDate {
		out[productID] = buildSKUCalendarSparkline(fromDate, toDate, byDate)
	}
	return out
}

func buildSKUCalendarSparkline(from, to time.Time, byDate map[string]skuSparklineDayMetrics) []SparklinePoint {
	points := make([]SparklinePoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if m, ok := byDate[key]; ok {
			pt.HasData = true
			orders := m.orders
			revenue := m.revenue
			pt.Value = &orders
			pt.Revenue = &revenue
			if m.hasStock {
				stock := m.stock
				pt.Stock = &stock
			}
		}
		points = append(points, pt)
	}
	return points
}

// countSKUsWithSparkline counts SKUFactsIndex entries whose sparkline has
// at least 2 observed (HasData) days — a signal that a real day-to-day
// trend (not just today) is available for that SKU.
func countSKUsWithSparkline(idx SKUFactsIndex) int {
	n := 0
	for _, rec := range idx.ByOzonProductID {
		observed := 0
		for _, pt := range rec.Sparkline7D {
			if pt.HasData {
				observed++
			}
		}
		if observed >= 2 {
			n++
		}
	}
	return n
}
