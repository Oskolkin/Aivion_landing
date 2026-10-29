package dashboardhome

import (
	"encoding/json"
	"testing"
)

func TestDashboardHomeResponseContractTopLevelBlocks(t *testing.T) {
	resp := Response{
		Status: StatusNotEnoughData,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			WhatChanged:    WhatChangedWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}, Groups: []WhatChangedGroup{}},
			TopActions:     []TopActionItem{},
			SalesFunnel:    SalesFunnelWidget{WidgetBase: WidgetBase{Status: WidgetSourceNotAvailable}},
			FunnelAnalysis: FunnelAnalysisWidget{
				WidgetBase: WidgetBase{Status: WidgetNotEnoughData},
				AIAnalysis: FunnelAIAnalysisWidget{
					WidgetBase:       WidgetBase{Status: WidgetNotEnoughData},
					RelatedSignals:   []string{},
					RecommendedFocus: []string{},
				},
			},
			StockCoverage:    StockCoverageWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			AdEfficiency:     AdEfficiencyWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
			RiskDistribution: RiskDistributionWidget{WidgetBase: WidgetBase{Status: WidgetNotEnoughData}},
		},
		SellerPulse: SellerPulse{KPIs: []KPICard{}, Trend7D: []TrendDayPoint{}},
		SKUFocus: SKUFocus{
			OrderDrop: []SKUFocusCard{}, StockRisk: []SKUFocusCard{},
			NoSalesWithStock: []SKUFocusCard{}, Surplus: []SKUFocusCard{}, KeySKUs: []KeySkuCard{},
		},
		RecommendedActions: []ActionItem{},
		DataQualityNotes:   []string{},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"metric_date", "status", "data_quality_notes", "ai_diagnostics", "seller_pulse",
		"sku_focus", "recommended_actions",
	} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing top-level key %q", key)
		}
	}
	ai, ok := m["ai_diagnostics"].(map[string]any)
	if !ok {
		t.Fatal("ai_diagnostics must be object")
	}
	wc, ok := ai["what_changed"].(map[string]any)
	if !ok {
		t.Fatal("ai_diagnostics.what_changed must be object")
	}
	groupsRaw, hasGroups := wc["groups"]
	if !hasGroups || groupsRaw == nil {
		t.Fatal("ai_diagnostics.what_changed.groups must be present (use empty array when no data)")
	}
	for _, w := range []string{
		"account_summary", "what_changed", "top_actions", "category_summaries",
		"sales_funnel", "funnel_analysis", "stock_coverage", "ad_efficiency", "risk_distribution",
	} {
		if _, ok := ai[w]; !ok {
			t.Fatalf("missing ai_diagnostics.%s", w)
		}
	}
	funnelAnalysis, ok := ai["funnel_analysis"].(map[string]any)
	if !ok {
		t.Fatal("funnel_analysis must be object")
	}
	if _, ok := funnelAnalysis["ai_analysis"]; !ok {
		t.Fatal("missing funnel_analysis.ai_analysis")
	}
}

func TestWidgetStatusConstants(t *testing.T) {
	statuses := []WidgetStatus{
		WidgetAvailable, WidgetPartial, WidgetNotEnoughData, WidgetSourceNotAvailable,
		WidgetSourceAccessDenied, WidgetPremiumRequired, WidgetSourcePending, WidgetError,
	}
	seen := make(map[WidgetStatus]struct{})
	for _, st := range statuses {
		if st == "" {
			t.Fatal("empty widget status")
		}
		seen[st] = struct{}{}
	}
	if len(seen) != len(statuses) {
		t.Fatal("duplicate widget status constants")
	}
}
