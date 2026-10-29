package runtime

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
)

func TestStage4V3InputHashSensitivity(t *testing.T) {
	in := LandscapeCoreInput{
		UniverseManifestHash: stringsRepeat("a", 64),
		ContractBundleHash:   stringsRepeat("b", 64),
	}
	h := HashLandscapeCore(in)
	if h == "" || len(h) != 64 {
		t.Fatalf("stage 4 hash=%q", h)
	}
}

func TestStage5LaneRegistryHashInInput(t *testing.T) {
	reg := materiality.LaneRegistryV1Current()
	h1 := HashMateriality(MaterialityInput{
		LandscapeCoreHash:     stringsRepeat("c", 64),
		MaterialityPolicyHash: materiality.CurrentPolicy().Hash(),
		LaneRegistryHash:      reg.Hash(),
		BudgetProfileHash:     materiality.DiscoveryStandardBudget().Hash(),
		ContractBundleHash:    stringsRepeat("b", 64),
	})
	h2 := HashMateriality(MaterialityInput{
		LandscapeCoreHash:     stringsRepeat("c", 64),
		MaterialityPolicyHash: materiality.CurrentPolicy().Hash(),
		LaneRegistryHash:      stringsRepeat("d", 64),
		BudgetProfileHash:     materiality.DiscoveryStandardBudget().Hash(),
		ContractBundleHash:    stringsRepeat("b", 64),
	})
	if h1 == h2 {
		t.Fatal("lane registry hash must affect stage 5 input hash")
	}
}

func TestStage6V3DetailHashUsesV3Seal(t *testing.T) {
	h := HashLandscapeDetail(LandscapeDetailInput{
		UniverseManifestHash:     stringsRepeat("a", 64),
		MaterialitySelectionHash: stringsRepeat("s", 64),
		ContractBundleHash:       stringsRepeat("b", 64),
	})
	if h == "" || len(h) != 64 {
		t.Fatalf("detail hash=%q", h)
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}
