package healthpolicy

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

// Health policy v1 thresholds. These are versioned constants — any change
// that could move a previously-published score requires bumping
// HealthPolicyVersion.
const (
	revenueDeclineMinDropPct   = 8.0
	salesDeclineMinDropPct     = 8.0
	keySKUCriticalDays         = 3.0
	keySKUShortageDays         = 7.0
	surplusMinDaysWithoutSales = int32(14)
	surplusMinExcessShare      = 0.3
	funnelStageMinDropPct      = 10.0
	adShareGrowthMinPp         = 3.0
)

// BuildCandidateRegistry builds the complete, backend-owned risk candidate
// registry from immutable Dashboard Facts (dashboardfacts.DashboardFactsV1).
// This is the single source powering health_score (via ComputeHealthScore) —
// every candidate here is grounded directly in a fact, independent of what
// the AI output says. The returned slice is unsorted; use SortCandidates or
// ComputeHealthScore/ToRiskDrivers for a deterministic order.
func BuildCandidateRegistry(facts dashboardfacts.DashboardFactsV1) []RiskCandidate {
	builders := []func(dashboardfacts.DashboardFactsV1) (RiskCandidate, bool){
		buildRevenueDeclineCandidate,
		buildSalesDeclineCandidate,
		buildKeySKUStockRiskCandidate,
		buildStockoutCandidate,
		buildNoSalesWithStockCandidate,
		buildSurplusCandidate,
		buildViewToCartDeclineCandidate,
		buildCartToOrderDeclineCandidate,
		buildHighDRRCampaignCandidate,
		buildCampaignSpendWithoutOrdersCandidate,
		buildAdShareGrowthCandidate,
		buildDataLimitationCandidate,
	}
	candidates := make([]RiskCandidate, 0, len(builders))
	for _, build := range builders {
		c, ok := build(facts)
		if !ok {
			continue
		}
		c.Penalty = ComputePenalty(c.Severity, c.ImpactScore)
		candidates = append(candidates, c)
	}
	return candidates
}

func buildRevenueDeclineCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	account := facts.DashboardMetrics.Account
	dayPct := percentFromRatio(account.RevenueDayDelta.Pct)
	weekPct := percentFromRatio(account.RevenueWeekDelta.Pct)
	worst := math.Max(negativeMagnitude(dayPct), negativeMagnitude(weekPct))
	if worst < revenueDeclineMinDropPct {
		return RiskCandidate{}, false
	}
	impact := clampInt(int(math.Round(worst*1.45))+topSKURevenueDropImpact(facts.DashboardMetrics.SKUs), 12, 95)
	severity := severityFromScore(worst, 30, 18, 8)
	data := map[string]any{
		"metric_date":            facts.MetricDate,
		"revenue_today":          account.RevenueToday,
		"revenue_day_delta_pct":  dayPct,
		"revenue_week_delta_pct": weekPct,
	}
	var entityIDs []string
	if top := topRevenueDeclineSKU(facts.DashboardMetrics.SKUs); top != nil {
		data["top_declining_sku_name"] = derefStr(top.ProductName)
		entityIDs = []string{strconv.FormatInt(top.OzonProductID, 10)}
	}
	return RiskCandidate{
		DriverID:           DriverRevenueDecline,
		Domain:             "sales",
		FactIDs:            []string{"dashboard_metrics.account.revenue_day_to_day_delta", "dashboard_metrics.account.revenue_week_to_week_delta"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             facts.MetricDate,
		ReasonTemplateData: data,
	}, true
}

func buildSalesDeclineCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	account := facts.DashboardMetrics.Account
	if account.OrdersYesterday <= 0 {
		return RiskCandidate{}, false
	}
	ordersPct := intChangePercent(account.OrdersToday, account.OrdersYesterday)
	drop := negativeMagnitude(ordersPct)
	if drop < salesDeclineMinDropPct {
		return RiskCandidate{}, false
	}
	impact := clampInt(int(math.Round(drop*1.3)), 10, 90)
	severity := severityFromScore(drop, 30, 18, 8)
	return RiskCandidate{
		DriverID:    DriverSalesDecline,
		Domain:      "sales",
		FactIDs:     []string{"dashboard_metrics.account.orders_today", "dashboard_metrics.account.orders_yesterday"},
		Severity:    severity,
		ImpactScore: impact,
		Period:      facts.MetricDate,
		ReasonTemplateData: map[string]any{
			"metric_date":          facts.MetricDate,
			"orders_today":         account.OrdersToday,
			"orders_yesterday":     account.OrdersYesterday,
			"orders_day_delta_pct": ordersPct,
		},
	}, true
}

func buildKeySKUStockRiskCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	if strings.ToLower(strings.TrimSpace(facts.KeySKUCohort.Status)) != "available" || len(facts.KeySKUCohort.Items) == 0 {
		return RiskCandidate{}, false
	}
	critical, shortage := 0, 0
	var entityIDs []string
	for _, item := range facts.KeySKUCohort.Items {
		rec, ok := skuFactByOzonID(facts.SKUFactsIndex, item.OzonProductID)
		if !ok || rec.DaysOfCover == nil {
			continue
		}
		days := *rec.DaysOfCover
		switch {
		case days <= keySKUCriticalDays:
			critical++
			entityIDs = append(entityIDs, strconv.FormatInt(item.OzonProductID, 10))
		case days <= keySKUShortageDays:
			shortage++
			entityIDs = append(entityIDs, strconv.FormatInt(item.OzonProductID, 10))
		}
	}
	if critical == 0 && shortage == 0 {
		return RiskCandidate{}, false
	}
	impact := clampInt(critical*18+shortage*9, 14, 96)
	severity := severityFromInts(critical, shortage)
	return RiskCandidate{
		DriverID:           DriverKeySKUStockRisk,
		Domain:             "stock",
		FactIDs:            []string{"key_sku_cohort.items[].days_of_cover"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             facts.MetricDate,
		ReasonTemplateData: map[string]any{"critical_key_sku_count": critical, "shortage_key_sku_count": shortage},
	}, true
}

func buildStockoutCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	var entityIDs []string
	for key, rec := range facts.SKUFactsIndex.ByOzonProductID {
		if rec.StockAvailable > 0 {
			continue
		}
		// Only flag SKUs that were actually selling recently — a SKU with
		// zero stock and zero recent orders is not a demand-loss risk
		// (that's covered separately, if at all, by discontinued/no-demand
		// signals outside this policy's scope).
		if rec.Orders7d <= 0 {
			continue
		}
		entityIDs = append(entityIDs, key)
	}
	if len(entityIDs) == 0 {
		return RiskCandidate{}, false
	}
	sort.Strings(entityIDs)
	count := len(entityIDs)
	impact := clampInt(count*20, 15, 95)
	severity := severityFromScore(float64(count), 5, 3, 1)
	return RiskCandidate{
		DriverID:           DriverStockout,
		Domain:             "stock",
		FactIDs:            []string{"sku_facts_index.by_ozon_product_id[].stock_available", "sku_facts_index.by_ozon_product_id[].orders_7d"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             facts.MetricDate,
		ReasonTemplateData: map[string]any{"stockout_sku_count": count},
	}, true
}

func buildNoSalesWithStockCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	var entityIDs []string
	for key, rec := range facts.SKUFactsIndex.ByOzonProductID {
		if rec.StockAvailable <= 0 {
			continue
		}
		if rec.Orders7d > 0 || rec.Orders > 0 {
			continue
		}
		entityIDs = append(entityIDs, key)
	}
	if len(entityIDs) == 0 {
		return RiskCandidate{}, false
	}
	sort.Strings(entityIDs)
	count := len(entityIDs)
	impact := clampInt(count*10, 10, 85)
	severity := severityFromScore(float64(count), 6, 3, 1)
	return RiskCandidate{
		DriverID:           DriverNoSalesWithStock,
		Domain:             "sku",
		FactIDs:            []string{"sku_facts_index.by_ozon_product_id[].orders_7d", "sku_facts_index.by_ozon_product_id[].stock_available"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             facts.MetricDate,
		ReasonTemplateData: map[string]any{"no_sales_with_stock_sku_count": count},
	}, true
}

func buildSurplusCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	var entityIDs []string
	for _, sku := range facts.DashboardMetrics.SKUs {
		slow := sku.StockTurnoverGrade != nil && strings.EqualFold(strings.TrimSpace(*sku.StockTurnoverGrade), "slow")
		excess := sku.ExcessStockShare != nil && *sku.ExcessStockShare >= surplusMinExcessShare
		stale := sku.DaysWithoutSales != nil && *sku.DaysWithoutSales >= surplusMinDaysWithoutSales && sku.StockAvailable > 0
		if !slow && !excess && !stale {
			continue
		}
		entityIDs = append(entityIDs, strconv.FormatInt(sku.OzonProductID, 10))
	}
	if len(entityIDs) == 0 {
		return RiskCandidate{}, false
	}
	count := len(entityIDs)
	impact := clampInt(count*12, 8, 80)
	severity := severityFromScore(float64(count), 5, 3, 1)
	return RiskCandidate{
		DriverID:           DriverSurplus,
		Domain:             "surplus",
		FactIDs:            []string{"dashboard_metrics.skus[].stock_turnover_grade", "dashboard_metrics.skus[].excess_stock_share", "dashboard_metrics.skus[].days_without_sales"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             facts.MetricDate,
		ReasonTemplateData: map[string]any{"surplus_sku_count": count},
	}, true
}

func buildViewToCartDeclineCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	return buildFunnelStageDeclineCandidate(facts, "add_to_cart", DriverViewToCartDecline)
}

func buildCartToOrderDeclineCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	return buildFunnelStageDeclineCandidate(facts, "orders", DriverCartToOrderDecline)
}

func buildFunnelStageDeclineCandidate(facts dashboardfacts.DashboardFactsV1, stageKey string, driverID string) (RiskCandidate, bool) {
	stage, ok := funnelStageByKey(facts, stageKey)
	if !ok || stage.ChangeVsPreviousDayPct == nil {
		return RiskCandidate{}, false
	}
	drop := negativeMagnitude(*stage.ChangeVsPreviousDayPct)
	if drop < funnelStageMinDropPct {
		return RiskCandidate{}, false
	}
	impact := clampInt(int(math.Round(drop*1.35))+15, 10, 90)
	severity := severityFromScore(drop, 35, 20, 10)
	return RiskCandidate{
		DriverID:    driverID,
		Domain:      "funnel",
		FactIDs:     []string{fmt.Sprintf("funnel.stages[%s].change_vs_previous_day_pct", stageKey)},
		Severity:    severity,
		ImpactScore: impact,
		Period:      facts.MetricDate,
		ReasonTemplateData: map[string]any{
			"stage_label":                stage.Label,
			"change_vs_previous_day_pct": *stage.ChangeVsPreviousDayPct,
		},
	}, true
}

func buildHighDRRCampaignCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	summary := facts.AdEfficiency.Summary
	if summary == nil || summary.CampaignsHighDRRCount < 1 {
		return RiskCandidate{}, false
	}
	revenueBase := math.Max(summary.TotalStoreRevenueToday, 1)
	share := summary.HighDRRSpend / revenueBase
	impact := clampInt(int(math.Round(share*40))+summary.CampaignsHighDRRCount*10, 15, 95)
	severity := "medium"
	switch {
	case summary.CampaignsHighDRRCount >= 3 || summary.HighDRRSpend >= 50000:
		severity = "critical"
	case summary.CampaignsHighDRRCount >= 2 || summary.HighDRRSpend >= 20000:
		severity = "high"
	}
	entityIDs := make([]string, 0, len(facts.AdEfficiency.HighDRRCampaigns))
	for _, c := range facts.AdEfficiency.HighDRRCampaigns {
		entityIDs = append(entityIDs, strconv.FormatInt(c.CampaignExternalID, 10))
	}
	return RiskCandidate{
		DriverID:           DriverHighDRRCampaign,
		Domain:             "advertising",
		FactIDs:            []string{"ad_efficiency.summary.campaigns_high_drr_count", "ad_efficiency.summary.high_drr_spend"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             adPeriod(facts),
		ReasonTemplateData: map[string]any{"campaigns_high_drr_count": summary.CampaignsHighDRRCount, "high_drr_spend": summary.HighDRRSpend},
	}, true
}

func buildCampaignSpendWithoutOrdersCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	summary := facts.AdEfficiency.Summary
	if summary == nil || summary.CampaignsNoOrdersCount < 1 || summary.SpendWithoutOrders <= 0 {
		return RiskCandidate{}, false
	}
	severity := "medium"
	switch {
	case summary.SpendWithoutOrders >= 20000:
		severity = "critical"
	case summary.SpendWithoutOrders >= 8000 || summary.CampaignsNoOrdersCount >= 2:
		severity = "high"
	}
	revenueBase := math.Max(summary.TotalStoreRevenueToday, 1)
	share := summary.SpendWithoutOrders / revenueBase
	impact := clampInt(int(math.Round(share*45))+summary.CampaignsNoOrdersCount*12, 15, 96)
	entityIDs := make([]string, 0, len(facts.AdEfficiency.NoOrderCampaigns))
	for _, c := range facts.AdEfficiency.NoOrderCampaigns {
		entityIDs = append(entityIDs, strconv.FormatInt(c.CampaignExternalID, 10))
	}
	return RiskCandidate{
		DriverID:           DriverCampaignSpendWithoutOrders,
		Domain:             "advertising",
		FactIDs:            []string{"ad_efficiency.summary.campaigns_no_orders_count", "ad_efficiency.summary.spend_without_orders"},
		EntityIDs:          entityIDs,
		Severity:           severity,
		ImpactScore:        impact,
		Period:             adPeriod(facts),
		ReasonTemplateData: map[string]any{"campaigns_no_orders_count": summary.CampaignsNoOrdersCount, "spend_without_orders": summary.SpendWithoutOrders},
	}, true
}

func buildAdShareGrowthCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	summary := facts.AdEfficiency.Summary
	if summary == nil {
		return RiskCandidate{}, false
	}
	growth := 0.0
	if summary.DeltaVsPreviousPp != nil && *summary.DeltaVsPreviousPp > growth {
		growth = *summary.DeltaVsPreviousPp
	}
	if summary.DeltaVsAverage7dPp != nil && *summary.DeltaVsAverage7dPp > growth {
		growth = *summary.DeltaVsAverage7dPp
	}
	if growth < adShareGrowthMinPp {
		return RiskCandidate{}, false
	}
	impact := clampInt(int(math.Round(growth*4)), 12, 85)
	severity := severityFromScore(growth, 12, 7, 3)
	return RiskCandidate{
		DriverID:    DriverAdShareGrowth,
		Domain:      "advertising",
		FactIDs:     []string{"ad_efficiency.summary.delta_vs_previous_pp", "ad_efficiency.summary.delta_vs_average_7d_pp"},
		Severity:    severity,
		ImpactScore: impact,
		Period:      adPeriod(facts),
		ReasonTemplateData: map[string]any{
			"drr_growth_pp":   growth,
			"current_drr_pct": derefFloat(summary.CurrentDRRPct),
		},
	}, true
}

// buildDataLimitationCandidate flags backend-visible data-coverage gaps
// (explicit non-"available" statuses, or zero account metrics coverage).
// It is always informational-only (see ScoringCandidates) — never scored —
// so a missing optional source (e.g. no advertising data) is transparently
// surfaced without ever penalizing health_score.
func buildDataLimitationCandidate(facts dashboardfacts.DashboardFactsV1) (RiskCandidate, bool) {
	var notes []string
	var factIDs []string
	addGap := func(note, factID string) {
		notes = append(notes, note)
		factIDs = append(factIDs, factID)
	}
	if !isAvailableStatus(facts.StockCoverage.Status) {
		addGap("stock_coverage", "stock_coverage.status")
	}
	if !isAvailableStatus(facts.AdEfficiency.Status) {
		addGap("ad_efficiency", "ad_efficiency.status")
	}
	if !isAvailableStatus(facts.Funnel.Status) {
		addGap("funnel", "funnel.status")
	}
	if facts.DataCoverage.AccountMetricsDays <= 0 {
		addGap("account_metrics", "data_coverage.account_metrics_days")
	}
	if len(notes) == 0 {
		return RiskCandidate{}, false
	}
	return RiskCandidate{
		DriverID:           DriverDataLimitation,
		Domain:             "data_quality",
		FactIDs:            factIDs,
		Severity:           "low",
		ImpactScore:        0,
		Period:             facts.MetricDate,
		ReasonTemplateData: map[string]any{"missing_domains": notes},
	}, true
}
