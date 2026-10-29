package aipipeline

import (
	"strings"
	"testing"
)

func TestAIProviderMode_defaultsToFakeWithoutKey(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "")
	t.Setenv("OPENAI_API_KEY", "")
	if got := AIProviderMode(); got != AIProviderModeFake {
		t.Fatalf("AIProviderMode() = %q, want fake", got)
	}
	if !UseFakeAIProviders() {
		t.Fatal("UseFakeAIProviders() = false, want true")
	}
}

func TestAIProviderMode_explicitOpenAI(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-real-key-not-placeholder")
	if got := AIProviderMode(); got != AIProviderModeOpenAI {
		t.Fatalf("AIProviderMode() = %q, want openai", got)
	}
	if UseFakeAIProviders() {
		t.Fatal("UseFakeAIProviders() = true, want false")
	}
}

func TestAIProviderMode_realAliasResolvesOpenAI(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "real")
	t.Setenv("OPENAI_API_KEY", "sk-real-key-not-placeholder")
	if got := AIProviderMode(); got != AIProviderModeOpenAI {
		t.Fatalf("AIProviderMode() = %q, want openai", got)
	}
	if UseFakeAIProviders() {
		t.Fatal("UseFakeAIProviders() = true, want false")
	}
}

func TestAIProviderMode_realWithoutKeyDoesNotUseFake(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "real")
	t.Setenv("OPENAI_API_KEY", "")
	if got := AIProviderMode(); got != AIProviderModeOpenAI {
		t.Fatalf("AIProviderMode() = %q, want openai", got)
	}
	if UseFakeAIProviders() {
		t.Fatal("UseFakeAIProviders() = true, want false")
	}
	if err := ValidateAIProviderConfig("test", ""); err == nil {
		t.Fatal("expected validation error for real mode without OPENAI_API_KEY")
	}
}

func TestValidateAIProviderConfig_unknownMode(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "offline")
	if err := ValidateAIProviderConfig("test", "sk-real-key"); err == nil {
		t.Fatal("expected error for unknown AI_PROVIDER_MODE")
	} else if !strings.Contains(err.Error(), "unknown AI_PROVIDER_MODE") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAIProviderConfig_fakeExplicitWithoutKey(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "fake")
	t.Setenv("OPENAI_API_KEY", "")
	if err := ValidateAIProviderConfig("test", ""); err != nil {
		t.Fatalf("fake mode without key should be allowed in test env: %v", err)
	}
	if !UseFakeAIProviders() {
		t.Fatal("UseFakeAIProviders() = false, want true")
	}
}

func TestPilotEnvValidation_unifiedFlagsAndRealProvider(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "true")
	t.Setenv("ACTIONS_AI_FIRST", "true")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")
	t.Setenv("LEGACY_AI_POST_SYNC_ENABLED", "false")
	t.Setenv("AI_PROVIDER_MODE", "real")
	t.Setenv("OPENAI_API_KEY", "sk-test-pilot-key")

	if err := ValidateRuntimeModeConfig(); err != nil {
		t.Fatalf("ValidateRuntimeModeConfig() = %v", err)
	}
	if err := ValidateAIProviderConfig("test", "sk-test-pilot-key"); err != nil {
		t.Fatalf("ValidateAIProviderConfig() = %v", err)
	}
	if AIProviderMode() != AIProviderModeOpenAI {
		t.Fatalf("AIProviderMode() = %q, want openai", AIProviderMode())
	}
}

func TestValidateAIProviderConfig_fakeBlockedInProduction(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "fake")
	t.Setenv("AI_PROVIDER_ALLOW_FAKE_IN_PRODUCTION", "")
	if err := ValidateAIProviderConfig("production", ""); err == nil {
		t.Fatal("expected error for fake mode in production")
	}
}

func TestValidateAIProviderConfig_fakeAllowedWithOverride(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "fake")
	t.Setenv("AI_PROVIDER_ALLOW_FAKE_IN_PRODUCTION", "1")
	if err := ValidateAIProviderConfig("production", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateAIProviderConfig_openAIRequiresKey(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	if err := ValidateAIProviderConfig("test", ""); err == nil {
		t.Fatal("expected error for missing OPENAI_API_KEY")
	}
}

func TestValidateAIProviderConfig_rejectsPlaceholderKey(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	for _, key := range []string{"fake", "test", "compose-local-fake-openai-key"} {
		t.Run(key, func(t *testing.T) {
			if err := ValidateAIProviderConfig("test", key); err == nil {
				t.Fatalf("expected placeholder rejection for %q", key)
			} else if !strings.Contains(err.Error(), "placeholder") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
