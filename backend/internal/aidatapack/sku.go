package aidatapack

import (
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// allSKUsStableOrder returns every known SKU in stable entity_id (sku string) ascending order.
// No business scoring or top-N selection — full normalized dataset for chunking.
func allSKUsStableOrder(known map[string]*skuMeta) ([]string, error) {
	all := make([]string, 0, len(known))
	for sku := range known {
		all = append(all, sku)
	}
	if len(all) > MaxSKUsTechnicalLimit {
		return nil, fmt.Errorf("too_large_for_single_pack: %d SKUs exceeds technical limit %d", len(all), MaxSKUsTechnicalLimit)
	}
	sort.Strings(all)
	return all, nil
}

func buildSKURollups(b *Builder, ctx *loadContext, known map[string]*skuMeta, selected []string) []SKURollupRow {
	metricsBySKU := groupSKUMetrics(ctx.SKUMetrics)
	enrichmentSKUs := map[string]struct{}{}
	for _, snap := range ctx.ProductInfoSnapshots {
		if snap.Sku.Valid {
			enrichmentSKUs[int64ToString(snap.Sku.Int64)] = struct{}{}
		}
	}

	var totalRev7d, totalOrd7d float64
	for _, sku := range selected {
		for d, m := range metricsBySKU[sku] {
			if windowContains(ctx.Windows, "last_7d", d) {
				totalRev7d += m.Revenue
				totalOrd7d += float64(m.Orders)
			}
		}
	}

	rows := make([]SKURollupRow, 0, len(selected))
	w := b.Windows()
	for _, sku := range selected {
		meta := known[sku]
		byDay := metricsBySKU[sku]
		current := byDay[w.CurrentDay]
		last7 := sumSKUWindow(byDay, w.Last7d)
		prev7 := sumSKUWindow(byDay, w.Previous7d)

		activeDays := int64(0)
		for d, m := range byDay {
			if windowContains(w, "last_7d", d) && m.Orders > 0 {
				activeDays++
			}
		}
		var avgDaily *float64
		if activeDays > 0 {
			v := round2(float64(last7.Orders) / 7.0)
			avgDaily = &v
		}

		currentDay := map[string]any{
			"revenue":    round2(current.Revenue),
			"orders":     current.Orders,
			"units_sold": current.UnitsSold,
		}
		if current.Stock != nil {
			currentDay["stock_available"] = *current.Stock
			currentDay["stock_available_semantic"] = StockSemanticCurrent
		}
		if current.DaysCover != nil {
			currentDay["days_of_cover"] = *current.DaysCover
		}

		last7Map := map[string]any{
			"revenue":           round2(last7.Revenue),
			"orders":            last7.Orders,
			"units_sold":        last7.UnitsSold,
			"active_sales_days": activeDays,
		}
		if avgDaily != nil {
			last7Map["avg_daily_orders"] = *avgDaily
		}

		ordAbs, ordPct := pctDelta(float64(last7.Orders), float64(prev7.Orders))
		revAbs, revPct := pctDelta(last7.Revenue, prev7.Revenue)
		deltas := map[string]any{
			"orders_abs":  int64(ordAbs),
			"revenue_abs": revAbs,
		}
		if ordPct != nil {
			deltas["orders_pct"] = *ordPct
		}
		if revPct != nil {
			deltas["revenue_pct"] = *revPct
		}

		shares := map[string]any{}
		if totalRev7d > 0 {
			shares["revenue_share_7d_pct"] = round2(last7.Revenue / totalRev7d * 100)
		}
		if totalOrd7d > 0 {
			shares["orders_share_7d_pct"] = round2(float64(last7.Orders) / totalOrd7d * 100)
		}

		enrichStatus := "not_available"
		if _, ok := enrichmentSKUs[sku]; ok {
			enrichStatus = "available"
		}

		row := SKURollupRow{
			EntityID:   skuEntityID(sku),
			SKU:        sku,
			OfferID:    meta.OfferID,
			Name:       meta.Name,
			CurrentDay: currentDay,
			Last7d:     last7Map,
			Previous7d: map[string]any{
				"revenue":    round2(prev7.Revenue),
				"orders":     prev7.Orders,
				"units_sold": prev7.UnitsSold,
			},
			Deltas:                  deltas,
			Shares:                  shares,
			ProductEnrichmentStatus: enrichStatus,
		}

		var factIDs []string
		if current.Revenue > 0 || current.Orders > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".current_day.revenue", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "revenue", Window: "current_day", Date: w.CurrentDay,
				Value: round2(current.Revenue), Unit: UnitRUB,
				Source: FactSource{Table: "daily_sku_metrics", Field: "revenue"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if last7.Revenue > 0 || last7.Orders > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".last_7d.revenue", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "revenue", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: round2(last7.Revenue), Unit: UnitRUB,
				Source: FactSource{Table: "daily_sku_metrics", Field: "revenue", Calculation: "sum over last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".last_7d.orders", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "orders", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: last7.Orders, Unit: UnitCount,
				Source: FactSource{Table: "daily_sku_metrics", Field: "orders_count", Calculation: "sum over last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if current.Stock != nil {
			if id, err := b.AddFact(Fact{
				FactID: "sku." + sku + ".current.stock_available", EntityType: "sku", EntityID: skuEntityID(sku),
				Metric: "stock_available", Window: "current_day", Date: w.CurrentDay,
				Value: *current.Stock, Unit: UnitCount,
				Source: FactSource{Table: "daily_sku_metrics", Field: "stock_available", Calculation: "current_snapshot"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		row.FactIDs = factIDs
		rows = append(rows, row)
	}
	return rows
}

func groupSKUMetrics(rows []dbgen.DailySkuMetric) map[string]map[string]skuMetricDay {
	out := map[string]map[string]skuMetricDay{}
	for _, row := range rows {
		if !row.Sku.Valid {
			continue
		}
		sku := int64ToString(row.Sku.Int64)
		d := formatDate(row.MetricDate.Time)
		if out[sku] == nil {
			out[sku] = map[string]skuMetricDay{}
		}
		day := out[sku][d]
		day.Revenue += numericToFloat(row.Revenue)
		day.Orders += int64(row.OrdersCount)
		if row.UnitsSold.Valid {
			day.UnitsSold += int64(row.UnitsSold.Int32)
		}
		stock := row.StockAvailable
		day.Stock = &stock
		day.DaysCover = numericToFloatPtr(row.DaysOfCover)
		out[sku][d] = day
	}
	return out
}

func sumSKUWindow(byDay map[string]skuMetricDay, r DateRange) skuMetricDay {
	var out skuMetricDay
	from := parseDate(r.From)
	to := parseDate(r.To)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if day, ok := byDay[formatDate(d)]; ok {
			out.Revenue += day.Revenue
			out.Orders += day.Orders
			out.UnitsSold += day.UnitsSold
		}
	}
	return out
}
