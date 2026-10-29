package dashboardhome

import (
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestApplyUnifiedAccountHealth_UsesFreshStoredDashboardNarrative(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.08)},
			OrdersToday:      55,
			OrdersYesterday:  63,
			CancelsToday:     2,
			CancellationRate: riskFloatPtr(0.03),
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk:        []SKUFocusCard{},
			Surplus:          []SKUFocusCard{},
			OrderDrop:        []SKUFocusCard{},
			NoSalesWithStock: []SKUFocusCard{},
			KeySKUs:          []KeySkuCard{},
		},
	}
	actionPool := []TopActionItem{}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	score := computeHealthScoreBreakdown(drivers).FinalScore
	severity := computeHealthSeverity(score)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, score, severity, nil, actionPool, nil, nil)
	fallbackWhatChanged := buildFallbackWhatChangedPayload(build.Context.WhatChangedContext)

	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		ID:                  "narrative-1",
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload: &dashboardNarrativePayload{
			Headline:           "Главный риск дня в рекламе.",
			ExecutiveSummary:   "Сильнее всего индекс давит реклама: кампания «Холодный трафик» тратит бюджет без заказов.",
			ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов без результата.",
			WhatChanged:        fallbackWhatChanged,
			HealthReasons: []dashboardNarrativeHealthReason{
				{
					DriverID: "driver-advertising",
					Category: "advertising",
					Title:    "Реклама тратит бюджет без результата",
					Reason:   "Кампания «Холодный трафик» уже тратит деньги без заказов и выручки.",
				},
			},
			TopActions: []dashboardNarrativeTopAction{
				{
					DriverID:             "driver-advertising",
					SourceCandidateID:    "driver-advertising-action",
					Priority:             "critical",
					Category:             "advertising",
					Title:                "Сократите расход по кампании без заказов",
					Reason:               "Кампания «Холодный трафик» продолжает расход без результата.",
					NextStep:             "Сначала поставьте кампанию на паузу. Затем проверьте запросы и товары в ней: если после корректировки заказов не появится, не возвращайте бюджет.",
					ExpectedEffect:       "Снизить риск рекламного перерасхода.",
					SupportingMetricRefs: []string{"ad_spend"},
				},
			},
			SellerPulseNarrative: sampleSellerPulseNarrativeForTests(),
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, actionPool, nil, nil)

	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		t.Fatalf("source=%q", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	if resp.AIDiagnostics.AccountSummary.Headline != "Главный риск дня в рекламе." {
		t.Fatalf("headline=%q", resp.AIDiagnostics.AccountSummary.Headline)
	}
	if resp.AIDiagnostics.AccountSummary.HealthScore == nil || *resp.AIDiagnostics.AccountSummary.HealthScore != score {
		t.Fatalf("health_score=%v want %d", resp.AIDiagnostics.AccountSummary.HealthScore, score)
	}
	if len(resp.AIDiagnostics.AccountSummary.RiskDrivers) == 0 || !strings.Contains(resp.AIDiagnostics.AccountSummary.RiskDrivers[0].Reason, "Холодный трафик") {
		t.Fatalf("risk_drivers=%#v", resp.AIDiagnostics.AccountSummary.RiskDrivers)
	}
	if len(resp.AIDiagnostics.TopActions) == 0 || resp.AIDiagnostics.TopActions[0].Source != "ai_summary" {
		t.Fatalf("top_actions=%#v", resp.AIDiagnostics.TopActions)
	}
	if got := resp.AIDiagnostics.TopActions[0].SupportingMetrics; len(got) != 1 || got[0].Value != "42 000 ₽" {
		t.Fatalf("supporting_metrics=%#v", got)
	}
	if resp.AIDiagnostics.TopActions[0].ExpectedEffect != "Снизить риск рекламного перерасхода." {
		t.Fatalf("expected_effect=%q", resp.AIDiagnostics.TopActions[0].ExpectedEffect)
	}
}

func TestApplyUnifiedAccountHealth_FallsBackWhenStoredNarrativeIsStale(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.08)},
			OrdersToday:      55,
			OrdersYesterday:  63,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				WidgetBase: pendingWidget(""),
				storedNarrative: &storedDashboardNarrative{
					ID:              "stale-narrative",
					Status:          "completed",
					PromptVersion:   DashboardNarrativePromptVersion,
					ContextHash:     "old-context",
					RiskDriversHash: "old-drivers",
					Payload: &dashboardNarrativePayload{
						Headline:           "Старый headline",
						ImprovementSummary: "Старое improvement summary",
					},
				},
			},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
			}},
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, nil, nil, nil)

	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "fallback" {
		t.Fatalf("source=%q", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	if resp.AIDiagnostics.AccountSummary.Headline == "Старый headline" {
		t.Fatalf("stale headline should not be used: %#v", resp.AIDiagnostics.AccountSummary)
	}
	if resp.AIDiagnostics.AccountSummary.ValidationStatus != "fallback_stale" {
		t.Fatalf("validation_status=%q", resp.AIDiagnostics.AccountSummary.ValidationStatus)
	}
	if len(resp.AIDiagnostics.TopActions) == 0 {
		t.Fatal("stale narrative should still expose fallback top actions")
	}
	for _, action := range resp.AIDiagnostics.TopActions {
		if action.Source == "ai_summary" {
			t.Fatalf("stale narrative must not expose AI narrative top actions: %#v", action)
		}
		if strings.TrimSpace(action.ExpectedEffect) == "" {
			t.Fatalf("fallback top action missing expected_effect: %#v", action)
		}
	}
}

func TestValidateDashboardNarrativePayload_RejectsUnknownNamesAndForbiddenRecommendations(t *testing.T) {
	build := dashboardNarrativeBuildResult{
		CandidateByID: map[string]dashboardNarrativeCandidate{
			"driver-stock-action": {
				ID:                  "driver-stock-action",
				Category:            "stock",
				CompatibleDriverIDs: []string{"driver-stock"},
				SupportingMetrics: []dashboardNarrativeMetric{
					{Key: "days_of_cover", Label: "Хватит примерно на", Value: "2 дня"},
				},
			},
		},
		MetricCatalogByKey: map[string]dashboardNarrativeMetric{
			"days_of_cover": {Key: "days_of_cover", Label: "Хватит примерно на", Value: "2 дня"},
		},
		AllowedQuotedNames: map[string]struct{}{
			"Товар A": {},
		},
	}
	drivers := []RiskDriver{
		{
			ID:            "driver-stock",
			Category:      "stock",
			CategoryLabel: "Остатки",
			Severity:      "critical",
			ImpactScore:   90,
			Title:         "Есть риск дефицита",
			Reason:        "Товар A заканчивается.",
		},
	}
	ctx := buildDashboardNarrativeValidationContext(drivers, build)
	payload := &dashboardNarrativePayload{
		Headline:           "Headline",
		ExecutiveSummary:   "Executive summary with enough words for validation.",
		ImprovementSummary: "Improvement summary",
		WhatChanged:        testValidWhatChangedPayload(),
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: "driver-stock", Category: "stock", Title: "Остаток под риском", Reason: "Товар «Несуществующий товар» скоро закончится."},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:             "driver-stock",
				SourceCandidateID:    "driver-stock-action",
				Priority:             "critical",
				Category:             "stock",
				Title:                "Усильте рекламу по товару",
				Reason:               "Остаток небольшой, но можно нарастить спрос.",
				NextStep:             "Сначала увеличьте рекламу по товару и поднимите бюджет.",
				ExpectedEffect:       "Снизить риск потери продаж из-за дефицита.",
				SupportingMetricRefs: []string{"days_of_cover"},
			},
		},
	}

	errs := validateDashboardNarrativePayload(payload, ctx)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	joined := strings.Join(errs, " ")
	if !strings.Contains(joined, "unknown_name") {
		t.Fatalf("errs=%v", errs)
	}
	if !strings.Contains(joined, "forbidden_recommendation") {
		t.Fatalf("errs=%v", errs)
	}
}

func TestValidateDashboardNarrativePayload_RejectsUnknownDriverAndMetricRef(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{
		DriversByID: map[string]RiskDriver{
			"driver-advertising": {
				ID:            "driver-advertising",
				Category:      "advertising",
				CategoryLabel: "Реклама",
				Severity:      "critical",
				ImpactScore:   98,
				Title:         "Слабая отдача рекламы",
				Reason:        "Есть проблемная реклама.",
			},
		},
		CandidatesByID: map[string]dashboardNarrativeCandidate{
			"driver-advertising-action": {
				ID:                  "driver-advertising-action",
				Category:            "advertising",
				CompatibleDriverIDs: []string{"driver-advertising"},
			},
		},
		MetricsByKey:      map[string]dashboardNarrativeMetric{},
		AllowedQuotedName: map[string]struct{}{},
	}
	payload := &dashboardNarrativePayload{
		Headline:           "Headline",
		ExecutiveSummary:   "Executive summary with enough words for validation.",
		ImprovementSummary: "Improvement summary",
		WhatChanged:        testValidWhatChangedPayload(),
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: "driver-unknown", Category: "advertising", Title: "Title", Reason: "Reason enough words"},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:             "driver-advertising",
				SourceCandidateID:    "driver-advertising-action",
				Priority:             "critical",
				Category:             "advertising",
				Title:                "Остановите кампанию без заказов",
				Reason:               "Кампания продолжает тратить бюджет без заказов.",
				NextStep:             "Сначала поставьте кампанию на паузу, затем проверьте запросы и товары.",
				ExpectedEffect:       "Снизить риск рекламного перерасхода.",
				SupportingMetricRefs: []string{"invented_metric"},
			},
		},
	}

	errs := validateDashboardNarrativePayload(payload, ctx)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	joined := strings.Join(errs, " ")
	if !strings.Contains(joined, "unknown_driver_id") {
		t.Fatalf("errs=%v", errs)
	}
	if !strings.Contains(joined, "unknown_metric_ref") {
		t.Fatalf("errs=%v", errs)
	}
}

func TestValidateDashboardNarrativePayload_AllowsCandidateBackedNumbersWithoutExplicitRefs(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{
		DriversByID: map[string]RiskDriver{
			"driver-stock": {
				ID:            "driver-stock",
				Category:      "stock",
				CategoryLabel: "Остатки",
				Severity:      "critical",
				ImpactScore:   90,
				Title:         "Есть риск дефицита",
				Reason:        "Часть товаров заканчивается.",
			},
		},
		CandidatesByID: map[string]dashboardNarrativeCandidate{
			"driver-stock-action": {
				ID:                  "driver-stock-action",
				Category:            "stock",
				CompatibleDriverIDs: []string{"driver-stock"},
				SupportingMetrics: []dashboardNarrativeMetric{
					{Key: "critical_stock_skus", Label: "0-3 дня", Value: "6"},
					{Key: "shortage_stock_skus", Label: "4-7 дней", Value: "5"},
				},
			},
		},
		MetricsByKey: map[string]dashboardNarrativeMetric{
			"critical_stock_skus": {Key: "critical_stock_skus", Label: "0-3 дня", Value: "6"},
			"shortage_stock_skus": {Key: "shortage_stock_skus", Label: "4-7 дней", Value: "5"},
		},
		AllowedQuotedName: map[string]struct{}{},
	}
	payload := &dashboardNarrativePayload{
		Headline:           "Headline",
		ExecutiveSummary:   "Executive summary with enough words for validation.",
		ImprovementSummary: "Improvement summary",
		WhatChanged:        testValidWhatChangedPayload(),
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: "driver-stock", Category: "stock", Title: "Остатки близки к дефициту", Reason: "Есть товары с коротким покрытием."},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-stock",
				SourceCandidateID: "driver-stock-action",
				Priority:          "critical",
				Category:          "stock",
				Title:             "Подтвердите пополнение по товарам с критичным остатком",
				Reason:            "Есть 6 артикулов с критичным остатком.",
				NextStep:          "Сначала проверьте 2 позиции с самым коротким покрытием и срок поставки.",
				ExpectedEffect:    "Снизить риск потери продаж из-за дефицита.",
			},
		},
	}

	errs := validateDashboardNarrativePayload(payload, ctx)
	joined := strings.Join(errs, " ")
	if strings.Contains(joined, "invented_metric_or_number") {
		t.Fatalf("errs=%v", errs)
	}
}

func TestBuildAllowedNarrativeQuotedNames_IncludesFunnelStages(t *testing.T) {
	dto := analytics.DashboardMetricsDTO{}
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			SalesFunnel: SalesFunnelWidget{
				Stages: []FunnelStage{
					{Label: "Показы"},
					{Label: "Просмотры карточки"},
					{Label: "Добавления в корзину"},
				},
			},
			FunnelAnalysis: FunnelAnalysisWidget{
				PrimaryDropStage: "Показы → Просмотры карточки",
			},
		},
	}

	got := buildAllowedNarrativeQuotedNames(dto, resp)
	for _, name := range []string{
		"Показы",
		"Просмотры карточки",
		"Добавления в корзину",
		"Показы → Просмотры карточки",
	} {
		if _, ok := got[name]; !ok {
			t.Fatalf("expected %q in allowed names: %#v", name, got)
		}
	}
}

func TestFactualContextHashIgnoresRenderedLabelDrift(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday: 120000,
			OrdersToday:  55,
		},
	}
	respReady := newDashboardResponseShellForMetrics()
	respPartial := newDashboardResponseShellForMetrics()
	respPartial.Status = StatusPartial
	respPartial.DataQualityNotes = []string{"Рекламные кампании пока недоступны или не синхронизированы."}

	buildReady := buildDashboardNarrativeContext(metricDate, dto, &respReady, nil, 70, "attention", nil, nil, nil, nil)
	buildPartial := buildDashboardNarrativeContext(metricDate, dto, &respPartial, nil, 70, "attention", nil, nil, nil, nil)

	if buildReady.FactualContextHash != buildPartial.FactualContextHash {
		t.Fatalf("factual hash must ignore rendered label drift: ready=%q partial=%q", buildReady.FactualContextHash, buildPartial.FactualContextHash)
	}
	if buildReady.ContextHash == buildPartial.ContextHash {
		t.Fatalf("rendered hash should still differ: %q", buildReady.ContextHash)
	}
	if !evaluateNarrativeFreshness(&storedDashboardNarrative{
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         buildReady.FactualContextHash,
		RenderedContextHash: buildReady.ContextHash,
		Payload:             &dashboardNarrativePayload{Headline: "ok"},
	}, buildPartial).Fresh {
		t.Fatal("factual hash match should keep narrative current despite rendered drift")
	}
}

func TestNarrativeFreshForCurrentContext(t *testing.T) {
	build := dashboardNarrativeBuildResult{ContextHash: "rendered", FactualContextHash: "ctx", RiskDriversHash: "drivers"}
	if narrativeFreshForCurrentContext(nil, build) {
		t.Fatal("nil narrative must be stale")
	}
	if narrativeFreshForCurrentContext(&storedDashboardNarrative{
		Status:          "completed",
		PromptVersion:   DashboardNarrativePromptVersion,
		ContextHash:     "ctx",
		RiskDriversHash: "drivers",
	}, build) {
		t.Fatal("missing payload must be stale")
	}
	if !narrativeFreshForCurrentContext(&storedDashboardNarrative{
		Status:          "completed",
		PromptVersion:   DashboardNarrativePromptVersion,
		ContextHash:     "ctx",
		RiskDriversHash: "drivers",
		Payload:         &dashboardNarrativePayload{Headline: "ok", TopActions: []dashboardNarrativeTopAction{{DriverID: "d1", Title: "t", Reason: "r", NextStep: "n", ExpectedEffect: "e", Category: "sales"}}},
	}, build) {
		t.Fatal("matching narrative should be fresh")
	}
	if narrativeFreshForCurrentContext(&storedDashboardNarrative{
		Status:          "completed",
		PromptVersion:   "dashboard_narrative_v5",
		ContextHash:     "ctx",
		RiskDriversHash: "drivers",
		Payload:         &dashboardNarrativePayload{Headline: "ok"},
	}, build) {
		t.Fatal("old prompt version must be stale")
	}
}

func TestValidateDashboardNarrativePayload_RejectsImprovementSummaryAboutLackOfImprovements(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{
		DriversByID: map[string]RiskDriver{
			"driver-advertising": {ID: "driver-advertising", Category: "advertising"},
		},
		CandidatesByID: map[string]dashboardNarrativeCandidate{
			"driver-advertising-action": {ID: "driver-advertising-action", Category: "advertising", CompatibleDriverIDs: []string{"driver-advertising"}},
		},
		MetricsByKey:      map[string]dashboardNarrativeMetric{},
		AllowedQuotedName: map[string]struct{}{},
	}
	payload := &dashboardNarrativePayload{
		Headline:           "Headline",
		ExecutiveSummary:   "Executive summary with enough words for validation.",
		ImprovementSummary: "Заметных улучшений по сравнению с прошлой сводкой не выявлено.",
		WhatChanged:        testValidWhatChangedPayload(),
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: "driver-advertising", Category: "advertising", Title: "Реклама", Reason: "Кампания тратит бюджет без заказов."},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Сократите расход",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу и проверьте запросы.",
				ExpectedEffect:    "Снизить риск рекламного перерасхода.",
			},
		},
	}
	errs := validateDashboardNarrativePayload(payload, ctx)
	joined := strings.Join(errs, " ")
	if !strings.Contains(joined, "improvement_summary_about_lack_of_improvements") {
		t.Fatalf("errs=%v", errs)
	}
}

func TestParseDashboardNarrativePayload_PreservesExpectedEffect(t *testing.T) {
	raw := `{
	  "headline":"Headline",
	  "executive_summary":"Executive summary with enough words for validation.",
	  "improvement_summary":"Improvement summary",
	  "health_reasons":[],
	  "top_actions":[
	    {
	      "driver_id":"driver-advertising",
	      "source_candidate_id":"driver-advertising-action",
	      "priority":"critical",
	      "category":"advertising",
	      "title":"Сократите расход",
	      "reason":"Кампания тратит бюджет без заказов.",
	      "next_step":"Поставьте кампанию на паузу и проверьте запросы.",
	      "expected_effect":"Снизить риск рекламного перерасхода.",
	      "supporting_metric_refs":["ad_spend"]
	    }
	  ]
	}`
	payload, err := parseDashboardNarrativePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if payload.TopActions[0].ExpectedEffect != "Снизить риск рекламного перерасхода." {
		t.Fatalf("expected_effect=%q", payload.TopActions[0].ExpectedEffect)
	}
}

func TestValidateDashboardNarrativePayload_RejectsEmptyExpectedEffect(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{
		DriversByID: map[string]RiskDriver{
			"driver-advertising": {ID: "driver-advertising", Category: "advertising"},
		},
		CandidatesByID: map[string]dashboardNarrativeCandidate{
			"driver-advertising-action": {ID: "driver-advertising-action", Category: "advertising", CompatibleDriverIDs: []string{"driver-advertising"}},
		},
		MetricsByKey:      map[string]dashboardNarrativeMetric{},
		AllowedQuotedName: map[string]struct{}{},
	}
	payload := &dashboardNarrativePayload{
		Headline:           "Headline",
		ExecutiveSummary:   "Executive summary with enough words for validation.",
		ImprovementSummary: "Improvement summary",
		WhatChanged:        testValidWhatChangedPayload(),
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: "driver-advertising", Category: "advertising", Title: "Реклама", Reason: "Кампания тратит бюджет без заказов."},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Сократите расход",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу и проверьте запросы.",
			},
		},
	}
	errs := validateDashboardNarrativePayload(payload, ctx)
	joined := strings.Join(errs, " ")
	if !strings.Contains(joined, "top_action_0_empty_text") {
		t.Fatalf("errs=%v", errs)
	}
}

func TestApplyUnifiedAccountHealth_DoesNotApplyV5NarrativeWithoutExpectedEffect(t *testing.T) {
	metricDate := mustRiskDate(t, "2026-05-21")
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{
			RevenueToday:     120000,
			RevenueDayDelta:  analytics.DashboardDelta{Pct: riskFloatPtr(-0.11)},
			RevenueWeekDelta: analytics.DashboardDelta{Pct: riskFloatPtr(-0.08)},
			OrdersToday:      55,
			OrdersYesterday:  63,
		},
	}
	resp := Response{
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: pendingWidget("")},
			AdEfficiency: AdEfficiencyWidget{Points: []AdEfficiencyPoint{
				{CampaignExternalID: 21, CampaignName: "Холодный трафик", Spend: 42000, Revenue: 0, Orders: 0, EfficiencySignal: "growth_without_result"},
			}},
		},
		SKUFocus: SKUFocus{
			StockRisk: []SKUFocusCard{}, Surplus: []SKUFocusCard{},
			OrderDrop: []SKUFocusCard{}, NoSalesWithStock: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
	}
	actionPool := []TopActionItem{}
	drivers := buildRiskDrivers(metricDate, dto, &resp, nil, nil)
	score := computeHealthScoreBreakdown(drivers).FinalScore
	severity := computeHealthSeverity(score)
	build := buildDashboardNarrativeContext(metricDate, dto, &resp, drivers, score, severity, nil, actionPool, nil, nil)

	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		ID:                  "narrative-v5",
		Status:              "completed",
		PromptVersion:       "dashboard_narrative_v5",
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload: &dashboardNarrativePayload{
			Headline:           "Главный риск дня в рекламе.",
			ExecutiveSummary:   "Сильнее всего индекс давит реклама.",
			ImprovementSummary: "Индекс улучшится после сокращения рекламных расходов.",
			WhatChanged:        testValidWhatChangedPayload(),
			HealthReasons: []dashboardNarrativeHealthReason{
				{
					DriverID: "driver-advertising",
					Category: "advertising",
					Title:    "Реклама тратит бюджет без результата",
					Reason:   "Кампания «Холодный трафик» уже тратит деньги без заказов.",
				},
			},
			TopActions: []dashboardNarrativeTopAction{
				{
					DriverID:          "driver-advertising",
					SourceCandidateID: "driver-advertising-action",
					Priority:          "critical",
					Category:          "advertising",
					Title:             "Сократите расход по кампании без заказов",
					Reason:            "Кампания «Холодный трафик» продолжает расход без результата.",
					NextStep:          "Сначала поставьте кампанию на паузу.",
				},
			},
			SellerPulseNarrative: sampleSellerPulseNarrativeForTests(),
		},
	}

	applyUnifiedAccountHealth(metricDate, dto, &resp, nil, nil, actionPool, nil, nil)

	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "fallback" {
		t.Fatalf("source=%q want fallback", resp.AIDiagnostics.AccountSummary.NarrativeSource)
	}
	if resp.AIDiagnostics.AccountSummary.Freshness != "fallback" {
		t.Fatalf("freshness=%q want fallback", resp.AIDiagnostics.AccountSummary.Freshness)
	}
	if resp.AIDiagnostics.AccountSummary.ValidationStatus != "fallback_stale" {
		t.Fatalf("validation_status=%q", resp.AIDiagnostics.AccountSummary.ValidationStatus)
	}
	for _, action := range resp.AIDiagnostics.TopActions {
		if action.Source == "ai_summary" {
			t.Fatalf("v5 narrative must not expose AI narrative top actions: %#v", action)
		}
	}
}

func TestValidateDashboardNarrativePayload_RejectsUnbackedNumberInExpectedEffect(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{
		DriversByID: map[string]RiskDriver{
			"driver-advertising": {ID: "driver-advertising", Category: "advertising"},
		},
		CandidatesByID: map[string]dashboardNarrativeCandidate{
			"driver-advertising-action": {
				ID:                  "driver-advertising-action",
				Category:            "advertising",
				CompatibleDriverIDs: []string{"driver-advertising"},
			},
		},
		MetricsByKey:      map[string]dashboardNarrativeMetric{},
		AllowedQuotedName: map[string]struct{}{},
	}
	payload := &dashboardNarrativePayload{
		Headline:           "Headline",
		ExecutiveSummary:   "Executive summary with enough words for validation.",
		ImprovementSummary: "Improvement summary",
		WhatChanged:        testValidWhatChangedPayload(),
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: "driver-advertising", Category: "advertising", Title: "Реклама", Reason: "Кампания тратит бюджет без заказов."},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID:          "driver-advertising",
				SourceCandidateID: "driver-advertising-action",
				Priority:          "critical",
				Category:          "advertising",
				Title:             "Сократите расход",
				Reason:            "Кампания тратит бюджет без заказов.",
				NextStep:          "Поставьте кампанию на паузу и проверьте запросы.",
				ExpectedEffect:    "Снизить расход на 15% уже сегодня.",
			},
		},
	}
	errs := validateDashboardNarrativePayload(payload, ctx)
	joined := strings.Join(errs, " ")
	if !strings.Contains(joined, "top_action_0_invented_metric_or_number") {
		t.Fatalf("errs=%v", errs)
	}
}

func mustNarrativeDate(t *testing.T, iso string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		t.Fatal(err)
	}
	return d.UTC()
}
