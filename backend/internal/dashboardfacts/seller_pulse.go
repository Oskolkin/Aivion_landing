package dashboardfacts

import (
	"math"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func buildSellerPulseFacts(
	snap *analysisdata.AnalysisDataSnapshot,
	metrics analytics.DashboardMetricsDTO,
	metricDate time.Time,
) SellerPulseFacts {
	acc := metrics.Account
	from := metricDate.AddDate(0, 0, -6)

	accountDays := observedAccountMetricDays(snap.AccountMetrics, from, metricDate)
	trend := buildObservedAccountTrend7D(accountDays)
	revByDate, ordByDate := accountMetricMaps(accountDays)

	sparkRevenue := buildCalendarSparkline(from, metricDate, revByDate)
	sparkOrders := buildCalendarSparkline(from, metricDate, ordByDate)
	sparkAdShare := buildDRRSparkline(from, metricDate, snap.AdMetrics, revByDate)
	sparkAOV := buildAOVSparkline(from, metricDate, revByDate, ordByDate)

	ordersToday := acc.OrdersToday
	ordersYesterday := acc.OrdersYesterday
	revenueToday := acc.RevenueToday
	revenueYesterday := acc.RevenueYesterday
	if ord, ok := accountOrdersForDate(accountDays, metricDate); ok {
		ordersToday = ord
	}
	yesterday := metricDate.AddDate(0, 0, -1)
	if ord, ok := accountOrdersForDate(accountDays, yesterday); ok {
		ordersYesterday = ord
	}
	if rev, ok := accountRevenueForDate(accountDays, metricDate); ok {
		revenueToday = rev
	}
	if rev, ok := accountRevenueForDate(accountDays, yesterday); ok {
		revenueYesterday = rev
	}

	var revenueDeltaPct *float64
	if acc.RevenueDayDelta.Pct != nil {
		v := ratioToPercent(acc.RevenueDayDelta.Pct)
		revenueDeltaPct = &v
	}
	ordersDeltaPct := intChangePercent(ordersToday, ordersYesterday)
	ordersDeltaPctPtr := &ordersDeltaPct

	kpis := []KPICardFact{
		{
			Key: "revenue", Label: "Выручка", Value: formatMoney(revenueToday),
			Delta: &KPICardDelta{
				Abs: acc.RevenueDayDelta.Abs, Pct: revenueDeltaPct, Label: "к вчера",
			},
			Sparkline7D: sparkRevenue, Hint: "За отчётный день",
		},
		{
			Key: "orders", Label: "Заказы", Value: formatInt(int64(ordersToday)),
			Delta: &KPICardDelta{
				Abs: float64(ordersToday - ordersYesterday), Pct: ordersDeltaPctPtr, Label: "к вчера",
			},
			Sparkline7D: sparkOrders, Hint: "За отчётный день",
		},
	}

	if aovKPI, ok := buildAOVKPI(revenueToday, ordersToday, revenueYesterday, ordersYesterday, sparkAOV); ok {
		kpis = append(kpis, aovKPI)
	}
	// Seller Pulse ad card uses current_7d store share (same window as AI summary /
	// advertising account summary). Daily values remain on the sparkline only.
	spend7d := sumAdSpendBetween(snap.AdMetrics, from, metricDate)
	rev7d := sumAccountRevenueBetween(accountDays, from, metricDate)
	if spend7d > 0 && rev7d > 0 {
		pct := spend7d / rev7d * 100
		if canonical := analytics.AccountDRRPct(spend7d, rev7d); canonical != nil {
			pct = *canonical
		}
		kpis = append(kpis, KPICardFact{
			Key: "ad_spend_share_total_revenue", Label: "Рекламные расходы / общая выручка", Value: formatDRRPercent(pct),
			Sparkline7D: sparkAdShare,
			Hint: "Доля рекламных расходов в общей выручке кабинета за последние 7 дней (включая отчётный день).\n\n" +
				"Формула:\nсумма расходов по всем кампаниям за 7 дней ÷ сумма выручки кабинета за 7 дней × 100%.\n\n" +
				"Не путать с атрибутированным ДРР кампаний (расход ÷ выручка, атрибутированная рекламе).\n" +
				"Дневные значения показаны на графике.",
		})
	} else if acc.AccountAdSpendToRevenue != nil {
		pct := *acc.AccountAdSpendToRevenue * 100
		kpis = append(kpis, KPICardFact{
			Key: "ad_spend_share_total_revenue", Label: "Рекламные расходы / общая выручка", Value: formatDRRPercent(pct),
			Sparkline7D: sparkAdShare,
			Hint: "Показывает, какую долю общей выручки кабинета составляют рекламные расходы.\n\n" +
				"Формула:\nрасходы по всем рекламным кампаниям ÷ общая выручка кабинета × 100%.\n\n" +
				"Не путать с ДРР кампаний (расход ÷ атрибутированная рекламная выручка).",
		})
	}

	return SellerPulseFacts{KPIs: kpis, Trend7D: trend}
}

type accountMetricDay struct {
	Date    string
	Revenue float64
	Orders  int32
}

func observedAccountMetricDays(rows []dbgen.DailyAccountMetric, from, to time.Time) []accountMetricDay {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	out := make([]accountMetricDay, 0)
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		out = append(out, accountMetricDay{
			Date:    d.Format("2006-01-02"),
			Revenue: pgNumericToFloat(row.Revenue),
			Orders:  row.OrdersCount,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

func buildObservedAccountTrend7D(days []accountMetricDay) []TrendDayPoint {
	out := make([]TrendDayPoint, 0, len(days))
	for _, day := range days {
		rev := day.Revenue
		ord := day.Orders
		out = append(out, TrendDayPoint{
			Date:    day.Date,
			Revenue: &rev,
			Orders:  &ord,
		})
	}
	return out
}

func accountMetricMaps(days []accountMetricDay) (map[string]float64, map[string]float64) {
	rev := make(map[string]float64, len(days))
	ord := make(map[string]float64, len(days))
	for _, day := range days {
		rev[day.Date] = day.Revenue
		ord[day.Date] = float64(day.Orders)
	}
	return rev, ord
}

func accountOrdersForDate(days []accountMetricDay, metricDate time.Time) (int32, bool) {
	key := metricDate.Format("2006-01-02")
	for _, row := range days {
		if row.Date == key {
			return row.Orders, true
		}
	}
	return 0, false
}

func accountRevenueForDate(days []accountMetricDay, metricDate time.Time) (float64, bool) {
	key := metricDate.Format("2006-01-02")
	for _, row := range days {
		if row.Date == key {
			return row.Revenue, true
		}
	}
	return 0, false
}

// buildAOVKPI returns average order value for the metric day.
// When orders are zero, AOV is omitted (null) — never coerced to 0.
func buildAOVKPI(
	revenueToday float64,
	ordersToday int32,
	revenueYesterday float64,
	ordersYesterday int32,
	sparkline []SparklinePoint,
) (KPICardFact, bool) {
	aovToday := analytics.RatioOrNil(revenueToday, float64(ordersToday))
	if aovToday == nil {
		return KPICardFact{}, false
	}
	kpi := KPICardFact{
		Key:         "average_order_value",
		Label:       "Средний чек",
		Value:       formatMoney(*aovToday),
		Sparkline7D: sparkline,
		Hint:        "Выручка отчётного дня ÷ число заказов. При нуле заказов показатель не рассчитывается.",
	}
	if aovPrev := analytics.RatioOrNil(revenueYesterday, float64(ordersYesterday)); aovPrev != nil && *aovPrev != 0 {
		deltaAbs := *aovToday - *aovPrev
		deltaPct := math.Round((deltaAbs / *aovPrev)*10000) / 100
		kpi.Delta = &KPICardDelta{
			Abs:   deltaAbs,
			Pct:   &deltaPct,
			Label: "к вчера",
		}
	}
	return kpi, true
}

func buildAOVSparkline(from, to time.Time, revByDate, ordByDate map[string]float64) []SparklinePoint {
	aovByDate := make(map[string]float64)
	for date, rev := range revByDate {
		ord, ok := ordByDate[date]
		if !ok || ord <= 0 {
			continue
		}
		if aov := analytics.RatioOrNil(rev, ord); aov != nil {
			aovByDate[date] = *aov
		}
	}
	return buildCalendarSparkline(from, to, aovByDate)
}

func buildCalendarSparkline(from, to time.Time, valueByDate map[string]float64) []SparklinePoint {
	points := make([]SparklinePoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if v, ok := valueByDate[key]; ok {
			pt.HasData = true
			vv := v
			pt.Value = &vv
		}
		points = append(points, pt)
	}
	return points
}

func buildDRRSparkline(from, to time.Time, adRows []dbgen.AdMetricsDaily, revenueByDate map[string]float64) []SparklinePoint {
	spendByDate := make(map[string]float64)
	for _, row := range adRows {
		if !row.MetricDate.Valid {
			continue
		}
		key := row.MetricDate.Time.UTC().Format("2006-01-02")
		spendByDate[key] += pgNumericToFloat(row.Spend)
	}
	series := analytics.BuildAccountDRRDailySeries(from, to, spendByDate, revenueByDate)
	ratioByDate := make(map[string]float64)
	for _, day := range series {
		if day.HasData && day.DRRPct != nil {
			ratioByDate[day.Date] = *day.DRRPct
		}
	}
	return buildCalendarSparkline(from, to, ratioByDate)
}

func sumAdSpendOnDate(adRows []dbgen.AdMetricsDaily, metricDate time.Time) float64 {
	key := metricDate.UTC().Format("2006-01-02")
	var total float64
	for _, row := range adRows {
		if !row.MetricDate.Valid {
			continue
		}
		if row.MetricDate.Time.UTC().Format("2006-01-02") != key {
			continue
		}
		total += pgNumericToFloat(row.Spend)
	}
	return total
}

func sumAdSpendBetween(adRows []dbgen.AdMetricsDaily, from, to time.Time) float64 {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	var total float64
	for _, row := range adRows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		total += pgNumericToFloat(row.Spend)
	}
	return total
}

func sumAccountRevenueBetween(days []accountMetricDay, from, to time.Time) float64 {
	fromKey := analysisdata.DateOnlyUTC(from).Format("2006-01-02")
	toKey := analysisdata.DateOnlyUTC(to).Format("2006-01-02")
	var total float64
	for _, day := range days {
		if day.Date < fromKey || day.Date > toKey {
			continue
		}
		total += day.Revenue
	}
	return total
}

type dailyRatioPoint struct {
	value   float64
	hasData bool
}

func buildCTRSparkline(from, to time.Time, funnelRows []dbgen.ProductAnalyticsDaily) []SparklinePoint {
	ratioByDate := make(map[string]dailyRatioPoint)
	for _, row := range filterAccountProductAnalytics(funnelRows) {
		impressions := pgInt8Value(row.Impressions)
		views := pgInt8Value(row.ProductViews)
		ctr, ok := computeAccountCTRPercent(impressions, views)
		if !ok || ctr == nil || !row.MetricDate.Valid {
			continue
		}
		key := row.MetricDate.Time.UTC().Format("2006-01-02")
		ratioByDate[key] = dailyRatioPoint{value: *ctr, hasData: true}
	}
	points := make([]SparklinePoint, 0, 8)
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if row, ok := ratioByDate[key]; ok && row.hasData {
			v := row.value
			pt.HasData = true
			pt.Value = &v
		}
		points = append(points, pt)
	}
	return points
}

func buildCTRKPIFromSnapshot(funnelRows []dbgen.ProductAnalyticsDaily, metricDate time.Time, sparkline []SparklinePoint) (KPICardFact, bool) {
	current, prev := accountFunnelRowsForDate(funnelRows, metricDate)
	if current == nil {
		return KPICardFact{}, false
	}
	impressions := pgInt8Value(current.Impressions)
	views := pgInt8Value(current.ProductViews)
	ctrToday, ok := computeAccountCTRPercent(impressions, views)
	if !ok || ctrToday == nil {
		return KPICardFact{}, false
	}
	kpi := KPICardFact{
		Key:         "ctr",
		Label:       "CTR всех карточек товаров кабинета",
		Value:       formatCTRPercentValue(*ctrToday),
		Sparkline7D: sparkline,
		Hint: "Показывает долю переходов в карточки товаров от общего количества их показов по всему кабинету.\n\n" +
			"Формула:\nклики по всем карточкам ÷ показы всех карточек × 100%.",
	}
	if prev != nil {
		prevImp := pgInt8Value(prev.Impressions)
		prevViews := pgInt8Value(prev.ProductViews)
		if ctrPrev, prevOK := computeAccountCTRPercent(prevImp, prevViews); prevOK && ctrPrev != nil {
			deltaPP := math.Round((*ctrToday-*ctrPrev)*100) / 100
			kpi.Delta = &KPICardDelta{
				Abs:     deltaPP,
				Label:   "к вчера",
				Unit:    "percentage_point",
				Display: formatCTRDeltaDisplay(deltaPP),
			}
		}
	}
	return kpi, true
}

func computeAccountCTRPercent(impressions, productViews int64) (*float64, bool) {
	if impressions <= 0 {
		return nil, false
	}
	v := math.Round((float64(productViews)/float64(impressions))*10000) / 100
	return &v, true
}

func pgInt8Value(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func pgTextString(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func pgNumericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return math.Round(v.Float64*100) / 100
}

func pgNumericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	out := math.Round(v.Float64*100) / 100
	return &out
}
