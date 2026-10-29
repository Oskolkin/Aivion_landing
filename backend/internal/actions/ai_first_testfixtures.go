package actions

import "strings"

// ValidV2DraftFromCandidateForTests builds a validation-friendly V2 draft for integration tests.
func ValidV2DraftFromCandidateForTests(c AIFirstActionCandidate) AIFirstDraftAction {
	return ValidDetailedPilotDraftFromCandidateForTests(c)
}

// ValidDetailedPilotDraftFromCandidateForTests builds a structurally complete V2 draft
// that passes pilot quality validation.
func ValidDetailedPilotDraftFromCandidateForTests(c AIFirstActionCandidate) AIFirstDraftAction {
	ent := c.AffectedEntities[0]
	longReason := strings.TrimSpace(c.Problem + " " + c.WhyActionable + " " +
		"По данным evidence_fact_ids видны конкретные метрики по затронутым объектам. " +
		"Нужно обработать объекты с наибольшим риском для кабинета и проверить динамику через 7 дней.")
	if len([]rune(longReason)) < aiFirstMinObjectBasedReasonLen {
		longReason = longReason + strings.Repeat(" Нужны конкретные шаги по каждому объекту.", 4)
	}
	draft := AIFirstDraftAction{
		Type:                   c.Type,
		Category:               c.Category,
		Priority:               ActionPriorityCritical,
		Title:                  "Разобрать приоритетные объекты по " + c.CandidateID,
		Summary:                c.RecommendedAction + " по затронутым объектам из candidate",
		Reason:                 longReason,
		NextStep:               "Открыть список затронутых объектов и начать с наиболее приоритетного SKU по остатку",
		ExpectedEffect:         "Снижение количества проблемных объектов и возврат части товарного остатка в активные продажи.",
		CompletionCriteria:     "По каждому объекту из списка принято решение и через 7 дней проверена динамика заказов.",
		EvidenceFactIDs:        append([]string(nil), c.EvidenceFactIDs...),
		AffectedEntities:       append([]AIFirstAffectedEntityRef(nil), c.AffectedEntities...),
		AffectedObjectsSummary: "Top-объекты отобраны по evidence из source candidate.",
		ObjectActions: []AIFirstObjectAction{{
			EntityType:        ent.EntityType,
			EntityID:          ent.EntityID,
			Name:              ent.Name,
			RecommendedAction: c.RecommendedAction + " для " + ent.EntityID,
			Reason:            c.Problem,
			EvidenceFactIDs:   append([]string(nil), c.EvidenceFactIDs...),
			Metrics: []AIFirstObjectMetric{
				{Label: "Остаток", Value: "120", Unit: "шт"},
				{Label: "Заказы 7д", Value: "0"},
			},
		}},
		ActionPlan: []AIFirstActionStep{
			{
				Title:          "Открыть список объектов",
				Instruction:    "Открыть список затронутых SKU и начать с товаров с максимальным остатком и нулевыми заказами за 7 дней.",
				ExpectedResult: "Определён порядок обработки объектов.",
			},
			{
				Title:          "Проверить карточку SKU",
				Instruction:    "По каждому SKU проверить цену, фото, название, отзывы и видимость карточки.",
				ExpectedResult: "Выявлены SKU с проблемами контента.",
			},
			{
				Title:          "Запустить стимулирование спроса",
				Instruction:    "Для SKU с нормальной карточкой запустить акцию, скидку или перераспределить рекламное внимание на кампании.",
				ExpectedResult: "Запущены меры стимулирования спроса.",
			},
			{
				Title:          "Проверить результат через 7 дней",
				Instruction:    "Через 7 дней проверить, появились ли заказы и снизился ли остаток по каждому SKU.",
				ExpectedResult: "Зафиксирована динамика заказов и остатков.",
			},
		},
		SourceCandidateIDs: []string{c.CandidateID},
	}
	switch c.Type {
	case ActionTypeStockReplenishment, ActionTypeStockRedistribution, ActionTypeNoSalesWithStock:
		draft.VisibleLimitations = []string{"Остатки и поставки in-transit учитываются по snapshot stock analytics."}
	case ActionTypeAdHighDRRReview, ActionTypeAdBudgetReallocation, ActionTypeAdSpendNoOrders, ActionTypeAdHighDRR:
		draft.VisibleLimitations = []string{"Рекомендация на уровне кампании campaign."}
	}
	return draft
}

// ValidMinimalPilotDraftFromCandidateForTests builds a pilot-minimal V2 draft without optional fields.
func ValidMinimalPilotDraftFromCandidateForTests(c AIFirstActionCandidate) AIFirstDraftAction {
	return AIFirstDraftAction{
		Type:               c.Type,
		Category:           c.Category,
		Priority:           ActionPriorityCritical,
		Title:              "Заголовок действия " + c.CandidateID,
		Reason:             c.Problem,
		NextStep:           c.RecommendedAction,
		CompletionCriteria: "Критерий выполнен",
		EvidenceFactIDs:    append([]string(nil), c.EvidenceFactIDs...),
		AffectedEntities:   append([]AIFirstAffectedEntityRef(nil), c.AffectedEntities...),
		SourceCandidateIDs: []string{c.CandidateID},
		ScopeType:          AIFirstActionScopeEntity,
	}
}
