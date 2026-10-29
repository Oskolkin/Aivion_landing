package config

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func TestResolveMaxInputTokensPerRequest_MatchesCanonicalResolver(t *testing.T) {
	tests := []struct {
		name       string
		perRequest string
		approx     string
	}{
		{name: "primary valid", perRequest: "50000", approx: "99999"},
		{name: "primary invalid fallback valid", perRequest: "bad", approx: "64000"},
		{name: "both absent", perRequest: "", approx: ""},
		{name: "both invalid", perRequest: "bad", approx: "also-bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", tt.perRequest)
			t.Setenv("AI_MAX_INPUT_TOKENS_APPROX", tt.approx)
			want := aitokens.MaxInputTokensPerRequest()
			if got := resolveMaxInputTokensPerRequest(); got != want {
				t.Fatalf("config resolver=%d canonical=%d", got, want)
			}
		})
	}
}

func TestLoad_MaxInputTokensPerRequest_MatchesCanonicalResolver(t *testing.T) {
	tests := []struct {
		name       string
		perRequest string
		approx     string
	}{
		{name: "primary valid", perRequest: "42000", approx: "99000"},
		{name: "primary invalid fallback valid", perRequest: "oops", approx: "61000"},
		{name: "both absent", perRequest: "", approx: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnvForLoad(t)
			t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", tt.perRequest)
			t.Setenv("AI_MAX_INPUT_TOKENS_APPROX", tt.approx)
			want := aitokens.MaxInputTokensPerRequest()
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AI.MaxInputTokensPerRequest != want {
				t.Fatalf("cfg.AI.MaxInputTokensPerRequest=%d want=%d", cfg.AI.MaxInputTokensPerRequest, want)
			}
			if cfg.AI.OpenAIMaxInputTokensApprox() != want {
				t.Fatalf("OpenAIMaxInputTokensApprox()=%d want=%d", cfg.AI.OpenAIMaxInputTokensApprox(), want)
			}
		})
	}
}
