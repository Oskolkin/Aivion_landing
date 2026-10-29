package dashboardhome

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestAdEfficiencyNarrative_RejectsSKULevelWording(t *testing.T) {
	n := &AdEfficiencyNarrative{
		Summary: "Кампания Alpha тратит бюджет без заказов.",
		Checklist: []AdEfficiencyCheckItem{
			{Title: "Проверить SKU сливает бюджет", Text: "Остановите расход по SKU"},
		},
	}
	repairAdEfficiencyNarrative(n)
	if len(n.Checklist) != 0 {
		t.Fatalf("expected SKU checklist dropped, got %+v", n.Checklist)
	}
}

func TestAdEfficiencyNarrative_AcceptsValidPayload(t *testing.T) {
	delta := 6.0
	n := &AdEfficiencyNarrative{
		Summary: "ДРР выше среднего за период; две кампании без заказов держат расход.",
		Checklist: []AdEfficiencyCheckItem{
			{
				Title:    "Проверить кампании с высоким ДРР",
				Text:     "Сверьте ставки и релевантность трафика по кампаниям из списка high DRR.",
				Priority: "high",
				Target:   "high_drr_campaigns",
			},
		},
		Limitations: []string{"campaign_level_only"},
	}
	ctx := adEfficiencyNarrativeValidationContext{
		WidgetStatus:      WidgetAvailable,
		HasFactualData:    true,
		DeltaVsPreviousPP: &delta,
	}
	if errs := validateAdEfficiencyNarrativePayload(n, ctx); len(errs) != 0 {
		t.Fatalf("expected valid narrative, got %v", errs)
	}
}

func TestAdEfficiencyNarrative_RejectsDirectionalConflict(t *testing.T) {
	delta := -5.2
	n := &AdEfficiencyNarrative{
		Summary: "Рост DRR к вчера связан с кампаниями с высоким расходом.",
		Checklist: []AdEfficiencyCheckItem{
			{
				Title:    "Проверить кампании с высоким ДРР",
				Text:     "Сверьте ставки и релевантность трафика.",
				Priority: "high",
				Target:   "high_drr_campaigns",
			},
		},
	}
	ctx := adEfficiencyNarrativeValidationContext{
		WidgetStatus:      WidgetAvailable,
		HasFactualData:    true,
		DeltaVsPreviousPP: &delta,
	}
	errs := validateAdEfficiencyNarrativePayload(n, ctx)
	hasDirectionalErr := false
	for _, err := range errs {
		if err == "ad_efficiency_summary_direction_conflict" {
			hasDirectionalErr = true
			break
		}
	}
	if !hasDirectionalErr {
		t.Fatalf("expected direction conflict error, got %v", errs)
	}
}

func TestAdEfficiencyNarrative_RejectsOppositeDirectionalConflict(t *testing.T) {
	delta := 4.7
	n := &AdEfficiencyNarrative{
		Summary: "ДРР снизился к вчера, но остаются кампании с высоким расходом.",
		Checklist: []AdEfficiencyCheckItem{
			{
				Title:    "Проверить кампании с высоким ДРР",
				Text:     "Сверьте ставки и релевантность трафика.",
				Priority: "high",
				Target:   "high_drr_campaigns",
			},
		},
	}
	ctx := adEfficiencyNarrativeValidationContext{
		WidgetStatus:      WidgetAvailable,
		HasFactualData:    true,
		DeltaVsPreviousPP: &delta,
	}
	errs := validateAdEfficiencyNarrativePayload(n, ctx)
	hasDirectionalErr := false
	for _, err := range errs {
		if err == "ad_efficiency_summary_direction_conflict" {
			hasDirectionalErr = true
			break
		}
	}
	if !hasDirectionalErr {
		t.Fatalf("expected direction conflict error, got %v", errs)
	}
}

func TestAdEfficiencyNarrative_AcceptsNeutralDirectionalSummary(t *testing.T) {
	delta := -0.1
	n := &AdEfficiencyNarrative{
		Summary: "ДРР близок к предыдущему дню, но несколько кампаний остаются в зоне внимания.",
		Checklist: []AdEfficiencyCheckItem{
			{
				Title:    "Проверить кампании без заказов",
				Text:     "Проверьте трафик и запросы в кампаниях без заказов.",
				Priority: "medium",
				Target:   "no_order_campaigns",
			},
		},
	}
	ctx := adEfficiencyNarrativeValidationContext{
		WidgetStatus:      WidgetAvailable,
		HasFactualData:    true,
		DeltaVsPreviousPP: &delta,
	}
	if errs := validateAdEfficiencyNarrativePayload(n, ctx); len(errs) != 0 {
		t.Fatalf("expected no direction conflict, got %v", errs)
	}
}

func TestApplyAdEfficiencyNarrativeToResponse_ConflictingSummaryFallsBackNeutral(t *testing.T) {
	delta := -0.3
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{
				WidgetBase: WidgetBase{Status: WidgetAvailable},
				Summary: &AdEfficiencySummary{
					DeltaVsPreviousPp: &delta,
				},
			},
		},
	}
	payload := &dashboardNarrativePayload{
		AdEfficiencyNarrative: &AdEfficiencyNarrative{
			Summary: "Рост DRR связан с проблемными кампаниями.",
			Checklist: []AdEfficiencyCheckItem{
				{
					Title:    "Проверить кампании с высоким ДРР",
					Text:     "Проверьте ставки и запросы.",
					Priority: "high",
					Target:   "high_drr_campaigns",
				},
			},
		},
	}
	applyAdEfficiencyNarrativeToResponse(resp, true, payload)
	if resp.AIDiagnostics.AdEfficiency.Narrative == nil {
		t.Fatal("expected narrative result")
	}
	if resp.AIDiagnostics.AdEfficiency.Narrative.Status != "partial" {
		t.Fatalf("expected partial for conflicting summary, got %q", resp.AIDiagnostics.AdEfficiency.Narrative.Status)
	}
	if resp.AIDiagnostics.AdEfficiency.Narrative.Summary != "" {
		t.Fatalf("expected no conflicting summary in response, got %q", resp.AIDiagnostics.AdEfficiency.Narrative.Summary)
	}
}

func TestAdEfficiencyNarrative_DropsGenericChecklist(t *testing.T) {
	n := &AdEfficiencyNarrative{
		Summary: "Рекламный блок показывает рост ДРР относительно среднего за доступные дни.",
		Checklist: []AdEfficiencyCheckItem{
			{Title: "Проверьте всё", Text: "Разберите ситуацию"},
		},
	}
	repairAdEfficiencyNarrative(n)
	if len(n.Checklist) != 0 {
		t.Fatalf("expected generic checklist dropped, got %+v", n.Checklist)
	}
}

func TestApplyAdEfficiencyNarrative_NoDeterministicFallback(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{
				WidgetBase: WidgetBase{Status: WidgetAvailable},
				Period:     &AdEfficiencyPeriod{MetricDate: "2026-03-10", SampleDays: 3},
				Summary:    &AdEfficiencySummary{HighDRRThresholdPct: analytics.AdEfficiencyHighDRRThresholdPct},
			},
		},
	}
	applyAdEfficiencyNarrativeToResponse(resp, false, nil)
	if resp.AIDiagnostics.AdEfficiency.Narrative == nil {
		t.Fatal("expected narrative status object")
	}
	if resp.AIDiagnostics.AdEfficiency.Narrative.Status != "not_available" {
		t.Fatalf("got status %q", resp.AIDiagnostics.AdEfficiency.Narrative.Status)
	}
	if resp.AIDiagnostics.AdEfficiency.Narrative.Summary == "" {
		t.Fatal("expected explicit not_available reason")
	}
}

func TestAdEfficiencyHomeResponseContract(t *testing.T) {
	drr := 25.0
	resp := Response{
		MetricDate: "2026-03-10",
		Status:     StatusReady,
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{
				WidgetBase: WidgetBase{Status: WidgetAvailable},
				Period: &AdEfficiencyPeriod{
					MetricDate: "2026-03-10",
					DateFrom:   "2026-03-04",
					DateTo:     "2026-03-10",
					SampleDays: 5,
				},
				Summary: &AdEfficiencySummary{
					CurrentDRRPct:              &drr,
					HighDRRThresholdPct:        100,
					SpendWithoutOrdersDateFrom: "2026-03-04",
					SpendWithoutOrdersDateTo:   "2026-03-10",
				},
				Trend7D:          []AdEfficiencyTrendPoint{{Date: "2026-03-10", DataStatus: "available"}},
				HighDRRCampaigns: []AdCampaignDisplayRow{{CampaignExternalID: 1, CampaignName: "A", ReasonCode: "high_drr"}},
				NoOrderCampaigns: []AdCampaignDisplayRow{{CampaignExternalID: 2, CampaignName: "B", ReasonCode: "no_orders"}},
				Narrative:        &AdEfficiencyNarrative{Status: "pending"},
				CTA:              &AdEfficiencyCTA{Label: adEfficiencyCTALabel, Href: adEfficiencyCTAHref},
				Points:           []AdEfficiencyPoint{{CampaignExternalID: 1}},
			},
		},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	ai := root["ai_diagnostics"].(map[string]any)
	ad := ai["ad_efficiency"].(map[string]any)
	for _, key := range []string{
		"period", "summary", "trend_7d", "high_drr_campaigns", "no_order_campaigns",
		"ad_efficiency_narrative", "cta",
	} {
		if _, ok := ad[key]; !ok {
			t.Fatalf("missing ad_efficiency.%s", key)
		}
	}
	if _, ok := ad["sku_risks"]; ok {
		t.Fatal("sku_risks must not appear in dashboard ad_efficiency")
	}
	summary := ad["summary"].(map[string]any)
	if _, ok := summary["current_drr_pct"]; !ok {
		t.Fatal("missing summary.current_drr_pct")
	}
}

func TestBuildAdEfficiencyNarrativeContext_CampaignLevelOnly(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency: AdEfficiencyWidget{
				WidgetBase: WidgetBase{Status: WidgetAvailable},
				Period:     &AdEfficiencyPeriod{MetricDate: "2026-03-10", SampleDays: 2},
				Summary: &AdEfficiencySummary{
					CampaignsHighDRRCount:  1,
					CampaignsNoOrdersCount: 1,
					SpendWithoutOrders:     50,
				},
				HighDRRCampaigns: []AdCampaignDisplayRow{{CampaignName: "Alpha", Spend: 100, Revenue: 50, DRRPct: ptrFloat64(200)}},
				NoOrderCampaigns: []AdCampaignDisplayRow{{CampaignName: "Beta", Spend: 50, Orders: 0}},
			},
		},
	}
	ctx := buildAdEfficiencyNarrativeContextMap(resp)
	inner, ok := ctx["ad_efficiency"].(map[string]any)
	if !ok {
		t.Fatal("missing ad_efficiency key")
	}
	if inner["campaigns_high_drr_count"] != 1 {
		t.Fatalf("unexpected context: %+v", inner)
	}
	lims, ok := inner["limitations"].([]string)
	if !ok || len(lims) == 0 || lims[0] != "campaign_level_only" {
		t.Fatalf("expected campaign_level_only limitation, got %+v", inner["limitations"])
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
