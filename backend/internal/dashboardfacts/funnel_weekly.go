package dashboardfacts

import (
	"math"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// buildOverviewFunnelWeeklyFacts preserves the Overview-only WoW contract.
// Funnel detail facts are built separately from completed D-1/D-2 rows.
func buildOverviewFunnelWeeklyFacts(rows []dbgen.ProductAnalyticsDaily, metricDate time.Time) FunnelWeeklyFacts {
	current := sumFunnelWindow(rows, metricDate.AddDate(0, 0, -6), metricDate)
	previous := sumFunnelWindow(rows, metricDate.AddDate(0, 0, -13), metricDate.AddDate(0, 0, -7))
	if !current.hasAny {
		return FunnelWeeklyFacts{Status: "not_enough_data"}
	}
	stages := buildWeeklyFunnelStages(current, previous)
	if len(stages) == 0 {
		return FunnelWeeklyFacts{Status: "not_enough_data"}
	}
	return FunnelWeeklyFacts{Status: "available", Stages: stages}
}

type funnelWindowAgg struct {
	impressions     int64
	productViews    int64
	addToCart       int64
	orders          int64
	deliveredUnits  int64
	hasImpressions  bool
	hasProductViews bool
	hasAddToCart    bool
	hasOrders       bool
	hasDelivered    bool
	hasAny          bool
}

func sumFunnelWindow(rows []dbgen.ProductAnalyticsDaily, from, to time.Time) funnelWindowAgg {
	fromDate := analysisdata.DateOnlyUTC(from)
	toDate := analysisdata.DateOnlyUTC(to)
	var out funnelWindowAgg
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		if _, blocked := funnelFactsStatusFromRow(row); blocked {
			continue
		}
		date := analysisdata.DateOnlyUTC(row.MetricDate.Time)
		if date.Before(fromDate) || date.After(toDate) {
			continue
		}
		out.hasAny = true
		if row.Impressions.Valid {
			out.impressions += row.Impressions.Int64
			out.hasImpressions = true
		}
		if row.ProductViews.Valid {
			out.productViews += row.ProductViews.Int64
			out.hasProductViews = true
		}
		if row.AddToCart.Valid {
			out.addToCart += row.AddToCart.Int64
			out.hasAddToCart = true
		}
		if row.OrdersCount.Valid {
			out.orders += row.OrdersCount.Int64
			out.hasOrders = true
		}
		if row.CompletedOrders.Valid {
			out.deliveredUnits += row.CompletedOrders.Int64
			out.hasDelivered = true
		}
	}
	return out
}

func buildWeeklyFunnelStages(current, previous funnelWindowAgg) []FunnelWeeklyStage {
	ptr := func(ok bool, value int64) *int64 {
		if !ok {
			return nil
		}
		return &value
	}
	keys := []string{"impressions", "product_views", "add_to_cart", "orders"}
	labels := []string{"Показы", "Переходы в карточку", "Добавления в корзину", "Заказанные единицы"}
	values := []*int64{
		ptr(current.hasImpressions, current.impressions),
		ptr(current.hasProductViews, current.productViews),
		ptr(current.hasAddToCart, current.addToCart),
		ptr(current.hasOrders, current.orders),
	}
	previousValues := []*int64{
		ptr(previous.hasImpressions, previous.impressions),
		ptr(previous.hasProductViews, previous.productViews),
		ptr(previous.hasAddToCart, previous.addToCart),
		ptr(previous.hasOrders, previous.orders),
	}
	if current.hasDelivered {
		keys = append(keys, "delivered_units")
		labels = append(labels, "Доставленные единицы")
		values = append(values, ptr(true, current.deliveredUnits))
		previousValues = append(previousValues, ptr(previous.hasDelivered, previous.deliveredUnits))
	}

	stages := make([]FunnelWeeklyStage, 0, len(keys))
	var currentPreviousStage *int64
	var previousPreviousStage *int64
	for index := range keys {
		if values[index] == nil {
			continue
		}
		stage := FunnelWeeklyStage{
			Key:                       keys[index],
			Label:                     labels[index],
			Value:                     values[index],
			PreviousPeriodValue:       previousValues[index],
			ChangeVsPreviousPeriodPct: computePercentChange(values[index], previousValues[index]),
		}
		if currentPreviousStage != nil && *currentPreviousStage > 0 {
			value := roundFunnelConversion(*values[index], *currentPreviousStage)
			stage.ConversionFromPrevious = &value
		}
		if previousValues[index] != nil && previousPreviousStage != nil && *previousPreviousStage > 0 {
			value := roundFunnelConversion(*previousValues[index], *previousPreviousStage)
			stage.PreviousConversionFromPrevious = &value
		}
		if stage.ConversionFromPrevious != nil && stage.PreviousConversionFromPrevious != nil {
			value := math.Round((*stage.ConversionFromPrevious-*stage.PreviousConversionFromPrevious)*100) / 100
			stage.ConversionDeltaPp = &value
		}
		stages = append(stages, stage)
		currentPreviousStage = values[index]
		previousPreviousStage = previousValues[index]
	}
	return stages
}

func roundFunnelConversion(value, previous int64) float64 {
	return math.Round((float64(value)/float64(previous))*10000) / 100
}
