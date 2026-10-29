package devseed

// mvpNoSalesDemoSKU is a deterministic demo scenario for SKU-фокус → Без продаж.
// BuildMVPScenarioPlan writes positive stock plus a trailing zero-order streak into SOURCE,
// so the "no recent sales but stock on hand" pattern emerges from the metrics rebuild.
type mvpNoSalesDemoSKU struct {
	CatalogIndex int
	BasePrice    float64
	Stock        int32
	NoSalesDays  int
}

var mvpNoSalesDemoSKUs = []mvpNoSalesDemoSKU{
	{CatalogIndex: 37, BasePrice: 527, Stock: 342, NoSalesDays: 21},
	{CatalogIndex: 31, BasePrice: 1011, Stock: 94, NoSalesDays: 18},
	{CatalogIndex: 35, BasePrice: 897, Stock: 78, NoSalesDays: 16},
	{CatalogIndex: 34, BasePrice: 1038, Stock: 53, NoSalesDays: 14},
}

func (s mvpNoSalesDemoSKU) productName() string {
	idx := s.CatalogIndex % len(mvpProductCatalog)
	return mvpProductCatalog[idx].NameRU
}

func mvpNoSalesDemoOrdersForDay(dayOffset int, noSalesDays int, anchorIndex int) int32 {
	if dayOffset > anchorIndex {
		return 0
	}
	streakStart := anchorIndex - noSalesDays + 1
	if dayOffset >= streakStart {
		return 0
	}
	return 2
}

func noSalesDemoProductNames() []string {
	names := make([]string, len(mvpNoSalesDemoSKUs))
	for i, scenario := range mvpNoSalesDemoSKUs {
		names[i] = scenario.productName()
	}
	return names
}

func mvpNoSalesDemoFrozenValue(stock int32, unitPrice float64) float64 {
	return mvpRoundMoney(float64(stock) * unitPrice)
}
