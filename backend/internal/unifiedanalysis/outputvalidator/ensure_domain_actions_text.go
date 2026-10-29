package outputvalidator

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// buildDomainAction renders one well-formed UnifiedActionV1 for a material
// domain bucket. Every number/entity id quoted in the text comes from the
// Pack reference index — no narrative claim is invented.
func buildDomainAction(d materialDomain, cands []domainCandidate, idx *PackReferenceIndex, id string) contract.UnifiedActionV1 {
	scopeType := cands[0].scopeType
	// Persist bare canonical entity IDs (not "sku:…") so publication hydration
	// can resolve Dashboard Facts / Pack metrics. Prefixed forms remain only
	// in exclusion audit trails via prefixedEntityIDs.
	entityIDs := bareEntityIDs(cands)
	n := len(cands)

	title, summary, problem, whyNow, goal, effect, completion, evidenceSummary, factSuffix :=
		domainActionText(d, cands, idx, n)

	steps := domainActionSteps(d, entityIDs)
	evidenceFactIDs := make([]string, 0, n)
	for _, c := range cands {
		evidenceFactIDs = append(evidenceFactIDs, c.scopeType+":"+c.entityID+":"+factSuffix)
	}

	return contract.UnifiedActionV1{
		AnalysisActionID: id,
		Type:             d.actionType,
		Category:         d.category,
		Priority:         d.priority,
		ExecutionTier:    d.executionTier,
		Title:            title,
		Summary:          summary,
		Problem:          problem,
		WhyNow:           whyNow,
		OperationalGoal:  goal,
		ExpectedEffect:   effect,
		Scope: contract.UnifiedActionScopeV1{
			ScopeType: scopeType,
			EntityIDs: entityIDs,
		},
		Steps:              steps,
		CompletionCriteria: completion,
		EvidenceSummary:    evidenceSummary,
		EvidenceFactIDs:    evidenceFactIDs,
		Confidence:         "high",
	}
}

func bareEntityIDs(cands []domainCandidate) []string {
	ids := make([]string, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.entityID)
	}
	return ids
}

func domainActionText(d materialDomain, cands []domainCandidate, idx *PackReferenceIndex, n int) (title, summary, problem, whyNow, goal, effect, completion, evidenceSummary, factSuffix string) {
	switch d.key {
	case "stock_oos":
		title = fmt.Sprintf("Пополнить %d SKU без остатка с недавними продажами", n)
		covMin, covMax, orders, revenue := stockBandStats(cands, idx)
		summary = fmt.Sprintf("%d SKU полностью без остатка при наличии спроса в недавних периодах.", n)
		problem = fmt.Sprintf("%d SKU имеют нулевой остаток и подтверждённый спрос — продажи по ним уже потеряны.", n)
		whyNow = fmt.Sprintf("За последние 7 дней по этим SKU было %d заказов (≈ %.0f ₽). Каждый день без остатка — прямая потеря заказов.", orders, revenue)
		if orders == 0 {
			whyNow = "Каждый день без остатка — прямая потеря заказов по SKU с подтверждённым спросом в недавних периодах."
		}
		goal = "Восстановить остаток и довести покрытие до целевого уровня (срок поставки + буфер + страховой запас; по умолчанию политики ≈ 14 дней)."
		effect = fmt.Sprintf("После поступления рекомендуемого количества покрытие вырастет с 0 до целевого уровня. Диапазон текущего покрытия: %.1f–%.1f дн.", covMin, covMax)
		completion = "Прогресс: X из N SKU с остатком > 0 и покрытием ≥ индивидуального целевого уровня."
		evidenceSummary = "По указанным SKU в Pack подтверждены нулевой доступный остаток и недавний спрос."
		factSuffix = "available_stock_units"
	case "stock_critical":
		title = fmt.Sprintf("Пополнить %d SKU с покрытием до трёх дней", n)
		covMin, covMax, orders, revenue := stockBandStats(cands, idx)
		summary = fmt.Sprintf("%d SKU имеют покрытие спроса 0–3 дня и рискуют уйти в стокаут.", n)
		problem = fmt.Sprintf("%d SKU находятся в критическом дефиците: покрытие от %.1f до %.1f дня.", n, covMin, covMax)
		whyNow = fmt.Sprintf("За 7 дней эти SKU дали %d заказов (≈ %.0f ₽). При текущей скорости продаж остаток закончится в ближайшие дни — до восстановления позиций и выручки будет сложнее.", orders, revenue)
		if orders == 0 {
			whyNow = "При текущей скорости продаж остаток закончится в ближайшие дни."
		}
		goal = "Довести покрытие до целевого уровня (по умолчанию политики ≈ 14 дней) по каждому SKU из списка."
		effect = fmt.Sprintf("После поставки рекомендуемого количества покрытие вырастет с %.1f–%.1f до целевого уровня и снизится вероятность отсутствия товара.", covMin, covMax)
		completion = "Прогресс: X из N SKU достигли индивидуального целевого покрытия."
		evidenceSummary = "По указанным SKU в Pack покрытие спроса не превышает трёх дней."
		factSuffix = "stock_coverage_days_7d"
	case "stock_urgent":
		// Legacy key kept for older tests / callers.
		title = fmt.Sprintf("Пополнить %d SKU без остатка и в критическом дефиците", n)
		summary = fmt.Sprintf("%d SKU уже без остатка или с покрытием спроса 0–3 дня при наличии спроса за последние периоды.", n)
		problem = fmt.Sprintf("Из %d SKU: %d без остатка, %d в критическом дефиците (покрытие ≤3 дней).", n, d.context.oosCount, d.context.criticalCount)
		whyNow = "Каждый день без остатка или в критическом дефиците — прямая потеря заказов, которые уже подтверждены спросом."
		goal = "Восстановить остаток и довести покрытие спроса до целевого уровня по каждому SKU из списка."
		effect = "Снижение риска стокаута и сохранение выручки по SKU с подтверждённым спросом."
		completion = "Прогресс: X из N SKU с остатком > 0 и покрытием ≥ целевого уровня."
		evidenceSummary = "По указанным SKU в Pack подтверждены отсутствие остатка или покрытие ≤3 дней."
		factSuffix = "available_stock_units"
	case "stock_short":
		title = fmt.Sprintf("Запланировать пополнение %d SKU с покрытием 4–7 дней", n)
		covMin, covMax, orders, revenue := stockBandStats(cands, idx)
		summary = fmt.Sprintf("%d SKU имеют покрытие спроса 4–7 дней — риск стокаута в ближайшую неделю без пополнения.", n)
		problem = fmt.Sprintf("%d SKU имеют покрытие от %.1f до %.1f дня и перейдут в дефицит в течение недели при текущей скорости продаж.", n, covMin, covMax)
		whyNow = fmt.Sprintf("За 7 дней эти SKU обеспечили %d заказов (≈ %.0f ₽). Заказ заранее дешевле экстренного пополнения после стокаута.", orders, revenue)
		if orders == 0 {
			whyNow = "Заказ поставки заранее дешевле и быстрее, чем экстренное пополнение после стокаута."
		}
		goal = "Включить SKU в ближайшую плановую поставку и поднять покрытие до целевого уровня (по умолчанию политики ≈ 14 дней)."
		effect = fmt.Sprintf("После подтверждённой поставки покрытие вырастет с %.1f–%.1f дней и снизится риск перехода в критический дефицит.", covMin, covMax)
		completion = "Прогресс: X из N SKU достигли индивидуального целевого покрытия."
		evidenceSummary = "По указанным SKU в Pack покрытие спроса составляет 4–7 дней."
		factSuffix = "stock_coverage_days_7d"
	case "sales_decline":
		title = fmt.Sprintf("Разобрать падение заказов по %d SKU", n)
		summary = fmt.Sprintf("%d SKU потеряли часть заказов неделя к неделе; суммарная потеря выручки ≈ %.0f ₽.", n, d.context.lostRevenue)
		problem = fmt.Sprintf("Заказы по %d SKU упали неделя к неделе (максимальное падение ≈ %.0f%%), выручка снизилась на ≈ %.0f ₽.", n, d.context.maxDropRatio*100, d.context.lostRevenue)
		whyNow = "Падение уже наблюдается в последних 7 днях против предыдущей недели — чем дольше без реакции, тем выше накопленная потеря."
		goal = "Определить причину падения (остаток, цена, карточка, реклама) и восстановить заказы минимум до уровня предыдущей недели."
		effect = "Восстановление заказов и выручки по SKU до уровня предыдущей недели."
		completion = "Заказы за 7 дней по каждому SKU из списка не ниже уровня предыдущей недели."
		evidenceSummary = "orders_7d и orders_previous_7d по указанным SKU в Pack показывают падение заказов неделя к неделе."
		factSuffix = "orders_7d"
	case "ads_high_drr":
		title = fmt.Sprintf("Пересмотреть рекламные кампании с высоким ДРР (%d шт.)", n)
		summary = fmt.Sprintf("%d рекламных кампаний работают с ДРР выше 80%% (максимум ≈ %.0f%%).", n, d.context.maxDRR)
		problem = fmt.Sprintf("ДРР по %d кампаниям превышает порог 80%% при наличии заказов — реклама съедает маржу.", n)
		whyNow = "Каждый день с ДРР выше порога — прямой перерасход рекламного бюджета сверх эффективного уровня."
		goal = "Снизить ДРР по перечисленным кампаниям ниже 80%, не потеряв ключевые заказы."
		effect = "Более эффективное расходование рекламного бюджета без потери оборота."
		completion = "ДРР по каждой перечисленной кампании ниже 80%."
		evidenceSummary = "drr_pct и orders_7d по указанным кампаниям в Pack показывают ДРР выше порога 80% при наличии заказов."
		factSuffix = "drr_pct"
	case "ads_spend_no_orders":
		title = fmt.Sprintf("Остановить или скорректировать кампании без заказов (%d шт.)", n)
		summary = fmt.Sprintf("%d рекламных кампаний потратили бюджет за 7 дней без единого заказа (суммарно ≈ %.0f ₽).", n, d.context.totalSpend)
		problem = fmt.Sprintf("Расход по %d кампаниям за 7 дней составил ≈ %.0f ₽ при нуле заказов.", n, d.context.totalSpend)
		whyNow = "Бюджет расходуется каждый день без единого заказа — это прямая потеря без отдачи."
		goal = "Остановить или скорректировать настройки перечисленных кампаний до появления заказов."
		effect = "Прекращение расхода бюджета без отдачи по кампаниям из списка."
		completion = "По каждой перечисленной кампании расход остановлен или появились заказы за последние 7 дней."
		evidenceSummary = "spend_7d и orders_7d по указанным кампаниям в Pack показывают расход бюджета при нуле заказов."
		factSuffix = "spend_7d"
	case "no_sales_with_stock", "no_sales_persistent", "no_sales_early":
		band := "≥5"
		if d.key == "no_sales_persistent" {
			band = "14+"
		} else if d.key == "no_sales_early" {
			band = "5–13"
		}
		title = fmt.Sprintf("Разобрать %d SKU без продаж %s дней при наличии остатка", n, band)
		summary = fmt.Sprintf("%d SKU не продавались %s дней, при этом остаток на складе положительный (макс. %d дн.).", n, band, d.context.maxDaysNoSale)
		problem = fmt.Sprintf("По %d SKU остаток заморожен без продаж %s дней.", n, band)
		whyNow = "Замороженный остаток без продаж увеличивает издержки хранения и снижает оборачиваемость капитала."
		goal = "Проверить историю спроса, цену, доступность и рекламную поддержку — без утверждения уже известной причины."
		effect = "Возобновление продаж или осознанное решение по неликвиду."
		completion = "По каждому SKU сформирован вердикт (цена/наличие/реклама/спрос) и назначено следующее действие."
		evidenceSummary = "days_without_sales и available_stock_units по указанным SKU в Pack подтверждают отсутствие продаж при положительном остатке."
		factSuffix = "days_without_sales"
	case "key_sku_protection":
		title = fmt.Sprintf("Защитить наличие %d ключевых SKU с риском дефицита", n)
		summary = fmt.Sprintf("%d SKU из ядра выручки одновременно имеют риск дефицита остатка.", n)
		problem = fmt.Sprintf("%d ключевых SKU с высоким вкладом в выручку находятся в зоне стокаута или короткого покрытия.", n)
		whyNow = "Потеря доступности по ядру выручки сильнее бьёт по кабинету, чем дефицит по периферийным SKU."
		goal = "Обеспечить покрытие ≥14 дней по ключевым SKU и не допустить стокаута."
		effect = "Сохранение выручки по SKU, формирующим основной оборот."
		completion = "У всех перечисленных ключевых SKU покрытие спроса ≥ 14 дней."
		evidenceSummary = "revenue_7d и stock_status / stock_coverage_days_7d по указанным SKU в Pack показывают высокий вклад и риск дефицита."
		factSuffix = "revenue_7d"
	case "surplus":
		title = fmt.Sprintf("Снизить избыточный запас по %d SKU", n)
		summary = fmt.Sprintf("%d SKU имеют избыточный запас (покрытие более 60 дней), суммарно ≈ %d ед.", n, d.context.surplusUnits)
		problem = fmt.Sprintf("По %d SKU остаток существенно превышает текущий спрос (покрытие ≥60 дней).", n)
		whyNow = "Избыточный остаток увеличивает издержки хранения и связывает оборотный капитал. Без складского измерения перемещение между складами не предлагается."
		goal = "Снизить избыточный запас: не пополнять, скорректировать закупку, при возможности — акция или цена."
		effect = fmt.Sprintf("После сокращения закупки или иных мер объём избыточного запаса (≈ %d ед.) уменьшится, покрытие снизится ниже 60 дней.", d.context.surplusUnits)
		completion = "Прогресс: X из N SKU снизили покрытие ниже 60 дней."
		evidenceSummary = "По указанным SKU в Pack подтверждён устойчивый избыточный запас."
		factSuffix = "available_stock_units"
	default:
		title = fmt.Sprintf("Проверить %d SKU/кампаний по данным Pack", n)
		summary = "Обнаружены материальные отклонения по данным Pack, требующие проверки."
		problem = "Обнаружены материальные отклонения, не покрытые существующими Actions."
		whyNow = "Отклонения зафиксированы в текущем анализе и требуют внимания."
		goal = "Проверить и устранить причину отклонения по перечисленным сущностям."
		effect = "Снижение риска, связанного с обнаруженным отклонением."
		completion = "Отклонение по каждой перечисленной сущности устранено."
		evidenceSummary = "Данные Pack по перечисленным сущностям подтверждают материальное отклонение."
		factSuffix = "row_id"
	}
	return
}

func domainActionSteps(d materialDomain, entityIDs []string) []contract.UnifiedActionStepV1 {
	switch d.key {
	case "stock_oos":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить подтверждённые поставки и дату приёмки", Description: "Сверить, есть ли уже созданные поставки и когда товар должен поступить на склад.", EntityIDs: entityIDs},
			{Order: 2, Title: "Заказать или ускорить рекомендуемое количество", Description: "По каждому SKU использовать рассчитанный объём поставки с учётом продаж, товаров в пути и целевого покрытия.", EntityIDs: entityIDs},
			{Order: 3, Title: "При необходимости скорректировать рекламу", Description: "Временно снизить продвижение SKU без остатка, чтобы не тратить бюджет на недоступный товар."},
			{Order: 4, Title: "Проверить восстановление доступности", Description: "После приёмки убедиться, что остаток > 0 и покрытие достигло целевого уровня."},
		}
	case "stock_critical", "stock_urgent":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить дату возможного обнуления", Description: "Сверить остаток, продажи и расчётную дату обнуления по каждому SKU.", EntityIDs: entityIDs},
			{Order: 2, Title: "Учесть товары в пути", Description: "Если поставка уже в пути и успевает до обнуления — не дублировать заказ; иначе заказать рекомендуемое количество.", EntityIDs: entityIDs},
			{Order: 3, Title: "Заказать до рассчитанной даты", Description: "Создать поставку с рекомендуемым количеством до даты, указанной по каждому SKU."},
			{Order: 4, Title: "Проверить поступление", Description: "После приёмки проверить, что покрытие достигло целевого уровня."},
		}
	case "stock_short":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Сверить срок поставки", Description: "Сопоставить текущее покрытие со сроком поставки и буфером приёмки.", EntityIDs: entityIDs},
			{Order: 2, Title: "Включить в ближайшую плановую поставку", Description: "Добавить рекомендуемое количество в плановую поставку без создания экстренной, если срок позволяет.", EntityIDs: entityIDs},
			{Order: 3, Title: "Не создавать экстренную поставку без необходимости", Description: "Экстренная поставка нужна только если плановая не успевает до риска обнуления."},
			{Order: 4, Title: "Проверить покрытие после подтверждения", Description: "После подтверждения поставки убедиться, что покрытие достигло целевого уровня."},
		}
	case "sales_decline":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить остаток, цену и участие в акциях", Description: "Сопоставить падение заказов с остатком, ценой и рекламной активностью по каждому SKU.", EntityIDs: entityIDs},
			{Order: 2, Title: "Определить причину падения", Description: "Выделить основной фактор (остаток/цена/конкуренция/сезонность) по каждому SKU из списка."},
			{Order: 3, Title: "Устранить причину и отследить восстановление", Description: "Внести корректировку и проверить заказы за следующие 7 дней против предыдущей недели."},
		}
	case "ads_high_drr":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить ставки, targeting и search terms", Description: "Проверить настройки кампаний с высоким ДРР и связанные SKU.", EntityIDs: entityIDs},
			{Order: 2, Title: "Скорректировать ставки и бюджет", Description: "Снизить ставку или дневной бюджет, не отключая автоматически результативные кампании."},
			{Order: 3, Title: "Проверить ДРР после корректировки", Description: "Проконтролировать ДРР за следующие 7 дней и убедиться, что он вернулся ниже порога."},
		}
	case "ads_spend_no_orders":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить настройки кампании", Description: "Проверить таргетинг, ставки и статус связанных SKU по кампаниям без заказов.", EntityIDs: entityIDs},
			{Order: 2, Title: "Приостановить или скорректировать кампанию", Description: "Остановить кампанию либо скорректировать настройки, если причина устранима."},
			{Order: 3, Title: "Проверить результат за следующий период", Description: "Через 7 дней проверить, появились ли заказы или расход прекращён."},
		}
	case "no_sales_with_stock", "no_sales_persistent", "no_sales_early":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить историю спроса и цену", Description: "Сопоставить дни без продаж с историческими заказами и текущей ценой.", EntityIDs: entityIDs},
			{Order: 2, Title: "Проверить доступность и рекламную поддержку", Description: "Убедиться, что карточка доступна и при необходимости имеет рекламную поддержку."},
			{Order: 3, Title: "Сформировать решение по SKU", Description: "Зафиксировать вердикт без неподтверждённых причин и назначить следующее действие."},
		}
	case "key_sku_protection":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить покрытие ключевых SKU", Description: "Сверить остаток и покрытие по SKU с высоким вкладом в выручку.", EntityIDs: entityIDs},
			{Order: 2, Title: "Приоритизировать поставку", Description: "Включить ключевые SKU в ближайшую поставку с целевым покрытием по политике."},
			{Order: 3, Title: "Проконтролировать доступность", Description: "Убедиться, что после поставки ключевые SKU не уходят в стокаут."},
		}
	case "surplus":
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Подтвердить устойчиво низкий спрос", Description: "Сверить остаток, продажи и покрытие по каждому SKU из списка.", EntityIDs: entityIDs},
			{Order: 2, Title: "Остановить или сократить следующую закупку", Description: "Не пополнять позиции с избыточным запасом; скорректировать объём следующей закупки."},
			{Order: 3, Title: "Оценить акцию, цену или продвижение", Description: "Если сокращения закупки недостаточно — рассмотреть акцию, корректировку цены или усиление продвижения. Межскладское перемещение только при наличии данных по складам."},
			{Order: 4, Title: "Проверить снижение покрытия", Description: "Через контрольный период убедиться, что покрытие снизилось ниже 60 дней."},
		}
	default:
		return []contract.UnifiedActionStepV1{
			{Order: 1, Title: "Проверить данные анализа", Description: "Сверить показатели по перечисленным сущностям с данными анализа.", EntityIDs: entityIDs},
			{Order: 2, Title: "Устранить отклонение", Description: "Внести корректировку, устраняющую обнаруженное отклонение."},
			{Order: 3, Title: "Проверить результат", Description: "Проверить показатели по перечисленным сущностям после корректировки."},
		}
	}
}

func stockBandStats(cands []domainCandidate, idx *PackReferenceIndex) (covMin, covMax float64, orders int64, revenue float64) {
	covMin, covMax = 0, 0
	first := true
	for _, c := range cands {
		f, ok := idx.SKUFacts[c.entityID]
		if !ok {
			continue
		}
		orders += f.Orders7D
		revenue += f.Revenue7D
		var cov float64
		if f.CoverageDays7D != nil {
			cov = *f.CoverageDays7D
		} else if f.AvailableStock == 0 && hasRecentDemand(f) {
			cov = 0
		} else {
			continue
		}
		if first {
			covMin, covMax = cov, cov
			first = false
			continue
		}
		if cov < covMin {
			covMin = cov
		}
		if cov > covMax {
			covMax = cov
		}
	}
	return covMin, covMax, orders, revenue
}
