package dashboardhome

import (
	"fmt"
	"math"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func filterAccountProductAnalyticsForFunnel(rows []dbgen.ProductAnalyticsDaily) []dbgen.ProductAnalyticsDaily {
	out := make([]dbgen.ProductAnalyticsDaily, 0, len(rows))
	for _, row := range rows {
		if row.ScopeType != "account" || row.ScopeID != "" {
			continue
		}
		if row.SourceStatus.Valid && row.SourceStatus.String != "" && row.SourceStatus.String != "available" {
			continue
		}
		out = append(out, row)
	}
	return out
}

func funnelMetricFromProductRow(row *dbgen.ProductAnalyticsDaily, key string) *int64 {
	if row == nil {
		return nil
	}
	switch strings.TrimSpace(key) {
	case "impressions":
		return pgInt8Ptr(row.Impressions)
	case "product_views":
		return pgInt8Ptr(row.ProductViews)
	case "add_to_cart":
		return pgInt8Ptr(row.AddToCart)
	case "orders":
		return pgInt8Ptr(row.OrdersCount)
	case "delivered_units":
		return pgInt8Ptr(row.CompletedOrders)
	default:
		return nil
	}
}

// buildSevenDayAverageComparisons derives seven_day_average rows from account-scoped
// product_analytics_daily history (same 7d window as AI funnel context: metric_date in [asOf-6, asOf]).
// Stage rows: current counts vs simple mean of daily values. Conversion rows: current rate vs 7d mean in п.п.
// Business rows: revenue and average_order_value when computable from history + current row.
func buildSevenDayAverageComparisons(
	stages []FunnelStage,
	history []dbgen.ProductAnalyticsDaily,
	current *dbgen.ProductAnalyticsDaily,
) []FunnelComparisonItem {
	rows := filterAccountProductAnalyticsForFunnel(history)
	if len(rows) == 0 || len(stages) == 0 {
		return nil
	}

	avgByKey := make(map[string]float64)
	countByKey := make(map[string]int)
	for _, row := range rows {
		for _, st := range stages {
			k := strings.TrimSpace(st.Key)
			if k == "" {
				continue
			}
			v := funnelMetricFromProductRow(&row, k)
			if v == nil {
				continue
			}
			avgByKey[k] += float64(*v)
			countByKey[k]++
		}
	}
	for k, sum := range avgByKey {
		if n := countByKey[k]; n > 0 {
			avgByKey[k] = sum / float64(n)
		}
	}

	out := make([]FunnelComparisonItem, 0, len(stages)+6)
	for _, st := range stages {
		k := strings.TrimSpace(st.Key)
		if k == "" || st.Value == nil {
			continue
		}
		avg, ok := avgByKey[k]
		if !ok || countByKey[k] == 0 || avg <= 0 {
			continue
		}
		pct := (float64(*st.Value) - avg) / avg * 100
		rounded := math.Round(pct*100) / 100
		out = append(out, FunnelComparisonItem{
			Key:   k,
			Label: st.Label,
			Value: formatInt(*st.Value),
			Delta: formatSignedPercent(rounded),
			Tone:  toneByDelta(rounded),
		})
	}

	for i := 1; i < len(stages); i++ {
		prev := stages[i-1]
		cur := stages[i]
		if prev.Value == nil || cur.Value == nil || *prev.Value <= 0 || cur.ConversionFromPrevious == nil {
			continue
		}
		avg := averageTransitionConversion(rows, prev.Key, cur.Key)
		if avg == nil {
			continue
		}
		key := fmt.Sprintf("%s_to_%s", prev.Key, cur.Key)
		curConv := *cur.ConversionFromPrevious
		dpp := math.Round((curConv-*avg)*100) / 100
		out = append(out, FunnelComparisonItem{
			Key:   key,
			Label: fmt.Sprintf("%s → %s", transitionStageLabel(prev.Key, prev.Label), transitionStageLabel(cur.Key, cur.Label)),
			Value: fmt.Sprintf("%.1f%%", curConv),
			Delta: formatSignedPercentPP(dpp),
			Tone:  toneByPPDelta(dpp),
		})
	}

	out = append(out, buildSevenDayBusinessComparisons(current, rows)...)

	if len(out) == 0 {
		return nil
	}
	return out
}

func buildSevenDayBusinessComparisons(current *dbgen.ProductAnalyticsDaily, history []dbgen.ProductAnalyticsDaily) []FunnelComparisonItem {
	if current == nil {
		return nil
	}
	curRev := numericToFloat(current.Revenue)
	curOrdPtr := pgInt8Ptr(current.OrdersCount)
	if curOrdPtr == nil || *curOrdPtr <= 0 {
		return nil
	}
	curOrd := float64(*curOrdPtr)
	out := make([]FunnelComparisonItem, 0, 2)

	var sumRev, sumAOV float64
	revDays, aovDays := 0, 0
	for i := range history {
		rev := numericToFloat(history[i].Revenue)
		ordPtr := pgInt8Ptr(history[i].OrdersCount)
		if ordPtr == nil || *ordPtr <= 0 {
			continue
		}
		ord := float64(*ordPtr)
		if rev > 0 {
			sumRev += rev
			revDays++
		}
		dayAOV := rev / ord
		if dayAOV > 0 {
			sumAOV += dayAOV
			aovDays++
		}
	}

	if curRev > 0 && revDays > 0 {
		avgRev := sumRev / float64(revDays)
		if avgRev > 0 {
			pct := math.Round(((curRev-avgRev)/avgRev*100)*100) / 100
			out = append(out, FunnelComparisonItem{
				Key:   "revenue",
				Label: "Выручка",
				Value: formatMoney(curRev),
				Delta: formatSignedPercent(pct),
				Tone:  toneByDelta(pct),
			})
		}
	}

	if curRev > 0 && aovDays > 0 {
		curAOV := curRev / curOrd
		avgAOV := sumAOV / float64(aovDays)
		if avgAOV > 0 {
			pct := math.Round(((curAOV-avgAOV)/avgAOV*100)*100) / 100
			out = append(out, FunnelComparisonItem{
				Key:   "average_order_value",
				Label: "Средний чек",
				Value: formatMoney(curAOV),
				Delta: formatSignedPercent(pct),
				Tone:  toneByDelta(pct),
			})
		}
	}

	return out
}

func findFunnelStageByKey(stages []FunnelStage, key string) *FunnelStage {
	for i := range stages {
		if strings.TrimSpace(stages[i].Key) == key {
			return &stages[i]
		}
	}
	return nil
}

func formatSignedPercentPP(dpp float64) string {
	sign := "+"
	if dpp < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s%.2f п.п.", sign, math.Abs(dpp))
}

func toneByPPDelta(dpp float64) string {
	switch {
	case dpp <= -3:
		return "negative"
	case dpp < 0:
		return "warning"
	case dpp >= 3:
		return "positive"
	default:
		return "neutral"
	}
}
