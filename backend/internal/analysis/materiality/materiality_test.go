package materiality

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"
)

func ex(v int64) MoneyFeature { return KnownExact(Exact(big.NewInt(v).String())) }

func exZero() MoneyFeature { return KnownExact(Exact("0")) }

func exUnknown() MoneyFeature { return MoneyFeature{State: FeatureUnknown} }

func count(v int64) CountFeature { return KnownCount(v) }

func stock(v string) MoneyFeature { return KnownExact(Exact(v)) }

func baseCapabilities() map[string]string {
	return map[string]string{
		"SALES.SKU_DEMAND":                 ReadinessReady,
		"SALES.CATEGORY_DEMAND":            ReadinessReady,
		"SALES.ACCOUNT_DEMAND":             ReadinessReady,
		"STOCK.SKU_CURRENT":                ReadinessReady,
		"STOCK.COVERAGE_7D":                ReadinessReady,
		"ADVERTISING.SKU_TRAFFIC":          ReadinessReady,
		"ADVERTISING.CAMPAIGN_TRAFFIC":     ReadinessReady,
		"ADVERTISING.CAMPAIGN_ATTRIBUTION": ReadinessReady,
		"ADVERTISING.CAMPAIGN_DRR":         ReadinessReady,
		"PRICE.CUSTOMER_PRICE":             ReadinessReady,
		"SEARCH.SELLER_SUMMARY":            ReadinessReady,
		"SUPPLY.STATE":                     ReadinessReady,
	}
}

func TestExposureGuardTinyRelativeVsLargeAbsolute(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{
			SemanticKey: "acct", OrderedAmountCurrent7: ex(10_000_000), OrderedAmountPrevious7: ex(10_000_000),
		},
		SKUs: []SKUFeatures{
			{
				SemanticKey: "tiny", CategorySemanticKey: "catA", SalesAvailable: true,
				OrderedAmountPrevious7: ex(10), OrderedAmountCurrent7: ex(100), // +900%
			},
			{
				SemanticKey: "large", CategorySemanticKey: "catA", SalesAvailable: true,
				OrderedAmountPrevious7: ex(1_000_000), OrderedAmountCurrent7: ex(800_000), // -20%
			},
		},
		Capabilities: baseCapabilities(),
	}
	policy := CurrentPolicy()
	relInc := skuSalesRelDelta(core, policy, true)
	relDec := skuSalesRelDelta(core, policy, false)
	for _, r := range relInc {
		if r.SemanticKey == "tiny" {
			t.Fatalf("tiny +900%% must not enter relative-increase lane")
		}
	}
	foundLarge := false
	for _, r := range relDec {
		if r.SemanticKey == "large" {
			foundLarge = true
		}
	}
	if !foundLarge {
		t.Fatalf("large absolute-share decrease must qualify for relative-decrease")
	}
	absInc := AssignRanks(skuSalesAbsDelta(core, true), DirectionDesc)
	if len(absInc) == 0 || absInc[0].SemanticKey != "tiny" {
		_ = absInc
	}
}

func TestFromZeroToZero(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "from_big", CategorySemanticKey: "c", SalesAvailable: true, OrderedAmountPrevious7: ex(0), OrderedAmountCurrent7: ex(500_000)},
			{SemanticKey: "from_small", CategorySemanticKey: "c", SalesAvailable: true, OrderedAmountPrevious7: ex(0), OrderedAmountCurrent7: ex(500)},
			{SemanticKey: "to_zero", CategorySemanticKey: "c", SalesAvailable: true, OrderedAmountPrevious7: ex(700_000), OrderedAmountCurrent7: ex(0)},
		},
		Capabilities: baseCapabilities(),
	}
	from := AssignRanks(skuSalesFromZero(core), DirectionDesc)
	if len(from) < 2 || from[0].SemanticKey != "from_big" {
		t.Fatalf("FROM_ZERO must rank by current amount: got %#v", from)
	}
	to := AssignRanks(skuSalesToZero(core), DirectionDesc)
	if len(to) != 1 || to[0].SemanticKey != "to_zero" {
		t.Fatalf("TO_ZERO must keep previous-positive SKU: got %#v", to)
	}
}

func TestMissingCapabilityDisablesLane(t *testing.T) {
	caps := baseCapabilities()
	delete(caps, "SEARCH.SELLER_SUMMARY")
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000), OrderedAmountPrevious7: ex(1_000_000)},
		SKUs: []SKUFeatures{{
			SemanticKey: "s1", CategorySemanticKey: "c", SalesAvailable: true, SearchAvailable: true,
			OrderedAmountCurrent7: ex(100_000), OrderedAmountPrevious7: ex(100_000),
			SearchPositionCurrent: stock("70"), SearchPositionPrevious: stock("10"),
		}},
		Capabilities: caps,
	}
	policy := CurrentPolicy()
	ranking := BuildMaterialityRanking(core, policy, "")
	for _, e := range ranking.SKUs {
		for _, m := range e.LaneMemberships {
			if m.LaneKey == "SEARCH_POSITION_CHANGE" {
				t.Fatalf("SEARCH lane must be disabled when capability missing")
			}
		}
	}
}

func TestBudgetSeparationHashes(t *testing.T) {
	core := fixtureCore()
	policy := CurrentPolicy()
	r := BuildMaterialityRanking(core, policy, "")
	b1 := DiscoveryStandardBudget()
	b2 := DiscoveryStandardBudget()
	b2.MaxSKUs = 5
	s1 := SelectMaterialEntities(r, b1, core.Account)
	s2 := SelectMaterialEntities(r, b2, core.Account)
	if s1.RankingHash != s2.RankingHash || s1.RankingHash != r.RankingHash {
		t.Fatalf("ranking_hash must be independent of budget")
	}
	if s1.SelectionHash == s2.SelectionHash {
		t.Fatalf("selection_hash must change when budget changes")
	}
	if b1.Hash() == b2.Hash() {
		t.Fatalf("budget hashes must differ")
	}
}

func TestRRFDeterminism(t *testing.T) {
	memberships := map[string][]LaneMembership{
		"a": {{LaneKey: "L1", Rank: 1}, {LaneKey: "L2", Rank: 3}},
		"b": {{LaneKey: "L1", Rank: 2}, {LaneKey: "L2", Rank: 1}},
		"c": {{LaneKey: "L1", Rank: 3}},
	}
	weights := map[string]*big.Rat{"L1": ratInt(1), "L2": ratInt(1)}
	u1 := WeightedRRF(60, memberships, weights)
	u2 := WeightedRRF(60, memberships, weights)
	o1 := OrderByFusion(u1)
	o2 := OrderByFusion(u2)
	if strings.Join(o1, ",") != strings.Join(o2, ",") {
		t.Fatalf("RRF order not deterministic: %v vs %v", o1, o2)
	}
	eq := map[string]Exact{"z": Exact("0.5"), "a": Exact("0.5")}
	ord := OrderByFusion(eq)
	if ord[0] != "a" || ord[1] != "z" {
		t.Fatalf("tie-break must be semantic key ASC: %v", ord)
	}
}

func TestDiversitySoftNoHardCap(t *testing.T) {
	cands := []DiversityCandidate{
		{SemanticKey: "a1", CategoryKey: "A", FusionUtility: Exact("1.0"), IsAnchor: true, LaneFingerprint: []string{"SALES"}},
		{SemanticKey: "a2", CategoryKey: "A", FusionUtility: Exact("0.9"), LaneFingerprint: []string{"SALES"}},
		{SemanticKey: "a3", CategoryKey: "A", FusionUtility: Exact("0.85"), LaneFingerprint: []string{"SALES"}},
		{SemanticKey: "b1", CategoryKey: "B", FusionUtility: Exact("0.2"), LaneFingerprint: []string{"ADS"}},
	}
	order := SoftDiversityOrder(cands, CurrentPolicy().Diversity)
	if order[0] != "a1" {
		t.Fatalf("anchor must stay first: %v", order)
	}
	idx := map[string]int{}
	for i, k := range order {
		idx[k] = i
	}
	if _, ok := idx["a2"]; !ok {
		t.Fatalf("dominant category SKU must not be hard-capped out")
	}
	if len(order) != 4 {
		t.Fatalf("soft diversity must keep all candidates, got %v", order)
	}
}

func TestSoftDiversityDoesNotOverturnLargeGap(t *testing.T) {
	p := CurrentPolicy().Diversity
	// Prove: utility 0.020 vs 0.010 cannot reorder under normal (max) penalties.
	high := DiversityCandidate{SemanticKey: "high", CategoryKey: "A", FusionUtility: Exact("0.020"), LaneFingerprint: []string{"L1", "L2"}}
	low := DiversityCandidate{SemanticKey: "low", CategoryKey: "B", FusionUtility: Exact("0.010"), LaneFingerprint: []string{"L3"}}
	// Pretend category A and lanes already selected so high takes max relative penalty; low gets new-category bonus.
	hs := adjustedUtility(high, map[string]int{"A": 3}, map[string]int{"L1": 1, "L2": 1}, p)
	ls := adjustedUtility(low, map[string]int{"A": 1}, map[string]int{}, p)
	if cmpRat(hs, ls) <= 0 {
		t.Fatalf("large fusion gap must survive max relative penalty: high=%s low=%s", formatRat(hs), formatRat(ls))
	}
	// Close values 0.0200 vs 0.0198 may reorder under penalties.
	closeHigh := DiversityCandidate{SemanticKey: "ch", CategoryKey: "A", FusionUtility: Exact("0.0200"), LaneFingerprint: []string{"L1"}}
	closeLow := DiversityCandidate{SemanticKey: "cl", CategoryKey: "B", FusionUtility: Exact("0.0198"), LaneFingerprint: []string{"L2"}}
	chs := adjustedUtility(closeHigh, map[string]int{"A": 2}, map[string]int{"L1": 1}, p)
	cls := adjustedUtility(closeLow, map[string]int{"A": 1}, map[string]int{}, p)
	if cmpRat(chs, cls) >= 0 {
		t.Fatalf("close utilities must be able to reorder under relative penalties: high=%s low=%s", formatRat(chs), formatRat(cls))
	}
	order := SoftDiversityOrder([]DiversityCandidate{
		{SemanticKey: "a0", CategoryKey: "A", FusionUtility: Exact("1"), IsAnchor: true, LaneFingerprint: []string{"X"}},
		closeHigh,
		closeLow,
	}, p)
	if order[0] != "a0" {
		t.Fatalf("anchor protected: %v", order)
	}
}

func TestAnchorsUseCurrentExposureOnly(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(1_000_000), PaidSpendCurrent7: ex(100_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "sku-alive", CategorySemanticKey: "c", SalesAvailable: true, OrderedAmountCurrent7: ex(400_000), OrderedAmountPrevious7: ex(400_000)},
			{SemanticKey: "sku-to-zero", CategorySemanticKey: "c", SalesAvailable: true, OrderedAmountCurrent7: ex(0), OrderedAmountPrevious7: ex(700_000)},
		},
		Campaigns: []CampaignFeatures{
			{SemanticKey: "camp-on", TrafficAvailable: true, PaidSpendCurrent7: ex(40_000), PaidSpendPrevious7: ex(40_000)},
			{SemanticKey: "camp-off", TrafficAvailable: true, PaidSpendCurrent7: ex(0), PaidSpendPrevious7: ex(150_000)},
		},
		Categories: []CategoryFeatures{
			{SemanticKey: "cat1", OrderedAmountCurrent7: ex(400_000), OrderedAmountPrevious7: ex(1_000_000)},
		},
		Capabilities: baseCapabilities(),
	}
	policy := CurrentPolicy()
	skuAnchors := buildAnchors(EntitySKU, core, policy)
	for _, a := range skuAnchors {
		if a == "sku-to-zero" {
			t.Fatalf("TO_ZERO SKU with current=0 must not be current-exposure anchor: %v", skuAnchors)
		}
	}
	campAnchors := buildAnchors(EntityCampaign, core, policy)
	for _, a := range campAnchors {
		if a == "camp-off" {
			t.Fatalf("TO_ZERO campaign with current=0 must not be current-exposure anchor: %v", campAnchors)
		}
	}
	r := BuildMaterialityRanking(core, policy, "")
	for _, e := range r.SKUs {
		if e.SemanticKey == "sku-to-zero" && e.IsAnchor {
			t.Fatalf("sku-to-zero must not be ranked as anchor")
		}
		if e.SemanticKey == "sku-to-zero" && exactPositive(e.ExposureMetric) {
			t.Fatalf("exposure meta must be current-only (0), got %s", e.ExposureMetric)
		}
	}
}

func TestRankingHashDiffersForSameOrderDifferentFacts(t *testing.T) {
	base := fixtureCore()
	r1 := BuildMaterialityRanking(base, CurrentPolicy(), "core-hash-a")
	alt := fixtureCore()
	// Same entity set / likely same order, but different money facts.
	alt.SKUs[0].OrderedAmountCurrent7 = ex(401_000)
	alt.Account.OrderedAmountCurrent7 = ex(2_001_000)
	r2 := BuildMaterialityRanking(alt, CurrentPolicy(), "core-hash-a")
	// Even if diversified order matches, semantic ranking_hash must differ via metrics/core facts.
	sameOrder := true
	if len(r1.SKUs) != len(r2.SKUs) {
		sameOrder = false
	} else {
		for i := range r1.SKUs {
			if r1.SKUs[i].SemanticKey != r2.SKUs[i].SemanticKey {
				sameOrder = false
				break
			}
		}
	}
	if !sameOrder {
		// Still require hash difference when facts differ.
	}
	if r1.RankingHash == "" || r2.RankingHash == "" {
		t.Fatalf("ranking hashes must be non-empty")
	}
	if r1.RankingHash == r2.RankingHash {
		t.Fatalf("same entity order with different money facts must yield different ranking_hash")
	}
	// Different input_core_hash also changes ranking_hash.
	r3 := BuildMaterialityRanking(base, CurrentPolicy(), "core-hash-b")
	if r1.RankingHash == r3.RankingHash {
		t.Fatalf("different input_core_hash must change ranking_hash")
	}
}

func TestAccountAlwaysIncluded(t *testing.T) {
	core := fixtureCore()
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	s := SelectMaterialEntities(r, DiscoveryStandardBudget(), core.Account)
	if s.Account.EntityKind != EntityAccount || s.Account.SemanticKey == "" {
		t.Fatalf("account must always be selected: %#v", s.Account)
	}
	audit := ExplainSelection(r, s, EntityAccount, s.Account.SemanticKey)
	if !audit.FinalSelected || audit.ReasonCodes[0] != "ACCOUNT_ALWAYS_INCLUDED" {
		t.Fatalf("account audit: %#v", audit)
	}
}

func TestNoSearchQuerySelection(t *testing.T) {
	core := fixtureCore()
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	s := SelectMaterialEntities(r, DiscoveryStandardBudget(), core.Account)
	for _, e := range append(append(s.Categories, s.SKUs...), s.Campaigns...) {
		if e.EntityKind == "SEARCH_QUERY" || strings.Contains(e.SemanticKey, "query:") {
			t.Fatalf("SEARCH_QUERY must not be selected")
		}
	}
	audit := ExplainSelection(r, s, EntityKind("SEARCH_QUERY"), "q1")
	if audit.ReasonCodes[0] != "ENTITY_KIND_NOT_MATERIAL" {
		t.Fatalf("expected NOT_MATERIAL, got %#v", audit)
	}
}

func TestAuditNotInSelectionArtifact(t *testing.T) {
	core := fixtureCore()
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	s := SelectMaterialEntities(r, DiscoveryStandardBudget(), core.Account)
	art := NewSelectionArtifact(s)
	b, err := json.Marshal(art)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(b)
	for _, banned := range []string{"fusion_utility", "reason_codes", "lane_memberships", "audit"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("selection artifact must not leak audit field %q", banned)
		}
	}
}

func TestLaneRegistrySKUCoverage(t *testing.T) {
	reg := LaneRegistryV1Current()
	need := []string{
		"SALES_CURRENT_EXPOSURE", "SALES_30D_EXPOSURE", "SALES_ABSOLUTE_INCREASE", "SALES_ABSOLUTE_DECREASE",
		"SALES_RELATIVE_INCREASE", "SALES_RELATIVE_DECREASE", "SALES_FROM_ZERO", "SALES_TO_ZERO",
		"ZERO_SELLABLE_WITH_RECENT_DEMAND", "STOCK_COVERAGE_ASC",
		"AD_PAID_SPEND_EXPOSURE", "AD_PAID_SPEND_INCREASE", "AD_PAID_SPEND_DECREASE",
		"PRICE_ABSOLUTE_CHANGE", "SEARCH_POSITION_CHANGE", "SUPPLY_OVERDUE_EXPOSURE",
	}
	have := map[string]bool{}
	for _, lv := range reg.LanesForKind(EntitySKU) {
		have[lv.LaneKey] = true
		if lv.TypedImplKey == "" || lv.RequiredCapability == "" {
			t.Fatalf("lane %s missing impl/capability", lv.LaneKey)
		}
	}
	for _, k := range need {
		if !have[k] {
			t.Fatalf("missing SKU lane %s", k)
		}
	}
}

func TestCurrentPolicyIsV1_4(t *testing.T) {
	p := CurrentPolicy()
	if p.Version != MaterialityPolicyV1_4 {
		t.Fatalf("CurrentPolicy version=%s want %s", p.Version, MaterialityPolicyV1_4)
	}
	if p.LaneRegistryVersion != LaneRegistryV2 {
		t.Fatalf("CurrentPolicy lane registry=%s want %s", p.LaneRegistryVersion, LaneRegistryV2)
	}
	if PolicyV1_2().Version != MaterialityPolicyV1_2 {
		t.Fatalf("PolicyV1_2 must remain frozen at %s", MaterialityPolicyV1_2)
	}
	if p.Diversity.MaxPenaltyFraction <= 0 {
		t.Fatalf("V1_4 diversity must use relative MaxPenaltyFraction")
	}
}

func fixtureCore() CoreFacts {
	return CoreFacts{
		Account: AccountFeatures{
			SemanticKey: "acct-1", OrderedAmountCurrent7: ex(2_000_000), OrderedAmountPrevious7: ex(1_800_000),
			OrderedAmount30D: ex(7_000_000), PaidSpendCurrent7: ex(200_000), PaidSpendPrevious7: ex(180_000),
			AttributedAmountCurrent7: ex(500_000),
		},
		Categories: []CategoryFeatures{
			{SemanticKey: "cat-core", OrderedAmountCurrent7: ex(1_200_000), OrderedAmountPrevious7: ex(1_100_000), PaidSpendCurrent7: ex(100_000)},
			{SemanticKey: "cat-tail", OrderedAmountCurrent7: ex(50_000), OrderedAmountPrevious7: ex(40_000)},
		},
		SKUs: []SKUFeatures{
			{SemanticKey: "sku-a", CategorySemanticKey: "cat-core", SalesAvailable: true, AdsAvailable: true,
				OrderedAmountCurrent7: ex(400_000), OrderedAmountPrevious7: ex(350_000), OrderedAmount30D: ex(1_500_000), PaidSpendCurrent7: ex(40_000)},
			{SemanticKey: "sku-b", CategorySemanticKey: "cat-core", SalesAvailable: true, StockAvailable: true,
				OrderedAmountCurrent7: ex(0), OrderedAmountPrevious7: ex(700_000), OrderedUnitsPrevious7: count(50), SellableStock: stock("0")},
			{SemanticKey: "sku-c", CategorySemanticKey: "cat-tail", SalesAvailable: true,
				OrderedAmountCurrent7: ex(80_000), OrderedAmountPrevious7: ex(20_000)},
		},
		Campaigns: []CampaignFeatures{
			{SemanticKey: "camp-1", TrafficAvailable: true, AttributionAvailable: true,
				PaidSpendCurrent7: ex(80_000), PaidSpendPrevious7: ex(50_000), AttributedAmountCurrent7: ex(200_000)},
			{SemanticKey: "camp-2", TrafficAvailable: true,
				PaidSpendCurrent7: ex(0), PaidSpendPrevious7: ex(30_000)},
		},
		Capabilities: baseCapabilities(),
	}
}
