package dashboardhome

import (
	"strings"
	"testing"
)

func TestApplyWhatChanged_Run586ShapeMissingGroupSummaries(t *testing.T) {
	// Mirrors validated AI output for seller1 run 586: home keys present, but
	// group summaries omitted while items carry the narrative. Repair must fill
	// summaries so the widget is available (not source_not_available).
	pct := 3.0
	resp := &Response{
		MetricDate: "2026-07-08",
		SellerPulse: SellerPulse{
			KPIs: []KPICard{
				{Key: "revenue", Label: "Revenue", Value: "147203", Delta: &KPICardDelta{Abs: 1000, Pct: &pct, Label: "vs prev"}},
			},
		},
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{ExecutiveSummary: "Sales stable, ads inefficient."},
		},
	}
	payload := &dashboardNarrativePayload{
		WhatChanged: &dashboardNarrativeWhatChanged{
			Summary:        "Stable revenue, SKU order drops and high DRR on selected campaigns.",
			MainConclusion: "Review inefficient campaigns and address SKU order drops.",
			Groups: []dashboardNarrativeWhatChangedGroup{
				{
					Key: whatChangedKeyWorsened, Label: "Worsened", Tone: "negative",
					Items: []dashboardNarrativeWhatChangedItem{
						{Label: "SKU order drop", Text: "SKU 801000100031 and 801000100004 lost orders over 7 days.", Highlights: []string{"801000100031", "801000100004"}},
						{Label: "Ad DRR rise", Text: "Campaign 882000021528 has DRR 57.8%.", Highlights: []string{"882000021528"}},
					},
				},
				{
					Key: whatChangedKeyNewRisks, Label: "New risks", Tone: "warning",
					Items: []dashboardNarrativeWhatChangedItem{
						{Label: "Ads without orders", Text: "Campaign 882000041542 spent 10962 with zero orders.", Highlights: []string{"882000041542"}},
					},
				},
				{
					Key: whatChangedKeyUnchangedRisks, Label: "Active", Tone: "neutral",
					Items: []dashboardNarrativeWhatChangedItem{
						{Label: "Key SKU concentration", Text: "Top-10 SKU still contribute over 46% of revenue.", Highlights: []string{"46"}},
					},
				},
				{
					Key: whatChangedKeyImproved, Label: "Improved", Tone: "positive",
					EmptyText: "No positive changes were recorded in the period.",
					Items:     []dashboardNarrativeWhatChangedItem{},
				},
			},
		},
	}
	emptyBundle := buildWhatChangedValidationBundleFromPeriodComparison(dashboardNarrativePeriodComparison{
		CurrentMetricDate: "2026-07-08",
		Facts:             []periodComparisonFact{},
	}, map[string]struct{}{})
	applyWhatChangedToResponse(resp, payload, emptyBundle, true)
	if resp.AIDiagnostics.WhatChanged.Status != WidgetAvailable {
		t.Fatalf("status=%s message=%q want available", resp.AIDiagnostics.WhatChanged.Status, resp.AIDiagnostics.WhatChanged.Message)
	}
	if len(resp.AIDiagnostics.WhatChanged.Groups) != 4 {
		t.Fatalf("groups=%d", len(resp.AIDiagnostics.WhatChanged.Groups))
	}
	for _, g := range resp.AIDiagnostics.WhatChanged.Groups {
		if g.Key == whatChangedKeyImproved {
			continue
		}
		if strings.TrimSpace(g.Summary) == "" && len(g.Items) > 0 {
			t.Fatalf("group %s still missing summary with %d items", g.Key, len(g.Items))
		}
	}
}

func TestApplyWhatChanged_UnifiedHomeWithSellerPulseDeltas(t *testing.T) {
	pct3 := 3.0
	pct4 := 4.12
	resp := &Response{
		MetricDate: "2026-07-08",
		SellerPulse: SellerPulse{
			KPIs: []KPICard{
				{Key: "revenue", Label: "Revenue", Value: "147203", Delta: &KPICardDelta{Abs: 3926.5, Pct: &pct3, Label: "vs prev"}},
				{Key: "orders", Label: "Orders", Value: "101", Delta: &KPICardDelta{Abs: 4, Pct: &pct4, Label: "vs prev"}},
			},
		},
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{ExecutiveSummary: "Sales grew while ads stayed inefficient."},
		},
	}
	payload := &dashboardNarrativePayload{
		WhatChanged: &dashboardNarrativeWhatChanged{
			Summary:        "Sales and revenue grew, but new ad and SKU risks appeared.",
			MainConclusion: "Business keeps growing while part of the budget goes into inefficient campaigns.",
			Groups: []dashboardNarrativeWhatChangedGroup{
				{
					Key: whatChangedKeyWorsened, Label: "Worsened", Tone: "negative",
					Summary: "New DRR problems and SKU order drops appeared.",
					Items: []dashboardNarrativeWhatChangedItem{
						{Label: "Campaign DRR", Text: "DRR rose above 1000% on several campaigns.", Highlights: []string{"1067%"}},
						{Label: "SKU order drop", Text: "5 SKU showed a sharp weekly order drop.", Highlights: []string{"-4", "-1"}},
					},
				},
				{
					Key: whatChangedKeyNewRisks, Label: "New risks", Tone: "warning",
					Summary: "Ad efficiency is extremely low.",
					Items: []dashboardNarrativeWhatChangedItem{
						{Label: "Inefficient ads", Text: "Campaign spend barely drives orders.", Highlights: []string{"campaign 882000071563"}},
					},
				},
				{
					Key: whatChangedKeyUnchangedRisks, Label: "Active", Tone: "neutral",
					Summary:   "No unresolved warehouse or category-wide issues.",
					EmptyText: "No active problems outside ads and selected SKU.",
					Items:     []dashboardNarrativeWhatChangedItem{},
				},
				{
					Key: whatChangedKeyImproved, Label: "Improved", Tone: "positive",
					Summary:   "Overall sales and revenue keep growing.",
					EmptyText: "No material improvements in this period.",
					Items: []dashboardNarrativeWhatChangedItem{
						{Label: "Sales growth", Text: "Sales grew by 9% over the period.", Highlights: []string{"+61 orders"}},
						{Label: "Revenue growth", Text: "Revenue is 13% above last week.", Highlights: []string{"+123k"}},
					},
				},
			},
		},
	}
	emptyBundle := buildWhatChangedValidationBundleFromPeriodComparison(dashboardNarrativePeriodComparison{
		CurrentMetricDate: "2026-07-08",
		Facts:             []periodComparisonFact{},
	}, map[string]struct{}{})
	applyWhatChangedToResponse(resp, payload, emptyBundle, true)
	if resp.AIDiagnostics.WhatChanged.Status != WidgetAvailable {
		t.Fatalf("status=%s message=%q want available", resp.AIDiagnostics.WhatChanged.Status, resp.AIDiagnostics.WhatChanged.Message)
	}
	if len(resp.AIDiagnostics.WhatChanged.Groups) != 4 {
		t.Fatalf("groups=%d", len(resp.AIDiagnostics.WhatChanged.Groups))
	}
}
