package dashboardfacts

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

// Exclusive stock coverage segment keys. Every actionable SKU maps to exactly one.
var stockCoverageSegmentOrder = []string{
	"out_of_stock",
	"critical_0_3",
	"shortage_4_7",
	"planning_8_14",
	"healthy_14plus",
	"surplus",
	"no_sales",
	"collecting_data",
}

func buildStockCoverageFacts(metrics analytics.DashboardMetricsDTO) StockCoverageFacts {
	buckets := map[string]*StockCoverageSegment{
		"out_of_stock":    {Key: "out_of_stock", Label: "Нет остатка", Severity: "critical"},
		"critical_0_3":    {Key: "critical_0_3", Label: "0–3 дня", Severity: "critical"},
		"shortage_4_7":    {Key: "shortage_4_7", Label: "4–7 дней", Severity: "high"},
		"planning_8_14":   {Key: "planning_8_14", Label: "8–14 дней", Severity: "medium"},
		"healthy_14plus":  {Key: "healthy_14plus", Label: ">14 дней", Severity: "low"},
		"surplus":         {Key: "surplus", Label: "Избыток (>60 дн.)", Severity: "high"},
		"no_sales":        {Key: "no_sales", Label: "Без продаж", Severity: "high"},
		"collecting_data": {Key: "collecting_data", Label: "Недостаточно данных", Severity: "low"},
	}
	total := 0
	for _, sku := range metrics.SKUs {
		total++
		key := exclusiveStockSegmentKey(sku)
		if seg, ok := buckets[key]; ok {
			seg.SKUCount++
		} else {
			buckets["collecting_data"].SKUCount++
		}
	}
	segments := make([]StockCoverageSegment, 0, len(stockCoverageSegmentOrder))
	for _, k := range stockCoverageSegmentOrder {
		segments = append(segments, *buckets[k])
	}
	status := "available"
	if total == 0 {
		status = "not_enough_data"
	}
	return StockCoverageFacts{
		Status:    status,
		TotalSKUs: total,
		Segments:  segments,
	}
}

func exclusiveStockSegmentKey(sku analytics.DashboardSKUMetricsItem) string {
	if sku.StockStatusBucket != nil {
		return stockpolicy.MapToCoverageSegment(*sku.StockStatusBucket)
	}
	return analytics.StockCoverageBucketKey(sku)
}
