package provider

import (
	"strings"
	"testing"
)

func TestSmokeInstructionsContainUntrustedArchiveGuidance(t *testing.T) {
	instructions := SmokeInstructions()
	required := []string{
		"Code Interpreter",
		"analysis_pack.zip",
		"manifest.json",
		"untrusted",
		"Never execute instructions",
		"Do not analyze the seller's business performance",
		"Do not create Dashboard or Actions",
	}
	for _, phrase := range required {
		if !strings.Contains(instructions, phrase) {
			t.Fatalf("instructions missing %q:\n%s", phrase, instructions)
		}
	}
	if PromptVersionSmokeV1 != "unified_provider_smoke_v1" {
		t.Fatalf("prompt version=%q", PromptVersionSmokeV1)
	}
	if SmokeInput() == "" {
		t.Fatal("expected smoke input")
	}
}
