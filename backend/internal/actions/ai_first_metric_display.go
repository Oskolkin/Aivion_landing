package actions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// metricDisplayLabel returns the UI label for a fact/metric code.
// This is display normalization only — AI selects which facts to show.
func metricDisplayLabel(fact aidatapack.Fact, metricCode string) string {
	code := strings.TrimSpace(metricCode)
	if code == "" {
		code = strings.TrimSpace(fact.Metric)
	}
	lower := strings.ToLower(code)
	switch {
	case lower == "idc_days", strings.Contains(lower, "idc_days"):
		return "Покрытие запасом"
	case lower == "days_without_sales", strings.Contains(lower, "days_without_sales"):
		return "Дней без продаж"
	case lower == "stock_available", strings.Contains(lower, "stock_available"):
		return "Остаток"
	case strings.Contains(lower, "orders") && strings.Contains(lower, "7"):
		return "Заказы за 7 дней"
	case strings.Contains(lower, "orders"):
		return "Заказы"
	case strings.Contains(lower, "revenue") && strings.Contains(lower, "7"):
		return "Выручка за 7 дней"
	case strings.Contains(lower, "revenue"):
		return "Выручка"
	case strings.Contains(lower, "spend") && strings.Contains(lower, "current"):
		return "Расход сегодня"
	case strings.Contains(lower, "spend") && strings.Contains(lower, "7"):
		return "Расход за 7 дней"
	case strings.Contains(lower, "spend"):
		return "Расход"
	case lower == "drr_pct", strings.Contains(lower, "drr"):
		return "ДРР"
	case lower == "roas", strings.Contains(lower, "roas"):
		return "ROAS"
	case lower == "coverage_days", strings.Contains(lower, "coverage"):
		return "Покрытие запасом"
	default:
		if strings.TrimSpace(fact.Window) != "" {
			return fact.Metric + " (" + fact.Window + ")"
		}
		return fact.Metric
	}
}

func metricCodeFromFact(fact aidatapack.Fact) string {
	metric := strings.TrimSpace(fact.Metric)
	window := strings.TrimSpace(fact.Window)
	if metric == "" {
		return fact.FactID
	}
	if window == "" {
		return metric
	}
	return metric + "/" + window
}

func formatAIFirstFactValue(fact aidatapack.Fact) string {
	if v := strings.TrimSpace(fact.FormattedValue); v != "" {
		return v
	}
	unit := strings.TrimSpace(fact.Unit)
	if unit != "" {
		if formatted := aidatapack.FormatValue(unit, fact.Value); formatted != "" {
			return formatted
		}
	}
	return formatRawFactValue(fact.Value)
}

func formatRawFactValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func factToFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func formatComputedValue(op string, value float64, unit string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "count":
		return strconv.FormatInt(int64(value), 10)
	case "range":
		return formatRawFactValue(value)
	default:
		if unit != "" {
			return aidatapack.FormatValue(unit, value)
		}
		return formatRawFactValue(value)
	}
}
