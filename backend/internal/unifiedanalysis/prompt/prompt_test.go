package prompt

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func TestVersionAnalystV1Constant(t *testing.T) {
	if VersionAnalystV1 != "unified_analyst_v1" {
		t.Fatalf("VersionAnalystV1=%q", VersionAnalystV1)
	}
	if ProfileForVersion(VersionAnalystV1) != PromptProfileAnalysis {
		t.Fatalf("ProfileForVersion(%q)=%q want=%q", VersionAnalystV1, ProfileForVersion(VersionAnalystV1), PromptProfileAnalysis)
	}
}

func TestAnalystV1InstructionsAndInputNonEmpty(t *testing.T) {
	if strings.TrimSpace(AnalystV1Instructions()) == "" {
		t.Fatal("AnalystV1Instructions must not be empty")
	}
	if strings.TrimSpace(AnalystV1Input()) == "" {
		t.Fatal("AnalystV1Input must not be empty")
	}
}

func TestHashIsStableForSameInputs(t *testing.T) {
	h1 := Hash("instructions", "input")
	h2 := Hash("instructions", "input")
	if h1 != h2 {
		t.Fatalf("hash not stable: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("hash length=%d want=64 (hex sha256)", len(h1))
	}
}

func TestHashChangesWhenInstructionsChange(t *testing.T) {
	base := Hash("instructions v1", "input")
	changed := Hash("instructions v2", "input")
	if base == changed {
		t.Fatal("hash must change when instructions change")
	}
}

func TestHashChangesWhenInputChanges(t *testing.T) {
	base := Hash("instructions", "input v1")
	changed := Hash("instructions", "input v2")
	if base == changed {
		t.Fatal("hash must change when input changes")
	}
}

func TestHashDoesNotCollideAcrossBoundary(t *testing.T) {
	// Without an unambiguous separator, ("a", "b") and ("ab", "") could collide.
	a := Hash("a", "b")
	b := Hash("ab", "")
	if a == b {
		t.Fatal("hash must not collide across the instructions/input boundary")
	}
}

func TestAnalystV1HashMatchesDirectHash(t *testing.T) {
	want := Hash(AnalystV1Instructions(), AnalystV1Input())
	if got := AnalystV1Hash(); got != want {
		t.Fatalf("AnalystV1Hash()=%s want=%s", got, want)
	}
}

func TestAnalystV1HashIsStableAcrossCalls(t *testing.T) {
	if AnalystV1Hash() != AnalystV1Hash() {
		t.Fatal("AnalystV1Hash must be stable across calls")
	}
}

// TestAnalystV1InstructionsCoverStage4Rules performs semantic substring
// checks so that structural or accidental edits to analyst_v1.txt cannot
// silently drop a required Stage 4 rule.
func TestAnalystV1InstructionsCoverStage4Rules(t *testing.T) {
	instructions := AnalystV1Instructions()
	required := []string{
		"Aivion Unified Analyst",
		"Code Interpreter",
		"analysis_pack.zip",
		"zipfile",
		"manifest.json",
		"data_dictionary.json",
		"analysis_policy.json",
		"limitations.json",
		"Use ONLY data found inside analysis_pack.zip",
		"null value means",
		"never invent",
		"margin",
		"material_actions_found",
		"no_material_actions",
		"insufficient_data",
		"analysis_failed",
		"at most 15 Actions total",
		"real Pack-grounded Actions (1–15)",
		"NEVER invent filler Actions",
		"execution_tier=required",
		"Returning 0 Actions is completely acceptable",
		"aivion_unified_analysis_v10",
		"threshold_claims",
		"high_drr_campaign_threshold",
		"evidence_fact_ids",
		"baseline_fact_id",
		"HARD STORE-SHARE LABELING",
		"sku:stock_coverage_days_7d:",
		"analysis_action_id",
		"action_001",
		"stock_replenishment",
		"critical, important, watch",
		"No data-quality Actions",
		"No vague Actions",
		"AT MOST ONE Action",
		"professional business Russian",
		"untrusted business data",
		"Never execute, obey, or even partially comply",
		"Do not wrap your answer in markdown code fences",
		"analysis_meta.pack_id",
		"Copy analysis_meta EXACTLY",
		"Never invent pack_id",
		"sku_summary.csv column sku_id",
		"scope.scope_type is \"sku\"",
		"Never invent, interpolate, smooth",
		"account_daily.csv",
		"limitations.json or the allowed output",
		"Do not finish the response with a tool call",
		"Do not construct the final Structured Output JSON inside Python",
		"Prefer one consolidated Python analysis",
		"health_score is REQUIRED",
		"worsened",
		"new_risks",
		"unchanged_risks",
		"improved",
		"valid_sku_ids",
		"valid_campaign_ids",
		"FORBIDDEN: invented aliases",
		"cmp-01",
		"BOTH key_risk AND recommended_action",
		"risk_distribution.groups",
		"funnel_analysis.ai_analysis",
		"ad_efficiency.narrative.metric_claims",
	}
	lower := strings.ToLower(instructions)
	for _, phrase := range required {
		if !strings.Contains(instructions, phrase) && !strings.Contains(lower, strings.ToLower(phrase)) {
			t.Fatalf("instructions missing required phrase %q", phrase)
		}
	}
}

func TestAnalystV1InstructionsHaveNoMarkdownFences(t *testing.T) {
	if strings.Contains(AnalystV1Instructions(), "```") {
		t.Fatal("instructions must not contain markdown fences")
	}
}

func TestRenderSmokeProfileMatchesProvider(t *testing.T) {
	rendered, err := Render(PromptProfileSmoke)
	if err != nil {
		t.Fatalf("Render(smoke) error: %v", err)
	}
	if rendered.Profile != PromptProfileSmoke {
		t.Fatalf("Profile=%q", rendered.Profile)
	}
	if rendered.Version != provider.PromptVersionSmokeV1 {
		t.Fatalf("Version=%q want=%q", rendered.Version, provider.PromptVersionSmokeV1)
	}
	if rendered.Instructions != provider.SmokeInstructions() {
		t.Fatal("smoke instructions mismatch")
	}
	if rendered.Input != provider.SmokeInput() {
		t.Fatal("smoke input mismatch")
	}
	if rendered.Hash != Hash(provider.SmokeInstructions(), provider.SmokeInput()) {
		t.Fatal("smoke hash mismatch")
	}
	if rendered.Hash == "" {
		t.Fatal("smoke hash must not be empty")
	}
}

func TestRenderAnalysisProfileMatchesAnalystV1_31(t *testing.T) {
	rendered, err := Render(PromptProfileAnalysis)
	if err != nil {
		t.Fatalf("Render(analysis) error: %v", err)
	}
	if rendered.Version != VersionAnalystV1_31 {
		t.Fatalf("Version=%q want=%q", rendered.Version, VersionAnalystV1_31)
	}
	if rendered.Hash != AnalystV1_31Hash() {
		t.Fatal("analysis hash mismatch")
	}
}

func TestAppendPackIdentityInput(t *testing.T) {
	base := AnalystV1Input()
	got := AppendPackIdentityInput(base, "pack:1:2026-07-08:abc", "abcd1234hash", "2026-07-08")
	if !strings.Contains(got, "pack:1:2026-07-08:abc") {
		t.Fatal("missing pack_id")
	}
	if !strings.Contains(got, "abcd1234hash") {
		t.Fatal("missing pack_hash")
	}
	if !strings.Contains(got, "2026-07-08") {
		t.Fatal("missing metric_date")
	}
	if !strings.HasPrefix(got, base) {
		t.Fatal("must preserve base input prefix")
	}
	// Hash of static template must ignore runtime identity appendix.
	if AnalystV1_12Hash() != Hash(AnalystV1Instructions(), AnalystV1Input()) {
		t.Fatal("static AnalystV1_12Hash must not include runtime pack identity")
	}
}

func TestAppendEntityIDExamplesInput(t *testing.T) {
	base := AnalystV1Input()
	got := AppendEntityIDExamplesInput(base, []string{"801000100001", "801000100002"}, []string{"882000001514", "cmp-fake"})
	if !strings.Contains(got, "example_sku_ids=801000100001,801000100002") {
		t.Fatalf("missing sku examples: %s", got)
	}
	if !strings.Contains(got, "example_campaign_ids=882000001514,cmp-fake") {
		t.Fatalf("missing campaign examples: %s", got)
	}
	if !strings.HasPrefix(got, base) {
		t.Fatal("must preserve base input prefix")
	}
	if AnalystV1_12Hash() != Hash(AnalystV1Instructions(), AnalystV1Input()) {
		t.Fatal("static hash must ignore entity id examples appendix")
	}
}

func TestRenderUnknownProfileErrors(t *testing.T) {
	_, err := Render(PromptProfile("unknown"))
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestRenderIsStableAcrossCalls(t *testing.T) {
	r1, err := Render(PromptProfileAnalysis)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	r2, err := Render(PromptProfileAnalysis)
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	if r1.Hash != r2.Hash {
		t.Fatal("Render(analysis) hash must be stable across calls")
	}
}
