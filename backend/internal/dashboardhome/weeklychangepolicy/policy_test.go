package weeklychangepolicy

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
)

func TestClassificationWorsened(t *testing.T) {
	current := baseFacts()
	current.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"1": {OzonProductID: 1, StockAvailable: 0, Orders7d: 5, Revenue7d: 10000, ProductName: strPtr("Товар А")},
		"2": {OzonProductID: 2, StockAvailable: 0, Orders7d: 3, Revenue7d: 8000, ProductName: strPtr("Товар Б")},
		"3": {OzonProductID: 3, StockAvailable: 0, Orders7d: 2, Revenue7d: 5000, ProductName: strPtr("Товар В")},
		"4": {OzonProductID: 4, StockAvailable: 0, Orders7d: 1, Revenue7d: 2000, ProductName: strPtr("Товар Г")},
	}
	prev := baseFacts()
	prev.MetricDate = "2026-07-01"
	prev.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"1": {OzonProductID: 1, StockAvailable: 0, Orders7d: 5, Revenue7d: 10000, ProductName: strPtr("Товар А")},
		"2": {OzonProductID: 2, StockAvailable: 0, Orders7d: 3, Revenue7d: 8000, ProductName: strPtr("Товар Б")},
	}

	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	found := false
	for _, c := range reg.ByChangeType[ChangeWorsened] {
		if c.MetricCode == MetricStockoutSKUCount {
			found = true
			if c.PreviousValue == nil || *c.PreviousValue != 2 {
				t.Fatalf("expected previous=2, got %#v", c.PreviousValue)
			}
			if c.CurrentValue == nil || *c.CurrentValue != 4 {
				t.Fatalf("expected current=4, got %#v", c.CurrentValue)
			}
			if ContainsNumericCampaignID(c.TitleCandidate + c.DescriptionCandidate) {
				t.Fatalf("copy leaked numeric id: %s", c.TitleCandidate)
			}
		}
	}
	if !found {
		t.Fatalf("expected stockout worsened, got %#v", reg.ByChangeType[ChangeWorsened])
	}
}

func TestClassificationNewRisk(t *testing.T) {
	current := baseFacts()
	current.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsNoOrdersCount: 1,
		SpendWithoutOrders:     21700,
		HighDRRThresholdPct:    80,
		TotalStoreRevenue7d:    3000000,
		TotalSpend7d:           200000,
	}
	current.AdEfficiency.NoOrderCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 882000041542, CampaignName: "Тестовая кампания", Spend: 21700},
	}
	prev := baseFacts()
	prev.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsNoOrdersCount: 0,
		SpendWithoutOrders:     0,
		TotalStoreRevenue7d:    2800000,
		TotalSpend7d:           180000,
	}

	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	found := false
	for _, c := range reg.ByChangeType[ChangeNewRisk] {
		if c.MetricCode == MetricSpendWithoutOrders {
			found = true
			if ContainsNumericCampaignID(c.TitleCandidate) || ContainsNumericCampaignID(c.DescriptionCandidate) {
				t.Fatalf("campaign id leaked: %s / %s", c.TitleCandidate, c.DescriptionCandidate)
			}
			if !strings.Contains(c.DescriptionCandidate, "Тестовая кампания") && !strings.Contains(c.TitleCandidate, "21") {
				t.Fatalf("expected campaign name or spend in copy: %s | %s", c.TitleCandidate, c.DescriptionCandidate)
			}
		}
	}
	if !found {
		t.Fatalf("expected new spend-without-orders risk, got %#v", reg.ByChangeType[ChangeNewRisk])
	}
}

func TestClassificationActiveProblem(t *testing.T) {
	current := baseFacts()
	current.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"10": {OzonProductID: 10, StockAvailable: 20, Orders7d: 0, Orders: 0, ProductName: strPtr("Тихий товар")},
		"11": {OzonProductID: 11, StockAvailable: 15, Orders7d: 0, Orders: 0, ProductName: strPtr("Ещё тихий")},
	}
	prev := baseFacts()
	prev.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"10": {OzonProductID: 10, StockAvailable: 20, Orders7d: 0, Orders: 0, ProductName: strPtr("Тихий товар")},
		"11": {OzonProductID: 11, StockAvailable: 15, Orders7d: 0, Orders: 0, ProductName: strPtr("Ещё тихий")},
	}

	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	found := false
	for _, c := range reg.ByChangeType[ChangeActiveProblem] {
		if c.MetricCode == MetricNoSalesWithStock {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected active no_sales problem, got %#v", reg.ByChangeType[ChangeActiveProblem])
	}
}

func TestClassificationImproved(t *testing.T) {
	pct := 0.121
	current := baseFacts()
	current.DashboardMetrics.Account.RevenueLast7Days = 3359790
	current.DashboardMetrics.Account.RevenueWeekDelta = analytics.DashboardDelta{Pct: &pct, Abs: 363000}

	reg := BuildCandidateRegistry(current, nil, PreviousContext{})
	found := false
	for _, c := range reg.ByChangeType[ChangeImproved] {
		if c.MetricCode == MetricRevenue7d {
			found = true
			if c.PreviousValue == nil || c.CurrentValue == nil {
				t.Fatal("improved revenue must have previous/current")
			}
			if c.RelativeDeltaPct == nil || *c.RelativeDeltaPct < 10 {
				t.Fatalf("expected ~12%% growth, got %#v", c.RelativeDeltaPct)
			}
		}
	}
	if !found {
		t.Fatalf("expected revenue improved, candidates=%#v", reg.Candidates)
	}
}

func TestNoCrossGroupDuplicate(t *testing.T) {
	current := baseFacts()
	current.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsHighDRRCount: 3,
		HighDRRSpend:          318000,
		TotalStoreRevenue7d:   3000000,
		TotalSpend7d:          400000,
		HighDRRThresholdPct:   80,
	}
	current.AdEfficiency.HighDRRCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 1, CampaignName: "Кампания 1", Spend: 100000},
		{CampaignExternalID: 2, CampaignName: "Кампания 2", Spend: 100000},
		{CampaignExternalID: 3, CampaignName: "Кампания 3", Spend: 118000},
	}
	prev := baseFacts()
	prev.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsHighDRRCount: 1,
		HighDRRSpend:          50000,
		TotalStoreRevenue7d:   2800000,
		TotalSpend7d:          300000,
	}

	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	seen := map[string]int{}
	for _, c := range reg.Candidates {
		seen[c.ChangeID]++
		if seen[c.ChangeID] > 1 {
			t.Fatalf("duplicate change_id %s", c.ChangeID)
		}
	}
	if errs := ValidateRegistrySemantics(reg); len(errs) > 0 {
		t.Fatalf("semantic errors: %v", errs)
	}
}

func TestPreviousPublicationAbsent(t *testing.T) {
	current := baseFacts()
	current.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"1": {OzonProductID: 1, StockAvailable: 0, Orders7d: 4, Revenue7d: 9000, ProductName: strPtr("A")},
	}
	reg := BuildCandidateRegistry(current, nil, PreviousContext{})
	if reg.HistoryAvailable {
		t.Fatal("history must be unavailable without previous facts/publication")
	}
	if len(reg.ByChangeType[ChangeActiveProblem]) != 0 {
		t.Fatalf("active_problems must be empty without history, got %#v", reg.ByChangeType[ChangeActiveProblem])
	}
	empty := reg.EmptyTextByGroup[GroupKeyUnchangedRisks]
	if !strings.Contains(empty, "Недостаточно истории") {
		t.Fatalf("expected history-insufficient empty text, got %q", empty)
	}
}

func TestMaterialityRanking(t *testing.T) {
	current := baseFacts()
	pct := 0.12
	current.DashboardMetrics.Account.RevenueLast7Days = 3000000
	current.DashboardMetrics.Account.RevenueWeekDelta = analytics.DashboardDelta{Pct: &pct, Abs: 320000}
	current.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsHighDRRCount: 3, HighDRRSpend: 318000, TotalStoreRevenue7d: 3000000, TotalSpend7d: 400000, HighDRRThresholdPct: 80,
	}
	current.AdEfficiency.HighDRRCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 1, CampaignName: "A"},
		{CampaignExternalID: 2, CampaignName: "B"},
		{CampaignExternalID: 3, CampaignName: "C"},
	}
	prev := baseFacts()
	prev.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{CampaignsHighDRRCount: 1, HighDRRSpend: 40000, TotalStoreRevenue7d: 2800000, TotalSpend7d: 300000}

	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	sorted := SortCandidates(reg.Candidates)
	if len(sorted) < 2 {
		t.Fatal("expected multiple candidates")
	}
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].MaterialityScore < sorted[i].MaterialityScore {
			t.Fatalf("not sorted by materiality: %d < %d", sorted[i-1].MaterialityScore, sorted[i].MaterialityScore)
		}
	}
}

func TestCampaignNameMapping(t *testing.T) {
	row := dashboardfacts.AdCampaignDisplayRow{CampaignExternalID: 882000041542, CampaignName: ""}
	ids, names := campaignEntities([]dashboardfacts.AdCampaignDisplayRow{row})
	if len(ids) != 1 || ids[0] != "882000041542" {
		t.Fatalf("ids=%v", ids)
	}
	if names[0] != "Кампания без названия" {
		t.Fatalf("name=%q", names[0])
	}
	text := StripNumericCampaignIDs("Кампания 882000041542 потратила бюджет", names)
	if ContainsNumericCampaignID(text) {
		t.Fatalf("id still present: %s", text)
	}
}

func TestActionReference(t *testing.T) {
	cands := []ChangeCandidate{{
		ChangeID: "worsened:advertising.high_drr_campaign_count", Domain: "advertising",
		ChangeType: ChangeWorsened, EntityType: "campaign", EntityIDs: []string{"1", "2"},
	}}
	actions := []ActionRef{{
		AnalysisActionID: "action_001", Category: "advertising", ScopeType: "campaign", EntityIDs: []string{"1"},
	}}
	out := AttachActionLinks(cands, actions)
	if out[0].RelatedActionID != "action_001" {
		t.Fatalf("expected linked action, got %q", out[0].RelatedActionID)
	}
}

func TestStructuredOutputCompleteness(t *testing.T) {
	pct := 0.091
	current := baseFacts()
	current.DashboardMetrics.Account.RevenueLast7Days = 3359790
	revPct := 0.121
	current.DashboardMetrics.Account.RevenueWeekDelta = analytics.DashboardDelta{Pct: &revPct, Abs: 363000}
	current.SellerPulse.KPIs = []dashboardfacts.KPICardFact{
		{Key: "orders", Delta: &dashboardfacts.KPICardDelta{Abs: 113, Pct: &pct}},
	}
	current.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"1": {OzonProductID: 1, StockAvailable: 0, Orders7d: 5, Revenue7d: 12000, ProductName: strPtr("SKU1")},
		"2": {OzonProductID: 2, StockAvailable: 0, Orders7d: 4, Revenue7d: 9000, ProductName: strPtr("SKU2")},
	}
	current.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsHighDRRCount: 3, HighDRRSpend: 318000, CampaignsNoOrdersCount: 1, SpendWithoutOrders: 21700,
		TotalStoreRevenue7d: 3359790, TotalSpend7d: 400000, HighDRRThresholdPct: 80,
	}
	current.AdEfficiency.HighDRRCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 1, CampaignName: "Альфа"},
		{CampaignExternalID: 2, CampaignName: "Бета"},
		{CampaignExternalID: 3, CampaignName: "Гамма"},
	}
	current.AdEfficiency.NoOrderCampaigns = []dashboardfacts.AdCampaignDisplayRow{
		{CampaignExternalID: 99, CampaignName: "Новая", Spend: 21700},
	}
	prev := baseFacts()
	prev.SKUFactsIndex.ByOzonProductID = map[string]dashboardfacts.SKUFactRecord{
		"1": {OzonProductID: 1, StockAvailable: 0, Orders7d: 5, Revenue7d: 12000, ProductName: strPtr("SKU1")},
	}
	prev.AdEfficiency.Summary = &dashboardfacts.AdEfficiencySummary{
		CampaignsHighDRRCount: 1, HighDRRSpend: 50000, CampaignsNoOrdersCount: 0, SpendWithoutOrders: 0,
		TotalStoreRevenue7d: 3000000, TotalSpend7d: 350000,
	}
	prev.SKUFactsIndex.ByOzonProductID["10"] = dashboardfacts.SKUFactRecord{
		OzonProductID: 10, StockAvailable: 12, Orders7d: 0, Orders: 0, ProductName: strPtr("Тихий"),
	}
	current.SKUFactsIndex.ByOzonProductID["10"] = dashboardfacts.SKUFactRecord{
		OzonProductID: 10, StockAvailable: 12, Orders7d: 0, Orders: 0, ProductName: strPtr("Тихий"),
	}

	reg := BuildCandidateRegistry(current, &prev, PreviousContext{})
	_, _, groups := BuildGroupsFromRegistry(reg, nil, 3)
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}
	counts := map[string]int{}
	for _, g := range groups {
		counts[g.Key] = len(g.Items)
		for _, it := range g.Items {
			if it.Title == "" || it.Description == "" {
				t.Fatalf("incomplete item in %s: %#v", g.Key, it)
			}
			if ContainsNumericCampaignID(it.Title + it.Description) {
				t.Fatalf("numeric id in %s: %s", g.Key, it.Title)
			}
		}
	}
	if counts[GroupKeyWorsened] == 0 || counts[GroupKeyImproved] == 0 {
		t.Fatalf("expected worsened+improved filled, got %v", counts)
	}
	if counts[GroupKeyNewRisks] == 0 && counts[GroupKeyUnchangedRisks] == 0 {
		t.Fatalf("expected new or active problems, got %v", counts)
	}
}

func baseFacts() dashboardfacts.DashboardFactsV1 {
	return dashboardfacts.DashboardFactsV1{
		MetricDate: "2026-07-08",
		DashboardMetrics: analytics.DashboardMetricsDTO{
			Account: analytics.DashboardAccountMetrics{
				RevenueLast7Days: 1000000,
			},
		},
		SKUFactsIndex: dashboardfacts.SKUFactsIndex{ByOzonProductID: map[string]dashboardfacts.SKUFactRecord{}},
		AdEfficiency:  dashboardfacts.AdEfficiencyFacts{Status: "available"},
		Funnel:        dashboardfacts.FunnelFacts{Status: "available"},
		KeySKUCohort:  dashboardfacts.KeySKUCohortFacts{Status: "not_enough_data"},
	}
}

func strPtr(s string) *string { return &s }
