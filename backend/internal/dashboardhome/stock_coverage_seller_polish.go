package dashboardhome

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const sellerFacingRevenueMethodNote = "Средняя дневная выручка за последние 7 дней по товарам с нулевым или критическим запасом."

const defaultSKUFocusLinkLabel = "Открыть SKU с риском"

const defaultNoSalesFocusLinkLabel = "Проверить SKU без продаж"

const stockNoSalesSKUFocusTarget = "sku-focus-no-sales"

var (
	reArticulCount   = regexp.MustCompile(`(\d+)\s+артикул(?:а|ов)?`)
	reTovarCount     = regexp.MustCompile(`(\d+)\s+товар(?:а|ов)?`)
	reAnyArticulForm = regexp.MustCompile(`(?i)артикул(а|ов|у|ом|е|ы|ами|ах)?`)
)

func polishRussianArticulAndTovarCounts(s string) string {
	s = polishCountWordPhrase(s, reArticulCount, "артикул", "артикула", "артикулов")
	s = polishCountWordPhrase(s, reTovarCount, "товар", "товара", "товаров")
	return s
}

func replaceArticulFormsWithSKU(s string) string {
	if strings.TrimSpace(s) == "" {
		return s
	}
	return reAnyArticulForm.ReplaceAllString(s, "SKU")
}

func normalizeStockCoverageSummaryLanguage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "в зоне 0–3 дня", "могут закончиться в ближайшие 3 дня")
	s = strings.ReplaceAll(s, "в зоне 4–7 дней", "могут закончиться в течение недели")
	s = strings.ReplaceAll(s, "зона 0–3 дня", "период ближайших 3 дней")
	s = strings.ReplaceAll(s, "зона 4–7 дней", "период ближайшей недели")
	s = strings.ReplaceAll(s, "основная нагрузка сосредоточена", "основной риск связан")
	return strings.TrimSpace(s)
}

func polishCountWordPhrase(s string, re *regexp.Regexp, sing, paucal, plural string) string {
	return re.ReplaceAllStringFunc(s, func(full string) string {
		parts := re.FindStringSubmatch(full)
		if len(parts) < 2 {
			return full
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return full
		}
		return fmt.Sprintf("%d %s", n, russianCountWord(n, sing, paucal, plural))
	})
}

func revenueMethodNoteLooksTechnical(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return true
	}
	return strings.Contains(s, "weighted_avg") ||
		strings.Contains(s, "daily_revenue_7d") ||
		strings.Contains(s, "weighted_avg_daily_revenue_7d")
}

// polishStockCoverageNarrativeForSeller runs after successful validation: fixes common
// Russian plural mistakes from the model, strips technical revenue labels, normalizes link copy.
func polishStockCoverageNarrativeForSeller(n *StockCoverageNarrative) {
	if n == nil {
		return
	}
	n.Summary = polishRussianArticulAndTovarCounts(n.Summary)
	n.NoSalesInsight = polishRussianArticulAndTovarCounts(n.NoSalesInsight)
	n.RiskComment = polishRussianArticulAndTovarCounts(n.RiskComment)
	n.Summary = normalizeStockCoverageSummaryLanguage(n.Summary)
	n.Summary = replaceArticulFormsWithSKU(n.Summary)
	n.NoSalesInsight = replaceArticulFormsWithSKU(n.NoSalesInsight)
	n.RiskComment = replaceArticulFormsWithSKU(n.RiskComment)
	for i := range n.Actions {
		n.Actions[i].Title = replaceArticulFormsWithSKU(polishRussianArticulAndTovarCounts(n.Actions[i].Title))
		n.Actions[i].Text = replaceArticulFormsWithSKU(polishRussianArticulAndTovarCounts(n.Actions[i].Text))
	}
	for i := range n.Checklist {
		n.Checklist[i].Title = replaceArticulFormsWithSKU(polishRussianArticulAndTovarCounts(n.Checklist[i].Title))
		n.Checklist[i].Text = replaceArticulFormsWithSKU(polishRussianArticulAndTovarCounts(n.Checklist[i].Text))
	}
	for i := range n.Segments {
		n.Segments[i].Comment = polishRussianArticulAndTovarCounts(n.Segments[i].Comment)
		n.Segments[i].Comment = replaceArticulFormsWithSKU(n.Segments[i].Comment)
	}
	for i := range n.RiskExamples {
		n.RiskExamples[i].Reason = polishRussianArticulAndTovarCounts(n.RiskExamples[i].Reason)
		n.RiskExamples[i].RiskLabel = polishRussianArticulAndTovarCounts(n.RiskExamples[i].RiskLabel)
		n.RiskExamples[i].Reason = replaceArticulFormsWithSKU(n.RiskExamples[i].Reason)
		n.RiskExamples[i].RiskLabel = replaceArticulFormsWithSKU(n.RiskExamples[i].RiskLabel)
	}
	n.NoSalesWithStock.Comment = polishRussianArticulAndTovarCounts(n.NoSalesWithStock.Comment)
	n.NoSalesWithStock.Comment = replaceArticulFormsWithSKU(n.NoSalesWithStock.Comment)

	if n.RevenueAtRisk.Status == "available" {
		if revenueMethodNoteLooksTechnical(n.RevenueAtRisk.MethodNote) {
			n.RevenueAtRisk.MethodNote = sellerFacingRevenueMethodNote
		} else {
			n.RevenueAtRisk.MethodNote = polishRussianArticulAndTovarCounts(n.RevenueAtRisk.MethodNote)
			n.RevenueAtRisk.MethodNote = replaceArticulFormsWithSKU(n.RevenueAtRisk.MethodNote)
		}
	}

	label := strings.TrimSpace(n.SKUFocusLink.Label)
	lowerLabel := strings.ToLower(label)
	if label == "" ||
		strings.Contains(lowerLabel, "перейти к артикул") ||
		strings.Contains(lowerLabel, "артикул с риском") ||
		strings.Contains(lowerLabel, "sku-фокус") ||
		strings.Contains(lowerLabel, "смотреть товары") {
		n.SKUFocusLink.Label = defaultSKUFocusLinkLabel
	} else {
		n.SKUFocusLink.Label = polishRussianArticulAndTovarCounts(n.SKUFocusLink.Label)
		n.SKUFocusLink.Label = replaceArticulFormsWithSKU(n.SKUFocusLink.Label)
	}
	n.SKUFocusLink.Target = stockSKUFocusTarget
	if n.NoSalesFocusLink != nil {
		n.NoSalesFocusLink.Label = polishRussianArticulAndTovarCounts(n.NoSalesFocusLink.Label)
		n.NoSalesFocusLink.Label = replaceArticulFormsWithSKU(n.NoSalesFocusLink.Label)
		if strings.TrimSpace(n.NoSalesFocusLink.Label) == "" {
			n.NoSalesFocusLink.Label = defaultNoSalesFocusLinkLabel
		}
		n.NoSalesFocusLink.Target = stockNoSalesSKUFocusTarget
	}
	n.Headline = polishRussianArticulAndTovarCounts(n.Headline)
	n.Headline = replaceArticulFormsWithSKU(n.Headline)
	n.NextStep = polishRussianArticulAndTovarCounts(n.NextStep)
	n.NextStep = replaceArticulFormsWithSKU(n.NextStep)
	enrichStockCoverageSellerFacingFields(n)
}

func enrichStockCoverageSellerFacingFields(n *StockCoverageNarrative) {
	polishStockCoverageHeadlineSummary(n)
	if strings.TrimSpace(n.NextStep) == "" {
		if len(n.Actions) > 0 {
			n.NextStep = strings.TrimSpace(firstNonEmpty(n.Actions[0].Text, n.Actions[0].Title))
		}
		if n.NextStep == "" && len(n.Checklist) > 0 {
			n.NextStep = strings.TrimSpace(firstNonEmpty(n.Checklist[0].Text, n.Checklist[0].Title))
		}
	}
	if strings.TrimSpace(n.Tone) == "" {
		n.Tone = "warning"
	}
	if strings.TrimSpace(n.Source) == "" {
		n.Source = "dashboard_narrative_ai_only"
	}
}
