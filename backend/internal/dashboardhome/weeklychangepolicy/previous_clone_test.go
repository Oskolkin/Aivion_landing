package weeklychangepolicy

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

func TestBuildPreviousPeriodFactsClone_ClassifiesActiveAndNew(t *testing.T) {
	current := baseFacts()
	current.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"1":  {OzonProductID: 1, StockAvailable: 0, Orders7d: 5, Revenue7d: 10000, ProductName: strPtr("Товар А")},
		"2":  {OzonProductID: 2, StockAvailable: 0, Orders7d: 3, Revenue7d: 8000, ProductName: strPtr("Товар Б")},
		"3":  {OzonProductID: 3, StockAvailable: 0, Orders7d: 2, Revenue7d: 5000, ProductName: strPtr("Товар В")},
		"4":  {OzonProductID: 4, StockAvailable: 0, Orders7d: 1, Revenue7d: 2000, ProductName: strPtr("Товар Г")},
		"10": {OzonProductID: 10, StockAvailable: 20, Orders7d: 0, ProductName: strPtr("Тихий товар")},
		"11": {OzonProductID: 11, StockAvailable: 15, Orders7d: 0, ProductName: strPtr("Ещё тихий")},
	}
	current.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsNoOrdersCount: 1,
		SpendWithoutOrders:     21700,
		CampaignsHighDRRCount:  3,
		HighDRRSpend:           318000,
		HighDRRThresholdPct:    80,
		TotalStoreRevenue7d:    3000000,
		TotalSpend7d:           200000,
	}
	current.AdEfficiency.NoOrderCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 882000041542, CampaignName: "Новая", Spend: 21700},
	}
	current.AdEfficiency.HighDRRCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 1, CampaignName: "Альфа", Spend: 100000},
		{CampaignExternalID: 2, CampaignName: "Бета", Spend: 110000},
		{CampaignExternalID: 3, CampaignName: "Гамма", Spend: 108000},
	}

	prev := BuildPreviousPeriodFactsClone(current)
	if !strings.HasSuffix(prev.MetricDate, "_prev") {
		t.Fatalf("metric date suffix: %q", prev.MetricDate)
	}
	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	if len(reg.Limitations) == 0 || reg.Limitations[0] != "previous_period_facts_cloned_from_current" {
		t.Fatalf("limitations=%v", reg.Limitations)
	}
	if !reg.HistoryAvailable {
		t.Fatal("expected history available from cloned previous facts")
	}
	if len(reg.ByChangeType[ChangeActiveProblem]) == 0 && len(reg.ByChangeType[ChangeWorsened]) == 0 {
		t.Fatalf("expected active or worsened from clone; byType=%#v", reg.ByChangeType)
	}
	foundNewAd := false
	for _, c := range reg.ByChangeType[ChangeNewRisk] {
		if c.MetricCode == MetricSpendWithoutOrders {
			foundNewAd = true
		}
	}
	if !foundNewAd {
		t.Fatalf("expected new_risk for spend without orders; new=%#v", reg.ByChangeType[ChangeNewRisk])
	}
}
