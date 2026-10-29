package dashboardhome

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"

// StockCoverageBucketKey maps SKU metrics to one of five coverage segments used on the dashboard.
func StockCoverageBucketKey(sku analytics.DashboardSKUMetricsItem) string {
	return analytics.StockCoverageBucketKey(sku)
}
