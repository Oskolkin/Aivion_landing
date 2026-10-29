package landscape

import (
	"testing"
)

func TestProfileHashV1V2V3Distinct(t *testing.T) {
	h2 := AccountDiscoveryV2().ProfileHash()
	h3 := AccountDiscoveryV3().ProfileHash()
	if h2 == h3 {
		t.Fatalf("profile hashes must differ across V2/V3: v2=%s v3=%s", h2, h3)
	}
	if CoreProfileVersionSeal() == CoreProfileVersionSealV2() {
		t.Fatal("core seal v3 must differ from v2")
	}
	if DetailProfileVersionSeal() == DetailProfileVersionSealV2() {
		t.Fatal("detail seal v3 must differ from v2")
	}
}

func TestV2ArtifactFailsV3Validation(t *testing.T) {
	set := LandscapeSet{
		Context: LandscapeContext{
			LandscapeSchemaVersion:  LandscapeSchemaV2,
			LandscapeProfileVersion: CoreProfileVersionSealV2(),
		},
	}
	if err := ValidateCoreProfileConformanceV3(set, DefaultCoreProfilesV3()); err == nil {
		t.Fatal("expected V2 artifact to fail V3 conformance")
	}
}

func TestV3ReplayDeterministic(t *testing.T) {
	p := SKUCoreV3()
	h1 := p.ProfileHash()
	h2 := p.ProfileHash()
	if h1 != h2 {
		t.Fatalf("profile hash not stable: %s vs %s", h1, h2)
	}
}
