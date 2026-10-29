package devseed

// mvpOrderDropDemoSKU is a deterministic demo scenario for SKU-фокус → Заказы.
// The scenario is written into SOURCE orders/sales by BuildMVPScenarioPlan, so the
// order-drop pattern emerges naturally from the metrics rebuild (no post-rebuild patch).
type mvpOrderDropDemoSKU struct {
	ProductName  string
	BasePrice    float64
	Orders7D     [7]int32
	PriceFactors [7]float64
}

// mvpOrderDropDemoSKUs defines five material WoW order-drop patterns.
// Current-week totals stay meaningful (≈20–55 orders); previous week is elevated
// in BuildMVPScenarioPlan so decline lands in the 20–45% band with lost revenue
// that matters for the cabinet — not day-to-day noise like 5→2.
var mvpOrderDropDemoSKUs = []mvpOrderDropDemoSKU{
	{
		ProductName:  "Швабра с отжимом и ведром",
		BasePrice:    3500,
		Orders7D:     [7]int32{8, 7, 8, 7, 6, 6, 5},
		PriceFactors: [7]float64{1.00, 1.02, 0.99, 1.00, 0.98, 1.01, 1.00},
	},
	{
		ProductName:  "Светильник настенный бра 1×E14",
		BasePrice:    2100,
		Orders7D:     [7]int32{7, 6, 7, 6, 5, 5, 4},
		PriceFactors: [7]float64{1.00, 0.99, 1.01, 1.00, 0.98, 1.02, 1.00},
	},
	{
		ProductName:  "Сушилка для белья настенная",
		BasePrice:    1900,
		Orders7D:     [7]int32{6, 5, 6, 5, 5, 4, 4},
		PriceFactors: [7]float64{1.00, 1.01, 0.99, 1.00, 1.00, 1.01, 0.99},
	},
	{
		ProductName:  "Органайзер для косметики",
		BasePrice:    1600,
		Orders7D:     [7]int32{5, 5, 4, 5, 4, 4, 3},
		PriceFactors: [7]float64{1.00, 1.00, 0.98, 1.01, 1.00, 0.99, 1.00},
	},
	{
		ProductName:  "LED-лампа настольная складная",
		BasePrice:    1700,
		Orders7D:     [7]int32{5, 4, 5, 4, 4, 3, 3},
		PriceFactors: [7]float64{1.00, 0.99, 1.02, 1.00, 0.98, 1.01, 1.00},
	},
}

func mvpOrderDropDemoUnitPrice(base float64, factor float64) float64 {
	return mvpRoundMoney(base * factor)
}

func mvpOrderDropDemoDayRevenue(orders int32, unitPrice float64) float64 {
	if orders <= 0 {
		return 0
	}
	return mvpRoundMoney(float64(orders) * unitPrice)
}

func mvpOrderDropDemoOrdersDelta(series [7]int32) int32 {
	return series[6] - series[5]
}

func mvpOrderDropDemoRevenueDelta(revenues [7]float64) float64 {
	return mvpRoundMoney(revenues[6] - revenues[5])
}

func orderDropDemoProductNames() []string {
	names := make([]string, len(mvpOrderDropDemoSKUs))
	for i, scenario := range mvpOrderDropDemoSKUs {
		names[i] = scenario.ProductName
	}
	return names
}
