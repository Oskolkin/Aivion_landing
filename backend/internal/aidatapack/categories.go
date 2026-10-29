package aidatapack

import "strings"

func buildCategoryRollups(b *Builder, ctx *loadContext, known map[string]*skuMeta, categories map[string]CategoryEntity, stockLites []stockSnapshotLite) []CategoryRollupRow {
	skuToCategory := map[string]string{}
	for sku, m := range known {
		if m.CategoryID != "" {
			skuToCategory[sku] = m.CategoryID
		}
	}

	type agg struct {
		activeSKUs     map[string]struct{}
		withStock      map[string]struct{}
		revenue7d      float64
		orders7d       int64
		units7d        int64
		deficit        int
		noSales        int
		surplus        int
		campaignLinked map[string]struct{}
	}
	byCat := map[string]*agg{}
	getAgg := func(catID string) *agg {
		if byCat[catID] == nil {
			byCat[catID] = &agg{
				activeSKUs:     map[string]struct{}{},
				withStock:      map[string]struct{}{},
				campaignLinked: map[string]struct{}{},
			}
		}
		return byCat[catID]
	}

	for sku, catEID := range skuToCategory {
		a := getAgg(catEID)
		a.activeSKUs[sku] = struct{}{}
	}

	w := ctx.Windows
	for _, row := range ctx.SKUMetrics {
		if !row.Sku.Valid {
			continue
		}
		sku := int64ToString(row.Sku.Int64)
		catEID, ok := skuToCategory[sku]
		if !ok {
			continue
		}
		d := formatDate(row.MetricDate.Time)
		if windowContains(w, "last_7d", d) {
			a := getAgg(catEID)
			a.revenue7d += numericToFloat(row.Revenue)
			a.orders7d += int64(row.OrdersCount)
			if row.UnitsSold.Valid {
				a.units7d += int64(row.UnitsSold.Int32)
			}
		}
		if d == w.CurrentDay && row.StockAvailable > 0 {
			getAgg(catEID).withStock[sku] = struct{}{}
		}
	}

	stockBySKU := map[string]aggregatedStock{}
	for sku := range known {
		stockBySKU[sku] = aggregateStockFromSnapshots(stockLites, sku)
	}
	for sku, st := range stockBySKU {
		catEID, ok := skuToCategory[sku]
		if !ok {
			continue
		}
		a := getAgg(catEID)
		switch st.TurnoverGrade {
		case "DEFICIT":
			a.deficit++
		case "NO_SALES":
			a.noSales++
		case "SURPLUS":
			a.surplus++
		}
	}

	for _, link := range ctx.CampaignSKUs {
		if !link.Sku.Valid {
			continue
		}
		sku := int64ToString(link.Sku.Int64)
		if catEID, ok := skuToCategory[sku]; ok {
			getAgg(catEID).campaignLinked[sku] = struct{}{}
		}
	}

	rows := make([]CategoryRollupRow, 0, len(byCat))
	for catEID, a := range byCat {
		catID := strings.TrimPrefix(catEID, "category:")
		ent, ok := categories[catEID]
		name := "Category " + catID
		var path *string
		if ok {
			name = ent.Name
			path = ent.Path
		}
		row := CategoryRollupRow{
			EntityID:               catEID,
			CategoryID:             catID,
			Name:                   name,
			Path:                   path,
			ActiveSKUCount:         len(a.activeSKUs),
			ProductsWithStockCount: len(a.withStock),
			Revenue7d:              round2(a.revenue7d),
			Orders7d:               a.orders7d,
			Units7d:                a.units7d,
			DeficitSKUCount:        a.deficit,
			NoSalesSKUCount:        a.noSales,
			SurplusSKUCount:        a.surplus,
			CampaignLinkedSKUCount: len(a.campaignLinked),
		}
		var factIDs []string
		if a.revenue7d > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "category." + catID + ".last_7d.revenue", EntityType: "category", EntityID: catEID,
				Metric: "revenue", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: round2(a.revenue7d), Unit: UnitRUB,
				Source: FactSource{Table: "daily_sku_metrics", Field: "revenue", Calculation: "sum by category last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if a.orders7d > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "category." + catID + ".last_7d.orders", EntityType: "category", EntityID: catEID,
				Metric: "orders", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: a.orders7d, Unit: UnitCount,
				Source: FactSource{Table: "daily_sku_metrics", Field: "orders_count", Calculation: "sum by category last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		row.FactIDs = factIDs
		rows = append(rows, row)
	}
	return rows
}
