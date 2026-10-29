package landscape

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type capTestSeal struct{ id, hash string }

func (s capTestSeal) SealedBindingHash() string { return s.hash }
func (s capTestSeal) SealedUniverseID() string  { return s.id }

func sealedContextForCapabilityTest(t *testing.T) factualapi.FactualReadContext {
	t.Helper()
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    cut.AddDate(0, 0, -1),
		CutoffAt:        cut,
		Policies:        factualapi.V1Policies(),
	}, capTestSeal{id: "u1", hash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func TestCapabilityMissingIsNotUsable(t *testing.T) {
	sealed := sealedContextForCapabilityTest(t)
	ok, state, reason := capabilityUsable(sealed, "SALES.ACCOUNT_DEMAND")
	if ok {
		t.Fatal("missing capability must not be usable")
	}
	if state != "MISSING" || reason != "CAPABILITY_NOT_BOUND" {
		t.Fatalf("got state=%q reason=%q", state, reason)
	}
}

func TestRequiredProfileCapabilityMissingFails(t *testing.T) {
	sealed := sealedContextForCapabilityTest(t)
	if err := ValidateCoreProfileCapabilitiesV3(sealed, DefaultCoreProfilesV3()); err == nil {
		t.Fatal("expected required capability validation failure")
	}
}

func TestOptionalCapabilityReadyBlockMissingFails(t *testing.T) {
	profile := SKUCoreV3()
	present := map[string]bool{
		"sku.sales": true,
		"sku.stock": true,
		"sku.price": true,
	}
	caps := []CapabilityFlag{
		{Key: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "PRICE.CUSTOMER_PRICE", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "ADVERTISING.SKU_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
	}
	err := validateEntityProfileConformanceV3(present, profile, caps)
	if err == nil {
		t.Fatal("optional capability READY but block omitted must fail")
	}
}

func TestOptionalCapabilityUnavailableBlockPresentFails(t *testing.T) {
	profile := SKUCoreV3()
	present := map[string]bool{
		"sku.sales":       true,
		"sku.stock":       true,
		"sku.price":       true,
		"sku.advertising": true,
	}
	caps := []CapabilityFlag{
		{Key: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "PRICE.CUSTOMER_PRICE", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
	}
	err := validateEntityProfileConformanceV3(present, profile, caps)
	if err == nil {
		t.Fatal("optional capability unavailable but block present must fail")
	}
}

func TestOptionalCapabilityUnavailableBlockOmittedPasses(t *testing.T) {
	profile := SKUCoreV3()
	present := map[string]bool{
		"sku.sales": true,
		"sku.stock": true,
		"sku.price": true,
	}
	caps := []CapabilityFlag{
		{Key: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "PRICE.CUSTOMER_PRICE", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
	}
	if err := validateEntityProfileConformanceV3(present, profile, caps); err != nil {
		t.Fatalf("optional block omitted when capability unavailable should pass: %v", err)
	}
}

func sealedSKUCoreContext(t *testing.T, searchState string) factualapi.FactualReadContext {
	t.Helper()
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	analysisDate := cut.AddDate(0, 0, -1)
	bindings := []factualapi.CapabilityBinding{
		{CapabilityKey: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{CapabilityKey: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{CapabilityKey: "PRICE.CUSTOMER_PRICE", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
	}
	if searchState != "" {
		bindings = append(bindings, factualapi.CapabilityBinding{
			CapabilityKey: "SEARCH.SELLER_SUMMARY", Scope: "HISTORICAL_QUERY", FinalState: searchState,
		})
	}
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    analysisDate,
		CutoffAt:        cut,
		Policies:        factualapi.V1Policies(),
		Sealed:          capTestSeal{id: "u1", hash: strings.Repeat("a", 64)},
		Capabilities:    bindings,
		Domains: []factualapi.DomainBinding{{
			Domain: "SEARCH", FactFamily: "SEARCH.SELLER_SUMMARY", DomainRunID: 1,
			FinalizedAt: factualapi.FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: strings.Repeat("b", 64),
			CoverageTo: &analysisDate,
		}},
	}, capTestSeal{id: "u1", hash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

func minimalSKUProfiles() CoreProfileBundleV3 {
	idOnly := func(name, blockID string) ExecutableProfileV3 {
		return ExecutableProfileV3{
			Name: name, SchemaVersion: LandscapeSchemaV3,
			Blocks: []BlockSpecV3{requiredBlock(blockID, nil, nil, nil)},
		}
	}
	return CoreProfileBundleV3{
		Account:  idOnly(ProfileAccountDiscoveryV3, "account.identity"),
		Category: idOnly(ProfileCategoryCoreV3, "category.identity"),
		SKU:      SKUCoreV3(),
		Campaign: idOnly(ProfileCampaignCoreV3, "campaign.identity"),
	}
}

type searchBatchSpy struct {
	factualapi.API
	searchCalls atomic.Int64
}

func (s *searchBatchSpy) GetAccount(context.Context, factualapi.FactualReadContext) (factualapi.Account, error) {
	return factualapi.Account{SellerAccountID: 1}, nil
}
func (s *searchBatchSpy) ListCategories(context.Context, factualapi.FactualReadContext, factualapi.PageRequest) (factualapi.Page[factualapi.Category], error) {
	return factualapi.Page[factualapi.Category]{}, nil
}
func (s *searchBatchSpy) ListSKUs(context.Context, factualapi.FactualReadContext, factualapi.SKUFilter, factualapi.PageRequest) (factualapi.Page[factualapi.SKU], error) {
	return factualapi.Page[factualapi.SKU]{Items: []factualapi.SKU{{
		ID: 42, OzonSKU: 4242, CatalogState: "ACTIVE",
		Category: factualapi.CategoryRelation{CategoryID: int64PtrTest(7), MappingStatus: "MAPPED"},
	}}}, nil
}
func (s *searchBatchSpy) ListCampaigns(context.Context, factualapi.FactualReadContext, factualapi.CampaignFilter, factualapi.PageRequest) (factualapi.Page[factualapi.Campaign], error) {
	return factualapi.Page[factualapi.Campaign]{}, nil
}
func (s *searchBatchSpy) BatchGetSKUDemand(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUDemandResult, error) {
	return factualapi.BatchSKUDemandResult{}, nil
}
func (s *searchBatchSpy) BatchGetSKUStock(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUStockResult, error) {
	return factualapi.BatchSKUStockResult{}, nil
}
func (s *searchBatchSpy) BatchGetSKUPrice(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUPriceResult, error) {
	return factualapi.BatchSKUPriceResult{}, nil
}
func (s *searchBatchSpy) BatchGetSKUSearchSummary(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUSearchSummaryResult, error) {
	s.searchCalls.Add(1)
	return factualapi.BatchSKUSearchSummaryResult{}, nil
}
func (s *searchBatchSpy) BatchGetCampaignPeriodPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchCampaignPeriodPerformanceResult, error) {
	return factualapi.BatchCampaignPeriodPerformanceResult{}, nil
}
func (s *searchBatchSpy) BatchListCampaignTargetSKUs(context.Context, factualapi.FactualReadContext, []int64, int) (factualapi.BatchCampaignTargetSKUResult, error) {
	return factualapi.BatchCampaignTargetSKUResult{}, nil
}

func TestOptionalCapabilityMissingSkipsFactualCall(t *testing.T) {
	sealed := sealedSKUCoreContext(t, "")
	fetch, err := profileBlockExecutes(sealed, SKUCoreV3(), "sku.search")
	if err != nil || fetch {
		t.Fatalf("missing search capability must not authorize fetch: fetch=%v err=%v", fetch, err)
	}
	spy := &searchBatchSpy{}
	if fetch {
		_, _ = spy.BatchGetSKUSearchSummary(context.Background(), sealed, []int64{42}, factualapi.PeriodCurrent7D, nil)
	}
	if spy.searchCalls.Load() != 0 {
		t.Fatalf("BatchGetSKUSearchSummary calls=%d want 0", spy.searchCalls.Load())
	}
}

func TestOptionalCapabilityUnavailableSkipsFactualCall(t *testing.T) {
	sealed := sealedSKUCoreContext(t, "FAILED")
	fetch, err := profileBlockExecutes(sealed, SKUCoreV3(), "sku.search")
	if err != nil || fetch {
		t.Fatalf("unavailable search capability must not authorize fetch: fetch=%v err=%v", fetch, err)
	}
	spy := &searchBatchSpy{}
	if fetch {
		_, _ = spy.BatchGetSKUSearchSummary(context.Background(), sealed, []int64{42}, factualapi.PeriodCurrent7D, nil)
	}
	if spy.searchCalls.Load() != 0 {
		t.Fatalf("BatchGetSKUSearchSummary calls=%d want 0", spy.searchCalls.Load())
	}
}

func TestUnavailableCapabilityCannotProduceFacts(t *testing.T) {
	searchErr := errors.New("search unavailable")
	block := buildSKUSearchBlock(1, nil, nil, searchErr, searchErr)
	if block.State != BlockUnavailable {
		t.Fatalf("state=%s want UNAVAILABLE", block.State)
	}
	if block.ProviderAveragePosition != nil || block.PreviousProviderAveragePosition != nil {
		t.Fatalf("UNAVAILABLE search must not carry positions: %#v", block)
	}
	if len(block.Limitations) == 0 {
		t.Fatal("UNAVAILABLE search must carry limitations")
	}
}

func int64PtrTest(v int64) *int64 { return &v }
