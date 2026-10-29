package dashboardhome

import (
	"fmt"
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func enrichWeakestStageFromComparisons(ws *FunnelWeakestStage, cmp FunnelComparisons) {
	enrichWeakestStageMetrics(ws, nil, cmp)
}

func findComparisonDelta(items []FunnelComparisonItem, stageKey, metric, label string) string {
	want := comparisonKeyAliases(stageKey)
	for _, it := range items {
		k := strings.ToLower(strings.TrimSpace(it.Key))
		if k != "" {
			for _, w := range want {
				if w == k {
					return strings.TrimSpace(it.Delta)
				}
			}
		}
	}
	for _, it := range items {
		l := strings.TrimSpace(it.Label)
		if l != "" && (l == metric || l == label) {
			return strings.TrimSpace(it.Delta)
		}
	}
	return ""
}

func comparisonKeyAliases(stageKey string) []string {
	k := strings.ToLower(strings.TrimSpace(stageKey))
	out := []string{k}
	switch k {
	case "product_views":
		out = append(out, "card_views", "views", "product_view")
	case "add_to_cart":
		out = append(out, "cart", "add_to_basket")
	case "impressions":
		out = append(out, "shows")
	case "orders":
		out = append(out, "order")
	case "delivered_units":
		out = append(out, "delivered", "completed_orders")
	}
	return out
}

func canonicalComparisonKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "cart_to_order", "cart_to_orders":
		return "add_to_cart_to_orders"
	default:
		return k
	}
}

// mergeComparisonItems fills gaps in primary from supplemental slices: missing keys and empty deltas.
func mergeComparisonItems(primary []FunnelComparisonItem, supplemental ...[]FunnelComparisonItem) []FunnelComparisonItem {
	extra := make([]FunnelComparisonItem, 0)
	for _, s := range supplemental {
		extra = append(extra, s...)
	}
	if len(extra) == 0 {
		return primary
	}
	byKey := make(map[string]FunnelComparisonItem, len(primary)+len(extra))
	order := make([]string, 0, len(primary)+len(extra))
	add := func(item FunnelComparisonItem) {
		k := canonicalComparisonKey(item.Key)
		if k == "" {
			return
		}
		item.Key = k
		if _, seen := byKey[k]; !seen {
			order = append(order, k)
		}
		byKey[k] = item
	}
	for _, item := range primary {
		add(item)
	}
	for _, item := range extra {
		k := canonicalComparisonKey(item.Key)
		if k == "" {
			continue
		}
		item.Key = k
		cur, ok := byKey[k]
		if !ok {
			order = append(order, k)
			byKey[k] = item
			continue
		}
		if strings.TrimSpace(cur.Delta) == "" && strings.TrimSpace(item.Delta) != "" {
			cur.Delta = item.Delta
		}
		if strings.TrimSpace(cur.Value) == "" && strings.TrimSpace(item.Value) != "" {
			cur.Value = item.Value
		}
		if strings.TrimSpace(cur.Label) == "" && strings.TrimSpace(item.Label) != "" {
			cur.Label = item.Label
		}
		if cur.Tone == "" || cur.Tone == "neutral" {
			if item.Tone != "" {
				cur.Tone = item.Tone
			}
		}
		byKey[k] = cur
	}
	out := make([]FunnelComparisonItem, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

// buildPreviousBusinessComparisons adds revenue (and AOV when missing) vs previous account analytics day.
func buildPreviousBusinessComparisons(current, previous *dbgen.ProductAnalyticsDaily) []FunnelComparisonItem {
	if current == nil || previous == nil {
		return nil
	}
	curRev := numericToFloat(current.Revenue)
	prevRev := numericToFloat(previous.Revenue)
	curOrdPtr := pgInt8Ptr(current.OrdersCount)
	prevOrdPtr := pgInt8Ptr(previous.OrdersCount)

	out := make([]FunnelComparisonItem, 0, 2)
	if curRev > 0 && prevRev > 0 {
		pct := math.Round(((curRev-prevRev)/prevRev*100)*100) / 100
		out = append(out, FunnelComparisonItem{
			Key:   "revenue",
			Label: "Выручка",
			Value: formatMoney(curRev),
			Delta: formatSignedPercent(pct),
			Tone:  toneByDelta(pct),
		})
	}
	if curRev > 0 && prevRev > 0 && curOrdPtr != nil && prevOrdPtr != nil && *curOrdPtr > 0 && *prevOrdPtr > 0 {
		curAOV := curRev / float64(*curOrdPtr)
		prevAOV := prevRev / float64(*prevOrdPtr)
		if prevAOV > 0 {
			pct := math.Round(((curAOV-prevAOV)/prevAOV*100)*100) / 100
			out = append(out, FunnelComparisonItem{
				Key:   "average_order_value",
				Label: "Средний чек",
				Value: formatMoney(curAOV),
				Delta: formatSignedPercent(pct),
				Tone:  toneByDelta(pct),
			})
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildPreviousComparisons(stages []FunnelStage) []FunnelComparisonItem {
	out := make([]FunnelComparisonItem, 0, len(stages))
	for _, s := range stages {
		if s.Value == nil {
			continue
		}
		delta, tone := "", "neutral"
		if s.ChangeVsPreviousDayPct != nil {
			delta = formatSignedPercent(*s.ChangeVsPreviousDayPct)
			tone = toneByDelta(*s.ChangeVsPreviousDayPct)
		}
		out = append(out, FunnelComparisonItem{
			Key: s.Key, Label: s.Label, Value: formatInt(*s.Value), Delta: delta, Tone: tone,
		})
	}
	return out
}

func deriveWeakestStage(stages []FunnelStage) FunnelWeakestStage {
	bestIdx := -1
	bestScore := 0.0
	for i, s := range stages {
		if s.ChangeVsPreviousDayPct == nil {
			continue
		}
		if *s.ChangeVsPreviousDayPct >= 0 {
			continue
		}
		weight := 1.0 + float64(len(stages)-i)*0.12
		score := math.Abs(*s.ChangeVsPreviousDayPct) * weight
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		if len(stages) == 0 {
			return FunnelWeakestStage{
				Status: "unavailable", Label: "Недостаточно данных", Tone: "neutral",
				Reason: "Нет данных этапов для оценки слабого места.",
			}
		}
		return FunnelWeakestStage{
			Status: "stable", Label: "Явного провала не выявлено", Tone: "neutral",
			Reason: "Ключевые этапы близки к обычному уровню.",
		}
	}
	stage := stages[bestIdx]
	label, metric := weakestStageLabels(stage.Key, stage.Label)
	return FunnelWeakestStage{
		Status:        "available",
		Key:           stage.Key,
		Label:         label,
		Metric:        metric,
		Reason:        "",
		DeltaPrevious: formatSignedPercent(*stage.ChangeVsPreviousDayPct),
		Tone:          toneByDelta(*stage.ChangeVsPreviousDayPct),
	}
}

func weakestStageLabels(key, defaultLabel string) (label, metric string) {
	metric = defaultLabel
	switch strings.TrimSpace(key) {
	case "impressions":
		return "Входящий трафик (показы)", "Показы"
	case "product_views":
		return "Переход в карточку", "Открытия карточки"
	case "add_to_cart":
		return "Переход в корзину", "Добавления в корзину"
	case "orders":
		return "Оформление заказа", "Заказанные единицы"
	case "delivered_units":
		return "Доставка", "Доставленные единицы"
	default:
		return defaultLabel, defaultLabel
	}
}

func toneByDelta(delta float64) string {
	switch {
	case delta <= -10:
		return "negative"
	case delta < 0:
		return "warning"
	case delta >= 10:
		return "positive"
	default:
		return "neutral"
	}
}

func formatSignedPercent(v float64) string {
	sign := "+"
	if v < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s%.1f%%", sign, math.Abs(v))
}
