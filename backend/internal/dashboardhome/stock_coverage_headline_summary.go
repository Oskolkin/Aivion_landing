package dashboardhome

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	stockCoverageHeadlineMaxRunes = 80
	stockCoverageSummaryMinRunes  = 80
	stockCoverageSummaryMaxRunes  = 350
)

func stockCoverageSegmentCounts(segments []StockCoverageNarrativeSegment) (oos, critical, shortage int) {
	for _, s := range segments {
		switch s.Key {
		case "out_of_stock":
			oos = s.Count
		case "critical_0_3":
			critical = s.Count
		case "shortage_4_7":
			shortage = s.Count
		}
	}
	return oos, critical, shortage
}

func buildStockCoverageSellerHeadline(segments []StockCoverageNarrativeSegment, noSalesCount int) string {
	oos, critical, shortage := stockCoverageSegmentCounts(segments)
	switch {
	case oos > 0 || critical > 0 || shortage > 0:
		return "Есть риск дефицита у продающих SKU"
	case noSalesCount > 0:
		return "Есть зависший запас без продаж"
	default:
		return "Покрытие остатков в норме"
	}
}

func buildStockCoverageSellerSummary(segments []StockCoverageNarrativeSegment, noSalesCount int) string {
	oos, critical, shortage := stockCoverageSegmentCounts(segments)
	switch {
	case oos > 0 || critical > 0 || shortage > 0:
		parts := make([]string, 0, 3)
		if oos > 0 {
			parts = append(parts, fmt.Sprintf("%d SKU без остатка", oos))
		}
		if critical > 0 {
			parts = append(parts, fmt.Sprintf("%d SKU с покрытием 0–3 дня", critical))
		}
		if shortage > 0 {
			parts = append(parts, fmt.Sprintf("%d SKU с покрытием 4–7 дней", shortage))
		}
		summary := strings.Join(parts, ", ") + "."
		summary += " Приоритет — пополнить товары с подтверждённым спросом, чтобы не потерять продажи на ходовых позициях."
		return strings.TrimSpace(summary)
	case noSalesCount > 0:
		return fmt.Sprintf(
			"%d SKU с остатком не дали заказов за период — стоит отдельно разобрать зависший запас и снизить риск заморозки оборотных средств.",
			noSalesCount,
		)
	default:
		return "По доступным данным критичных зон дефицита и короткого запаса не выявлено. Продолжайте отслеживать темп продаж и покрытие по ходовым позициям."
	}
}

func defaultStockCoverageNextStep(segments []StockCoverageNarrativeSegment, noSalesCount int) string {
	oos, critical, shortage := stockCoverageSegmentCounts(segments)
	switch {
	case oos > 0 || critical > 0 || shortage > 0:
		return "Открыть SKU с риском и приоритизировать поставку по темпу продаж и текущему покрытию."
	case noSalesCount > 0:
		return "Проверить SKU без продаж и решить, что делать с зависшим остатком."
	default:
		return "Поддерживать запас по ходовым SKU и следить за сигналами короткого покрытия."
	}
}

func stockCoverageHeadlineLooksInvalid(headline string) bool {
	headline = strings.TrimSpace(headline)
	if headline == "" {
		return true
	}
	if utf8.RuneCountInString(headline) > stockCoverageHeadlineMaxRunes {
		return true
	}
	if strings.Contains(headline, "...") || strings.Contains(headline, "…") || strings.Contains(headline, "\uFFFD") {
		return true
	}
	if strings.Contains(headline, ":") && containsDigits(headline) {
		return true
	}
	return false
}

func stockCoverageSummaryLooksInvalid(summary, headline string) bool {
	summary = strings.TrimSpace(summary)
	headline = strings.TrimSpace(headline)
	if summary == "" {
		return true
	}
	if strings.Contains(summary, "...") || strings.Contains(summary, "…") || strings.Contains(summary, "\uFFFD") {
		return true
	}
	runes := utf8.RuneCountInString(summary)
	if runes < stockCoverageSummaryMinRunes {
		return true
	}
	if runes > stockCoverageSummaryMaxRunes {
		return true
	}
	if headline != "" && strings.EqualFold(normalizeComparableText(summary), normalizeComparableText(headline)) {
		return true
	}
	if headline != "" && strings.HasPrefix(normalizeComparableText(summary), normalizeComparableText(headline)) {
		return true
	}
	return false
}

func normalizeComparableText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.TrimSuffix(text, ".")
	text = strings.TrimSuffix(text, "!")
	text = strings.TrimSuffix(text, "?")
	return strings.Join(strings.Fields(text), " ")
}

func containsDigits(text string) bool {
	for _, r := range text {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func polishStockCoverageHeadlineSummary(n *StockCoverageNarrative) {
	if n == nil {
		return
	}
	noSales := 0
	if n.NoSalesWithStock.Status == "available" {
		noSales = n.NoSalesWithStock.Count
	}
	headline := strings.TrimSpace(n.Headline)
	summary := strings.TrimSpace(n.Summary)

	if stockCoverageHeadlineLooksInvalid(headline) {
		n.Headline = buildStockCoverageSellerHeadline(n.Segments, noSales)
	} else {
		n.Headline = truncateRunes(headline, stockCoverageHeadlineMaxRunes)
	}

	if stockCoverageSummaryLooksInvalid(summary, n.Headline) {
		n.Summary = buildStockCoverageSellerSummary(n.Segments, noSales)
	} else {
		n.Summary = truncateRunes(summary, stockCoverageSummaryMaxRunes)
	}

	if strings.TrimSpace(n.NextStep) == "" {
		n.NextStep = defaultStockCoverageNextStep(n.Segments, noSales)
	}
}
