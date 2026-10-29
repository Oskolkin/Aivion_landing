package devseed

import "math"

// mvpStockRiskDemoSKU is a deterministic demo scenario for SKU-фокус → Остатки.
// CatalogIndex must stay within default --products=80 and must not overlap order-drop demo SKUs.
// BuildMVPScenarioPlan writes the current stock (Stock7D[6]) into the stocks table and steady
// daily orders into sales, so the metrics rebuild derives a low days-of-cover from SOURCE.
type mvpStockRiskDemoSKU struct {
	CatalogIndex int
	BasePrice    float64
	Stock7D      [7]int32
	CoverageDays float64
	OrdersPerDay int32
}

var mvpStockRiskDemoSKUs = []mvpStockRiskDemoSKU{
	{
		CatalogIndex: 48, // Светильник подсветки кухонного шкафа
		BasePrice:    1590,
		Stock7D:      [7]int32{14, 11, 9, 7, 5, 4, 2},
		CoverageDays: 0.6,
		OrdersPerDay: 3,
	},
	{
		CatalogIndex: 61, // Светильник напольный торшер
		BasePrice:    4290,
		Stock7D:      [7]int32{18, 16, 13, 10, 8, 5, 3},
		CoverageDays: 0.9,
		OrdersPerDay: 3,
	},
	{
		CatalogIndex: 21, // Гирлянда LED на батарейках
		BasePrice:    720,
		Stock7D:      [7]int32{25, 23, 21, 18, 16, 14, 10},
		CoverageDays: 3.5,
		OrdersPerDay: 3,
	},
	{
		CatalogIndex: 68, // Лампа USB для чтения прищепка
		BasePrice:    490,
		Stock7D:      [7]int32{40, 36, 32, 28, 24, 20, 15},
		CoverageDays: 5.0,
		OrdersPerDay: 3,
	},
}

func (s mvpStockRiskDemoSKU) productName() string {
	idx := s.CatalogIndex % len(mvpProductCatalog)
	return mvpProductCatalog[idx].NameRU
}

func mvpStockRiskDemoDayRevenue(orders int32, unitPrice float64) float64 {
	if orders <= 0 {
		return 0
	}
	return mvpRoundMoney(float64(orders) * unitPrice)
}

func mvpStockRiskDemoRevenueAtRisk(avgDailyRevenue float64, coverageDays float64) float64 {
	uncovered := math.Max(0, 7-coverageDays)
	return mvpRoundMoney(avgDailyRevenue * uncovered)
}

func stockRiskDemoProductNames() []string {
	names := make([]string, len(mvpStockRiskDemoSKUs))
	for i, scenario := range mvpStockRiskDemoSKUs {
		names[i] = scenario.productName()
	}
	return names
}
