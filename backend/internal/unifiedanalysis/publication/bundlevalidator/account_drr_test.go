package bundlevalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
)

func TestCheckAccountDRRConsistency_AllowsDifferentPulseAndAds(t *testing.T) {
	storeShare := 19.4
	attrShare := 91.76
	in := Input{
		FactsPresent:    true,
		FactsIdentityOK: true,
		Facts: dashboardfacts.DashboardFactsV1{
			AdEfficiency: dashboardfacts.AdEfficiencyFacts{
				Status: "available",
				Summary: &dashboardfacts.AdEfficiencySummary{
					CurrentDRRPct:          &attrShare,
					TotalSpendToday:        23400,
					TotalRevenueToday:      25500,
					TotalStoreRevenueToday: 120618,
				},
			},
		},
		Projection: adapter.DashboardProjection{
			Frozen: dashboardhome.FrozenDashboardData{
				SellerPulse: dashboardhome.SellerPulse{
					KPIs: []dashboardhome.KPICard{{
						Key:   "drr",
						Value: "19.4%",
						Sparkline7D: []dashboardhome.SparklinePoint{
							{Date: "2026-07-08", Value: floatPtr(19.4), HasData: true},
						},
					}},
				},
				AdEfficiency: dashboardhome.AdEfficiencyWidget{
					WidgetBase: dashboardhome.WidgetBase{Status: dashboardhome.WidgetAvailable},
					Summary: &dashboardhome.AdEfficiencySummary{
						CurrentDRRPct:          &attrShare,
						AverageDRR7dPct:        &attrShare,
						TotalSpendToday:        23400,
						TotalRevenueToday:      25500,
						TotalStoreRevenueToday: 120618,
					},
					Trend7D: []dashboardhome.AdEfficiencyTrendPoint{
						{Date: "2026-07-08", Spend: 23400, Revenue: 25500, DRRPct: &attrShare, DataStatus: "available"},
					},
					Narrative: &dashboardhome.AdEfficiencyNarrative{
						Headline: "Реклама умеренно нагружает кабинет",
						Summary:  "Доля в общей выручке кабинета 19,4%. Доля в атрибутированной рекламной выручке 91,8%.",
					},
				},
			},
		},
	}
	_ = storeShare
	issues := checkAccountDRRConsistency(in)
	if len(issues) != 0 {
		t.Fatalf("expected no issues when pulse and ads differ by design, got %+v", issues)
	}
}

func TestCheckAccountDRRConsistency_FactsFrozenMismatchRejected(t *testing.T) {
	facts := 91.76
	frozen := 19.4
	in := Input{
		FactsPresent:    true,
		FactsIdentityOK: true,
		Facts: dashboardfacts.DashboardFactsV1{
			AdEfficiency: dashboardfacts.AdEfficiencyFacts{
				Status:  "available",
				Summary: &dashboardfacts.AdEfficiencySummary{CurrentDRRPct: &facts},
			},
		},
		Projection: adapter.DashboardProjection{
			Frozen: dashboardhome.FrozenDashboardData{
				AdEfficiency: dashboardhome.AdEfficiencyWidget{
					WidgetBase: dashboardhome.WidgetBase{Status: dashboardhome.WidgetAvailable},
					Summary:    &dashboardhome.AdEfficiencySummary{CurrentDRRPct: &frozen},
				},
			},
		},
	}
	issues := checkAccountDRRConsistency(in)
	if len(issues) == 0 {
		t.Fatal("expected facts vs frozen mismatch issue")
	}
	if issues[0].Code != CodeAccountDRRSourceMismatch {
		t.Fatalf("code=%s", issues[0].Code)
	}
}

func TestCheckAccountDRRPeriodAggregate(t *testing.T) {
	avg := 20.0
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{AverageDRR7dPct: &avg},
		Trend7D: []dashboardhome.AdEfficiencyTrendPoint{
			{Spend: 100, Revenue: 1000, DataStatus: "available", DRRPct: floatPtr(10)},
			{Spend: 300, Revenue: 1000, DataStatus: "available", DRRPct: floatPtr(30)},
		},
	}
	if issues := checkAccountDRRPeriodAggregate(ad); len(issues) != 0 {
		t.Fatalf("canonical totals should pass: %+v", issues)
	}
	bad := 50.0
	ad.Summary.AverageDRR7dPct = &bad
	if issues := checkAccountDRRPeriodAggregate(ad); len(issues) == 0 {
		t.Fatal("mean-of-percents style average must fail")
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestCheckAccountDRRRepeatedAggregate(t *testing.T) {
	same := 51.36
	trend := []dashboardhome.AdEfficiencyTrendPoint{
		{Spend: 100, Revenue: 1000, DRRPct: &same, DataStatus: "available"},
		{Spend: 200, Revenue: 1000, DRRPct: &same, DataStatus: "available"},
		{Spend: 300, Revenue: 1000, DRRPct: &same, DataStatus: "available"},
	}
	if issues := checkAccountDRRRepeatedAggregate(trend); len(issues) == 0 {
		t.Fatal("expected repeated aggregate detection")
	}
	// Legitimate flat ratio: spend scales with revenue.
	flat := 10.0
	okTrend := []dashboardhome.AdEfficiencyTrendPoint{
		{Spend: 100, Revenue: 1000, DRRPct: &flat, DataStatus: "available"},
		{Spend: 200, Revenue: 2000, DRRPct: &flat, DataStatus: "available"},
		{Spend: 300, Revenue: 3000, DRRPct: &flat, DataStatus: "available"},
	}
	if issues := checkAccountDRRRepeatedAggregate(okTrend); len(issues) != 0 {
		t.Fatalf("legitimate flat ratio must pass: %+v", issues)
	}
}

func TestCheckAdvertisingNarrativeNoRecommendation(t *testing.T) {
	ad := dashboardhome.AdEfficiencyWidget{
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline:       "ok",
			Summary:        "Расходы умеренные.",
			Recommendation: "Нужно проверить ставки.",
		},
	}
	issues := checkAdvertisingNarrativeNoRecommendation(ad)
	if len(issues) == 0 || issues[0].Code != "advertising_recommendation_not_allowed" {
		t.Fatalf("expected recommendation block, got %+v", issues)
	}
}

func TestCheckAdvertisingNarrativeArithmetic_BareDRRBlocked(t *testing.T) {
	store := 19.4
	attr := 91.76
	ad := dashboardhome.AdEfficiencyWidget{
		Summary: &dashboardhome.AdEfficiencySummary{
			CurrentDRRPct:          &attr,
			TotalSpendToday:        23400,
			TotalStoreRevenueToday: 120618,
		},
		Narrative: &dashboardhome.AdEfficiencyNarrative{
			Headline: "Общий ДРР кабинета высокий",
			Summary:  "Общий ДРР кабинета составляет 109%. Кампании требуют внимания.",
		},
	}
	_ = store
	issues := checkAdvertisingNarrativeArithmetic(ad)
	if !containsIssueCode(issues, "advertising_bare_drr_for_store_share") {
		t.Fatalf("expected bare DRR block, got %+v", issues)
	}
}

func containsIssueCode(issues []Issue, code string) bool {
	for _, i := range issues {
		if i.Code == code {
			return true
		}
	}
	return false
}
