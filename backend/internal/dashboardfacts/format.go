package dashboardfacts

import (
	"fmt"
	"math"
)

func formatMoney(v float64) string {
	return fmt.Sprintf("%.0f ₽", math.Round(v))
}

func formatInt(v int64) string {
	return fmt.Sprintf("%d", v)
}

func formatCTRPercentValue(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}

func formatDRRPercent(pct float64) string {
	return fmt.Sprintf("%.1f%%", pct)
}

func formatCTRDeltaDisplay(deltaPP float64) string {
	sign := ""
	if deltaPP > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.1f п.п. к вчера", sign, deltaPP)
}

func ratioToPercent(ratio *float64) float64 {
	if ratio == nil {
		return 0
	}
	return math.Round(*ratio*10000) / 100
}

func intChangePercent(current, previous int32) float64 {
	if previous == 0 {
		return 0
	}
	return math.Round((float64(current-previous)/float64(previous))*10000) / 100
}
