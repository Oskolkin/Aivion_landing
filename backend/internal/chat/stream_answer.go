package chat

import (
	"strings"
	"unicode/utf8"
)

func synthesizeStreamAnswer(text string, ctx *FactContext) ChatAnswer {
	answer := ChatAnswer{
		Answer:          strings.TrimSpace(text),
		Summary:         truncateStreamSummary(text, 240),
		ConfidenceLevel: ConfidenceLevelMedium,
	}
	if ctx == nil {
		answer.ConfidenceLevel = ConfidenceLevelLow
		answer.Limitations = []string{"Контекст фактов недоступен."}
		answer.SupportingFacts = []SupportingFact{{Source: "limitation", Fact: answer.Limitations[0]}}
		return answer
	}
	answer.Intent = ctx.Intent
	answer.RelatedAlertIDs = relatedAlertIDs(ctx)
	answer.RelatedRecommendationIDs = relatedRecommendationIDs(ctx)
	answer.Limitations = append([]string(nil), ctx.Limitations...)
	answer.SupportingFacts = streamSupportingFacts(ctx)
	hasFacts := factContextHasFacts(ctx)
	if !hasFacts {
		limitation := "В контексте нет фактических данных кабинета; ответ носит общий характер."
		if len(answer.Limitations) == 0 {
			answer.Limitations = []string{limitation}
		}
		hasLimitationSource := false
		for _, sf := range answer.SupportingFacts {
			if strings.EqualFold(strings.TrimSpace(sf.Source), "limitation") {
				hasLimitationSource = true
				break
			}
		}
		if !hasLimitationSource {
			answer.SupportingFacts = append(answer.SupportingFacts, SupportingFact{Source: "limitation", Fact: limitation})
		}
		answer.ConfidenceLevel = ConfidenceLevelLow
	} else if len(answer.SupportingFacts) == 0 {
		limitation := "В контексте нет фактических данных для подтверждения ответа."
		answer.Limitations = append(answer.Limitations, limitation)
		answer.SupportingFacts = []SupportingFact{{Source: "limitation", Fact: limitation}}
		answer.ConfidenceLevel = ConfidenceLevelLow
	} else if len(answer.Limitations) == 0 {
		answer.ConfidenceLevel = ConfidenceLevelHigh
	}
	return answer
}

func streamSupportingFacts(ctx *FactContext) []SupportingFact {
	facts := make([]SupportingFact, 0, 4)
	for _, item := range ctx.Facts.Other {
		if source, _ := item["source"].(string); source == "attachment" {
			name, _ := item["file_name"].(string)
			facts = append(facts, SupportingFact{Source: "attachment", Fact: "Использован контекст вложения: " + strings.TrimSpace(name)})
			break
		}
	}
	if ctx.Action != nil {
		label := firstNonEmpty(ctx.Action.Title, ctx.Action.Summary, ctx.Action.Reason)
		if label != "" {
			facts = append(facts, SupportingFact{Source: "action_context", Fact: label})
		}
	}
	for _, result := range ctx.ToolResults {
		if result.Error != nil {
			continue
		}
		facts = append(facts, SupportingFact{Source: "tool_result", Fact: "Использованы данные инструмента: " + result.Name})
		break
	}
	if len(facts) == 0 && factContextHasFacts(ctx) {
		facts = append(facts, SupportingFact{Source: "tool_result", Fact: "Ответ основан на предоставленном фактическом контексте."})
	}
	for _, limitation := range ctx.Limitations {
		if strings.TrimSpace(limitation) != "" {
			facts = append(facts, SupportingFact{Source: "limitation", Fact: limitation})
			break
		}
	}
	return facts
}

func relatedAlertIDs(ctx *FactContext) []int64 {
	ids := make([]int64, 0, len(ctx.RelatedAlerts))
	for _, item := range ctx.RelatedAlerts {
		ids = append(ids, item.ID)
	}
	return ids
}

func relatedRecommendationIDs(ctx *FactContext) []int64 {
	ids := make([]int64, 0, len(ctx.RelatedRecommendations))
	for _, item := range ctx.RelatedRecommendations {
		ids = append(ids, item.ID)
	}
	return ids
}

func truncateStreamSummary(value string, limit int) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return string([]rune(value)[:limit]) + "…"
}
