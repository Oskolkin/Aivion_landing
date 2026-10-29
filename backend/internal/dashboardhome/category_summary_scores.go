package dashboardhome

import (
	"strings"
)

const categoryScoreMax = int32(10)

func finalizeCategorySummaryScores(cards []CategorySummaryCard) []CategorySummaryCard {
	if len(cards) == 0 {
		return cards
	}
	out := make([]CategorySummaryCard, 0, len(cards))
	for _, card := range cards {
		card = enrichCategorySummaryScore(card)
		out = append(out, card)
	}
	return out
}

func enrichCategorySummaryScore(card CategorySummaryCard) CategorySummaryCard {
	if card.CategoryScore != nil && *card.CategoryScore >= 0 && *card.CategoryScore <= categoryScoreMax {
		card.ScoreMax = categoryScoreMax
		if card.HealthScore == nil {
			v := *card.CategoryScore * 10
			card.HealthScore = &v
		}
		return card
	}
	if card.HealthScore != nil && *card.HealthScore >= 0 && *card.HealthScore <= 100 {
		score10 := int32((*card.HealthScore + 5) / 10)
		if score10 > categoryScoreMax {
			score10 = categoryScoreMax
		}
		card.CategoryScore = &score10
		card.ScoreMax = categoryScoreMax
		return card
	}
	score := computeCategoryConditionScore10(card)
	card.CategoryScore = &score
	card.ScoreMax = categoryScoreMax
	health := score * 10
	card.HealthScore = &health
	if strings.TrimSpace(card.Severity) == "" {
		card.Severity = severityForCategoryScore10(score)
	}
	return card
}

func computeCategoryConditionScore10(card CategorySummaryCard) int32 {
	score := severityBaselineScore10(card.Severity)
	risk := strings.ToLower(strings.TrimSpace(card.KeyRisk))
	if strings.Contains(risk, "дефицит") || strings.Contains(risk, "критич") || strings.Contains(risk, "коротк") {
		score -= 1
	}
	if strings.Contains(risk, "без продаж") || strings.Contains(risk, "зависш") {
		score -= 1
	}
	if strings.Contains(risk, "излиш") || strings.Contains(risk, "оборачива") {
		score -= 1
	}
	if strings.Contains(risk, "дрр") || strings.Contains(risk, "реклам") {
		score -= 1
	}
	headline := strings.ToLower(strings.TrimSpace(card.Headline))
	if strings.Contains(headline, "лидер") || strings.Contains(headline, "рост") {
		score += 1
	}
	return clampCategoryScore10(score)
}

func severityBaselineScore10(severity string) int32 {
	switch strings.TrimSpace(strings.ToLower(severity)) {
	case "critical":
		return 2
	case "high":
		return 3
	case "medium":
		return 5
	case "low":
		return 7
	case "stable", "positive":
		return 9
	default:
		return 5
	}
}

func severityForCategoryScore10(score int32) string {
	switch {
	case score <= 3:
		return "high"
	case score <= 6:
		return "medium"
	case score <= 8:
		return "low"
	default:
		return "stable"
	}
}

func clampCategoryScore10(score int32) int32 {
	if score < 0 {
		return 0
	}
	if score > categoryScoreMax {
		return categoryScoreMax
	}
	return score
}
