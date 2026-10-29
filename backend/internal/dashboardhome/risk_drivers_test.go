package dashboardhome

import (
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestApplyUnifiedAccountHealth_OverallAccountAnalysisKeepsActionsMultiCategory(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     80000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.28)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.19)},
			OrdersToday:      40,
			OrdersYesterday:  58,
			CancelsToday:     9,
			CancellationRate: riskFloatPtr(0.225),
		},
		SKUs: []analytics.DashboardSKUMetricsItem{
			{OzonProductID: 1, ProductName: riskStringPtr("Футболка базовая"), ContributionToRevenueChange: -28000},
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			SalesFunnel: SalesFunnelWidget{Stages: []FunnelStage{
				{Key: "product_views", Label: "Просмотры карточки", ChangeVsPreviousDayPct: riskFloatPtr(-18)},
				{Key: "add_to_cart", Label: "Добавление в корзину", ChangeVsPreviousDayPct: riskFloatPtr(-34)},
				{Key: "orders", Label: "Заказы", ChangeVsPreviousDayPct: riskFloatPtr(-22)},
			}},
			FunnelAnalysis: FunnelAnalysisWidget{
				PrimaryDropStage:   "Просмотры карточки → Добавление в корзину",
				PrimaryDropMessage: "Основная потеря сейчас на этапе Просмотры карточки → Добавление в корзину.",
			},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 11, CampaignName: "Кампания под контролем", Spend: 7000, Revenue: 5000, Orders: 2, EfficiencySignal: "weak_efficiency"},
			}},
		},
	}
	actionPool := []TopActionItem{
		{
			ID:            "ad-1",
			Source:        "recommendation",
			Priority:      "high",
			PriorityLabel: "Высокий",
			Category:      "advertising",
			CategoryLabel: "Реклама",
			Title:         "Сократите бюджет кампании с низкой отдачей",
			Reason:        "Часть рекламных расходов не окупается.",
			NextStep:      "Проверьте запросы и площадки показа.",
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, actionPool, nil, nil)

	drivers := resp.AIDiagnostics.AccountSummary.RiskDrivers
	if len(drivers) < 3 {
		t.Fatalf("drivers=%v", drivers)
	}
	want := map[string]bool{"sales": false, "cancellations": false, "funnel": false}
	for _, driver := range drivers[:3] {
		if _, ok := want[driver.Category]; ok {
			want[driver.Category] = true
		}
	}
	for category, ok := range want {
		if !ok {
			t.Fatalf("top drivers missing %s: %#v", category, drivers[:3])
		}
	}
	if len(resp.AIDiagnostics.TopActions) == 0 {
		t.Fatal("expected fallback top actions when AI narrative is not current")
	}
	if allTopActionsAreCategory(resp.AIDiagnostics.TopActions, "advertising") {
		t.Fatalf("fallback must spread across categories, got %#v", resp.AIDiagnostics.TopActions)
	}
	for _, action := range resp.AIDiagnostics.TopActions {
		if strings.TrimSpace(action.ExpectedEffect) == "" {
			t.Fatalf("fallback top action missing expected_effect: %#v", action)
		}
	}
}

func TestApplyUnifiedAccountHealth_AdvertisingDominantAllowsMultipleAdvertisingActions(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.05)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.03)},
			OrdersToday:      55,
			OrdersYesterday:  57,
			CancelsToday:     2,
			CancellationRate: riskFloatPtr(0.03),
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			SalesFunnel: SalesFunnelWidget{Stages: []FunnelStage{
				{Key: "product_views", Label: "Просмотры карточки", ChangeVsPreviousDayPct: riskFloatPtr(-6)},
				{Key: "add_to_cart", Label: "Добавление в корзину", ChangeVsPreviousDayPct: riskFloatPtr(-7)},
			}},
			FunnelAnalysis: FunnelAnalysisWidget{
				PrimaryDropStage:   "Просмотры карточки → Добавление в корзину",
				PrimaryDropMessage: "Есть умеренное снижение конверсии.",
			},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик — без заказов", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
				{CampaignExternalID: 22, CampaignName: "Слабая окупаемость", Spend: 31000, Revenue: 9000, Orders: 1, EfficiencySignal: "weak_efficiency"},
				{CampaignExternalID: 23, CampaignName: "Контрольный расход", Spend: 18000, Revenue: 12000, Orders: 2, EfficiencySignal: "weak_efficiency"},
			}},
		},
	}
	actionPool := []TopActionItem{
		makeAdAction("ad-1", "critical", "Остановите неэффективную рекламную кампанию"),
		makeAdAction("ad-2", "high", "Сократите бюджет кампании с низкой отдачей"),
		makeAdAction("ad-3", "medium", "Поставьте рекламную кампанию на контроль"),
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, actionPool, nil, nil)

	if len(resp.AIDiagnostics.AccountSummary.RiskDrivers) == 0 || resp.AIDiagnostics.AccountSummary.RiskDrivers[0].Category != "advertising" {
		t.Fatalf("top driver=%#v", resp.AIDiagnostics.AccountSummary.RiskDrivers)
	}
	if len(resp.AIDiagnostics.TopActions) == 0 {
		t.Fatal("expected fallback top actions when advertising dominates")
	}
	advertisingCount := 0
	for _, action := range resp.AIDiagnostics.TopActions {
		if action.Category == "advertising" {
			advertisingCount++
		}
	}
	if advertisingCount < 2 {
		t.Fatalf("advertising-dominant fallback should allow multiple advertising cards, got %#v", resp.AIDiagnostics.TopActions)
	}
}

func TestApplyUnifiedAccountHealth_MixedDriversSpreadTopActionsAcrossCategories(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     95000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.07)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.06)},
			OrdersToday:      45,
			OrdersYesterday:  50,
			CancelsToday:     6,
			CancellationRate: riskFloatPtr(0.13),
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			StockCoverage: StockCoverageWidget{Segments: []StockCoverageSegment{
				{Key: "critical_0_3", Label: "0-3 дня", SKUCount: 2, Severity: "critical"},
				{Key: "shortage_4_7", Label: "4-7 дней", SKUCount: 3, Severity: "high"},
			}},
			SalesFunnel: SalesFunnelWidget{Stages: []FunnelStage{
				{Key: "product_views", Label: "Просмотры карточки", ChangeVsPreviousDayPct: riskFloatPtr(-12)},
			}},
			FunnelAnalysis: FunnelAnalysisWidget{
				PrimaryDropStage:   "Просмотры карточки → Добавление в корзину",
				PrimaryDropMessage: "Конверсия на входе в корзину ослабла.",
			},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 31, CampaignName: "Слабая кампания", Spend: 24000, Revenue: 9000, Orders: 1, EfficiencySignal: "weak_efficiency"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk: []SKUFocusCard{{ProductName: "Кроссовки"}},
		},
	}
	actionPool := []TopActionItem{
		makeAdAction("ad-1", "high", "Сократите бюджет кампании с низкой отдачей"),
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, actionPool, nil, nil)

	if len(resp.AIDiagnostics.TopActions) == 0 {
		t.Fatal("expected fallback top actions for mixed drivers")
	}
	categories := map[string]struct{}{}
	for _, action := range resp.AIDiagnostics.TopActions {
		categories[action.Category] = struct{}{}
	}
	if len(categories) < 2 {
		t.Fatalf("mixed-driver fallback should spread across categories, got %#v", resp.AIDiagnostics.TopActions)
	}
}

func TestApplyUnifiedAccountHealth_ConsistencyLinksActionsToRiskDrivers(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     70000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.16)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.12)},
			OrdersToday:      32,
			OrdersYesterday:  40,
			CancelsToday:     7,
			CancellationRate: riskFloatPtr(0.22),
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			SalesFunnel: SalesFunnelWidget{Stages: []FunnelStage{
				{Key: "add_to_cart", Label: "Добавление в корзину", ChangeVsPreviousDayPct: riskFloatPtr(-25)},
			}},
			FunnelAnalysis: FunnelAnalysisWidget{
				PrimaryDropStage:   "Карточка → Корзина",
				PrimaryDropMessage: "Воронка заметно ослабла.",
			},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 41, CampaignName: "Слабая кампания", Spend: 15000, Revenue: 5000, Orders: 1, EfficiencySignal: "weak_efficiency"},
			}},
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil)

	if resp.AIDiagnostics.AccountSummary.HealthScoreBreakdown == nil {
		t.Fatal("missing health score breakdown")
	}

	driverIDs := map[string]struct{}{}
	for _, driver := range resp.AIDiagnostics.AccountSummary.RiskDrivers {
		driverIDs[driver.ID] = struct{}{}
	}
	for _, action := range resp.AIDiagnostics.TopActions {
		if action.DriverID == "" {
			t.Fatalf("missing driver_id in action %#v", action)
		}
		if _, ok := driverIDs[action.DriverID]; !ok {
			t.Fatalf("driver %q not found for action %#v", action.DriverID, action)
		}
	}
	summary := strings.ToLower(resp.AIDiagnostics.AccountSummary.ImprovementSummary)
	if !strings.Contains(summary, "отмен") && !strings.Contains(summary, "воронк") && !strings.Contains(summary, "выруч") {
		t.Fatalf("summary=%q", summary)
	}
}

func TestAdvertisingDriver_PreservesCampaignNameButNotTechnicalTokenInActionTitle(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 51, CampaignName: "Перегрев по ad_waste SKU", Spend: 295178, Revenue: 3473, Orders: 3, EfficiencySignal: "weak_efficiency"},
			}},
		},
	}
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{RevenueToday: 180000},
	}
	driver, ok := buildAdvertisingRiskDriver(resp, dto, nil, nil)
	if !ok {
		t.Fatal("expected advertising driver")
	}
	action := fallbackActionForDriver(driver)
	if !strings.Contains(driver.Reason, "Перегрев по ad_waste SKU") {
		t.Fatalf("campaign name lost: %q", driver.Reason)
	}
	if strings.Contains(strings.ToLower(action.Title), "ad_waste") || strings.Contains(strings.ToLower(action.NextStep), "ad_waste") {
		t.Fatalf("technical token leaked into seller-facing action: %#v", action)
	}
}

func TestDetectTopActionCategory_IgnoresCampaignNameForStockClassification(t *testing.T) {
	entityID := "882000071563"
	category := detectTopActionCategory(topActionCandidate{
		Source:            "ai_summary",
		Priority:          "high",
		RelatedEntityType: "campaign",
		RelatedEntityID:   &entityID,
		Title:             "Проверьте кампанию «низкий остаток»",
		WhatHappened:      "Кампания слабо окупается.",
	})
	if category != "advertising" {
		t.Fatalf("category=%q want advertising", category)
	}
}

func TestHealthScoreNoDrivers(t *testing.T) {
	score := computeHealthScore(nil)
	if score != 88 {
		t.Fatalf("score=%d want 88", score)
	}
	breakdown := computeHealthScoreBreakdown(nil)
	if breakdown.FinalScore != 88 || breakdown.RawPenalty != 0 || breakdown.SaturatedPenalty != 0 || len(breakdown.Drivers) != 0 {
		t.Fatalf("breakdown=%#v", breakdown)
	}
}

func TestHealthScoreSingleMedium(t *testing.T) {
	score := computeHealthScore([]RiskDriver{
		makeRiskDriver("driver-medium", "sales", "medium", 40),
	})
	assertScoreInRange(t, int(score), 90, 95)
}

func TestHealthScoreSingleCritical(t *testing.T) {
	score := computeHealthScore([]RiskDriver{
		makeRiskDriver("driver-critical", "advertising", "critical", 90),
	})
	assertScoreInRange(t, int(score), 78, 95)
}

func TestHealthScoreTwoCritical(t *testing.T) {
	score := computeHealthScore([]RiskDriver{
		makeRiskDriver("driver-ad", "advertising", "critical", 90),
		makeRiskDriver("driver-stock", "stock", "critical", 90),
	})
	assertScoreInRange(t, int(score), 60, 80)
}

func TestHealthScoreLiveLikeCase(t *testing.T) {
	drivers := []RiskDriver{
		makeRiskDriver("driver-advertising", "advertising", "critical", 98),
		makeRiskDriver("driver-stock", "stock", "critical", 92),
		makeRiskDriver("driver-funnel", "funnel", "high", 53),
		makeRiskDriver("driver-cancellations", "cancellations", "critical", 50),
		makeRiskDriver("driver-sales", "sales", "medium", 26),
	}
	score := computeHealthScore(drivers)
	assertScoreInRange(t, int(score), 45, 62)
	if score == 10 {
		t.Fatal("live-like score should not collapse to 10")
	}
	breakdown := computeHealthScoreBreakdown(drivers)
	if breakdown.FinalScore != score {
		t.Fatalf("score=%d breakdown=%#v", score, breakdown)
	}
	if len(breakdown.Drivers) != len(drivers) {
		t.Fatalf("breakdown drivers=%d", len(breakdown.Drivers))
	}
	if breakdown.Drivers[0].CappedDriverPenalty > 28 {
		t.Fatalf("per-driver cap exceeded: %#v", breakdown.Drivers[0])
	}
}

func TestHealthScoreExtremeCase(t *testing.T) {
	drivers := []RiskDriver{
		makeRiskDriver("d1", "advertising", "critical", 100),
		makeRiskDriver("d2", "stock", "critical", 100),
		makeRiskDriver("d3", "funnel", "critical", 100),
		makeRiskDriver("d4", "cancellations", "critical", 100),
		makeRiskDriver("d5", "sales", "critical", 100),
		makeRiskDriver("d6", "surplus", "critical", 100),
	}
	score := computeHealthScore(drivers)
	assertScoreInRange(t, int(score), 38, 55)
	if score < 10 {
		t.Fatalf("score=%d", score)
	}
}

func TestHealthScoreAdditionalDriversDoNotHardCrashScore(t *testing.T) {
	baseDrivers := []RiskDriver{
		makeRiskDriver("d1", "advertising", "critical", 95),
		makeRiskDriver("d2", "stock", "critical", 90),
		makeRiskDriver("d3", "funnel", "high", 60),
		makeRiskDriver("d4", "cancellations", "high", 55),
	}
	score4 := computeHealthScore(baseDrivers)
	score6 := computeHealthScore(append(append([]RiskDriver{}, baseDrivers...),
		makeRiskDriver("d5", "sales", "medium", 35),
		makeRiskDriver("d6", "surplus", "medium", 30),
	))
	if score6 == 10 {
		t.Fatalf("score6=%d should not hard crash to 10", score6)
	}
	if int(score4)-int(score6) > 8 {
		t.Fatalf("extra drivers dropped score too sharply: score4=%d score6=%d", score4, score6)
	}
}

func TestHealthSeverityBands(t *testing.T) {
	cases := []struct {
		score int32
		want  string
	}{
		{score: 88, want: "low"},
		{score: 75, want: "attention"},
		{score: 60, want: "medium"},
		{score: 40, want: "high"},
		{score: 25, want: "critical"},
	}
	for _, tc := range cases {
		if got := computeHealthSeverity(tc.score); got != tc.want {
			t.Fatalf("score=%d got=%q want=%q", tc.score, got, tc.want)
		}
	}
}

func TestTopActionCategoryFromDriver_Funnel(t *testing.T) {
	action := fallbackActionForDriver(makeRiskDriver("driver-funnel", "funnel", "high", 53))
	if action.Category != "funnel" || action.CategoryLabel != "Воронка" {
		t.Fatalf("action=%#v", action)
	}
}

func TestTopActionCategoryFromDriver_Cancellations(t *testing.T) {
	action := fallbackActionForDriver(makeRiskDriver("driver-cancellations", "cancellations", "critical", 50))
	if action.Category != "cancellations" || action.CategoryLabel != "Отмены" {
		t.Fatalf("action=%#v", action)
	}
}

func TestTopActionCategoryFromDriver_SalesRevenue(t *testing.T) {
	action := fallbackActionForDriver(makeRiskDriver("driver-sales-revenue", "sales", "medium", 26))
	if action.Category != "sales" || action.CategoryLabel != "Продажи" {
		t.Fatalf("action=%#v", action)
	}
}

func TestSelectTopActionsFromDrivers_OverridesLinkedActionCategoryByDriver(t *testing.T) {
	drivers := []RiskDriver{
		makeRiskDriver("driver-funnel", "funnel", "high", 53),
	}
	pool := []TopActionItem{
		{
			ID:                      "recommendation-7",
			Source:                  "recommendation",
			Priority:                "high",
			PriorityLabel:           "Высокий",
			Category:                "sales",
			CategoryLabel:           "Продажи",
			Title:                   "Разберите причину падения продаж",
			Reason:                  "Продажи снизились.",
			NextStep:                "Проверьте товары.",
			LinkedRecommendationIDs: []int64{7},
		},
	}
	drivers[0].LinkedRecommendationIDs = []int64{7}
	actions := selectTopActionsFromDrivers(drivers, pool, 3)
	if len(actions) != 1 {
		t.Fatalf("actions=%#v", actions)
	}
	if actions[0].Category != "funnel" || actions[0].CategoryLabel != "Воронка" {
		t.Fatalf("action=%#v", actions[0])
	}
}

func TestRiskDriverReasons_LocalizeRussianFormatting(t *testing.T) {
	reason := buildStockRiskReason(6, 5, "Светильник")
	if !strings.Contains(reason, "6 SKU") {
		t.Fatalf("reason=%q", reason)
	}
	if !strings.Contains(reason, "ещё 5 SKU") {
		t.Fatalf("reason=%q", reason)
	}
	if !strings.Contains(reason, "4–7 дней") {
		t.Fatalf("reason=%q", reason)
	}
}

func TestImprovementSummary_UsesTopDriversWithoutDuplicatingActions(t *testing.T) {
	drivers := []RiskDriver{
		makeRiskDriver("driver-advertising", "advertising", "critical", 98),
		makeRiskDriver("driver-stock", "stock", "critical", 92),
		makeRiskDriver("driver-funnel", "funnel", "high", 53),
	}
	summary := buildRiskImprovementSummary(drivers)
	for _, expected := range []string{
		"сокращения рекламных расходов с низкой отдачей",
		"пополнения критичных SKU",
		"устранения провала воронки",
	} {
		if !strings.Contains(summary, expected) {
			t.Fatalf("summary=%q missing %q", summary, expected)
		}
	}
	for _, forbidden := range []string{
		"Остановите неэффективную рекламную кампанию",
		"Проверьте товар с риском дефицита",
		"Разберите причину падения продаж",
	} {
		if strings.Contains(summary, forbidden) {
			t.Fatalf("summary duplicated action title: %q", summary)
		}
	}
}

func TestHealthScoreBreakdownAlwaysExposed(t *testing.T) {
	if !shouldExposeHealthScoreBreakdown() {
		t.Fatal("health score breakdown must always be exposed")
	}
}

func TestAnalyzeFunnel_LocalizesPercent(t *testing.T) {
	stages := []FunnelStage{
		{Key: "impressions", Label: "Показы", Value: riskInt64Ptr(100)},
		{Key: "product_views", Label: "Просмотры карточки", Value: riskInt64Ptr(13)},
	}
	result := analyzeFunnel(stages, nil)
	if !strings.Contains(result.message, "87,0%") {
		t.Fatalf("message=%q", result.message)
	}
}

func TestAdvertisingDriver_RoundsMoneyInReason(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 52, CampaignName: "Перегрев по ad_waste SKU", Spend: 295178, Revenue: 42242.04, Orders: 33, EfficiencySignal: "weak_efficiency"},
			}},
		},
	}
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{RevenueToday: 180000},
	}
	driver, ok := buildAdvertisingRiskDriver(resp, dto, nil, nil)
	if !ok {
		t.Fatal("expected advertising driver")
	}
	if !strings.Contains(driver.Reason, "295 178 ₽") || !strings.Contains(driver.Reason, "42 242 ₽") {
		t.Fatalf("reason=%q", driver.Reason)
	}
}

func makeAdAction(id string, priority string, title string) TopActionItem {
	return TopActionItem{
		ID:            id,
		Source:        "recommendation",
		Priority:      priority,
		PriorityLabel: topActionPriorityLabel(priority),
		Category:      "advertising",
		CategoryLabel: "Реклама",
		Title:         title,
		Reason:        "Часть рекламных расходов не окупается.",
		NextStep:      "Проверьте кампанию.",
	}
}

func allTopActionsAreCategory(actions []TopActionItem, category string) bool {
	if len(actions) == 0 {
		return false
	}
	for _, action := range actions {
		if action.Category != category {
			return false
		}
	}
	return true
}

func countTopActionsByCategory(actions []TopActionItem, category string) int {
	count := 0
	for _, action := range actions {
		if action.Category == category {
			count++
		}
	}
	return count
}

func uniqueActionCategories(actions []TopActionItem) int {
	seen := map[string]struct{}{}
	for _, action := range actions {
		seen[action.Category] = struct{}{}
	}
	return len(seen)
}

func makeRiskDriver(id string, category string, severity string, impactScore int) RiskDriver {
	return RiskDriver{
		ID:            id,
		Category:      category,
		CategoryLabel: riskDriverCategoryLabel(category),
		Severity:      severity,
		ImpactScore:   impactScore,
		Title:         "Driver title",
		Reason:        "Driver reason",
	}
}

func assertScoreInRange(t *testing.T, score int, minValue int, maxValue int) {
	t.Helper()
	if score < minValue || score > maxValue {
		t.Fatalf("score=%d want range [%d,%d]", score, minValue, maxValue)
	}
}

func riskFloatPtr(v float64) *float64 { return &v }
func riskStringPtr(v string) *string  { return &v }
func riskInt64Ptr(v int64) *int64     { return &v }

func mustRiskDate(t *testing.T, iso string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatal(err)
	}
	return d.UTC()
}
