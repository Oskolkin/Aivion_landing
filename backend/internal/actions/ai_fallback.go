package actions

import (
	"fmt"
	"strings"
)

// BuildFallbackActions creates deterministic actions from domain candidates.
func BuildFallbackActions(domainCtx ActionDomainContext) []DomainAIAction {
	if len(domainCtx.Candidates) == 0 {
		return nil
	}
	out := make([]DomainAIAction, 0, len(domainCtx.Candidates))
	for _, cand := range domainCtx.Candidates {
		action, ok := buildFallbackActionForCandidate(cand)
		if ok {
			out = append(out, action)
		}
	}
	return out
}

func buildFallbackActionForCandidate(cand ActionCandidate) (DomainAIAction, bool) {
	factIDs := make([]string, 0, len(cand.Facts))
	for _, f := range cand.Facts {
		factIDs = append(factIDs, f.FactID)
	}
	ctas := append([]string(nil), cand.AllowedCTAs...)
	limitations := append([]string(nil), cand.Limitations...)

	title, reason, nextStep := fallbackCopyForType(cand)
	if title == "" {
		return DomainAIAction{}, false
	}

	action := DomainAIAction{
		SourceCandidateIDs: []string{cand.CandidateID},
		Type:               cand.Type,
		Category:           cand.Category,
		Priority:           cand.PriorityHint,
		Title:              title,
		Summary:            cand.Summary,
		Reason:             reason,
		NextStep:           nextStep,
		EvidenceFactIDs:    factIDs,
		CTAKeys:            pickFallbackCTAs(ctas, cand.Type),
		Limitations:        limitations,
	}
	return FinalizeDomainAction(action, []ActionCandidate{cand}), true
}

func pickFallbackCTAs(ctas []string, typ ActionType) []string {
	if len(ctas) == 0 {
		return nil
	}
	_ = typ
	// Preserve the full curated allowlist (navigate/export/ask_ai/mark_checked) in a
	// consistent order. The final mapping re-derives the displayed CTAs the same way,
	// so deterministic fallback actions are not stripped of navigate/ask_ai.
	return orderCTAKeys(ctas)
}

func fallbackCopyForType(cand ActionCandidate) (title, reason, nextStep string) {
	agg := cand.Aggregates
	skuCount := intFromAgg(agg, "sku_count", "count")
	campaignCount := intFromAgg(agg, "campaign_count", "count")
	coverageMin := floatFromAgg(agg, "coverage_min", "coverage_min_days")
	coverageMax := floatFromAgg(agg, "coverage_max_days", "coverage_max")
	threshold := floatFromAgg(agg, "drr_threshold_pct", "threshold_pct")
	avgDRR := floatFromAgg(agg, "avg_drr", "drr")
	spendTotal := floatFromAgg(agg, "spend_7d_total")
	stockUnits := floatFromAgg(agg, "stock_units_total")
	revenueShare := floatFromAgg(agg, "revenue_share_pct")
	fromStage := stringFromAgg(agg, "from_stage")
	toStage := stringFromAgg(agg, "to_stage")

	switch cand.Type {
	case ActionTypeStockReplenishment:
		switch {
		case coverageMin > 0 && coverageMax > 0:
			title = fmt.Sprintf("Пополнить %d SKU с покрытием %.1f–%.0f дней", skuCount, coverageMin, coverageMax)
		case coverageMax > 0:
			title = fmt.Sprintf("Пополнить %d SKU с покрытием до %.0f дней", skuCount, coverageMax)
		default:
			title = fmt.Sprintf("Пополнить %d SKU с низким покрытием остатков", skuCount)
		}
		reason = cand.Summary
		nextStep = "Сформируйте поставку, начиная с SKU с минимальным покрытием и наибольшей долей выручки."
	case ActionTypeNoSalesWithStock:
		if stockUnits > 0 {
			title = fmt.Sprintf("Разобрать %d SKU без заказов при остатке %s шт.", skuCount, formatThousands(stockUnits))
		} else {
			title = fmt.Sprintf("Разобрать %d SKU без продаж при наличии остатка", skuCount)
		}
		reason = cand.Summary
		nextStep = "Начните с SKU с максимальным замороженным остатком и разберите карточку, цену и видимость."
	case ActionTypeSKUOrdersDrop:
		ordersDelta := floatFromAgg(agg, "orders_delta_total", "orders_delta")
		if ordersDelta != 0 {
			title = fmt.Sprintf("Разобрать %d SKU с падением заказов на %s шт.", skuCount, formatAbsInt(ordersDelta))
		} else {
			title = fmt.Sprintf("Разобрать %d SKU с падением заказов день-к-дню", skuCount)
		}
		reason = cand.Summary
		nextStep = "Начните с SKU с максимальным вкладом в падение и проверьте видимость, конверсию и остатки."
	case ActionTypeKeySKURisk:
		if revenueShare > 0 {
			title = fmt.Sprintf("Разобрать %d ключевых SKU с рисками (доля выручки %.1f%%)", skuCount, revenueShare)
		} else {
			title = fmt.Sprintf("Разобрать %d ключевых SKU с признаками риска", skuCount)
		}
		reason = cand.Summary
		nextStep = "Сначала разберите SKU с двойным риском: падение заказов и низкое покрытие."
	case ActionTypeAdHighDRR:
		switch {
		case avgDRR > 0:
			title = fmt.Sprintf("Ограничить расходы по %d %s с ДРР %s", campaignCount, campaignsDative(campaignCount), formatPercent(avgDRR))
		case threshold > 0:
			title = fmt.Sprintf("Снизить расход по %d %s с ДРР выше %.0f%%", campaignCount, campaignsDative(campaignCount), threshold)
		default:
			title = fmt.Sprintf("Ограничить расходы по %d %s с высоким ДРР", campaignCount, campaignsDative(campaignCount))
		}
		reason = cand.Summary
		nextStep = "Снизьте бюджет или ставки, начиная с кампаний с максимальным расходом."
	case ActionTypeAdSpendNoOrders:
		if spendTotal > 0 && campaignCount == 1 {
			title = fmt.Sprintf("Поставить на паузу 1 кампанию с расходом %s ₽ и 0 заказов", formatThousands(spendTotal))
			reason = fmt.Sprintf("Кампания потратила %s ₽ за 7 дней и не принесла заказов.", formatThousands(spendTotal))
		} else if spendTotal > 0 {
			title = fmt.Sprintf("Поставить на паузу %d %s с расходом %s ₽ и 0 заказов", campaignCount, campaignsAccusative(campaignCount), formatThousands(spendTotal))
			reason = fmt.Sprintf("За 7 дней %s %d %s с расходом %s ₽ и 0 заказов.", ruPlural(campaignCount, "обнаружена", "обнаружены", "обнаружено"), campaignCount, campaignsNominative(campaignCount), formatThousands(spendTotal))
		} else {
			title = fmt.Sprintf("Поставить на паузу %d %s с расходом без заказов", campaignCount, campaignsAccusative(campaignCount))
			reason = cand.Summary
		}
		nextStep = "Поставьте кампанию на паузу или снизьте бюджет до минимума перед дальнейшим расходом."
	case ActionTypeFunnelWeakTransition:
		if fromStage != "" && toStage != "" {
			title = fmt.Sprintf("Разобрать просадку перехода %s → %s", fromStage, toStage)
		} else {
			title = "Разобрать просадку перехода воронки"
		}
		reason = cand.Summary
		nextStep = "Проверьте этап воронки и возможные причины просадки по данным аналитики."
	default:
		return "", "", ""
	}
	return title, reason, nextStep
}

func intFromAgg(agg map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := agg[k]; ok {
			switch n := v.(type) {
			case int:
				return n
			case int64:
				return int(n)
			case float64:
				return int(n)
			}
		}
	}
	return 0
}

func floatFromAgg(agg map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := agg[k]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case int:
				return float64(n)
			case int64:
				return float64(n)
			}
		}
	}
	return 0
}

func stringFromAgg(agg map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := agg[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}
