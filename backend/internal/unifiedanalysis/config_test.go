package unifiedanalysis

import (
	"testing"
)

func TestConfigValidateDisabledSkipsAPIKey(t *testing.T) {
	cfg := Config{Enabled: false}
	if err := cfg.Validate(""); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestConfigValidateEnabledRequiresKeyAndModel(t *testing.T) {
	cfg := Config{
		Enabled:                 true,
		Provider:                ProviderNameOpenAI,
		Model:                   "gpt-test",
		MemoryLimit:             "4g",
		AnalysisMaxOutputTokens: 1024,
		SmokeMaxOutputTokens:    1024,
		MaxOutputTokens:         1024,
		ProviderMaxAttempts:     2,
		PollIntervalSeconds:     30,
		MaxPollDurationSeconds:  3600,
		HTTPTimeoutSeconds:      120,
	}
	if err := cfg.Validate(""); err == nil {
		t.Fatal("expected api key error")
	}
	if err := cfg.Validate("sk-test"); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestConfigValidateMemoryAllowlist(t *testing.T) {
	cfg := Config{
		Enabled:                 true,
		Provider:                ProviderNameOpenAI,
		Model:                   "gpt-test",
		MemoryLimit:             "8g",
		AnalysisMaxOutputTokens: 1024,
		SmokeMaxOutputTokens:    1024,
		MaxOutputTokens:         1024,
		ProviderMaxAttempts:     2,
		PollIntervalSeconds:     30,
		MaxPollDurationSeconds:  3600,
		HTTPTimeoutSeconds:      120,
	}
	if err := cfg.Validate("sk-test"); err == nil {
		t.Fatal("expected memory limit validation error")
	}
}

func TestConfigFromEnvDefaultsDisabled(t *testing.T) {
	t.Setenv("UNIFIED_ANALYSIS_PROVIDER_ENABLED", "")
	t.Setenv("UNIFIED_ANALYSIS_ENABLED", "")
	cfg := ConfigFromEnv()
	if cfg.Enabled {
		t.Fatal("expected disabled by default")
	}
}

func TestConfigFromEnvEnabled(t *testing.T) {
	t.Setenv("UNIFIED_ANALYSIS_PROVIDER_ENABLED", "true")
	t.Setenv("UNIFIED_ANALYSIS_ENABLED", "")
	t.Setenv("UNIFIED_ANALYSIS_MODEL", "gpt-test")
	t.Setenv("UNIFIED_ANALYSIS_CODE_INTERPRETER_MEMORY", "16g")
	t.Setenv("UNIFIED_ANALYSIS_MEMORY_LIMIT", "")
	cfg := ConfigFromEnv()
	if !cfg.Enabled {
		t.Fatal("expected enabled")
	}
	if cfg.Model != "gpt-test" {
		t.Fatalf("model=%q", cfg.Model)
	}
	if cfg.MemoryLimit != "16g" {
		t.Fatalf("memory=%q", cfg.MemoryLimit)
	}
}
