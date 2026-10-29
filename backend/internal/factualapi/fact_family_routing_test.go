package factualapi

import "testing"

func TestSearchSellerSummaryMapsToSellerSummaryFamily(t *testing.T) {
	got, ok := FactFamilyForCapability("SEARCH.SELLER_SUMMARY")
	if !ok {
		t.Fatal("SEARCH.SELLER_SUMMARY missing from registry")
	}
	if got.Domain != "SEARCH" || got.FactFamily != "SEARCH.SELLER_SUMMARY" {
		t.Fatalf("got %+v, want SEARCH/SEARCH.SELLER_SUMMARY", got)
	}
	market, ok := FactFamilyForCapability("SEARCH.MARKET_SEARCH")
	if !ok || market.FactFamily != "SEARCH.MARKET_SEARCH" {
		t.Fatalf("MARKET_SEARCH routing=%+v ok=%v", market, ok)
	}
	if got.FactFamily == market.FactFamily {
		t.Fatal("seller summary must not map to MARKET_SEARCH")
	}
	for _, key := range []string{"SEARCH.UNIQUE_SEARCH_USERS", "SEARCH.UNIQUE_VIEW_USERS", "SEARCH.PROVIDER_GMV"} {
		ff, ok := FactFamilyForCapability(key)
		if !ok || ff.FactFamily != "SEARCH.SELLER_SUMMARY" {
			t.Fatalf("%s → %+v ok=%v", key, ff, ok)
		}
	}
}

func TestCapabilityFactFamilyIndexIsUnique(t *testing.T) {
	// init() panics on duplicate; this asserts invertibility of the registry.
	seen := map[string]FactFamilyKey{}
	for family, keys := range factFamilyCapabilityRegistry {
		for _, key := range keys {
			if prev, ok := seen[key]; ok {
				t.Fatalf("capability %s maps to %v and %v", key, prev, family)
			}
			seen[key] = family
			got, ok := FactFamilyForCapability(key)
			if !ok || got != family {
				t.Fatalf("inverse lookup %s: got %+v ok=%v want %+v", key, got, ok, family)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("empty registry")
	}
}

func TestAdvertisingTrafficAttributionAreDistinctFamilies(t *testing.T) {
	traffic, ok := FactFamilyForCapability("ADVERTISING.CAMPAIGN_TRAFFIC")
	if !ok || traffic.FactFamily != "ADVERTISING.TRAFFIC" {
		t.Fatalf("traffic=%+v ok=%v", traffic, ok)
	}
	attr, ok := FactFamilyForCapability("ADVERTISING.CAMPAIGN_ATTRIBUTION")
	if !ok || attr.FactFamily != "ADVERTISING.ATTRIBUTION" {
		t.Fatalf("attribution=%+v ok=%v", attr, ok)
	}
	if traffic == attr {
		t.Fatal("traffic and attribution must be distinct fact families")
	}
}
