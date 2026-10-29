package dashboardhome

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const (
	DashboardNarrativeContextSchemaV8    = "dashboard_narrative_context_v8"
	ContextSourceSellerAIDataPackChunks  = "seller_ai_data_pack_chunks"
	ContextSourceSellerAIDataPackOneShot = "seller_ai_data_pack_one_shot"
	ContextSourceSellerAIDataPack        = "seller_ai_data_pack"
	ContextSourceLegacyDashboard         = "legacy_dashboard_context"
)

// DataPack driver IDs — stable for validation and AI output.
const (
	driverAccountSales   = "driver-account-sales"
	driverStock          = "driver-stock"
	driverAdvertising    = "driver-advertising"
	driverFunnel         = "driver-funnel"
	driverProductQuality = "driver-product-quality"
	driverDataQuality    = "driver-data-quality"
	driverSlowTurnover   = "driver-slow-turnover"
)

// DataPack narrative candidate IDs for top_actions.
const (
	candidateStockReview        = "candidate-stock-review"
	candidateAdEfficiencyReview = "candidate-ad-efficiency-review"
	candidateFunnelDiagnosis    = "candidate-funnel-diagnosis"
	candidateProductCardCheck   = "candidate-product-card-check"
	candidateDataQualityNote    = "candidate-data-quality-note"
)

const dataPackStockExampleLimit = 5

type DataPackDashboardSignal struct {
	ID              string
	Category        string
	Severity        string
	Title           string
	EvidenceFactIDs []string
}

type dashboardNarrativeContextV8 struct {
	ContextSource            string                               `json:"context_source"`
	SchemaVersion            string                               `json:"schema_version"`
	MetricDate               string                               `json:"metric_date"`
	SellerContext            map[string]any                       `json:"seller_context"`
	DataQuality              map[string]any                       `json:"data_quality"`
	Account                  map[string]any                       `json:"account"`
	Sales                    map[string]any                       `json:"sales"`
	Stock                    map[string]any                       `json:"stock"`
	Advertising              map[string]any                       `json:"advertising"`
	Funnel                   map[string]any                       `json:"funnel"`
	Products                 map[string]any                       `json:"products"`
	Categories               map[string]any                       `json:"categories"`
	Constraints              map[string]any                       `json:"constraints"`
	FactCatalog              map[string]any                       `json:"fact_catalog"`
	EntityCatalog            map[string]any                       `json:"entity_catalog"`
	Limitations              []string                             `json:"limitations"`
	HealthScore              int32                                `json:"health_score"`
	Severity                 string                               `json:"severity"`
	RiskDrivers              []dashboardNarrativeRiskDriverView   `json:"risk_drivers"`
	RecommendationCandidates []dashboardNarrativeCandidate        `json:"recommendation_candidates"`
	SellerPulseContext       map[string]any                       `json:"seller_pulse_context"`
	StockCoverageContext     map[string]any                       `json:"stock_coverage_context"`
	AdEfficiencyContext      map[string]any                       `json:"ad_efficiency_context"`
	WhatChangedContext       dashboardNarrativeWhatChangedContext `json:"what_changed_context"`
}

type dataPackNarrativeLegacyInput struct {
	DTO          analytics.DashboardMetricsDTO
	AlertRows    []dbgen.Alert
	Recs         []dbgen.Recommendation
	PreviousSnap *accountMetricsSnapshot
}

// DataPackNarrativeLegacyInput carries optional legacy dashboard data for validation contexts.
type DataPackNarrativeLegacyInput struct {
	DTO analytics.DashboardMetricsDTO
}

func legacyInputFromExported(in DataPackNarrativeLegacyInput) dataPackNarrativeLegacyInput {
	return dataPackNarrativeLegacyInput{DTO: in.DTO}
}

func buildDashboardNarrativeContextFromDataPack(
	pack *aidatapack.SellerAIDataPack,
	legacyResp *Response,
) (dashboardNarrativeBuildResult, error) {
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, legacyResp, dataPackNarrativeLegacyInput{})
	return build, err
}

func buildDashboardNarrativeContextFromDataPackFull(
	pack *aidatapack.SellerAIDataPack,
	legacyResp *Response,
	legacy dataPackNarrativeLegacyInput,
) (dashboardNarrativeBuildResult, int32, string, error) {
	if pack == nil {
		return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("data pack is nil")
	}
	metricDate, err := time.Parse("2006-01-02", pack.SellerContext.MetricDate)
	if err != nil {
		return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("invalid metric_date in pack: %w", err)
	}

	signals := buildDataPackDashboardSignals(pack)
	var drivers []RiskDriver
	var score int32
	var severity string
	if dashboardAIOnlyEnabled() {
		drivers = []RiskDriver{}
		score = 0
		severity = ""
	} else {
		drivers = dataPackSignalsToRiskDrivers(signals, pack)
		score = computeHealthScoreBreakdown(drivers).FinalScore
		severity = computeHealthSeverity(score)
	}

	usedFactIDs := make(map[string]struct{})
	collectFactIDs := func(ids ...string) {
		for _, id := range ids {
			if id != "" {
				usedFactIDs[id] = struct{}{}
			}
		}
	}

	accountCtx, accountFacts := buildDataPackAccountContext(pack)
	collectFactIDs(accountFacts...)

	stockCtx, stockFacts := buildDataPackStockContext(pack)
	collectFactIDs(stockFacts...)

	adCtx, adFacts := buildDataPackAdvertisingContext(pack)
	collectFactIDs(adFacts...)

	funnelCtx, funnelFacts := buildDataPackFunnelContext(pack)
	if legacyResp != nil && len(legacyResp.AIDiagnostics.SalesFunnel.Stages) > 0 {
		enrichFunnelContextWithDayOverDay(funnelCtx, legacyResp.AIDiagnostics.SalesFunnel.Stages)
	}
	collectFactIDs(funnelFacts...)

	productsCtx, productFacts := buildDataPackProductsContext(pack)
	collectFactIDs(productFacts...)

	categoriesCtx, categoryFacts := buildDataPackCategoriesContext(pack)
	collectFactIDs(categoryFacts...)

	sellerPulseCtx, pulseFacts := buildDataPackSellerPulseContext(pack, drivers)
	collectFactIDs(pulseFacts...)

	factCatalog := buildDataPackFactCatalog(pack, usedFactIDs)
	entityCatalog := buildDataPackEntityCatalog(pack, usedFactIDs)
	limitations := append([]string(nil), pack.DataQuality.Limitations...)
	for _, lim := range pack.DataQuality.StructuredLimitations {
		if strings.TrimSpace(lim.Message) != "" {
			limitations = append(limitations, lim.Message)
		}
	}
	if pack.Tables.FinanceRollup.DataStatus != "available" {
		limitations = append(limitations, "finance_rollup_not_available")
	}

	candidates := buildDataPackNarrativeCandidates(signals)
	candidateByID := make(map[string]dashboardNarrativeCandidate, len(candidates))
	for _, c := range candidates {
		candidateByID[c.ID] = c
	}

	var whatChangedCtx dashboardNarrativeWhatChangedContext
	allowedQuotedNames := buildDataPackAllowedQuotedNames(pack)
	if dashboardAIOnlyEnabled() {
		periodComparison := buildDataPackPeriodComparison(pack)
		whatChangedCtx = buildWhatChangedValidationBundleFromPeriodComparison(periodComparison, allowedQuotedNames).Context
	} else if legacy.DTO.Account.RevenueToday > 0 || legacy.DTO.Account.OrdersToday > 0 || legacyResp != nil {
		if legacyResp != nil {
			whatChangedCtx = buildWhatChangedContext(metricDate, legacy.DTO, legacyResp, drivers, legacy.PreviousSnap)
			allowedQuotedNames = mergeAllowedNames(allowedQuotedNames, buildAllowedNarrativeQuotedNames(legacy.DTO, legacyResp))
		}
	} else {
		whatChangedCtx = buildDataPackWhatChangedContext(pack, drivers)
	}

	v8 := dashboardNarrativeContextV8{
		ContextSource:            ContextSourceSellerAIDataPack,
		SchemaVersion:            DashboardNarrativeContextSchemaV8,
		MetricDate:               pack.SellerContext.MetricDate,
		SellerContext:            dataPackSellerContextMap(pack),
		DataQuality:              dataPackDataQualityMap(pack),
		Account:                  accountCtx,
		Sales:                    accountCtx,
		Stock:                    stockCtx,
		Advertising:              adCtx,
		Funnel:                   funnelCtx,
		Products:                 productsCtx,
		Categories:               categoriesCtx,
		Constraints:              buildDataPackConstraintsMap(pack),
		FactCatalog:              factCatalog,
		EntityCatalog:            entityCatalog,
		Limitations:              limitations,
		HealthScore:              score,
		Severity:                 severity,
		RiskDrivers:              buildDashboardNarrativeRiskDriverViews(drivers),
		RecommendationCandidates: candidates,
		SellerPulseContext:       sellerPulseCtx,
		WhatChangedContext:       whatChangedCtx,
	}

	stockRequired := len(pack.Tables.StockPositions) > 0
	adRequired := len(pack.Tables.CampaignRollups) > 0
	sellerPulseRequired := len(sellerPulseCtx) > 0

	var stockNarrativeCtx stockCoverageNarrativeValidationContext
	var adEfficiencyCtx adEfficiencyNarrativeValidationContext
	if legacyResp != nil {
		v8.StockCoverageContext = buildStockCoverageContextMap(legacy.DTO, legacyResp, legacy.Recs)
		v8.AdEfficiencyContext = buildAdEfficiencyNarrativeContextMap(legacyResp)
		stockNarrativeCtx = buildStockCoverageNarrativeValidationContext(legacy.DTO, legacyResp, legacy.Recs)
		adEfficiencyCtx = buildAdEfficiencyNarrativeValidationContext(legacyResp)
	} else {
		v8.StockCoverageContext = map[string]any{"source": "seller_ai_data_pack", "stock": stockCtx}
		v8.AdEfficiencyContext = map[string]any{"source": "seller_ai_data_pack", "advertising": adCtx}
		stockNarrativeCtx = buildDataPackStockValidationContext(pack, stockCtx)
		adEfficiencyCtx = buildDataPackAdEfficiencyValidationContext(pack, adCtx)
	}

	build := dashboardNarrativeBuildResult{
		Context: dashboardNarrativeContext{
			MetricDate:               pack.SellerContext.MetricDate,
			HealthScore:              score,
			Severity:                 severity,
			RiskDrivers:              buildDashboardNarrativeRiskDriverViews(drivers),
			RecommendationCandidates: candidates,
		},
		ContextSource:      ContextSourceSellerAIDataPack,
		AIContext:          v8,
		ContextHash:        hashNarrativeValue(v8),
		FactualContextHash: buildDataPackFactualContextHash(pack, drivers, signals),
		RiskDriversHash:    hashNarrativeValue(buildRiskDriverHashInput(drivers)),
		CandidateByID:      candidateByID,
		MetricCatalogByKey: buildDataPackMetricCatalog(factCatalog),
		AllowedQuotedNames: allowedQuotedNames,
	}
	if dashboardAIOnlyEnabled() {
		periodComparison := buildDataPackPeriodComparison(pack)
		build.WhatChanged = buildWhatChangedValidationBundleFromPeriodComparison(periodComparison, allowedQuotedNames)
	} else {
		build.WhatChanged = buildWhatChangedValidationBundle(whatChangedCtx, allowedQuotedNames)
	}
	build.StockNarrativeCtx = stockNarrativeCtx
	build.StockNarrativeRequired = stockRequired
	build.AdEfficiencyNarrativeCtx = adEfficiencyCtx
	build.AdEfficiencyNarrativeRequired = adRequired
	build.SellerPulseCtx = buildSellerPulseNarrativeValidationContext(extractKPIsFromPulseContext(sellerPulseCtx), dashboardNarrativeBuildResult{CandidateByID: candidateByID})
	build.SellerPulseRequired = sellerPulseRequired
	build.DataPackFactIDs = factIDsFromSet(usedFactIDs)
	build.FinanceAvailable = pack.Tables.FinanceRollup.DataStatus == "available"
	build.PriceConstraintsAvailable = pack.Constraints.PriceConstraintsAvailable
	return build, score, severity, nil
}

func buildDataPackDashboardSignals(pack *aidatapack.SellerAIDataPack) []DataPackDashboardSignal {
	var signals []DataPackDashboardSignal

	if pack.Tables.AccountRollup.DataStatus == "available" {
		facts := pack.Tables.AccountRollup.FactIDs
		signals = append(signals, DataPackDashboardSignal{
			ID: driverAccountSales, Category: "sales", Severity: "medium",
			Title: "Динамика продаж аккаунта", EvidenceFactIDs: facts,
		})
	}

	stockDist := countStockGrades(pack.Tables.StockPositions)
	rollupRisks := countStockRisksFromSKURollups(pack)
	if stockDist["DEFICIT"]+stockDist["WAITING_FOR_SUPPLY"]+stockDist["NO_SALES"]+stockDist["SURPLUS"] > 0 ||
		rollupRisks.criticalOrShort > 0 || rollupRisks.noSalesWithStock > 0 {
		var facts []string
		for _, row := range pack.Tables.StockPositions {
			facts = append(facts, row.FactIDs...)
		}
		for _, row := range pack.Tables.SKURollups {
			facts = append(facts, row.FactIDs...)
		}
		sev := "medium"
		if stockDist["DEFICIT"] > 0 || rollupRisks.criticalOrShort > 0 {
			sev = "high"
		}
		signals = append(signals, DataPackDashboardSignal{
			ID: driverStock, Category: "stock", Severity: sev,
			Title: "Риски по остаткам и оборачиваемости", EvidenceFactIDs: compactFactIDs(facts),
		})
	}

	if rollupRisks.slowTurnover > 0 {
		var facts []string
		for _, row := range pack.Tables.SKURollups {
			facts = append(facts, row.FactIDs...)
		}
		signals = append(signals, DataPackDashboardSignal{
			ID: driverSlowTurnover, Category: "slow_turnover", Severity: "medium",
			Title: "Медленная оборачиваемость и риск хранения", EvidenceFactIDs: compactFactIDs(facts),
		})
	}

	adSignals := classifyDataPackCampaigns(pack)
	if len(adSignals.highDRR)+len(adSignals.spendNoOrders)+len(adSignals.inactive) > 0 {
		var facts []string
		for _, c := range pack.Tables.CampaignRollups {
			facts = append(facts, c.FactIDs...)
		}
		signals = append(signals, DataPackDashboardSignal{
			ID: driverAdvertising, Category: "advertising", Severity: "high",
			Title: "Эффективность рекламных кампаний", EvidenceFactIDs: compactFactIDs(facts),
		})
	}

	if pack.Tables.FunnelRollup.DataStatus == "available" {
		signals = append(signals, DataPackDashboardSignal{
			ID: driverFunnel, Category: "funnel", Severity: "medium",
			Title: "Воронка продаж", EvidenceFactIDs: pack.Tables.FunnelRollup.FactIDs,
		})
	}

	pq := countProductQualityIssues(pack.Tables.ProductEnrichment)
	if pq.visibility+pq.validation+pq.priceYellow+pq.priceRed > 0 {
		var facts []string
		for _, row := range pack.Tables.ProductEnrichment {
			facts = append(facts, row.FactIDs...)
		}
		signals = append(signals, DataPackDashboardSignal{
			ID: driverProductQuality, Category: "products", Severity: "medium",
			Title: "Качество карточек товаров", EvidenceFactIDs: compactFactIDs(facts),
		})
	}

	if pack.Tables.FinanceRollup.DataStatus != "available" || hasDataQualityLimitations(pack) {
		signals = append(signals, DataPackDashboardSignal{
			ID: driverDataQuality, Category: "data_quality", Severity: "low",
			Title: "Ограничения данных", EvidenceFactIDs: nil,
		})
	}

	return signals
}

func dataPackSignalsToRiskDrivers(signals []DataPackDashboardSignal, pack *aidatapack.SellerAIDataPack) []RiskDriver {
	out := make([]RiskDriver, 0, len(signals))
	for _, sig := range signals {
		evidence := map[string]any{"fact_ids": sig.EvidenceFactIDs}
		if sig.ID == driverStock {
			evidence["distribution"] = countStockGrades(pack.Tables.StockPositions)
		}
		if sig.ID == driverAdvertising {
			cs := classifyDataPackCampaigns(pack)
			evidence["high_drr_count"] = len(cs.highDRR)
			evidence["spend_without_orders_count"] = len(cs.spendNoOrders)
		}
		out = append(out, RiskDriver{
			ID:            sig.ID,
			Category:      sig.Category,
			CategoryLabel: categoryLabelFor(sig.Category),
			Severity:      sig.Severity,
			ImpactScore:   impactScoreForSeverity(sig.Severity),
			Title:         sig.Title,
			Reason:        dataPackDriverReason(sig, pack),
			Evidence:      evidence,
		})
	}
	return out
}

func categoryLabelFor(category string) string {
	switch category {
	case "sales":
		return "Продажи"
	case "stock":
		return "Остатки"
	case "advertising":
		return "Реклама"
	case "funnel":
		return "Воронка"
	case "products":
		return "Товары"
	case "data_quality":
		return "Качество данных"
	case "slow_turnover":
		return "Медленная оборачиваемость"
	default:
		return category
	}
}

func impactScoreForSeverity(severity string) int {
	switch severity {
	case "critical":
		return 90
	case "high":
		return 70
	case "medium":
		return 50
	case "low":
		return 30
	default:
		return 40
	}
}

func dataPackDriverReason(sig DataPackDashboardSignal, pack *aidatapack.SellerAIDataPack) string {
	switch sig.ID {
	case driverStock:
		d := countStockGrades(pack.Tables.StockPositions)
		return fmt.Sprintf("DEFICIT=%d, WAITING_FOR_SUPPLY=%d, NO_SALES=%d, SURPLUS=%d", d["DEFICIT"], d["WAITING_FOR_SUPPLY"], d["NO_SALES"], d["SURPLUS"])
	case driverDataQuality:
		return strings.Join(pack.DataQuality.Limitations, "; ")
	default:
		return sig.Title
	}
}

func buildDataPackNarrativeCandidates(signals []DataPackDashboardSignal) []dashboardNarrativeCandidate {
	signalIDs := map[string]bool{}
	for _, s := range signals {
		signalIDs[s.ID] = true
	}
	var out []dashboardNarrativeCandidate
	add := func(id, category, title, reason string, drivers ...string) {
		out = append(out, dashboardNarrativeCandidate{
			ID: id, Source: "data_pack", Priority: "medium", Category: category,
			CategoryLabel: categoryLabelFor(category), Title: title, Reason: reason,
			NextStep:            "Проверьте соответствующий блок дашборда и факты из fact_catalog.",
			CompatibleDriverIDs: drivers,
		})
	}
	if signalIDs[driverStock] {
		add(candidateStockReview, "stock", "Проверить остатки", "Есть SKU с риском дефицита или излишка", driverStock)
	}
	if signalIDs[driverSlowTurnover] {
		add("candidate-slow-turnover-review", "slow_turnover", "Проверить медленную оборачиваемость", "Есть SKU с риском хранения", driverSlowTurnover)
	}
	if signalIDs[driverAdvertising] {
		add(candidateAdEfficiencyReview, "advertising", "Проверить рекламу", "Есть кампании с высоким DRR или расходом без заказов", driverAdvertising)
	}
	if signalIDs[driverFunnel] {
		add(candidateFunnelDiagnosis, "funnel", "Диагностика воронки", "Доступны данные воронки за период", driverFunnel)
	}
	if signalIDs[driverProductQuality] {
		add(candidateProductCardCheck, "products", "Проверить карточки", "Есть предупреждения по видимости или ценовым индексам", driverProductQuality)
	}
	if signalIDs[driverDataQuality] {
		add(candidateDataQualityNote, "data_quality", "Учесть ограничения данных", "Часть данных недоступна или неполна", driverDataQuality)
	}
	if signalIDs[driverAccountSales] {
		add("candidate-account-review", "sales", "Проверить динамику продаж", "Есть метрики продаж за период", driverAccountSales)
	}
	return out
}

func buildDataPackAccountContext(pack *aidatapack.SellerAIDataPack) (map[string]any, []string) {
	ar := pack.Tables.AccountRollup
	return map[string]any{
		"data_status": ar.DataStatus,
		"current_day": ar.CurrentDay,
		"last_7d":     ar.Last7d,
		"previous_7d": ar.Previous7d,
		"deltas":      ar.Deltas,
		"fact_ids":    ar.FactIDs,
	}, ar.FactIDs
}

func countStockGrades(rows []aidatapack.StockPositionRow) map[string]int {
	out := map[string]int{"DEFICIT": 0, "WAITING_FOR_SUPPLY": 0, "NO_SALES": 0, "SURPLUS": 0, "ACTUAL": 0}
	for _, row := range rows {
		grade := strings.TrimSpace(strings.ToUpper(row.TurnoverGrade))
		if grade == "" {
			continue
		}
		out[grade]++
	}
	return out
}

func buildDataPackStockContext(pack *aidatapack.SellerAIDataPack) (map[string]any, []string) {
	dist := countStockGrades(pack.Tables.StockPositions)
	var allFacts []string
	note := ""
	for _, row := range pack.Tables.StockPositions {
		allFacts = append(allFacts, row.FactIDs...)
		if row.TurnoverGradeAggregationMethod != "" && note == "" {
			note = row.TurnoverGradeAggregationMethod
		}
	}
	examples := map[string][]map[string]any{
		"deficit":            {},
		"waiting_for_supply": {},
		"no_sales":           {},
		"surplus":            {},
	}
	gradeToKey := map[string]string{
		"DEFICIT": "deficit", "WAITING_FOR_SUPPLY": "waiting_for_supply",
		"NO_SALES": "no_sales", "SURPLUS": "surplus",
	}
	for _, row := range pack.Tables.StockPositions {
		key := gradeToKey[strings.ToUpper(row.TurnoverGrade)]
		if key == "" {
			continue
		}
		if len(examples[key]) >= dataPackStockExampleLimit {
			continue
		}
		name := entityNameForSKU(pack, row.EntityID)
		examples[key] = append(examples[key], map[string]any{
			"entity_id": row.EntityID, "sku": skuFromEntityID(row.EntityID),
			"name": name, "turnover_grade": row.TurnoverGrade,
			"available_stock_count": row.AvailableStockCount,
		})
	}
	status := "available"
	if len(pack.Tables.StockPositions) == 0 {
		status = "not_available"
	}
	return map[string]any{
		"data_status":                  status,
		"distribution":                 dist,
		"examples":                     examples,
		"stock_grade_aggregation_note": note,
		"fact_ids":                     compactFactIDs(allFacts),
	}, compactFactIDs(allFacts)
}

type dataPackCampaignScenarios struct {
	highDRR       []map[string]any
	spendNoOrders []map[string]any
	healthy       []map[string]any
	inactive      []map[string]any
}

func classifyDataPackCampaigns(pack *aidatapack.SellerAIDataPack) dataPackCampaignScenarios {
	var out dataPackCampaignScenarios
	for _, c := range pack.Tables.CampaignRollups {
		entry := campaignSummaryEntry(pack, c)
		if isInactiveCampaignState(c.State) {
			out.inactive = append(out.inactive, entry)
		}
		w := c.Last7d
		if w.SpendWithoutOrders && w.Spend > 0 {
			out.spendNoOrders = append(out.spendNoOrders, entry)
		}
		if w.DRRPct != nil && *w.DRRPct > analytics.AdEfficiencyHighDRRThresholdPct {
			out.highDRR = append(out.highDRR, entry)
		}
		if w.ROAS != nil && *w.ROAS >= 3 && w.Orders > 0 {
			out.healthy = append(out.healthy, entry)
		}
	}
	return out
}

func isInactiveCampaignState(state string) bool {
	s := strings.ToUpper(strings.TrimSpace(state))
	return s == "INACTIVE" || s == "ARCHIVED" || s == "STOPPED" || s == "PAUSED"
}

func campaignSummaryEntry(pack *aidatapack.SellerAIDataPack, c aidatapack.CampaignRollupRow) map[string]any {
	name := c.Name
	if ent, ok := pack.EntityRegistry.Campaigns[c.EntityID]; ok && ent.Name != "" {
		name = ent.Name
	}
	entry := map[string]any{
		"entity_id": c.EntityID, "campaign_external_id": c.CampaignExternalID,
		"name": name, "state": c.State, "last_7d": c.Last7d, "fact_ids": c.FactIDs,
	}
	return entry
}

func buildDataPackAdvertisingContext(pack *aidatapack.SellerAIDataPack) (map[string]any, []string) {
	cs := classifyDataPackCampaigns(pack)
	var allFacts []string
	for _, c := range pack.Tables.CampaignRollups {
		allFacts = append(allFacts, c.FactIDs...)
	}
	linkSummary := map[string]any{
		"total_links":  len(pack.Tables.CampaignProductLinks),
		"active_links": countActiveCampaignLinks(pack.Tables.CampaignProductLinks),
	}
	return map[string]any{
		"campaign_level_only":            true,
		"high_drr_campaigns":             cs.highDRR,
		"spend_without_orders_campaigns": cs.spendNoOrders,
		"healthy_campaigns":              cs.healthy,
		"inactive_by_state_campaigns":    cs.inactive,
		"campaign_product_link_summary":  linkSummary,
		"fact_ids":                       compactFactIDs(allFacts),
	}, compactFactIDs(allFacts)
}

func countActiveCampaignLinks(links []aidatapack.CampaignProductLinkRow) int {
	n := 0
	for _, l := range links {
		if l.IsActive {
			n++
		}
	}
	return n
}

func buildDataPackFunnelContext(pack *aidatapack.SellerAIDataPack) (map[string]any, []string) {
	fr := pack.Tables.FunnelRollup
	ctx := map[string]any{
		"data_status": fr.DataStatus,
		"current_day": fr.CurrentDay,
		"last_7d":     fr.Last7d,
		"transitions": []any{},
		"fact_ids":    fr.FactIDs,
	}
	if fr.DataStatus == "available" && !dashboardAIOnlyEnabled() {
		ctx["primary_analysis"] = map[string]any{
			"absolute_weakest_stage":      inferAbsoluteWeakestFunnelStage(fr.CurrentDay),
			"largest_deterioration_stage": inferLargestDeteriorationFunnelStage(fr.CurrentDay),
		}
	}
	return ctx, fr.FactIDs
}

type skuRollupStockRiskCounts struct {
	criticalOrShort  int
	noSalesWithStock int
	slowTurnover     int
}

func countStockRisksFromSKURollups(pack *aidatapack.SellerAIDataPack) skuRollupStockRiskCounts {
	var out skuRollupStockRiskCounts
	for _, row := range pack.Tables.SKURollups {
		stock, _ := mapNumber(row.CurrentDay["stock_available"])
		orders7, _ := mapNumber(row.Last7d["orders"])
		doc, hasDoc := mapNumber(row.CurrentDay["days_of_cover"])
		if stock > 0 && orders7 == 0 {
			out.noSalesWithStock++
		}
		if stock > 0 && orders7 > 0 && hasDoc && doc > 0 && doc <= 7 {
			out.criticalOrShort++
		}
		if stock >= 14 && orders7 > 0 && orders7 <= 3 && stock/orders7 >= 10 {
			out.slowTurnover++
		}
	}
	return out
}

func inferAbsoluteWeakestFunnelStage(current map[string]any) map[string]any {
	bestKey, bestLabel, bestVal := "", "", 100.0
	stages := []struct {
		key, label string
	}{
		{"impressions_to_product_views", "Показы → Просмотры карточки"},
		{"product_views_to_add_to_cart", "Просмотры карточки → Корзина"},
		{"add_to_cart_to_orders", "Корзина → Заказанные единицы"},
		{"orders_to_delivered_units", "Заказанные единицы → Доставленные единицы"},
	}
	for _, st := range stages {
		if v, ok := mapNumber(current[st.key]); ok && v < bestVal {
			bestVal = v
			bestKey = st.key
			bestLabel = st.label
		}
	}
	if bestKey == "" {
		return map[string]any{}
	}
	return map[string]any{
		"key": bestKey, "label": bestLabel, "metric": "Конверсия", "value_pct": bestVal,
	}
}

func inferLargestDeteriorationFunnelStage(current map[string]any) map[string]any {
	// Placeholder from current_day transition deltas when present.
	if raw, ok := current["transitions_delta_pp"].(map[string]any); ok {
		worstKey, worstDelta := "", 0.0
		for k, v := range raw {
			if f, ok := mapNumber(v); ok && f < worstDelta {
				worstDelta = f
				worstKey = k
			}
		}
		if worstKey != "" {
			return map[string]any{"key": worstKey, "delta_pp": worstDelta}
		}
	}
	return map[string]any{}
}

type productQualityCounts struct {
	visibility, validation, priceYellow, priceRed int
}

func countProductQualityIssues(rows []aidatapack.ProductEnrichmentRow) productQualityCounts {
	var c productQualityCounts
	for _, row := range rows {
		if vis, ok := row.Visibility["has_stock"].(bool); ok && !vis {
			c.visibility++
		}
		if st, ok := row.Status["validation_status"].(string); ok && strings.Contains(strings.ToLower(st), "warn") {
			c.validation++
		}
		if pi, ok := row.PriceIndexes["color_index"].(string); ok {
			switch strings.ToUpper(pi) {
			case "YELLOW":
				c.priceYellow++
			case "RED":
				c.priceRed++
			}
		}
	}
	return c
}

func buildDataPackProductsContext(pack *aidatapack.SellerAIDataPack) (map[string]any, []string) {
	pq := countProductQualityIssues(pack.Tables.ProductEnrichment)
	var allFacts []string
	var examples []map[string]any
	for _, row := range pack.Tables.ProductEnrichment {
		allFacts = append(allFacts, row.FactIDs...)
		if len(examples) >= dataPackStockExampleLimit {
			continue
		}
		if hasProductIssue(row) {
			examples = append(examples, map[string]any{
				"entity_id": row.EntityID, "sku": skuFromEntityID(row.EntityID),
				"name":       entityNameForSKU(pack, row.EntityID),
				"visibility": row.Visibility, "status": row.Status, "price_indexes": row.PriceIndexes,
			})
		}
	}
	return map[string]any{
		"visibility_has_stock_false_count": pq.visibility,
		"validation_warning_count":         pq.validation,
		"price_index_yellow_count":         pq.priceYellow,
		"price_index_red_count":            pq.priceRed,
		"examples":                         examples,
		"source":                           enrichmentSource(pack),
	}, compactFactIDs(allFacts)
}

func hasProductIssue(row aidatapack.ProductEnrichmentRow) bool {
	if vis, ok := row.Visibility["has_stock"].(bool); ok && !vis {
		return true
	}
	if st, ok := row.Status["validation_status"].(string); ok && strings.Contains(strings.ToLower(st), "warn") {
		return true
	}
	if pi, ok := row.PriceIndexes["color_index"].(string); ok && (pi == "YELLOW" || pi == "RED") {
		return true
	}
	return false
}

func enrichmentSource(pack *aidatapack.SellerAIDataPack) string {
	if enrich, ok := pack.DataQuality.Sources["product_enrichment"].(map[string]any); ok {
		if src, ok := enrich["source"].(string); ok {
			return src
		}
	}
	return ""
}

func buildDataPackCategoriesContext(pack *aidatapack.SellerAIDataPack) (map[string]any, []string) {
	rows := append([]aidatapack.CategoryRollupRow(nil), pack.Tables.CategoryRollups...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Revenue7d != rows[j].Revenue7d {
			return rows[i].Revenue7d > rows[j].Revenue7d
		}
		return rows[i].Orders7d > rows[j].Orders7d
	})
	limit := 10
	if len(rows) > limit {
		rows = rows[:limit]
	}
	var allFacts []string
	top := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		allFacts = append(allFacts, row.FactIDs...)
		top = append(top, map[string]any{
			"entity_id": row.EntityID, "name": row.Name, "path": row.Path,
			"revenue_7d": row.Revenue7d, "orders_7d": row.Orders7d,
			"deficit_sku_count": row.DeficitSKUCount, "no_sales_sku_count": row.NoSalesSKUCount,
			"surplus_sku_count": row.SurplusSKUCount,
		})
	}
	return map[string]any{"top_by_revenue": top, "fact_ids": compactFactIDs(allFacts)}, compactFactIDs(allFacts)
}

func buildDataPackSellerPulseContext(pack *aidatapack.SellerAIDataPack, drivers []RiskDriver) (map[string]any, []string) {
	var kpis []map[string]any
	var factIDs []string
	ar := pack.Tables.AccountRollup
	if ar.DataStatus == "available" {
		factIDs = append(factIDs, ar.FactIDs...)
		if rev, ok := ar.Last7d["revenue"]; ok {
			kpis = append(kpis, map[string]any{"key": "revenue_7d", "value": rev, "label": "Выручка 7д"})
		}
		if ord, ok := ar.Last7d["orders"]; ok {
			kpis = append(kpis, map[string]any{"key": "orders_7d", "value": ord, "label": "Заказы 7д"})
		}
		if rev, ok := ar.CurrentDay["revenue"]; ok {
			kpis = append(kpis, map[string]any{"key": "revenue_today", "value": rev, "label": "Выручка сегодня"})
		}
	}
	var topSignals []map[string]any
	for _, d := range drivers {
		topSignals = append(topSignals, map[string]any{
			"driver_id": d.ID, "category": d.Category, "severity": d.Severity, "title": d.Title,
		})
	}
	allowedRefs := []string{"revenue", "orders", "drr", "risks", "stock"}
	return map[string]any{
		"kpis": kpis, "top_signals": topSignals,
		"fact_ids": compactFactIDs(factIDs), "allowed_metric_refs": allowedRefs,
	}, compactFactIDs(factIDs)
}

func buildDataPackFactCatalog(pack *aidatapack.SellerAIDataPack, used map[string]struct{}) map[string]any {
	out := make(map[string]any, len(used))
	for id := range used {
		fact, ok := pack.FactRegistry[id]
		if !ok {
			continue
		}
		out[id] = map[string]any{
			"formatted_value": fact.FormattedValue,
			"unit":            fact.Unit,
			"metric":          fact.Metric,
			"entity_id":       fact.EntityID,
		}
	}
	return out
}

func buildDataPackEntityCatalog(pack *aidatapack.SellerAIDataPack, usedFacts map[string]struct{}) map[string]any {
	entityIDs := map[string]struct{}{}
	for id := range usedFacts {
		if f, ok := pack.FactRegistry[id]; ok && f.EntityID != "" {
			entityIDs[f.EntityID] = struct{}{}
		}
	}
	out := map[string]any{}
	for eid := range entityIDs {
		if ent, ok := pack.EntityRegistry.SKUs[eid]; ok {
			out[eid] = map[string]any{"type": "sku", "name": ent.Name, "sku": ent.SKU}
			continue
		}
		if ent, ok := pack.EntityRegistry.Campaigns[eid]; ok {
			out[eid] = map[string]any{"type": "campaign", "name": ent.Name, "campaign_external_id": ent.CampaignExternalID}
		}
	}
	return out
}

func buildDataPackConstraintsMap(pack *aidatapack.SellerAIDataPack) map[string]any {
	return map[string]any{
		"do_not_invent_metrics":                     true,
		"do_not_rename_campaigns":                   true,
		"do_not_classify_by_campaign_name":          true,
		"do_not_recommend_price_cut_without_margin": !pack.Constraints.PriceConstraintsAvailable,
		"do_not_increase_ads_for_low_stock":         true,
		"ad_efficiency_campaign_level_only":         true,
		"ad_efficiency_no_sku_attribution":          true,
		"finance_available":                         pack.Tables.FinanceRollup.DataStatus == "available",
		"price_constraints_available":               pack.Constraints.PriceConstraintsAvailable,
	}
}

func dataPackSellerContextMap(pack *aidatapack.SellerAIDataPack) map[string]any {
	return map[string]any{
		"seller_account_id": pack.SellerContext.SellerAccountID,
		"marketplace":       pack.SellerContext.Marketplace,
		"metric_date":       pack.SellerContext.MetricDate,
		"timezone":          pack.SellerContext.Timezone,
	}
}

func dataPackDataQualityMap(pack *aidatapack.SellerAIDataPack) map[string]any {
	return map[string]any{
		"sources":     pack.DataQuality.Sources,
		"limitations": pack.DataQuality.Limitations,
	}
}

func buildDataPackAllowedQuotedNames(pack *aidatapack.SellerAIDataPack) map[string]struct{} {
	names := map[string]struct{}{}
	for _, ent := range pack.EntityRegistry.Campaigns {
		if n := strings.TrimSpace(ent.Name); n != "" {
			names[n] = struct{}{}
		}
	}
	for _, ent := range pack.EntityRegistry.SKUs {
		if n := strings.TrimSpace(ent.Name); n != "" {
			names[n] = struct{}{}
		}
	}
	return names
}

func mergeAllowedNames(a, b map[string]struct{}) map[string]struct{} {
	if a == nil {
		a = map[string]struct{}{}
	}
	for k := range b {
		a[k] = struct{}{}
	}
	return a
}

func buildDataPackWhatChangedContext(pack *aidatapack.SellerAIDataPack, drivers []RiskDriver) dashboardNarrativeWhatChangedContext {
	ctx := dashboardNarrativeWhatChangedContext{
		CurrentMetricDate: pack.SellerContext.MetricDate,
		AllowedNumbers:    []string{},
	}
	ar := pack.Tables.AccountRollup
	if ar.DataStatus != "available" {
		return ctx
	}
	if deltas, ok := ar.Deltas["revenue_pct"]; ok {
		ctx.WorsenedFacts = append(ctx.WorsenedFacts, whatChangedFact{
			ID: "account.revenue_delta", Category: "sales", Label: "Выручка",
			Direction: "worsened", DeltaPct: fmt.Sprint(deltas),
		})
	}
	for _, d := range drivers {
		if d.Severity == "high" || d.Severity == "critical" {
			ctx.NewRiskFacts = append(ctx.NewRiskFacts, whatChangedFact{
				ID: d.ID, Category: d.Category, Label: d.Title, Direction: "new_risk", Note: d.Reason,
			})
		}
	}
	return ctx
}

func buildDataPackFactualContextHash(pack *aidatapack.SellerAIDataPack, drivers []RiskDriver, signals []DataPackDashboardSignal) string {
	input := map[string]any{
		"schema_version":     DashboardNarrativeContextSchemaV8,
		"metric_date":        pack.SellerContext.MetricDate,
		"context_source":     ContextSourceSellerAIDataPack,
		"account_status":     pack.Tables.AccountRollup.DataStatus,
		"stock_count":        len(pack.Tables.StockPositions),
		"campaign_count":     len(pack.Tables.CampaignRollups),
		"funnel_status":      pack.Tables.FunnelRollup.DataStatus,
		"finance_status":     pack.Tables.FinanceRollup.DataStatus,
		"fact_registry_len":  len(pack.FactRegistry),
		"drivers":            buildRiskDriverHashInput(drivers),
		"stock_distribution": countStockGrades(pack.Tables.StockPositions),
	}
	return hashNarrativeValue(input)
}

func buildDataPackMetricCatalog(factCatalog map[string]any) map[string]dashboardNarrativeMetric {
	out := make(map[string]dashboardNarrativeMetric)
	for id, raw := range factCatalog {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		label, _ := m["metric"].(string)
		value, _ := m["formatted_value"].(string)
		out[id] = dashboardNarrativeMetric{Key: id, Label: label, Value: value}
	}
	return out
}

func buildDataPackStockValidationContext(pack *aidatapack.SellerAIDataPack, stockCtx map[string]any) stockCoverageNarrativeValidationContext {
	dist, _ := stockCtx["distribution"].(map[string]int)
	seg := map[string]int{
		"critical_0_3":   dist["DEFICIT"],
		"shortage_4_7":   dist["WAITING_FOR_SUPPLY"],
		"no_sales":       dist["NO_SALES"],
		"surplus":        dist["SURPLUS"],
		"healthy_14plus": dist["ACTUAL"],
	}
	names := buildDataPackAllowedQuotedNames(pack)
	nums := map[string]struct{}{}
	for _, f := range pack.FactRegistry {
		if f.FormattedValue != "" {
			nums[f.FormattedValue] = struct{}{}
		}
	}
	return stockCoverageNarrativeValidationContext{
		SegmentCounts: seg, AllowedProductNames: names, AllowedNumbers: nums,
	}
}

func buildDataPackAdEfficiencyValidationContext(pack *aidatapack.SellerAIDataPack, _ map[string]any) adEfficiencyNarrativeValidationContext {
	ctx := adEfficiencyNarrativeValidationContext{
		WidgetStatus:         WidgetAvailable,
		AllowedCampaignNames: buildDataPackAllowedQuotedNames(pack),
		HighDRRCampaignNames: map[string]struct{}{},
		NoOrderCampaignNames: map[string]struct{}{},
		CampaignLevelOnly:    true,
		HasFactualData:       len(pack.Tables.CampaignRollups) > 0,
	}
	cs := classifyDataPackCampaigns(pack)
	addNames := func(target map[string]struct{}, entries []map[string]any) {
		for _, e := range entries {
			if name, ok := e["name"].(string); ok && name != "" {
				target[name] = struct{}{}
				ctx.AllowedCampaignNames[name] = struct{}{}
			}
		}
	}
	addNames(ctx.HighDRRCampaignNames, cs.highDRR)
	addNames(ctx.NoOrderCampaignNames, cs.spendNoOrders)
	return ctx
}

func entityNameForSKU(pack *aidatapack.SellerAIDataPack, entityID string) string {
	if ent, ok := pack.EntityRegistry.SKUs[entityID]; ok {
		return ent.Name
	}
	return ""
}

func skuFromEntityID(entityID string) string {
	if strings.HasPrefix(entityID, "sku:") {
		return strings.TrimPrefix(entityID, "sku:")
	}
	return entityID
}

func compactFactIDs(ids []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func factIDsFromSet(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func hasDataQualityLimitations(pack *aidatapack.SellerAIDataPack) bool {
	if len(pack.DataQuality.Limitations) > 0 || len(pack.DataQuality.StructuredLimitations) > 0 {
		return true
	}
	if enrich, ok := pack.DataQuality.Sources["product_enrichment"].(map[string]any); ok {
		if src, _ := enrich["source"].(string); src == "dev_seed" {
			return true
		}
	}
	return pack.Tables.FinanceRollup.DataStatus != "available"
}

func extractKPIsFromPulseContext(ctx map[string]any) []KPICard {
	raw, ok := ctx["kpis"].([]map[string]any)
	if !ok {
		return nil
	}
	var out []KPICard
	for _, k := range raw {
		key, _ := k["key"].(string)
		label, _ := k["label"].(string)
		val := fmt.Sprint(k["value"])
		out = append(out, KPICard{Key: key, Label: label, Value: val})
	}
	return out
}

// DumpDashboardNarrativeContextForDebug exports the context that would be sent to AI.
func DumpDashboardNarrativeContextForDebug(
	pack *aidatapack.SellerAIDataPack,
	legacyResp *Response,
	legacy DataPackNarrativeLegacyInput,
) (map[string]any, error) {
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, legacyResp, legacyInputFromExported(legacy))
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(build.AIContext)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
