package pack

import (
	"math"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

// round2 rounds to 2 decimal places (money, most percentages).
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// round4 rounds to 4 decimal places (fine-grained ratios/percentages).
func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

// DeltaAbs returns current-previous rounded to 2dp. Never null.
func DeltaAbs(current, previous float64) float64 {
	return round2(current - previous)
}

// DeltaPct returns the percent delta between current and previous.
//
//	previous == 0 && current == 0 -> 0 (pointer to 0, not null: no change observed)
//	previous == 0 && current != 0 -> nil (division by zero, undefined direction)
//	otherwise                     -> (current-previous)/previous*100, rounded 2dp
func DeltaPct(current, previous float64) *float64 {
	if previous == 0 {
		if current == 0 {
			zero := 0.0
			return &zero
		}
		return nil
	}
	v := round2((current - previous) / previous * 100)
	return &v
}

// ConversionPct returns to/from*100 rounded to 2dp; nil when from<=0.
func ConversionPct(from, to float64) *float64 {
	if from <= 0 {
		return nil
	}
	v := round2(to / from * 100)
	return &v
}

// DRRPct returns spend/revenue*100 rounded to 2dp; nil when revenue<=0.
func DRRPct(spend, revenue float64) *float64 {
	return analytics.CampaignDRRPct(spend, revenue)
}

// CTRPct returns clicks/impressions*100 rounded to 2dp; nil when impressions<=0.
func CTRPct(clicks, impressions int64) *float64 {
	ratio := analytics.CampaignCTR(clicks, impressions)
	if ratio == nil {
		return nil
	}
	v := round2(*ratio * 100)
	return &v
}

// CPC returns spend/clicks rounded to 4dp; nil when clicks<=0.
func CPC(spend float64, clicks int64) *float64 {
	return analytics.CampaignCPC(spend, clicks)
}

// AOV returns revenue/orders rounded to 2dp; nil when orders<=0.
func AOV(revenue float64, orders int64) *float64 {
	if orders <= 0 {
		return nil
	}
	v := round2(revenue / float64(orders))
	return &v
}

// RevenueSharePct returns entityRevenue/totalRevenue*100 rounded to 2dp; nil when totalRevenue<=0.
func RevenueSharePct(entityRevenue, totalRevenue float64) *float64 {
	if totalRevenue <= 0 {
		return nil
	}
	v := round2(entityRevenue / totalRevenue * 100)
	return &v
}

// CoverageDays returns availableStock/avgDailySales rounded to 2dp;
// nil when avgDailySales is nil or <= 0 (division by zero / no velocity signal).
func CoverageDays(availableStock int64, avgDailySales *float64) *float64 {
	if avgDailySales == nil || *avgDailySales <= 0 {
		return nil
	}
	v := round2(float64(availableStock) / *avgDailySales)
	return &v
}

// FormatMoneyCSV formats a required (never-null) money value with 2 decimal places.
func FormatMoneyCSV(v float64) string {
	return strconv.FormatFloat(round2(v), 'f', 2, 64)
}

// FormatMoneyCSVPtr formats a nullable money value with 2 decimal places; empty cell when nil.
func FormatMoneyCSVPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return FormatMoneyCSV(*v)
}

// FormatPctCSV formats a nullable percentage with up to 4 decimal places
// (trailing zeros trimmed); empty cell when nil.
func FormatPctCSV(v *float64) string {
	if v == nil {
		return ""
	}
	return trimTrailingZeros(strconv.FormatFloat(round4(*v), 'f', 4, 64))
}

// FormatFloatCSV formats a nullable generic float with up to 4 decimal places
// (trailing zeros trimmed); empty cell when nil.
func FormatFloatCSV(v *float64) string {
	if v == nil {
		return ""
	}
	return trimTrailingZeros(strconv.FormatFloat(round4(*v), 'f', 4, 64))
}

// FormatIntCSV formats a required (never-null) integer value.
func FormatIntCSV(v int64) string {
	return strconv.FormatInt(v, 10)
}

// FormatIntCSVPtr formats a nullable integer value; empty cell when nil.
func FormatIntCSVPtr(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// FormatBoolCSVPtr formats a nullable boolean as "true"/"false"; empty cell when nil.
func FormatBoolCSVPtr(v *bool) string {
	if v == nil {
		return ""
	}
	if *v {
		return "true"
	}
	return "false"
}

func trimTrailingZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
