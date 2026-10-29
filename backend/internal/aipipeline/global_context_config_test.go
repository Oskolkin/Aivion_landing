package aipipeline_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestGlobalContextRequestedMaxOutputTokens_Default16384(t *testing.T) {
	t.Setenv("OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS", "")
	t.Setenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", "8192")
	if got := aipipeline.GlobalContextRequestedMaxOutputTokensForTest(); got != 16384 {
		t.Fatalf("got=%d want=16384", got)
	}
}

func TestGlobalContextRequestedMaxOutputTokens_EnvOverride(t *testing.T) {
	t.Setenv("OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS", "20480")
	if got := aipipeline.GlobalContextRequestedMaxOutputTokensForTest(); got != 20480 {
		t.Fatalf("got=%d want=20480", got)
	}
}

func TestClassifyGlobalContextFailureTruncatedConfirmed(t *testing.T) {
	raw := &aipipeline.GlobalContextRawResponse{
		ProviderStatus:           "incomplete",
		IncompleteReason:         "max_output_tokens",
		OutputTokens:             16384,
		RequestedMaxOutputTokens: 16384,
	}
	got := aipipeline.ClassifyGlobalContextFailureForTest(raw, nil, "")
	if got != aipipeline.GlobalContextFailureTruncatedConfirmed {
		t.Fatalf("got=%s", got)
	}
}
