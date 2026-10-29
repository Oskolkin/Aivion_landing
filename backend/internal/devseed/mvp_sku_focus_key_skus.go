package devseed

import (
	"math"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// mvpKeySkuDemoSKU is a deterministic demo scenario for SKU-фокус → Ключевые.
// RevenueWeight is a RELATIVE Pareto weight; the plan converts it into a SHARE OF TOTAL
// account 7-day revenue (see keySkuAccountRevenueShare), so key SKUs are ~80-85% of the
// store — NOT an absolute target independent of the account. Product names must not overlap
// order-drop / stock-risk / no-sales demo SKUs.
type mvpKeySkuDemoSKU struct {
	ProductName    string
	BasePrice      float64
	RevenueWeight  float64
	OrdersWeight7D [7]float64
	CoverageDays   float64
	Stock          int32
}

// Weights (14,12,10.5,...,4.5) sum to 81.5; scaled to keySkuAccountRevenueShare of the
// account they land the dashboard 80%-cohort selection on ~10 SKUs.
var mvpKeySkuDemoSKUs = []mvpKeySkuDemoSKU{
	{
		ProductName:    "Полка-органайзер для ванной",
		BasePrice:      6400,
		RevenueWeight:  14.0,
		OrdersWeight7D: [7]float64{1.0, 1.1, 0.9, 1.0, 1.2, 1.0, 1.1},
		CoverageDays:   2.0, // critical
		Stock:          58,
	},
	{
		ProductName:    "Набор банок для специй",
		BasePrice:      5625,
		RevenueWeight:  12.0,
		OrdersWeight7D: [7]float64{1.0, 0.9, 1.0, 1.1, 0.95, 1.05, 1.0},
		CoverageDays:   5.0, // short
		Stock:          14,
	},
	{
		ProductName:    "Вешалки пластиковые 10 шт",
		BasePrice:      5000,
		RevenueWeight:  10.5,
		OrdersWeight7D: [7]float64{0.95, 1.0, 1.05, 1.0, 0.9, 1.1, 1.0},
		CoverageDays:   28.0, // healthy
		Stock:          52,
	},
	{
		ProductName:    "Корзина для белья складная",
		BasePrice:      4688,
		RevenueWeight:  9.0,
		OrdersWeight7D: [7]float64{1.0, 1.0, 0.9, 1.1, 1.0, 0.95, 1.05},
		CoverageDays:   4.5, // short
		Stock:          22,
	},
	{
		ProductName:    "Диспенсер для моющего средства",
		BasePrice:      4375,
		RevenueWeight:  8.0,
		OrdersWeight7D: [7]float64{1.05, 0.95, 1.0, 1.0, 1.1, 0.9, 1.0},
		CoverageDays:   22.0, // healthy
		Stock:          9,
	},
	{
		ProductName:    "Разделитель для ящика кухонного",
		BasePrice:      4063,
		RevenueWeight:  7.0,
		OrdersWeight7D: [7]float64{1.0, 1.05, 0.95, 1.0, 1.0, 1.1, 0.9},
		CoverageDays:   30.0, // healthy
		Stock:          56,
	},
	{
		ProductName:    "Подставка для столовых приборов",
		BasePrice:      3750,
		RevenueWeight:  6.0,
		OrdersWeight7D: [7]float64{0.9, 1.0, 1.1, 1.0, 0.95, 1.05, 1.0},
		CoverageDays:   12.0, // planning
		Stock:          24,
	},
	{
		ProductName:    "Сушилка для посуды над раковиной",
		BasePrice:      3438,
		RevenueWeight:  5.5,
		OrdersWeight7D: [7]float64{1.0, 0.9, 1.0, 1.05, 1.0, 1.0, 0.95},
		CoverageDays:   25.0, // healthy
		Stock:          36,
	},
	{
		ProductName:    "Коврик противоскользящий для ванной",
		BasePrice:      3125,
		RevenueWeight:  5.0,
		OrdersWeight7D: [7]float64{1.05, 1.0, 0.95, 1.0, 1.0, 0.9, 1.1},
		CoverageDays:   6.0, // short
		Stock:          18,
	},
	{
		ProductName:    "Плед флисовый 150×200",
		BasePrice:      2813,
		RevenueWeight:  4.5,
		OrdersWeight7D: [7]float64{1.0, 0.95, 1.05, 1.0, 1.0, 1.1, 0.9},
		CoverageDays:   20.0, // healthy
		Stock:          42,
	},
}

func keySkuDemoProductNames() []string {
	names := make([]string, len(mvpKeySkuDemoSKUs))
	for i, scenario := range mvpKeySkuDemoSKUs {
		names[i] = scenario.ProductName
	}
	return names
}

func mvpKeySkuWeightSum() float64 {
	sum := 0.0
	for _, sc := range mvpKeySkuDemoSKUs {
		sum += sc.RevenueWeight
	}
	return sum
}

// mvpKeySkuWeeklyRevenues returns each curated key SKU's planned 7-day revenue given the
// combined weekly revenue of everything else in the account (long tail + other demos).
func mvpKeySkuWeeklyRevenues(otherWeekly float64) []float64 {
	weightSum := mvpKeySkuWeightSum()
	keyTotal := (keySkuAccountRevenueShare / (1 - keySkuAccountRevenueShare)) * otherWeekly
	out := make([]float64, len(mvpKeySkuDemoSKUs))
	if weightSum <= 0 {
		return out
	}
	for i, sc := range mvpKeySkuDemoSKUs {
		out[i] = mvpRoundMoney((sc.RevenueWeight / weightSum) * keyTotal)
	}
	return out
}

// ExpectedMVPKeySkuCount returns curated demo key SKU rows (not dashboard cohort size).
func ExpectedMVPKeySkuCount() int {
	return len(mvpKeySkuDemoSKUs)
}

// ExpectedMVPKeySkuCohortCount simulates dashboard key-SKU selection on demo targets + long tail.
func ExpectedMVPKeySkuCohortCount() int {
	const otherWeekly = 1_000_000.0
	keyRevenues := mvpKeySkuWeeklyRevenues(otherWeekly)
	items := make([]analytics.DashboardSKUMetricsItem, 0, len(keyRevenues)+70)
	for i, rev := range keyRevenues {
		items = append(items, analytics.DashboardSKUMetricsItem{
			OzonProductID:    int64(i + 1),
			RevenueLast7Days: rev,
			OrdersLast7Days:  int64(math.Round(rev / mvpKeySkuDemoSKUs[i].BasePrice)),
		})
	}
	// Long tail: 70 tiny SKUs summing to otherWeekly, each far below the smallest key SKU.
	tailEach := otherWeekly / 70.0
	for i := 0; i < 70; i++ {
		items = append(items, analytics.DashboardSKUMetricsItem{
			OzonProductID:    int64(1000 + i),
			RevenueLast7Days: mvpRoundMoney(tailEach),
			OrdersLast7Days:  1,
		})
	}
	return len(dashboardhome.KeySkuProductIDs(items))
}

// ExpectedMVPKeySkuMinCumulativeSharePct returns the cumulative share of total account
// revenue owned by curated key SKUs (demo target band ~55–75%).
func ExpectedMVPKeySkuMinCumulativeSharePct() float64 {
	const otherWeekly = 1_000_000.0
	keyRevenues := mvpKeySkuWeeklyRevenues(otherWeekly)
	total := otherWeekly
	for _, rev := range keyRevenues {
		total += rev
	}
	if total <= 0 {
		return 0
	}
	var cumulative float64
	for _, rev := range keyRevenues {
		cumulative += (rev / total) * 100
	}
	return math.Round(cumulative*10) / 10
}
