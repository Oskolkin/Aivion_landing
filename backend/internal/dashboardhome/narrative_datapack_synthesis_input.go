package dashboardhome

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func buildDashboardSynthesisInput(
	build dashboardNarrativeBuildResult,
	manifestSummary map[string]any,
	limitations []string,
	validatedSignals []DashboardChunkSignal,
	pack *aidatapack.SellerAIDataPack,
	opts NarrativeRunOptions,
) DashboardSynthesisInput {
	compactSignals := compactSignalsForSynthesis(validatedSignals)
	allowedFacts := sortedAllowlistKeys(signalFactIDSet(compactSignals))
	allowedEntities := sortedAllowlistKeys(signalEntityIDSet(compactSignals))

	metricDate := build.Context.MetricDate
	if metricDate == "" {
		if v8, ok := build.AIContext.(dashboardNarrativeContextV8); ok {
			metricDate = v8.MetricDate
		}
	}
	if metricDate == "" && manifestSummary != nil {
		if md, ok := manifestSummary["metric_date"].(string); ok {
			metricDate = md
		}
	}
	if metricDate == "" && pack != nil {
		metricDate = pack.SellerContext.MetricDate
	}

	hint := map[string]any{
		"headline":          "string",
		"executive_summary": "string",
		"account_health": map[string]any{
			"health_score":        "int 0-100",
			"severity":            "critical|high|medium|low|stable",
			"headline":            "string",
			"executive_summary":   "string",
			"improvement_summary": "string",
			"risk_drivers":        []any{},
		},
		"health_reasons":           []any{},
		"improvement_summary":      "string",
		"top_actions":              []any{},
		"what_changed":             map[string]any{"items": []any{}},
		"seller_pulse_narrative":   map[string]any{},
		"stock_coverage_narrative": map[string]any{"summary": "string"},
		"ad_efficiency_narrative":  map[string]any{"status": "available", "summary": "string", "checklist": []any{}},
		"data_quality_notes":       []any{},
		"driver_hints":             allDataPackDriverHints(),
		"candidate_hints":          allDataPackCandidateHints(build.CandidateByID),
	}

	catalog := buildCategoryCatalogForSynthesis(pack)
	return DashboardSynthesisInput{
		SchemaVersion:             DashboardSynthesisInputSchemaVersion,
		MetricDate:                metricDate,
		ContextSource:             ContextSourceSellerAIDataPackChunks,
		ManifestSummary:           trimManifestSummaryForSynthesis(manifestSummary),
		DataQualityLimitations:    limitations,
		ValidatedSignals:          compactSignals,
		AllowedFactIDs:            allowedFacts,
		AllowedEntityIDs:          allowedEntities,
		OutputSchemaHint:          hint,
		FinanceAvailable:          build.FinanceAvailable,
		PriceConstraintsAvailable: build.PriceConstraintsAvailable,
		CategoryCatalog:           catalog,
		CategorySelectionPolicy:   CategorySummariesSelectionPolicyAllCatalog,
		CategoryCatalogCount:      len(catalog),
		FunnelSemantics:           buildFunnelSemanticsForSynthesis(pack, build),
		OrderSemantics:            buildOrderSemanticsForSynthesis(pack),
		AdvertisingMetrics:        buildAdvertisingMetricsForSynthesis(pack),
		PeriodComparison:          buildPeriodComparisonForSynthesis(pack),
		GlobalSemanticContext:     firstNonNilGlobalContext(opts.GlobalContext, build.GlobalSemanticContext),
		DataPackHash:              firstNonEmpty(opts.DataPackHash, build.DataPackHash),
		GlobalContextHash:         firstNonEmpty(opts.GlobalContextHash, build.GlobalContextHash),
		PipelineRunID:             opts.PipelineRunID,
	}
}

func firstNonNilGlobalContext(a, b *aicontext.GlobalSemanticContext) *aicontext.GlobalSemanticContext {
	if a != nil {
		return a
	}
	return b
}

func allDataPackDriverHints() []map[string]string {
	return []map[string]string{
		{"id": driverStock, "category": "stock", "title": "Остатки и наличие", "definition": "Дефицит, покрытие, пополнение, out-of-stock, товары без продаж с остатком"},
		{"id": driverAdvertising, "category": "advertising", "title": "Реклама", "definition": "ДРР, кампании, ставки, бюджет, расход без заказов"},
		{"id": driverFunnel, "category": "funnel", "title": "Воронка", "definition": "Показы, просмотры карточек, корзина, заказы, переходы"},
		{"id": driverAccountSales, "category": "sales", "title": "Динамика продаж", "definition": "Общая динамика выручки/заказов аккаунта, без действий по пополнению"},
		{"id": driverSlowTurnover, "category": "slow_turnover", "title": "Медленная оборачиваемость", "definition": "Медленная оборачиваемость, зависшие остатки, риск хранения"},
		{"id": driverProductQuality, "category": "products", "title": "Качество карточек", "definition": "Видимость, валидация, ценовые индексы"},
		{"id": driverDataQuality, "category": "data_quality", "title": "Ограничения данных", "definition": "Неполные или недоступные данные"},
	}
}

func allDataPackCandidateHints(candidates map[string]dashboardNarrativeCandidate) []map[string]string {
	if len(candidates) == 0 {
		return []map[string]string{
			{"id": candidateStockReview, "category": "stock", "title": "Проверить остатки"},
			{"id": candidateAdEfficiencyReview, "category": "advertising", "title": "Проверить рекламу"},
			{"id": candidateFunnelDiagnosis, "category": "funnel", "title": "Диагностика воронки"},
			{"id": "candidate-account-review", "category": "sales", "title": "Проверить динамику продаж"},
		}
	}
	out := make([]map[string]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, map[string]string{"id": c.ID, "category": c.Category, "title": c.Title})
	}
	return out
}

func buildCategoryCatalogForSynthesis(pack *aidatapack.SellerAIDataPack) []DashboardCategoryCatalogEntry {
	if pack == nil {
		return nil
	}
	out := make([]DashboardCategoryCatalogEntry, 0, len(pack.Tables.CategoryRollups))
	for _, row := range pack.Tables.CategoryRollups {
		if row.Name == "" {
			continue
		}
		out = append(out, DashboardCategoryCatalogEntry{
			EntityID:     row.EntityID,
			CategoryID:   row.CategoryID,
			CategoryName: row.Name,
			Revenue7d:    row.Revenue7d,
			Orders7d:     row.Orders7d,
		})
	}
	return out
}

func buildOrderSemanticsForSynthesis(pack *aidatapack.SellerAIDataPack) map[string]any {
	sem := map[string]any{
		"daily_account_orders_count": map[string]string{
			"label": "Заказы аккаунта за день", "source": "account_rollup.current_day.orders_count",
		},
		"status_orders_count": map[string]string{
			"label": "Заказы по статусам (сумма)", "source": "orders_by_status",
		},
		"funnel_ordered_units": map[string]string{
			"label": "Заказанные единицы воронки", "source": "funnel.current_day.orders",
		},
		"funnel_completed_units": map[string]string{
			"label": "Завершённые единицы воронки", "source": "funnel.current_day.delivered_units",
		},
		"cancellations_denominator": map[string]string{
			"label": "База для отмен", "source": "daily_account_orders_count",
		},
		"rule": "Never write plain «заказы» when metrics differ. Use semantic labels from this block.",
	}
	if pack != nil && pack.Tables.AccountRollup.DataStatus == "available" {
		if v, ok := mapNumber(pack.Tables.AccountRollup.CurrentDay["orders_count"]); ok {
			sem["daily_account_orders_count"] = int64(v)
		}
	}
	return sem
}

func buildFunnelSemanticsForSynthesis(pack *aidatapack.SellerAIDataPack, build dashboardNarrativeBuildResult) map[string]any {
	sem := map[string]any{
		"rule":         "absolute_weakest_stage and largest_deterioration_stage are different concepts. Explain both separately; do not call both the main problem without explanation.",
		"instructions": "Use stage_day_over_day for weakest_stage (lowest absolute conversion/volume bottleneck) and largest_deterioration_stage (largest negative change_vs_previous_day_pct). Never claim prior-day comparison is unavailable when stage_day_over_day is present.",
	}
	if pack != nil && pack.Tables.FunnelRollup.DataStatus == "available" {
		sem["funnel_current_day"] = pack.Tables.FunnelRollup.CurrentDay
	}
	if v8, ok := build.AIContext.(dashboardNarrativeContextV8); ok {
		if comparisons, ok := v8.Funnel["stage_day_over_day"]; ok {
			sem["stage_day_over_day"] = comparisons
			if available, ok := v8.Funnel["comparison_available"].(bool); ok && available {
				sem["comparison_available"] = true
			}
		}
		if !dashboardAIOnlyEnabled() {
			if funnel, ok := v8.Funnel["primary_analysis"].(map[string]any); ok {
				sem["absolute_weakest_stage"] = funnel["absolute_weakest_stage"]
				sem["largest_deterioration_stage"] = funnel["largest_deterioration_stage"]
			}
		}
	}
	return sem
}

func buildAdvertisingMetricsForSynthesis(pack *aidatapack.SellerAIDataPack) map[string]any {
	if pack == nil {
		return nil
	}
	rollups := make([]map[string]any, 0, len(pack.Tables.CampaignRollups))
	for _, row := range pack.Tables.CampaignRollups {
		rollups = append(rollups, compactCampaignRollupForSynthesis(row))
	}
	return map[string]any{
		"campaign_level_only":  true,
		"all_campaign_rollups": rollups,
		"campaign_count":       len(rollups),
		"link_summary": map[string]any{
			"total_links":  len(pack.Tables.CampaignProductLinks),
			"active_links": countActiveCampaignLinks(pack.Tables.CampaignProductLinks),
		},
		"rule": "Use all_campaign_rollups and daily_series for per-campaign conclusions. Do not rely on filtered high_drr subsets only.",
	}
}

func compactCampaignRollupForSynthesis(row aidatapack.CampaignRollupRow) map[string]any {
	out := map[string]any{
		"entity_id":            row.EntityID,
		"campaign_external_id": row.CampaignExternalID,
		"name":                 row.Name,
		"state":                row.State,
		"current_day":          row.CurrentDay,
		"last_7d":              row.Last7d,
		"previous_7d":          row.Previous7d,
		"deltas":               row.Deltas,
		"daily_series":         row.DailySeries,
		"fact_ids":             row.FactIDs,
	}
	return out
}

func buildPeriodComparisonForSynthesis(pack *aidatapack.SellerAIDataPack) map[string]any {
	if pack == nil {
		return nil
	}
	comparison := buildDataPackPeriodComparison(pack)
	return map[string]any{
		"current_metric_date":  comparison.CurrentMetricDate,
		"previous_metric_date": comparison.PreviousMetricDate,
		"facts":                comparison.Facts,
		"rule":                 "AI must classify facts into what_changed groups. Backend provides neutral period_comparison only.",
	}
}
