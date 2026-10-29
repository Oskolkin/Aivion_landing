package dashboardhome

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

type topActionCandidate struct {
	Source             string
	Priority           string
	RelatedEntityType  string
	RelatedEntityID    *string
	RecommendationType string
	AlertType          string
	Title              string
	WhatHappened       string
	WhyItMatters       string
	NextStep           string
	SupportingMetrics  map[string]any
	LinkedAlertID      *int64
	LinkedRecID        *int64
}

type topActionContext struct {
	alertEvidenceByEntity map[string][]map[string]any
	adByCampaignID        map[string]AdEfficiencyPoint
}

var (
	topActionRatioPattern      = regexp.MustCompile(`(?i)\b(\d+(?:[.,]\d+)?)\s*x\b`)
	topActionRatioRangePattern = regexp.MustCompile(`(?i)\b\d+(?:[.,]\d+)?\s*x\s*[–-]\s*\d+(?:[.,]\d+)?\s*x\b`)
	topActionTokenRules        = []struct {
		pattern *regexp.Regexp
		value   string
	}{
		{pattern: regexp.MustCompile(`(?i)\bad_waste\b`), value: "неэффективные рекламные расходы"},
		{pattern: regexp.MustCompile(`(?i)\bad_weak_sku\b`), value: "товар с неэффективной рекламой"},
		{pattern: regexp.MustCompile(`(?i)\bad_weak_campaign_efficiency\b`), value: "слабая эффективность рекламы"},
		{pattern: regexp.MustCompile(`(?i)\bad spend\b`), value: "рекламные расходы"},
		{pattern: regexp.MustCompile(`(?i)\bcampaign\b`), value: "рекламная кампания"},
		{pattern: regexp.MustCompile(`(?i)\bROAS\b`), value: "окупаемость рекламы"},
		{pattern: regexp.MustCompile(`(?i)\bCPO\b`), value: "стоимость заказа"},
		{pattern: regexp.MustCompile(`(?i)\bCTR\b`), value: "кликабельность"},
		{pattern: regexp.MustCompile(`(?i)\bCPC\b`), value: "стоимость клика"},
		{pattern: regexp.MustCompile(`(?i)\bhigh\b`), value: "высокий"},
		{pattern: regexp.MustCompile(`(?i)\bcritical\b`), value: "критический"},
		{pattern: regexp.MustCompile(`(?i)\bmedium\b`), value: "средний"},
		{pattern: regexp.MustCompile(`(?i)\blow\b`), value: "низкий"},
	}
)

func buildTopActions(
	aiItems []ActionItem,
	recommendationRows []dbgen.Recommendation,
	alertRows []dbgen.Alert,
	adPoints []AdEfficiencyPoint,
	limit int,
) []TopActionItem {
	return buildTopActionPool(aiItems, recommendationRows, alertRows, adPoints, limit)
}

func buildTopActionPool(
	aiItems []ActionItem,
	recommendationRows []dbgen.Recommendation,
	alertRows []dbgen.Alert,
	adPoints []AdEfficiencyPoint,
	limit int,
) []TopActionItem {
	candidates := make([]topActionCandidate, 0, len(aiItems)+len(recommendationRows)+len(alertRows))
	for _, row := range recommendationRows {
		candidates = append(candidates, topActionCandidateFromRecommendation(row))
	}
	for _, item := range aiItems {
		candidates = append(candidates, topActionCandidateFromAISummary(item))
	}
	for _, row := range alertRows {
		candidates = append(candidates, topActionCandidateFromAlert(row))
	}
	return normalizeTopActionFeed(selectTopActionCandidates(candidates), buildTopActionContext(alertRows, adPoints), limit)
}

func buildTopActionContext(alertRows []dbgen.Alert, adPoints []AdEfficiencyPoint) topActionContext {
	ctx := topActionContext{
		alertEvidenceByEntity: make(map[string][]map[string]any),
		adByCampaignID:        make(map[string]AdEfficiencyPoint),
	}
	for _, row := range alertRows {
		key := topActionEntityKey(row.EntityType, textPtr(row.EntityID))
		if key != "||" {
			ctx.alertEvidenceByEntity[key] = append(ctx.alertEvidenceByEntity[key], decodeJSONMap(row.EvidencePayload))
		}
	}
	for _, point := range adPoints {
		ctx.adByCampaignID[strconv.FormatInt(point.CampaignExternalID, 10)] = point
	}
	return ctx
}

func topActionCandidateFromRecommendation(row dbgen.Recommendation) topActionCandidate {
	id := row.ID
	return topActionCandidate{
		Source:             "recommendation",
		Priority:           normalizeTopActionPriority(row.PriorityLevel),
		RelatedEntityType:  row.EntityType,
		RelatedEntityID:    textPtr(row.EntityID),
		RecommendationType: recommendationTypeKey(row.RecommendationType),
		Title:              strings.TrimSpace(row.Title),
		WhatHappened:       strings.TrimSpace(row.WhatHappened),
		WhyItMatters:       strings.TrimSpace(row.WhyItMatters),
		NextStep:           strings.TrimSpace(row.RecommendedAction),
		SupportingMetrics:  decodeJSONMap(row.SupportingMetricsPayload),
		LinkedRecID:        &id,
	}
}

func topActionCandidateFromAISummary(item ActionItem) topActionCandidate {
	return topActionCandidate{
		Source:            "ai_summary",
		Priority:          normalizeTopActionPriority(item.Priority),
		RelatedEntityType: item.RelatedEntityType,
		RelatedEntityID:   item.RelatedEntityID,
		Title:             strings.TrimSpace(item.Action),
		WhatHappened:      strings.TrimSpace(item.Reason),
		NextStep:          strings.TrimSpace(item.ExpectedEffect),
	}
}

func topActionCandidateFromAlert(row dbgen.Alert) topActionCandidate {
	id := row.ID
	return topActionCandidate{
		Source:            "alert",
		Priority:          normalizeTopActionPriority(severityToPriority(row.Severity)),
		RelatedEntityType: row.EntityType,
		RelatedEntityID:   textPtr(row.EntityID),
		AlertType:         alertTypeKey(row.AlertType),
		Title:             strings.TrimSpace(row.Title),
		WhatHappened:      strings.TrimSpace(row.Message),
		SupportingMetrics: decodeJSONMap(row.EvidencePayload),
		LinkedAlertID:     &id,
	}
}

func selectTopActionCandidates(candidates []topActionCandidate) []topActionCandidate {
	if len(candidates) == 0 {
		return nil
	}
	sortCandidatesBySourcePriority(candidates)
	seen := make(map[string]struct{}, len(candidates))
	out := make([]topActionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		key := topActionCandidateDedupKey(candidate)
		if key == "||" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func normalizeTopActionFeed(candidates []topActionCandidate, ctx topActionContext, limit int) []TopActionItem {
	if len(candidates) == 0 || limit <= 0 {
		return []TopActionItem{}
	}
	out := make([]TopActionItem, 0, limit)
	for _, candidate := range candidates {
		item := normalizeTopActionCandidate(candidate, ctx)
		if !isValidTopActionItem(item) {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	sortTopActionsByPriority(out)
	return out
}

func sortCandidatesBySourcePriority(candidates []topActionCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		pi := actionSourcePriority[strings.ToLower(candidates[i].Source)]
		pj := actionSourcePriority[strings.ToLower(candidates[j].Source)]
		if pi != pj {
			return pi < pj
		}
		return priorityRank(candidates[i].Priority) < priorityRank(candidates[j].Priority)
	})
}

func sortTopActionsByPriority(items []TopActionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		return priorityRank(items[i].Priority) < priorityRank(items[j].Priority)
	})
}

func topActionCandidateDedupKey(candidate topActionCandidate) string {
	entityType := strings.ToLower(strings.TrimSpace(candidate.RelatedEntityType))
	entityID := ""
	if candidate.RelatedEntityID != nil {
		entityID = strings.ToLower(strings.TrimSpace(*candidate.RelatedEntityID))
	}
	kind := firstNonEmpty(
		recommendationTypeKey(candidate.RecommendationType),
		alertTypeKey(candidate.AlertType),
		normalizeActionKey(candidate.NextStep),
		normalizeActionKey(candidate.Title),
		normalizeActionKey(candidate.WhatHappened),
	)
	return entityType + "|" + entityID + "|" + kind
}

func topActionEntityKey(entityType string, entityID *string) string {
	normalizedEntityType := strings.ToLower(strings.TrimSpace(entityType))
	normalizedEntityID := ""
	if entityID != nil {
		normalizedEntityID = strings.ToLower(strings.TrimSpace(*entityID))
	}
	return normalizedEntityType + "|" + normalizedEntityID
}

func normalizeTopActionCandidate(candidate topActionCandidate, ctx topActionContext) TopActionItem {
	candidate = enrichTopActionCandidate(candidate, ctx)
	priority := normalizeTopActionPriority(candidate.Priority)
	category := detectTopActionCategory(candidate)

	var item TopActionItem
	switch category {
	case "advertising":
		item = normalizeAdvertisingTopAction(candidate)
	case "stock":
		item = normalizeStockTopAction(candidate)
	case "sales":
		item = normalizeSalesTopAction(candidate)
	case "products":
		item = normalizeProductsTopAction(candidate)
	case "account":
		item = normalizeAccountTopAction(candidate)
	default:
		item = normalizeOtherTopAction(candidate)
	}

	item.ID = topActionID(candidate, item)
	if candidate.LinkedAlertID != nil {
		item.LinkedAlertIDs = []int64{*candidate.LinkedAlertID}
	}
	if candidate.LinkedRecID != nil {
		item.LinkedRecommendationIDs = []int64{*candidate.LinkedRecID}
	}
	item.Source = candidate.Source
	item.Priority = priority
	item.PriorityLabel = topActionPriorityLabel(priority)
	item.Category = category
	item.CategoryLabel = topActionCategoryLabel(category)
	item.Title = cleanupSellerFacingText(item.Title)
	item.Reason = cleanupSellerFacingText(item.Reason)
	item.NextStep = cleanupSellerFacingText(item.NextStep)
	for i := range item.SupportingMetrics {
		item.SupportingMetrics[i].Label = cleanupSellerFacingText(item.SupportingMetrics[i].Label)
		item.SupportingMetrics[i].Value = cleanupSellerFacingText(item.SupportingMetrics[i].Value)
	}
	item.SupportingMetrics = compactSupportingMetrics(item.SupportingMetrics)
	return item
}

func normalizeAdvertisingTopAction(candidate topActionCandidate) TopActionItem {
	name := metricString(candidate.SupportingMetrics, "campaign_name", "campaign")
	spend, hasSpend := metricFloat(candidate.SupportingMetrics, "spend", "ad_spend", "spend_total")
	revenue, hasRevenue := metricFloat(candidate.SupportingMetrics, "revenue", "revenue_total")
	orders, hasOrders := metricInt(candidate.SupportingMetrics, "orders_count", "orders", "orders_total")
	ratio, hasRatio := advertisingRevenuePerRuble(candidate.SupportingMetrics, spend, hasSpend, revenue, hasRevenue)

	if advertisingHasZeroResult(candidate, revenue, hasRevenue, orders, hasOrders) && hasSpend {
		reason := fmt.Sprintf("Кампания потратила %s за день, но не принесла заказов и выручки.", formatMoneyAmount(spend))
		if name != "" {
			reason = fmt.Sprintf("Кампания «%s» потратила %s за день, но не принесла заказов и выручки.", name, formatMoneyAmount(spend))
		}
		return TopActionItem{
			Title:    "Остановите неэффективную рекламную кампанию",
			Reason:   reason,
			NextStep: "Проверьте поисковые запросы, карточки товаров и площадки показа. Если причин продолжать рекламу нет — остановите кампанию или резко сократите бюджет.",
			SupportingMetrics: buildAdvertisingSupportingMetrics(
				spend, true,
				0, true,
				0, true,
				0, false,
			),
		}
	}

	if hasSpend && hasRevenue && hasRatio && ratio < 0.5 {
		reason := fmt.Sprintf(
			"Кампания потратила %s и принесла %s выручки. На каждый 1 ₽ рекламных расходов пришлось %s.",
			formatMoneyAmount(spend),
			formatMoneyAmount(revenue),
			formatRevenuePerAdRubleShort(ratio),
		)
		if name != "" {
			reason = fmt.Sprintf(
				"Кампания «%s» потратила %s и принесла %s выручки. На каждый 1 ₽ рекламных расходов пришлось %s.",
				name,
				formatMoneyAmount(spend),
				formatMoneyAmount(revenue),
				formatRevenuePerAdRubleShort(ratio),
			)
		}
		return TopActionItem{
			Title:    "Сократите бюджет кампании с низкой отдачей",
			Reason:   reason,
			NextStep: "Проверьте поисковые запросы, товары в рекламе и нерелевантные показы. До проверки лучше снизить бюджет.",
			SupportingMetrics: buildAdvertisingSupportingMetrics(
				spend, true,
				revenue, true,
				orders, hasOrders,
				ratio, true,
			),
		}
	}

	if hasSpend && hasRevenue && hasRatio && ratio < 1 {
		reason := fmt.Sprintf(
			"Кампания потратила %s и принесла %s выручки. На каждый 1 ₽ рекламных расходов пришлось %s.",
			formatMoneyAmount(spend),
			formatMoneyAmount(revenue),
			formatRevenuePerAdRubleShort(ratio),
		)
		if name != "" {
			reason = fmt.Sprintf(
				"Кампания «%s» потратила %s и принесла %s выручки. На каждый 1 ₽ рекламных расходов пришлось %s.",
				name,
				formatMoneyAmount(spend),
				formatMoneyAmount(revenue),
				formatRevenuePerAdRubleShort(ratio),
			)
		}
		return TopActionItem{
			Title:    "Поставьте рекламную кампанию на контроль",
			Reason:   reason,
			NextStep: "Временно ограничьте бюджет и проверьте, какие запросы, товары или площадки дают слабую отдачу.",
			SupportingMetrics: buildAdvertisingSupportingMetrics(
				spend, true,
				revenue, true,
				orders, hasOrders,
				ratio, true,
			),
		}
	}

	reason := "Рекламная кампания требует проверки: текущая отдача ниже ожидаемой."
	if name != "" {
		reason = fmt.Sprintf("Кампания «%s» требует проверки: текущая отдача ниже ожидаемой.", name)
	}
	if hasSpend {
		reason = fmt.Sprintf("Кампания уже потратила %s и требует проверки эффективности.", formatMoneyAmount(spend))
		if name != "" {
			reason = fmt.Sprintf("Кампания «%s» уже потратила %s и требует проверки эффективности.", name, formatMoneyAmount(spend))
		}
	}

	return TopActionItem{
		Title:    "Поставьте рекламную кампанию на контроль",
		Reason:   reason,
		NextStep: "Временно ограничьте бюджет и проверьте, какие запросы, товары или площадки дают слабую отдачу.",
		SupportingMetrics: buildAdvertisingSupportingMetrics(
			spend, hasSpend,
			revenue, hasRevenue,
			orders, hasOrders,
			ratio, hasRatio,
		),
	}
}

func normalizeStockTopAction(candidate topActionCandidate) TopActionItem {
	days, hasDays := metricFloat(candidate.SupportingMetrics, "days_of_cover", "idc")
	currentStock, hasCurrentStock := metricInt(candidate.SupportingMetrics, "current_stock", "available_stock_count", "stock_available")

	if isSurplusTopAction(candidate) {
		metrics := make([]TopActionSupportingMetric, 0, 2)
		if hasCurrentStock {
			metrics = append(metrics, TopActionSupportingMetric{Label: "Остаток", Value: formatInt(currentStock)})
		}
		if hasDays {
			metrics = append(metrics, TopActionSupportingMetric{Label: "Хватит примерно на", Value: formatDaysOfCover(days)})
		}
		return TopActionItem{
			Title:             "Проверьте товар с избыточным остатком",
			Reason:            "У товара есть признаки избыточного хранения: остаток высокий, а продажи идут медленно.",
			NextStep:          "Проверьте спрос, рекламу и возможность аккуратного промо. Не снижайте цену без оценки маржинальности.",
			SupportingMetrics: metrics,
		}
	}

	title := "Запланируйте пополнение товара"
	reason := "По товару есть признаки риска нехватки остатка."
	if hasDays {
		reason = fmt.Sprintf("Текущего остатка хватит примерно на %s продаж.", formatDaysOfCover(days))
		if days <= 7 {
			title = "Проверьте товар с риском дефицита"
		}
	}

	metrics := make([]TopActionSupportingMetric, 0, 2)
	if hasCurrentStock {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Остаток", Value: formatInt(currentStock)})
	}
	if hasDays {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Хватит примерно на", Value: formatDaysOfCover(days)})
	}

	return TopActionItem{
		Title:             title,
		Reason:            reason,
		NextStep:          "Проверьте наличие товара на складе и запланируйте поставку, чтобы не потерять продажи.",
		SupportingMetrics: metrics,
	}
}

func normalizeSalesTopAction(candidate topActionCandidate) TopActionItem {
	deltaPct, hasDelta := metricFloat(candidate.SupportingMetrics, "delta_percent", "orders_delta_percent", "sku_orders_delta_percent")
	currentOrders, hasCurrentOrders := metricInt(candidate.SupportingMetrics, "current_orders", "orders", "orders_count")
	previousOrders, hasPreviousOrders := metricInt(candidate.SupportingMetrics, "previous_orders", "comparison_orders")

	reason := "Продажи снизились по сравнению с предыдущим периодом."
	if hasDelta {
		reason = fmt.Sprintf("Продажи снизились на %s по сравнению с предыдущим периодом.", formatPercent(math.Abs(deltaPct)))
	}

	metrics := make([]TopActionSupportingMetric, 0, 3)
	if hasCurrentOrders {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Текущие заказы", Value: formatInt(currentOrders)})
	}
	if hasPreviousOrders {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Заказы ранее", Value: formatInt(previousOrders)})
	}
	if hasDelta {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Изменение", Value: formatPercent(deltaPct)})
	}

	return TopActionItem{
		Title:             "Разберите причину падения продаж",
		Reason:            reason,
		NextStep:          "Проверьте товары с наибольшей просадкой, остатки, рекламную активность и изменения цены.",
		SupportingMetrics: metrics,
	}
}

func normalizeProductsTopAction(candidate topActionCandidate) TopActionItem {
	if candidate.RecommendationType == "discount_overstock" {
		return TopActionItem{
			Title:    "Проверьте товар с избыточным остатком",
			Reason:   "У товара есть признаки избыточного хранения: остаток высокий, а продажи идут медленно.",
			NextStep: "Проверьте спрос, рекламу и возможность аккуратного промо. Не снижайте цену без оценки маржинальности.",
		}
	}
	return TopActionItem{
		Title:    "Проверьте товар, который требует внимания",
		Reason:   "Товар требует дополнительной проверки по продажам, рекламе или остаткам.",
		NextStep: "Проверьте карточку товара, остатки и недавнюю динамику продаж, чтобы выбрать следующее действие.",
	}
}

func normalizeAccountTopAction(_ topActionCandidate) TopActionItem {
	return TopActionItem{
		Title:    "Проверьте кабинет с высоким приоритетом",
		Reason:   "В кабинете появились сигналы, которые требуют управленческого внимания.",
		NextStep: "Оцените изменения по продажам, рекламе и остаткам и выберите 1-2 приоритетных действия на сегодня.",
	}
}

func normalizeOtherTopAction(candidate topActionCandidate) TopActionItem {
	title := cleanupSellerFacingText(candidate.Title)
	if title == "" {
		title = "Проверьте приоритетное действие"
	}
	return TopActionItem{
		Title:    title,
		Reason:   "Есть сигнал, который стоит проверить в ближайшее время.",
		NextStep: "Проверьте детали сигнала и выберите следующее действие.",
	}
}

func detectTopActionCategory(candidate topActionCandidate) string {
	if category := categoryForRecommendationType(candidate.RecommendationType); category != "" {
		return category
	}
	if category := categoryForAlertType(candidate.AlertType); category != "" {
		return category
	}

	text := strings.ToLower(strings.Join([]string{
		candidate.Title,
		candidate.WhatHappened,
		candidate.WhyItMatters,
		candidate.NextStep,
	}, " "))
	entityType := strings.ToLower(strings.TrimSpace(candidate.RelatedEntityType))

	switch {
	case entityType == "campaign" || strings.Contains(text, "реклам") || strings.Contains(text, "кампан"):
		return "advertising"
	case strings.Contains(text, "остат") || strings.Contains(text, "дефицит") || strings.Contains(text, "пополн"):
		return "stock"
	case strings.Contains(text, "продаж") || strings.Contains(text, "заказ"):
		return "sales"
	case entityType == "account" || strings.Contains(text, "кабинет"):
		return "account"
	case entityType == "sku" || entityType == "product" || strings.Contains(text, "товар") || strings.Contains(text, "артикул"):
		return "products"
	default:
		return "other"
	}
}

func categoryForRecommendationType(raw string) string {
	switch recommendationTypeKey(raw) {
	case "review_ad_spend", "pause_or_reduce_ads":
		return "advertising"
	case "replenish_sku", "avoid_ads_for_low_stock_sku":
		return "stock"
	case "investigate_sales_drop":
		return "sales"
	case "monitor_sku":
		return "products"
	case "discount_overstock":
		return "stock"
	case "account_priority_review":
		return "account"
	default:
		return ""
	}
}

func categoryForAlertType(raw string) string {
	switch alertTypeKey(raw) {
	case string(alerts.AlertTypeAdWeakCampaignEfficiency):
		return "advertising"
	case string(alerts.AlertTypeStockLowCoverage), string(alerts.AlertTypeStockOOSRisk), string(alerts.AlertTypeFBOSurplusStorageRisk):
		return "stock"
	case string(alerts.AlertTypeSalesRevenueDrop), string(alerts.AlertTypeSalesOrdersDrop), string(alerts.AlertTypeSKUOrdersDrop):
		return "sales"
	default:
		return ""
	}
}

func isSurplusTopAction(candidate topActionCandidate) bool {
	switch {
	case candidate.AlertType == string(alerts.AlertTypeFBOSurplusStorageRisk):
		return true
	case candidate.RecommendationType == "discount_overstock":
		return true
	default:
		text := strings.ToLower(strings.Join([]string{candidate.Title, candidate.WhatHappened, candidate.WhyItMatters}, " "))
		return strings.Contains(text, "избыточ") || strings.Contains(text, "surplus") || strings.Contains(text, "overstock")
	}
}

func advertisingHasZeroResult(candidate topActionCandidate, revenue float64, hasRevenue bool, orders int64, hasOrders bool) bool {
	reason := metricString(candidate.SupportingMetrics, "reason")
	if strings.Contains(reason, "without_revenue") || strings.Contains(reason, "without_orders") {
		return true
	}
	return (!hasRevenue || revenue <= 0) && (!hasOrders || orders <= 0)
}

func advertisingRevenuePerRuble(metrics map[string]any, spend float64, hasSpend bool, revenue float64, hasRevenue bool) (float64, bool) {
	if v, ok := metricFloat(metrics, "revenue_per_ad_ruble", "roas"); ok && v > 0 {
		return v, true
	}
	if drr, ok := metricFloat(metrics, "drr"); ok && drr > 0 {
		return 1 / drr, true
	}
	if hasSpend && spend > 0 && hasRevenue {
		return revenue / spend, true
	}
	return 0, false
}

func enrichTopActionCandidate(candidate topActionCandidate, ctx topActionContext) topActionCandidate {
	merged := cloneMetrics(candidate.SupportingMetrics)
	entityKey := topActionEntityKey(candidate.RelatedEntityType, candidate.RelatedEntityID)
	if evidenceRows, ok := ctx.alertEvidenceByEntity[entityKey]; ok {
		for _, evidence := range evidenceRows {
			merged = mergeMetrics(merged, evidence)
		}
	}
	if strings.EqualFold(strings.TrimSpace(candidate.RelatedEntityType), "campaign") && candidate.RelatedEntityID != nil {
		if point, ok := ctx.adByCampaignID[strings.TrimSpace(*candidate.RelatedEntityID)]; ok {
			merged = mergeMetrics(merged, adPointMetrics(point))
		}
	}
	candidate.SupportingMetrics = merged
	return candidate
}

func cloneMetrics(metrics map[string]any) map[string]any {
	if len(metrics) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(metrics))
	for key, value := range metrics {
		out[key] = value
	}
	return out
}

func mergeMetrics(base map[string]any, extra map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for key, value := range extra {
		if current, exists := base[key]; exists && !isZeroLikeMetric(current) {
			continue
		}
		base[key] = value
	}
	return base
}

func isZeroLikeMetric(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	default:
		return false
	}
}

func adPointMetrics(point AdEfficiencyPoint) map[string]any {
	metrics := map[string]any{
		"campaign_name": point.CampaignName,
		"spend":         point.Spend,
		"revenue":       point.Revenue,
		"orders_count":  point.Orders,
	}
	if point.DRR != nil {
		metrics["drr"] = *point.DRR
	}
	if point.Spend > 0 {
		metrics["revenue_per_ad_ruble"] = point.Revenue / point.Spend
	}
	return metrics
}

func topActionID(candidate topActionCandidate, item TopActionItem) string {
	if candidate.LinkedRecID != nil {
		return fmt.Sprintf("recommendation-%d", *candidate.LinkedRecID)
	}
	if candidate.LinkedAlertID != nil {
		return fmt.Sprintf("alert-%d", *candidate.LinkedAlertID)
	}
	entityType := strings.ToLower(strings.TrimSpace(candidate.RelatedEntityType))
	entityID := ""
	if candidate.RelatedEntityID != nil {
		entityID = strings.ToLower(strings.TrimSpace(*candidate.RelatedEntityID))
	}
	titleKey := strings.ReplaceAll(normalizeActionKey(item.Title), " ", "-")
	if titleKey == "" {
		titleKey = "item"
	}
	if entityType == "" {
		entityType = "action"
	}
	if entityID == "" {
		entityID = titleKey
	}
	return entityType + "-" + entityID + "-" + titleKey
}

func topActionPriorityLabel(priority string) string {
	switch normalizeTopActionPriority(priority) {
	case "critical":
		return "Критический"
	case "high":
		return "Высокий"
	case "low":
		return "Низкий"
	default:
		return "Средний"
	}
}

func topActionCategoryLabel(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "advertising":
		return "Реклама"
	case "funnel":
		return "Воронка"
	case "cancellations":
		return "Отмены"
	case "stock":
		return "Остатки"
	case "slow_turnover":
		return "Медленная оборачиваемость"
	case "sales":
		return "Продажи"
	case "products":
		return "Товары"
	case "account":
		return "Кабинет"
	default:
		return "Другое"
	}
}

func normalizeTopActionPriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "critical", "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(priority))
	case "urgent":
		return "critical"
	case "watch":
		return "low"
	default:
		return "medium"
	}
}

func recommendationTypeKey(raw string) string {
	if canonical, _, ok := recommendations.NormalizeRecommendationType(raw); ok {
		return canonical
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func alertTypeKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func decodeJSONMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func metricFloat(metrics map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		value, ok := metrics[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int32:
			return float64(v), true
		case int64:
			return float64(v), true
		case json.Number:
			f, err := v.Float64()
			if err == nil {
				return f, true
			}
		case string:
			f, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", "."), 64)
			if err == nil {
				return f, true
			}
		}
	}
	return 0, false
}

func metricInt(metrics map[string]any, keys ...string) (int64, bool) {
	for _, key := range keys {
		if value, ok := metricFloat(metrics, key); ok {
			return int64(math.Round(value)), true
		}
	}
	return 0, false
}

func metricString(metrics map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := metrics[key]
		if !ok || value == nil {
			continue
		}
		switch v := value.(type) {
		case string:
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		default:
			return strings.TrimSpace(fmt.Sprintf("%v", v))
		}
	}
	return ""
}

func buildAdvertisingSupportingMetrics(
	spend float64,
	hasSpend bool,
	revenue float64,
	hasRevenue bool,
	orders int64,
	hasOrders bool,
	ratio float64,
	hasRatio bool,
) []TopActionSupportingMetric {
	metrics := make([]TopActionSupportingMetric, 0, 4)
	if hasSpend {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Расход", Value: formatMoneyAmount(spend)})
	}
	if hasRevenue {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Выручка", Value: formatMoneyAmount(revenue)})
	}
	if hasOrders {
		metrics = append(metrics, TopActionSupportingMetric{Label: "Заказы", Value: formatInt(orders)})
	}
	if hasRatio {
		metrics = append(metrics, TopActionSupportingMetric{
			Label: "Окупаемость рекламы",
			Value: strings.ReplaceAll(strconv.FormatFloat(math.Round(ratio*100)/100, 'f', 2, 64), ".", ",") + " ₽ выручки на 1 ₽ расходов",
		})
	}
	return compactSupportingMetrics(metrics)
}

func formatMoneyAmount(v float64) string {
	rounded := math.Round(v*100) / 100
	if math.Abs(rounded-math.Round(rounded)) < 0.005 {
		return formatGroupedInt(int64(math.Round(rounded))) + " ₽"
	}
	parts := strings.SplitN(strconv.FormatFloat(rounded, 'f', 2, 64), ".", 2)
	parts[0] = formatGroupedDigits(parts[0])
	return strings.ReplaceAll(strings.Join(parts, "."), ".", ",") + " ₽"
}

func formatMoneyRounded(v float64) string {
	return formatGroupedInt(int64(math.Round(v))) + " ₽"
}

func formatDaysOfCover(days float64) string {
	rounded := math.Round(days*10) / 10
	if math.Abs(rounded-math.Round(rounded)) < 0.05 {
		return fmt.Sprintf("%d дней", int64(math.Round(rounded)))
	}
	return strings.ReplaceAll(strconv.FormatFloat(rounded, 'f', 1, 64), ".", ",") + " дней"
}

func formatPercent(v float64) string {
	rounded := math.Round(v*10) / 10
	return strings.ReplaceAll(strconv.FormatFloat(rounded, 'f', 1, 64), ".", ",") + "%"
}

func formatRevenuePerAdRuble(v float64) string {
	rounded := math.Round(v*100) / 100
	return strings.ReplaceAll(strconv.FormatFloat(rounded, 'f', 2, 64), ".", ",") + " ₽ выручки на 1 ₽ рекламных расходов"
}

func formatRevenuePerAdRubleShort(v float64) string {
	rounded := math.Round(v*100) / 100
	return strings.ReplaceAll(strconv.FormatFloat(rounded, 'f', 2, 64), ".", ",") + " ₽ выручки"
}

func formatGroupedInt(v int64) string {
	return formatGroupedDigits(strconv.FormatInt(v, 10))
}

func formatGroupedDigits(digits string) string {
	if digits == "" {
		return ""
	}
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign = "-"
		digits = strings.TrimPrefix(digits, "-")
	}
	if len(digits) <= 3 {
		return sign + digits
	}
	var groups []string
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	if digits != "" {
		groups = append([]string{digits}, groups...)
	}
	return sign + strings.Join(groups, " ")
}

func cleanupSellerFacingText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = topActionRatioRangePattern.ReplaceAllString(text, "более высокой отдачей")
	text = topActionRatioPattern.ReplaceAllStringFunc(text, func(match string) string {
		value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(match), "x"), "x"))
		value = strings.ReplaceAll(value, ",", ".")
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return match
		}
		return formatRevenuePerAdRuble(f)
	})
	text = replaceOutsideQuotes(text, func(fragment string) string {
		for _, rule := range topActionTokenRules {
			fragment = rule.pattern.ReplaceAllString(fragment, rule.value)
		}
		return fragment
	})
	text = strings.Join(strings.Fields(text), " ")
	text = strings.ReplaceAll(text, " .", ".")
	text = strings.ReplaceAll(text, " ,", ",")
	text = strings.ReplaceAll(text, " :", ":")
	text = replaceArticulFormsWithSKU(text)
	text = replaceInternalMetricNamesInText(text)
	return strings.TrimSpace(text)
}

func replaceOutsideQuotes(text string, transform func(string) string) string {
	var out strings.Builder
	var fragment strings.Builder
	inAngleQuotes := false
	inDoubleQuotes := false

	flush := func() {
		if fragment.Len() == 0 {
			return
		}
		out.WriteString(transform(fragment.String()))
		fragment.Reset()
	}

	for _, r := range text {
		switch r {
		case '«':
			if !inDoubleQuotes {
				flush()
				inAngleQuotes = true
				out.WriteRune(r)
				continue
			}
		case '»':
			if inAngleQuotes {
				inAngleQuotes = false
				out.WriteRune(r)
				continue
			}
		case '"':
			if !inAngleQuotes {
				flush()
				inDoubleQuotes = !inDoubleQuotes
				out.WriteRune(r)
				continue
			}
		}

		if inAngleQuotes || inDoubleQuotes {
			out.WriteRune(r)
			continue
		}
		fragment.WriteRune(r)
	}

	flush()
	return out.String()
}

func compactSupportingMetrics(metrics []TopActionSupportingMetric) []TopActionSupportingMetric {
	if len(metrics) == 0 {
		return nil
	}
	out := make([]TopActionSupportingMetric, 0, len(metrics))
	for _, metric := range metrics {
		if strings.TrimSpace(metric.Label) == "" || strings.TrimSpace(metric.Value) == "" {
			continue
		}
		out = append(out, metric)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isValidTopActionItem(item TopActionItem) bool {
	return strings.TrimSpace(item.ID) != "" &&
		strings.TrimSpace(item.Source) != "" &&
		strings.TrimSpace(item.Priority) != "" &&
		strings.TrimSpace(item.PriorityLabel) != "" &&
		strings.TrimSpace(item.Category) != "" &&
		strings.TrimSpace(item.CategoryLabel) != "" &&
		strings.TrimSpace(item.Title) != "" &&
		strings.TrimSpace(item.Reason) != "" &&
		strings.TrimSpace(item.NextStep) != ""
}
