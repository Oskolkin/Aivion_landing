package landscape

import "testing"

func TestSKUEligible_ORRules(t *testing.T) {
	if SKUEligible(SKUEligibilityInput{}) {
		t.Fatal("empty input must be ineligible")
	}
	if !SKUEligible(SKUEligibilityInput{CatalogState: "ACTIVE"}) {
		t.Fatal("active catalog must be eligible")
	}
	u := int64(3)
	if !SKUEligible(SKUEligibilityInput{SalesUnitsCur: &u}) {
		t.Fatal("current sales units must be eligible")
	}
	z := int64(0)
	five := int64(5)
	if !SKUEligible(SKUEligibilityInput{SellableUnits: &five}) {
		t.Fatal("sellable>0 must be eligible")
	}
	if SKUEligible(SKUEligibilityInput{SellableUnits: &z}) {
		t.Fatal("sellable=0 alone must be ineligible")
	}
	if !SKUEligible(SKUEligibilityInput{OpenInboundUnits: &five}) {
		t.Fatal("open inbound must be eligible")
	}
	if !SKUEligible(SKUEligibilityInput{AdsSpendCurKnown: true}) {
		t.Fatal("ads activity must be eligible")
	}
	if !SKUEligible(SKUEligibilityInput{HasPromoMembership: true}) {
		t.Fatal("promo membership must be eligible")
	}
	if !SKUEligible(SKUEligibilityInput{CatalogState: "ARCHIVED", SalesAmountPrevKnown: true}) {
		t.Fatal("archived with previous sales must remain eligible")
	}
}

func TestCampaignEligible_ORRules(t *testing.T) {
	if CampaignEligible(CampaignEligibilityInput{}) {
		t.Fatal("empty input must be ineligible")
	}
	if !CampaignEligible(CampaignEligibilityInput{ProviderState: "CAMPAIGN_STATE_RUNNING"}) {
		t.Fatal("running must be eligible")
	}
	if !CampaignEligible(CampaignEligibilityInput{ProviderState: "planned"}) {
		t.Fatal("planned must be eligible")
	}
	if !CampaignEligible(CampaignEligibilityInput{SpendCurKnown: true, SpendCurPositive: true}) {
		t.Fatal("positive spend must be eligible")
	}
	if CampaignEligible(CampaignEligibilityInput{SpendCurKnown: true, SpendCurPositive: false}) {
		t.Fatal("known zero spend alone must be ineligible")
	}
	if !CampaignEligible(CampaignEligibilityInput{AttrPrevKnown: true, AttrPrevPositive: true}) {
		t.Fatal("previous attribution must be eligible")
	}
}

func TestEligibilityPolicyVersion(t *testing.T) {
	if LandscapeEligibilityV1 != "LANDSCAPE_ELIGIBILITY_V1" {
		t.Fatalf("seal=%s", LandscapeEligibilityV1)
	}
}
