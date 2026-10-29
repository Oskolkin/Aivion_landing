package config

import "testing"

func TestAIContextLimitsConfig_perRequestEnvOverridesLegacy(t *testing.T) {
	c := AIContextLimitsConfig{
		MaxInputTokensPerRequest:  8000,
		MaxOutputTokensPerRequest: 1500,
		MaxInputTokensApprox:      50000,
		MaxOutputTokens:           4000,
	}
	if c.OpenAIMaxInputTokensApprox() != 8000 || c.OpenAIMaxOutputTokens() != 1500 {
		t.Fatalf("per-request caps not preferred: in=%d out=%d",
			c.OpenAIMaxInputTokensApprox(), c.OpenAIMaxOutputTokens())
	}
}
