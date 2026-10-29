package aidatapack

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

var funnelStages = []string{"impressions", "product_views", "add_to_cart", "orders_count", "completed_orders"}

func buildFunnelRollup(b *Builder, ctx *loadContext) FunnelRollupTable {
	w := b.Windows()
	byDate := map[string]dbgen.ProductAnalyticsDaily{}
	for _, row := range ctx.FunnelRows {
		d := formatDate(row.MetricDate.Time)
		if _, exists := byDate[d]; exists {
			continue
		}
		byDate[d] = row
	}
	current, ok := byDate[w.CurrentDay]
	if !ok {
		return FunnelRollupTable{
			DataStatus: "not_available",
			Reason:     "product_analytics_daily account scope row not found",
		}
	}

	scopeID := current.ScopeID
	sourceScopeID := scopeID
	if scopeID == "" {
		scopeID = "account"
	}

	stages := funnelStageMap(current)
	transitions := funnelTransitions(stages)

	table := FunnelRollupTable{
		DataStatus: "available",
		Scope: map[string]any{
			"type":            "account",
			"id":              scopeID,
			"source_scope_id": sourceScopeID,
		},
		CurrentDay: map[string]any{
			"date":          w.CurrentDay,
			"source_status": textValue(current.SourceStatus),
			"stages":        stages,
			"transitions":   transitions,
		},
		Last7d: buildFunnelLast7d(byDate, w),
	}

	var factIDs []string
	if v, ok := stages["impressions"]; ok && v > 0 {
		if id, err := b.AddFact(Fact{
			FactID: "funnel.account.current_day.impressions", EntityType: "funnel", EntityID: "funnel:account",
			Metric: "impressions", Window: "current_day", Date: w.CurrentDay, Value: v, Unit: UnitCount,
			Source: FactSource{Table: "product_analytics_daily", Field: "impressions"},
		}); err == nil {
			factIDs = append(factIDs, id)
		}
	}
	for _, key := range []string{"product_views", "add_to_cart", "orders_count", "completed_orders"} {
		if v, ok := stages[key]; ok && v > 0 {
			factID := "funnel.account.current_day." + key
			if id, err := b.AddFact(Fact{
				FactID: factID, EntityType: "funnel", EntityID: "funnel:account",
				Metric: key, Window: "current_day", Date: w.CurrentDay, Value: v, Unit: UnitCount,
				Source: FactSource{Table: "product_analytics_daily", Field: key},
			}); err == nil {
				factIDs = append(factIDs, id)
			}
		}
	}
	for _, tr := range transitions {
		if name, ok := tr["name"].(string); ok {
			if pct, ok := tr["conversion_pct"].(float64); ok {
				safeName := funnelTransitionFactName(name)
				if id, err := b.AddFact(Fact{
					FactID:     "funnel.account.transition." + safeName + ".conversion_pct",
					EntityType: "funnel", EntityID: "funnel:account",
					Metric: "conversion_pct", Window: "current_day", Date: w.CurrentDay,
					Value: pct, Unit: UnitPercent,
					Source: FactSource{Table: "product_analytics_daily", Field: safeName, Calculation: "stage conversion"},
				}); err == nil {
					factIDs = append(factIDs, id)
				}
			}
		}
	}
	table.FactIDs = factIDs
	return table
}

func funnelTransitionFactName(name string) string {
	switch name {
	case "impressions_to_product_views":
		return "impressions_to_product_views"
	case "product_views_to_add_to_cart":
		return "product_views_to_add_to_cart"
	case "add_to_cart_to_orders_count":
		return "add_to_cart_to_orders_count"
	case "orders_count_to_completed_orders":
		return "orders_count_to_completed_orders"
	default:
		return name
	}
}

func funnelStageMap(row dbgen.ProductAnalyticsDaily) map[string]int64 {
	return map[string]int64{
		"impressions":      pgInt8(row.Impressions),
		"product_views":    pgInt8(row.ProductViews),
		"add_to_cart":      pgInt8(row.AddToCart),
		"orders_count":     pgInt8(row.OrdersCount),
		"completed_orders": pgInt8(row.CompletedOrders),
	}
}

func pgInt8(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func funnelTransitions(stages map[string]int64) []map[string]any {
	pairs := [][2]string{
		{"impressions", "product_views"},
		{"product_views", "add_to_cart"},
		{"add_to_cart", "orders_count"},
		{"orders_count", "completed_orders"},
	}
	var out []map[string]any
	for _, p := range pairs {
		from := stages[p[0]]
		to := stages[p[1]]
		tr := map[string]any{
			"name":       p[0] + "_to_" + p[1],
			"from":       p[0],
			"to":         p[1],
			"from_value": from,
			"to_value":   to,
		}
		if from > 0 {
			pct := round2(float64(to) / float64(from) * 100)
			tr["conversion_pct"] = pct
		}
		out = append(out, tr)
	}
	return out
}

func buildFunnelLast7d(byDate map[string]dbgen.ProductAnalyticsDaily, w DataWindows) map[string]any {
	totals := map[string]int64{}
	var transitionSums map[string]float64
	transitionCounts := map[string]int{}
	from := parseDate(w.Last7d.From)
	to := parseDate(w.Last7d.To)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		row, ok := byDate[formatDate(d)]
		if !ok {
			continue
		}
		st := funnelStageMap(row)
		for k, v := range st {
			totals[k] += v
		}
		for _, tr := range funnelTransitions(st) {
			if pct, ok := tr["conversion_pct"].(float64); ok {
				name := tr["name"].(string)
				if transitionSums == nil {
					transitionSums = map[string]float64{}
				}
				transitionSums[name] += pct
				transitionCounts[name]++
			}
		}
	}
	stageTotals := map[string]any{}
	for _, k := range funnelStages {
		stageTotals[k] = totals[k]
	}
	avgTransitions := map[string]any{}
	for name, sum := range transitionSums {
		if c := transitionCounts[name]; c > 0 {
			avgTransitions[name] = round2(sum / float64(c))
		}
	}
	return map[string]any{
		"stage_totals":        stageTotals,
		"transition_averages": avgTransitions,
	}
}

// pickAccountFunnelRows filters account-scope funnel rows preferring empty scope_id.
func pickAccountFunnelRows(rows []dbgen.ProductAnalyticsDaily) []dbgen.ProductAnalyticsDaily {
	return analysisdata.SelectAccountFunnelRows(rows)
}
