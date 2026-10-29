package dashboardhome

import (
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

const (
	partialReasonSalesFunnel      = "sales_funnel"
	partialReasonAdvertising      = "advertising"
	partialReasonRiskDistribution = "risk_distribution"
	partialReasonSKUFocusData     = "sku_focus_data"
)

type partialTracker struct {
	reasons []string
}

func (p *partialTracker) mark(reason string) {
	for _, r := range p.reasons {
		if r == reason {
			return
		}
	}
	p.reasons = append(p.reasons, reason)
}

func (p *partialTracker) any() bool {
	return len(p.reasons) > 0
}

func buildPartialDataQualityNotes(reasons []string) []string {
	if len(reasons) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(reasons))
	ordered := make([]string, 0, len(reasons))
	for _, r := range reasons {
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		ordered = append(ordered, r)
	}
	if len(ordered) == 1 {
		switch ordered[0] {
		case partialReasonSalesFunnel:
			return []string{"Воронка продаж пока недоступна для выбранной даты — остальные блоки главной страницы заполнены по доступным метрикам."}
		case partialReasonAdvertising:
			return []string{"Рекламные кампании пока недоступны для выбранной даты — остальные блоки главной страницы заполнены по доступным метрикам."}
		case partialReasonRiskDistribution:
			return []string{"Распределение рисков временно недоступно — остальные блоки главной страницы заполнены по доступным метрикам."}
		case partialReasonSKUFocusData:
			return []string{"Часть SKU-фокуса временно недоступна — остальные блоки главной страницы заполнены по доступным метрикам."}
		}
	}
	return []string{"Часть блоков главной страницы временно недоступна для выбранной даты. Проверьте статус отдельных виджетов ниже."}
}

func buildAccountTrend7D(from, to time.Time, rows []accountMetricDay) []TrendDayPoint {
	byDate := make(map[string]accountMetricDay, len(rows))
	for _, row := range rows {
		byDate[row.Date] = row
	}
	out := make([]TrendDayPoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := TrendDayPoint{Date: key}
		if row, ok := byDate[key]; ok {
			rev := row.Revenue
			ord := row.Orders
			pt.Revenue = &rev
			pt.Orders = &ord
		}
		out = append(out, pt)
	}
	return out
}

type accountMetricDay struct {
	Date    string
	Revenue float64
	Orders  int32
}

func accountOrdersForDate(rows []accountMetricDay, metricDate time.Time) (int32, bool) {
	key := metricDate.Format("2006-01-02")
	for _, row := range rows {
		if row.Date == key {
			return row.Orders, true
		}
	}
	return 0, false
}

func isReplenishmentStockRisk(item analytics.ReplenishmentSKUItem) bool {
	switch item.DepletionRisk {
	case "critical_out_of_stock", "high_depletion_risk":
		return true
	}
	switch item.ReplenishmentPriority {
	case "high", "medium":
		return true
	}
	return false
}

// isDashboardStockRiskSKU is kept as an alias for narrative/risk-driver compatibility.
func isDashboardStockRiskSKU(sku analytics.DashboardSKUMetricsItem) bool {
	return isDashboardStockFocusSKU(sku)
}

func adEfficiencyHasSpend(points []AdEfficiencyPoint) bool {
	for _, p := range points {
		if p.Spend > 0 {
			return true
		}
	}
	return false
}

func aiSummaryContradictsAdEfficiency(headline, executiveSummary string, points []AdEfficiencyPoint) bool {
	if !adEfficiencyHasSpend(points) {
		return false
	}
	text := strings.ToLower(headline + " " + executiveSummary)
	phrases := []string{
		"рекламные кампании не активны",
		"реклама не активна",
		"кампании не активны",
		"нет активных кампаний",
		"рекламные кампании отсутствуют",
		"реклама отсутствует",
	}
	for _, phrase := range phrases {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}
