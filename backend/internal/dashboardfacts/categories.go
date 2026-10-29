package dashboardfacts

import (
	"math"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

// categoryRef identifies a description category by id+name.
type categoryRef struct {
	ID   int64
	Name string
}

// buildProductCategoryMap joins snapshot.Products (ozon_product_id ->
// description_category_id) with snapshot.Categories (id -> name) into a
// per-product category lookup. Products with no (or unknown) category are
// omitted.
func buildProductCategoryMap(snap *analysisdata.AnalysisDataSnapshot) map[int64]categoryRef {
	names := make(map[int64]string, len(snap.Categories))
	for _, c := range snap.Categories {
		names[c.DescriptionCategoryID] = c.CategoryName
	}
	out := make(map[int64]categoryRef, len(snap.Products))
	for _, p := range snap.Products {
		if !p.DescriptionCategoryID.Valid {
			continue
		}
		id := p.DescriptionCategoryID.Int64
		out[p.OzonProductID] = categoryRef{ID: id, Name: names[id]}
	}
	return out
}

// countDistinctSellerCategories reports how many distinct categories the
// seller's product catalog references, independent of whether those
// categories carry same-day SKU metrics. Feeds DataCoverage.CategoryCount.
func countDistinctSellerCategories(catByProduct map[int64]categoryRef) int {
	seen := make(map[int64]struct{}, len(catByProduct))
	for _, ref := range catByProduct {
		seen[ref.ID] = struct{}{}
	}
	return len(seen)
}

type categoryAgg struct {
	ref               categoryRef
	skuIDs            map[int64]struct{}
	revenue           float64
	orders            int32
	previousRevenue   float64
	previousOrders    int32
	stockRiskSKUCount int
}

// buildCategoryFacts aggregates today's (metrics.SKUs) and yesterday's
// (snapshot.SKUMetrics) SKU rows per category, using
// buildProductCategoryMap to resolve each SKU's category. Categories with
// no same-day SKU metrics are omitted — nothing factual to report for
// metricDate, even if the catalog has products in that category.
func buildCategoryFacts(snap *analysisdata.AnalysisDataSnapshot, metrics analytics.DashboardMetricsDTO, metricDate time.Time, catByProduct map[int64]categoryRef) []CategoryFacts {
	if len(catByProduct) == 0 {
		return nil
	}

	byCategory := make(map[int64]*categoryAgg)
	ensure := func(ref categoryRef) *categoryAgg {
		a, ok := byCategory[ref.ID]
		if !ok {
			a = &categoryAgg{ref: ref, skuIDs: make(map[int64]struct{})}
			byCategory[ref.ID] = a
		}
		return a
	}

	var totalRevenue float64
	var totalOrders int32
	for _, sku := range metrics.SKUs {
		ref, ok := catByProduct[sku.OzonProductID]
		if !ok {
			continue
		}
		a := ensure(ref)
		a.skuIDs[sku.OzonProductID] = struct{}{}
		a.revenue += sku.Revenue
		a.orders += sku.OrdersCount
		totalRevenue += sku.Revenue
		totalOrders += sku.OrdersCount
		switch analytics.StockCoverageBucketKey(sku) {
		case "out_of_stock", "critical_0_3", "shortage_4_7", "surplus":
			a.stockRiskSKUCount++
		}
	}
	if len(byCategory) == 0 {
		return nil
	}

	yesterdayKey := metricDate.AddDate(0, 0, -1).Format("2006-01-02")
	for _, row := range snap.SKUMetrics {
		if !row.MetricDate.Valid || row.MetricDate.Time.Format("2006-01-02") != yesterdayKey {
			continue
		}
		ref, ok := catByProduct[row.OzonProductID]
		if !ok {
			continue
		}
		a, ok := byCategory[ref.ID]
		if !ok {
			continue
		}
		a.previousRevenue += pgNumericToFloat(row.Revenue)
		a.previousOrders += row.OrdersCount
	}

	out := make([]CategoryFacts, 0, len(byCategory))
	for _, a := range byCategory {
		var revenueShare, ordersShare *float64
		if totalRevenue > 0 {
			v := round2(a.revenue / totalRevenue * 100)
			revenueShare = &v
		}
		if totalOrders > 0 {
			v := round2(float64(a.orders) / float64(totalOrders) * 100)
			ordersShare = &v
		}
		out = append(out, CategoryFacts{
			CategoryID:           a.ref.ID,
			CategoryName:         a.ref.Name,
			SKUCount:             len(a.skuIDs),
			Revenue:              round2(a.revenue),
			Orders:               a.orders,
			RevenueShare:         revenueShare,
			OrdersShare:          ordersShare,
			PreviousRevenue:      round2(a.previousRevenue),
			PreviousOrders:       a.previousOrders,
			RevenueDeltaDayToDay: round2(a.revenue - a.previousRevenue),
			OrdersDeltaDayToDay:  a.orders - a.previousOrders,
			StockRiskSKUCount:    a.stockRiskSKUCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Revenue != out[j].Revenue {
			return out[i].Revenue > out[j].Revenue
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
