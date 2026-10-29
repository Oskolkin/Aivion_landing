package recommendations

import (
	"strings"
	"testing"
)

func TestNormalizeRecommendationType_Aliases(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantChg bool
	}{
		{"stock_replenishment", "replenish_sku", true},
		{"replenish_stock", "replenish_sku", true},
		{"ad_spend_review", "review_ad_spend", true},
		{"reduce_ad_spend_for_low_stock", "avoid_ads_for_low_stock_sku", true},
		{"price_review", "review_price_margin", true},
		{"overstock_discount", "discount_overstock", true},
		{"replenish_sku", "replenish_sku", false},
		{"  Stock-Replenishment  ", "replenish_sku", true},
	}
	for _, tc := range cases {
		got, changed, ok := NormalizeRecommendationType(tc.in)
		if !ok {
			t.Fatalf("%q: expected ok", tc.in)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.in, got, tc.want)
		}
		if changed != tc.wantChg {
			t.Fatalf("%q: changed=%v want %v", tc.in, changed, tc.wantChg)
		}
	}
}

func TestNormalizeRecommendationType_Unknown(t *testing.T) {
	_, _, ok := NormalizeRecommendationType("unknown_type")
	if ok {
		t.Fatal("expected ok=false for unknown_type")
	}
}

func TestNormalizeRecommendationType_LegacyValidatorTypes(t *testing.T) {
	legacy := map[string]string{
		"review_price_below_min":                "review_price_floor",
		"review_margin_risk":                    "review_price_margin",
		"redirect_ad_budget_from_low_stock_sku": "avoid_ads_for_low_stock_sku",
		"review_campaign_without_result":        "review_ad_spend",
	}
	for in, want := range legacy {
		got, _, ok := NormalizeRecommendationType(in)
		if !ok || got != want {
			t.Fatalf("%q => %q ok=%v, want %q", in, got, ok, want)
		}
	}
}

func TestAllowedRecommendationTypesPromptLines_NoPriceEconomics(t *testing.T) {
	for _, line := range AllowedRecommendationTypesPromptLines() {
		if strings.Contains(line, "review_price") || strings.Contains(line, "margin") {
			t.Fatalf("active prompt must not include price type: %s", line)
		}
	}
}

func TestCanonicalRecommendationTypes_AllNormalizeToSelf(t *testing.T) {
	for _, canon := range ActiveCanonicalRecommendationTypes {
		got, changed, ok := NormalizeRecommendationType(canon)
		if !ok || got != canon || changed {
			t.Fatalf("canonical %q => %q changed=%v ok=%v", canon, got, changed, ok)
		}
	}
}
