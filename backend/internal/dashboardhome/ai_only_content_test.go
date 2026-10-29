package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestDashboardSynthesisInputMetricDateNotEmpty(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		SellerContext: aidatapack.SellerContext{MetricDate: "2026-06-30"},
		Tables: aidatapack.Tables{
			AccountRollup: aidatapack.AccountRollupTable{DataStatus: "available", CurrentDay: map[string]any{"orders_count": 84}},
		},
	}
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		t.Fatal(err)
	}
	input := buildDashboardSynthesisInput(build, map[string]any{"metric_date": "2026-06-30"}, nil, nil, pack, NarrativeRunOptions{})
	if input.MetricDate != "2026-06-30" {
		t.Fatalf("metric_date=%q", input.MetricDate)
	}
}

func TestDashboardSynthesisInputIncludesCategoryCatalogNames(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		SellerContext: aidatapack.SellerContext{MetricDate: "2026-06-30"},
		Tables: aidatapack.Tables{
			CategoryRollups: []aidatapack.CategoryRollupRow{{
				EntityID: "category:17036100", CategoryID: "17036100", Name: "Организация пространства",
			}},
		},
	}
	build, _, _, _ := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	input := buildDashboardSynthesisInput(build, nil, nil, nil, pack, NarrativeRunOptions{})
	if len(input.CategoryCatalog) != 1 || input.CategoryCatalog[0].CategoryName != "Организация пространства" {
		t.Fatalf("catalog=%+v", input.CategoryCatalog)
	}
}

func TestDashboardSynthesisInputIncludesStockDriverFromSKURollups(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		SellerContext: aidatapack.SellerContext{MetricDate: "2026-06-30"},
		Tables: aidatapack.Tables{
			SKURollups: []aidatapack.SKURollupRow{{
				EntityID:   "sku:1",
				CurrentDay: map[string]any{"stock_available": 5, "days_of_cover": 2.0},
				Last7d:     map[string]any{"orders": 10},
			}},
		},
	}
	signals := buildDataPackDashboardSignals(pack)
	foundStock := false
	for _, s := range signals {
		if s.ID == driverStock {
			foundStock = true
		}
	}
	if !foundStock {
		t.Fatalf("signals=%+v", signals)
	}
}

func TestDashboardAIOnlyRejectsStructurallyEmptyFunnelAnalysis(t *testing.T) {
	payload := minimalAIOnlyPayload()
	payload.FunnelAIAnalysis = &FunnelAIAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}, ShortSummary: "only"}
	errs := validateAIOnlyContentBlocks(payload)
	if len(errs) == 0 {
		t.Fatal("expected funnel structural rejection")
	}
}

func TestDashboardAIOnlyRejectsRiskOverviewEmptyGroupSeverity(t *testing.T) {
	payload := minimalAIOnlyPayload()
	payload.RiskOverview = &RiskDistributionWidget{
		WidgetBase: WidgetBase{Status: WidgetAvailable},
		Groups:     []RiskGroupCount{{Label: "Высокий риск", Count: 1}},
	}
	repair := repairDashboardNarrativePayloadAIOnly(payload)
	errs := validateAIOnlyContentBlocks(repair.Payload)
	if len(errs) != 0 {
		t.Fatalf("expected repair to fix groups, errs=%v", errs)
	}
}

func TestDashboardAIOnlyRejectsCategorySummaryRawIDWhenNameAvailable(t *testing.T) {
	input := DashboardSynthesisInput{
		CategoryCatalog: []DashboardCategoryCatalogEntry{{CategoryID: "17036100", CategoryName: "Организация пространства"}},
	}
	payload := minimalAIOnlyPayload()
	payload.CategorySummaries = []CategorySummaryCard{{CategoryName: "Категория 17036100", Headline: "h", KeyRisk: "r", RecommendedAction: "a"}}
	errs := ValidateDashboardAIOnlyContent(payload, input, nil)
	if len(errs) == 0 {
		t.Fatal("expected raw category id rejection")
	}
}

func TestDashboardAIOnlyWhatChangedDoesNotFallbackSilently(t *testing.T) {
	forceAIOnly(t)
	resp := Response{}
	bundle := whatChangedValidationBundle{}
	applyWhatChangedToResponse(&resp, nil, bundle, false)
	if blockApplySource(&resp, "what_changed") != "missing" {
		t.Fatalf("source=%s", blockApplySource(&resp, "what_changed"))
	}
}

func TestDashboardAIOnlyWhatChangedRepairsAttentionStableGroups(t *testing.T) {
	forceAIOnly(t)
	payload := &dashboardNarrativePayload{
		WhatChanged: &dashboardNarrativeWhatChanged{
			Summary: "Сводка изменений.",
			Groups: []dashboardNarrativeWhatChangedGroup{
				{Key: "worsened", Summary: "Ухудшение выручки.", Items: []dashboardNarrativeWhatChangedItem{{Text: "Снижение на 5,03%."}}},
				{Key: "attention", Summary: "Воронка требует внимания.", Items: []dashboardNarrativeWhatChangedItem{{Text: "Конверсия 13,22%."}}},
				{Key: "stable", Summary: "Спрос сохраняется.", Items: []dashboardNarrativeWhatChangedItem{{Text: "Лидеры продаются."}}},
			},
		},
	}
	resp := Response{}
	bundle := testWhatChangedBundle()
	applyWhatChangedToResponse(&resp, payload, bundle, true)
	if blockApplySource(&resp, "what_changed") != "ai" {
		t.Fatalf("source=%s status=%s", blockApplySource(&resp, "what_changed"), resp.AIDiagnostics.WhatChanged.Status)
	}
	if len(resp.AIDiagnostics.WhatChanged.Groups) != 4 {
		t.Fatalf("groups=%d", len(resp.AIDiagnostics.WhatChanged.Groups))
	}
}

func TestDashboardDebugAppliedBlocksReflectActualSources(t *testing.T) {
	forceAIOnly(t)
	resp := minimalAIOnlyHomeResponse()
	setBlockApplySource(&resp, "what_changed", "ai")
	debug := buildDashboardNarrativeDebug(&resp, nil)
	if debug.AppliedBlocks["what_changed"] != "ai" {
		t.Fatalf("what_changed=%s", debug.AppliedBlocks["what_changed"])
	}
}

func TestDashboardContentOrdersSemanticsConsistent(t *testing.T) {
	input := DashboardSynthesisInput{OrderSemantics: map[string]any{"daily_account_orders_count": int64(84)}}
	resp := minimalAIOnlyHomeResponse()
	for i := range resp.SellerPulse.KPIs {
		if resp.SellerPulse.KPIs[i].Key == "orders" {
			resp.SellerPulse.KPIs[i].Value = "84"
		}
	}
	resp.AIDiagnostics.AccountSummary.RiskDrivers = []RiskDriver{{
		ID: "driver-cancellations", Evidence: map[string]any{"orders_today": 84},
	}}
	errs := ValidateDashboardAIOnlyContent(nil, input, &resp)
	for _, e := range errs {
		if e.Code == "orders_semantics_mismatch" {
			t.Fatalf("unexpected mismatch: %s", e.Message)
		}
	}
}

func TestDashboardContentCriticalRiskKPIConsistent(t *testing.T) {
	resp := minimalAIOnlyHomeResponse()
	resp.AIDiagnostics.AccountSummary.RiskDrivers = []RiskDriver{
		{Severity: "critical"}, {Severity: "high"},
	}
	reconcileSellerPulseRisksKPI(&resp)
	for _, kpi := range resp.SellerPulse.KPIs {
		if kpi.Key == "risks" && kpi.Value != "2" {
			t.Fatalf("risks=%s", kpi.Value)
		}
	}
}

func TestDashboardTopActionStockRepair(t *testing.T) {
	payload := minimalAIOnlyPayload()
	payload.TopActions[0] = dashboardNarrativeTopAction{
		Title: "Защитить наличие у SKU", Reason: "остаток закончится", Category: "sales", DriverID: driverAccountSales,
	}
	repair := repairDashboardNarrativePayloadAIOnly(payload)
	if repair.Payload.TopActions[0].Category != "sales" || repair.Payload.TopActions[0].DriverID != driverAccountSales {
		t.Fatalf("action category/driver must stay AI-owned, got=%+v", repair.Payload.TopActions[0])
	}
}

func TestDashboardDRRMethodLabels(t *testing.T) {
	drr := 4.0
	resp := Response{
		SellerPulse:   SellerPulse{KPIs: []KPICard{{Key: "drr", Label: "ДРР в общей выручке", Value: "10%"}}},
		AIDiagnostics: AIDiagnostics{AdEfficiency: AdEfficiencyWidget{Summary: &AdEfficiencySummary{CurrentDRRPct: &drr}}},
	}
	errs := ValidateDashboardAIOnlyContent(nil, DashboardSynthesisInput{}, &resp)
	for _, e := range errs {
		if e.Code == "drr_semantics_unlabeled" {
			t.Fatalf("unexpected: %s", e.Message)
		}
	}
}

func TestDashboardSlowTurnoverNotOzonSurplus(t *testing.T) {
	resp := Response{
		SKUFocus: SKUFocus{Surplus: []SKUFocusCard{{}, {}}},
		AIDiagnostics: AIDiagnostics{StockCoverage: StockCoverageWidget{
			WidgetBase: WidgetBase{Message: "Медленная оборачиваемость / риск хранения: 2 SKU"},
			Segments:   []StockCoverageSegment{{Key: "surplus", Label: "SURPLUS (Ozon оборачиваемость)", SKUCount: 0}},
		}},
	}
	reconcileSurplusStockCoverageLabels(&resp)
	if !containsStr(resp.AIDiagnostics.StockCoverage.Message, "Медленная оборачиваемость") {
		t.Fatalf("message=%q", resp.AIDiagnostics.StockCoverage.Message)
	}
}

func TestDashboardRevenueAtRiskMethodNote(t *testing.T) {
	if sellerFacingRevenueMethodNote == "" {
		t.Fatal("empty method note")
	}
	if !containsStr(sellerFacingRevenueMethodNote, "критическим") {
		t.Fatalf("note=%q", sellerFacingRevenueMethodNote)
	}
}

func TestRiskDriversForAIOnlyUsesDailyAccountOrders(t *testing.T) {
	v8 := dashboardNarrativeContextV8{
		Account:     map[string]any{"current_day": map[string]any{"orders_count": 84}},
		RiskDrivers: []dashboardNarrativeRiskDriverView{{ID: driverAccountSales, Category: "sales", Severity: "medium", ImpactScore: 50, Title: "t", Reason: "r"}},
	}
	build := dashboardNarrativeBuildResult{AIContext: v8}
	drivers := riskDriversForAIOnlyApply(build, analyticsDashboardDTOWithOrders(247), &Response{}, nil, nil)
	if len(drivers) == 0 {
		t.Fatal("no drivers")
	}
	if v, ok := evidenceInt(drivers[0].Evidence, "orders_today"); !ok || v != 84 {
		t.Fatalf("orders_today=%v", drivers[0].Evidence)
	}
}

func analyticsDashboardDTOWithOrders(orders int32) analytics.DashboardMetricsDTO {
	return analytics.DashboardMetricsDTO{Account: analytics.DashboardAccountMetrics{OrdersToday: orders}}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexStr(s, sub))
}

func indexStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
