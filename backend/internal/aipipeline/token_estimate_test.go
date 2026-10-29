package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func TestMaxAIInputTokensPerRequest_MatchesCanonicalResolver(t *testing.T) {
	tests := []struct {
		name       string
		perRequest string
		approx     string
	}{
		{name: "primary valid", perRequest: "47000", approx: "98000"},
		{name: "primary invalid fallback valid", perRequest: "bad", approx: "62000"},
		{name: "both absent", perRequest: "", approx: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", tt.perRequest)
			t.Setenv("AI_MAX_INPUT_TOKENS_APPROX", tt.approx)
			want := aitokens.MaxInputTokensPerRequest()
			if got := MaxAIInputTokensPerRequest(); got != want {
				t.Fatalf("pipeline guard=%d want=%d", got, want)
			}
		})
	}
}
