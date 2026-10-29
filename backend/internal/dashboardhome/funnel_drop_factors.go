package dashboardhome

import (
	"fmt"
	"strings"
)

// Drop factor semantic groups (deterministic by key; AI-provided group is ignored).
const (
	DropFactorGroupFunnelSignal      = "funnel_signal"
	DropFactorGroupBusinessSignal    = "business_signal"
	DropFactorGroupPossibleAmplifier = "possible_amplifier"
	DropFactorGroupContextHypothesis = "context_hypothesis"
)

var dropFactorGroupRank = map[string]int{
	DropFactorGroupFunnelSignal:      0,
	DropFactorGroupBusinessSignal:    1,
	DropFactorGroupPossibleAmplifier: 2,
	DropFactorGroupContextHypothesis: 3,
}

func classifyDropFactorGroupByKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "views_decline", "cart_add_decline":
		return DropFactorGroupFunnelSignal
	case "aov_drop", "revenue_drop", "average_order_value":
		return DropFactorGroupBusinessSignal
	case "stock_pressure", "stock_risks", "inventory_pressure":
		return DropFactorGroupPossibleAmplifier
	case "inefficient_ads", "ad", "ads", "advertising":
		return DropFactorGroupContextHypothesis
	default:
		return DropFactorGroupContextHypothesis
	}
}

func sortDropFactorsByGroup(items []FunnelDropFactor) []FunnelDropFactor {
	if len(items) < 2 {
		return items
	}
	out := append([]FunnelDropFactor(nil), items...)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			ri := dropFactorGroupRank[classifyDropFactorGroupByKey(out[i].Key)]
			rj := dropFactorGroupRank[classifyDropFactorGroupByKey(out[j].Key)]
			if rj < ri {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func filterDropFactorsByEvidence(items []FunnelDropFactor, ev FunnelEvidenceSnapshot) []FunnelDropFactor {
	if len(items) == 0 {
		return items
	}
	out := make([]FunnelDropFactor, 0, len(items))
	for _, it := range items {
		it = applyDropFactorEvidenceLevel(it, ev)
		if dropFactorRequiresEvidence(it) && !dropFactorHasEvidence(it, ev) {
			if it.EvidenceLevel == "" || it.EvidenceLevel == "confirmed" || it.EvidenceLevel == "likely" {
				it.EvidenceLevel = "limited"
			}
			if strings.TrimSpace(it.Limitation) == "" {
				it.Limitation = "Связь фактора с этапами воронки ограничена доступными данными."
			}
		}
		out = append(out, it)
	}
	return out
}

func applyDropFactorEvidenceLevel(it FunnelDropFactor, ev FunnelEvidenceSnapshot) FunnelDropFactor {
	level := strings.ToLower(strings.TrimSpace(it.EvidenceLevel))
	if level == "" {
		level = "likely"
	}
	if !dropFactorHasEvidence(it, ev) {
		switch level {
		case "confirmed":
			level = "hypothesis"
		case "likely":
			level = "limited"
		}
	}
	it.EvidenceLevel = level
	return it
}

func dropFactorRequiresEvidence(it FunnelDropFactor) bool {
	k := strings.ToLower(strings.TrimSpace(it.EvidenceKey))
	if k != "" {
		switch k {
		case "advertising", "ads", "ad_spend", "campaigns", "ads_weak":
			return true
		case "stock", "inventory", "stock_coverage", "stock_risks":
			return true
		case "cancellations", "returns", "cancel":
			return true
		case "average_order_value", "aov", "revenue", "aov_drop":
			return false
		}
	}
	blob := strings.ToLower(it.Label + " " + it.Explanation + " " + it.Metric)
	return factorRequiresAdvertisingEvidence(FunnelTextItem{Label: it.Label, Text: blob}) ||
		factorRequiresStockEvidence(FunnelTextItem{Label: it.Label, Text: blob}) ||
		factorRequiresCancellationEvidence(FunnelTextItem{Label: it.Label, Text: blob})
}

func dropFactorHasEvidence(it FunnelDropFactor, ev FunnelEvidenceSnapshot) bool {
	k := strings.ToLower(strings.TrimSpace(it.EvidenceKey))
	switch k {
	case "advertising", "ads", "ad_spend", "campaigns", "ads_weak":
		return ev.HasAdvertising
	case "stock", "inventory", "stock_coverage", "stock_risks":
		return ev.HasStock
	case "cancellations", "returns", "cancel":
		return ev.HasCancellations
	default:
		if factorRequiresAdvertisingEvidence(FunnelTextItem{Label: it.Label, Text: it.Explanation}) {
			return ev.HasAdvertising
		}
		if factorRequiresStockEvidence(FunnelTextItem{Label: it.Label, Text: it.Explanation}) {
			return ev.HasStock
		}
		if factorRequiresCancellationEvidence(FunnelTextItem{Label: it.Label, Text: it.Explanation}) {
			return ev.HasCancellations
		}
		return true
	}
}

func enrichDropFactorFallback(f FunnelDropFactor, stages []FunnelStage) FunnelDropFactor {
	k := strings.ToLower(strings.TrimSpace(f.Key))
	f.Group = classifyDropFactorGroupByKey(f.Key)

	if f.EvidenceLevel == "" {
		f.EvidenceLevel = "likely"
	}

	switch k {
	case "cart_add_decline":
		f.EvidenceLevel = "confirmed"
		if strings.TrimSpace(f.Explanation) == "" {
			f.Explanation = "Добавления в корзину снизились сильнее других этапов. Это прямой подтверждённый сигнал ослабления перехода от интереса к выбору товара."
		}
		if strings.TrimSpace(f.Delta) == "" {
			for _, st := range stages {
				if st.Key == "add_to_cart" && st.ChangeVsPreviousDayPct != nil {
					f.Delta = fmt.Sprintf("%s к прошлой сводке", formatSignedPercent(*st.ChangeVsPreviousDayPct))
					break
				}
			}
		}
	case "views_decline":
		if f.EvidenceLevel == "confirmed" {
			f.EvidenceLevel = "likely"
		}
		if strings.TrimSpace(f.Limitation) == "" {
			f.Limitation = "Факт снижения подтверждён, связь с падением следующего этапа вероятна."
		}
		if strings.TrimSpace(f.Explanation) == "" {
			f.Explanation = "Просмотры карточки снизились сильнее, чем показы. Это уменьшает базу для добавлений в корзину и может усиливать просадку среднего этапа."
		}
		if strings.TrimSpace(f.Delta) == "" {
			for _, st := range stages {
				if st.Key == "product_views" && st.ChangeVsPreviousDayPct != nil {
					f.Delta = fmt.Sprintf("%s к прошлой сводке", formatSignedPercent(*st.ChangeVsPreviousDayPct))
					break
				}
			}
		}
	case "aov_drop", "revenue_drop", "average_order_value":
		f.EvidenceLevel = "confirmed"
		if strings.TrimSpace(f.Limitation) == "" {
			f.Limitation = "Это подтверждённый финансовый сигнал, а не доказанная причина просадки переходов."
		}
		if strings.TrimSpace(f.Explanation) == "" {
			if k == "revenue_drop" {
				f.Explanation = "Выручка снизилась. Это не объясняет падение переходов воронки, но усиливает финансовый эффект дня."
			} else {
				f.Explanation = "Средний чек снизился. Это не объясняет падение переходов воронки, но усиливает финансовый эффект дня: даже при близком количестве заказов выручка проседает сильнее."
			}
		}
	case "stock_pressure", "stock_risks", "inventory_pressure":
		f.EvidenceLevel = "limited"
		if strings.TrimSpace(f.Limitation) == "" {
			f.Limitation = "Остатки доступны текущим срезом, поэтому влияние дефицита на этапы воронки нельзя доказать строго."
		}
		if strings.TrimSpace(f.Explanation) == "" {
			f.Explanation = "Дефицит по значимым товарам может ограничивать продажи и ухудшать переходы, но текущий срез остатков не доказывает причинность по этапам воронки."
		}
	case "inefficient_ads", "ad", "ads", "advertising":
		if f.EvidenceLevel == "confirmed" || f.EvidenceLevel == "likely" {
			f.EvidenceLevel = "hypothesis"
		}
		if strings.TrimSpace(f.Limitation) == "" {
			f.Limitation = "Рекламные данные доступны на уровне кампаний; причинность с просадкой этапов воронки не доказана."
		}
		if strings.TrimSpace(f.Explanation) == "" {
			f.Explanation = "Часть рекламного трафика расходует бюджет при слабом результате. Это гипотеза ухудшения качества привлечения, но без прямой привязки кампаний к этапам воронки причинность не доказана."
		}
	}

	// Always re-apply deterministic group (ignore AI group).
	f.Group = classifyDropFactorGroupByKey(f.Key)
	return f
}
