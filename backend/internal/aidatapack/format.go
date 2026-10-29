package aidatapack

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	UnitRUB     = "RUB"
	UnitPercent = "percent"
	UnitCount   = "count"
	UnitDays    = "days"
	UnitRatio   = "ratio"
	UnitBoolean = "boolean"
	UnitText    = "text"
	UnitEnum    = "enum"
)

// FormatValue renders a human-readable formatted_value for the given unit.
func FormatValue(unit string, value any) string {
	switch unit {
	case UnitRUB:
		return formatRUB(value)
	case UnitPercent:
		return formatPercent(value)
	case UnitCount:
		return formatCount(value)
	case UnitDays:
		return formatDays(value)
	case UnitRatio:
		return formatRatio(value)
	case UnitBoolean:
		return formatBoolean(value)
	case UnitEnum, UnitText:
		return fmt.Sprint(value)
	default:
		return fmt.Sprint(value)
	}
}

func formatRUB(value any) string {
	f, ok := toFloat64(value)
	if !ok {
		return ""
	}
	if math.Abs(f-math.Round(f)) < 0.005 {
		s := formatThousands(int64(math.Round(f)))
		return s + " ₽"
	}
	parts := strings.Split(fmt.Sprintf("%.2f", f), ".")
	return formatThousandsFromString(parts[0]) + "," + parts[1] + " ₽"
}

func formatThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	return formatThousandsFromString(s)
}

func formatThousandsFromString(s string) string {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, " ")
	if neg {
		out = "-" + out
	}
	return out
}

func formatPercent(value any) string {
	f, ok := toFloat64(value)
	if !ok {
		return ""
	}
	return strings.Replace(fmt.Sprintf("%.2f", f), ".", ",", 1) + "%"
}

func formatCount(value any) string {
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatInt(int64(math.Round(v)), 10)
	default:
		return fmt.Sprint(value)
	}
}

func formatDays(value any) string {
	f, ok := toFloat64(value)
	if !ok {
		return ""
	}
	return strings.Replace(fmt.Sprintf("%.1f", f), ".", ",", 1) + " дн."
}

func formatRatio(value any) string {
	f, ok := toFloat64(value)
	if !ok {
		return ""
	}
	return strings.Replace(fmt.Sprintf("%.2f", f), ".", ",", 1)
}

func formatBoolean(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "да"
		}
		return "нет"
	case string:
		if strings.EqualFold(v, "true") || v == "1" {
			return "да"
		}
		return "нет"
	default:
		return "нет"
	}
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}
