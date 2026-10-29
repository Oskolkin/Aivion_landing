package dashboardhome

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type accountMetricsSnapshot struct {
	MetricDate string
	Revenue    float64
	Orders     int32
	Cancels    int64
}

func buildWhatChangedContext(
	metricDate time.Time,
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	drivers []RiskDriver,
	previous *accountMetricsSnapshot,
) dashboardNarrativeWhatChangedContext {
	currentDate := metricDate.UTC().Format("2006-01-02")
	previousDate := ""
	if previous != nil && previous.MetricDate != "" {
		previousDate = previous.MetricDate
	} else if !metricDate.IsZero() {
		previousDate = metricDate.UTC().AddDate(0, 0, -1).Format("2006-01-02")
	}

	ctx := dashboardNarrativeWhatChangedContext{
		CurrentMetricDate:  currentDate,
		PreviousMetricDate: previousDate,
		ImprovedFacts:      []whatChangedFact{},
		WorsenedFacts:      []whatChangedFact{},
		NewRiskFacts:       []whatChangedFact{},
		UnchangedRiskFacts: []whatChangedFact{},
		CampaignNames:      []string{},
		ProductNames:       []string{},
		AllowedNumbers:     []string{},
	}

	prevRevenue := previousRevenue(dto, previous)
	prevOrders := previousOrders(dto, previous)
	prevCancels := previousCancels(dto, previous)

	currentRevenue := dto.Account.RevenueToday
	currentOrders := dto.Account.OrdersToday
	currentCancels := dto.Account.CancelsToday
	currentAOV := valueOrZero(dto.Account.AverageOrderValue)
	prevAOV := 0.0
	if prevOrders > 0 && prevRevenue > 0 {
		prevAOV = prevRevenue / float64(prevOrders)
	}

	dayPct := ratioToPercent(dto.Account.RevenueDayDelta.Pct)
	ordersPct := intChangePercent(currentOrders, prevOrders)

	appendFact := func(slice *[]whatChangedFact, fact whatChangedFact) {
		*slice = append(*slice, fact)
		registerWhatChangedFactNumbers(&ctx, fact)
	}

	if dayPct <= -5 {
		appendFact(&ctx.WorsenedFacts, whatChangedFact{
			ID: "revenue-down", Category: "sales", Label: "Выручка", Direction: whatChangedKeyWorsened,
			CurrentValue: formatMoneyRounded(currentRevenue), PreviousValue: formatMoneyRounded(prevRevenue),
			DeltaAbs: formatMoneyRounded(math.Abs(currentRevenue - prevRevenue)),
			DeltaPct: formatPercent(math.Abs(dayPct)),
			Note:     "ниже прошлой сводки",
		})
	} else if dayPct >= 5 {
		appendFact(&ctx.ImprovedFacts, whatChangedFact{
			ID: "revenue-up", Category: "sales", Label: "Выручка", Direction: whatChangedKeyImproved,
			CurrentValue: formatMoneyRounded(currentRevenue), PreviousValue: formatMoneyRounded(prevRevenue),
			DeltaAbs: formatMoneyRounded(math.Abs(currentRevenue - prevRevenue)),
			DeltaPct: formatPercent(dayPct),
			Note:     "выше прошлой сводки",
		})
	}

	if ordersPct <= -8 {
		appendFact(&ctx.WorsenedFacts, whatChangedFact{
			ID: "orders-down", Category: "sales", Label: "Заказы", Direction: whatChangedKeyWorsened,
			CurrentValue: formatInt(int64(currentOrders)), PreviousValue: formatInt(int64(prevOrders)),
			DeltaPct: formatPercent(math.Abs(ordersPct)),
			Note:     "ниже прошлой сводки",
		})
	} else if ordersPct >= 8 {
		appendFact(&ctx.ImprovedFacts, whatChangedFact{
			ID: "orders-up", Category: "sales", Label: "Заказы", Direction: whatChangedKeyImproved,
			CurrentValue: formatInt(int64(currentOrders)), PreviousValue: formatInt(int64(prevOrders)),
			DeltaPct: formatPercent(ordersPct),
			Note:     "выше прошлой сводки",
		})
	}

	if currentAOV > 0 && prevAOV > 0 {
		aovDelta := currentAOV - prevAOV
		if aovDelta <= -50 {
			appendFact(&ctx.WorsenedFacts, whatChangedFact{
				ID: "aov-down", Category: "sales", Label: "Средний чек", Direction: whatChangedKeyWorsened,
				CurrentValue: formatMoneyRounded(currentAOV), PreviousValue: formatMoneyRounded(prevAOV),
				Note: "ниже прошлой сводки",
			})
		} else if aovDelta >= 50 {
			appendFact(&ctx.ImprovedFacts, whatChangedFact{
				ID: "aov-up", Category: "sales", Label: "Средний чек", Direction: whatChangedKeyImproved,
				CurrentValue: formatMoneyRounded(currentAOV), PreviousValue: formatMoneyRounded(prevAOV),
				Note: "выше прошлой сводки",
			})
		}
	}

	currentCancels64 := int64(currentCancels)
	if currentCancels64 > prevCancels && currentCancels64 >= 2 {
		appendFact(&ctx.WorsenedFacts, whatChangedFact{
			ID: "cancels-up", Category: "cancellations", Label: "Отмены", Direction: whatChangedKeyWorsened,
			CurrentValue: formatInt(currentCancels64), PreviousValue: formatInt(prevCancels),
			Note: "выше прошлой сводки",
		})
	} else if prevCancels > currentCancels64 && prevCancels >= 2 {
		appendFact(&ctx.ImprovedFacts, whatChangedFact{
			ID: "cancels-down", Category: "cancellations", Label: "Отмены", Direction: whatChangedKeyImproved,
			CurrentValue: formatInt(currentCancels64), PreviousValue: formatInt(prevCancels),
			Note: "ниже прошлой сводки",
		})
	}

	rate := safePercent(dto.Account.CancellationRate)
	if rate >= 8 {
		appendFact(&ctx.NewRiskFacts, whatChangedFact{
			ID: "cancellation-rate", Category: "cancellations", Label: "Отмены", Direction: whatChangedKeyNewRisks,
			CurrentValue: formatPercent(rate),
			Note:         "доля выросла относительно заказов",
		})
	}

	for _, point := range resp.AIDiagnostics.AdEfficiency.Points {
		name := strings.TrimSpace(point.CampaignName)
		if name != "" {
			ctx.CampaignNames = append(ctx.CampaignNames, name)
		}
		if point.Spend <= 0 {
			continue
		}
		if point.Orders <= 0 && point.Revenue <= 0 {
			appendFact(&ctx.NewRiskFacts, whatChangedFact{
				ID:           fmt.Sprintf("ad-zero-%d", point.CampaignExternalID),
				Category:     "advertising",
				Label:        "Реклама",
				Direction:    whatChangedKeyNewRisks,
				EntityName:   name,
				CurrentValue: formatMoneyRounded(point.Spend),
				Note:         "потратила бюджет без заказов",
			})
		} else if point.Spend >= 3000 && point.Revenue/point.Spend < 0.5 {
			appendFact(&ctx.NewRiskFacts, whatChangedFact{
				ID:           fmt.Sprintf("ad-weak-%d", point.CampaignExternalID),
				Category:     "advertising",
				Label:        "Реклама",
				Direction:    whatChangedKeyNewRisks,
				EntityName:   name,
				CurrentValue: formatMoneyRounded(point.Spend),
				Note:         "низкая отдача по расходу",
			})
		}
	}

	critical, shortage := stockCoverageCounts(resp.AIDiagnostics.StockCoverage.Segments)
	if critical > 0 || shortage > 0 {
		note := "сохраняются критичные риски у значимых товаров"
		if critical > 0 && shortage > 0 {
			note = fmt.Sprintf("сохраняются %d критичных и %d коротких остатков", critical, shortage)
		} else if critical > 0 {
			note = fmt.Sprintf("сохраняются %d критичных остатков", critical)
		}
		appendFact(&ctx.UnchangedRiskFacts, whatChangedFact{
			ID: "stock-risk", Category: "stock", Label: "Остатки", Direction: whatChangedKeyUnchangedRisks,
			CurrentValue: formatInt(int64(critical + shortage)),
			Note:         note,
		})
	}

	var critCount, highCount int64
	for _, group := range resp.AIDiagnostics.RiskDistribution.Groups {
		switch strings.ToLower(group.Severity) {
		case "critical":
			critCount += group.Count
		case "high":
			highCount += group.Count
		}
	}
	if critCount > 0 || highCount > 0 {
		appendFact(&ctx.UnchangedRiskFacts, whatChangedFact{
			ID: "open-risks", Category: "account", Label: "Сигналы", Direction: whatChangedKeyUnchangedRisks,
			CurrentValue: fmt.Sprintf("%d критичных и %d высоких", critCount, highCount),
			Note:         "открытые риски продолжают давить на кабинет",
		})
	}

	for _, driver := range drivers {
		if driver.Severity == "low" || driver.Severity == "attention" {
			continue
		}
		appendFact(&ctx.UnchangedRiskFacts, whatChangedFact{
			ID: driver.ID, Category: driver.Category, Label: driver.CategoryLabel, Direction: whatChangedKeyUnchangedRisks,
			EntityName: driver.Title,
			Note:       "сигнал продолжает сохраняться",
		})
	}

	for _, card := range resp.SKUFocus.StockRisk {
		name := strings.TrimSpace(card.ProductName)
		if name == "" {
			continue
		}
		ctx.ProductNames = append(ctx.ProductNames, name)
	}

	ctx.CampaignNames = compactStringSlice(ctx.CampaignNames)
	ctx.ProductNames = compactStringSlice(ctx.ProductNames)
	ctx.AllowedNumbers = compactStringSlice(ctx.AllowedNumbers)
	return ctx
}

func buildWhatChangedValidationBundle(ctx dashboardNarrativeWhatChangedContext, allowedNames map[string]struct{}) whatChangedValidationBundle {
	numbers := make(map[string]struct{}, len(ctx.AllowedNumbers))
	for _, value := range ctx.AllowedNumbers {
		for _, token := range expandAllowedNumberTokens(value) {
			numbers[token] = struct{}{}
			numbers[normalizeNumberKey(token)] = struct{}{}
		}
	}
	names := make(map[string]struct{}, len(allowedNames)+len(ctx.CampaignNames)+len(ctx.ProductNames))
	for name := range allowedNames {
		names[name] = struct{}{}
	}
	for _, name := range ctx.CampaignNames {
		names[name] = struct{}{}
	}
	for _, name := range ctx.ProductNames {
		names[name] = struct{}{}
	}
	return whatChangedValidationBundle{
		Context:        ctx,
		AllowedNumbers: numbers,
		AllowedNames:   names,
	}
}

func registerWhatChangedFactNumbers(ctx *dashboardNarrativeWhatChangedContext, fact whatChangedFact) {
	for _, value := range []string{fact.CurrentValue, fact.PreviousValue, fact.DeltaAbs, fact.DeltaPct} {
		value = strings.TrimSpace(value)
		if value != "" {
			ctx.AllowedNumbers = append(ctx.AllowedNumbers, value)
		}
	}
}

func previousRevenue(dto analytics.DashboardMetricsDTO, previous *accountMetricsSnapshot) float64 {
	if previous != nil && previous.Revenue > 0 {
		return previous.Revenue
	}
	dayPct := ratioToPercent(dto.Account.RevenueDayDelta.Pct)
	if dayPct == 0 || dto.Account.RevenueToday <= 0 {
		return 0
	}
	denominator := 1 + dayPct/100
	if denominator == 0 {
		return 0
	}
	return dto.Account.RevenueToday / denominator
}

func previousOrders(dto analytics.DashboardMetricsDTO, previous *accountMetricsSnapshot) int32 {
	if previous != nil && previous.Orders > 0 {
		return previous.Orders
	}
	return dto.Account.OrdersYesterday
}

func previousCancels(dto analytics.DashboardMetricsDTO, previous *accountMetricsSnapshot) int64 {
	if previous != nil {
		return previous.Cancels
	}
	if dto.Account.CancelsToday > 0 {
		return int64(math.Max(float64(dto.Account.CancelsToday-1), 0))
	}
	return 0
}

func accountSnapshotFromDailyRow(row dbgen.DailyAccountMetric) accountMetricsSnapshot {
	return accountMetricsSnapshot{
		MetricDate: row.MetricDate.Time.UTC().Format("2006-01-02"),
		Revenue:    numericToFloat(row.Revenue),
		Orders:     row.OrdersCount,
		Cancels:    int64(row.CancelCount),
	}
}

func findPreviousAccountSnapshot(rows []dbgen.DailyAccountMetric, metricDate time.Time) *accountMetricsSnapshot {
	currentKey := metricDate.UTC().Format("2006-01-02")
	var best *accountMetricsSnapshot
	for i := range rows {
		rowDate := rows[i].MetricDate.Time.UTC().Format("2006-01-02")
		if rowDate >= currentKey {
			continue
		}
		snapshot := accountSnapshotFromDailyRow(rows[i])
		if best == nil || snapshot.MetricDate > best.MetricDate {
			copySnapshot := snapshot
			best = &copySnapshot
		}
	}
	return best
}
