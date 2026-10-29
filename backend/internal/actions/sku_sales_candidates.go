package actions

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	skuOrdersDropLimitation = "Причина падения требует проверки карточки, остатков, цены, отзывов, доставки и рекламной активности в Ozon."
	keySKURiskLimitation    = "Ключевые SKU определяются по выручке за последние 7 дней. Без себестоимости Aivion не оценивает маржинальность."
)

func buildSKUSalesDomain(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	bundle *SKUMetricsBundle,
	cfg CandidateBuilderConfig,
) (ActionDomainContext, int) {
	domain := ActionDomainContext{
		Domain:     ActionDomainSKUSales,
		Status:     DomainStatusEmpty,
		Candidates: []ActionCandidate{},
	}
	if bundle == nil || bundle.TotalScanned == 0 {
		domain.DataQualityNotes = []string{"Нет SKU-метрик за дату анализа."}
		return domain, 0
	}

	dropRows := filterSKUOrdersDropRows(bundle)
	replenishSet := make(map[int64]bool)
	for _, row := range filterStockReplenishmentRows(bundle, cfg) {
		replenishSet[row.OzonProductID] = true
	}
	dropSet := make(map[int64]bool)
	for _, row := range dropRows {
		dropSet[row.OzonProductID] = true
	}
	keyRiskRows := filterKeySKURiskRows(bundle, cfg, replenishSet, dropSet)

	candidates := make([]ActionCandidate, 0, 2)
	found := 0

	if len(dropRows) > 0 {
		cand, err := buildSKUOrdersDropCandidate(ctx, repo, sellerAccountID, bundle.MetricDate, dropRows, cfg)
		if err != nil {
			domain.DataQualityNotes = append(domain.DataQualityNotes, fmt.Sprintf("sku_orders_drop: %v", err))
		} else if cand != nil {
			candidates = append(candidates, *cand)
			found++
		}
	}

	if len(keyRiskRows) > 0 {
		cand, err := buildKeySKURiskCandidate(ctx, repo, sellerAccountID, bundle.MetricDate, keyRiskRows, cfg)
		if err != nil {
			domain.DataQualityNotes = append(domain.DataQualityNotes, fmt.Sprintf("key_sku_risk: %v", err))
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

func buildSKUOrdersDropCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	rows []SKUOrdersDropRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	id := candidateID(ActionTypeSKUOrdersDrop, metricDate, "top_drop")
	groupKey := entityGroupKey(id)

	var ordersCurrent, ordersPrevious, ordersDeltaTotal int64
	var revenueDeltaTotal float64
	for _, row := range rows {
		ordersCurrent += int64(row.OrdersCount)
		ordersPrevious += int64(row.PrevOrdersCount)
		ordersDeltaTotal += int64(row.OrdersDelta)
		revenueDeltaTotal += row.RevenueDelta
	}

	aggregates := map[string]any{
		"sku_count":             len(rows),
		"orders_current_total":  ordersCurrent,
		"orders_previous_total": ordersPrevious,
		"orders_delta_total":    ordersDeltaTotal,
		"revenue_delta_total":   round2(revenueDeltaTotal),
	}

	facts := []ActionCandidateFact{
		formatCountFact(id, "sku_count", "SKU в группе", strconv.Itoa(len(rows)), "шт"),
		formatCountFact(id, "orders_delta", "Изменение заказов", strconv.FormatInt(ordersDeltaTotal, 10), "шт"),
		formatCountFact(id, "revenue_delta", "Изменение выручки", fmt.Sprintf("%.0f", revenueDeltaTotal), "₽"),
	}

	examples := make([]ActionCandidateExample, 0, cfg.MaxTopExamplesPerCandidate)
	for i, row := range rows {
		if i >= cfg.MaxTopExamplesPerCandidate {
			break
		}
		examples = append(examples, skuExample(row.SKUMetricRow, map[string]any{
			"orders_delta":  row.OrdersDelta,
			"revenue_delta": round2(row.RevenueDelta),
		}))
	}

	summary := map[string]any{
		"sku_count":           len(rows),
		"orders_delta_total":  ordersDeltaTotal,
		"revenue_delta_total": round2(revenueDeltaTotal),
	}
	groupRef, err := persistSKUEntityGroup(ctx, repo, sellerAccountID, metricDate, groupKey, ordersDropSKURows(rows), summary)
	if err != nil {
		return nil, err
	}

	priority := ActionPriorityImportant
	if ordersDeltaTotal <= -10 {
		priority = ActionPriorityCritical
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainSKUSales,
		Type:           ActionTypeSKUOrdersDrop,
		Category:       ActionCategorySales,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("%d SKU с падением заказов день-к-дню", len(rows)),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    examples,
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAOpenSKUList,
			ActionCTAExportSKUOrdersDrop,
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{skuOrdersDropLimitation},
		SourceRefs:  []string{"daily_sku_metrics"},
	}, nil
}

func buildKeySKURiskCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	rows []KeySKURiskRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	id := candidateID(ActionTypeKeySKURisk, metricDate, "multi_risk")
	groupKey := entityGroupKey(id)

	riskTypeSet := make(map[string]bool)
	var revenue7dTotal float64
	var shareTotal float64
	for _, row := range rows {
		revenue7dTotal += row.Revenue7d
		for _, rt := range row.RiskTypes {
			riskTypeSet[rt] = true
		}
		if row.RevenueSharePct != nil {
			shareTotal += *row.RevenueSharePct
		}
	}
	riskTypes := make([]string, 0, len(riskTypeSet))
	for rt := range riskTypeSet {
		riskTypes = append(riskTypes, rt)
	}

	aggregates := map[string]any{
		"sku_count":        len(rows),
		"revenue_7d_total": round2(revenue7dTotal),
		"risk_types":       riskTypes,
	}
	if shareTotal > 0 {
		aggregates["revenue_share_pct"] = round1(shareTotal)
	}

	facts := []ActionCandidateFact{
		formatCountFact(id, "sku_count", "Ключевые SKU", strconv.Itoa(len(rows)), "шт"),
	}
	if shareTotal > 0 {
		facts = append(facts, formatCountFact(id, "revenue_share_pct", "Доля выручки 7д", fmt.Sprintf("%.1f", shareTotal), "%"))
	}
	facts = append(facts, ActionCandidateFact{
		FactID: factID(id, "risk_types"),
		Kind:   "list",
		Label:  "Типы риска",
		Value:  fmt.Sprintf("%v", riskTypes),
	})

	examples := make([]ActionCandidateExample, 0, cfg.MaxTopExamplesPerCandidate)
	for i, row := range rows {
		if i >= cfg.MaxTopExamplesPerCandidate {
			break
		}
		examples = append(examples, skuExample(row.SKUMetricRow, map[string]any{
			"risk_types": row.RiskTypes,
			"risk_score": row.RiskScore,
		}))
	}

	summary := map[string]any{
		"sku_count":        len(rows),
		"risk_types":       riskTypes,
		"revenue_7d_total": round2(revenue7dTotal),
	}
	groupRef, err := persistSKUEntityGroup(ctx, repo, sellerAccountID, metricDate, groupKey, keyRiskSKURows(rows), summary)
	if err != nil {
		return nil, err
	}

	priority := ActionPriorityImportant
	for _, row := range rows {
		if row.RiskScore >= 4 {
			priority = ActionPriorityCritical
			break
		}
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainSKUSales,
		Type:           ActionTypeKeySKURisk,
		Category:       ActionCategorySKU,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("%d ключевых SKU с несколькими сигналами риска", len(rows)),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    examples,
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAOpenSKUList,
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{keySKURiskLimitation},
		SourceRefs:  []string{"daily_sku_metrics"},
	}, nil
}
