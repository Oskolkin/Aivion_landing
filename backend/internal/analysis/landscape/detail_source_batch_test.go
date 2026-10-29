package landscape

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type stubBatchDetailAPI struct {
	factualapi.API
	campaignBatch       factualapi.BatchSKUCampaignPerformanceResult
	queryBatch          factualapi.BatchSKUQueryDetailsResult
	promoBatch          factualapi.BatchSKUPromotionsResult
	campaignTargetBatch factualapi.BatchCampaignTargetSKUResult
	campaignErr         error
	queryErr            error
	promoErr            error
	campaignTargetErr   error
}

func (s stubBatchDetailAPI) BatchListSKUCampaignPerformance(_ context.Context, _ factualapi.FactualReadContext, _ []int64, _ factualapi.StandardPeriod, _ *factualapi.Period, _ int) (factualapi.BatchSKUCampaignPerformanceResult, error) {
	if s.campaignErr != nil {
		return factualapi.BatchSKUCampaignPerformanceResult{}, s.campaignErr
	}
	return s.campaignBatch, nil
}

func (s stubBatchDetailAPI) BatchGetSKUQueryDetails(_ context.Context, _ factualapi.FactualReadContext, _ []int64, _ factualapi.StandardPeriod, _ *factualapi.Period, _ int) (factualapi.BatchSKUQueryDetailsResult, error) {
	if s.queryErr != nil {
		return factualapi.BatchSKUQueryDetailsResult{}, s.queryErr
	}
	return s.queryBatch, nil
}

func (s stubBatchDetailAPI) BatchListCampaignTargetSKUs(_ context.Context, _ factualapi.FactualReadContext, _ []int64, _ int) (factualapi.BatchCampaignTargetSKUResult, error) {
	if s.campaignTargetErr != nil {
		return factualapi.BatchCampaignTargetSKUResult{}, s.campaignTargetErr
	}
	return s.campaignTargetBatch, nil
}

func (s stubBatchDetailAPI) BatchGetSKUPromotions(_ context.Context, _ factualapi.FactualReadContext, _ []int64, _ int) (factualapi.BatchSKUPromotionsResult, error) {
	if s.promoErr != nil {
		return factualapi.BatchSKUPromotionsResult{}, s.promoErr
	}
	return s.promoBatch, nil
}

func TestLoadSKUDetailExtrasBatchDomains(t *testing.T) {
	spendHigh, _ := factualapi.ParseDecimal("500")
	spendLow, _ := factualapi.ParseDecimal("100")
	src := FactualAPIDetailSource{API: stubBatchDetailAPI{
		campaignBatch: factualapi.BatchSKUCampaignPerformanceResult{
			Items: []factualapi.BatchSKUCampaignPerformanceItem{
				{
					SKUID:  42,
					Status: factualapi.BatchFound,
					Items: []factualapi.SKUCampaignPerformance{
						{CampaignID: 2, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendHigh, CurrencyCode: "RUB"})},
						{CampaignID: 1, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendLow, CurrencyCode: "RUB"})},
						{CampaignID: 3, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendLow, CurrencyCode: "RUB"})},
						{CampaignID: 4, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendLow, CurrencyCode: "RUB"})},
					},
					HasMore: true,
				},
			},
		},
		queryBatch: factualapi.BatchSKUQueryDetailsResult{
			Items: []factualapi.BatchSKUQueryDetailsItem{
				{SKUID: 42, Status: factualapi.BatchFound, Items: []factualapi.SearchQueryDetail{{SearchQueryID: 9, ProviderRank: 3}}},
			},
		},
		promoBatch: factualapi.BatchSKUPromotionsResult{
			Items: []factualapi.BatchSKUPromotionsItem{
				{SKUID: 42, Status: factualapi.BatchFound, Items: []factualapi.SKUPromotion{{Promotion: factualapi.Promotion{ID: 7, Origin: "PROVIDER", DiscountType: "PERCENT"}}}},
			},
		},
	}}
	cutoff := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cutoff,
		Policies:        factualapi.V1Policies(),
		Sealed:          stubSeal{id: "u1", hash: stringsRepeat("a", 64)},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "ADVERTISING.SKU_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
			{CapabilityKey: "SEARCH.QUERY_DETAILS", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
			{CapabilityKey: "PROMOTIONS.MEMBERSHIP", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		},
		Domains: []factualapi.DomainBinding{
			{Domain: "ADVERTISING", FactFamily: "ADVERTISING.TRAFFIC", DomainRunID: 1, FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("h", 64), CoverageTo: timePtr(cutoff.AddDate(0, 0, -1))},
			{Domain: "SEARCH", FactFamily: "SEARCH.QUERY_DETAILS", DomainRunID: 2, FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("h", 64), CoverageTo: timePtr(cutoff.AddDate(0, 0, -1))},
		},
	}, stubSeal{id: "u1", hash: stringsRepeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	out, err := src.LoadSKUDetailExtras(context.Background(), sealed, []int64{42}, SKUDetailV3())
	if err != nil {
		t.Fatal(err)
	}
	ex := out[42]
	if ex.CampaignContext.State != BlockReady || !ex.CampaignContext.Truncated || ex.CampaignContext.ReturnedCount != 4 {
		t.Fatalf("campaign context=%+v", ex.CampaignContext)
	}
	if len(ex.CampaignContext.Items) != 4 || ex.CampaignContext.Items[0].CampaignID != 2 {
		t.Fatalf("expected top spend campaign first, got %+v", ex.CampaignContext.Items)
	}
	if ex.QueryContext.State != BlockReady || len(ex.QueryContext.Items) != 1 || ex.QueryContext.Items[0].QueryID != 9 {
		t.Fatalf("query context=%+v", ex.QueryContext)
	}
	if ex.PromotionContext.State != BlockReady || len(ex.PromotionContext.Items) != 1 || ex.PromotionContext.Items[0].PromotionID != 7 {
		t.Fatalf("promotion context=%+v", ex.PromotionContext)
	}
}

func TestLoadSKUDetailExtrasPeriodUnsupportedUnavailable(t *testing.T) {
	src := FactualAPIDetailSource{API: stubBatchDetailAPI{
		campaignErr: factualapi.NewError(factualapi.CodePeriodUnsupported, "unsupported standard period"),
	}}
	cutoff := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cutoff,
		Policies:        factualapi.V1Policies(),
		Sealed:          stubSeal{id: "u1", hash: stringsRepeat("a", 64)},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "ADVERTISING.SKU_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		},
		Domains: []factualapi.DomainBinding{
			{Domain: "ADVERTISING", FactFamily: "ADVERTISING.TRAFFIC", DomainRunID: 1, FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("h", 64), CoverageTo: timePtr(cutoff.AddDate(0, 0, -1))},
		},
	}, stubSeal{id: "u1", hash: stringsRepeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	out, err := src.LoadSKUDetailExtras(context.Background(), sealed, []int64{42, 43}, SKUDetailV3())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{42, 43} {
		ex := out[id]
		if ex.CampaignContext.State != BlockUnavailable {
			t.Fatalf("sku %d campaign state=%s want UNAVAILABLE", id, ex.CampaignContext.State)
		}
		if len(ex.CampaignContext.Items) != 0 {
			t.Fatalf("sku %d campaign items must be empty when unavailable", id)
		}
		if ex.CampaignContext.Limitations == nil || ex.CampaignContext.Limitations[0].Code != string(factualapi.CodePeriodUnsupported) {
			t.Fatalf("sku %d limitations=%+v", id, ex.CampaignContext.Limitations)
		}
	}
}

func TestLoadSKUCampaignContextSortedAndTruncated(t *testing.T) {
	spendHigh, _ := factualapi.ParseDecimal("500")
	spendLow, _ := factualapi.ParseDecimal("100")
	src := FactualAPIDetailSource{API: stubBatchDetailAPI{
		campaignBatch: factualapi.BatchSKUCampaignPerformanceResult{
			Items: []factualapi.BatchSKUCampaignPerformanceItem{
				{
					SKUID:  42,
					Status: factualapi.BatchFound,
					Items: []factualapi.SKUCampaignPerformance{
						{CampaignID: 2, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendHigh, CurrencyCode: "RUB"})},
						{CampaignID: 1, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendLow, CurrencyCode: "RUB"})},
						{CampaignID: 3, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendLow, CurrencyCode: "RUB"})},
						{CampaignID: 4, PaidSpend: factualapi.Known(factualapi.Money{Amount: spendLow, CurrencyCode: "RUB"})},
					},
					HasMore: true,
				},
			},
		},
	}}
	cutoff := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cutoff,
		Policies:        factualapi.V1Policies(),
		Sealed:          stubSeal{id: "u1", hash: stringsRepeat("a", 64)},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "ADVERTISING.SKU_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
			{CapabilityKey: "SEARCH.QUERY_DETAILS", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
			{CapabilityKey: "PROMOTIONS.MEMBERSHIP", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		},
		Domains: []factualapi.DomainBinding{
			{Domain: "ADVERTISING", FactFamily: "ADVERTISING.TRAFFIC", DomainRunID: 1, FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("h", 64), CoverageTo: timePtr(cutoff.AddDate(0, 0, -1))},
			{Domain: "SEARCH", FactFamily: "SEARCH.QUERY_DETAILS", DomainRunID: 2, FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("h", 64), CoverageTo: timePtr(cutoff.AddDate(0, 0, -1))},
		},
	}, stubSeal{id: "u1", hash: stringsRepeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	out := map[int64]SKUDetailExtras{42: {}}
	if err := src.batchLoadSKUCampaignContext(context.Background(), sealed, []int64{42}, SupportingSpec{
		Limit: 2, SortBasis: "PAID_SPEND_DESC", SemanticScope: "TOP_N_BY_PAID_SPEND",
		Period: sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
	}, out); err != nil {
		t.Fatal(err)
	}
	cc := out[42].CampaignContext
	if !cc.Truncated || cc.ReturnedCount != 2 || cc.TotalCount != 3 {
		t.Fatalf("truncation metadata=%+v", cc)
	}
	if len(cc.Items) != 2 || cc.Items[0].CampaignID != 2 {
		t.Fatalf("expected top spend campaign first, got %+v", cc.Items)
	}
	if cc.SortBasis != "PAID_SPEND_DESC" {
		t.Fatalf("sort_basis=%q", cc.SortBasis)
	}
}

func TestPrompt96SelectedCampaignNotFoundFails(t *testing.T) {
	src := FactualAPIDetailSource{API: stubBatchDetailAPI{
		campaignTargetBatch: factualapi.BatchCampaignTargetSKUResult{
			Items: []factualapi.BatchCampaignTargetSKUItem{
				{CampaignID: 99, Status: factualapi.BatchNotFound},
			},
		},
	}}
	cutoff := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cutoff,
		Policies:        factualapi.CurrentPolicies(),
		Sealed:          stubSeal{id: "u1", hash: stringsRepeat("a", 64)},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "ADVERTISING.TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
			{CapabilityKey: "ADVERTISING.CAMPAIGN_CONFIG", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		},
		Domains: []factualapi.DomainBinding{
			{Domain: "ADVERTISING", FactFamily: "ADVERTISING.TRAFFIC", DomainRunID: 1, FinalizedAt: factualapi.FinalizedAtPtr(cutoff.Add(-time.Hour)), ContractHash: stringsRepeat("h", 64), CoverageTo: timePtr(cutoff.AddDate(0, 0, -1))},
		},
	}, stubSeal{id: "u1", hash: stringsRepeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = src.LoadCampaignDetailExtras(context.Background(), sealed, []int64{99}, CampaignDetailV4())
	if err == nil {
		t.Fatal("selected campaign BatchNotFound must fail BuildDetail, not READY_EMPTY")
	}
}
