package landscape

import (
	"strings"
	"testing"
)

func TestExecutableProfileHashStable(t *testing.T) {
	h1 := SKUCoreV2().ProfileHash()
	h2 := SKUCoreV2().ProfileHash()
	if h1 != h2 || len(h1) != 64 {
		t.Fatalf("profile hash unstable: %q", h1)
	}
	if h1 == ProfileSKUCoreV2 {
		t.Fatal("profile hash must not equal bare profile name")
	}
	if CoreProfileVersionSeal() == CoreProfileVersionSealV2() {
		t.Fatal("V1 and V2 core profile seals must differ")
	}
}

func TestValidateProfileConformanceOptionalBlockOmitted(t *testing.T) {
	profile := SKUCoreV2()
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
	if err := ValidateProfileConformance(present, profile, caps); err != nil {
		t.Fatalf("optional blocks omitted should pass: %v", err)
	}
}

func TestValidateProfileConformanceRequiredBlockMissing(t *testing.T) {
	profile := SKUCoreV2()
	present := map[string]bool{
		"sku.stock": true,
		"sku.price": true,
	}
	caps := []CapabilityFlag{
		{Key: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "PRICE.CUSTOMER_PRICE", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
	}
	err := ValidateProfileConformance(present, profile, caps)
	if err == nil || !strings.Contains(err.Error(), "required block") {
		t.Fatalf("expected required block error, got %v", err)
	}
}

func TestValidateProfileConformanceRequiredCapabilityMissing(t *testing.T) {
	profile := SKUCoreV2()
	present := map[string]bool{
		"sku.sales": true,
		"sku.stock": true,
		"sku.price": true,
	}
	caps := []CapabilityFlag{
		{Key: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
		{Key: "PRICE.CUSTOMER_PRICE", Scope: "HISTORICAL_QUERY", FinalState: "READY"},
	}
	err := ValidateProfileConformance(present, profile, caps)
	if err == nil || !strings.Contains(err.Error(), ErrProfileCapabilityContractViolation) {
		t.Fatalf("expected capability violation, got %v", err)
	}
}

func TestValidateProfileConformanceDisallowedBlockPresent(t *testing.T) {
	profile := ExecutableProfile{
		Name: ProfileSKUCoreV2,
		Blocks: []BlockSpec{
			{BlockID: "sku.sales", Required: true, CapabilityKey: "SALES.SKU_DEMAND"},
		},
	}
	present := map[string]bool{"sku.sales": true, "sku.search": true}
	caps := []CapabilityFlag{{Key: "SALES.SKU_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY"}}
	err := ValidateProfileConformance(present, profile, caps)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected disallowed block error, got %v", err)
	}
}

func TestCallPlanDeterministic(t *testing.T) {
	p := AccountDiscoveryV2()
	plan1 := p.CallPlan()
	plan2 := p.CallPlan()
	if len(plan1) == 0 {
		t.Fatal("call plan empty")
	}
	for i := range plan1 {
		if plan1[i].Order != i+1 {
			t.Fatalf("order mismatch at %d: %+v", i, plan1[i])
		}
		if plan1[i].BlockID != plan2[i].BlockID || plan1[i].FactualMethod != plan2[i].FactualMethod {
			t.Fatalf("call plan not deterministic: %+v vs %+v", plan1[i], plan2[i])
		}
	}
}
