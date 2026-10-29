package dashboardhome

import (
	"strings"
	"unicode/utf8"
)

func normalizeRiskDriverSellerFacingTitle(driver RiskDriver) string {
	title := strings.TrimSpace(driver.Title)
	reason := strings.TrimSpace(driver.Reason)
	if title != "" && title != reason && utf8.RuneCountInString(title) <= 80 && !riskDriverTitleLooksBad(title) {
		return title
	}
	if short := riskDriverTitleFromCategory(driver.Category); short != "" {
		return short
	}
	if reason != "" {
		return truncateRunes(firstRiskDriverSentence(reason), 80)
	}
	return "Требует внимания"
}

func riskDriverTitleLooksBad(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	if lower == "" {
		return true
	}
	if strings.HasPrefix(lower, "за последние") || strings.HasPrefix(lower, "у sku") {
		return true
	}
	if strings.Contains(title, "...") || strings.Contains(title, "…") {
		return true
	}
	return utf8.RuneCountInString(title) > 80
}

func riskDriverTitleFromCategory(category string) string {
	switch strings.TrimSpace(strings.ToLower(category)) {
	case "sales":
		return "Недельная выручка снизилась"
	case "stock", "sku":
		return "Риск дефицита у ходовых SKU"
	case "surplus":
		return "Медленная оборачиваемость запасов"
	case "advertising":
		return "Рекламный перерасход"
	case "funnel":
		return "Слабый верх воронки"
	case "cancellations":
		return "Рост отмен и возвратов"
	default:
		return ""
	}
}

func firstRiskDriverSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.IndexAny(text, ".!?"); idx > 0 {
		return strings.TrimSpace(text[:idx+1])
	}
	return text
}
