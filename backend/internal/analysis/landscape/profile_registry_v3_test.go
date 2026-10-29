package landscape

import (
	"testing"
)

func TestExecutableProfileV3HashStable(t *testing.T) {
	h1 := SKUCoreV3().ProfileHash()
	h2 := SKUCoreV3().ProfileHash()
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("profile hash unstable: %q", h1)
	}
	if h1 == ProfileSKUCoreV3 {
		t.Fatal("profile hash must not equal bare profile name")
	}
	if CoreProfileVersionSealV3() == CoreProfileVersionSealV2() {
		t.Fatal("V2 and V3 core profile seals must differ")
	}
	if DetailProfileVersionSealV3() == DetailProfileVersionSealV2() {
		t.Fatal("V2 and V3 detail profile seals must differ")
	}
}

func TestSKUDetailV3SupportingSpecs(t *testing.T) {
	p := SKUDetailV3()

	campaign, ok := p.Supporting("sku.detail.campaign_context")
	if !ok {
		t.Fatal("expected campaign supporting spec")
	}
	if campaign.Limit != 5 || campaign.SortBasis != "PAID_SPEND_DESC" || campaign.SemanticScope != "TOP_N_BY_PAID_SPEND" {
		t.Fatalf("unexpected campaign supporting spec: %+v", campaign)
	}

	promotion, ok := p.Supporting("sku.detail.promotion_context")
	if !ok {
		t.Fatal("expected promotion supporting spec")
	}
	if promotion.Limit != 5 || promotion.SortBasis != "PROMOTION_ID_ASC" || promotion.SemanticScope != "TOP_N_ACTIVE_OR_CANDIDATE" {
		t.Fatalf("unexpected promotion supporting spec: %+v", promotion)
	}

	query, ok := p.Supporting("sku.detail.query_context")
	if !ok {
		t.Fatal("expected query supporting spec")
	}
	if query.Limit != 5 || query.SortBasis != "PROVIDER_SEARCH_RANK" || query.SemanticScope != "TOP_N_PROVIDER_RANKED" {
		t.Fatalf("unexpected query supporting spec: %+v", query)
	}
}

func TestCampaignDetailV3SKUContextSupporting(t *testing.T) {
	spec, ok := CampaignDetailV3().Supporting("campaign.detail.sku_context")
	if !ok {
		t.Fatal("expected campaign detail sku supporting spec")
	}
	if spec.Limit != 10 || spec.SortBasis != "PAID_SPEND_DESC" || spec.SemanticScope != "TOP_N_OBSERVED_PERFORMANCE" {
		t.Fatalf("unexpected sku context supporting spec: %+v", spec)
	}
}

func TestProfileHashV3SensitivityLimit(t *testing.T) {
	base := SKUDetailV3().ProfileHash()
	mutated := cloneProfileV3(SKUDetailV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "sku.detail.campaign_context" && mutated.Blocks[i].Supporting != nil {
			mutated.Blocks[i].Supporting.Limit = 6
			break
		}
	}
	assertProfileHashChanged(t, "supporting limit", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivitySort(t *testing.T) {
	base := SKUDetailV3().ProfileHash()
	mutated := cloneProfileV3(SKUDetailV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "sku.detail.query_context" && mutated.Blocks[i].Supporting != nil {
			mutated.Blocks[i].Supporting.SortBasis = "QUERY_VOLUME_DESC"
			break
		}
	}
	assertProfileHashChanged(t, "supporting sort", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivitySemanticScope(t *testing.T) {
	base := CampaignDetailV3().ProfileHash()
	mutated := cloneProfileV3(CampaignDetailV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "campaign.detail.sku_context" && mutated.Blocks[i].Supporting != nil {
			mutated.Blocks[i].Supporting.SemanticScope = "TOP_N_BY_ASSIGNMENT"
			break
		}
	}
	assertProfileHashChanged(t, "supporting semantic scope", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivityCapability(t *testing.T) {
	base := SKUCoreV3().ProfileHash()
	mutated := cloneProfileV3(SKUCoreV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "sku.advertising" {
			mutated.Blocks[i].CapabilityKeys = []string{"ADVERTISING.CAMPAIGN_TRAFFIC"}
			break
		}
	}
	assertProfileHashChanged(t, "capability key", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivityFactFamily(t *testing.T) {
	base := CategoryCoreV3().ProfileHash()
	mutated := cloneProfileV3(CategoryCoreV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "category.sales" {
			mutated.Blocks[i].FactFamilies = []string{"SALES.DEMAND.ALT"}
			break
		}
	}
	assertProfileHashChanged(t, "fact family", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivityFactualMethod(t *testing.T) {
	base := AccountDiscoveryV3().ProfileHash()
	mutated := cloneProfileV3(AccountDiscoveryV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "account.finance" {
			mutated.Blocks[i].FactualMethods = []string{"GetFinanceAccountSummaryOnly"}
			break
		}
	}
	assertProfileHashChanged(t, "factual method", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivityPeriodAnchor(t *testing.T) {
	base := SKUCoreV3().ProfileHash()
	mutated := cloneProfileV3(SKUCoreV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "sku.sales" && len(mutated.Blocks[i].Periods) > 0 {
			mutated.Blocks[i].Periods[0].Anchor = PeriodAnchorSnapshot
			break
		}
	}
	assertProfileHashChanged(t, "period anchor", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivitySupportingPeriod(t *testing.T) {
	base := SKUDetailV3().ProfileHash()
	mutated := cloneProfileV3(SKUDetailV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "sku.detail.query_context" && mutated.Blocks[i].Supporting != nil {
			mutated.Blocks[i].Supporting.Period = sourceNativePeriod("PREVIOUS_7D", "SEARCH.QUERY_DETAILS")
			break
		}
	}
	assertProfileHashChanged(t, "supporting period", base, mutated.ProfileHash())
}

func TestProfileHashV3SensitivitySerialization(t *testing.T) {
	base := CategoryDetailV3().ProfileHash()
	mutated := cloneProfileV3(CategoryDetailV3())
	for i := range mutated.Blocks {
		if mutated.Blocks[i].BlockID == "category.search" {
			mutated.Blocks[i].Serialization = SerializeBlockState
			break
		}
	}
	assertProfileHashChanged(t, "serialization", base, mutated.ProfileHash())
}

func TestDefaultCoreProfilesV3Names(t *testing.T) {
	b := DefaultCoreProfilesV3()
	if b.Account.Name != ProfileAccountDiscoveryV3 ||
		b.Category.Name != ProfileCategoryCoreV3 ||
		b.SKU.Name != ProfileSKUCoreV3 ||
		b.Campaign.Name != ProfileCampaignCoreV3 {
		t.Fatalf("unexpected core bundle names: %+v", b)
	}
}

func TestDefaultDetailProfilesV3Names(t *testing.T) {
	b := DefaultDetailProfilesV3()
	if b.Account.Name != ProfileAccountDiscoveryV3 ||
		b.Category.Name != ProfileCategoryDetailV3 ||
		b.SKU.Name != ProfileSKUDetailV3 ||
		b.Campaign.Name != ProfileCampaignDetailV3 {
		t.Fatalf("unexpected detail bundle names: %+v", b)
	}
}

func cloneProfileV3(p ExecutableProfileV3) ExecutableProfileV3 {
	out := p
	out.Blocks = make([]BlockSpecV3, len(p.Blocks))
	for i, b := range p.Blocks {
		out.Blocks[i] = cloneBlockSpecV3(b)
	}
	return out
}

func cloneBlockSpecV3(b BlockSpecV3) BlockSpecV3 {
	out := b
	if len(b.CapabilityKeys) > 0 {
		out.CapabilityKeys = append([]string(nil), b.CapabilityKeys...)
	}
	if len(b.FactFamilies) > 0 {
		out.FactFamilies = append([]string(nil), b.FactFamilies...)
	}
	if len(b.FactualMethods) > 0 {
		out.FactualMethods = append([]string(nil), b.FactualMethods...)
	}
	if len(b.Periods) > 0 {
		out.Periods = append([]PeriodSpec(nil), b.Periods...)
	}
	if b.Supporting != nil {
		s := *b.Supporting
		out.Supporting = &s
	}
	return out
}

func assertProfileHashChanged(t *testing.T, field string, baseHash, mutatedHash string) {
	t.Helper()
	if baseHash == "" || mutatedHash == "" {
		t.Fatalf("%s: empty hash base=%q mutated=%q", field, baseHash, mutatedHash)
	}
	if baseHash == mutatedHash {
		t.Fatalf("%s change must change profile hash", field)
	}
}
