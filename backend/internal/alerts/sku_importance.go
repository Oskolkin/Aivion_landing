package alerts

import (
	"math"
	"sort"
)

const (
	keySKUWindowDays              = 7
	keySKURevenueShareMin         = 0.05
	keySKUOrdersShareMin          = 0.05
	keySKUTopActivePercent        = 0.10
	keySKUTopActivePercentMax     = 30
	keySKUSmallCatalogActiveMax   = 50
	keySKUSmallCatalogTopN        = 5
	keySKUImportanceMethod        = "relative_seller_context_7d"
	keySKUReasonRevenueShare      = "revenue_share_7d_gte_5_percent"
	keySKUReasonOrdersShare       = "orders_share_7d_gte_5_percent"
	keySKUReasonTopRevenuePercent = "revenue_rank_top_10_percent_cap_30"
	keySKUReasonTopOrdersPercent  = "orders_rank_top_10_percent_cap_30"
	keySKUReasonTopRevenueSmall   = "revenue_rank_top_5_small_account"
	keySKUReasonTopOrdersSmall    = "orders_rank_top_5_small_account"
)

// SKUImportanceContext describes SKU significance within a seller account over a 7-day window.
type SKUImportanceContext struct {
	OzonProductID int64
	SKU           *int64
	OfferID       *string
	ProductName   *string

	Revenue7D float64
	Orders7D  int32

	TotalRevenue7D float64
	TotalOrders7D  int32
	ActiveSKUCount int

	RevenueShare7D float64
	OrdersShare7D  float64
	RevenueRank7D  int
	OrdersRank7D   int

	IsKeySKU   bool
	KeyReasons []string
}

// SKUImportanceByProduct maps ozon_product_id to importance context for the evaluation window.
type SKUImportanceByProduct map[int64]SKUImportanceContext

type skuImportanceAgg struct {
	ozonProductID int64
	sku           *int64
	offerID       *string
	productName   *string
	revenue7D     float64
	orders7D      int32
}

// BuildSKUImportanceFromDailyMetrics aggregates daily_sku_metrics rows over a date range
// (typically asOfDate-6 .. asOfDate) and classifies key/top SKUs relative to the seller account.
func BuildSKUImportanceFromDailyMetrics(metrics []SKUDailyMetric) SKUImportanceByProduct {
	byProduct := make(map[int64]*skuImportanceAgg)
	for _, m := range metrics {
		agg := byProduct[m.OzonProductID]
		if agg == nil {
			agg = &skuImportanceAgg{ozonProductID: m.OzonProductID}
			byProduct[m.OzonProductID] = agg
		}
		agg.revenue7D += m.Revenue
		agg.orders7D += m.OrdersCount
		if m.SKU != nil {
			agg.sku = m.SKU
		}
		if m.OfferID != nil {
			agg.offerID = m.OfferID
		}
		if m.ProductName != nil {
			agg.productName = m.ProductName
		}
	}

	active := make([]*skuImportanceAgg, 0, len(byProduct))
	var totalRevenue float64
	var totalOrders int32
	for _, agg := range byProduct {
		if isSKUActiveOverWindow(agg.revenue7D, agg.orders7D) {
			active = append(active, agg)
			totalRevenue += agg.revenue7D
			totalOrders += agg.orders7D
		}
	}
	activeCount := len(active)

	revenueRanks := rankActiveSKUs(active, func(a *skuImportanceAgg) float64 { return a.revenue7D })
	ordersRanks := rankActiveSKUs(active, func(a *skuImportanceAgg) int32 { return a.orders7D })

	topPercentileCutoff := keySKUTopPercentileCutoff(activeCount)
	smallCatalog := activeCount > 0 && activeCount < keySKUSmallCatalogActiveMax

	out := make(SKUImportanceByProduct, len(byProduct))
	for _, agg := range byProduct {
		ctx := SKUImportanceContext{
			OzonProductID:  agg.ozonProductID,
			SKU:            agg.sku,
			OfferID:        agg.offerID,
			ProductName:    agg.productName,
			Revenue7D:      agg.revenue7D,
			Orders7D:       agg.orders7D,
			TotalRevenue7D: totalRevenue,
			TotalOrders7D:  totalOrders,
			ActiveSKUCount: activeCount,
		}
		if totalRevenue > 0 {
			ctx.RevenueShare7D = agg.revenue7D / totalRevenue
		}
		if totalOrders > 0 {
			ctx.OrdersShare7D = float64(agg.orders7D) / float64(totalOrders)
		}
		if rank, ok := revenueRanks[agg.ozonProductID]; ok {
			ctx.RevenueRank7D = rank
		}
		if rank, ok := ordersRanks[agg.ozonProductID]; ok {
			ctx.OrdersRank7D = rank
		}
		ctx.IsKeySKU, ctx.KeyReasons = classifyKeySKU(ctx, topPercentileCutoff, smallCatalog)
		out[agg.ozonProductID] = ctx
	}
	return out
}

func isSKUActiveOverWindow(revenue7D float64, orders7D int32) bool {
	return revenue7D > 0 || orders7D > 0
}

func classifyKeySKU(ctx SKUImportanceContext, topPercentileCutoff int, smallCatalog bool) (bool, []string) {
	// Inactive only when there is no revenue and no orders over the window.
	if ctx.Revenue7D <= 0 && ctx.Orders7D <= 0 {
		return false, nil
	}
	reasons := make([]string, 0, 6)
	if ctx.Revenue7D > 0 && ctx.RevenueShare7D >= keySKURevenueShareMin {
		reasons = append(reasons, keySKUReasonRevenueShare)
	}
	if ctx.Orders7D > 0 && ctx.OrdersShare7D >= keySKUOrdersShareMin {
		reasons = append(reasons, keySKUReasonOrdersShare)
	}
	if ctx.Revenue7D > 0 && ctx.RevenueRank7D > 0 && ctx.RevenueRank7D <= topPercentileCutoff {
		reasons = append(reasons, keySKUReasonTopRevenuePercent)
	}
	if ctx.Orders7D > 0 && ctx.OrdersRank7D > 0 && ctx.OrdersRank7D <= topPercentileCutoff {
		reasons = append(reasons, keySKUReasonTopOrdersPercent)
	}
	if smallCatalog {
		if ctx.Revenue7D > 0 && ctx.RevenueRank7D > 0 && ctx.RevenueRank7D <= keySKUSmallCatalogTopN {
			reasons = append(reasons, keySKUReasonTopRevenueSmall)
		}
		if ctx.Orders7D > 0 && ctx.OrdersRank7D > 0 && ctx.OrdersRank7D <= keySKUSmallCatalogTopN {
			reasons = append(reasons, keySKUReasonTopOrdersSmall)
		}
	}
	if len(reasons) == 0 {
		return false, nil
	}
	return true, reasons
}

func keySKUTopPercentileCutoff(activeCount int) int {
	if activeCount <= 0 {
		return 0
	}
	n := int(math.Ceil(float64(activeCount) * keySKUTopActivePercent))
	if n < 1 {
		n = 1
	}
	if n > keySKUTopActivePercentMax {
		n = keySKUTopActivePercentMax
	}
	return n
}

func rankActiveSKUs[T int32 | float64](active []*skuImportanceAgg, value func(*skuImportanceAgg) T) map[int64]int {
	type ranked struct {
		productID int64
		value     T
	}
	items := make([]ranked, 0, len(active))
	for _, agg := range active {
		items = append(items, ranked{productID: agg.ozonProductID, value: value(agg)})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].value == items[j].value {
			return items[i].productID < items[j].productID
		}
		return items[i].value > items[j].value
	})
	ranks := make(map[int64]int, len(items))
	for i, item := range items {
		ranks[item.productID] = i + 1
	}
	return ranks
}

// MergeKeySKUImportanceEvidence adds relative key-SKU context fields to an evidence payload.
func MergeKeySKUImportanceEvidence(payload EvidencePayload, ctx SKUImportanceContext) EvidencePayload {
	if payload == nil {
		payload = EvidencePayload{}
	}
	payload["key_sku_method"] = keySKUImportanceMethod
	payload["revenue_7d"] = ctx.Revenue7D
	payload["orders_7d"] = ctx.Orders7D
	payload["total_revenue_7d"] = ctx.TotalRevenue7D
	payload["total_orders_7d"] = ctx.TotalOrders7D
	payload["active_sku_count"] = ctx.ActiveSKUCount
	payload["revenue_share_7d"] = ctx.RevenueShare7D
	payload["orders_share_7d"] = ctx.OrdersShare7D
	payload["revenue_rank_7d"] = ctx.RevenueRank7D
	payload["orders_rank_7d"] = ctx.OrdersRank7D
	payload["is_key_sku"] = ctx.IsKeySKU
	payload["key_sku_reasons"] = ctx.KeyReasons
	return payload
}

func isHighPriorityKeySKU(ctx SKUImportanceContext) bool {
	if !ctx.IsKeySKU {
		return false
	}
	if ctx.RevenueShare7D >= 0.10 || ctx.OrdersShare7D >= 0.10 {
		return true
	}
	if ctx.RevenueRank7D > 0 && ctx.RevenueRank7D <= 3 {
		return true
	}
	return ctx.OrdersRank7D > 0 && ctx.OrdersRank7D <= 3
}
