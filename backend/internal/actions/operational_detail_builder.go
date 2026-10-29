package actions

import (
	"fmt"
	"strconv"
	"strings"
)

// BuildOperationalDetailFromCandidate builds a deterministic operational_detail from candidate facts.
func BuildOperationalDetailFromCandidate(cand ActionCandidate) OperationalDetail {
	detail := OperationalDetail{
		Status:        "available",
		FormatVersion: OperationalDetailFormatVersion,
		ActionPlan: OperationalActionPlan{
			Headline: "Что сделать",
		},
		HiddenLimitations: append([]string(nil), cand.Limitations...),
	}

	switch cand.Type {
	case ActionTypeAdHighDRR:
		buildAdHighDRROperationalDetail(cand, &detail)
	case ActionTypeAdSpendNoOrders:
		buildAdSpendNoOrdersOperationalDetail(cand, &detail)
	case ActionTypeStockReplenishment:
		buildStockReplenishmentOperationalDetail(cand, &detail)
	case ActionTypeNoSalesWithStock:
		buildNoSalesWithStockOperationalDetail(cand, &detail)
	case ActionTypeSKUOrdersDrop:
		buildSKUOrdersDropOperationalDetail(cand, &detail)
	case ActionTypeKeySKURisk:
		buildKeySKURiskOperationalDetail(cand, &detail)
	case ActionTypeFunnelWeakTransition:
		buildFunnelWeakTransitionOperationalDetail(cand, &detail)
	default:
		buildGenericOperationalDetail(cand, &detail)
	}

	detail.SecondaryActions = buildSecondaryActions(cand)
	if len(cand.TopExamples) > 0 && len(detail.ObjectActions) == 0 {
		detail.ObjectActions = buildObjectActionsFromExamples(cand)
	}
	return detail
}

func buildAdHighDRROperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	agg := cand.Aggregates
	campaignCount := intFromAgg(agg, "campaign_count", "count")
	spendTotal := floatFromAgg(agg, "spend_7d_total")
	revenueTotal := floatFromAgg(agg, "revenue_7d_total")
	avgDRR := floatFromAgg(agg, "avg_drr", "drr")
	threshold := floatFromAgg(agg, "drr_threshold_pct", "threshold_pct")
	if threshold <= 0 {
		threshold = 100
	}

	spendFmt := formatRubles(spendTotal)
	revenueFmt := formatRubles(revenueTotal)
	drrFmt := formatPercent(avgDRR)

	detail.Why = OperationalWhy{
		Headline: fmt.Sprintf("%d %s тратят бюджет с ДРР %s", campaignCount, campaignsNominative(campaignCount), drrFmt),
		Text: fmt.Sprintf(
			"За 7 дней %d %s потратили %s и принесли %s рекламной выручки. Средний ДРР — %s, что выше порога %.0f%%. Это значит, что рекламный расход по группе существенно превышает рекламную выручку.",
			campaignCount, campaignsNominative(campaignCount), spendFmt, revenueFmt, drrFmt, threshold,
		),
		Facts: factsToOperational(cand.Facts),
	}

	factIDs := collectFactIDsBySuffix(cand, "spend_7d_total", "drr", "campaign_count")
	targets := targetEntitiesFromExamples(cand.TopExamples, "campaign")

	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:          "step_1",
			ActionKind:      "reduce_budget_or_bids",
			Title:           "Ограничить расход по кампаниям с ДРР выше порога",
			Instruction:     fmt.Sprintf("Снизьте бюджет или ставки у %s с ДРР выше %.0f%%, начиная с кампаний с максимальным расходом.", campaignsGenitive(campaignCount), threshold),
			WhyThisStep:     fmt.Sprintf("Группа уже потратила %s за 7 дней при ДРР %s.", spendFmt, drrFmt),
			ExpectedResult:  "Остановить дальнейший перерасход по кампаниям, которые сейчас не окупают рекламу.",
			DecisionMode:    DecisionModeDirect,
			Confidence:      ConfidenceHigh,
			EvidenceFactIDs: factIDs,
			TargetEntities:  targets,
		},
		{
			StepID:         "step_2",
			ActionKind:     "review_search_phrases",
			Title:          "Проверить поисковые фразы",
			Instruction:    "Если в кампаниях есть нерелевантные поисковые фразы — отключите их.",
			WhyThisStep:    "Нерелевантный трафик часто повышает ДРР без роста заказов.",
			ExpectedResult: "Снизить расход на нецелевые показы и клики.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_3",
			ActionKind:     "verify_drr_after_change",
			Title:          "Проверить динамику ДРР",
			Instruction:    fmt.Sprintf("Если после корректировок ДРР остаётся выше порога %.0f%% — дополнительно ограничьте расход.", threshold),
			WhyThisStep:    "Изменения ставок и бюджета требуют времени для отражения в метриках.",
			ExpectedResult: "Подтвердить, что ДРР снизился относительно порога.",
			DecisionMode:   DecisionModeDiagnostic,
			Confidence:     ConfidenceMedium,
		},
	}

	detail.ObjectActions = buildCampaignObjectActions(cand, "Поставить на паузу или снизить бюджет",
		"Кампания входит в группу с ДРР выше порога и высоким расходом.")
}

func buildAdSpendNoOrdersOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	agg := cand.Aggregates
	campaignCount := intFromAgg(agg, "campaign_count", "count")
	spendTotal := floatFromAgg(agg, "spend_7d_total")
	spendFmt := formatRubles(spendTotal)

	var headline, whyText string
	if campaignCount == 1 && len(cand.TopExamples) == 1 {
		name := cand.TopExamples[0].Name
		if name == "" {
			name = "кампания"
		}
		headline = fmt.Sprintf("Кампания «%s» потратила %s и не принесла заказов", name, spendFmt)
		whyText = fmt.Sprintf("За 7 дней кампания «%s» потратила %s и принесла 0 заказов.", name, spendFmt)
	} else {
		headline = fmt.Sprintf("%d %s с расходом %s и 0 заказов", campaignCount, campaignsNominative(campaignCount), spendFmt)
		whyText = fmt.Sprintf("За 7 дней %d %s потратили %s и не принесли заказов.", campaignCount, campaignsNominative(campaignCount), spendFmt)
	}

	detail.Why = OperationalWhy{
		Headline: headline,
		Text:     whyText,
		Facts:    factsToOperational(cand.Facts),
	}

	targets := targetEntitiesFromExamples(cand.TopExamples, "campaign")
	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:          "step_1",
			ActionKind:      "pause_or_reduce_budget",
			Title:           "Поставить на паузу или снизить бюджет",
			Instruction:     fmt.Sprintf("Поставьте %s на паузу или снизьте дневной бюджет до минимального, потому что за 7 дней был расход %s и 0 заказов.", campaignsAccusative(campaignCount), spendFmt),
			WhyThisStep:     fmt.Sprintf("Расход %s без заказов указывает на неэффективный трафик.", spendFmt),
			ExpectedResult:  "Остановить дальнейший расход без заказов до проверки настроек.",
			DecisionMode:    DecisionModeDirect,
			Confidence:      ConfidenceHigh,
			EvidenceFactIDs: collectFactIDsBySuffix(cand, "spend_7d_total", "zero_orders"),
			TargetEntities:  targets,
		},
		{
			StepID:         "step_2",
			ActionKind:     "inspect_before_restart",
			Title:          "Проверить фразы и товары перед перезапуском",
			Instruction:    "Перед повторным запуском проверьте поисковые фразы и товары внутри кампании, исключите нерелевантный трафик.",
			WhyThisStep:    "Без корректировки настроек расход без заказов может повториться.",
			ExpectedResult: "Запустить кампанию только после устранения источника нецелевого трафика.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
	}

	detail.ObjectActions = buildCampaignObjectActions(cand, "Поставить на паузу или снизить бюджет до минимума",
		"Кампания с расходом без заказов за 7 дней.")
}

func buildStockReplenishmentOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	agg := cand.Aggregates
	skuCount := intFromAgg(agg, "sku_count", "count")
	coverageMin := floatFromAgg(agg, "coverage_min", "coverage_min_days")
	coverageMax := floatFromAgg(agg, "coverage_max_days", "coverage_max")
	targetDays := floatFromAgg(agg, "target_coverage_days")
	if targetDays <= 0 {
		targetDays = 14
	}
	revenueShare := floatFromAgg(agg, "revenue_share_pct")

	var coverageRange string
	switch {
	case coverageMin > 0 && coverageMax > 0:
		coverageRange = fmt.Sprintf("%.1f–%.0f", coverageMin, coverageMax)
	case coverageMax > 0:
		coverageRange = fmt.Sprintf("до %.0f", coverageMax)
	default:
		coverageRange = "низким"
	}

	whyText := fmt.Sprintf("%d SKU имеют покрытие %s дней.", skuCount, coverageRange)
	if revenueShare > 0 {
		whyText = fmt.Sprintf("%d SKU имеют покрытие %s дней и дают %.1f%% выручки за 7 дней.", skuCount, coverageRange, revenueShare)
	}

	detail.Why = OperationalWhy{
		Headline: fmt.Sprintf("%d SKU с покрытием %s дней", skuCount, coverageRange),
		Text:     whyText,
		Facts:    factsToOperational(cand.Facts),
	}

	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:         "step_1",
			ActionKind:     "prioritize_replenishment",
			Title:          "Начать с SKU с минимальным покрытием",
			Instruction:    "Сформируйте поставку, начиная с SKU с минимальным покрытием и наибольшей долей выручки.",
			WhyThisStep:    "Эти SKU быстрее всего теряют продажи из-за нехватки остатка.",
			ExpectedResult: "Снизить риск out-of-stock по наиболее важным позициям.",
			DecisionMode:   DecisionModeDirect,
			Confidence:     ConfidenceHigh,
		},
		{
			StepID:         "step_2",
			ActionKind:     "verify_incoming_supplies",
			Title:          "Сверить поставки в пути",
			Instruction:    "Проверьте фактические поставки в Ozon перед расчётом объёма пополнения.",
			WhyThisStep:    "Aivion не видит будущие поставки — без сверки можно заказать лишний объём.",
			ExpectedResult: "Уточнить, какие SKU уже пополняются поставкой в пути.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_3",
			ActionKind:     "replenish_to_target_coverage",
			Title:          "Довести покрытие до целевого уровня",
			Instruction:    "Если поставок в пути нет, сформируйте поставку до целевого покрытия. Точное количество рассчитывайте по sales velocity в Ozon.",
			WhyThisStep:    "Целевое покрытие снижает риск обнуления остатков между поставками.",
			ExpectedResult: "Обеспечить достаточный запас по приоритетным SKU.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
	}

	detail.ObjectActions = buildSKUObjectActions(cand, "Сформировать поставку",
		"SKU с низким покрытием остатков.")
}

func buildNoSalesWithStockOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	agg := cand.Aggregates
	skuCount := intFromAgg(agg, "sku_count", "count")
	stockUnits := floatFromAgg(agg, "stock_units_total")
	frozenValue := floatFromAgg(agg, "frozen_stock_value_total", "frozen_stock_value")

	whyText := fmt.Sprintf("%d SKU имеют %.0f шт. остатка", skuCount, stockUnits)
	if frozenValue > 0 {
		whyText = fmt.Sprintf("%d SKU имеют %.0f шт. остатка и %s замороженного остатка при подтверждённом нуле заказов.", skuCount, stockUnits, formatRubles(frozenValue))
	}

	detail.Why = OperationalWhy{
		Headline: fmt.Sprintf("%d SKU без заказов при остатке %.0f шт.", skuCount, stockUnits),
		Text:     whyText,
		Facts:    factsToOperational(cand.Facts),
	}

	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:         "step_1",
			ActionKind:     "prioritize_by_frozen_stock",
			Title:          "Начать с SKU с максимальным замороженным остатком",
			Instruction:    "Разберите SKU, отсортировав по замороженному остатку и объёму на складе.",
			WhyThisStep:    "Эти позиции сильнее всего связывают складской запас без продаж.",
			ExpectedResult: "Быстрее высвободить оборотный капитал и место на складе.",
			DecisionMode:   DecisionModeDirect,
			Confidence:     ConfidenceHigh,
		},
		{
			StepID:         "step_2",
			ActionKind:     "check_active_card",
			Title:          "Проверить цену, контент и рейтинг",
			Instruction:    "Если карточка активна и товар доступен — проверьте цену, контент и рейтинг.",
			WhyThisStep:    "При доступном товаре отсутствие заказов часто связано с видимостью или конверсией карточки.",
			ExpectedResult: "Найти и устранить барьеры покупки на активных SKU.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_3",
			ActionKind:     "test_ads_for_historical_sellers",
			Title:          "Вынести в тест рекламы",
			Instruction:    "Если товар раньше продавался и не рекламируется — добавьте в тестовую рекламу с ограниченным бюджетом.",
			WhyThisStep:    "Исторически продававшиеся SKU могут вернуть спрос при дополнительной видимости.",
			ExpectedResult: "Проверить, восстанавливается ли спрос при ограниченном рекламном бюджете.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_4",
			ActionKind:     "clearance_or_delist",
			Title:          "Распродажа или вывод из ассортимента",
			Instruction:    "Если товар давно не продаётся и занимает склад — вынесите в список на распродажу или вывод из ассортимента.",
			WhyThisStep:    "Длительный нулевой спрос при остатке увеличивает замороженный запас.",
			ExpectedResult: "Сократить долю неликвидного остатка.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
	}

	detail.ObjectActions = buildSKUObjectActions(cand, "Разобрать карточку и видимость",
		"SKU с остатком без заказов.")
}

func buildSKUOrdersDropOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	agg := cand.Aggregates
	skuCount := intFromAgg(agg, "sku_count", "count")
	ordersDelta := floatFromAgg(agg, "orders_delta_total", "orders_delta")
	revenueDelta := floatFromAgg(agg, "revenue_delta_total", "revenue_delta")

	deltaFmt := formatSignedInt(ordersDelta)
	whyText := fmt.Sprintf("%d SKU дали %s заказов день-к-дню.", skuCount, deltaFmt)
	if revenueDelta != 0 {
		whyText = fmt.Sprintf("%d SKU дали %s заказов и %s выручки день-к-дню.", skuCount, deltaFmt, formatSignedRubles(revenueDelta))
	}

	detail.Why = OperationalWhy{
		Headline: fmt.Sprintf("%d SKU с падением заказов на %s шт.", skuCount, formatAbsInt(ordersDelta)),
		Text:     whyText,
		Facts:    factsToOperational(cand.Facts),
	}

	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:         "step_1",
			ActionKind:     "prioritize_by_drop_contribution",
			Title:          "Разобрать SKU по вкладу в падение",
			Instruction:    "Начните с SKU с максимальным вкладом в падение заказов.",
			WhyThisStep:    "Приоритет по вкладу быстрее возвращает потерянные заказы.",
			ExpectedResult: "Сфокусировать усилия на SKU с наибольшим эффектом.",
			DecisionMode:   DecisionModeDirect,
			Confidence:     ConfidenceHigh,
		},
		{
			StepID:         "step_2",
			ActionKind:     "diagnose_visibility",
			Title:          "Если упали показы или переходы",
			Instruction:    "Если одновременно снизились показы или переходы — проверьте видимость и рекламу.",
			WhyThisStep:    "Падение трафика указывает на проблемы видимости, а не конверсии карточки.",
			ExpectedResult: "Восстановить поток показов и переходов.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_3",
			ActionKind:     "diagnose_conversion",
			Title:          "Если упала конверсия",
			Instruction:    "Если показы стабильны, но упала конверсия — проверьте цену, рейтинг, отзывы и карточку.",
			WhyThisStep:    "Стабильный трафик при падении заказов указывает на проблему конверсии.",
			ExpectedResult: "Найти фактор, снижающий конверсию в заказ.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_4",
			ActionKind:     "replenish_if_low_stock",
			Title:          "Если низкое покрытие",
			Instruction:    "Если у SKU низкое покрытие остатков — приоритетно пополните остатки.",
			WhyThisStep:    "Нехватка остатка может выглядеть как падение заказов.",
			ExpectedResult: "Исключить out-of-stock как причину падения.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
	}

	detail.ObjectActions = buildSKUObjectActions(cand, "Разобрать причину падения заказов",
		"SKU с отрицательной динамикой заказов.")
}

func buildKeySKURiskOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	agg := cand.Aggregates
	skuCount := intFromAgg(agg, "sku_count", "count")
	revenueShare := floatFromAgg(agg, "revenue_share_pct")
	riskTypes := stringFromAgg(agg, "risk_types")

	whyText := fmt.Sprintf("%d ключевых SKU имеют признаки риска.", skuCount)
	if revenueShare > 0 {
		whyText = fmt.Sprintf("%d ключевых SKU дают %.1f%% выручки и имеют признаки риска.", skuCount, revenueShare)
	}
	if riskTypes != "" {
		whyText += " Риски: " + riskTypes + "."
	}

	detail.Why = OperationalWhy{
		Headline: fmt.Sprintf("%d ключевых SKU с рисками", skuCount),
		Text:     whyText,
		Facts:    factsToOperational(cand.Facts),
	}

	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:         "step_1",
			ActionKind:     "prioritize_double_risk",
			Title:          "Сначала SKU с двойным риском",
			Instruction:    "Начните с SKU, где одновременно падение заказов и низкое покрытие остатков.",
			WhyThisStep:    "Двойной риск быстрее всего бьёт по выручке.",
			ExpectedResult: "Защитить ключевую долю выручки в первую очередь.",
			DecisionMode:   DecisionModeDirect,
			Confidence:     ConfidenceHigh,
		},
		{
			StepID:         "step_2",
			ActionKind:     "accelerate_replenishment",
			Title:          "Ускорить пополнение при отсутствии поставки",
			Instruction:    "Если поставки в пути нет — ускорьте пополнение остатков.",
			WhyThisStep:    "Низкое покрытие у ключевых SKU повышает риск потери продаж.",
			ExpectedResult: "Вернуть достаточный запас по приоритетным SKU.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
		{
			StepID:         "step_3",
			ActionKind:     "diagnose_orders_drop",
			Title:          "Разобрать падение заказов при достаточном остатке",
			Instruction:    "Если остаток достаточный — перейдите к причинам падения заказов: видимость, конверсия, реклама.",
			WhyThisStep:    "При достаточном остатке падение заказов связано с другими факторами.",
			ExpectedResult: "Найти и устранить причину падения спроса.",
			DecisionMode:   DecisionModeConditional,
			Confidence:     ConfidenceMedium,
		},
	}

	detail.ObjectActions = buildSKUObjectActions(cand, "Приоритетно разобрать риски",
		"Ключевой SKU с высокой долей выручки.")
}

func buildFunnelWeakTransitionOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	from := stringFromAgg(cand.Aggregates, "from_stage")
	to := stringFromAgg(cand.Aggregates, "to_stage")
	deltaPct := floatFromAgg(cand.Aggregates, "conversion_delta_pct")

	headline := "Просадка перехода воронки"
	if from != "" && to != "" {
		headline = fmt.Sprintf("Просадка перехода %s → %s", from, to)
	}
	whyText := cand.Summary
	if deltaPct != 0 {
		whyText = fmt.Sprintf("Конверсия перехода %s → %s изменилась на %.1f%%.", from, to, deltaPct)
	}

	detail.Why = OperationalWhy{
		Headline: headline,
		Text:     whyText,
		Facts:    factsToOperational(cand.Facts),
	}

	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:         "step_1",
			ActionKind:     "analyze_funnel_stage",
			Title:          "Разобрать этап воронки",
			Instruction:    fmt.Sprintf("Проверьте, что изменилось на этапе %s → %s: трафик, карточки, цены, остатки.", from, to),
			WhyThisStep:    "Просадка перехода требует проверки факторов на конкретном этапе.",
			ExpectedResult: "Определить основной фактор просадки конверсии.",
			DecisionMode:   DecisionModeDiagnostic,
			Confidence:     ConfidenceMedium,
		},
	}

	if len(cand.TopExamples) > 0 {
		detail.ObjectActions = buildSKUObjectActions(cand, "Проверить вклад SKU в просадку",
			"SKU-участник просадки перехода.")
	}
}

func buildGenericOperationalDetail(cand ActionCandidate, detail *OperationalDetail) {
	detail.Why = OperationalWhy{
		Headline: cand.Summary,
		Text:     cand.Summary,
		Facts:    factsToOperational(cand.Facts),
	}
	detail.ActionPlan.Steps = []OperationalStep{
		{
			StepID:         "step_1",
			ActionKind:     "review_signal",
			Title:          "Разобрать сигнал",
			Instruction:    cand.Summary,
			WhyThisStep:    "Сигнал требует операционного разбора по доступным данным.",
			ExpectedResult: "Определить конкретное действие по результатам проверки.",
			DecisionMode:   DecisionModeDiagnostic,
			Confidence:     ConfidenceMedium,
		},
	}
}

func buildSecondaryActions(cand ActionCandidate) []OperationalSecondaryAction {
	out := []OperationalSecondaryAction{
		{Type: "ask_ai", Label: "Спросить AI"},
		{Type: "mark_checked", Label: "Отметить выполненным"},
	}

	entityCount := 0
	entityLabel := ""
	if cand.EntityGroupRef != nil {
		entityCount = cand.EntityGroupRef.Count
		switch cand.EntityGroupRef.EntityType {
		case "sku":
			entityLabel = fmt.Sprintf("Показать все %d SKU", entityCount)
		case "campaign":
			entityLabel = fmt.Sprintf("Показать все %d %s", entityCount, campaignsAccusative(entityCount))
		}
	}
	if entityLabel == "" {
		switch cand.Type {
		case ActionTypeAdHighDRR, ActionTypeAdSpendNoOrders:
			entityCount = intFromAgg(cand.Aggregates, "campaign_count")
			if entityCount > 0 {
				entityLabel = fmt.Sprintf("Показать все %d %s", entityCount, campaignsAccusative(entityCount))
			}
		default:
			entityCount = intFromAgg(cand.Aggregates, "sku_count")
			if entityCount > 0 {
				entityLabel = fmt.Sprintf("Показать все %d SKU", entityCount)
			}
		}
	}
	if entityLabel != "" {
		out = append([]OperationalSecondaryAction{{Type: "show_objects", Label: entityLabel}}, out...)
	}

	for _, cta := range cand.AllowedCTAs {
		if strings.HasPrefix(cta, "export_") {
			out = append(out, OperationalSecondaryAction{Type: "export", Label: "Скачать XLSX"})
			break
		}
	}
	for _, cta := range cand.AllowedCTAs {
		switch cta {
		case ActionCTAOpenSKUList:
			out = append(out, OperationalSecondaryAction{Type: "open_section", Label: "Открыть в разделе SKU"})
		case ActionCTAOpenCampaignList:
			out = append(out, OperationalSecondaryAction{Type: "open_section", Label: "Открыть в разделе рекламы"})
		}
	}
	return out
}

func buildObjectActionsFromExamples(cand ActionCandidate) []OperationalObjectAction {
	recommended := defaultRecommendedAction(cand.Type)
	reason := defaultObjectReason(cand.Type)
	switch cand.Type {
	case ActionTypeAdHighDRR, ActionTypeAdSpendNoOrders:
		return buildCampaignObjectActions(cand, recommended, reason)
	default:
		return buildSKUObjectActions(cand, recommended, reason)
	}
}

func buildCampaignObjectActions(cand ActionCandidate, recommended, reason string) []OperationalObjectAction {
	out := make([]OperationalObjectAction, 0, len(cand.TopExamples))
	for _, ex := range cand.TopExamples {
		if ex.EntityType != "campaign" {
			continue
		}
		out = append(out, OperationalObjectAction{
			EntityType:        "campaign",
			EntityID:          ex.EntityID,
			Name:              ex.Name,
			Metrics:           metricsFromExample(ex),
			RecommendedAction: recommended,
			Reason:            reason,
			Confidence:        ConfidenceHigh,
		})
	}
	return out
}

func buildSKUObjectActions(cand ActionCandidate, recommended, reason string) []OperationalObjectAction {
	out := make([]OperationalObjectAction, 0, len(cand.TopExamples))
	for _, ex := range cand.TopExamples {
		if ex.EntityType != "sku" {
			continue
		}
		out = append(out, OperationalObjectAction{
			EntityType:        "sku",
			EntityID:          ex.EntityID,
			Name:              ex.Name,
			Metrics:           metricsFromExample(ex),
			RecommendedAction: recommended,
			Reason:            reason,
			Confidence:        ConfidenceHigh,
		})
	}
	return out
}

func metricsFromExample(ex ActionCandidateExample) []OperationalMetricRef {
	if len(ex.Metrics) == 0 {
		return nil
	}
	labels := map[string]string{
		"spend_7d":      "Расход 7д",
		"revenue_7d":    "Выручка 7д",
		"orders_7d":     "Заказы 7д",
		"drr_pct":       "ДРР",
		"coverage_days": "Покрытие",
		"stock_units":   "Остаток",
		"orders_delta":  "Δ заказов",
		"revenue_share": "Доля выручки",
	}
	out := make([]OperationalMetricRef, 0, len(ex.Metrics))
	for key, val := range ex.Metrics {
		label := labels[key]
		if label == "" {
			label = key
		}
		out = append(out, OperationalMetricRef{Label: label, Value: formatMetricValue(key, val)})
	}
	return out
}

func formatMetricValue(key string, val any) string {
	switch key {
	case "spend_7d", "revenue_7d":
		if f, ok := val.(float64); ok {
			return formatRubles(f)
		}
	case "drr_pct":
		if f, ok := val.(float64); ok {
			return formatPercent(f)
		}
	case "coverage_days":
		if f, ok := val.(float64); ok {
			return fmt.Sprintf("%.1f дн.", f)
		}
	}
	return fmt.Sprint(val)
}

func factsToOperational(facts []ActionCandidateFact) []OperationalFact {
	out := make([]OperationalFact, 0, len(facts))
	for _, f := range facts {
		of := OperationalFact{
			FactID: f.FactID,
			Label:  f.Label,
			Value:  f.Value,
			Unit:   f.Unit,
		}
		of.FormattedValue = formatFactValue(f)
		out = append(out, of)
	}
	return out
}

func formatFactValue(f ActionCandidateFact) string {
	val := strings.TrimSpace(f.Value)
	if val == "" {
		return ""
	}
	switch f.Unit {
	case "₽":
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			return formatRubles(n)
		}
	case "%":
		if n, err := strconv.ParseFloat(val, 64); err == nil {
			return formatPercent(n)
		}
	case "шт":
		return val + " шт"
	case "дней":
		return val + " дн."
	}
	if f.Unit != "" {
		return val + " " + f.Unit
	}
	return val
}

func formatRubles(v float64) string {
	if v == 0 {
		return "0 ₽"
	}
	return formatThousands(v) + " ₽"
}

func formatPercent(v float64) string {
	return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) + "%"
}

func formatSignedInt(v float64) string {
	n := int(v)
	if n > 0 {
		return "+" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func formatAbsInt(v float64) string {
	n := int(v)
	if n < 0 {
		n = -n
	}
	return strconv.Itoa(n)
}

func formatSignedRubles(v float64) string {
	if v > 0 {
		return "+" + formatRubles(v)
	}
	return formatRubles(v)
}

func collectFactIDsBySuffix(cand ActionCandidate, suffixes ...string) []string {
	out := make([]string, 0, len(suffixes))
	for _, s := range suffixes {
		for _, f := range cand.Facts {
			if strings.HasSuffix(f.FactID, "."+s) || strings.HasSuffix(f.FactID, s) {
				out = append(out, f.FactID)
				break
			}
		}
	}
	return out
}

func targetEntitiesFromExamples(examples []ActionCandidateExample, entityType string) []OperationalTargetEntity {
	out := make([]OperationalTargetEntity, 0, len(examples))
	for _, ex := range examples {
		if ex.EntityType != entityType {
			continue
		}
		out = append(out, OperationalTargetEntity{
			EntityType: entityType,
			ID:         ex.EntityID,
			Name:       ex.Name,
		})
	}
	return out
}

func defaultRecommendedAction(typ ActionType) string {
	switch typ {
	case ActionTypeAdHighDRR:
		return "Снизить бюджет или ставки"
	case ActionTypeAdSpendNoOrders:
		return "Поставить на паузу или снизить бюджет"
	case ActionTypeStockReplenishment:
		return "Сформировать поставку"
	case ActionTypeNoSalesWithStock:
		return "Разобрать карточку"
	case ActionTypeSKUOrdersDrop:
		return "Разобрать падение заказов"
	case ActionTypeKeySKURisk:
		return "Приоритетно разобрать"
	default:
		return "Разобрать"
	}
}

func defaultObjectReason(typ ActionType) string {
	switch typ {
	case ActionTypeAdHighDRR:
		return "Высокий ДРР и расход в группе."
	case ActionTypeAdSpendNoOrders:
		return "Расход без заказов за 7 дней."
	default:
		return "Входит в группу сигнала."
	}
}

// SyncLegacyFieldsFromOperationalDetail copies v2 content into legacy fields for API compatibility.
func SyncLegacyFieldsFromOperationalDetail(action *DomainAIAction, detail OperationalDetail) {
	if strings.TrimSpace(action.Title) == "" || isGenericTitle(action.Title) || strings.HasPrefix(strings.ToLower(action.Title), "провер") {
		action.Title = operationalTitleFromDetail(action.Type, detail)
	}
	if strings.TrimSpace(detail.Why.Text) != "" {
		action.Reason = detail.Why.Text
	}
	if len(detail.ActionPlan.Steps) > 0 {
		action.NextStep = joinStepInstructions(detail.ActionPlan.Steps)
		action.ExpectedEffect = detail.ActionPlan.Steps[0].ExpectedResult
	}
}

func operationalTitleFromDetail(typ ActionType, detail OperationalDetail) string {
	if strings.TrimSpace(detail.Why.Headline) != "" {
		headline := detail.Why.Headline
		switch typ {
		case ActionTypeAdHighDRR:
			if !strings.Contains(strings.ToLower(headline), "огранич") {
				return "Ограничить расходы: " + headline
			}
		case ActionTypeAdSpendNoOrders:
			if !strings.Contains(strings.ToLower(headline), "пауз") {
				return "Поставить на паузу: " + headline
			}
		case ActionTypeStockReplenishment:
			if !strings.Contains(strings.ToLower(headline), "пополн") {
				return "Пополнить " + headline
			}
		case ActionTypeSKUOrdersDrop, ActionTypeNoSalesWithStock, ActionTypeKeySKURisk:
			if !strings.Contains(strings.ToLower(headline), "разобр") {
				return "Разобрать " + headline
			}
		}
		return headline
	}
	return detail.Why.Text
}

func joinStepInstructions(steps []OperationalStep) string {
	parts := make([]string, 0, len(steps))
	for _, s := range steps {
		if instr := strings.TrimSpace(s.Instruction); instr != "" {
			parts = append(parts, instr)
		}
	}
	return strings.Join(parts, " ")
}

// FinalizeDomainAction ensures operational_detail is present and syncs legacy fields.
func FinalizeDomainAction(action DomainAIAction, candidates []ActionCandidate) DomainAIAction {
	var primary ActionCandidate
	if len(candidates) > 0 {
		primary = candidates[0]
	}

	if action.OperationalDetail == nil || !IsOperationalDetailStructurallyValid(*action.OperationalDetail) {
		built := BuildOperationalDetailFromCandidate(primary)
		action.OperationalDetail = &built
	} else {
		// Enrich secondary actions and hidden limitations from candidate if missing.
		if len(action.OperationalDetail.SecondaryActions) == 0 {
			action.OperationalDetail.SecondaryActions = buildSecondaryActions(primary)
		}
		if len(action.OperationalDetail.HiddenLimitations) == 0 {
			action.OperationalDetail.HiddenLimitations = append([]string(nil), primary.Limitations...)
		}
	}

	SyncLegacyFieldsFromOperationalDetail(&action, *action.OperationalDetail)
	return action
}

func IsOperationalDetailStructurallyValid(d OperationalDetail) bool {
	if strings.TrimSpace(d.Why.Text) == "" {
		return false
	}
	if len(d.ActionPlan.Steps) == 0 {
		return false
	}
	for _, step := range d.ActionPlan.Steps {
		if strings.TrimSpace(step.Title) == "" ||
			strings.TrimSpace(step.Instruction) == "" ||
			strings.TrimSpace(step.WhyThisStep) == "" ||
			strings.TrimSpace(step.ExpectedResult) == "" {
			return false
		}
	}
	return true
}

func finalizeAcceptedActions(domainCtx ActionDomainContext, actions []DomainAIAction) []DomainAIAction {
	if len(actions) == 0 {
		return nil
	}
	candidateByID := make(map[string]ActionCandidate, len(domainCtx.Candidates))
	for _, c := range domainCtx.Candidates {
		candidateByID[c.CandidateID] = c
	}
	out := make([]DomainAIAction, 0, len(actions))
	for _, action := range actions {
		candidates := make([]ActionCandidate, 0, len(action.SourceCandidateIDs))
		for _, id := range action.SourceCandidateIDs {
			if c, ok := candidateByID[id]; ok {
				candidates = append(candidates, c)
			}
		}
		finalized := FinalizeDomainAction(action, candidates)
		if finalized.OperationalDetail != nil {
			if issues := ValidateOperationalDetail(*finalized.OperationalDetail, finalized.Type, candidates); len(issues) > 0 {
				if len(candidates) > 0 {
					built := BuildOperationalDetailFromCandidate(candidates[0])
					finalized.OperationalDetail = &built
					SyncLegacyFieldsFromOperationalDetail(&finalized, built)
				}
			}
		}
		out = append(out, finalized)
	}
	return out
}
