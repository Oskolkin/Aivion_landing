package prompt

import (
	"strings"
	"testing"
)

// pinnedAnalystV1_32Hash pins the frozen registered (not current) v1_32
// template hash. Changing analyst_v1_32.txt / analyst_v1_32_input.txt
// requires deliberately recomputing and updating this constant.
const pinnedAnalystV1_32Hash = "cc45e7c57ff80c168db5ff50f49658ea14769ddb8867ecafe118f22b2464dd11"

func TestVersionAnalystV1_32_Registered(t *testing.T) {
	meta, err := RegistryMeta(VersionAnalystV1_32)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Version != VersionAnalystV1_32 {
		t.Fatalf("meta.Version=%q want=%q", meta.Version, VersionAnalystV1_32)
	}
	if meta.Status != PromptStatusPublished {
		t.Fatalf("meta.Status=%q want=%q (published != current)", meta.Status, PromptStatusPublished)
	}
	if !strings.Contains(meta.Notes, "NOT") && !strings.Contains(meta.Notes, "not") {
		t.Fatalf("meta.Notes must document that v1_32 is not production current: %q", meta.Notes)
	}

	tpl, err := TemplateByVersion(VersionAnalystV1_32)
	if err != nil {
		t.Fatalf("TemplateByVersion(v1_32) must resolve since status=published: %v", err)
	}
	if tpl.Version != VersionAnalystV1_32 {
		t.Fatalf("tpl.Version=%q want=%q", tpl.Version, VersionAnalystV1_32)
	}
	if tpl.Instructions == "" || tpl.Input == "" {
		t.Fatal("v1_32 instructions/input must not be empty")
	}
}

func TestVersionAnalystV1_32_HashStable(t *testing.T) {
	h1, err := HashForVersion(VersionAnalystV1_32)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashForVersion(VersionAnalystV1_32)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("hash must be stable across calls: %s vs %s", h1, h2)
	}
	if pinnedAnalystV1_32Hash == "PLACEHOLDER_COMPUTE" {
		t.Fatalf("set pinnedAnalystV1_32Hash to %s", h1)
	}
	if h1 != pinnedAnalystV1_32Hash {
		t.Fatalf("v1_32 hash drift: got=%s want=%s", h1, pinnedAnalystV1_32Hash)
	}
	if h1 != AnalystV1_32Hash() {
		t.Fatalf("AnalystV1_32Hash()=%s must equal HashForVersion result=%s", AnalystV1_32Hash(), h1)
	}
}

func TestVersionAnalystV1_32_CurrentStaysV1_31(t *testing.T) {
	if CurrentAnalystVersion != VersionAnalystV1_31 {
		t.Fatalf("CurrentAnalystVersion=%q want=%q — registering v1_32 must not change production current", CurrentAnalystVersion, VersionAnalystV1_31)
	}
	rendered, err := Render(PromptProfileAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	if rendered.Version != VersionAnalystV1_31 {
		t.Fatalf("Render(PromptProfileAnalysis).Version=%q want=%q", rendered.Version, VersionAnalystV1_31)
	}
}

func TestVersionAnalystV1_32_ProfileForVersion(t *testing.T) {
	if ProfileForVersion(VersionAnalystV1_32) != PromptProfileAnalysis {
		t.Fatalf("ProfileForVersion(v1_32)=%q want=%q", ProfileForVersion(VersionAnalystV1_32), PromptProfileAnalysis)
	}
}

func TestVersionAnalystV1_32_ContentRequirements(t *testing.T) {
	tpl, err := TemplateByVersion(VersionAnalystV1_32)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tpl.Instructions, "aivion_unified_analysis_v11") {
		t.Fatal("v1_32 must mention schema aivion_unified_analysis_v11")
	}
	if !strings.Contains(tpl.Instructions, "analysis_policy_v2") {
		t.Fatal("v1_32 must mention analysis_policy_v2")
	}
	if strings.Contains(tpl.Instructions, "prefer 8–12") || strings.Contains(tpl.Instructions, "8-12 Actions") {
		t.Fatal("v1_32 must not carry the v1_31 8-12 Actions preference forward")
	}
	if !strings.Contains(tpl.Instructions, "{{claim:") || !strings.Contains(tpl.Instructions, "{{threshold:") {
		t.Fatal("v1_32 must define {{claim:...}} and {{threshold:...}} placeholders")
	}
	if !strings.Contains(tpl.Instructions, "observed") || !strings.Contains(tpl.Instructions, "derived") || !strings.Contains(tpl.Instructions, "hypothesis") {
		t.Fatal("v1_32 must document observed/derived/hypothesis interpretation levels")
	}
	if !strings.Contains(tpl.Instructions, "SELF-CHECK") {
		t.Fatal("v1_32 must include a self-check section")
	}
	if len(tpl.Instructions) >= len(mustTemplate(t, VersionAnalystV1_31).Instructions) {
		t.Fatal("v1_32 instructions must be shorter than v1_31 (per spec)")
	}
	if !strings.Contains(tpl.Input, "aivion_unified_analysis_v11") {
		t.Fatal("v1_32 input must mention schema aivion_unified_analysis_v11")
	}
	if !strings.Contains(tpl.Input, "analysis_policy_v2") {
		t.Fatal("v1_32 input must mention analysis_policy_v2")
	}
	if !strings.Contains(tpl.Input, "pack_id") || !strings.Contains(tpl.Input, "pack_hash") || !strings.Contains(tpl.Input, "metric_date") {
		t.Fatal("v1_32 input must state pack_id/pack_hash/metric_date placeholders")
	}
	if !strings.Contains(tpl.Input, "analysis_pack.zip") {
		t.Fatal("v1_32 input must name the pack filename")
	}
	if !strings.Contains(tpl.Input, "Code Interpreter") {
		t.Fatal("v1_32 input must state the Code Interpreter obligation")
	}
}

func mustTemplate(t *testing.T, version string) Template {
	t.Helper()
	tpl, err := TemplateByVersion(version)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}
