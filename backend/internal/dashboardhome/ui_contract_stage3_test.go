package dashboardhome

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStockCoverageNarrativeHeadlineSummaryQuality(t *testing.T) {
	segments := []StockCoverageNarrativeSegment{
		{Key: "critical_0_3", Count: 4},
		{Key: "shortage_4_7", Count: 4},
	}
	n := &StockCoverageNarrative{
		Status:   "available",
		Headline: "Есть риск дефицита у продающих SKU: 4 в критичной зоне 0–3 дня и 4 могут закончиться в течение недели",
		Summary:  "Есть риск дефицита у продающих SKU: 4 в критичной зоне 0–3 дня и 4 могут закончиться в течение недели",
		Segments: segments,
	}
	polishStockCoverageHeadlineSummary(n)
	if utf8.RuneCountInString(n.Headline) > stockCoverageHeadlineMaxRunes {
		t.Fatalf("headline too long: %q", n.Headline)
	}
	if strings.Contains(n.Headline, "...") || strings.Contains(n.Headline, "\uFFFD") {
		t.Fatalf("headline corrupt: %q", n.Headline)
	}
	if utf8.RuneCountInString(n.Summary) < stockCoverageSummaryMinRunes {
		t.Fatalf("summary too short: %q", n.Summary)
	}
	if strings.HasPrefix(normalizeComparableText(n.Summary), normalizeComparableText(n.Headline)) {
		t.Fatalf("summary duplicates headline: headline=%q summary=%q", n.Headline, n.Summary)
	}
	if strings.TrimSpace(n.NextStep) == "" {
		t.Fatal("next_step missing")
	}
}

func TestWhatChangedMainConclusionNotTruncatedOrGeneric(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		MainConclusion: "AI видит три главные зоны внимания: просадка недельной выручки, риск дефицита и перерасход в рекламе. Одновременно есть большой объём...",
		Groups: []dashboardNarrativeWhatChangedGroup{
			{Key: whatChangedKeyWorsened, Summary: "Выручка снизилась на 5,03% за последние 7 дней."},
		},
	}
	repairWhatChangedMainConclusion(wc, "")
	normalizeWhatChangedMainConclusion(wc)
	if strings.HasSuffix(wc.MainConclusion, "...") {
		t.Fatalf("still truncated: %q", wc.MainConclusion)
	}
	if isGenericWhatChangedText(wc.MainConclusion) || isAISeesWhatChangedText(wc.MainConclusion) {
		t.Fatalf("unexpected generic: %q", wc.MainConclusion)
	}
}

func TestKPITrendLabelAvailableWhenSparklineExists(t *testing.T) {
	resp := &Response{
		SellerPulse: SellerPulse{
			KPIs: []KPICard{{
				Key: "ctr",
				Sparkline7D: []SparklinePoint{
					{Date: "2026-06-29", Value: floatPtr(100), HasData: true},
					{Date: "2026-06-30", Value: floatPtr(110), HasData: true},
				},
			}},
		},
	}
	finalizeSellerPulseKPITrends(resp)
	kpi := resp.SellerPulse.KPIs[0]
	if kpi.Trend == nil || kpi.Trend.Label != "Динамика за 7 дней" {
		t.Fatalf("trend=%#v", kpi.Trend)
	}
}

func TestWhatChangedGroupsHaveGroundedDensity(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		Groups: []dashboardNarrativeWhatChangedGroup{{
			Key:   whatChangedKeyWorsened,
			Label: "Ухудшилось",
			Tone:  "negative",
			Items: []dashboardNarrativeWhatChangedItem{{Label: "Выручка", Text: "Снизилась."}},
		}},
	}
	bundle := whatChangedValidationBundle{
		Context: dashboardNarrativeWhatChangedContext{
			WorsenedFacts: []whatChangedFact{
				{ID: "rev", Label: "Выручка за 7 дней", Direction: whatChangedKeyWorsened},
				{ID: "ord", Label: "Заказы за 7 дней", Direction: whatChangedKeyWorsened},
			},
		},
	}
	enrichWhatChangedGroupsFromFacts(wc, bundle)
	if len(wc.Groups[0].Items) < 2 {
		t.Fatalf("items=%d", len(wc.Groups[0].Items))
	}
}

func TestWhatChangedDoesNotInventImprovements(t *testing.T) {
	wc := &dashboardNarrativeWhatChanged{
		Groups: []dashboardNarrativeWhatChangedGroup{{
			Key:   whatChangedKeyImproved,
			Label: "Улучшилось",
			Tone:  "positive",
			Items: []dashboardNarrativeWhatChangedItem{{Label: "Выдумка", Text: "Всё стало лучше."}},
		}},
	}
	bundle := whatChangedValidationBundle{Context: dashboardNarrativeWhatChangedContext{}}
	repaired := repairWhatChangedAIOnly(wc, bundle)
	if len(repaired.Groups[0].Items) != 0 {
		t.Fatalf("improved items should be cleared: %#v", repaired.Groups[0].Items)
	}
}

func TestRiskDriversHaveSellerFacingTitles(t *testing.T) {
	drivers := []RiskDriver{{
		ID: "driver-sales-revenue", Category: "sales",
		Title:  "За последние 7 дней выручка аккаунта снизилась на 5,03% относительно предыдущих 7 дней",
		Reason: "За последние 7 дней выручка аккаунта снизилась на 5,03% относительно предыдущих 7 дней.",
	}}
	out := sanitizeRiskDriversForSellerFacing(drivers, &Response{})
	if out[0].Title == out[0].Reason {
		t.Fatalf("title still equals reason: %q", out[0].Title)
	}
	if utf8.RuneCountInString(out[0].Title) > 80 {
		t.Fatalf("title too long: %q", out[0].Title)
	}
}

func floatPtr(v float64) *float64 { return &v }
