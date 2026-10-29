package aidatapack

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
)

// loadContext is an alias for the shared AnalysisDataSnapshot.
type loadContext = analysisdata.AnalysisDataSnapshot

func buildAccountRollup(b *Builder, ctx *loadContext) AccountRollupTable {
	byDate := map[string]accountMetricDay{}
	for _, row := range ctx.AccountMetrics {
		d := formatDate(row.MetricDate.Time)
		day := byDate[d]
		day.Revenue += numericToFloat(row.Revenue)
		day.Orders += int64(row.OrdersCount)
		day.UnitsSold += int64(row.UnitsSold.Int32)
		day.Returns += int64(row.ReturnsCount)
		day.Cancels += int64(row.CancelCount)
		if day.Orders > 0 {
			aov := round2(day.Revenue / float64(day.Orders))
			day.AOV = &aov
		}
		byDate[d] = day
	}
	if len(byDate) == 0 {
		return AccountRollupTable{
			DataStatus: "not_available",
			Reason:     "daily_account_metrics rows not found for data window",
		}
	}

	w := b.Windows()
	current := byDate[w.CurrentDay]
	yesterday := byDate[w.Yesterday]
	last7 := collectWindowDays(byDate, w.Last7d)
	prev7 := collectWindowDays(byDate, w.Previous7d)
	last7Sum := sumAccountMetrics(last7)
	prev7Sum := sumAccountMetrics(prev7)

	table := AccountRollupTable{
		DataStatus: "available",
		CurrentDay: accountDayMap(current),
		Yesterday:  accountDayMap(yesterday),
		Last7d:     accountDayMap(last7Sum),
		Previous7d: accountDayMap(prev7Sum),
	}
	revAbs, revPct := pctDelta(last7Sum.Revenue, prev7Sum.Revenue)
	ordAbs, ordPct := pctDelta(float64(last7Sum.Orders), float64(prev7Sum.Orders))
	table.Deltas = map[string]any{
		"revenue_last_7d_vs_previous_7d_abs": revAbs,
		"orders_last_7d_vs_previous_7d_abs":  ordAbs,
	}
	if revPct != nil {
		table.Deltas["revenue_last_7d_vs_previous_7d_pct"] = *revPct
	}
	if ordPct != nil {
		table.Deltas["orders_last_7d_vs_previous_7d_pct"] = *ordPct
	}

	var factIDs []string
	if id, err := b.AddFact(Fact{
		FactID: "account.current_day.revenue", EntityType: "account", EntityID: "account:seller",
		Metric: "revenue", Window: "current_day", Date: w.CurrentDay,
		Value: round2(current.Revenue), Unit: UnitRUB,
		Source: FactSource{Table: "daily_account_metrics", Field: "revenue", Calculation: "value on current_day"},
	}); err == nil {
		factIDs = append(factIDs, id)
	}
	if id, err := b.AddFact(Fact{
		FactID: "account.last_7d.revenue", EntityType: "account", EntityID: "account:seller",
		Metric: "revenue", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
		Value: round2(last7Sum.Revenue), Unit: UnitRUB,
		Source: FactSource{Table: "daily_account_metrics", Field: "revenue", Calculation: "sum over last_7d"},
	}); err == nil {
		factIDs = append(factIDs, id)
	}
	if id, err := b.AddFact(Fact{
		FactID: "account.previous_7d.revenue", EntityType: "account", EntityID: "account:seller",
		Metric: "revenue", Window: "previous_7d", DateFrom: w.Previous7d.From, DateTo: w.Previous7d.To,
		Value: round2(prev7Sum.Revenue), Unit: UnitRUB,
		Source: FactSource{Table: "daily_account_metrics", Field: "revenue", Calculation: "sum over previous_7d"},
	}); err == nil {
		factIDs = append(factIDs, id)
	}
	if id, err := b.AddFact(Fact{
		FactID: "account.last_7d.orders", EntityType: "account", EntityID: "account:seller",
		Metric: "orders", Window: "last_7d", DateFrom: w.Last7d.From, DateTo: w.Last7d.To,
		Value: last7Sum.Orders, Unit: UnitCount,
		Source: FactSource{Table: "daily_account_metrics", Field: "orders_count", Calculation: "sum over last_7d"},
	}); err == nil {
		factIDs = append(factIDs, id)
	}
	if id, err := b.AddFact(Fact{
		FactID: "account.previous_7d.orders", EntityType: "account", EntityID: "account:seller",
		Metric: "orders", Window: "previous_7d", DateFrom: w.Previous7d.From, DateTo: w.Previous7d.To,
		Value: prev7Sum.Orders, Unit: UnitCount,
		Source: FactSource{Table: "daily_account_metrics", Field: "orders_count", Calculation: "sum over previous_7d"},
	}); err == nil {
		factIDs = append(factIDs, id)
	}
	if revPct != nil {
		if id, err := b.AddFact(Fact{
			FactID: "account.revenue.last_7d_vs_previous_7d_delta_pct", EntityType: "account", EntityID: "account:seller",
			Metric: "revenue_delta_pct", Window: "last_7d_vs_previous_7d",
			DateFrom: w.Previous7d.From, DateTo: w.Last7d.To,
			Value: *revPct, Unit: UnitPercent,
			Source: FactSource{Table: "daily_account_metrics", Field: "revenue", Calculation: "pct delta last_7d vs previous_7d"},
		}); err == nil {
			factIDs = append(factIDs, id)
		}
	}
	if ordPct != nil {
		if id, err := b.AddFact(Fact{
			FactID: "account.orders.last_7d_vs_previous_7d_delta_pct", EntityType: "account", EntityID: "account:seller",
			Metric: "orders_delta_pct", Window: "last_7d_vs_previous_7d",
			DateFrom: w.Previous7d.From, DateTo: w.Last7d.To,
			Value: *ordPct, Unit: UnitPercent,
			Source: FactSource{Table: "daily_account_metrics", Field: "orders_count", Calculation: "pct delta last_7d vs previous_7d"},
		}); err == nil {
			factIDs = append(factIDs, id)
		}
	}
	if ordAbs != 0 || last7Sum.Orders > 0 || prev7Sum.Orders > 0 {
		if id, err := b.AddFact(Fact{
			FactID: "account.orders.last_7d_vs_previous_7d_delta_abs", EntityType: "account", EntityID: "account:seller",
			Metric: "orders_delta_abs", Window: "last_7d_vs_previous_7d",
			DateFrom: w.Previous7d.From, DateTo: w.Last7d.To,
			Value: ordAbs, Unit: UnitCount,
			Source: FactSource{Table: "daily_account_metrics", Field: "orders_count", Calculation: "abs delta last_7d vs previous_7d"},
		}); err == nil {
			factIDs = append(factIDs, id)
		}
	}
	table.FactIDs = factIDs
	return table
}

func collectWindowDays(byDate map[string]accountMetricDay, r DateRange) []accountMetricDay {
	var out []accountMetricDay
	from := parseDate(r.From)
	to := parseDate(r.To)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if day, ok := byDate[formatDate(d)]; ok {
			out = append(out, day)
		}
	}
	return out
}

func accountDayMap(d accountMetricDay) map[string]any {
	m := map[string]any{
		"revenue":       round2(d.Revenue),
		"orders_count":  d.Orders,
		"units_sold":    d.UnitsSold,
		"returns_count": d.Returns,
		"cancel_count":  d.Cancels,
	}
	if d.AOV != nil {
		m["aov"] = *d.AOV
	}
	return m
}
