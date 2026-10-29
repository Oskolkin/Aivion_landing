package dashboardhome

import (
	"math"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

const keySkuRevenueShareThresholdPct = 70.0

// KeySkuRevenueTrendPoint is a daily revenue/orders point for SKU-фокус → Ключевые charts.
type KeySkuRevenueTrendPoint struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Orders  int32   `json:"orders"`
	HasData *bool   `json:"has_data,omitempty"`
}

// KeySkuCard is a structured card for SKU-фокус → Ключевые (80% store revenue over 7 days).
type KeySkuCard struct {
	OzonProductID int64   `json:"ozon_product_id"`
	ProductName   string  `json:"product_name"`
	SKU           *int64  `json:"sku,omitempty"`
	OfferID       *string `json:"offer_id,omitempty"`
	ImageURL      *string `json:"image_url,omitempty"`
	ProductURL    *string `json:"product_url,omitempty"`

	Revenue7d                 float64 `json:"revenue_7d"`
	Orders7d                  int64   `json:"orders_7d"`
	RevenueSharePct           float64 `json:"revenue_share_pct"`
	CumulativeRevenueSharePct float64 `json:"cumulative_revenue_share_pct"`

	CoverageDays *float64 `json:"coverage_days,omitempty"`
	CurrentStock *int32   `json:"current_stock,omitempty"`

	RevenueTrend7D []KeySkuRevenueTrendPoint `json:"revenue_trend_7d"`
}

type keySkuCandidate struct {
	ozonProductID      int64
	sku                *int64
	offerID            *string
	productName        string
	revenue7d          float64
	orders7d           int64
	coverageDays       *float64
	currentStock       int32
	revenueTrend       []KeySkuRevenueTrendPoint
	revenueSharePct    float64
	cumulativeSharePct float64
}

func buildKeySkuCandidates(skus []analytics.DashboardSKUMetricsItem) []keySkuCandidate {
	out := make([]keySkuCandidate, 0)
	for _, sku := range skus {
		if sku.RevenueLast7Days <= 0 {
			continue
		}
		out = append(out, keySkuCandidate{
			ozonProductID: sku.OzonProductID,
			sku:           sku.SKU,
			offerID:       sku.OfferID,
			productName:   derefStr(sku.ProductName, "Товар"),
			revenue7d:     sku.RevenueLast7Days,
			orders7d:      sku.OrdersLast7Days,
			coverageDays:  sku.DaysOfCover,
			currentStock:  sku.StockAvailable,
		})
	}
	return out
}

func calcStoreTotalRevenue7dFromDashboardSKUs(skus []analytics.DashboardSKUMetricsItem) float64 {
	var total float64
	for _, sku := range skus {
		if sku.RevenueLast7Days <= 0 {
			continue
		}
		total += sku.RevenueLast7Days
	}
	return total
}

func sumKeySkuSelectedRevenue7d(selected []keySkuCandidate) float64 {
	var total float64
	for _, c := range selected {
		total += c.revenue7d
	}
	return total
}

func selectKeySkuCandidates(candidates []keySkuCandidate, storeTotalRevenue7d float64) []keySkuCandidate {
	if storeTotalRevenue7d <= 0 {
		return nil
	}
	sortKeySkuCandidates(candidates)
	var cumulative float64
	selected := make([]keySkuCandidate, 0)
	for _, c := range candidates {
		share := (c.revenue7d / storeTotalRevenue7d) * 100
		cumulative += share
		c.revenueSharePct = roundKeySkuPct(share)
		c.cumulativeSharePct = roundKeySkuPct(cumulative)
		selected = append(selected, c)
		if c.cumulativeSharePct >= keySkuRevenueShareThresholdPct {
			break
		}
	}
	return selected
}

func roundKeySkuPct(v float64) float64 {
	return math.Round(v*10) / 10
}

func sortKeySkuCandidates(candidates []keySkuCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.revenue7d != b.revenue7d {
			return a.revenue7d > b.revenue7d
		}
		if a.orders7d != b.orders7d {
			return a.orders7d > b.orders7d
		}
		if a.productName != b.productName {
			return a.productName < b.productName
		}
		if a.sku != nil && b.sku != nil && *a.sku != *b.sku {
			return *a.sku < *b.sku
		}
		return a.ozonProductID < b.ozonProductID
	})
}

func buildCalendarKeySkuRevenueTrend(from, to time.Time, metricsByDate map[string]sparklineDayMetrics) []KeySkuRevenueTrendPoint {
	points := make([]KeySkuRevenueTrendPoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := KeySkuRevenueTrendPoint{Date: key}
		if m, ok := metricsByDate[key]; ok && m.hasOrders {
			hasData := true
			pt.HasData = &hasData
			pt.Orders = int32(m.orders)
			if m.hasRevenue {
				pt.Revenue = m.revenue
			}
		}
		points = append(points, pt)
	}
	return points
}

func keySkuCardFromCandidate(c keySkuCandidate) KeySkuCard {
	var stock *int32
	if c.currentStock > 0 {
		v := c.currentStock
		stock = &v
	}
	trend := c.revenueTrend
	if trend == nil {
		trend = []KeySkuRevenueTrendPoint{}
	}
	return KeySkuCard{
		OzonProductID:             c.ozonProductID,
		ProductName:               c.productName,
		SKU:                       c.sku,
		OfferID:                   c.offerID,
		Revenue7d:                 math.Round(c.revenue7d*100) / 100,
		Orders7d:                  c.orders7d,
		RevenueSharePct:           c.revenueSharePct,
		CumulativeRevenueSharePct: c.cumulativeSharePct,
		CoverageDays:              c.coverageDays,
		CurrentStock:              stock,
		RevenueTrend7D:            trend,
	}
}

// KeySkuSelectionFromDashboard returns store total, sum of selected key SKU revenue, and full key SKU cards (no preview limit).
func KeySkuSelectionFromDashboard(skus []analytics.DashboardSKUMetricsItem) (storeTotal float64, keySkusTotal float64, cards []KeySkuCard) {
	storeTotal = calcStoreTotalRevenue7dFromDashboardSKUs(skus)
	candidates := buildKeySkuCandidates(skus)
	selected := selectKeySkuCandidates(candidates, storeTotal)
	keySkusTotal = sumKeySkuSelectedRevenue7d(selected)
	cards = make([]KeySkuCard, 0, len(selected))
	for _, c := range selected {
		cards = append(cards, keySkuCardFromCandidate(c))
	}
	return storeTotal, keySkusTotal, cards
}

// KeySkuProductIDs returns ozon_product_id values for SKUs in the 80% revenue cohort.
func KeySkuProductIDs(skus []analytics.DashboardSKUMetricsItem) []int64 {
	candidates := buildKeySkuCandidates(skus)
	storeTotal := calcStoreTotalRevenue7dFromDashboardSKUs(skus)
	selected := selectKeySkuCandidates(candidates, storeTotal)
	ids := make([]int64, 0, len(selected))
	for _, c := range selected {
		ids = append(ids, c.ozonProductID)
	}
	return ids
}
