package dashboardhome

import "strings"

func buildFallbackWhatChangedPayload(ctx dashboardNarrativeWhatChangedContext) *dashboardNarrativeWhatChanged {
	groups := make([]dashboardNarrativeWhatChangedGroup, 0, len(whatChangedRequiredGroups))
	for _, spec := range whatChangedRequiredGroups {
		group := dashboardNarrativeWhatChangedGroup{
			Key:   spec.Key,
			Label: spec.Label,
			Tone:  spec.Tone,
			Items: []dashboardNarrativeWhatChangedItem{},
		}
		switch spec.Key {
		case whatChangedKeyWorsened:
			group.Summary, group.Items, group.EmptyText = fallbackGroupFromFacts(ctx.WorsenedFacts, "Заметных ухудшений относительно прошлой сводки не выявлено.")
		case whatChangedKeyNewRisks:
			group.Summary, group.Items, group.EmptyText = fallbackGroupFromFacts(ctx.NewRiskFacts, "Новых рисков относительно прошлой сводки не выявлено.")
		case whatChangedKeyUnchangedRisks:
			group.Summary, group.Items, group.EmptyText = fallbackGroupFromFacts(ctx.UnchangedRiskFacts, "Повторяющихся рисков из прошлой сводки не выявлено.")
		case whatChangedKeyImproved:
			group.Summary, group.Items, group.EmptyText = fallbackImprovedGroup(ctx.ImprovedFacts)
		}
		groups = append(groups, group)
	}
	return &dashboardNarrativeWhatChanged{
		Summary: buildFallbackWhatChangedSummary(ctx),
		Groups:  groups,
	}
}

func buildFallbackWhatChangedSummary(ctx dashboardNarrativeWhatChangedContext) string {
	if ctx.PreviousMetricDate == "" {
		return "Сравнение с прошлой сводкой пока недоступно — не хватает данных за предыдущий период."
	}
	if len(ctx.WorsenedFacts) > 0 && len(ctx.ImprovedFacts) == 0 {
		return "Относительно прошлой сводки сильнее проявились негативные изменения, заметных улучшений пока нет."
	}
	if len(ctx.ImprovedFacts) > 0 && len(ctx.WorsenedFacts) == 0 {
		return "Относительно прошлой сводки появились положительные сдвиги без выраженных ухудшений."
	}
	if len(ctx.WorsenedFacts) > 0 {
		return "Относительно прошлой сводки одновременно видны и ухудшения, и отдельные улучшения."
	}
	return "Относительно прошлой сводки существенных изменений по ключевым метрикам не выявлено."
}

func fallbackImprovedGroup(facts []whatChangedFact) (string, []dashboardNarrativeWhatChangedItem, string) {
	if len(facts) == 0 {
		return "Заметных улучшений по сравнению с прошлой сводкой не выявлено.",
			[]dashboardNarrativeWhatChangedItem{},
			"Текущие риски пока не компенсируются улучшениями в продажах, остатках или рекламе."
	}
	return fallbackGroupFromFacts(facts, "")
}

func fallbackGroupFromFacts(facts []whatChangedFact, emptyText string) (string, []dashboardNarrativeWhatChangedItem, string) {
	if len(facts) == 0 {
		return emptyText, []dashboardNarrativeWhatChangedItem{}, emptyText
	}
	items := make([]dashboardNarrativeWhatChangedItem, 0, minInt(len(facts), whatChangedGroupMaxItems))
	for _, fact := range facts {
		if len(items) >= whatChangedGroupMaxItems {
			break
		}
		text, highlights := fallbackFactText(fact)
		items = append(items, dashboardNarrativeWhatChangedItem{
			Label:      fact.Label,
			Text:       text,
			Highlights: highlights,
		})
	}
	summary := fallbackGroupSummary(facts)
	return summary, items, ""
}

func fallbackGroupSummary(facts []whatChangedFact) string {
	if len(facts) == 0 {
		return ""
	}
	switch facts[0].Direction {
	case whatChangedKeyWorsened:
		return "Главное ухудшение связано с " + strings.TrimSpace(facts[0].Label) + "."
	case whatChangedKeyNewRisks:
		return "Появились новые сигналы, которые могут быстро повлиять на результат."
	case whatChangedKeyUnchangedRisks:
		return "Часть проблем не новая, но продолжает давить на кабинет."
	default:
		return "Появились улучшения по сравнению с прошлой сводкой."
	}
}

func fallbackFactText(fact whatChangedFact) (string, []string) {
	highlights := make([]string, 0, 3)
	parts := make([]string, 0, 4)
	if fact.EntityName != "" {
		if strings.HasPrefix(fact.ID, "ad-") {
			parts = append(parts, "«"+fact.EntityName+"»")
		} else {
			parts = append(parts, fact.EntityName)
		}
	}
	if fact.CurrentValue != "" {
		parts = append(parts, fact.CurrentValue)
		highlights = append(highlights, fact.CurrentValue)
	}
	if fact.PreviousValue != "" {
		parts = append(parts, "против "+fact.PreviousValue+" в прошлой сводке")
		highlights = append(highlights, fact.PreviousValue)
	}
	if fact.DeltaAbs != "" {
		parts = append(parts, "на "+fact.DeltaAbs+" "+fact.Note)
		highlights = append(highlights, fact.DeltaAbs)
	} else if fact.Note != "" {
		parts = append(parts, fact.Note)
	}
	text := strings.TrimSpace(strings.Join(parts, ", "))
	if text == "" {
		text = fact.Note
	}
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text, compactStringSlice(highlights)
}
