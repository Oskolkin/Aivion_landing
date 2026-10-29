package actions

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	adHighDRRLimitation       = "Рекламная аналитика в MVP доступна на уровне кампаний, без точного SKU-level рекламного P&L."
	adSpendNoOrdersLimitation = "Aivion показывает проблему на уровне кампаний. Проверку товаров, ставок и поисковых фраз нужно выполнить в Ozon Performance."
)

func buildAdvertisingDomain(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	cfg CandidateBuilderConfig,
) (ActionDomainContext, int, int) {
	domain := ActionDomainContext{
		Domain:     ActionDomainAdvertising,
		Status:     DomainStatusEmpty,
		Candidates: []ActionCandidate{},
	}
	campaignCount, err := repo.CountAdCampaigns(ctx, sellerAccountID)
	if err != nil {
		domain.Status = DomainStatusError
		domain.DataQualityNotes = []string{fmt.Sprintf("Реклама: ошибка загрузки: %v", err)}
		return domain, 0, 0
	}
	if campaignCount == 0 {
		domain.Status = DomainStatusSourceNotAvailable
		domain.DataQualityNotes = []string{"Рекламные кампании не подключены или не синхронизированы."}
		return domain, 0, 0
	}

	rows, err := repo.ListAdCampaignMetrics7d(ctx, sellerAccountID, metricDate)
	if err != nil {
		domain.Status = DomainStatusError
		domain.DataQualityNotes = []string{fmt.Sprintf("Реклама: ошибка метрик: %v", err)}
		return domain, 0, 0
	}

	campaignsScanned := countCampaignsWithMetricData(rows)
	hasAnyMetricData := campaignsScanned > 0
	if !hasAnyMetricData {
		domain.Status = DomainStatusSourceNotAvailable
		domain.DataQualityNotes = []string{"Нет рекламных метрик за последние 7 дней. Отсутствие данных не считается нулевым расходом."}
		return domain, 0, 0
	}

	highDRR := filterHighDRRCampaigns(rows, cfg)
	noOrders := filterNoOrderCampaigns(rows, cfg)

	candidates := make([]ActionCandidate, 0, 2)
	found := 0

	if len(highDRR) > 0 {
		cand, err := buildAdHighDRRCandidate(ctx, repo, sellerAccountID, metricDate, highDRR, cfg)
		if err != nil {
			domain.DataQualityNotes = append(domain.DataQualityNotes, fmt.Sprintf("ad_high_drr: %v", err))
		} else if cand != nil {
			candidates = append(candidates, *cand)
			found++
		}
	}

	if len(noOrders) > 0 {
		cand, err := buildAdSpendNoOrdersCandidate(ctx, repo, sellerAccountID, metricDate, noOrders, cfg)
		if err != nil {
			domain.DataQualityNotes = append(domain.DataQualityNotes, fmt.Sprintf("ad_spend_no_orders: %v", err))
		} else if cand != nil {
			candidates = append(candidates, *cand)
			found++
		}
	}

	if len(candidates) > 0 {
		domain.Status = DomainStatusAvailable
		domain.Candidates = limitCandidates(candidates, cfg.MaxCandidatesPerDomain)
	} else {
		domain.Status = DomainStatusEmpty
	}
	return domain, found, campaignsScanned
}

// countCampaignsWithMetricData returns campaigns that have ad_metrics_daily rows in the 7d window.
func countCampaignsWithMetricData(rows []AdCampaignMetricRow) int {
	n := 0
	for _, row := range rows {
		if row.HasMetricData {
			n++
		}
	}
	return n
}

func filterHighDRRCampaigns(rows []AdCampaignMetricRow, cfg CandidateBuilderConfig) []AdCampaignMetricRow {
	out := make([]AdCampaignMetricRow, 0)
	for _, row := range rows {
		if !row.HasMetricData || row.Spend7d < cfg.MinAdSpend7d {
			continue
		}
		if row.DRRPct == nil || *row.DRRPct <= cfg.HighDRRThresholdPct {
			continue
		}
		out = append(out, row)
	}
	sortHighDRRRows(out)
	return out
}

func filterNoOrderCampaigns(rows []AdCampaignMetricRow, cfg CandidateBuilderConfig) []AdCampaignMetricRow {
	out := make([]AdCampaignMetricRow, 0)
	for _, row := range rows {
		if !row.HasMetricData || row.Spend7d < cfg.MinAdSpend7d {
			continue
		}
		if row.Orders7d != 0 {
			continue
		}
		out = append(out, row)
	}
	sortNoOrderCampaignRows(out)
	return out
}

func buildAdHighDRRCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	rows []AdCampaignMetricRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	id := candidateID(ActionTypeAdHighDRR, metricDate, "over_threshold")
	groupKey := entityGroupKey(id)

	var spendTotal, revenueTotal float64
	var maxDRR float64
	var drrSum float64
	var drrCount int
	for _, row := range rows {
		spendTotal += row.Spend7d
		revenueTotal += row.Revenue7d
		if row.DRRPct != nil {
			drrSum += *row.DRRPct
			drrCount++
			if *row.DRRPct > maxDRR {
				maxDRR = *row.DRRPct
			}
		}
	}
	avgDRR := 0.0
	if drrCount > 0 {
		avgDRR = round2(drrSum / float64(drrCount))
	}

	aggregates := map[string]any{
		"campaign_count":    len(rows),
		"spend_7d_total":    round2(spendTotal),
		"revenue_7d_total":  round2(revenueTotal),
		"avg_drr":           avgDRR,
		"max_drr":           round2(maxDRR),
		"drr_threshold_pct": round2(cfg.HighDRRThresholdPct),
	}

	facts := []ActionCandidateFact{
		formatCountFact(id, "campaign_count", "Кампаний", strconv.Itoa(len(rows)), "шт"),
		formatCountFact(id, "spend_7d_total", "Расход 7д", fmt.Sprintf("%.0f", spendTotal), "₽"),
		formatCountFact(id, "revenue_7d_total", "Выручка 7д", fmt.Sprintf("%.0f", revenueTotal), "₽"),
		formatCountFact(id, "drr", "ДРР", fmt.Sprintf("%.1f", avgDRR), "%"),
	}

	examples := make([]ActionCandidateExample, 0, cfg.MaxTopExamplesPerCandidate)
	for i, row := range rows {
		if i >= cfg.MaxTopExamplesPerCandidate {
			break
		}
		metrics := map[string]any{
			"spend_7d":   round2(row.Spend7d),
			"revenue_7d": round2(row.Revenue7d),
			"orders_7d":  row.Orders7d,
			"high_drr":   true,
			"high_spend": row.Spend7d > 0,
		}
		if row.DRRPct != nil {
			metrics["drr_pct"] = *row.DRRPct
		}
		examples = append(examples, campaignExample(row, metrics))
	}

	summary := map[string]any{
		"campaign_count": len(rows),
		"spend_7d_total": round2(spendTotal),
		"avg_drr":        avgDRR,
	}
	groupRef, err := persistCampaignEntityGroup(ctx, repo, sellerAccountID, metricDate, groupKey, rows, summary)
	if err != nil {
		return nil, err
	}

	priority := ActionPriorityImportant
	if maxDRR >= cfg.HighDRRThresholdPct*1.5 {
		priority = ActionPriorityCritical
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainAdvertising,
		Type:           ActionTypeAdHighDRR,
		Category:       ActionCategoryAdvertising,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("%d %s с ДРР выше %.0f%%", len(rows), campaignsNominative(len(rows)), cfg.HighDRRThresholdPct),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    examples,
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAOpenCampaignList,
			ActionCTAExportHighDRRCampaigns,
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{adHighDRRLimitation},
		SourceRefs:  []string{"ad_metrics_daily"},
	}, nil
}

func buildAdSpendNoOrdersCandidate(
	ctx context.Context,
	repo CandidateRepository,
	sellerAccountID int64,
	metricDate time.Time,
	rows []AdCampaignMetricRow,
	cfg CandidateBuilderConfig,
) (*ActionCandidate, error) {
	id := candidateID(ActionTypeAdSpendNoOrders, metricDate, "zero_orders")
	groupKey := entityGroupKey(id)

	var spendTotal float64
	for _, row := range rows {
		spendTotal += row.Spend7d
	}

	aggregates := map[string]any{
		"campaign_count":  len(rows),
		"spend_7d_total":  round2(spendTotal),
		"orders_7d_total": int64(0),
	}

	facts := []ActionCandidateFact{
		formatCountFact(id, "campaign_count", "Кампаний", strconv.Itoa(len(rows)), "шт"),
		formatCountFact(id, "spend_7d_total", "Расход 7д", fmt.Sprintf("%.0f", spendTotal), "₽"),
		formatCountFact(id, "zero_orders", "Заказы 7д", "0", "шт"),
	}

	examples := make([]ActionCandidateExample, 0, cfg.MaxTopExamplesPerCandidate)
	for i, row := range rows {
		if i >= cfg.MaxTopExamplesPerCandidate {
			break
		}
		examples = append(examples, campaignExample(row, map[string]any{
			"spend_7d":    round2(row.Spend7d),
			"revenue_7d":  round2(row.Revenue7d),
			"orders_7d":   row.Orders7d,
			"zero_orders": true,
			"high_spend":  row.Spend7d > 0,
		}))
	}

	summary := map[string]any{
		"campaign_count": len(rows),
		"spend_7d_total": round2(spendTotal),
	}
	groupRef, err := persistCampaignEntityGroup(ctx, repo, sellerAccountID, metricDate, groupKey, rows, summary)
	if err != nil {
		return nil, err
	}

	priority := ActionPriorityImportant
	if spendTotal >= cfg.MinAdSpend7d*5 {
		priority = ActionPriorityCritical
	}

	return &ActionCandidate{
		CandidateID:    id,
		Domain:         ActionDomainAdvertising,
		Type:           ActionTypeAdSpendNoOrders,
		Category:       ActionCategoryAdvertising,
		PriorityHint:   priority,
		Summary:        fmt.Sprintf("%d %s с расходом ≥%.0f₽ и нулём заказов за 7 дней", len(rows), campaignsNominative(len(rows)), cfg.MinAdSpend7d),
		Aggregates:     aggregates,
		Facts:          limitFacts(facts, cfg.MaxFactsPerCandidate),
		TopExamples:    examples,
		EntityGroupRef: groupRef,
		AllowedCTAs: []string{
			ActionCTAOpenCampaignList,
			ActionCTAExportNoOrderCampaigns,
			ActionCTAAskAI,
			ActionCTAMarkChecked,
		},
		Limitations: []string{adSpendNoOrdersLimitation},
		SourceRefs:  []string{"ad_metrics_daily"},
	}, nil
}
