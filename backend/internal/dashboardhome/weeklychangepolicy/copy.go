package weeklychangepolicy

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

func buildCopy(c ChangeCandidate, cond conditionSnapshot) (title, description string) {
	period := c.CurrentPeriod
	if period == "" {
		period = "текущий период"
	}
	names := namedEntities(c.EntityNames, 2)

	switch c.MetricCode {
	case MetricRevenue7d:
		if c.ChangeType == ChangeImproved {
			title = fmt.Sprintf("Выручка выросла на %s", formatPctAbs(c.RelativeDeltaPct))
			description = fmt.Sprintf(
				"За %s выручка составила %s против %s в предыдущем периоде (%s). Рост продаж сохраняется и формирует основную положительную динамику кабинета.",
				period, c.CurrentValueText, nullText(c.PreviousValueText, "прошлого уровня"), deltaClause(c),
			)
		} else {
			title = fmt.Sprintf("Выручка снизилась на %s", formatPctAbs(c.RelativeDeltaPct))
			description = fmt.Sprintf(
				"За %s выручка составила %s против %s ранее. Снижение продаж — главный негативный сдвиг недели.",
				period, c.CurrentValueText, nullText(c.PreviousValueText, "прошлого уровня"),
			)
		}
	case MetricOrders7d:
		if c.ChangeType == ChangeImproved {
			title = fmt.Sprintf("Заказы выросли на %s", formatPctAbs(c.RelativeDeltaPct))
			description = fmt.Sprintf(
				"Количество заказов за %s выросло до %s с %s. Операционный объём продаж поддерживает рост выручки.",
				period, c.CurrentValueText, nullText(c.PreviousValueText, "прошлого уровня"),
			)
		} else {
			title = fmt.Sprintf("Заказы снизились на %s", formatPctAbs(c.RelativeDeltaPct))
			description = fmt.Sprintf(
				"Количество заказов за %s снизилось до %s с %s. Падение спроса требует проверки ассортимента и трафика.",
				period, c.CurrentValueText, nullText(c.PreviousValueText, "прошлого уровня"),
			)
		}
	case MetricStockoutSKUCount:
		n := int(math.Round(cond.value))
		title = fmt.Sprintf("Без остатка осталось %s", pluralSKU(n))
		description = fmt.Sprintf(
			"Количество товаров с нулевым остатком и недавними продажами %s. %s Сохранение спроса по этим позициям может привести к недополученным заказам. В первую очередь нужно проверить товары с недавними продажами и ближайшие поставки.",
			wasBecame(c), impactSentence(c),
		)
	case MetricCriticalSKUCount, MetricShortageSKUCount:
		n := int(math.Round(cond.value))
		title = fmt.Sprintf("Дефицит по %s ключевым SKU", formatInt(n))
		description = fmt.Sprintf(
			"Ключевые товары с коротким запасом %s. %s %s",
			wasBecame(c), impactSentence(c), namesClause(names, "В зоне риска"),
		)
	case MetricNoSalesWithStock:
		n := int(math.Round(cond.value))
		title = fmt.Sprintf("%s без продаж при наличии остатка", pluralSKU(n))
		description = fmt.Sprintf(
			"Товары с остатком, но без заказов за неделю, %s. Замороженные позиции продолжают занимать складской ресурс без вклада в выручку. %s",
			wasBecame(c), namesClause(names, "Среди них"),
		)
	case MetricSurplusSKUCount:
		n := int(math.Round(cond.value))
		title = fmt.Sprintf("Избыточные остатки по %s", pluralSKU(n))
		description = fmt.Sprintf(
			"Медленно оборачиваемые или избыточные остатки %s. %s Это ограничивает оборачиваемость капитала.",
			wasBecame(c), impactSentence(c),
		)
	case MetricHighDRRCampaigns:
		n := int(math.Round(cond.value))
		title = fmt.Sprintf("%s превысили ДРР 80%%", pluralCampaigns(n))
		description = fmt.Sprintf(
			"Количество проблемных кампаний %s. %s %s Необходимо проверить ставки, аудитории и рекламируемые товары.",
			wasBecame(c), impactSentence(c), namesClause(names, "Основной вклад дали кампании"),
		)
	case MetricSpendWithoutOrders:
		title = fmt.Sprintf("Кампания потратила %s без заказов", c.CurrentValueText)
		if c.EntityCount > 1 {
			title = fmt.Sprintf("%s потратили %s без заказов", pluralCampaigns(c.EntityCount), c.CurrentValueText)
		}
		prevNote := "В предыдущем периоде такой проблемы не было."
		if c.ChangeType == ChangeActiveProblem {
			prevNote = "Проблема сохраняется относительно предыдущего отчёта."
		} else if c.ChangeType == ChangeWorsened {
			prevNote = fmt.Sprintf("Расход без заказов вырос с %s.", nullText(c.PreviousValueText, "более низкого уровня"))
		}
		description = fmt.Sprintf(
			"%s израсходовала %s за %s и не получила ни одного заказа. %s Необходимо проверить ставки, запросы, рекламируемые товары и корректность выбранной стратегии.",
			namesClause(names, "Кампания"), c.CurrentValueText, period, prevNote,
		)
		if c.EntityCount > 1 {
			description = fmt.Sprintf(
				"%s суммарно израсходовали %s за %s без заказов. %s",
				namesClause(names, "Кампании"), c.CurrentValueText, period, prevNote,
			)
		}
	case MetricAdShareOfRevenue:
		title = "Рекламная нагрузка кабинета стабильна"
		if c.ChangeType == ChangeWorsened {
			title = fmt.Sprintf("Рекламная доля выросла до %s", c.CurrentValueText)
		} else if c.ChangeType == ChangeImproved && c.RelativeDeltaPct != nil && *c.RelativeDeltaPct < -1 {
			title = fmt.Sprintf("Рекламная доля снизилась до %s", c.CurrentValueText)
		}
		description = fmt.Sprintf(
			"Доля рекламных расходов к выручке за %s составила %s (было %s). %s",
			period, c.CurrentValueText, nullText(c.PreviousValueText, "сопоставимый уровень"),
			"Это характеризует рекламную нагрузку на уровне всего кабинета, а не отдельных кампаний.",
		)
	case MetricFunnelViewToCart, MetricFunnelCartToOrder:
		label := "конверсия воронки"
		if c.MetricCode == MetricFunnelViewToCart {
			label = "конверсия в корзину"
		} else {
			label = "конверсия в заказ"
		}
		if c.ChangeType == ChangeImproved {
			title = fmt.Sprintf("Улучшилась %s", label)
		} else {
			title = fmt.Sprintf("Снизилась %s", label)
		}
		r := []rune(label)
		lead := label
		if len(r) > 0 {
			lead = strings.ToUpper(string(r[0])) + string(r[1:])
		}
		description = fmt.Sprintf(
			"%s за %s составила %s против %s ранее (%s). Изменение влияет на эффективность трафика.",
			lead, period, c.CurrentValueText, nullText(c.PreviousValueText, "прошлого уровня"), deltaClause(c),
		)
	default:
		title = cond.metricCode
		description = fmt.Sprintf("Показатель изменился до %s за %s.", c.CurrentValueText, period)
	}

	title = trimRunes(title, 90)
	return title, description
}

func defaultActionLinkLabel(c ChangeCandidate) string {
	switch c.ChangeType {
	case ChangeImproved:
		return ""
	}
	switch c.EntityType {
	case "campaign":
		if c.EntityCount == 1 {
			return "Проверить кампанию"
		}
		return fmt.Sprintf("Проверить %s", pluralCampaigns(c.EntityCount))
	case "sku":
		if c.EntityCount > 0 {
			return fmt.Sprintf("Посмотреть %s", pluralSKU(c.EntityCount))
		}
		return "Перейти к проверке"
	default:
		return "Открыть действие"
	}
}

func formatValue(v float64, unit string, cond conditionSnapshot) string {
	switch unit {
	case "money":
		return formatMoney(v)
	case "count":
		if cond.entityType == "campaign" {
			return pluralCampaigns(int(math.Round(v)))
		}
		if cond.entityType == "sku" {
			return pluralSKU(int(math.Round(v)))
		}
		return formatInt(int(math.Round(v)))
	case "pct", "pp":
		return formatPct(v)
	default:
		return fmt.Sprintf("%.1f", v)
	}
}

func formatDelta(cur, prev float64, cond conditionSnapshot) string {
	d := cur - prev
	switch cond.unit {
	case "money":
		sign := "+"
		if d < 0 {
			sign = "−"
			d = -d
		}
		return sign + formatMoney(d)
	case "count":
		return fmt.Sprintf("%s → %s", formatInt(int(math.Round(prev))), formatInt(int(math.Round(cur))))
	case "pct", "pp":
		return fmt.Sprintf("%s → %s", formatPct(prev), formatPct(cur))
	default:
		return fmt.Sprintf("%.1f → %.1f", prev, cur)
	}
}

func formatImpact(cond conditionSnapshot) string {
	if cond.financialImpact <= 0 {
		return ""
	}
	switch cond.metricCode {
	case MetricStockoutSKUCount, MetricCriticalSKUCount, MetricShortageSKUCount:
		return fmt.Sprintf("На эти позиции приходилось %s выручки за последние 7 дней.", formatMoney(cond.financialImpact))
	case MetricHighDRRCampaigns:
		return fmt.Sprintf("На них пришлось %s расходов за неделю.", formatMoney(cond.financialImpact))
	case MetricSpendWithoutOrders:
		return fmt.Sprintf("Суммарный расход без заказов — %s.", formatMoney(cond.financialImpact))
	case MetricSurplusSKUCount:
		return fmt.Sprintf("Затронутый объём остатка — около %s ед.", formatInt(int(math.Round(cond.financialImpact))))
	case MetricRevenue7d:
		return fmt.Sprintf("Абсолютное изменение выручки — %s.", formatMoney(cond.financialImpact))
	default:
		return fmt.Sprintf("Оценка влияния — %s.", formatMoney(cond.financialImpact))
	}
}

func wasBecame(c ChangeCandidate) string {
	if c.PreviousValueText != "" && c.CurrentValueText != "" {
		return fmt.Sprintf("изменилось с %s до %s", c.PreviousValueText, c.CurrentValueText)
	}
	if c.CurrentValueText != "" {
		return "составляет " + c.CurrentValueText
	}
	return "сохраняется"
}

func impactSentence(c ChangeCandidate) string {
	if strings.TrimSpace(c.ImpactText) == "" {
		return ""
	}
	return strings.TrimSpace(c.ImpactText)
}

func namesClause(names []string, lead string) string {
	if len(names) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(names))
	for _, n := range names {
		quoted = append(quoted, "«"+n+"»")
	}
	return lead + " " + strings.Join(quoted, " и ") + "."
}

func namedEntities(names []string, limit int) []string {
	if limit <= 0 || len(names) <= limit {
		return names
	}
	return names[:limit]
}

func deltaClause(c ChangeCandidate) string {
	if c.RelativeDeltaPct != nil {
		return "изменение " + formatPctSigned(*c.RelativeDeltaPct)
	}
	if c.DeltaText != "" {
		return c.DeltaText
	}
	return "есть подтверждённое изменение"
}

func nullText(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func formatMoney(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	n := int64(math.Round(v))
	s := formatInt(int(n)) + " ₽"
	if neg {
		return "−" + s
	}
	return s
}

func formatInt(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconvFormatInt(int64(n))
	// thousands thin spaces
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteRune('\u00a0')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if neg {
		return "−" + out
	}
	return out
}

func strconvFormatInt(n int64) string {
	return fmt.Sprintf("%d", n)
}

func formatPct(v float64) string {
	return strings.ReplaceAll(fmt.Sprintf("%.1f%%", v), ".", ",")
}

func formatPctAbs(p *float64) string {
	if p == nil {
		return "—"
	}
	return formatPct(math.Abs(*p))
}

func formatPctSigned(v float64) string {
	sign := "+"
	if v < 0 {
		sign = "−"
		v = -v
	}
	return sign + formatPct(v)
}

func pluralSKU(n int) string {
	return fmt.Sprintf("%s %s", formatInt(n), russianPlural(n, "SKU", "SKU", "SKU"))
}

func pluralCampaigns(n int) string {
	return fmt.Sprintf("%s %s", formatInt(n), russianPlural(n, "кампания", "кампании", "кампаний"))
}

func russianPlural(n int, one, few, many string) string {
	n = int(math.Abs(float64(n)))
	n100 := n % 100
	n10 := n % 10
	if n100 >= 11 && n100 <= 14 {
		return many
	}
	switch n10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	default:
		return many
	}
}

func trimRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max-1]) + "…"
}

// StripNumericCampaignIDs removes bare numeric campaign IDs from seller-facing copy.
func StripNumericCampaignIDs(text string, allowedNames []string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	var b strings.Builder
	runes := []rune(text)
	for i := 0; i < len(runes); {
		if unicode.IsDigit(runes[i]) {
			j := i
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			token := string(runes[i:j])
			// Long numeric tokens look like Ozon campaign/product external IDs.
			if len(token) >= 9 {
				replacement := "кампания без названия"
				if len(allowedNames) > 0 {
					replacement = "«" + allowedNames[0] + "»"
				}
				// Avoid double spaces when replacing.
				b.WriteString(replacement)
				i = j
				continue
			}
			b.WriteString(token)
			i = j
			continue
		}
		b.WriteRune(runes[i])
		i++
	}
	out := b.String()
	out = strings.Join(strings.Fields(out), " ")
	out = strings.ReplaceAll(out, " ,", ",")
	out = strings.ReplaceAll(out, " .", ".")
	return out
}

// ContainsNumericCampaignID reports whether text includes a long bare numeric ID.
func ContainsNumericCampaignID(text string) bool {
	runes := []rune(text)
	for i := 0; i < len(runes); {
		if unicode.IsDigit(runes[i]) {
			j := i
			for j < len(runes) && unicode.IsDigit(runes[j]) {
				j++
			}
			if j-i >= 9 {
				return true
			}
			i = j
			continue
		}
		i++
	}
	return false
}

func looksLikeNumericID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 9 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
