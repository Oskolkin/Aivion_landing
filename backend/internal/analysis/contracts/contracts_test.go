package contracts

import "testing"

func TestContractHashStableAndFutureMarked(t *testing.T) {
	v := Versions{OrchestrationPolicy: "analysis_orchestration_policy_v1", FactualAPI: "factual_api_v3", UniverseManifest: "factual_universe_manifest_v1", ContextManifest: "resolved_context_manifest_v1"}
	a, err := New(v)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New(v)
	if a.Hash() == "" || a.Hash() != b.Hash() {
		t.Fatal("contract hash is not stable")
	}
	if a.Versions().Publication != NotImplemented {
		t.Fatal("future contract lacks explicit marker")
	}
	cur, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	vv := cur.Versions()
	if vv.LandscapeCore == NotImplemented || vv.Materiality == NotImplemented || vv.LandscapeDetail == NotImplemented || vv.ContinuityContract == NotImplemented {
		t.Fatalf("Prompt 9 seals must be set: %#v", vv)
	}
	if vv.FactFamilyBinding == NotImplemented || vv.WatermarkPolicy == NotImplemented || vv.CapabilityRegistry == NotImplemented {
		t.Fatalf("Prompt 8.2 seals must be set: %#v", vv)
	}
	if !containsHashSeal(vv.FactFamilyBinding, "FACT_FAMILY_BINDING_V1") ||
		!containsHashSeal(vv.WatermarkPolicy, "WATERMARK_REVISION_POLICY_V1") ||
		!containsHashSeal(vv.CapabilityRegistry, "FACT_FAMILY_CAPABILITY_REGISTRY_V1") {
		t.Fatalf("Prompt 8.2 seals must be NAME#hash: binding=%s wm=%s reg=%s", vv.FactFamilyBinding, vv.WatermarkPolicy, vv.CapabilityRegistry)
	}
	if vv.DiscoveryPack == NotImplemented {
		t.Fatal("Discovery pack must be implemented")
	}
}

func containsHashSeal(v, name string) bool {
	return len(v) > len(name)+1 && v[:len(name)] == name && v[len(name)] == '#'
}
