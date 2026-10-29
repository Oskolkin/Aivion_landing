package healthpolicy

import (
	"math"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

// HasSufficientCoreData reports whether facts carry enough backend coverage
// to compute a meaningful health_score at all. When false, callers must
// treat health_score as unavailable (nil) rather than defaulting to the
// "no risks found" score — a seller with genuinely no data is not the same
// as a healthy seller with no active risks.
func HasSufficientCoreData(facts dashboardfacts.DashboardFactsV1) bool {
	coverage := facts.DataCoverage
	if coverage.AccountMetricsDays > 0 || coverage.SKUMetricsDays > 0 ||
		coverage.StockSnapshotRows > 0 || coverage.AdMetricsDays > 0 || coverage.FunnelAvailable {
		return true
	}
	if len(facts.SKUFactsIndex.ByOzonProductID) > 0 {
		return true
	}
	account := facts.DashboardMetrics.Account
	if account.RevenueToday > 0 || account.OrdersToday > 0 || account.RevenueLast7Days > 0 {
		return true
	}
	if len(facts.DashboardMetrics.SKUs) > 0 {
		return true
	}
	return false
}

func percentFromRatio(v *float64) float64 {
	if v == nil || !isFinite(*v) {
		return 0
	}
	return math.Round(*v*1000) / 10
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// negativeMagnitude returns abs(v) when v is negative, else 0.
func negativeMagnitude(v float64) float64 {
	if v >= 0 || !isFinite(v) {
		return 0
	}
	return -v
}

func intChangePercent(today int32, yesterday int32) float64 {
	if yesterday <= 0 {
		if today <= 0 {
			return 0
		}
		return 100
	}
	return math.Round(((float64(today)-float64(yesterday))/float64(yesterday))*1000) / 10
}

func severityFromScore(value float64, critical, high, medium float64) string {
	switch {
	case value >= critical:
		return "critical"
	case value >= high:
		return "high"
	case value >= medium:
		return "medium"
	default:
		return "low"
	}
}

func severityFromInts(criticalCount int, shortageCount int) string {
	switch {
	case criticalCount >= 3:
		return "critical"
	case criticalCount > 0 || shortageCount >= 4:
		return "high"
	case shortageCount > 0:
		return "medium"
	default:
		return "low"
	}
}

func topRevenueDeclineSKU(items []analytics.DashboardSKUMetricsItem) *analytics.DashboardSKUMetricsItem {
	var best *analytics.DashboardSKUMetricsItem
	bestLoss := 0.0
	for i := range items {
		item := items[i]
		if item.ContributionToRevenueChange >= 0 {
			continue
		}
		loss := -item.ContributionToRevenueChange
		if best == nil || loss > bestLoss {
			copyItem := item
			best = &copyItem
			bestLoss = loss
		}
	}
	return best
}

func topSKURevenueDropImpact(items []analytics.DashboardSKUMetricsItem) int {
	top := topRevenueDeclineSKU(items)
	if top == nil {
		return 0
	}
	share := 0.0
	if top.ShareOfRevenue != nil {
		share = *top.ShareOfRevenue * 100
	}
	return clampInt(int(math.Round(share/2))+int(math.Abs(top.ContributionToRevenueChange)/5000), 0, 20)
}

func skuFactByOzonID(idx dashboardfacts.SKUFactsIndex, ozonProductID int64) (dashboardfacts.SKUFactRecord, bool) {
	rec, ok := idx.ByOzonProductID[strconv.FormatInt(ozonProductID, 10)]
	return rec, ok
}

func funnelStageByKey(facts dashboardfacts.DashboardFactsV1, key string) (dashboardfacts.FunnelStage, bool) {
	for _, s := range facts.Funnel.Stages {
		if s.Key == key {
			return s, true
		}
	}
	return dashboardfacts.FunnelStage{}, false
}

func adPeriod(facts dashboardfacts.DashboardFactsV1) string {
	if facts.AdEfficiency.Period != nil {
		p := facts.AdEfficiency.Period
		if p.DateFrom != "" && p.DateTo != "" {
			return p.DateFrom + ".." + p.DateTo
		}
		if p.MetricDate != "" {
			return p.MetricDate
		}
	}
	return facts.MetricDate
}

func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func isAvailableStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "available", "ok", "ready":
		return true
	default:
		return false
	}
}
