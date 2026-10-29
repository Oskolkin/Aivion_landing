package analytics

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"

// StockCoverageBucketKey maps SKU metrics to one exclusive coverage segment.
func StockCoverageBucketKey(sku DashboardSKUMetricsItem) string {
	if sku.StockStatusBucket != nil {
		return stockpolicy.MapToCoverageSegment(*sku.StockStatusBucket)
	}
	if sku.StockAvailable <= 0 {
		return "out_of_stock"
	}
	if sku.DaysWithoutSales != nil && *sku.DaysWithoutSales >= int32(stockpolicy.PersistentNoSalesMinDays) && sku.OrdersLast7Days == 0 {
		return "no_sales"
	}
	if sku.DaysOfCover != nil {
		d := *sku.DaysOfCover
		switch {
		case d <= stockpolicy.CoverageCriticalMax:
			return "critical_0_3"
		case d <= stockpolicy.CoverageShortageMax:
			return "shortage_4_7"
		case d <= stockpolicy.CoveragePlanningMax:
			return "planning_8_14"
		case d >= stockpolicy.CoverageSurplusMin:
			return "surplus"
		default:
			return "healthy_14plus"
		}
	}
	if sku.ExcessStockShare != nil && *sku.ExcessStockShare > 0.2 {
		return "surplus"
	}
	return "healthy_14plus"
}
