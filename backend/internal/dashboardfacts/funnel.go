package dashboardfacts

import (
	"math"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func buildFunnelFacts(snap *analysisdata.AnalysisDataSnapshot, metricDate time.Time) FunnelFacts {
	completedDate := metricDate.AddDate(0, 0, -1)
	previousDate := metricDate.AddDate(0, 0, -2)
	selected := analysisdata.SelectAccountFunnelRows(snap.FunnelRows)
	current := accountFunnelRowForDate(selected, completedDate)
	prev := accountFunnelRowForDate(selected, previousDate)
	facts := FunnelFacts{
		Status:                "not_enough_data",
		CurrentCompletedDate:  analysisdata.FormatDate(completedDate),
		PreviousCompletedDate: analysisdata.FormatDate(previousDate),
		DailySeries:           buildFunnelDailySeries(selected, metricDate.AddDate(0, 0, -6), completedDate),
		Overview7D:            buildOverviewFunnelWeeklyFacts(selected, metricDate),
	}
	if current == nil {
		return facts
	}
	if status, blocked := funnelFactsStatusFromRow(*current); blocked {
		facts.Status = status
		return facts
	}
	if prev != nil {
		if _, blocked := funnelFactsStatusFromRow(*prev); blocked {
			prev = nil
		}
	}
	stages := buildFunnelStages(*current, prev)
	if len(stages) == 0 {
		return facts
	}
	facts.Status = "available"
	facts.Stages = stages
	return facts
}

func buildFunnelDailySeries(rows []dbgen.ProductAnalyticsDaily, from, to time.Time) []FunnelDailyPoint {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	out := make([]FunnelDailyPoint, 0, 6)
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		if _, blocked := funnelFactsStatusFromRow(row); blocked {
			continue
		}
		out = append(out, FunnelDailyPoint{
			Date:           analysisdata.FormatDate(d),
			Impressions:    pgInt8Ptr(row.Impressions),
			ProductViews:   pgInt8Ptr(row.ProductViews),
			AddToCart:      pgInt8Ptr(row.AddToCart),
			OrderedUnits:   pgInt8Ptr(row.OrdersCount),
			DeliveredUnits: pgInt8Ptr(row.CompletedOrders),
		})
	}
	return out
}

func accountFunnelRowForDate(rows []dbgen.ProductAnalyticsDaily, date time.Time) *dbgen.ProductAnalyticsDaily {
	want := analysisdata.FormatDate(date)
	for i := range rows {
		if rows[i].MetricDate.Valid && analysisdata.FormatDate(rows[i].MetricDate.Time) == want {
			return &rows[i]
		}
	}
	return nil
}

func funnelFactsStatusFromRow(row dbgen.ProductAnalyticsDaily) (status string, blocked bool) {
	if row.SourceStatus.Valid && row.SourceStatus.String != "" && row.SourceStatus.String != "available" {
		switch row.SourceStatus.String {
		case "not_enough_data":
			return "not_enough_data", true
		default:
			return "source_not_available", true
		}
	}
	return "available", false
}

func filterAccountProductAnalytics(rows []dbgen.ProductAnalyticsDaily) []dbgen.ProductAnalyticsDaily {
	selected := analysisdata.SelectAccountFunnelRows(rows)
	out := make([]dbgen.ProductAnalyticsDaily, 0, len(selected))
	for _, row := range selected {
		if row.SourceStatus.Valid && row.SourceStatus.String != "" && row.SourceStatus.String != "available" {
			continue
		}
		out = append(out, row)
	}
	return out
}

func buildFunnelStages(row dbgen.ProductAnalyticsDaily, prev *dbgen.ProductAnalyticsDaily) []FunnelStage {
	keys := []string{"impressions", "product_views", "add_to_cart", "orders"}
	labels := []string{"Показы", "Переходы в карточку", "Добавления в корзину", "Заказанные единицы"}
	vals := []*int64{
		pgInt8Ptr(row.Impressions),
		pgInt8Ptr(row.ProductViews),
		pgInt8Ptr(row.AddToCart),
		pgInt8Ptr(row.OrdersCount),
	}
	prevVals := []*int64{
		productAnalyticsMetricPtr(prev, "impressions"),
		productAnalyticsMetricPtr(prev, "product_views"),
		productAnalyticsMetricPtr(prev, "add_to_cart"),
		productAnalyticsMetricPtr(prev, "orders"),
	}
	if row.CompletedOrders.Valid {
		keys = append(keys, "delivered_units")
		labels = append(labels, "Доставленные единицы")
		vals = append(vals, pgInt8Ptr(row.CompletedOrders))
		prevVals = append(prevVals, productAnalyticsMetricPtr(prev, "delivered_units"))
	}
	return buildFinalFunnelStages(keys, labels, vals, prevVals, "completed_day")
}

// buildFinalFunnelStages is the shared stage finalizer.
func buildFinalFunnelStages(
	keys []string,
	labels []string,
	vals []*int64,
	prevVals []*int64,
	periodType string,
) []FunnelStage {
	stages := make([]FunnelStage, 0, len(keys))
	var top *int64
	var prevStage *int64
	var prevPrevStage *int64
	for i := range keys {
		if vals[i] == nil {
			continue
		}
		stage := FunnelStage{
			Key:              keys[i],
			Label:            labels[i],
			Value:            vals[i],
			PreviousDayValue: prevVals[i],
			PeriodType:       periodType,
		}
		if top == nil {
			top = vals[i]
		}
		if prevStage != nil && *prevStage > 0 {
			c := math.Round((float64(*vals[i])/float64(*prevStage))*10000) / 100
			stage.ConversionFromPrevious = &c
		}
		if prevVals[i] != nil && prevPrevStage != nil && *prevPrevStage > 0 {
			pc := math.Round((float64(*prevVals[i])/float64(*prevPrevStage))*10000) / 100
			stage.PreviousConversionFromPrevious = &pc
		}
		if stage.ConversionFromPrevious != nil && stage.PreviousConversionFromPrevious != nil {
			d := math.Round((*stage.ConversionFromPrevious-*stage.PreviousConversionFromPrevious)*100) / 100
			stage.ConversionDeltaPp = &d
		}
		if top != nil && *top > 0 {
			c := math.Round((float64(*vals[i])/float64(*top))*10000) / 100
			stage.ConversionFromTop = &c
		}
		stage.ChangeVsPreviousDayPct = computePercentChange(vals[i], prevVals[i])
		stages = append(stages, stage)
		prevPrevStage = prevVals[i]
		prevStage = vals[i]
	}
	return stages
}

func productAnalyticsMetricPtr(row *dbgen.ProductAnalyticsDaily, key string) *int64 {
	if row == nil {
		return nil
	}
	switch key {
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

func computePercentChange(today, yesterday *int64) *float64 {
	if today == nil || yesterday == nil || *yesterday == 0 {
		return nil
	}
	delta := (float64(*today) - float64(*yesterday)) / float64(*yesterday) * 100
	rounded := math.Round(delta*100) / 100
	return &rounded
}

func pgInt8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	value := v.Int64
	return &value
}

func countObservedFunnelDays(rows []dbgen.ProductAnalyticsDaily, from, to time.Time) int {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	count := 0
	for _, row := range filterAccountProductAnalytics(rows) {
		if !row.MetricDate.Valid {
			continue
		}
		d := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		count++
	}
	return count
}

func accountFunnelRowsForDate(rows []dbgen.ProductAnalyticsDaily, metricDate time.Time) (*dbgen.ProductAnalyticsDaily, *dbgen.ProductAnalyticsDaily) {
	filtered := filterAccountProductAnalytics(rows)
	return accountFunnelRowForDate(filtered, metricDate), accountFunnelRowForDate(filtered, metricDate.AddDate(0, 0, -1))
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
