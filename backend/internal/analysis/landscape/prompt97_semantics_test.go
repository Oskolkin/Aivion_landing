package landscape

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestAccountAdImpressionsUnknownNotZero(t *testing.T) {
	feed := &CoreFeed{
		GlobalCapabilities: []CapabilityFlag{{Key: "ADVERTISING.CAMPAIGN_TRAFFIC", FinalState: "READY"}},
		Campaigns: []CampaignLandscape{{
			Identity: CampaignIdentity{CampaignID: 1},
			Performance: &CampaignPerfBlock{
				Traffic: &CampaignTrafficBlock{
					State:     BlockReady,
					PaidSpend: MoneyComparison{Current: &MoneyAmount{Amount: "100", CurrencyCode: "RUB"}},
				},
			},
		}},
	}
	deriveAccountAdvertising(feed)
	if feed.Account.Advertising == nil || feed.Account.Advertising.Traffic == nil {
		t.Fatal("expected account traffic")
	}
	if feed.Account.Advertising.Traffic.Impressions.Current != nil {
		t.Fatalf("unknown impressions became %v", *feed.Account.Advertising.Traffic.Impressions.Current)
	}
}

func TestCampaignCountWithSpendZeroSafe(t *testing.T) {
	zero := &MoneyAmount{Amount: "0", CurrencyCode: "RUB"}
	pos := &MoneyAmount{Amount: "100", CurrencyCode: "RUB"}
	feed := &CoreFeed{
		GlobalCapabilities: []CapabilityFlag{{Key: "ADVERTISING.CAMPAIGN_TRAFFIC", FinalState: "READY"}},
		Campaigns: []CampaignLandscape{
			{Identity: CampaignIdentity{CampaignID: 1}, Performance: &CampaignPerfBlock{Traffic: &CampaignTrafficBlock{PaidSpend: MoneyComparison{Current: zero}}}},
			{Identity: CampaignIdentity{CampaignID: 2}, Performance: &CampaignPerfBlock{Traffic: &CampaignTrafficBlock{PaidSpend: MoneyComparison{Current: pos}}}},
			{Identity: CampaignIdentity{CampaignID: 3}, Performance: &CampaignPerfBlock{Traffic: &CampaignTrafficBlock{}}},
		},
	}
	deriveAccountAdvertising(feed)
	if feed.Account.Advertising.CampaignCountWithSpend == nil || *feed.Account.Advertising.CampaignCountWithSpend != 1 {
		t.Fatalf("withSpend=%v want 1", feed.Account.Advertising.CampaignCountWithSpend)
	}
}

func TestCampaignStockBoundedContextHonest(t *testing.T) {
	ids := make([]int64, 10)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	block := buildCampaignStockBlock(100, ids, true, nil)
	if block.AssignedSKUCount == nil || *block.AssignedSKUCount != 100 {
		t.Fatalf("assigned=%v want 100", block.AssignedSKUCount)
	}
	if block.ContextSKUCount == nil || *block.ContextSKUCount != 10 {
		t.Fatalf("context=%v want 10", block.ContextSKUCount)
	}
	if !block.ContextTruncated {
		t.Fatal("expected ContextTruncated")
	}
	if block.State != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", block.State)
	}
	found := false
	for _, lim := range block.Limitations {
		if lim.Code == "CAMPAIGN_STOCK_BOUNDED_CONTEXT" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing CAMPAIGN_STOCK_BOUNDED_CONTEXT: %+v", block.Limitations)
	}
}

func TestDomainWindowsSortDeterministic(t *testing.T) {
	mk := func(order []DomainWindow) LandscapeSet {
		return LandscapeSet{
			Context:       LandscapeContext{SellerAccountID: 1, LandscapeSchemaVersion: LandscapeSchemaV4},
			Account:       AccountLandscape{Identity: AccountIdentity{SellerAccountID: 1}},
			DomainWindows: append([]DomainWindow(nil), order...),
		}
	}
	a := DomainWindow{Domain: "advertising", FactFamily: "ADVERTISING.TRAFFIC"}
	b := DomainWindow{Domain: "advertising", FactFamily: "ADVERTISING.ATTRIBUTION"}
	c := DomainWindow{Domain: "sales"}
	_, h1, err := MarshalAndHash(mk([]DomainWindow{a, b, c}))
	if err != nil {
		t.Fatal(err)
	}
	_, h2, err := MarshalAndHash(mk([]DomainWindow{c, b, a}))
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("shuffled DomainWindows changed hash: %s vs %s", h1, h2)
	}
}

func TestFunnelAllLimitedNotReadyEmpty(t *testing.T) {
	st := FunnelBuildState{LimitedMetricCount: 12, Limitations: []Limitation{{Code: "FUNNEL_LIMITED"}}}
	state, lims := finalizeFunnelState(st)
	if state != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", state)
	}
	if len(lims) == 0 {
		t.Fatal("expected limitations")
	}
	cur := map[string]map[int64]skuFunnelObs{}
	prev := map[string]map[int64]skuFunnelObs{}
	for _, metric := range []string{"SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION", "PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION"} {
		cur[metric] = map[int64]skuFunnelObs{1: {Status: factualapi.BatchLimited}}
		prev[metric] = map[int64]skuFunnelObs{1: {Status: factualapi.BatchLimited}}
	}
	block := buildFunnelBlockFromSKUBatches(1, cur, prev, nil, nil)
	if block.State != BlockPartial {
		t.Fatalf("sku funnel state=%s want PARTIAL", block.State)
	}
}

func TestMoneyIsKnownPositive(t *testing.T) {
	if moneyIsKnownPositive(nil) || moneyIsKnownPositive(&MoneyAmount{Amount: "0", CurrencyCode: "RUB"}) {
		t.Fatal("nil/zero must not be known-positive")
	}
	if !moneyIsKnownPositive(&MoneyAmount{Amount: "100", CurrencyCode: "RUB"}) {
		t.Fatal("100 must be known-positive")
	}
}

func TestCountAccumulatorUnknownNotZero(t *testing.T) {
	var a CountAccumulator
	a.Observe(nil)
	a.Observe(nil)
	if a.Result() != nil {
		t.Fatal("UNKNOWN+UNKNOWN must be nil")
	}
	var b CountAccumulator
	z := int64(0)
	b.Observe(&z)
	b.Observe(&z)
	if b.Result() == nil || *b.Result() != 0 {
		t.Fatalf("KNOWN 0 + KNOWN 0 must be 0, got %v", b.Result())
	}
	var c CountAccumulator
	ten := int64(10)
	c.Observe(&ten)
	c.Observe(nil)
	if c.Result() == nil || *c.Result() != 10 || !c.Incomplete() {
		t.Fatalf("mixed known/unknown: result=%v incomplete=%v", c.Result(), c.Incomplete())
	}
}

func TestFunnelUnknownNotReadyEmpty(t *testing.T) {
	st := FunnelBuildState{MissingMetricCount: 12}
	state, _ := finalizeFunnelState(st)
	if state != BlockPartial {
		t.Fatalf("unknown/missing funnel state=%s want PARTIAL", state)
	}
}
