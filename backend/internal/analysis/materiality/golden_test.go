package materiality

import (
	"testing"
)

// goldenCase encodes MUST_INCLUDE / SHOULD_NOT_INCLUDE expectations for offline recall.
type goldenCase struct {
	Name            string
	Core            CoreFacts
	MustIncludeSKU  []string
	MustIncludeCat  []string
	MustIncludeCamp []string
	ShouldNotSKU    []string
	Critical        bool // critical cases require 100% MUST_INCLUDE recall
}

func TestGoldenSuiteRecall(t *testing.T) {
	cases := goldenSuite()
	var criticalHit, criticalTotal, generalHit, generalTotal int
	for _, c := range cases {
		r := BuildMaterialityRanking(c.Core, CurrentPolicy(), "")
		budget := DiscoveryStandardBudget()
		// Tight SKU budget when asserting exclusions so noise cannot ride empty slots.
		if n := len(c.MustIncludeSKU); n > 0 && len(c.ShouldNotSKU) > 0 {
			budget.MaxSKUs = n
			budget.AnchorMaxSKUs = n
		}
		sel := SelectMaterialEntities(r, budget, c.Core.Account)
		skuSet := map[string]bool{}
		for _, e := range sel.SKUs {
			skuSet[e.SemanticKey] = true
		}
		catSet := map[string]bool{}
		for _, e := range sel.Categories {
			catSet[e.SemanticKey] = true
		}
		campSet := map[string]bool{}
		for _, e := range sel.Campaigns {
			campSet[e.SemanticKey] = true
		}
		check := func(keys []string, set map[string]bool, label string) {
			for _, k := range keys {
				generalTotal++
				if c.Critical {
					criticalTotal++
				}
				ok := set[k]
				if ok {
					generalHit++
					if c.Critical {
						criticalHit++
					}
				} else {
					t.Errorf("%s: missing MUST_INCLUDE %s %s", c.Name, label, k)
				}
			}
		}
		check(c.MustIncludeSKU, skuSet, "SKU")
		check(c.MustIncludeCat, catSet, "CATEGORY")
		check(c.MustIncludeCamp, campSet, "CAMPAIGN")
		for _, k := range c.ShouldNotSKU {
			if skuSet[k] {
				t.Errorf("%s: SHOULD_NOT_INCLUDE SKU %s was selected", c.Name, k)
			}
			// Exposure guard: must not appear on relative-change lanes.
			for _, e := range r.SKUs {
				if e.SemanticKey != k {
					continue
				}
				for _, m := range e.LaneMemberships {
					if m.LaneKey == "SALES_RELATIVE_INCREASE" || m.LaneKey == "SALES_RELATIVE_DECREASE" {
						t.Errorf("%s: %s must not qualify for %s", c.Name, k, m.LaneKey)
					}
				}
			}
		}
		if sel.Account.SemanticKey == "" {
			t.Errorf("%s: account missing", c.Name)
		}
	}
	if criticalTotal > 0 && criticalHit != criticalTotal {
		t.Fatalf("critical recall %d/%d want 100%%", criticalHit, criticalTotal)
	}
	if generalTotal > 0 {
		ratio := float64(generalHit) / float64(generalTotal)
		if ratio < 0.95 {
			t.Fatalf("general recall %.2f%% < 95%% (%d/%d)", ratio*100, generalHit, generalTotal)
		}
	}
}

func goldenSuite() []goldenCase {
	caps := baseCapabilities()
	return []goldenCase{
		{
			Name:     "stable_core_anchors",
			Critical: true,
			Core: CoreFacts{
				Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(1_000_000), OrderedAmountPrevious7: ex(1_000_000), PaidSpendCurrent7: ex(100_000)},
				Categories: []CategoryFeatures{
					{SemanticKey: "cat-major", OrderedAmountCurrent7: ex(800_000), OrderedAmountPrevious7: ex(790_000)},
					{SemanticKey: "cat-minor", OrderedAmountCurrent7: ex(50_000), OrderedAmountPrevious7: ex(50_000)},
				},
				SKUs: []SKUFeatures{
					{SemanticKey: "sku-anchor", CategorySemanticKey: "cat-major", SalesAvailable: true, OrderedAmountCurrent7: ex(600_000), OrderedAmountPrevious7: ex(590_000), OrderedAmount30D: ex(2_000_000)},
					{SemanticKey: "sku-tiny", CategorySemanticKey: "cat-minor", SalesAvailable: true, OrderedAmountCurrent7: ex(100), OrderedAmountPrevious7: ex(10)},
				},
				Campaigns: []CampaignFeatures{
					{SemanticKey: "camp-anchor", TrafficAvailable: true, PaidSpendCurrent7: ex(90_000), PaidSpendPrevious7: ex(88_000)},
				},
				Capabilities: caps,
			},
			MustIncludeSKU:  []string{"sku-anchor"},
			MustIncludeCat:  []string{"cat-major"},
			MustIncludeCamp: []string{"camp-anchor"},
			ShouldNotSKU:    []string{"sku-tiny"}, // tiny +900% relative without exposure
		},
		{
			Name:     "to_zero_preserved",
			Critical: true,
			Core: CoreFacts{
				Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(500_000), OrderedAmountPrevious7: ex(1_200_000)},
				Categories: []CategoryFeatures{
					{SemanticKey: "cat1", OrderedAmountCurrent7: ex(400_000), OrderedAmountPrevious7: ex(1_000_000)},
				},
				SKUs: []SKUFeatures{
					{SemanticKey: "sku-died", CategorySemanticKey: "cat1", SalesAvailable: true, StockAvailable: true,
						OrderedAmountCurrent7: ex(0), OrderedAmountPrevious7: ex(700_000), OrderedUnitsPrevious7: count(100), SellableStock: stock("0")},
					{SemanticKey: "sku-alive", CategorySemanticKey: "cat1", SalesAvailable: true,
						OrderedAmountCurrent7: ex(400_000), OrderedAmountPrevious7: ex(400_000)},
				},
				Capabilities: caps,
			},
			MustIncludeSKU: []string{"sku-died", "sku-alive"},
			MustIncludeCat: []string{"cat1"},
		},
		{
			Name:     "from_zero_big_over_small",
			Critical: true,
			Core: CoreFacts{
				Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(600_000), OrderedAmountPrevious7: ex(50_000)},
				Categories: []CategoryFeatures{
					{SemanticKey: "cat1", OrderedAmountCurrent7: ex(550_000), OrderedAmountPrevious7: ex(40_000)},
				},
				SKUs: []SKUFeatures{
					{SemanticKey: "sku-new-big", CategorySemanticKey: "cat1", SalesAvailable: true, OrderedAmountPrevious7: ex(0), OrderedAmountCurrent7: ex(500_000)},
					{SemanticKey: "sku-new-small", CategorySemanticKey: "cat1", SalesAvailable: true, OrderedAmountPrevious7: ex(0), OrderedAmountCurrent7: ex(500)},
				},
				Capabilities: caps,
			},
			MustIncludeSKU: []string{"sku-new-big"},
			ShouldNotSKU:   []string{"sku-new-small"},
		},
		{
			Name:     "symmetric_increase_decrease",
			Critical: false,
			Core: CoreFacts{
				Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(2_000_000), OrderedAmountPrevious7: ex(2_000_000)},
				Categories: []CategoryFeatures{
					{SemanticKey: "catX", OrderedAmountCurrent7: ex(1_500_000), OrderedAmountPrevious7: ex(1_500_000)},
				},
				SKUs: []SKUFeatures{
					{SemanticKey: "sku-up", CategorySemanticKey: "catX", SalesAvailable: true, OrderedAmountPrevious7: ex(500_000), OrderedAmountCurrent7: ex(1_000_000)},
					{SemanticKey: "sku-down", CategorySemanticKey: "catX", SalesAvailable: true, OrderedAmountPrevious7: ex(1_000_000), OrderedAmountCurrent7: ex(500_000)},
				},
				Capabilities: caps,
			},
			MustIncludeSKU: []string{"sku-up", "sku-down"},
			MustIncludeCat: []string{"catX"},
		},
		{
			Name:     "campaign_to_zero_spend",
			Critical: true,
			Core: CoreFacts{
				Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(100_000), PaidSpendCurrent7: ex(50_000), PaidSpendPrevious7: ex(200_000)},
				Categories: []CategoryFeatures{
					{SemanticKey: "cat1", OrderedAmountCurrent7: ex(100_000), OrderedAmountPrevious7: ex(100_000)},
				},
				SKUs: []SKUFeatures{
					{SemanticKey: "sku1", CategorySemanticKey: "cat1", SalesAvailable: true, OrderedAmountCurrent7: ex(100_000), OrderedAmountPrevious7: ex(100_000)},
				},
				Campaigns: []CampaignFeatures{
					{SemanticKey: "camp-off", TrafficAvailable: true, PaidSpendCurrent7: ex(0), PaidSpendPrevious7: ex(150_000)},
					{SemanticKey: "camp-on", TrafficAvailable: true, PaidSpendCurrent7: ex(40_000), PaidSpendPrevious7: ex(40_000)},
				},
				Capabilities: caps,
			},
			MustIncludeCamp: []string{"camp-off", "camp-on"},
		},
		{
			Name:     "tiny_relative_excluded",
			Critical: true,
			Core: CoreFacts{
				Account: AccountFeatures{SemanticKey: "a", OrderedAmountCurrent7: ex(10_000_000), OrderedAmountPrevious7: ex(10_000_000)},
				Categories: []CategoryFeatures{
					{SemanticKey: "cat1", OrderedAmountCurrent7: ex(9_000_000), OrderedAmountPrevious7: ex(9_000_000)},
				},
				SKUs: []SKUFeatures{
					{SemanticKey: "sku-noise", CategorySemanticKey: "cat1", SalesAvailable: true, OrderedAmountPrevious7: ex(10), OrderedAmountCurrent7: ex(100)},
					{SemanticKey: "sku-real", CategorySemanticKey: "cat1", SalesAvailable: true, OrderedAmountPrevious7: ex(1_000_000), OrderedAmountCurrent7: ex(800_000)},
				},
				Capabilities: caps,
			},
			MustIncludeSKU: []string{"sku-real"},
			ShouldNotSKU:   []string{"sku-noise"},
		},
	}
}
