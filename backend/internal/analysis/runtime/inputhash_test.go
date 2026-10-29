package runtime

import (
	"strings"
	"testing"
	"time"
)

func TestHashResolveContextDeterministic(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	in := ResolveInput{
		SellerID: 7, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		RunKind: "STANDARD_DAILY", IntentHash: strings.Repeat("a", 64), ContractBundleHash: strings.Repeat("b", 64),
	}
	h1 := HashResolveContext(in)
	h2 := HashResolveContext(in)
	if RequireInputHash(h1) != nil || h1 != h2 {
		t.Fatalf("resolve hash unstable or forbidden: %q", h1)
	}
	in.SellerID = 8
	if HashResolveContext(in) == h1 {
		t.Fatal("seller change must alter resolve input hash")
	}
}

func TestHashSealUniverseDeterministicAndRequiresResolved(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	in := SealInput{
		ResolvedContextHash: strings.Repeat("c", 64),
		AnalysisDate:        cut.AddDate(0, 0, -1), CutoffAt: cut,
		ContractBundleHash: strings.Repeat("d", 64),
	}
	h1 := HashSealUniverse(in)
	h2 := HashSealUniverse(in)
	if RequireInputHash(h1) != nil || h1 != h2 {
		t.Fatalf("seal hash unstable or forbidden: %q", h1)
	}
	if HashSealUniverse(SealInput{ResolvedContextHash: "", AnalysisDate: in.AnalysisDate, CutoffAt: in.CutoffAt, ContractBundleHash: in.ContractBundleHash}) != "" {
		t.Fatal("empty resolved hash must yield empty seal input hash")
	}
	if HashSealUniverse(SealInput{ResolvedContextHash: strings.Repeat("0", 64), AnalysisDate: in.AnalysisDate, CutoffAt: in.CutoffAt, ContractBundleHash: in.ContractBundleHash}) != "" {
		t.Fatal("zero resolved hash must be forbidden")
	}
}

func TestHashPrepareContinuityAndDownstream(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	c1 := HashPrepareContinuity(ContinuityInput{
		SellerID: 7, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		RunKind: "STANDARD_DAILY", IntentHash: strings.Repeat("a", 64), ContractBundleHash: strings.Repeat("b", 64),
	})
	c2 := HashPrepareContinuity(ContinuityInput{
		SellerID: 7, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		RunKind: "STANDARD_DAILY", IntentHash: strings.Repeat("a", 64), ContractBundleHash: strings.Repeat("b", 64),
	})
	if RequireInputHash(c1) != nil || c1 != c2 {
		t.Fatalf("continuity hash unstable: %q", c1)
	}
	base := int64(9)
	if HashPrepareContinuity(ContinuityInput{
		SellerID: 7, AnalysisDate: cut.AddDate(0, 0, -1), CutoffAt: cut,
		RunKind: "STANDARD_DAILY", IntentHash: strings.Repeat("a", 64), ContractBundleHash: strings.Repeat("b", 64),
		BasePublicationID: &base,
	}) == c1 {
		t.Fatal("base_publication_id must alter continuity input hash")
	}
	core := HashLandscapeCore(LandscapeCoreInput{
		UniverseManifestHash: strings.Repeat("c", 64), ContractBundleHash: strings.Repeat("d", 64),
	})
	if RequireInputHash(core) != nil {
		t.Fatalf("landscape core hash invalid: %q", core)
	}
	mat := HashMateriality(MaterialityInput{
		LandscapeCoreHash: core, MaterialityPolicyHash: strings.Repeat("e", 64),
		LaneRegistryHash: strings.Repeat("g", 64), BudgetProfileHash: strings.Repeat("f", 64),
		ContractBundleHash: strings.Repeat("d", 64),
	})
	if RequireInputHash(mat) != nil {
		t.Fatalf("materiality hash invalid: %q", mat)
	}
	det := HashLandscapeDetail(LandscapeDetailInput{
		UniverseManifestHash: strings.Repeat("c", 64), MaterialitySelectionHash: mat,
		ContractBundleHash: strings.Repeat("d", 64),
	})
	if RequireInputHash(det) != nil {
		t.Fatalf("landscape detail hash invalid: %q", det)
	}
}

func TestProductionMaterialityComponentHashesValid(t *testing.T) {
	p := materialityPolicyHash()
	b := materialityBudgetHash()
	l := materialityLaneRegistryHash()
	for name, h := range map[string]string{"policy": p, "budget": b, "lane": l} {
		if err := RequireInputHash(h); err != nil {
			t.Fatalf("%s hash invalid len=%d: %q", name, len(h), h)
		}
	}
	mat := HashMateriality(MaterialityInput{
		LandscapeCoreHash: strings.Repeat("a", 64), MaterialityPolicyHash: p,
		LaneRegistryHash: l, BudgetProfileHash: b, ContractBundleHash: strings.Repeat("b", 64),
	})
	if err := RequireInputHash(mat); err != nil {
		t.Fatalf("materiality input hash invalid len=%d: %q", len(mat), mat)
	}
}

func TestIsForbiddenInputHash(t *testing.T) {
	if !IsForbiddenInputHash("") || !IsForbiddenInputHash(strings.Repeat("0", 64)) || !IsForbiddenInputHash("abc") {
		t.Fatal("expected empty/zero/short hashes forbidden")
	}
	if IsForbiddenInputHash(strings.Repeat("a", 64)) {
		t.Fatal("valid 64-hex must be allowed")
	}
	if RequireInputHash(strings.Repeat("0", 64)) == nil {
		t.Fatal("RequireInputHash must reject zero hash")
	}
}
