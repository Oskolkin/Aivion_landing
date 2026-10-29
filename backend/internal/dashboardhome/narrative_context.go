package dashboardhome

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func buildDashboardNarrativeContext(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	drivers []RiskDriver,
	healthScore int32,
	severity string,
	alertRows []dbgen.Alert,
	actionPool []TopActionItem,
	recs []dbgen.Recommendation,
	previousSnapshot *accountMetricsSnapshot,
) dashboardNarrativeBuildResult {
	candidates := buildDashboardNarrativeCandidates(drivers, actionPool)
	metricCatalog := make(map[string]dashboardNarrativeMetric)
	candidateByID := make(map[string]dashboardNarrativeCandidate, len(candidates))
	for _, candidate := range candidates {
		candidateByID[candidate.ID] = candidate
		for _, metric := range candidate.SupportingMetrics {
			if metric.Key == "" {
				continue
			}
			if _, exists := metricCatalog[metric.Key]; exists {
				continue
			}
			metricCatalog[metric.Key] = metric
		}
	}

	allowedQuotedNames := buildAllowedNarrativeQuotedNames(dto, resp)
	for _, driver := range drivers {
		if title := strings.TrimSpace(driver.Title); title != "" {
			allowedQuotedNames[title] = struct{}{}
		}
	}
	for _, row := range alertRows {
		if title := strings.TrimSpace(row.Title); title != "" {
			allowedQuotedNames[title] = struct{}{}
		}
	}
	whatChangedCtx := buildWhatChangedContext(metricDate, dto, resp, drivers, previousSnapshot)
	alertsView := buildDashboardNarrativeAlerts(alertRows)
	riskDriversView := buildDashboardNarrativeRiskDriverViews(drivers)
	stockCtx := buildStockCoverageNarrativeValidationContext(dto, resp, recs)
	stockRequired := resp.AIDiagnostics.StockCoverage.Status == WidgetAvailable && resp.AIDiagnostics.StockCoverage.TotalSKUs > 0
	adEfficiencyCtx := buildAdEfficiencyNarrativeValidationContext(resp)
	adEfficiencyRequired := resp.AIDiagnostics.AdEfficiency.Status == WidgetAvailable || resp.AIDiagnostics.AdEfficiency.Status == WidgetPartial
	context := dashboardNarrativeContext{
		MetricDate:               metricDate.UTC().Format("2006-01-02"),
		HealthScore:              healthScore,
		Severity:                 severity,
		RiskDrivers:              riskDriversView,
		RecommendationCandidates: candidates,
		Alerts:                   alertsView,
		SupportingMetricsCatalog: buildNarrativeMetricLabelCatalog(metricCatalog),
		SalesSummary:             buildDashboardNarrativeSalesSummary(metricDate, dto),
		CancellationsSummary:     buildDashboardNarrativeCancellationsSummary(metricDate, dto),
		FunnelSummary:            buildDashboardNarrativeFunnelSummary(resp),
		StockRisks:               buildDashboardNarrativeStockSummary(resp),
		StockCoverageContext:     buildStockCoverageContextMap(dto, resp, recs),
		AdvertisingEfficiency:    buildDashboardNarrativeAdvertisingSummary(resp),
		AdEfficiencyContext:      buildAdEfficiencyNarrativeContextMap(resp),
		SupportingMetrics:        buildDashboardNarrativeSupportingMetrics(dto, resp),
		DataFreshness: map[string]any{
			"metric_date":        metricDate.UTC().Format("2006-01-02"),
			"source":             "post_sync",
			"dashboard_status":   resp.Status,
			"data_quality_notes": stringSliceOrEmpty(resp.DataQualityNotes),
		},
		WhatChangedContext: whatChangedCtx,
		SellerPulseContext: buildSellerPulseContextMap(metricDate.UTC().Format("2006-01-02"), dto, resp, alertRows, actionPool),
		Constraints: map[string]bool{
			"do_not_invent_metrics":                     true,
			"do_not_rename_campaigns":                   true,
			"do_not_classify_by_campaign_name":          true,
			"do_not_recommend_price_cut_without_margin": true,
			"do_not_increase_ads_for_low_stock":         true,
			"stock_narrative_no_sku_level_actions":      true,
			"stock_narrative_no_action_verbs":           true,
			"ad_efficiency_campaign_level_only":         true,
			"ad_efficiency_no_sku_attribution":          true,
		},
	}

	sellerPulseRequired := len(resp.SellerPulse.KPIs) > 0 || dto.Account.RevenueToday > 0 || dto.Account.OrdersToday > 0
	factualHash := buildFactualContextHash(metricDate, dto, resp, drivers, healthScore, severity, alertRows, actionPool, recs)
	return dashboardNarrativeBuildResult{
		Context:                       context,
		ContextHash:                   hashNarrativeValue(context),
		FactualContextHash:            factualHash,
		RiskDriversHash:               hashNarrativeValue(buildRiskDriverHashInput(drivers)),
		CandidateByID:                 candidateByID,
		MetricCatalogByKey:            metricCatalog,
		AllowedQuotedNames:            allowedQuotedNames,
		WhatChanged:                   buildWhatChangedValidationBundle(whatChangedCtx, allowedQuotedNames),
		StockNarrativeCtx:             stockCtx,
		StockNarrativeRequired:        stockRequired,
		AdEfficiencyNarrativeCtx:      adEfficiencyCtx,
		AdEfficiencyNarrativeRequired: adEfficiencyRequired,
		SellerPulseCtx:                buildSellerPulseNarrativeValidationContext(resp.SellerPulse.KPIs, dashboardNarrativeBuildResult{CandidateByID: candidateByID}),
		SellerPulseRequired:           sellerPulseRequired,
	}
}

func buildDashboardNarrativeRiskDriverViews(drivers []RiskDriver) []dashboardNarrativeRiskDriverView {
	out := make([]dashboardNarrativeRiskDriverView, 0, len(drivers))
	for _, driver := range drivers {
		out = append(out, dashboardNarrativeRiskDriverView{
			ID:                      driver.ID,
			Category:                driver.Category,
			CategoryLabel:           driver.CategoryLabel,
			Severity:                driver.Severity,
			ImpactScore:             driver.ImpactScore,
			Title:                   driver.Title,
			Reason:                  driver.Reason,
			Evidence:                driver.Evidence,
			LinkedAlertIDs:          compactInt64s(driver.LinkedAlertIDs),
			LinkedRecommendationIDs: compactInt64s(driver.LinkedRecommendationIDs),
		})
	}
	return out
}

func buildDashboardNarrativeCandidates(drivers []RiskDriver, actionPool []TopActionItem) []dashboardNarrativeCandidate {
	out := make([]dashboardNarrativeCandidate, 0, len(actionPool)+len(drivers))
	seen := make(map[string]struct{}, len(actionPool)+len(drivers))
	appendCandidate := func(candidate dashboardNarrativeCandidate) {
		if candidate.ID == "" {
			return
		}
		if _, exists := seen[candidate.ID]; exists {
			return
		}
		seen[candidate.ID] = struct{}{}
		out = append(out, candidate)
	}

	for _, action := range actionPool {
		appendCandidate(dashboardNarrativeCandidateFromTopAction(action, drivers))
	}
	for _, driver := range drivers {
		appendCandidate(dashboardNarrativeCandidateFromTopAction(fallbackActionForDriver(driver), []RiskDriver{driver}))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if priorityRank(out[i].Priority) != priorityRank(out[j].Priority) {
			return priorityRank(out[i].Priority) < priorityRank(out[j].Priority)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func dashboardNarrativeCandidateFromTopAction(action TopActionItem, drivers []RiskDriver) dashboardNarrativeCandidate {
	compatibleDriverIDs := compatibleDriverIDsForAction(action, drivers)
	metrics := make([]dashboardNarrativeMetric, 0, len(action.SupportingMetrics))
	for _, metric := range action.SupportingMetrics {
		key := inferDashboardNarrativeMetricKey(action.Category, metric.Label)
		if key == "" {
			continue
		}
		metrics = append(metrics, dashboardNarrativeMetric{
			Key:   key,
			Label: metric.Label,
			Value: metric.Value,
		})
	}
	return dashboardNarrativeCandidate{
		ID:                      strings.TrimSpace(action.ID),
		Source:                  strings.TrimSpace(action.Source),
		Priority:                normalizeTopActionPriority(action.Priority),
		Category:                strings.TrimSpace(action.Category),
		CategoryLabel:           strings.TrimSpace(action.CategoryLabel),
		Title:                   cleanupSellerFacingText(action.Title),
		Reason:                  cleanupSellerFacingText(action.Reason),
		NextStep:                cleanupSellerFacingText(action.NextStep),
		CompatibleDriverIDs:     compactStringSlice(compatibleDriverIDs),
		LinkedAlertIDs:          compactInt64s(action.LinkedAlertIDs),
		LinkedRecommendationIDs: compactInt64s(action.LinkedRecommendationIDs),
		SupportingMetrics:       metrics,
	}
}

func compatibleDriverIDsForAction(action TopActionItem, drivers []RiskDriver) []string {
	ids := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		if action.DriverID != "" && action.DriverID == driver.ID {
			ids = append(ids, driver.ID)
			continue
		}
		if overlapsInt64(action.LinkedAlertIDs, driver.LinkedAlertIDs) || overlapsInt64(action.LinkedRecommendationIDs, driver.LinkedRecommendationIDs) {
			ids = append(ids, driver.ID)
			continue
		}
		if action.Category != "" && action.Category == topActionCategoryForDriver(driver.Category) {
			ids = append(ids, driver.ID)
		}
	}
	if len(ids) == 0 && action.DriverID != "" {
		ids = append(ids, action.DriverID)
	}
	return ids
}

func inferDashboardNarrativeMetricKey(category string, label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(label, "расход"):
		return "ad_spend"
	case strings.Contains(label, "кампан"):
		return "campaigns_at_risk"
	case strings.Contains(label, "выручк") && category == "advertising":
		return "ad_revenue"
	case strings.Contains(label, "выручк"):
		return "revenue"
	case strings.Contains(label, "заказы ранее"):
		return "previous_orders"
	case strings.Contains(label, "заказы"):
		return "orders"
	case strings.Contains(label, "окупаем"):
		return "ad_roas"
	case strings.Contains(label, "остат"):
		return "stock_available"
	case strings.Contains(label, "0-3"):
		return "critical_stock_skus"
	case strings.Contains(label, "4-7"):
		return "shortage_stock_skus"
	case strings.Contains(label, "хватит") || strings.Contains(label, "покрыт"):
		return "days_of_cover"
	case strings.Contains(label, "ко вчера"):
		return "revenue_day_delta_pct"
	case strings.Contains(label, "к 7"):
		return "revenue_week_delta_pct"
	case strings.Contains(label, "доля отмен"):
		return "cancellation_rate"
	case strings.Contains(label, "отмен"):
		return "cancels_today"
	case strings.Contains(label, "sku в риске"):
		return "surplus_sku_count"
	case strings.Contains(label, "слабый этап"):
		return "funnel_stage"
	case strings.Contains(label, "изменение"):
		return "delta_percent"
	default:
		return ""
	}
}

func buildNarrativeMetricLabelCatalog(metrics map[string]dashboardNarrativeMetric) map[string]string {
	out := make(map[string]string, len(metrics))
	keys := make([]string, 0, len(metrics))
	for key := range metrics {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out[key] = metrics[key].Label
	}
	return out
}

func buildDashboardNarrativeAlerts(rows []dbgen.Alert) []map[string]any {
	if len(rows) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, minInt(len(rows), 8))
	for _, row := range rows {
		out = append(out, map[string]any{
			"id":          row.ID,
			"severity":    row.Severity,
			"alert_type":  row.AlertType,
			"title":       cleanupSellerFacingText(row.Title),
			"message":     cleanupSellerFacingText(row.Message),
			"entity_type": row.EntityType,
		})
		if len(out) >= 8 {
			break
		}
	}
	return out
}

func buildDashboardNarrativeSalesSummary(metricDate time.Time, dto analytics.DashboardMetricsDTO) map[string]any {
	return map[string]any{
		"metric_date":            metricDate.UTC().Format("2006-01-02"),
		"revenue_today":          dto.Account.RevenueToday,
		"revenue_day_delta_pct":  ratioToPercent(dto.Account.RevenueDayDelta.Pct),
		"revenue_week_delta_pct": ratioToPercent(dto.Account.RevenueWeekDelta.Pct),
		"orders_today":           dto.Account.OrdersToday,
		"orders_yesterday":       dto.Account.OrdersYesterday,
	}
}

func buildDashboardNarrativeCancellationsSummary(metricDate time.Time, dto analytics.DashboardMetricsDTO) map[string]any {
	return map[string]any{
		"metric_date":           metricDate.UTC().Format("2006-01-02"),
		"cancels_today":         dto.Account.CancelsToday,
		"orders_today":          dto.Account.OrdersToday,
		"cancellation_rate_pct": safePercent(dto.Account.CancellationRate),
	}
}

func buildDashboardNarrativeFunnelSummary(resp *Response) map[string]any {
	stages := make([]map[string]any, 0, len(resp.AIDiagnostics.SalesFunnel.Stages))
	for _, stage := range resp.AIDiagnostics.SalesFunnel.Stages {
		stages = append(stages, map[string]any{
			"key":                        stage.Key,
			"label":                      stage.Label,
			"value":                      stage.Value,
			"conversion_from_previous":   stage.ConversionFromPrevious,
			"conversion_from_top":        stage.ConversionFromTop,
			"change_vs_previous_day_pct": stage.ChangeVsPreviousDayPct,
		})
	}
	return map[string]any{
		"status":               resp.AIDiagnostics.SalesFunnel.Status,
		"primary_drop_stage":   resp.AIDiagnostics.FunnelAnalysis.PrimaryDropStage,
		"primary_drop_message": resp.AIDiagnostics.FunnelAnalysis.PrimaryDropMessage,
		"stages":               stages,
	}
}

func buildDashboardNarrativeStockSummary(resp *Response) map[string]any {
	stockRisk := make([]string, 0, minInt(len(resp.SKUFocus.StockRisk), 5))
	for _, card := range resp.SKUFocus.StockRisk {
		name := strings.TrimSpace(card.ProductName)
		if name != "" {
			stockRisk = append(stockRisk, name)
		}
	}
	surplus := make([]string, 0, minInt(len(resp.SKUFocus.Surplus), 5))
	for _, card := range resp.SKUFocus.Surplus {
		name := strings.TrimSpace(card.ProductName)
		if name != "" {
			surplus = append(surplus, name)
		}
	}
	return map[string]any{
		"coverage_segments": resp.AIDiagnostics.StockCoverage.Segments,
		"stock_risk_skus":   stockRisk,
		"surplus_skus":      surplus,
	}
}

func buildDashboardNarrativeAdvertisingSummary(resp *Response) map[string]any {
	w := resp.AIDiagnostics.AdEfficiency
	if ctx := buildAdEfficiencyNarrativeContextMap(resp); ctx != nil {
		if inner, ok := ctx["ad_efficiency"].(map[string]any); ok {
			return inner
		}
	}
	points := make([]map[string]any, 0, minInt(len(w.Points), 6))
	for _, point := range w.Points {
		points = append(points, map[string]any{
			"campaign_external_id": point.CampaignExternalID,
			"campaign_name":        point.CampaignName,
			"spend":                point.Spend,
			"revenue":              point.Revenue,
			"orders":               point.Orders,
			"efficiency_signal":    point.EfficiencySignal,
		})
	}
	return map[string]any{
		"status":    w.Status,
		"campaigns": points,
	}
}

func buildDashboardNarrativeSupportingMetrics(dto analytics.DashboardMetricsDTO, resp *Response) map[string]any {
	criticalStock, shortageStock := stockCoverageCounts(resp.AIDiagnostics.StockCoverage.Segments)
	critRiskCount := int64(0)
	for _, group := range resp.AIDiagnostics.RiskDistribution.Groups {
		if strings.EqualFold(group.Severity, "critical") || strings.EqualFold(group.Severity, "high") {
			critRiskCount += group.Count
		}
	}
	return map[string]any{
		"account_ad_spend_to_revenue": valueOrZero(dto.Account.AccountAdSpendToRevenue),
		"critical_stock_skus":         criticalStock,
		"shortage_stock_skus":         shortageStock,
		"open_high_or_critical_risks": critRiskCount,
		"data_quality_notes":          stringSliceOrEmpty(resp.DataQualityNotes),
	}
}

func buildAllowedNarrativeQuotedNames(dto analytics.DashboardMetricsDTO, resp *Response) map[string]struct{} {
	out := make(map[string]struct{})
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		out[value] = struct{}{}
	}
	for _, sku := range dto.SKUs {
		add(derefStr(sku.ProductName, ""))
	}
	for _, point := range resp.AIDiagnostics.AdEfficiency.Points {
		add(point.CampaignName)
	}
	for _, stage := range resp.AIDiagnostics.SalesFunnel.Stages {
		add(stage.Label)
	}
	add(resp.AIDiagnostics.FunnelAnalysis.PrimaryDropStage)
	for _, card := range resp.SKUFocus.StockRisk {
		add(card.ProductName)
	}
	for _, card := range resp.SKUFocus.Surplus {
		add(card.ProductName)
	}
	return out
}

func buildRiskDriverHashInput(drivers []RiskDriver) []map[string]any {
	out := make([]map[string]any, 0, len(drivers))
	for _, driver := range drivers {
		out = append(out, map[string]any{
			"id":                        driver.ID,
			"category":                  driver.Category,
			"severity":                  driver.Severity,
			"impact_score":              driver.ImpactScore,
			"evidence":                  driver.Evidence,
			"linked_alert_ids":          compactInt64s(driver.LinkedAlertIDs),
			"linked_recommendation_ids": compactInt64s(driver.LinkedRecommendationIDs),
		})
	}
	return out
}

func hashNarrativeValue(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
