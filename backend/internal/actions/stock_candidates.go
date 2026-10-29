package actions

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	stockReplenishmentLimitation = "Aivion не видит будущие поставки. Перед созданием поставки проверьте фактический план поставок в Ozon."
	noSalesLimitation            = "В выборку попадают только SKU с подтверждённой строкой метрик за дату анализа. Отсутствие данных не считается нулём продаж."
)

func buildStockDomain(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	bundle *SKUMetricsBundle,
	cfg CandidateBuilderConfig,
) (ActionDomainContext, int) {
	domain := ActionDomainContext{
		Domain:     ActionDomainStock,
		Status:     DomainStatusEmpty,
		Candidates: []ActionCandidate{},
	}
	if bundle == nil || bundle.TotalScanned == 0 {
		domain.DataQualityNotes = []string{"Нет SKU-метрик за дату анализа."}
		return domain, 0
	}

	replenishRows := filterStockReplenishmentRows(bundle, cfg)
	noSalesRows := filterNoSalesWithStockRows(bundle)

	candidates := make([]ActionCandidate, 0, 2)
	found := 0

	if len(replenishRows) > 0 {
		cand, err := buildStockReplenishmentCandidate(ctx, repo, sellerAccountID, bundle.MetricDate, replenishRows, cfg)
		if err != nil {
			domain.DataQualityNotes = append(domain.DataQualityNotes, fmt.Sprintf("stock_replenishment: %v", err))
		} else if cand != nil {
			candidates = append(candidates, *cand)
			found++
		}
	}

	if len(noSalesRows) > 0 {
		cand, err := buildNoSalesWithStockCandidate(ctx, repo, sellerAccountID, bundle.MetricDate, noSalesRows, cfg)
		if err != nil {
			domain.DataQualityNotes = append(domain.DataQualityNotes, fmt.Sprintf("no_sales_with_stock: %v", err))
		} else if cand != nil {
			candidates = append(candidates, *cand)
			found++
		}
	}

	if len(candidates) > 0 {
		domain.Status = DomainStatusAvailable
		domain.Candidates = limitCandidates(candidates, cfg.MaxCandidatesPerDomain)
	}
	return domain, found
}

func buildStockReplenishmentCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	rows []StockReplenishmentRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	id := candidateID(ActionTypeStockReplenishment, metricDate, "key_low_cover")
	groupKey := entityGroupKey(id)

	var coverageMin, coverageMax float64
	var revenue7dTotal float64
	var orders7dTotal int64
	criticalCount := 0
	for i, row := range rows {
		if row.DaysOfCover != nil {
			if i == 0 || *row.DaysOfCover < coverageMin {
				coverageMin = *row.DaysOfCover
			}
			if *row.DaysOfCover > coverageMax {
				coverageMax = *row.DaysOfCover
			}
		}
		revenue7dTotal += row.Revenue7d
		orders7dTotal += row.Orders7d
		if row.PriorityCritical {
			criticalCount++
		}
	}

	priority := ActionPriorityImportant
	if criticalCount > 0 {
		priority = ActionPriorityCritical
	}

	aggregates := map[string]any{
		"sku_count":        len(rows),
		"coverage_min":     round2(coverageMin),
		"coverage_max":     round2(coverageMax),
		"revenue_7d_total": round2(revenue7dTotal),
	}
	if bundleShare := stockRevenueSharePct(rows); bundleShare != nil {
		aggregates["revenue_share_pct"] = *bundleShare
	}
	if orders7dTotal > 0 {
		aggregates["avg_daily_orders_7d_total"] = round2(float64(orders7dTotal) / 7)
	}

	facts := []ActionCandidateFact{
		formatCountFact(id, "sku_count", "SKU в группе", strconv.Itoa(len(rows)), "шт"),
		formatCountFact(id, "coverage_range", "Покрытие", fmt.Sprintf("%.1f–%.1f", coverageMin, coverageMax), "дн"),
	}
	if v, ok := aggregates["revenue_share_pct"]; ok {
		facts = append(facts, formatCountFact(id, "revenue_share_pct", "Доля выручки 7д", fmt.Sprintf("%v", v), "%"))
	}
	facts = append(facts, formatCountFact(id, "revenue_7d_total", "Выручка 7д", fmt.Sprintf("%.0f", revenue7dTotal), "₽"))

	examples := make([]ActionCandidateExample, 0, cfg.MaxTopExamplesPerCandidate)
	for i, row := range rows {
		if i >= cfg.MaxTopExamplesPerCandidate {
			break
		}
		metrics := map[string]any{
			"stock":      row.StockAvailable,
			"revenue_7d": round2(row.Revenue7d),
			"orders_7d":  row.Orders7d,
		}
		if row.DaysOfCover != nil {
			metrics["coverage_days"] = round2(*row.DaysOfCover)
		}
		examples = append(examples, skuExample(row.SKUMetricRow, metrics))
	}

	summary := map[string]any{
		"sku_count":        len(rows),
		"coverage_min":     round2(coverageMin),
		"coverage_max":     round2(coverageMax),
		"revenue_7d_total": round2(revenue7dTotal),
	}
	groupRef, err := persistSKUEntityGroup(ctx, repo, sellerAccountID, metricDate, groupKey, stockReplenishmentSKURows(rows), summary)
	if err != nil {
		return nil, err
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainStock,
		Type:           ActionTypeStockReplenishment,
		Category:       ActionCategoryStock,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("%d SKU с низким покрытием остатков (≤%.0f дн)", len(rows), cfg.StockWarningCoverageDays),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    examples,
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAOpenSKUList,
			ActionCTAExportStockReplenishment,
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{stockReplenishmentLimitation},
		SourceRefs:  []string{"daily_sku_metrics"},
	}, nil
}

func buildNoSalesWithStockCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	rows []NoSalesWithStockRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	id := candidateID(ActionTypeNoSalesWithStock, metricDate, "confirmed_zero_orders")
	groupKey := entityGroupKey(id)

	var stockUnitsTotal int64
	var frozenTotal float64
	hasFrozen := false
	for _, row := range rows {
		stockUnitsTotal += int64(row.StockAvailable)
		if row.FrozenStockValue != nil {
			frozenTotal += *row.FrozenStockValue
			hasFrozen = true
		}
	}

	aggregates := map[string]any{
		"sku_count":         len(rows),
		"stock_units_total": stockUnitsTotal,
	}
	if hasFrozen {
		aggregates["frozen_stock_value"] = round2(frozenTotal)
	}
	if top5Share := noSalesTop5Share(rows); top5Share != nil {
		aggregates["top_5_share_pct"] = *top5Share
	}

	facts := []ActionCandidateFact{
		formatCountFact(id, "sku_count", "SKU в группе", strconv.Itoa(len(rows)), "шт"),
		formatCountFact(id, "stock_units_total", "Остаток", strconv.FormatInt(stockUnitsTotal, 10), "шт"),
	}
	if hasFrozen {
		facts = append(facts, formatCountFact(id, "frozen_stock_value", "Замороженный остаток", fmt.Sprintf("%.0f", frozenTotal), "₽"))
	}

	examples := make([]ActionCandidateExample, 0, cfg.MaxTopExamplesPerCandidate)
	for i, row := range rows {
		if i >= cfg.MaxTopExamplesPerCandidate {
			break
		}
		metrics := map[string]any{"stock": row.StockAvailable}
		if row.FrozenStockValue != nil {
			metrics["frozen_stock_value"] = *row.FrozenStockValue
		}
		examples = append(examples, skuExample(row.SKUMetricRow, metrics))
	}

	summary := map[string]any{
		"sku_count":         len(rows),
		"stock_units_total": stockUnitsTotal,
	}
	groupRef, err := persistSKUEntityGroup(ctx, repo, sellerAccountID, metricDate, groupKey, noSalesSKURows(rows), summary)
	if err != nil {
		return nil, err
	}

	priority := ActionPriorityWatch
	if len(rows) >= 5 || stockUnitsTotal >= 50 {
		priority = ActionPriorityImportant
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainStock,
		Type:           ActionTypeNoSalesWithStock,
		Category:       ActionCategoryStock,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("%d SKU с остатком и нулём заказов за дату анализа", len(rows)),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    examples,
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAOpenSKUList,
			ActionCTAExportNoSalesSKUs,
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{noSalesLimitation},
		SourceRefs:  []string{"daily_sku_metrics"},
	}, nil
}

func stockRevenueSharePct(rows []StockReplenishmentRow) *float64 {
	var total float64
	for _, row := range rows {
		if row.RevenueSharePct != nil {
			total += *row.RevenueSharePct
		}
	}
	if total <= 0 {
		return nil
	}
	v := round1(total)
	return &v
}

func noSalesTop5Share(rows []NoSalesWithStockRow) *float64 {
	if len(rows) == 0 {
		return nil
	}
	limit := 5
	if len(rows) < limit {
		limit = len(rows)
	}
	var topUnits int64
	var totalUnits int64
	for _, row := range rows {
		totalUnits += int64(row.StockAvailable)
	}
	for i := 0; i < limit; i++ {
		topUnits += int64(rows[i].StockAvailable)
	}
	if totalUnits <= 0 {
		return nil
	}
	v := round1(float64(topUnits) / float64(totalUnits) * 100)
	return &v
}
