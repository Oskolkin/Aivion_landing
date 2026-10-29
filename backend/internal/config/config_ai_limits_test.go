package config

import "testing"

func TestAIContextLimitsConfig_OpenAITokenHelpers(t *testing.T) {
	c := AIContextLimitsConfig{
		MaxInputTokensPerRequest:  9000,
		MaxOutputTokensPerRequest: 2000,
		MaxInputTokensApprox:      120000,
		MaxOutputTokens:           4096,
	}
	if c.OpenAIMaxInputTokensApprox() != 9000 {
		t.Fatalf("input=%d", c.OpenAIMaxInputTokensApprox())
	}
	if c.OpenAIMaxOutputTokens() != 2000 {
		t.Fatalf("output=%d", c.OpenAIMaxOutputTokens())
	}
}
