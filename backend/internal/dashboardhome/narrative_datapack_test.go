package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestBuildDashboardNarrativeContextFromDataPack_Basic(t *testing.T) {
	pack := sampleDataPackForNarrativeTest()
	build, err := buildDashboardNarrativeContextFromDataPack(pack, nil)
	if err != nil {
		t.Fatal(err)
	}
	if build.ContextSource != ContextSourceSellerAIDataPack {
		t.Fatalf("context_source=%s", build.ContextSource)
	}
	v8, ok := build.AIContext.(dashboardNarrativeContextV8)
	if !ok {
		t.Fatal("expected dashboardNarrativeContextV8")
	}
	if v8.SchemaVersion != DashboardNarrativeContextSchemaV8 {
		t.Fatalf("schema=%s", v8.SchemaVersion)
	}
	if v8.ContextSource != ContextSourceSellerAIDataPack {
		t.Fatalf("v8 context_source=%s", v8.ContextSource)
	}
}

func TestDataPackFactCatalogOnlyExistingFacts(t *testing.T) {
	pack := sampleDataPackForNarrativeTest()
	build, err := buildDashboardNarrativeContextFromDataPack(pack, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range build.DataPackFactIDs {
		if _, ok := pack.FactRegistry[id]; !ok {
			t.Fatalf("catalog fact %q not in registry", id)
		}
	}
}

func TestDataPackDriversStableIDs(t *testing.T) {
	pack := sampleDataPackForNarrativeTest()
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		t.Fatal(err)
	}
	v8 := build.AIContext.(dashboardNarrativeContextV8)
	seen := map[string]struct{}{}
	for _, d := range v8.RiskDrivers {
		if d.ID == "" {
			t.Fatal("empty driver id")
		}
		if _, ok := seen[d.ID]; ok {
			t.Fatalf("duplicate driver %s", d.ID)
		}
		seen[d.ID] = struct{}{}
	}
	want := []string{driverAccountSales, driverStock, driverAdvertising, driverDataQuality}
	for _, id := range want {
		if _, ok := seen[id]; !ok {
			t.Fatalf("missing driver %s", id)
		}
	}
}

func TestDataPackCandidatesStableIDs(t *testing.T) {
	pack := sampleDataPackForNarrativeTest()
	build, err := buildDashboardNarrativeContextFromDataPack(pack, nil)
	if err != nil {
		t.Fatal(err)
	}
	for id := range build.CandidateByID {
		if id == "" {
			t.Fatal("empty candidate id")
		}
	}
	if _, ok := build.CandidateByID[candidateStockReview]; !ok {
		t.Fatalf("missing %s", candidateStockReview)
	}
}

func TestDashboardNarrativePromptVersionV8(t *testing.T) {
	if DashboardNarrativePromptVersion != "dashboard_narrative_v9_ai_only_chunks" {
		t.Fatalf("prompt version=%s", DashboardNarrativePromptVersion)
	}
}

func TestV8PayloadAcceptsFactIDs(t *testing.T) {
	payload := &dashboardNarrativePayload{
		Headline:           "Test",
		ExecutiveSummary:   "Summary with facts.",
		ImprovementSummary: "Improve stock and ads.",
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: driverStock, Category: "stock", Title: "Stock", Reason: "DEFICIT present", FactIDs: []string{"sku.801.last_7d.revenue"}},
		},
		TopActions: []dashboardNarrativeTopAction{
			{
				DriverID: driverStock, SourceCandidateID: candidateStockReview, Priority: "high", Category: "stock",
				Title: "Review stock", Reason: "Risk", NextStep: "Check", ExpectedEffect: "Lower risk",
				FactIDs: []string{"sku.801.last_7d.revenue"},
			},
		},
	}
	if len(payload.HealthReasons[0].FactIDs) != 1 {
		t.Fatal("expected fact_ids on health reason")
	}
}

func TestV8ValidationRejectsUnknownFactID(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{
		ContextSource:   ContextSourceSellerAIDataPack,
		DataPackFactIDs: map[string]struct{}{"account.last_7d.revenue": {}},
		DriversByID:     map[string]RiskDriver{driverStock: {ID: driverStock, Category: "stock"}},
		CandidatesByID: map[string]dashboardNarrativeCandidate{
			candidateStockReview: {ID: candidateStockReview, Category: "stock", CompatibleDriverIDs: []string{driverStock}},
		},
		FinanceAvailable: true,
	}
	payload := &dashboardNarrativePayload{
		Headline: "H", ExecutiveSummary: "E", ImprovementSummary: "I",
		HealthReasons: []dashboardNarrativeHealthReason{
			{DriverID: driverStock, Category: "stock", Title: "T", Reason: "R", FactIDs: []string{"unknown.fact"}},
		},
		TopActions: []dashboardNarrativeTopAction{
			{DriverID: driverStock, SourceCandidateID: candidateStockReview, Priority: "high", Category: "stock",
				Title: "T", Reason: "R", NextStep: "N", ExpectedEffect: "E"},
		},
	}
	errs := validateDataPackNarrativeExtras(payload, ctx)
	if len(errs) == 0 {
		t.Fatal("expected unknown fact_id error")
	}
}

func TestV8ValidationRejectsSKULevelAdWording(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{ContextSource: ContextSourceSellerAIDataPack, FinanceAvailable: false}
	payload := &dashboardNarrativePayload{
		Headline: "H", ExecutiveSummary: "расход по sku высокий", ImprovementSummary: "I",
		HealthReasons: []dashboardNarrativeHealthReason{{DriverID: driverAdvertising, Category: "advertising", Title: "T", Reason: "R"}},
		TopActions: []dashboardNarrativeTopAction{
			{DriverID: driverAdvertising, SourceCandidateID: candidateAdEfficiencyReview, Priority: "high", Category: "advertising",
				Title: "T", Reason: "R", NextStep: "N", ExpectedEffect: "E"},
		},
	}
	errs := validateDataPackNarrativeExtras(payload, ctx)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "sku_level_ad") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sku level ad error, got %v", errs)
	}
}

func TestV8ValidationRespectsFinanceUnavailable(t *testing.T) {
	ctx := dashboardNarrativeValidationContext{ContextSource: ContextSourceSellerAIDataPack, FinanceAvailable: false}
	payload := &dashboardNarrativePayload{
		Headline: "H", ExecutiveSummary: "маржа под давлением", ImprovementSummary: "I",
		HealthReasons: []dashboardNarrativeHealthReason{{DriverID: driverAccountSales, Category: "sales", Title: "T", Reason: "R"}},
		TopActions: []dashboardNarrativeTopAction{
			{DriverID: driverAccountSales, SourceCandidateID: "candidate-account-review", Priority: "high", Category: "sales",
				Title: "T", Reason: "R", NextStep: "N", ExpectedEffect: "E"},
		},
	}
	errs := validateDataPackNarrativeExtras(payload, ctx)
	if len(errs) == 0 {
		t.Fatal("expected finance unavailable error")
	}
}

func sampleDataPackForNarrativeTest() *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{
			SellerAccountID: 1, Marketplace: aidatapack.MarketplaceOzon, MetricDate: "2026-06-25",
		},
		DataQuality: aidatapack.DataQuality{Limitations: []string{"finance_not_available"}},
		FactRegistry: map[string]aidatapack.Fact{
			"account.last_7d.revenue":    {FactID: "account.last_7d.revenue", FormattedValue: "100 000 ₽", Unit: "RUB", Metric: "revenue"},
			"sku.801.last_7d.revenue":    {FactID: "sku.801.last_7d.revenue", FormattedValue: "1 000 ₽", Unit: "RUB", Metric: "revenue", EntityID: "sku:801"},
			"campaign.882.last_7d.spend": {FactID: "campaign.882.last_7d.spend", FormattedValue: "5 000 ₽", Unit: "RUB", Metric: "spend", EntityID: "campaign:882"},
		},
		EntityRegistry: aidatapack.EntityRegistry{
			SKUs:      map[string]aidatapack.SKUEntity{"sku:801": {EntityID: "sku:801", SKU: "801", Name: "Test SKU"}},
			Campaigns: map[string]aidatapack.CampaignEntity{"campaign:882": {EntityID: "campaign:882", CampaignExternalID: "882", Name: "Test Campaign"}},
		},
		Tables: aidatapack.Tables{
			AccountRollup: aidatapack.AccountRollupTable{
				DataStatus: "available",
				Last7d:     map[string]any{"revenue": 100000, "orders": 10},
				FactIDs:    []string{"account.last_7d.revenue"},
			},
			StockPositions: []aidatapack.StockPositionRow{
				{EntityID: "sku:801", TurnoverGrade: "DEFICIT", FactIDs: []string{"sku.801.last_7d.revenue"}},
				{EntityID: "sku:802", TurnoverGrade: "SURPLUS"},
			},
			CampaignRollups: []aidatapack.CampaignRollupRow{
				{
					EntityID: "campaign:882", CampaignExternalID: "882", Name: "Test Campaign", State: "CAMPAIGN_STATE_RUNNING",
					Last7d:  aidatapack.CampaignMetricsWindow{Spend: 5000, SpendWithoutOrders: true, DRRPct: func() *float64 { v := 45.0; return &v }()},
					FactIDs: []string{"campaign.882.last_7d.spend"},
				},
			},
			FunnelRollup:  aidatapack.FunnelRollupTable{DataStatus: "not_available"},
			FinanceRollup: aidatapack.FinanceRollupTable{DataStatus: "not_available"},
			ProductEnrichment: []aidatapack.ProductEnrichmentRow{
				{EntityID: "sku:801", Visibility: map[string]any{"has_stock": false}},
			},
		},
		Constraints: aidatapack.ConstraintsBlock{PriceConstraintsAvailable: false},
	}
}
