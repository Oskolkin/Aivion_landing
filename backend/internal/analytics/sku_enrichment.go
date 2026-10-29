package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type skuEnrichmentContext struct {
	accountOrdersForShare  int32
	accountRevenueForShare float64
	velocityRows           []dbgen.DailySkuMetric
	productPricing         map[int64]productPricingRow
	stockAnalyticsBySKU    map[int64]stockAnalyticsAgg
	asOf                   time.Time
}

type productPricingRow struct {
	oldPrice        float64
	referencePrice  float64
	hasOldPrice     bool
	hasCurrentPrice bool
}

type stockAnalyticsAgg struct {
	available        int32
	excess           int32
	ads              *float64
	idc              *int32
	turnoverGrade    *string
	daysWithoutSales *int32
}

func loadSKUEnrichmentContext(
	ctx context.Context,
	queries *dbgen.Queries,
	sellerAccountID int64,
	asOf time.Time,
	todayAccount dbgen.DailyAccountMetric,
) (skuEnrichmentContext, error) {
	velocityFrom := asOf.AddDate(0, 0, -13)
	velocityRows, err := queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(velocityFrom),
		MetricDate_2:    dateValue(asOf),
	})
	if err != nil {
		return skuEnrichmentContext{}, fmt.Errorf("list sku metrics for velocity: %w", err)
	}

	pricingRows, err := queries.ListProductPricingBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return skuEnrichmentContext{}, fmt.Errorf("list product pricing: %w", err)
	}
	pricingByProduct := make(map[int64]productPricingRow, len(pricingRows))
	skuToProduct := make(map[int64]int64)
	for _, row := range pricingRows {
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

	stockBySKU := make(map[int64]stockAnalyticsAgg)
	snapshotDate := dateValue(asOf)
	stockRows, err := queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		SnapshotDate:    snapshotDate,
	})
	if err != nil {
		return skuEnrichmentContext{}, fmt.Errorf("list stock analytics snapshots: %w", err)
	}
	if len(stockRows) == 0 {
		latest, err := queries.GetLatestStockAnalyticsSnapshotDateBySeller(ctx, sellerAccountID)
		if err != nil {
			return skuEnrichmentContext{}, fmt.Errorf("get latest stock analytics date: %w", err)
		}
		if latest.Valid {
			stockRows, err = queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
				SellerAccountID: sellerAccountID,
				SnapshotDate:    latest,
			})
			if err != nil {
				return skuEnrichmentContext{}, fmt.Errorf("list stock analytics on latest date: %w", err)
			}
		}
	}

	aggregated := aggregateStockAnalyticsBySKU(stockRows)
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
	}, nil
}

func sumSKUFieldInRange(rows []dbgen.DailySkuMetric, productID int64, from, to time.Time, pick func(dbgen.DailySkuMetric) int32) int64 {
	fromDate := normalizeDate(from)
	toDate := normalizeDate(to)
	var sum int64
	for _, row := range rows {
		if row.OzonProductID != productID {
			continue
		}
		d := normalizeDate(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		sum += int64(pick(row))
	}
	return sum
}

// sumSKUUnitsSoldInRange sums units_sold only on days with a stored value (non-null after rebuild).
// Returns ok=false when any calendar day in [from,to] has a metric row but null units_sold (pre-rebuild history).
func sumSKUUnitsSoldInRange(rows []dbgen.DailySkuMetric, productID int64, from, to time.Time) (sum int64, ok bool) {
	fromDate := normalizeDate(from)
	toDate := normalizeDate(to)
	byDate := make(map[string]struct {
		hasRow   bool
		hasUnits bool
		units    int64
	})
	for d := fromDate; !d.After(toDate); d = d.AddDate(0, 0, 1) {
		byDate[d.Format("2006-01-02")] = struct {
			hasRow   bool
			hasUnits bool
			units    int64
		}{}
	}
	for _, row := range rows {
		if row.OzonProductID != productID {
			continue
		}
		key := normalizeDate(row.MetricDate.Time).Format("2006-01-02")
		entry, exists := byDate[key]
		if !exists {
			continue
		}
		entry.hasRow = true
		if units, valid := int32FromPgInt4(row.UnitsSold); valid {
			entry.hasUnits = true
			entry.units = int64(units)
		}
		byDate[key] = entry
	}
	var total int64
	for _, entry := range byDate {
		if entry.hasRow && !entry.hasUnits {
			return 0, false
		}
		if entry.hasUnits {
			total += entry.units
		}
	}
	return total, true
}

func enrichSKUMetricsItem(item *DashboardSKUMetricsItem, ectx skuEnrichmentContext, yesterdayOrders int32) {
	item.SKUOrdersShare = RatioOrNil(float64(item.OrdersCount), float64(ectx.accountOrdersForShare))
	item.SKURevenueShare = RatioOrNil(item.Revenue, ectx.accountRevenueForShare)

	orders7 := sumSKUFieldInRange(ectx.velocityRows, item.OzonProductID, ectx.asOf.AddDate(0, 0, -6), ectx.asOf, func(r dbgen.DailySkuMetric) int32 { return r.OrdersCount })
	orders14 := sumSKUFieldInRange(ectx.velocityRows, item.OzonProductID, ectx.asOf.AddDate(0, 0, -13), ectx.asOf, func(r dbgen.DailySkuMetric) int32 { return r.OrdersCount })
	item.SKUSalesVelocityOrders7D = AverageDailyVelocity(orders7, 7)
	item.SKUSalesVelocityOrders14D = AverageDailyVelocity(orders14, 14)
	item.SKUOrdersDeltaPercent = PercentDeltaOrNil(float64(item.OrdersCount), float64(yesterdayOrders))

	if item.UnitsSold != nil {
		if units7, ok := sumSKUUnitsSoldInRange(ectx.velocityRows, item.OzonProductID, ectx.asOf.AddDate(0, 0, -6), ectx.asOf); ok {
			item.SKUSalesVelocityUnits7D = AverageDailyVelocity(units7, 7)
		}
		if units14, ok := sumSKUUnitsSoldInRange(ectx.velocityRows, item.OzonProductID, ectx.asOf.AddDate(0, 0, -13), ectx.asOf); ok {
			item.SKUSalesVelocityUnits14D = AverageDailyVelocity(units14, 14)
		}
	}

	if p, ok := ectx.productPricing[item.OzonProductID]; ok && p.hasOldPrice && p.hasCurrentPrice {
		item.DiscountDepth = DiscountDepth(p.oldPrice, p.referencePrice)
	}

	var excessShare *float64
	if stock, ok := ectx.stockAnalyticsBySKU[item.OzonProductID]; ok {
		if stock.idc != nil {
			v := *stock.idc
			item.OzonIDC = &v
		}
		if stock.ads != nil {
			v := *stock.ads
			item.OzonADS = &v
		}
		if stock.turnoverGrade != nil {
			g := *stock.turnoverGrade
			item.StockTurnoverGrade = &g
		}
		if stock.daysWithoutSales != nil {
			v := *stock.daysWithoutSales
			item.DaysWithoutSales = &v
		}
		excessShare = ExcessStockShare(stock.excess, stock.available)
		item.ExcessStockShare = excessShare
	}

	bucket := ComputeStockStatusBucket(item.StockAvailable, item.DaysOfCover, item.OzonIDC, item.StockTurnoverGrade, excessShare, item.DaysWithoutSales)
	item.StockStatusBucket = &bucket.Bucket
	item.StockStatusBucketSource = &bucket.Source
}
