package materiality

import "testing"

func drrCampaign() CampaignFeatures {
	return CampaignFeatures{
		SemanticKey: "camp-drr", CanonicalRefID: "9",
		TrafficAvailable: true, AttributionAvailable: true, DRRAvailable: true,
		PaidSpendCurrent7: ex(20_000), PaidSpendPrevious7: ex(18_000),
		DRRCurrentPP: stock("20"), DRRPreviousPP: stock("10"),
	}
}

func rankingHasLane(r Ranking, lane string) bool {
	for _, e := range r.Campaigns {
		for _, m := range e.LaneMemberships {
			if m.LaneKey == lane {
				return true
			}
		}
	}
	return false
}

func TestCapabilitiesAllowLaneDRRAllCapabilities(t *testing.T) {
	core := CoreFacts{
		Account:      AccountFeatures{SemanticKey: "a", PaidSpendCurrent7: ex(100_000)},
		Campaigns:    []CampaignFeatures{drrCampaign()},
		Capabilities: baseCapabilities(),
	}
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	if !rankingHasLane(r, "DRR_ABSOLUTE_PP_CHANGE") {
		t.Fatal("DRR lane must be active when Traffic and Attribution are READY")
	}
}

func TestCapabilitiesAllowLaneDRRAttributionUnavailable(t *testing.T) {
	caps := baseCapabilities()
	caps["ADVERTISING.CAMPAIGN_ATTRIBUTION"] = "UNAVAILABLE"
	core := CoreFacts{
		Account:      AccountFeatures{SemanticKey: "a", PaidSpendCurrent7: ex(100_000)},
		Campaigns:    []CampaignFeatures{drrCampaign()},
		Capabilities: caps,
	}
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	if rankingHasLane(r, "DRR_ABSOLUTE_PP_CHANGE") {
		t.Fatal("DRR lane must be inactive when Attribution is UNAVAILABLE")
	}
}

func TestCapabilitiesAllowLaneDRRTrafficUnavailable(t *testing.T) {
	caps := baseCapabilities()
	caps["ADVERTISING.CAMPAIGN_TRAFFIC"] = "UNAVAILABLE"
	core := CoreFacts{
		Account:      AccountFeatures{SemanticKey: "a", PaidSpendCurrent7: ex(100_000)},
		Campaigns:    []CampaignFeatures{drrCampaign()},
		Capabilities: caps,
	}
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	if rankingHasLane(r, "DRR_ABSOLUTE_PP_CHANGE") {
		t.Fatal("DRR lane must be inactive when Traffic is UNAVAILABLE")
	}
}

func TestCapabilitiesAllowLaneDRRPeriodsIncompatible(t *testing.T) {
	camp := drrCampaign()
	camp.DRRAvailable = false
	camp.DRRCurrentPP = MoneyFeature{State: FeatureUnknown}
	camp.DRRPreviousPP = MoneyFeature{State: FeatureUnknown}
	core := CoreFacts{
		Account:      AccountFeatures{SemanticKey: "a", PaidSpendCurrent7: ex(100_000)},
		Campaigns:    []CampaignFeatures{camp},
		Capabilities: baseCapabilities(),
	}
	r := BuildMaterialityRanking(core, CurrentPolicy(), "")
	if rankingHasLane(r, "DRR_ABSOLUTE_PP_CHANGE") {
		t.Fatal("DRR lane must be inactive when periods are incompatible")
	}
}
