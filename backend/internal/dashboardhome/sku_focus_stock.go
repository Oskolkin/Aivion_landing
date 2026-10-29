package dashboardhome

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

const stockFocusThresholdDays = 7.0

// StockTrendPoint is a daily stock snapshot for SKU-фокус → Остатки charts.
type StockTrendPoint struct {
	Date         string   `json:"date"`
	Stock        int32    `json:"stock"`
	CoverageDays *float64 `json:"coverage_days,omitempty"`
	HasData      bool     `json:"has_data,omitempty"`
}

type stockTrendDayMetrics struct {
	stock        int32
	coverageDays *float64
	hasData      bool
}

type stockFocusMetrics struct {
	previousStock     *int32
	avgDailyRevenue7d float64
	hasRevenue7d      bool
	stockTrend        []StockTrendPoint
}

type stockFocusCandidate struct {
	ozonProductID     int64
	sku               *int64
	offerID           *string
	productName       string
	currentStock      int32
	previousStock     *int32
	coverageDays      float64
	avgDailyRevenue7d float64
	revenueAtRisk7d   float64
	stockTrend        []StockTrendPoint
}

func qualifiesForStockFocusTab(stockAvailable int32, daysOfCover *float64) bool {
	if stockAvailable <= 0 {
		return false
	}
	if daysOfCover == nil {
		return false
	}
	if *daysOfCover <= 0 {
		return false
	}
	return *daysOfCover <= stockFocusThresholdDays
}

func isDashboardStockFocusSKU(sku analytics.DashboardSKUMetricsItem) bool {
	if !qualifiesForStockFocusTab(sku.StockAvailable, sku.DaysOfCover) {
		return false
	}
	if sku.StockStatusBucket != nil {
		switch *sku.StockStatusBucket {
		case "out_of_stock":
			return false
		}
	}
	return true
}

func isReplenishmentStockFocusCandidate(item analytics.ReplenishmentSKUItem) bool {
	if !qualifiesForStockFocusTab(item.CurrentAvailableStock, item.DaysOfCover) {
		return false
	}
	switch item.DepletionRisk {
	case "critical_out_of_stock":
		return false
	}
	return isReplenishmentStockRisk(item)
}

func calcRevenueAtRisk7d(avgDailyRevenue7d, coverageDays float64) float64 {
	uncovered := math.Max(0, 7-coverageDays)
	if avgDailyRevenue7d <= 0 || uncovered <= 0 {
		return 0
	}
	return math.Round(avgDailyRevenue7d*uncovered*100) / 100
}

func previousStockFromTrend(trend []StockTrendPoint) *int32 {
	if len(trend) < 2 {
		return nil
	}
	for i := len(trend) - 2; i >= 0; i-- {
		if trend[i].HasData {
			v := trend[i].Stock
			return &v
		}
	}
	return nil
}

func anchorStockFromTrend(trend []StockTrendPoint) (int32, bool) {
	if len(trend) == 0 {
		return 0, false
	}
	last := trend[len(trend)-1]
	if !last.HasData {
		return 0, false
	}
	return last.Stock, true
}

func buildCalendarStockTrendSeries(from, to time.Time, metricsByDate map[string]stockTrendDayMetrics) []StockTrendPoint {
	points := make([]StockTrendPoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := StockTrendPoint{Date: key}
		if m, ok := metricsByDate[key]; ok && m.hasData {
			pt.HasData = true
			pt.Stock = m.stock
			pt.CoverageDays = m.coverageDays
		}
		points = append(points, pt)
	}
	return points
}

func sortStockFocusCandidates(candidates []stockFocusCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.coverageDays != b.coverageDays {
			return a.coverageDays < b.coverageDays
		}
		if a.revenueAtRisk7d != b.revenueAtRisk7d {
			return a.revenueAtRisk7d > b.revenueAtRisk7d
		}
		if a.productName != b.productName {
			return a.productName < b.productName
		}
		return a.ozonProductID < b.ozonProductID
	})
}

func stockFocusCardFromCandidate(c stockFocusCandidate) SKUFocusCard {
	coverage := c.coverageDays
	avgRev := c.avgDailyRevenue7d
	revRisk := c.revenueAtRisk7d
	var stockDelta *int32
	if c.previousStock != nil {
		delta := c.currentStock - *c.previousStock
		stockDelta = &delta
	}
	return SKUFocusCard{
		OzonProductID:      c.ozonProductID,
		SKU:                c.sku,
		OfferID:            c.offerID,
		ProductName:        c.productName,
		Problem:            "Скоро закончатся",
		KeyMetric:          fmt.Sprintf("Остаток %d · покрытие %.1f дн.", c.currentStock, c.coverageDays),
		CurrentStock:       &c.currentStock,
		PreviousStock:      c.previousStock,
		StockDeltaDayToDay: stockDelta,
		CoverageDays:       &coverage,
		AvgDailyRevenue7d:  &avgRev,
		RevenueAtRisk7d:    &revRisk,
		StockTrend7D:       c.stockTrend,
	}
}

func buildStockFocusCandidate(
	ozonProductID int64,
	sku *int64,
	offerID *string,
	productName string,
	currentStock int32,
	coverageDays float64,
	metrics stockFocusMetrics,
) (stockFocusCandidate, bool) {
	if anchorStock, ok := anchorStockFromTrend(metrics.stockTrend); ok {
		currentStock = anchorStock
	}
	previousStock := metrics.previousStock
	if previousStock == nil {
		previousStock = previousStockFromTrend(metrics.stockTrend)
	}
	if !qualifiesForStockFocusTab(currentStock, &coverageDays) {
		return stockFocusCandidate{}, false
	}
	if !metrics.hasRevenue7d || metrics.avgDailyRevenue7d <= 0 {
		return stockFocusCandidate{}, false
	}
	revRisk := calcRevenueAtRisk7d(metrics.avgDailyRevenue7d, coverageDays)
	if revRisk <= 0 {
		return stockFocusCandidate{}, false
	}
	return stockFocusCandidate{
		ozonProductID:     ozonProductID,
		sku:               sku,
		offerID:           offerID,
		productName:       productName,
		currentStock:      currentStock,
		previousStock:     previousStock,
		coverageDays:      coverageDays,
		avgDailyRevenue7d: metrics.avgDailyRevenue7d,
		revenueAtRisk7d:   revRisk,
		stockTrend:        metrics.stockTrend,
	}, true
}
