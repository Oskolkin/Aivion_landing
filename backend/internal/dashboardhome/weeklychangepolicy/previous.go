package weeklychangepolicy

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

// BuildPreviousContextFromFacts builds continuity context from a prior facts snapshot.
func BuildPreviousContextFromFacts(previous dashboardfacts.DashboardFactsV1) PreviousContext {
	ctx := PreviousContext{
		HistoryAvailable: true,
		HasPreviousFacts: true,
		PeriodLabel:      previous.MetricDate,
		Problems:         map[string]Problem{},
		Metrics:          map[string]float64{},
	}
	if p := previous.AdEfficiency.Period; p != nil && p.DateFrom != "" && p.DateTo != "" {
		ctx.PeriodLabel = p.DateFrom + "–" + p.DateTo
	}
	for _, c := range extractConditions(previous) {
		ctx.Metrics[c.metricCode] = c.value
		if c.isProblem && c.present {
			ctx.Problems[c.metricCode] = Problem{
				MetricCode: c.metricCode,
				DriverID:   c.relatedDriver,
				EntityIDs:  append([]string{}, c.entityIDs...),
				Value:      c.value,
				Present:    true,
			}
		}
	}
	return ctx
}

// BuildPreviousContextFromFingerprints builds continuity from prior publication
// problem fingerprints (e.g. health risk driver IDs / metric codes).
func BuildPreviousContextFromFingerprints(periodLabel string, fingerprints []Problem) PreviousContext {
	ctx := PreviousContext{
		PeriodLabel: strings.TrimSpace(periodLabel),
		Problems:    map[string]Problem{},
		Metrics:     map[string]float64{},
	}
	for _, fp := range fingerprints {
		code := strings.TrimSpace(fp.MetricCode)
		if code == "" && fp.DriverID != "" {
			code = metricCodeForDriver(fp.DriverID)
		}
		if code == "" {
			continue
		}
		fp.MetricCode = code
		fp.Present = true
		ctx.Problems[code] = fp
		if fp.Value > 0 {
			ctx.Metrics[code] = fp.Value
		} else {
			ctx.Metrics[code] = 1
		}
	}
	if len(ctx.Problems) > 0 {
		ctx.HistoryAvailable = true
		ctx.HasPreviousPublication = true
	}
	return ctx
}

// EnrichPreviousContextWithImpliedDeltas adds previous KPI values reverse-engineered
// from current week-over-week deltas when a full previous facts snapshot is absent.
// This enables worsened/improved for sales without claiming problem continuity
// (does NOT set HistoryAvailable).
func EnrichPreviousContextWithImpliedDeltas(ctx PreviousContext, current dashboardfacts.DashboardFactsV1) PreviousContext {
	if ctx.Metrics == nil {
		ctx.Metrics = map[string]float64{}
	}
	account := current.DashboardMetrics.Account
	if _, ok := ctx.Metrics[MetricRevenue7d]; !ok && account.PreviousWeekTotal > 0 {
		ctx.Metrics[MetricRevenue7d] = account.PreviousWeekTotal
	}
	if _, ok := ctx.Metrics[MetricRevenue7d]; !ok && account.RevenueLast7Days > 0 && account.RevenueWeekDelta.Pct != nil {
		pct := *account.RevenueWeekDelta.Pct
		if pct > -0.999 {
			ctx.Metrics[MetricRevenue7d] = account.RevenueLast7Days / (1 + pct)
		}
	}
	// NOTE: Seller Pulse KPI deltas are day-over-day and must NOT backfill
	// weekly change metrics (sales.revenue_7d / sales.orders_7d).

	for _, stage := range current.Funnel.Stages {
		key := strings.ToLower(strings.TrimSpace(stage.Key))
		var code string
		switch key {
		case "add_to_cart", "cart", "view_to_cart":
			code = MetricFunnelViewToCart
		case "orders", "order", "cart_to_order":
			code = MetricFunnelCartToOrder
		default:
			continue
		}
		if stage.PreviousConversionFromPrevious == nil {
			continue
		}
		if _, ok := ctx.Metrics[code]; !ok {
			ctx.Metrics[code] = *stage.PreviousConversionFromPrevious
		}
	}

	if summary := current.AdEfficiency.Summary; summary != nil && summary.DeltaVsPreviousPp != nil {
		if summary.TotalStoreRevenue7d > 0 {
			cur := (summary.TotalSpend7d / summary.TotalStoreRevenue7d) * 100
			prev := cur - *summary.DeltaVsPreviousPp
			if _, ok := ctx.Metrics[MetricAdShareOfRevenue]; !ok {
				ctx.Metrics[MetricAdShareOfRevenue] = prev
			}
		}
	}
	return ctx
}

func metricCodeForDriver(driverID string) string {
	switch strings.TrimSpace(driverID) {
	case "stockout":
		return MetricStockoutSKUCount
	case "key_sku_stock_risk":
		return MetricCriticalSKUCount
	case "no_sales_with_stock":
		return MetricNoSalesWithStock
	case "surplus":
		return MetricSurplusSKUCount
	case "high_drr_campaign":
		return MetricHighDRRCampaigns
	case "campaign_spend_without_orders":
		return MetricSpendWithoutOrders
	case "view_to_cart_decline":
		return MetricFunnelViewToCart
	case "cart_to_order_decline":
		return MetricFunnelCartToOrder
	case "revenue_decline":
		return MetricRevenue7d
	case "sales_decline":
		return MetricOrders7d
	default:
		return ""
	}
}

// MergePreviousContext prefers earlier parts; later parts fill gaps.
func MergePreviousContext(parts ...PreviousContext) PreviousContext {
	out := PreviousContext{
		Problems: map[string]Problem{},
		Metrics:  map[string]float64{},
	}
	for _, p := range parts {
		if p.HasPreviousPublication {
			out.HasPreviousPublication = true
		}
		if p.HasPreviousFacts {
			out.HasPreviousFacts = true
		}
		if p.HistoryAvailable {
			out.HistoryAvailable = true
		}
		if out.PeriodLabel == "" && p.PeriodLabel != "" {
			out.PeriodLabel = p.PeriodLabel
		}
		for k, v := range p.Metrics {
			if _, ok := out.Metrics[k]; !ok {
				out.Metrics[k] = v
			}
		}
		for k, v := range p.Problems {
			if _, ok := out.Problems[k]; !ok {
				out.Problems[k] = v
			}
		}
	}
	out.HistoryAvailable = out.HasPreviousPublication || out.HasPreviousFacts || out.HistoryAvailable
	return out
}
