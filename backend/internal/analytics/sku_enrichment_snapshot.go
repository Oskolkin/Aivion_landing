package analytics

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func buildSKUEnrichmentContextFromSnapshot(
	snap *analysisdata.AnalysisDataSnapshot,
	asOf time.Time,
	todayAccount dbgen.DailyAccountMetric,
) skuEnrichmentContext {
	velocityFrom := asOf.AddDate(0, 0, -13)
	velocityRows := filterSKUMetricsInRange(snap.SKUMetrics, velocityFrom, asOf)

	pricingByProduct := make(map[int64]productPricingRow, len(snap.Products))
	skuToProduct := make(map[int64]int64, len(snap.Products))
	for _, row := range snap.Products {
		p := productPricingRow{}
		if v := numericToFloatPtr(row.OldPrice); v != nil && *v > 0 {
			p.oldPrice = *v
			p.hasOldPrice = true
		}
		if v := numericToFloatPtr(row.ReferencePrice); v != nil && *v > 0 {
			p.referencePrice = *v
			p.hasCurrentPrice = true
		}
		pricingByProduct[row.OzonProductID] = p
		if row.Sku.Valid {
			skuToProduct[row.Sku.Int64] = row.OzonProductID
		}
	}

	stockRows := selectStockSnapshotsForDate(snap.StockSnapshots, asOf)
	aggregated := aggregateStockAnalyticsBySKU(stockRows)
	stockBySKU := make(map[int64]stockAnalyticsAgg, len(aggregated))
	for sku, row := range aggregated {
		ozonProductID, ok := skuToProduct[sku]
		if !ok {
			continue
		}
		stockBySKU[ozonProductID] = stockAnalyticsAgg{
			available:        row.available,
			excess:           row.excess,
			ads:              row.ads,
			idc:              row.idc,
			turnoverGrade:    row.turnoverGrade,
			daysWithoutSales: row.daysWithoutSales,
		}
	}

	return skuEnrichmentContext{
		accountOrdersForShare:  todayAccount.OrdersCount,
		accountRevenueForShare: numericToFloat(todayAccount.Revenue),
		velocityRows:           velocityRows,
		productPricing:         pricingByProduct,
		stockAnalyticsBySKU:    stockBySKU,
		asOf:                   asOf,
	}
}

func selectStockSnapshotsForDate(rows []dbgen.StockAnalyticsSnapshot, asOf time.Time) []dbgen.StockAnalyticsSnapshot {
	targetKey := asOf.Format("2006-01-02")
	out := make([]dbgen.StockAnalyticsSnapshot, 0)
	for _, row := range rows {
		if row.SnapshotDate.Valid && row.SnapshotDate.Time.Format("2006-01-02") == targetKey {
			out = append(out, row)
		}
	}
	if len(out) > 0 {
		return out
	}
	latestKey := ""
	for _, row := range rows {
		if !row.SnapshotDate.Valid {
			continue
		}
		key := row.SnapshotDate.Time.Format("2006-01-02")
		if key > latestKey {
			latestKey = key
		}
	}
	if latestKey == "" {
		return nil
	}
	for _, row := range rows {
		if row.SnapshotDate.Valid && row.SnapshotDate.Time.Format("2006-01-02") == latestKey {
			out = append(out, row)
		}
	}
	return out
}
