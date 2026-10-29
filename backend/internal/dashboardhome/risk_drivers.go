package dashboardhome

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const riskDriverResponseLimit = 6

var (
	healthScoreWeights = []float64{1.0, 0.82, 0.65, 0.5, 0.38, 0.28}
	healthSeverityBase = map[string]float64{
		"critical": 18,
		"high":     13,
		"medium":   8,
		"low":      4,
	}
)

func applyUnifiedAccountHealth(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
	actionPool []TopActionItem,
	previousSnapshot *accountMetricsSnapshot,
	narrativeBuildOverride *dashboardNarrativeBuildResult,
	aiSummaryActions ...[]TopActionItem,
) {
	if dashboardAIOnlyEnabled() {
		applyUnifiedAccountHealthAIOnly(metricDate, dto, resp, recs, alertRows, actionPool, previousSnapshot, narrativeBuildOverride)
		return
	}
	var fallbackAISummaryActions []TopActionItem
	if len(aiSummaryActions) > 0 {
		fallbackAISummaryActions = aiSummaryActions[0]
	}
	drivers := buildRiskDrivers(metricDate, dto, resp, recs, alertRows)
	if dashboardAIOnlyEnabled() && narrativeBuildOverride != nil {
		if dp := riskDriversForAIOnlyApply(*narrativeBuildOverride, dto, resp, recs, alertRows); len(dp) > 0 {
			drivers = dp
		}
	}
	breakdown := computeHealthScoreBreakdown(drivers)
	score := breakdown.FinalScore
	severity := computeHealthSeverity(score)
	accountSummary := resp.AIDiagnostics.AccountSummary
	accountSummary.WidgetBase = WidgetBase{Status: WidgetAvailable}
	accountSummary.MetricDate = metricDate.UTC().Format("2006-01-02")
	accountSummary.HealthScore = &score
	accountSummary.Severity = severity
	if !dashboardAIOnlyEnabled() {
		accountSummary.NarrativeSource = "fallback"
		accountSummary.ValidationStatus = "fallback_stale"
		accountSummary.Freshness = "fallback"
	}
	build := buildDashboardNarrativeContext(metricDate, dto, resp, drivers, score, severity, alertRows, actionPool, recs, previousSnapshot)
	if narrativeBuildOverride != nil {
		build = *narrativeBuildOverride
	}
	accountSummary.narrativeBuild = &build
	enrichedDrivers := drivers
	var topActions []TopActionItem = []TopActionItem{}
	accountSummary.Headline = buildRiskHeadline(drivers)
	accountSummary.ImprovementSummary = buildRiskImprovementSummary(drivers)
	narrativeApplied := false
	var narrativePayload *dashboardNarrativePayload
	acceptance := acceptStoredDashboardNarrative(
		accountSummary.storedNarrative,
		build,
		drivers,
		actionPool,
		fallbackAISummaryActions,
		topActionsLimit,
	)
	if acceptance.Applied {
		narrativeApplied = true
		narrativePayload = acceptance.Payload
		if dashboardAIOnlyEnabled() {
			enrichedDrivers = buildAIRiskDriversFromHealthReasons(narrativePayload, build, drivers)
		} else {
			enrichedDrivers = applyDashboardNarrativeHealthReasons(drivers, narrativePayload)
		}
		accountSummary.Headline = narrativePayload.Headline
		accountSummary.ImprovementSummary = narrativePayload.ImprovementSummary
		accountSummary.ExecutiveSummary = narrativePayload.ExecutiveSummary
		topActions = acceptance.TopActions
		accountSummary.NarrativeSource = "ai"
		accountSummary.ValidationStatus = acceptance.ValidationStatus
		accountSummary.Freshness = acceptance.Freshness
		accountSummary.PromptVersion = accountSummary.storedNarrative.PromptVersion
		accountSummary.NarrativeRunID = accountSummary.storedNarrative.ID
	} else if accountSummary.storedNarrative != nil {
		accountSummary.PromptVersion = accountSummary.storedNarrative.PromptVersion
		accountSummary.NarrativeRunID = accountSummary.storedNarrative.ID
		accountSummary.ValidationStatus = acceptance.ValidationStatus
		if dashboardAIOnlyEnabled() && !acceptance.Applied {
			accountSummary.NarrativeSource = ""
			accountSummary.Freshness = acceptance.Freshness
		}
		if accountSummary.ValidationStatus == "" {
			if dashboardAIOnlyEnabled() {
				accountSummary.ValidationStatus = "failed"
			} else {
				accountSummary.ValidationStatus = "fallback_stale"
			}
		}
	}
	if shouldExposeNarrativeStaleDebug() && !dashboardAIOnlyEnabled() {
		accountSummary.NarrativeStaleDebug = buildNarrativeStaleDebug(metricDate, accountSummary.storedNarrative, build, accountSummary.ValidationStatus)
		accountSummary.NarrativeDebug = buildNarrativeDebug(accountSummary.storedNarrative, build, acceptance)
	}
	if !narrativeApplied {
		accountSummary.ExecutiveSummary = buildRiskExecutiveSummary(enrichedDrivers)
	}
	applyStockCoverageNarrativeToResponse(dto, resp, recs, narrativeApplied, narrativePayload)
	applyAdEfficiencyNarrativeToResponse(resp, narrativeApplied, narrativePayload)
	enrichedDrivers = sanitizeRiskDriversForSellerFacing(enrichedDrivers, resp)
	breakdown = computeHealthScoreBreakdown(enrichedDrivers)
	score = breakdown.FinalScore
	severity = computeHealthSeverity(score)
	accountSummary.HealthScore = &score
	accountSummary.Severity = severity
	accountSummary.RiskDrivers = enrichedDrivers
	if shouldExposeHealthScoreBreakdown() {
		accountSummary.HealthScoreBreakdown = &breakdown
	} else {
		accountSummary.HealthScoreBreakdown = nil
	}
	resp.AIDiagnostics.AccountSummary = accountSummary
	if narrativeApplied {
		resp.AIDiagnostics.TopActions = topActions
	} else {
		if dashboardAIOnlyEnabled() {
			resp.AIDiagnostics.TopActions = []TopActionItem{}
		} else {
			resp.AIDiagnostics.TopActions = buildAIDiagnosticsTopActions(
				false,
				nil,
				fallbackAISummaryActions,
				enrichedDrivers,
				actionPool,
				topActionsLimit,
			)
		}
	}
	applyWhatChangedToResponse(resp, narrativePayload, build.WhatChanged, narrativeApplied)
	setBlockApplySource(resp, "account_summary", appliedBlockSource(narrativeApplied))
	applyAIOnlyPayloadToResponse(resp, narrativePayload, narrativeApplied)
	reconcileSellerPulseRisksKPI(resp)
	reconcileRiskDistributionMessage(resp, enrichedDrivers)
}

func buildRiskDrivers(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
) []RiskDriver {
	drivers := make([]RiskDriver, 0, 6)
	appendDriver := func(driver RiskDriver, ok bool) {
		if !ok {
			return
		}
		if driver.ImpactScore <= 0 || strings.TrimSpace(driver.Title) == "" || strings.TrimSpace(driver.Reason) == "" {
			return
		}
		driver.Title = cleanupSellerFacingText(driver.Title)
		driver.Reason = cleanupSellerFacingText(driver.Reason)
		if driver.CategoryLabel == "" {
			driver.CategoryLabel = riskDriverCategoryLabel(driver.Category)
		}
		driver.Severity = normalizeTopActionPriority(driver.Severity)
		driver.LinkedAlertIDs = compactInt64s(driver.LinkedAlertIDs)
		driver.LinkedRecommendationIDs = compactInt64s(driver.LinkedRecommendationIDs)
		drivers = append(drivers, driver)
	}

	appendDriver(buildSalesRiskDriver(metricDate, dto, recs, alertRows))
	appendDriver(buildCancellationsRiskDriver(metricDate, dto, recs, alertRows))
	appendDriver(buildFunnelRiskDriver(resp, recs, alertRows))
	appendDriver(buildStockRiskDriver(resp, recs, alertRows))
	appendDriver(buildSurplusRiskDriver(resp, recs, alertRows))
	appendDriver(buildAdvertisingRiskDriver(resp, dto, recs, alertRows))

	sort.SliceStable(drivers, func(i, j int) bool {
		if drivers[i].ImpactScore != drivers[j].ImpactScore {
			return drivers[i].ImpactScore > drivers[j].ImpactScore
		}
		if priorityRank(drivers[i].Severity) != priorityRank(drivers[j].Severity) {
			return priorityRank(drivers[i].Severity) < priorityRank(drivers[j].Severity)
		}
		return drivers[i].Category < drivers[j].Category
	})
	if len(drivers) > riskDriverResponseLimit {
		drivers = drivers[:riskDriverResponseLimit]
	}
	return drivers
}

func buildSalesRiskDriver(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
) (RiskDriver, bool) {
	dayPct := ratioToPercent(dto.Account.RevenueDayDelta.Pct)
	weekPct := ratioToPercent(dto.Account.RevenueWeekDelta.Pct)
	ordersPct := intChangePercent(dto.Account.OrdersToday, dto.Account.OrdersYesterday)
	worstDrop := math.Max(math.Abs(minFloat(dayPct, 0)), math.Abs(minFloat(weekPct, 0)))
	worstDrop = math.Max(worstDrop, math.Abs(minFloat(ordersPct, 0)))
	if worstDrop < 8 {
		return RiskDriver{}, false
	}

	severity := severityFromScore(worstDrop, 30, 18, 8)
	impact := clampInt(int(math.Round(worstDrop*1.45))+topSKURevenueDropImpact(dto.SKUs), 12, 95)
	topDecline := topRevenueDeclineSKU(dto.SKUs)
	reason := fmt.Sprintf(
		"Выручка за %s составила %s: это %s ко вчера и %s к предыдущим 7 дням.",
		metricDate.UTC().Format("02.01.2006"),
		formatMoneyRounded(dto.Account.RevenueToday),
		describeSignedPercent(dayPct),
		describeSignedPercent(weekPct),
	)
	if topDecline != nil {
		reason += fmt.Sprintf(" Сильнее всего проседает «%s».", derefStr(topDecline.ProductName, "товар"))
	}

	return RiskDriver{
		ID:            "driver-sales-revenue",
		Category:      "sales",
		CategoryLabel: riskDriverCategoryLabel("sales"),
		Severity:      severity,
		ImpactScore:   impact,
		Title:         "Снижаются выручка и заказы",
		Reason:        reason,
		Evidence: map[string]any{
			"metric_date":             metricDate.UTC().Format("2006-01-02"),
			"revenue_today":           dto.Account.RevenueToday,
			"revenue_day_delta_pct":   dayPct,
			"revenue_week_delta_pct":  weekPct,
			"orders_today":            dto.Account.OrdersToday,
			"orders_yesterday":        dto.Account.OrdersYesterday,
			"orders_day_delta_pct":    ordersPct,
			"top_declining_sku_name":  productNameOrEmpty(topDecline),
			"top_declining_sku_delta": productRevenueDeltaOrZero(topDecline),
		},
		LinkedAlertIDs:          linkedAlertIDsForDriverCategory(alertRows, "sales"),
		LinkedRecommendationIDs: linkedRecommendationIDsForDriverCategory(recs, "sales"),
	}, true
}

func accountCancellationRatePercent(dto analytics.DashboardMetricsDTO) float64 {
	if dto.Account.OrdersToday > 0 {
		return float64(dto.Account.CancelsToday) / float64(dto.Account.OrdersToday) * 100
	}
	return safePercent(dto.Account.CancellationRate)
}

func buildCancellationsRiskDriver(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
) (RiskDriver, bool) {
	if dto.Account.OrdersToday <= 0 {
		return RiskDriver{}, false
	}
	rate := accountCancellationRatePercent(dto)
	if rate < 5 && dto.Account.CancelsToday < 4 {
		return RiskDriver{}, false
	}

	impact := clampInt(int(math.Round(rate*2.8))+int(dto.Account.CancelsToday), 10, 92)
	return RiskDriver{
		ID:            "driver-cancellations",
		Category:      "cancellations",
		CategoryLabel: riskDriverCategoryLabel("cancellations"),
		Severity:      severityFromScore(rate, 12, 8, 5),
		ImpactScore:   impact,
		Title:         "Растет доля отмен",
		Reason: fmt.Sprintf(
			"На %s отмены заняли %s от заказов: %d отмен при %d заказах.",
			metricDate.UTC().Format("02.01.2006"),
			formatPercent(rate),
			dto.Account.CancelsToday,
			dto.Account.OrdersToday,
		),
		Evidence: map[string]any{
			"metric_date":         metricDate.UTC().Format("2006-01-02"),
			"cancellation_rate":   rate,
			"cancels_today":       dto.Account.CancelsToday,
			"orders_today":        dto.Account.OrdersToday,
			"orders_by_status":    dto.Account.OrdersByStatus,
			"average_order_value": valueOrZero(dto.Account.AverageOrderValue),
		},
		LinkedAlertIDs:          linkedAlertIDsForDriverCategory(alertRows, "cancellations"),
		LinkedRecommendationIDs: linkedRecommendationIDsForDriverCategory(recs, "cancellations"),
	}, true
}

func buildFunnelRiskDriver(resp *Response, recs []dbgen.Recommendation, alertRows []dbgen.Alert) (RiskDriver, bool) {
	stages := resp.AIDiagnostics.SalesFunnel.Stages
	weakest, weakestDrop, ok := weakestFunnelStage(stages)
	if !ok && strings.TrimSpace(resp.AIDiagnostics.FunnelAnalysis.PrimaryDropMessage) == "" {
		return RiskDriver{}, false
	}
	impact := clampInt(int(math.Round(math.Abs(weakestDrop)*1.35))+15, 10, 90)
	severity := severityFromScore(math.Abs(weakestDrop), 35, 20, 10)
	reason := strings.TrimSpace(resp.AIDiagnostics.FunnelAnalysis.PrimaryDropMessage)
	if reason == "" {
		reason = fmt.Sprintf("На этапе «%s» показатель снизился на %s к предыдущему дню.", weakest.Label, formatPercent(math.Abs(weakestDrop)))
	} else if ok {
		reason += fmt.Sprintf(" Дополнительно этап «%s» просел на %s к предыдущему дню.", weakest.Label, formatPercent(math.Abs(weakestDrop)))
	}

	return RiskDriver{
		ID:            "driver-funnel",
		Category:      "funnel",
		CategoryLabel: riskDriverCategoryLabel("funnel"),
		Severity:      severity,
		ImpactScore:   impact,
		Title:         "Ослабла воронка продаж",
		Reason:        reason,
		Evidence: map[string]any{
			"primary_drop_stage":   resp.AIDiagnostics.FunnelAnalysis.PrimaryDropStage,
			"primary_drop_message": resp.AIDiagnostics.FunnelAnalysis.PrimaryDropMessage,
			"weakest_stage":        weakest.Label,
			"weakest_stage_change": weakestDrop,
		},
		LinkedAlertIDs:          linkedAlertIDsForDriverCategory(alertRows, "funnel"),
		LinkedRecommendationIDs: linkedRecommendationIDsForDriverCategory(recs, "funnel"),
	}, true
}

func buildStockRiskDriver(resp *Response, recs []dbgen.Recommendation, alertRows []dbgen.Alert) (RiskDriver, bool) {
	critical, shortage := stockCoverageCounts(resp.AIDiagnostics.StockCoverage.Segments)
	if critical == 0 && shortage == 0 && len(resp.SKUFocus.StockRisk) == 0 {
		return RiskDriver{}, false
	}
	firstSKU := firstSKUName(resp.SKUFocus.StockRisk)
	reason := buildStockRiskReason(critical, shortage, firstSKU)
	impact := clampInt(critical*16+shortage*8, 12, 92)
	return RiskDriver{
		ID:            "driver-stock",
		Category:      "stock",
		CategoryLabel: riskDriverCategoryLabel("stock"),
		Severity:      severityFromInts(critical, shortage),
		ImpactScore:   impact,
		Title:         "Есть риск дефицита по товарам",
		Reason:        reason,
		Evidence: map[string]any{
			"critical_0_3_skus": critical,
			"shortage_4_7_skus": shortage,
			"top_risk_sku_name": firstSKU,
		},
		LinkedAlertIDs:          linkedAlertIDsForDriverCategory(alertRows, "stock"),
		LinkedRecommendationIDs: linkedRecommendationIDsForDriverCategory(recs, "stock"),
	}, true
}

func buildSurplusRiskDriver(resp *Response, recs []dbgen.Recommendation, alertRows []dbgen.Alert) (RiskDriver, bool) {
	surplusCount := len(resp.SKUFocus.Surplus)
	if surplusCount == 0 && !hasAlertType(alertRows, string(alerts.AlertTypeFBOSurplusStorageRisk)) {
		return RiskDriver{}, false
	}
	firstSKU := firstSKUName(resp.SKUFocus.Surplus)
	reason := fmt.Sprintf("Есть товары с медленной оборачиваемостью и риском лишнего хранения: %d SKU требуют проверки.", maxInt(surplusCount, 1))
	if firstSKU != "" {
		reason += fmt.Sprintf(" В первую очередь проверьте «%s».", firstSKU)
	}
	return RiskDriver{
		ID:            driverSlowTurnover,
		Category:      "slow_turnover",
		CategoryLabel: riskDriverCategoryLabel("slow_turnover"),
		Severity:      severityFromScore(float64(surplusCount), 5, 3, 1),
		ImpactScore:   clampInt(surplusCount*12, 8, 80),
		Title:         "Часть остатков оборачивается слишком медленно",
		Reason:        reason,
		Evidence: map[string]any{
			"surplus_sku_count": surplusCount,
			"top_surplus_sku":   firstSKU,
		},
		LinkedAlertIDs:          linkedAlertIDsForDriverCategory(alertRows, "surplus"),
		LinkedRecommendationIDs: linkedRecommendationIDsForDriverCategory(recs, "surplus"),
	}, true
}

func buildAdvertisingRiskDriver(
	resp *Response,
	dto analytics.DashboardMetricsDTO,
	recs []dbgen.Recommendation,
	alertRows []dbgen.Alert,
) (RiskDriver, bool) {
	points := resp.AIDiagnostics.AdEfficiency.Points
	if len(points) == 0 {
		return RiskDriver{}, false
	}
	var spendAtRisk float64
	var zeroResultCount int
	var weakCount int
	var worst *AdEfficiencyPoint
	var worstScore float64
	for i := range points {
		point := points[i]
		if point.Spend <= 0 {
			continue
		}
		roas := 0.0
		if point.Spend > 0 {
			roas = point.Revenue / point.Spend
		}
		penalty := 0.0
		switch {
		case point.Orders <= 0 && point.Revenue <= 0:
			zeroResultCount++
			spendAtRisk += point.Spend
			weakCount++
			penalty = point.Spend * 1.4
		case roas < 0.5:
			spendAtRisk += point.Spend
			weakCount++
			penalty = point.Spend * 1.15
		case roas < 1 || strings.EqualFold(point.EfficiencySignal, "weak_efficiency") || strings.EqualFold(point.EfficiencySignal, "growth_without_result"):
			spendAtRisk += point.Spend
			weakCount++
			penalty = point.Spend
		}
		if penalty > worstScore {
			worstScore = penalty
			worst = &point
		}
	}
	if weakCount == 0 {
		return RiskDriver{}, false
	}

	revenueBase := math.Max(dto.Account.RevenueToday, 1)
	lossShare := spendAtRisk / revenueBase
	impact := clampInt(int(math.Round(lossShare*35))+zeroResultCount*14+weakCount*8, 10, 98)
	severity := "medium"
	switch {
	case zeroResultCount > 0 && spendAtRisk >= 20000:
		severity = "critical"
	case spendAtRisk >= 50000 || zeroResultCount >= 2:
		severity = "high"
	default:
		severity = "medium"
	}
	reason := fmt.Sprintf("Есть %d %s со слабой отдачей, на которые уже ушло %s.", weakCount, russianCountWord(weakCount, "кампания", "кампании", "кампаний"), formatMoneyRounded(spendAtRisk))
	if worst != nil {
		reason = fmt.Sprintf("Есть %d кампаний со слабой отдачей. Самая проблемная сейчас — «%s»: расход %s, выручка %s, заказов %d.",
			weakCount,
			worst.CampaignName,
			formatMoneyRounded(worst.Spend),
			formatMoneyRounded(worst.Revenue),
			worst.Orders,
		)
	}

	return RiskDriver{
		ID:            "driver-advertising",
		Category:      "advertising",
		CategoryLabel: riskDriverCategoryLabel("advertising"),
		Severity:      severity,
		ImpactScore:   impact,
		Title:         "Рекламные расходы работают с низкой отдачей",
		Reason:        reason,
		Evidence: map[string]any{
			"campaigns_at_risk":           weakCount,
			"zero_result_campaigns":       zeroResultCount,
			"spend_at_risk":               spendAtRisk,
			"account_ad_spend_to_revenue": valueOrZero(dto.Account.AccountAdSpendToRevenue),
			"worst_campaign_name":         campaignNameOrEmpty(worst),
		},
		LinkedAlertIDs:          linkedAlertIDsForDriverCategory(alertRows, "advertising"),
		LinkedRecommendationIDs: linkedRecommendationIDsForDriverCategory(recs, "advertising"),
	}, true
}

func selectTopActionsFromDrivers(drivers []RiskDriver, pool []TopActionItem, limit int) []TopActionItem {
	if limit <= 0 {
		return []TopActionItem{}
	}
	if len(drivers) == 0 {
		if len(pool) <= limit {
			return slices.Clone(pool)
		}
		return slices.Clone(pool[:limit])
	}

	usedIDs := map[string]struct{}{}
	out := make([]TopActionItem, 0, limit)
	topImpact := drivers[0].ImpactScore
	secondImpact := 0
	if len(drivers) > 1 {
		secondImpact = drivers[1].ImpactScore
	}
	dominantTopCategory := secondImpact == 0 || topImpact >= int(math.Round(float64(secondImpact)*1.5))

	appendAction := func(driver RiskDriver, action TopActionItem) {
		if len(out) >= limit {
			return
		}
		if _, exists := usedIDs[action.ID]; exists {
			return
		}
		action.DriverID = driver.ID
		action.LinkedAlertIDs = compactInt64s(append(action.LinkedAlertIDs, driver.LinkedAlertIDs...))
		action.LinkedRecommendationIDs = compactInt64s(append(action.LinkedRecommendationIDs, driver.LinkedRecommendationIDs...))
		action.Category = topActionCategoryForDriver(driver.Category)
		action.CategoryLabel = topActionCategoryLabel(action.Category)
		if strings.TrimSpace(action.Priority) == "" {
			action.Priority = driver.Severity
			action.PriorityLabel = topActionPriorityLabel(action.Priority)
		}
		out = append(out, action)
		usedIDs[action.ID] = struct{}{}
	}

	if dominantTopCategory {
		topDriver := drivers[0]
		appendAction(topDriver, actionForDriver(topDriver, pool, usedIDs))
		for _, candidate := range pool {
			if len(out) >= limit {
				break
			}
			if _, exists := usedIDs[candidate.ID]; exists {
				continue
			}
			if candidate.Category != topActionCategoryForDriver(topDriver.Category) {
				continue
			}
			appendAction(topDriver, candidate)
		}
		for _, driver := range drivers[1:] {
			if len(out) >= limit {
				break
			}
			appendAction(driver, actionForDriver(driver, pool, usedIDs))
		}
		return out
	}

	usedCategories := map[string]struct{}{}
	for _, driver := range drivers {
		if len(out) >= limit {
			break
		}
		actionCategory := topActionCategoryForDriver(driver.Category)
		if _, exists := usedCategories[actionCategory]; exists {
			continue
		}
		appendAction(driver, actionForDriver(driver, pool, usedIDs))
		usedCategories[actionCategory] = struct{}{}
	}
	for _, driver := range drivers {
		if len(out) >= limit {
			break
		}
		appendAction(driver, actionForDriver(driver, pool, usedIDs))
	}
	return out
}

func actionForDriver(driver RiskDriver, pool []TopActionItem, usedIDs map[string]struct{}) TopActionItem {
	if action, ok := linkedActionForDriver(driver, pool, usedIDs); ok {
		return action
	}
	return fallbackActionForDriver(driver)
}

func linkedActionForDriver(driver RiskDriver, pool []TopActionItem, usedIDs map[string]struct{}) (TopActionItem, bool) {
	var sameCategory *TopActionItem
	targetCategory := topActionCategoryForDriver(driver.Category)
	for i := range pool {
		action := pool[i]
		if _, exists := usedIDs[action.ID]; exists {
			continue
		}
		if overlapsInt64(action.LinkedRecommendationIDs, driver.LinkedRecommendationIDs) || overlapsInt64(action.LinkedAlertIDs, driver.LinkedAlertIDs) {
			return action, true
		}
		if action.Category == targetCategory && sameCategory == nil {
			copyAction := action
			sameCategory = &copyAction
		}
	}
	if sameCategory != nil {
		return *sameCategory, true
	}
	return TopActionItem{}, false
}

func fallbackActionForDriver(driver RiskDriver) TopActionItem {
	action := TopActionItem{
		ID:            driver.ID + "-action",
		Source:        "risk_driver",
		Priority:      driver.Severity,
		PriorityLabel: topActionPriorityLabel(driver.Severity),
		Category:      topActionCategoryForDriver(driver.Category),
		CategoryLabel: topActionCategoryLabel(topActionCategoryForDriver(driver.Category)),
		Reason:        driver.Reason,
	}
	switch driver.Category {
	case "cancellations":
		action.Title = "Разберите рост отмен"
		action.NextStep = "Проверьте товары и статусы с наибольшим числом отмен, найдите источник проблемы и уберите причину повторных отмен."
	case "funnel":
		action.Title = "Проверьте провал воронки продаж"
		action.NextStep = "Разберите этап с наибольшим падением: карточка, корзина или заказ. Сверьте карточки, цены, наличие и качество трафика."
	case "stock":
		action.Title = "Проверьте товары с риском дефицита"
		action.NextStep = "Проверьте критичные SKU, подтвердите сроки пополнения и не усиливайте трафик по позициям, которые скоро закончатся."
	case "surplus":
		action.Title = "Разберите товары с избыточным остатком"
		action.NextStep = "Проверьте спрос, текущие остатки и аккуратные способы ускорить оборачиваемость без потери маржи."
	case "advertising":
		action.Title = "Сократите рекламные расходы с низкой отдачей"
		action.NextStep = "Остановите кампании без заказов и временно снизьте бюджет там, где расход не окупается."
	case "sales":
		action.Title = "Разберите причину падения продаж"
		action.NextStep = "Проверьте SKU, которые сильнее всего тянут вниз выручку, и сверяйте продажи с остатками, ценой и источниками трафика."
	default:
		action.Title = "Проверьте главный риск кабинета"
		action.NextStep = "Сфокусируйтесь на наиболее сильном отклонении по кабинету и устраните его до следующей сводки."
	}
	action.SupportingMetrics = supportingMetricsFromDriver(driver)
	return action
}

func supportingMetricsFromDriver(driver RiskDriver) []TopActionSupportingMetric {
	metrics := make([]TopActionSupportingMetric, 0, 3)
	appendMetric := func(label string, value string) {
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if label == "" || value == "" {
			return
		}
		metrics = append(metrics, TopActionSupportingMetric{Label: label, Value: value})
	}
	switch driver.Category {
	case "sales":
		if v, ok := evidenceFloat(driver.Evidence, "revenue_day_delta_pct"); ok {
			appendMetric("Ко вчера", describeSignedPercent(v))
		}
		if v, ok := evidenceFloat(driver.Evidence, "revenue_week_delta_pct"); ok {
			appendMetric("К 7 дням", describeSignedPercent(v))
		}
	case "cancellations":
		if v, ok := evidenceFloat(driver.Evidence, "cancellation_rate"); ok {
			appendMetric("Доля отмен", formatPercent(v))
		}
		if v, ok := evidenceInt(driver.Evidence, "cancels_today"); ok {
			appendMetric("Отмены", formatInt(v))
		}
	case "funnel":
		if label, ok := driver.Evidence["weakest_stage"].(string); ok && label != "" {
			appendMetric("Слабый этап", label)
		}
		if v, ok := evidenceFloat(driver.Evidence, "weakest_stage_change"); ok {
			appendMetric("Изменение", "-"+formatPercent(math.Abs(v)))
		}
	case "stock":
		if v, ok := evidenceInt(driver.Evidence, "critical_0_3_skus"); ok {
			appendMetric("0-3 дня", formatInt(v))
		}
		if v, ok := evidenceInt(driver.Evidence, "shortage_4_7_skus"); ok {
			appendMetric("4-7 дней", formatInt(v))
		}
	case "surplus":
		if v, ok := evidenceInt(driver.Evidence, "surplus_sku_count"); ok {
			appendMetric("SKU в риске", formatInt(v))
		}
	case "advertising":
		if v, ok := evidenceInt(driver.Evidence, "campaigns_at_risk"); ok {
			appendMetric("Кампании", formatInt(v))
		}
		if v, ok := evidenceFloat(driver.Evidence, "spend_at_risk"); ok {
			appendMetric("Расход в риске", formatMoneyAmount(v))
		}
	}
	return compactSupportingMetrics(metrics)
}

func computeHealthScore(drivers []RiskDriver) int32 {
	return computeHealthScoreBreakdown(drivers).FinalScore
}

// ComputeHealthScoreBreakdown is the versioned publication formula for cabinet
// health_score. Published Unified dashboards must use this backend result
// (with drivers/breakdown), not an AI-invented score.
func ComputeHealthScoreBreakdown(drivers []RiskDriver) HealthScoreBreakdown {
	return computeHealthScoreBreakdown(drivers)
}

func computeHealthScoreBreakdown(drivers []RiskDriver) HealthScoreBreakdown {
	breakdown := HealthScoreBreakdown{
		Drivers: []HealthScoreBreakdownItem{},
	}
	if len(drivers) == 0 {
		breakdown.FinalScore = 88
		return breakdown
	}

	rawPenalty := 0.0
	items := make([]HealthScoreBreakdownItem, 0, len(drivers))
	domainPenalty := map[string]float64{}
	hasSalesDecline := false
	hasRevenueDecline := false
	for i, driver := range drivers {
		if driver.ID == "sales_decline" {
			hasSalesDecline = true
		}
		if driver.ID == "revenue_decline" {
			hasRevenueDecline = true
		}
		weight := healthScoreWeights[minInt(i, len(healthScoreWeights)-1)]
		severity := normalizeTopActionPriority(driver.Severity)
		severityBase := healthSeverityBase[severity]
		impact := float64(clampInt(driver.ImpactScore, 0, 100))
		impactPart := 14 * math.Sqrt(impact/100)
		rawDriverPenalty := (severityBase + impactPart) * weight
		cappedDriverPenalty := math.Min(rawDriverPenalty, 28)

		// Domain caps prevent stacking stockout + key_sku_stock + surplus + no_sales
		// (or multiple ad drivers) into an automatic critical score when risks are local.
		domain := strings.TrimSpace(driver.Category)
		if domain == "" {
			domain = "other"
		}
		switch domain {
		case "surplus", "sku", "slow_turnover":
			// Inventory-side issues share one inventory budget so a healthy growing
			// cabinet is not marked critical solely by overlapping stock taxonomies.
			domain = "stock"
		}
		const domainCap = 36.0
		remaining := domainCap - domainPenalty[domain]
		if remaining < 0 {
			remaining = 0
		}
		applied := math.Min(cappedDriverPenalty, remaining)
		// When sales are not declining, soft-weight pure surplus so it cannot alone
		// drag a growing cabinet into critical.
		if !hasSalesDecline && !hasRevenueDecline && driver.ID == "surplus" {
			applied = applied * 0.45
		}
		domainPenalty[domain] += applied
		rawPenalty += applied

		items = append(items, HealthScoreBreakdownItem{
			DriverID:            driver.ID,
			Category:            driver.Category,
			Severity:            severity,
			ImpactScore:         driver.ImpactScore,
			Weight:              roundBreakdownFloat(weight),
			SeverityBase:        roundBreakdownFloat(severityBase),
			ImpactPart:          roundBreakdownFloat(impactPart),
			RawDriverPenalty:    roundBreakdownFloat(rawDriverPenalty),
			CappedDriverPenalty: roundBreakdownFloat(applied),
		})
	}

	saturatedPenalty := 90 * (1 - math.Exp(-rawPenalty/60))
	score := int32(clampInt(int(math.Round(100-saturatedPenalty)), 10, 95))
	// Growing cabinets without sales/revenue decline drivers recover toward "attention"
	// rather than "critical" when remaining risks are localized.
	if !hasSalesDecline && !hasRevenueDecline {
		score = int32(clampInt(int(score)+22, 10, 95))
	}
	breakdown.Drivers = items
	breakdown.RawPenalty = roundBreakdownFloat(rawPenalty)
	breakdown.SaturatedPenalty = roundBreakdownFloat(saturatedPenalty)
	breakdown.FinalScore = score
	return breakdown
}

func computeHealthSeverity(score int32) string {
	switch {
	case score >= 85:
		return "low"
	case score >= 70:
		return "attention"
	case score >= 50:
		return "medium"
	case score >= 30:
		return "high"
	default:
		return "critical"
	}
}

func buildRiskHeadline(drivers []RiskDriver) string {
	top := topDriverLabels(drivers, 3)
	switch len(top) {
	case 0:
		return "Кабинет без выраженных рисков"
	case 1:
		return fmt.Sprintf("Главный риск дня: %s.", top[0])
	default:
		return fmt.Sprintf("Главные риски дня: %s.", joinTopics(top))
	}
}

func buildRiskExecutiveSummary(drivers []RiskDriver) string {
	if len(drivers) == 0 {
		return "Индекс строится по метрикам кабинета в целом: продажам, воронке, остаткам, отменам и рекламе."
	}
	top := topDriverLabels(drivers, 2)
	switch len(top) {
	case 0:
		return "Индекс сейчас снижен из-за нескольких рисков по кабинету. Подробные шаги — в блоке «Топ-3 действия»."
	case 1:
		return normalizeSellerFacingSentences(
			fmt.Sprintf("Сейчас индекс сильнее всего снижает риск по направлению «%s». Подробные шаги — в блоке «Топ-3 действия».", top[0]),
		)
	default:
		return normalizeSellerFacingSentences(
			fmt.Sprintf("Сейчас индекс сильнее всего снижают риски по направлениям %s. Подробные шаги — в блоке «Топ-3 действия».", joinTopics(top)),
		)
	}
}

func buildRiskImprovementSummary(drivers []RiskDriver) string {
	topics := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, driver := range drivers {
		topic := improvementTopicForDriver(driver.Category)
		if topic == "" {
			continue
		}
		if _, exists := seen[topic]; exists {
			continue
		}
		seen[topic] = struct{}{}
		topics = append(topics, topic)
		if len(topics) >= 3 {
			break
		}
	}
	if len(topics) == 0 {
		return "Индекс улучшится после устранения основных рисков по кабинету. Конкретные шаги — в блоке «Топ-3 действия»."
	}
	return fmt.Sprintf("Индекс улучшится после %s. Конкретные шаги — в блоке «Топ-3 действия».", joinTopics(topics))
}

func riskDriverCategoryLabel(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "sales":
		return "Продажи"
	case "cancellations":
		return "Отмены"
	case "funnel":
		return "Воронка"
	case "stock":
		return "Остатки"
	case "surplus":
		return "Медленная оборачиваемость"
	case "slow_turnover":
		return "Медленная оборачиваемость"
	case "advertising":
		return "Реклама"
	case "sku":
		return "SKU"
	default:
		return "Кабинет"
	}
}

func topActionCategoryForDriver(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "cancellations":
		return "cancellations"
	case "funnel":
		return "funnel"
	case "sales":
		return "sales"
	case "stock", "surplus":
		return "stock"
	case "slow_turnover":
		return "slow_turnover"
	case "advertising":
		return "advertising"
	case "sku":
		return "products"
	default:
		return "account"
	}
}

func linkedRecommendationIDsForDriverCategory(rows []dbgen.Recommendation, driverCategory string) []int64 {
	out := make([]int64, 0, len(rows))
	for _, row := range rows {
		if driverCategoryForRecommendationType(row.RecommendationType) != driverCategory {
			continue
		}
		out = append(out, row.ID)
	}
	return compactInt64s(out)
}

func linkedAlertIDsForDriverCategory(rows []dbgen.Alert, driverCategory string) []int64 {
	out := make([]int64, 0, len(rows))
	for _, row := range rows {
		if driverCategoryForAlertType(row.AlertType) != driverCategory {
			continue
		}
		out = append(out, row.ID)
	}
	return compactInt64s(out)
}

func driverCategoryForRecommendationType(raw string) string {
	switch recommendationTypeKey(raw) {
	case "review_ad_spend", "pause_or_reduce_ads":
		return "advertising"
	case "avoid_ads_for_low_stock_sku", "replenish_sku":
		return "stock"
	case "discount_overstock":
		return "surplus"
	case "investigate_sales_drop":
		return "sales"
	case "monitor_sku":
		return "sku"
	default:
		return "account"
	}
}

func driverCategoryForAlertType(raw string) string {
	switch alertTypeKey(raw) {
	case string(alerts.AlertTypeAdWeakCampaignEfficiency):
		return "advertising"
	case string(alerts.AlertTypeStockLowCoverage), string(alerts.AlertTypeStockOOSRisk):
		return "stock"
	case string(alerts.AlertTypeFBOSurplusStorageRisk):
		return "surplus"
	case string(alerts.AlertTypeSalesRevenueDrop), string(alerts.AlertTypeSalesOrdersDrop):
		return "sales"
	case string(alerts.AlertTypeSKUOrdersDrop):
		return "sku"
	default:
		return "account"
	}
}

func improvementTopicForDriver(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "sales":
		return "восстановления выручки и заказов"
	case "cancellations":
		return "снижения доли отмен"
	case "funnel":
		return "устранения провала воронки"
	case "stock":
		return "пополнения критичных SKU"
	case "surplus":
		return "снижения избыточных остатков"
	case "advertising":
		return "сокращения рекламных расходов с низкой отдачей"
	default:
		return ""
	}
}

func stockCoverageCounts(segments []StockCoverageSegment) (critical int, shortage int) {
	for _, segment := range segments {
		switch segment.Key {
		case "critical_0_3":
			critical = segment.SKUCount
		case "shortage_4_7":
			shortage = segment.SKUCount
		}
	}
	return critical, shortage
}

func buildStockRiskReason(critical int, shortage int, firstSKU string) string {
	switch {
	case critical > 0 && shortage > 0:
		reason := fmt.Sprintf("Есть %d SKU с критичным остатком и ещё %d SKU с покрытием 4–7 дней.", critical, shortage)
		if firstSKU != "" {
			reason += fmt.Sprintf(" В первую очередь проверьте «%s».", firstSKU)
		}
		return reason
	case critical > 0:
		reason := fmt.Sprintf("Есть %d SKU с критичным остатком или без запаса.", critical)
		if firstSKU != "" {
			reason += fmt.Sprintf(" В первую очередь проверьте «%s».", firstSKU)
		}
		return reason
	default:
		reason := fmt.Sprintf("Есть %d SKU с покрытием 4–7 дней.", shortage)
		if firstSKU != "" {
			reason += fmt.Sprintf(" В первую очередь проверьте «%s».", firstSKU)
		}
		return reason
	}
}

func weakestFunnelStage(stages []FunnelStage) (FunnelStage, float64, bool) {
	var weakest FunnelStage
	weakestDrop := 0.0
	for _, stage := range stages {
		if stage.ChangeVsPreviousDayPct == nil || *stage.ChangeVsPreviousDayPct >= 0 {
			continue
		}
		if stage.Key == "impressions" {
			continue
		}
		if !isFiniteFloat(*stage.ChangeVsPreviousDayPct) {
			continue
		}
		if *stage.ChangeVsPreviousDayPct < weakestDrop {
			weakest = stage
			weakestDrop = *stage.ChangeVsPreviousDayPct
		}
	}
	return weakest, weakestDrop, weakest.Label != ""
}

func topRevenueDeclineSKU(items []analytics.DashboardSKUMetricsItem) *analytics.DashboardSKUMetricsItem {
	var best *analytics.DashboardSKUMetricsItem
	bestLoss := 0.0
	for i := range items {
		item := items[i]
		if item.ContributionToRevenueChange >= 0 {
			continue
		}
		loss := math.Abs(item.ContributionToRevenueChange)
		if best == nil || loss > bestLoss {
			copyItem := item
			best = &copyItem
			bestLoss = loss
		}
	}
	return best
}

func topSKURevenueDropImpact(items []analytics.DashboardSKUMetricsItem) int {
	top := topRevenueDeclineSKU(items)
	if top == nil {
		return 0
	}
	share := 0.0
	if top.ShareOfRevenue != nil {
		share = *top.ShareOfRevenue * 100
	}
	return clampInt(int(math.Round(share/2))+int(math.Abs(top.ContributionToRevenueChange)/5000), 0, 20)
}

func topDriverLabels(drivers []RiskDriver, limit int) []string {
	labels := make([]string, 0, limit)
	for _, driver := range drivers {
		if len(labels) >= limit {
			break
		}
		labels = append(labels, driver.CategoryLabel)
	}
	return labels
}

func compactInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := map[int64]struct{}{}
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func overlapsInt64(left []int64, right []int64) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := map[int64]struct{}{}
	for _, value := range left {
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, exists := seen[value]; exists {
			return true
		}
	}
	return false
}

func hasAlertType(rows []dbgen.Alert, alertType string) bool {
	for _, row := range rows {
		if alertTypeKey(row.AlertType) == alertTypeKey(alertType) {
			return true
		}
	}
	return false
}

func evidenceFloat(evidence map[string]any, key string) (float64, bool) {
	return metricFloat(evidence, key)
}

func evidenceInt(evidence map[string]any, key string) (int64, bool) {
	return metricInt(evidence, key)
}

func safePercent(value *float64) float64 {
	if value == nil || !isFiniteFloat(*value) {
		return 0
	}
	return math.Round(*value*1000) / 10
}

func ratioToPercent(value *float64) float64 {
	return safePercent(value)
}

func intChangePercent(today int32, yesterday int32) float64 {
	if yesterday <= 0 {
		if today <= 0 {
			return 0
		}
		return 100
	}
	return math.Round(((float64(today)-float64(yesterday))/float64(yesterday))*1000) / 10
}

func severityFromScore(value float64, critical, high, medium float64) string {
	switch {
	case value >= critical:
		return "critical"
	case value >= high:
		return "high"
	case value >= medium:
		return "medium"
	default:
		return "low"
	}
}

func severityFromInts(criticalCount int, shortageCount int) string {
	switch {
	case criticalCount >= 3:
		return "critical"
	case criticalCount > 0 || shortageCount >= 4:
		return "high"
	case shortageCount > 0:
		return "medium"
	default:
		return "low"
	}
}

func describeSignedPercent(value float64) string {
	switch {
	case !isFiniteFloat(value):
		return "без заметного изменения"
	case value <= -0.05:
		return "на " + formatPercent(math.Abs(value)) + " ниже"
	case value >= 0.05:
		return "на " + formatPercent(value) + " выше"
	default:
		return "на уровне предыдущего периода"
	}
}

func firstSKUName(cards []SKUFocusCard) string {
	if len(cards) == 0 {
		return ""
	}
	return strings.TrimSpace(cards[0].ProductName)
}

func campaignNameOrEmpty(point *AdEfficiencyPoint) string {
	if point == nil {
		return ""
	}
	return strings.TrimSpace(point.CampaignName)
}

func productNameOrEmpty(item *analytics.DashboardSKUMetricsItem) string {
	if item == nil {
		return ""
	}
	return derefStr(item.ProductName, "")
}

func productRevenueDeltaOrZero(item *analytics.DashboardSKUMetricsItem) float64 {
	if item == nil {
		return 0
	}
	return item.ContributionToRevenueChange
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func joinTopics(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + " и " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " и " + parts[len(parts)-1]
}

func isFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func roundBreakdownFloat(value float64) float64 {
	return math.Round(value*100) / 100
}

func shouldExposeHealthScoreBreakdown() bool {
	// Breakdown is part of the seller-facing health contract (score must be explainable).
	return true
}

func russianCountWord(count int, singular string, paucal string, plural string) string {
	n := count % 100
	if n >= 11 && n <= 14 {
		return plural
	}
	switch count % 10 {
	case 1:
		return singular
	case 2, 3, 4:
		return paucal
	default:
		return plural
	}
}
