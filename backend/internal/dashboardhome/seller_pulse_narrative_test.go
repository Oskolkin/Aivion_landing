package dashboardhome

import (
	"encoding/json"
	"testing"
)

func sampleSellerPulseNarrativeForTests() *SellerPulseNarrative {
	return &SellerPulseNarrative{
		Tone:     "stable",
		Headline: "Ключевые показатели в пределах ожидаемой динамики.",
		Summary:  "Продолжайте мониторинг продаж и рисков.",
		ImportantNotes: []SellerPulseImportantNote{
			{Text: "Критичных ухудшений по доступным метрикам не видно."},
		},
		QuickActions: []SellerPulseQuickAction{
			{Label: "Продолжить мониторинг ключевых SKU", Kind: "action", Href: "/app/actions"},
		},
	}
}

func TestValidateSampleSellerPulseNarrativePasses(t *testing.T) {
	ctx := sellerPulseNarrativeValidationContext{
		KPIByKey:      buildSellerPulseKPIFacts(nil),
		CandidateByID: map[string]dashboardNarrativeCandidate{},
		AllowedHrefs:  sellerPulseAllowedHrefs,
	}
	errs := validateSellerPulseNarrativePayload(sampleSellerPulseNarrativeForTests(), ctx)
	if len(errs) > 0 {
		t.Fatalf("validation errors: %v", errs)
	}
}

func TestSellerPulseNarrativeFallbackAndReady(t *testing.T) {
	resp := Response{
		Status: StatusReady,
		SellerPulse: SellerPulse{
			KPIs: []KPICard{
				{Key: "revenue", Label: "Выручка", Value: "80 000 ₽", Delta: &KPICardDelta{Abs: -20000, Pct: riskFloatPtr(-20), Label: "к вчера"}},
				{Key: "orders", Label: "Заказы", Value: "20", Delta: &KPICardDelta{Abs: -8, Label: "к вчера"}},
				{Key: "drr", Label: "ДРР", Value: "42.0%"},
				{Key: "risks", Label: "Критичные риски", Value: "4"},
			},
		},
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{},
		},
	}

	applySellerPulseNarrativeToResponse(&resp, nil)
	if resp.SellerPulse.Narrative == nil {
		t.Fatal("expected fallback narrative")
	}
	if resp.SellerPulse.Narrative.Status != "pending" {
		t.Fatalf("expected pending, got %s", resp.SellerPulse.Narrative.Status)
	}
	if resp.SellerPulse.Narrative.Headline == "" {
		t.Fatal("expected headline")
	}
	if len(resp.SellerPulse.Narrative.ImportantNotes) == 0 {
		t.Fatal("expected important notes")
	}
	if len(resp.SellerPulse.Narrative.QuickActions) == 0 {
		t.Fatal("expected quick actions")
	}

	build := dashboardNarrativeBuildResult{
		ContextHash:        "ctx",
		FactualContextHash: "factual-ctx",
		RiskDriversHash:    "drivers",
		CandidateByID:      map[string]dashboardNarrativeCandidate{},
		SellerPulseCtx:     buildSellerPulseNarrativeValidationContext(resp.SellerPulse.KPIs, dashboardNarrativeBuildResult{}),
	}
	resp.AIDiagnostics.AccountSummary.storedNarrative = &storedDashboardNarrative{
		Status:              "completed",
		PromptVersion:       DashboardNarrativePromptVersion,
		ContextHash:         build.FactualContextHash,
		RenderedContextHash: build.ContextHash,
		RiskDriversHash:     build.RiskDriversHash,
		Payload: &dashboardNarrativePayload{
			Headline:           "ignored",
			ExecutiveSummary:   "ignored",
			ImprovementSummary: "ignored",
			SellerPulseNarrative: &SellerPulseNarrative{
				Tone:     "critical",
				Headline: "AI headline for seller pulse",
				Summary:  "AI summary for seller pulse",
				ImportantNotes: []SellerPulseImportantNote{
					{Text: "AI note one", MetricRefs: []string{"revenue"}},
					{Text: "AI note two", MetricRefs: []string{"drr"}},
				},
				QuickActions: []SellerPulseQuickAction{
					{Label: "Проверить рекламу", Kind: "action", Href: "/app/actions?action=ad-1", Priority: "high"},
				},
				Chips: []SellerPulseChip{{Label: "ДРР: критично", Tone: "critical"}},
			},
		},
	}

	applySellerPulseNarrativeToResponse(&resp, &build)
	if resp.SellerPulse.Narrative.Status != "ready" {
		t.Fatalf("expected ready, got %s", resp.SellerPulse.Narrative.Status)
	}
	if resp.SellerPulse.Narrative.Headline != "AI headline for seller pulse" {
		t.Fatalf("unexpected headline: %q", resp.SellerPulse.Narrative.Headline)
	}
}

func TestSellerPulseNarrativeResponseSerialization(t *testing.T) {
	resp := Response{
		Status: StatusReady,
		SellerPulse: SellerPulse{
			KPIs: []KPICard{{Key: "revenue", Label: "Выручка", Value: "10 000 ₽"}},
			Narrative: &SellerPulseNarrative{
				Status:   "ready",
				Tone:     "stable",
				Headline: "Стабильный день",
				Summary:  "Ключевые показатели без критичных ухудшений.",
				ImportantNotes: []SellerPulseImportantNote{
					{Text: "Выручка в пределах ожидаемой динамики."},
				},
				QuickActions: []SellerPulseQuickAction{
					{Label: "Продолжить мониторинг", Kind: "action", Href: "/app/actions"},
				},
			},
		},
		AIDiagnostics: AIDiagnostics{
			AccountSummary:    AccountSummaryWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}},
			WhatChanged:       WhatChangedWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}, Groups: []WhatChangedGroup{}},
			TopActions:        []TopActionItem{},
			SalesFunnel:       SalesFunnelWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			FunnelAnalysis:    FunnelAnalysisWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			StockCoverage:     StockCoverageWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			AdEfficiency:      AdEfficiencyWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			RiskDistribution:  RiskDistributionWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			CategorySummaries: []CategorySummaryCard{},
		},
		SKUFocus: SKUFocus{
			OrderDrop: []SKUFocusCard{}, StockRisk: []SKUFocusCard{},
			NoSalesWithStock: []SKUFocusCard{}, Surplus: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
		RecommendedActions: []ActionItem{},
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	sp, ok := decoded["seller_pulse"].(map[string]any)
	if !ok {
		t.Fatal("seller_pulse missing")
	}
	narrative, ok := sp["narrative"].(map[string]any)
	if !ok {
		t.Fatal("seller_pulse.narrative missing")
	}
	if narrative["headline"] != "Стабильный день" {
		t.Fatalf("unexpected headline: %v", narrative["headline"])
	}
	if _, ok := narrative["raw_ai_response"]; ok {
		t.Fatal("must not expose raw ai response")
	}
}
