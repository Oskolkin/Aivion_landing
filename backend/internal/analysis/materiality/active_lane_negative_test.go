package materiality

import (
	"reflect"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestActiveLaneManifestMatchesRegistry(t *testing.T) {
	reg := LaneRegistryV2Current()
	manifest := ActiveLaneManifest()
	if len(manifest) != len(reg.Versions) {
		t.Fatalf("manifest len=%d registry len=%d", len(manifest), len(reg.Versions))
	}
	byKey := map[string]ActiveLaneContract{}
	for _, c := range manifest {
		key := laneContractKey(c.EntityKind, c.LaneKey)
		if _, dup := byKey[key]; dup {
			t.Fatalf("duplicate manifest lane %s", key)
		}
		byKey[key] = c
	}
	for _, lv := range reg.Versions {
		key := laneContractKey(lv.EntityKind, lv.LaneKey)
		c, ok := byKey[key]
		if !ok {
			t.Fatalf("registry lane missing from manifest: %s", key)
		}
		if c.CapabilityKey != lv.RequiredCapability {
			t.Fatalf("lane %s capability=%s want %s", key, c.CapabilityKey, lv.RequiredCapability)
		}
		if c.ProfileName == "" || c.ProfileField == "" || c.FactFamily == "" || c.WindowPolicy == "" {
			t.Fatalf("lane %s incomplete contract: %#v", key, c)
		}
		if len(c.FactualMethods) == 0 {
			t.Fatalf("lane %s missing factual methods", key)
		}
	}
}

func TestNegativeLane_ZeroStockRequiresKnownStock(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "known-zero", CategorySemanticKey: "c", SalesAvailable: true, StockAvailable: true,
				SellableStock: stock("0"), OrderedUnitsPrevious7: count(10), OrderedAmountPrevious7: ex(50_000)},
			{SemanticKey: "unknown-stock", CategorySemanticKey: "c", SalesAvailable: true, StockAvailable: true,
				SellableStock: exUnknown(), OrderedUnitsPrevious7: count(10), OrderedAmountPrevious7: ex(50_000)},
		},
		Capabilities: baseCapabilities(),
	}
	ranked := AssignRanks(skuZeroSellable(core), DirectionDesc)
	for _, r := range ranked {
		if r.SemanticKey == "unknown-stock" {
			t.Fatalf("UNKNOWN stock must not qualify for ZERO_SELLABLE: %#v", ranked)
		}
	}
	if len(ranked) != 1 || ranked[0].SemanticKey != "known-zero" {
		t.Fatalf("expected only known-zero stock lane hit: %#v", ranked)
	}
}

func TestNegativeLane_SupplyOpenNotOverdue(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "open-not-overdue", CategorySemanticKey: "c", SalesAvailable: true,
				OrderedAmountCurrent7: ex(100_000), HasSupplyOverdue: false},
		},
		Categories: []CategoryFeatures{
			{SemanticKey: "cat-open", OrderedAmountCurrent7: ex(100_000), HasSupplyOverdue: false},
		},
		Capabilities: baseCapabilities(),
	}
	if len(skuSupplyOverdue(core)) != 0 {
		t.Fatalf("open supply must not enter SUPPLY_OVERDUE lane")
	}
	if len(catSupplyOverdue(core)) != 0 {
		t.Fatalf("open category supply must not enter SUPPLY_OVERDUE lane")
	}
}

func TestNegativeLane_SearchChangeRequiresBothPeriods(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000), OrderedAmountPrevious7: ex(1_000_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "missing-prev", CategorySemanticKey: "c", SalesAvailable: true, SearchAvailable: true,
				OrderedAmountCurrent7: ex(100_000), OrderedAmountPrevious7: ex(100_000),
				SearchPositionCurrent: stock("70"), SearchPositionPrevious: exUnknown()},
			{SemanticKey: "both-known", CategorySemanticKey: "c", SalesAvailable: true, SearchAvailable: true,
				OrderedAmountCurrent7: ex(100_000), OrderedAmountPrevious7: ex(100_000),
				SearchPositionCurrent: stock("70"), SearchPositionPrevious: stock("10")},
		},
		Capabilities: baseCapabilities(),
	}
	ranked := AssignRanks(skuSearchChange(core, CurrentPolicy()), DirectionDesc)
	for _, r := range ranked {
		if r.SemanticKey == "missing-prev" {
			t.Fatalf("missing previous search position must not qualify: %#v", ranked)
		}
	}
	if len(ranked) != 1 || ranked[0].SemanticKey != "both-known" {
		t.Fatalf("expected only both-known search lane hit: %#v", ranked)
	}
}

func TestInvalidDecimalFailsStage5Materiality(t *testing.T) {
	bad := canonicaljson.Decimal("not-a-decimal")
	set := landscape.LandscapeSet{
		Account: landscape.AccountLandscape{Identity: landscape.AccountIdentity{SellerAccountID: 1}},
		SKUs: []landscape.SKULandscape{{
			Identity: landscape.SKUIdentity{CanonicalSKUID: 42},
			Stock: &landscape.SKUStockBlock{
				State: landscape.BlockReady, CoverageDays7D: &bad,
			},
		}},
	}
	if _, err := CoreFactsFromLandscape(set); err == nil {
		t.Fatal("invalid decimal must fail CoreFactsFromLandscape (stage 5 input path)")
	}
}

func TestActiveLaneManifestFactualMethodsExistOnAPI(t *testing.T) {
	typ := reflect.TypeOf((*factualapi.API)(nil)).Elem()
	methods := map[string]struct{}{}
	for i := 0; i < typ.NumMethod(); i++ {
		methods[typ.Method(i).Name] = struct{}{}
	}
	for _, contract := range ActiveLaneManifest() {
		for _, method := range contract.FactualMethods {
			if _, ok := methods[method]; !ok {
				t.Fatalf("active lane %s (%s) references missing factual API method %s", contract.LaneKey, contract.EntityKind, method)
			}
		}
	}
}
