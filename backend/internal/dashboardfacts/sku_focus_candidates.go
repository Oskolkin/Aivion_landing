package dashboardfacts

import (
	"math"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

// SKUFocusCandidates are backend-deterministic tab memberships for SKU Focus.
// AI may explain reasons; it must not invent empty tabs when candidates exist.
type SKUFocusCandidates struct {
	OrderDrop      []SKUFactRecord
	StockRisk      []SKUFactRecord
	NoSales        []SKUFactRecord
	OrderDropTotal int
	StockRiskTotal int
	NoSalesTotal   int
}

const (
	skuFocusOrderDropMinPrevOrders = int64(20)
	skuFocusOrderDropMinDropPct    = 0.20
	skuFocusTabLimit               = 8
	// Dashboard display caps (top-N). Totals remain available for "show all N".
	SKUFocusDashboardOrderDropLimit = 6
	SKUFocusDashboardStockRiskLimit = 9
	SKUFocusDashboardNoSalesLimit   = 6
	SKUFocusDashboardKeySKULimit    = 9
)

// BuildSKUFocusCandidates selects factual tab candidates from the SKU facts index.
func BuildSKUFocusCandidates(idx SKUFactsIndex) SKUFocusCandidates {
	var orderDrop, stockRisk, noSales []SKUFactRecord
	for _, rec := range idx.ByOzonProductID {
		if qualifiesOrderDrop(rec) {
			orderDrop = append(orderDrop, rec)
		}
		if qualifiesStockRisk(rec) {
			stockRisk = append(stockRisk, rec)
		}
		if qualifiesNoSales(rec) {
			noSales = append(noSales, rec)
		}
	}
	sort.Slice(orderDrop, func(i, j int) bool {
		lostI := float64(orderDrop[i].OrdersPrev7d - orderDrop[i].Orders7d)
		lostJ := float64(orderDrop[j].OrdersPrev7d - orderDrop[j].Orders7d)
		if lostI != lostJ {
			return lostI > lostJ
		}
		revLostI := orderDrop[i].RevenuePrev7d - orderDrop[i].Revenue7d
		revLostJ := orderDrop[j].RevenuePrev7d - orderDrop[j].Revenue7d
		if revLostI != revLostJ {
			return revLostI > revLostJ
		}
		return orderDrop[i].OzonProductID < orderDrop[j].OzonProductID
	})
	sort.Slice(stockRisk, func(i, j int) bool {
		return compareStockFocusRecords(stockRisk[i], stockRisk[j])
	})
	sort.Slice(noSales, func(i, j int) bool {
		si := noSales[i].StockAvailable
		sj := noSales[j].StockAvailable
		if si != sj {
			return si > sj
		}
		return noSales[i].OzonProductID < noSales[j].OzonProductID
	})
	attentionTotal := 0
	for _, rec := range stockRisk {
		if stockpolicy.IsStockFocusAttentionSegment(rec.CoverageBucket) {
			attentionTotal++
		}
	}
	return SKUFocusCandidates{
		OrderDropTotal: len(orderDrop),
		// Attention (all-risk) total for CTA / «Все риски»; list also carries planning_8_14.
		StockRiskTotal: attentionTotal,
		NoSalesTotal:   len(noSales),
		OrderDrop:      clampFocus(orderDrop, skuFocusTabLimit),
		// Stock tab keeps the full ranked universe (risk + planning) so Dashboard
		// can navigate "show all N" and segment filters without a second fetch.
		StockRisk: stockRisk,
		NoSales:   clampFocus(noSales, skuFocusTabLimit),
	}
}

func stockRiskUrgencyRank(rec SKUFactRecord) int {
	switch rec.CoverageBucket {
	case stockpolicy.SegmentOutOfStock:
		return 0
	case stockpolicy.SegmentCritical0_3:
		return 1
	case stockpolicy.SegmentShortage4_7:
		return 2
	case stockpolicy.SegmentPlanning8_14:
		return 3
	default:
		return 9
	}
}

func compareStockFocusRecords(a, b SKUFactRecord) bool {
	ui := stockRiskUrgencyRank(a)
	uj := stockRiskUrgencyRank(b)
	if ui != uj {
		return ui < uj
	}
	ca, cb := coverageDaysOrMax(a), coverageDaysOrMax(b)
	if ca != cb {
		return ca < cb
	}
	ri := revenueAtRiskDaily(a)
	rj := revenueAtRiskDaily(b)
	if ri != rj {
		return ri > rj
	}
	if a.Revenue7d != b.Revenue7d {
		return a.Revenue7d > b.Revenue7d
	}
	if a.OzonProductID != b.OzonProductID {
		return a.OzonProductID < b.OzonProductID
	}
	return skuIDOrZero(a) < skuIDOrZero(b)
}

func coverageDaysOrMax(rec SKUFactRecord) float64 {
	if rec.DaysOfCover == nil {
		return math.MaxFloat64
	}
	return *rec.DaysOfCover
}

func skuIDOrZero(rec SKUFactRecord) int64 {
	if rec.SKU == nil {
		return 0
	}
	return *rec.SKU
}

func qualifiesOrderDrop(rec SKUFactRecord) bool {
	if rec.OrdersPrev7d < skuFocusOrderDropMinPrevOrders {
		return false
	}
	if rec.Orders7d >= rec.OrdersPrev7d {
		return false
	}
	drop := float64(rec.OrdersPrev7d-rec.Orders7d) / float64(rec.OrdersPrev7d)
	return drop >= skuFocusOrderDropMinDropPct
}

func qualifiesStockRisk(rec SKUFactRecord) bool {
	return stockpolicy.IsStockFocusStockTabSegment(rec.CoverageBucket)
}

func qualifiesNoSales(rec SKUFactRecord) bool {
	if rec.StockAvailable <= 0 {
		return false
	}
	if rec.CoverageBucket == "no_sales" {
		return true
	}
	if rec.DaysWithoutSales != nil && int64(*rec.DaysWithoutSales) >= stockpolicy.PersistentNoSalesMinDays && rec.Orders7d == 0 {
		return true
	}
	return false
}

func revenueAtRiskDaily(rec SKUFactRecord) float64 {
	if rec.Revenue7d <= 0 {
		return 0
	}
	return rec.Revenue7d / 7
}

func clampFocus(in []SKUFactRecord, n int) []SKUFactRecord {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

// ClampSKUFocusCards truncates projected cards to the dashboard top-N limit.
func ClampSKUFocusCards[T any](in []T, n int) []T {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

// ComputeCategoryScore returns a 0–10 backend score from factual category signals.
// AI must not invent an independent score when this value is available.
func ComputeCategoryScore(c CategoryFacts) float64 {
	score := 8.0
	if c.PreviousRevenue > 0 {
		deltaPct := (c.Revenue - c.PreviousRevenue) / c.PreviousRevenue
		switch {
		case deltaPct >= 0.15:
			score += 1.0
		case deltaPct <= -0.15:
			score -= 1.5
		case deltaPct <= -0.05:
			score -= 0.5
		}
	}
	if c.SKUCount > 0 {
		riskShare := float64(c.StockRiskSKUCount) / float64(c.SKUCount)
		switch {
		case riskShare >= 0.35:
			score -= 2.0
		case riskShare >= 0.2:
			score -= 1.0
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	return math.Round(score*10) / 10
}
