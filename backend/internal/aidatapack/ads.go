package aidatapack

import (
	"encoding/json"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func buildCampaignRollups(b *Builder, ctx *loadContext) []CampaignRollupRow {
	metricsByCampaign := groupAdMetrics(ctx.AdMetrics)
	campaignByID := map[int64]dbgen.AdCampaign{}
	for _, c := range ctx.Campaigns {
		campaignByID[c.CampaignExternalID] = c
	}

	ids := make([]int64, 0, len(campaignByID))
	for id := range campaignByID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	w := b.Windows()
	rows := make([]CampaignRollupRow, 0, len(ids))
	for _, cid := range ids {
		c := campaignByID[cid]
		extID := int64ToString(cid)
		eid := campaignEntityID(extID)
		byDay := metricsByCampaign[cid]

		current := sumAdDays(byDay, w.CurrentDay, w.CurrentDay)
		last7 := sumAdDays(byDay, w.Last7d.From, w.Last7d.To)
		prev7 := sumAdDays(byDay, w.Previous7d.From, w.Previous7d.To)
		last14 := sumAdDays(byDay, w.Last14d.From, w.Last14d.To)

		var placement []string
		if len(c.PlacementJson) > 0 {
			_ = json.Unmarshal(c.PlacementJson, &placement)
		}
		budget := CampaignBudget{Currency: "RUB"}
		if c.BudgetDaily.Valid {
			v := numericToFloat(c.BudgetDaily)
			budget.DailyAmount = &v
		}
		if c.BudgetWeekly.Valid {
			v := numericToFloat(c.BudgetWeekly)
			budget.WeeklyAmount = &v
		}
		if c.BudgetAmount.Valid {
			v := numericToFloat(c.BudgetAmount)
			budget.TotalAmount = &v
		}

		row := CampaignRollupRow{
			EntityID:            eid,
			CampaignExternalID:  extID,
			Name:                c.CampaignName,
			State:               textValue(c.Status),
			PaymentType:         textValue(c.PaymentType),
			AdvObjectType:       textValue(c.CampaignType),
			ProductCampaignMode: textValue(c.ProductCampaignMode),
			Placement:           placement,
			Budget:              budget,
			CurrentDay:          adAggToWindow(current),
			Last7d:              adAggToWindow(last7),
			Previous7d:          adAggToWindow(prev7),
			Last14d:             adAggToWindow(last14),
		}

		spendAbs, spendPct := pctDelta(last7.Spend, prev7.Spend)
		revAbs, revPct := pctDelta(last7.Revenue, prev7.Revenue)
		row.Deltas = map[string]any{
			"spend_abs":   spendAbs,
			"revenue_abs": revAbs,
		}
		if spendPct != nil {
			row.Deltas["spend_pct"] = *spendPct
		}
		if revPct != nil {
			row.Deltas["revenue_pct"] = *revPct
		}
		row.DailySeries = buildCampaignDailySeries(byDay, w.Last14d.From, w.Last14d.To)

		var factIDs []string
		if last7.Spend > 0 || last7.Impressions > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "campaign." + extID + ".last_7d.spend", EntityType: "campaign", EntityID: eid,
				Metric: "spend", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: round2(last7.Spend), Unit: UnitRUB,
				Source: FactSource{Table: "ad_metrics_daily", Field: "spend", Calculation: "sum over last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if last7.Revenue > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "campaign." + extID + ".last_7d.revenue", EntityType: "campaign", EntityID: eid,
				Metric: "revenue", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: round2(last7.Revenue), Unit: UnitRUB,
				Source: FactSource{Table: "ad_metrics_daily", Field: "revenue", Calculation: "sum over last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if last7.Orders > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "campaign." + extID + ".last_7d.orders", EntityType: "campaign", EntityID: eid,
				Metric: "orders", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: last7.Orders, Unit: UnitCount,
				Source: FactSource{Table: "ad_metrics_daily", Field: "orders_count", Calculation: "sum over last_7d"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if last7.DRRPct != nil {
			if id, err := b.AddFact(Fact{
				FactID: "campaign." + extID + ".last_7d.drr_pct", EntityType: "campaign", EntityID: eid,
				Metric: "drr_pct", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
				Value: *last7.DRRPct, Unit: UnitPercent,
				Source: FactSource{Table: "ad_metrics_daily", Field: "spend,revenue", Calculation: "spend/revenue*100"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if current.Spend > 0 {
			if id, err := b.AddFact(Fact{
				FactID: "campaign." + extID + ".current_day.spend", EntityType: "campaign", EntityID: eid,
				Metric: "spend", Window: "current_day", Date: w.CurrentDay,
				Value: round2(current.Spend), Unit: UnitRUB,
				Source: FactSource{Table: "ad_metrics_daily", Field: "spend"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		if budget.DailyAmount != nil {
			if id, err := b.AddFact(Fact{
				FactID: "campaign." + extID + ".budget.daily_amount", EntityType: "campaign", EntityID: eid,
				Metric: "daily_amount", Value: *budget.DailyAmount, Unit: UnitRUB,
				Source: FactSource{Table: "ad_campaigns", Field: "budget_daily"},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
		row.FactIDs = factIDs
		rows = append(rows, row)
	}
	return rows
}

func buildCampaignProductLinks(ctx *loadContext, known map[string]*skuMeta, stockBySKU map[string]StockPositionRow, enrichmentBySKU map[string]ProductEnrichmentRow) []CampaignProductLinkRow {
	rows := make([]CampaignProductLinkRow, 0, len(ctx.CampaignSKUs))
	seen := map[string]struct{}{}
	for _, link := range ctx.CampaignSKUs {
		extID := int64ToString(link.CampaignExternalID)
		sku := ""
		if link.Sku.Valid {
			sku = int64ToString(link.Sku.Int64)
		}
		dedupeKey := extID + ":" + sku
		if _, dup := seen[dedupeKey]; dup {
			continue
		}
		seen[dedupeKey] = struct{}{}
		campaignEID := campaignEntityID(extID)
		row := CampaignProductLinkRow{
			CampaignEntityID:   campaignEID,
			CampaignExternalID: extID,
			SKU:                sku,
			OfferID:            textValue(link.OfferID),
			ProductTitle:       textValue(link.ProductTitle),
			IsActive:           link.IsActive,
			Status:             textValue(link.Status),
			EnrichmentStatus:   "available",
		}
		if sku != "" {
			row.SKUEntityID = skuEntityID(sku)
			if _, ok := known[sku]; !ok {
				row.Partial = true
				row.EnrichmentStatus = "partial"
			}
		}
		row.Bid = buildCampaignBid(link, ctx)
		if stock, ok := stockBySKU[sku]; ok && stock.DataStatus == "available" {
			row.LinkedStockSummary = map[string]any{
				"turnover_grade":        stock.TurnoverGrade,
				"idc_days":              nullableFloat(stock.IDCDays),
				"available_stock_count": stock.AvailableStockCount,
			}
		}
		if enrich, ok := enrichmentBySKU[sku]; ok && enrich.DataStatus == "available" {
			row.LinkedProductStatus = map[string]any{
				"visibility_has_stock": enrich.Visibility["has_stock"],
				"validation_status":    enrich.Status["validation_status"],
				"price_index_color":    enrich.PriceIndexes["color"],
			}
		} else if row.EnrichmentStatus == "available" {
			row.EnrichmentStatus = "partial"
		}
		rows = append(rows, row)
	}
	return rows
}

func buildCampaignBid(link dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow, ctx *loadContext) *CampaignBid {
	autoStrategy := false
	for _, c := range ctx.Campaigns {
		if c.CampaignExternalID == link.CampaignExternalID {
			strategy := textValue(c.ProductAutopilotStrategy)
			if strategy != "" && strategy != "NO_AUTO_STRATEGY" {
				autoStrategy = true
			}
			break
		}
	}
	if link.BidAmount.Valid || link.BidRawMicro.Valid {
		bid := &CampaignBid{UnitAssumption: textValue(link.BidUnitAssumption), Applicability: "applicable"}
		if bid.UnitAssumption == "" {
			bid.UnitAssumption = "micro_rubles"
		}
		if link.BidAmount.Valid {
			v := numericToFloat(link.BidAmount)
			bid.Amount = &v
		}
		if link.BidRawMicro.Valid {
			v := link.BidRawMicro.Int64
			bid.RawMicro = &v
		}
		return bid
	}
	if autoStrategy {
		return &CampaignBid{Applicability: "not_applicable_auto_strategy"}
	}
	return nil
}

func groupAdMetrics(rows []dbgen.AdMetricsDaily) map[int64]map[string]adMetricDay {
	out := map[int64]map[string]adMetricDay{}
	for _, row := range rows {
		d := formatDate(row.MetricDate.Time)
		if out[row.CampaignExternalID] == nil {
			out[row.CampaignExternalID] = map[string]adMetricDay{}
		}
		day := out[row.CampaignExternalID][d]
		day.Impressions += row.Impressions
		day.Clicks += row.Clicks
		day.Spend += numericToFloat(row.Spend)
		day.Orders += int64(row.OrdersCount)
		day.Revenue += numericToFloat(row.Revenue)
		out[row.CampaignExternalID][d] = day
	}
	return out
}

func sumAdDays(byDay map[string]adMetricDay, from, to string) adMetricAgg {
	var days []adMetricDay
	start := parseDate(from)
	end := parseDate(to)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if day, ok := byDay[formatDate(d)]; ok {
			days = append(days, day)
		}
	}
	var raw []adMetricDay
	raw = days
	return aggregateAdMetrics(raw)
}

func buildCampaignDailySeries(byDay map[string]adMetricDay, from, to string) []CampaignDailyPoint {
	start := parseDate(from)
	end := parseDate(to)
	out := make([]CampaignDailyPoint, 0, 14)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := formatDate(d)
		day, ok := byDay[key]
		if !ok {
			out = append(out, CampaignDailyPoint{Date: key, CampaignMetricsWindow: adAggToWindow(adMetricAgg{})})
			continue
		}
		out = append(out, CampaignDailyPoint{Date: key, CampaignMetricsWindow: adAggToWindow(aggregateAdMetrics([]adMetricDay{day}))})
	}
	return out
}

func stockPositionsIndex(rows []StockPositionRow) map[string]StockPositionRow {
	out := make(map[string]StockPositionRow, len(rows))
	for _, r := range rows {
		out[stringsTrimSKU(r.EntityID)] = r
	}
	return out
}

func enrichmentIndex(rows []ProductEnrichmentRow) map[string]ProductEnrichmentRow {
	out := make(map[string]ProductEnrichmentRow, len(rows))
	for _, r := range rows {
		// extract sku from entity_id
		sku := stringsTrimSKU(r.EntityID)
		out[sku] = r
	}
	return out
}

func stringsTrimSKU(entityID string) string {
	const p = "sku:"
	if len(entityID) > len(p) && entityID[:len(p)] == p {
		return entityID[len(p):]
	}
	return entityID
}
