package dashboardhome

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var aiSeesPhrasePattern = regexp.MustCompile(`(?i)\b(ai|система)\s+видит\b`)

func containsAISeesSellerFacingPhrase(text string) bool {
	return aiSeesPhrasePattern.MatchString(strings.TrimSpace(text))
}

var aiSeesCombinationPrefix = regexp.MustCompile(`(?i)^(ai|система)\s+видит\s+сочетание`)
var aiSeesPrefix = regexp.MustCompile(`(?i)^(ai|система)\s+видит\s+`)

func repairAISeesSellerFacingPhrase(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if aiSeesCombinationPrefix.MatchString(text) {
		return aiSeesCombinationPrefix.ReplaceAllString(text, "В кабинете сочетаются")
	}
	if aiSeesPrefix.MatchString(text) {
		return aiSeesPrefix.ReplaceAllString(text, "В кабинете ")
	}
	return aiSeesPhrasePattern.ReplaceAllString(text, "В кабинете")
}

func isCategoryScoreSeverityConsistent(score int32, severity string) bool {
	sev := strings.ToLower(strings.TrimSpace(severity))
	switch {
	case score <= 3:
		return sev == "high" || sev == "critical"
	case score <= 6:
		return sev == "medium"
	case score <= 8:
		return sev == "low"
	default:
		return sev == "low" || sev == "stable" || sev == "positive"
	}
}

func allowedImprovedGroupLabel(label string, hasItems bool) bool {
	label = strings.TrimSpace(label)
	if !hasItems {
		switch label {
		case "Точки опоры", "Сильные стороны", "Точки роста":
			return true
		default:
			return false
		}
	}
	switch label {
	case "Улучшилось", "Точки роста", "Точки опоры", "Сильные стороны":
		return true
	default:
		return false
	}
}

func defaultImprovedGroupLabel(hasItems bool) string {
	if hasItems {
		return "Улучшилось"
	}
	return "Точки опоры"
}

func improvedGroupIsStrengthsNotProvenImprovement(group dashboardNarrativeWhatChangedGroup) bool {
	if len(group.Items) == 0 {
		return true
	}
	improvementMarkers := []string{
		"улучш", "вырос", "выросл", "снизил", "+%", "−%", "-%", "рост ", "лучше чем",
		"выше чем", "по сравнению", "увеличил", "восстанов",
	}
	for _, item := range group.Items {
		text := strings.ToLower(strings.TrimSpace(item.Label + " " + item.Text))
		for _, marker := range improvementMarkers {
			if strings.Contains(text, marker) {
				return false
			}
		}
	}
	return true
}

func resolveImprovedGroupLabel(group dashboardNarrativeWhatChangedGroup) string {
	hasItems := len(group.Items) > 0
	raw := strings.TrimSpace(group.Label)
	if !hasItems {
		if allowedImprovedGroupLabel(raw, false) {
			return raw
		}
		return defaultImprovedGroupLabel(false)
	}
	if improvedGroupIsStrengthsNotProvenImprovement(group) {
		if raw == "Сильные стороны" || raw == "Точки опоры" || raw == "Точки роста" {
			return raw
		}
		return "Точки опоры"
	}
	if raw == "Улучшилось" || raw == "Точки роста" {
		return raw
	}
	return "Улучшилось"
}

func hasDuplicateWhatChangedLabelPrefix(label, text string) bool {
	label = strings.TrimSpace(label)
	text = strings.TrimSpace(text)
	if label == "" || text == "" {
		return false
	}
	prefix := label + ":"
	if !strings.HasPrefix(text, prefix) {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(text, prefix))
	if rest == "" {
		return false
	}
	restLower := strings.ToLower(rest)
	labelLower := strings.ToLower(label)
	return strings.HasPrefix(restLower, labelLower)
}

func repairDuplicateWhatChangedLabelPrefix(label, text string) string {
	if !hasDuplicateWhatChangedLabelPrefix(label, text) {
		return text
	}
	prefix := strings.TrimSpace(label) + ":"
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), prefix))
	restLower := strings.ToLower(rest)
	labelLower := strings.ToLower(strings.TrimSpace(label))
	if strings.HasPrefix(restLower, labelLower) {
		rest = strings.TrimSpace(rest[utf8.RuneCountInString(label):])
		rest = strings.TrimLeft(rest, ": ")
	}
	if rest == "" {
		return text
	}
	return prefix + " " + rest
}

func deriveShortActionTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if idx := strings.IndexAny(text, ".!?"); idx > 0 && idx <= 80 {
		return strings.TrimSpace(text[:idx])
	}
	if utf8.RuneCountInString(text) <= 72 {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:72])) + "…"
}

func funnelComparisonUnavailablePhrases(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	patterns := []string{
		"сравнение с предыдущим дн",
		"сравнение с прошлым дн",
		"сравнение с предыдущим днем не",
		"сравнение с предыдущим днём не",
		"данные сравнения с предыдущим дн",
		"нет данных для сравнения с предыдущим дн",
		"сравнение с предыдущим днём недоступ",
		"сравнение с предыдущим днем недоступ",
		"сравнение с прошлым днём не",
		"сравнение с прошлым днем не",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
