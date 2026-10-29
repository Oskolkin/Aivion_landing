package config

import "testing"

func TestLoad_pilotEnvParsesCorrectly(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_API_KEY", "sk-test-pilot-key")
	t.Setenv("OPENAI_MAX_RETRIES", "0")
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", "120000")
	t.Setenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", "8192")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OpenAI.APIKey != "sk-test-pilot-key" {
		t.Fatalf("OpenAI.APIKey = %q", cfg.OpenAI.APIKey)
	}
	if cfg.OpenAI.MaxRetries != 0 {
		t.Fatalf("OpenAI.MaxRetries = %d, want 0", cfg.OpenAI.MaxRetries)
	}
	if cfg.AI.MaxInputTokensPerRequest != 120000 {
		t.Fatalf("MaxInputTokensPerRequest = %d, want 120000", cfg.AI.MaxInputTokensPerRequest)
	}
	if cfg.AI.OpenAIMaxOutputTokens() != 8192 {
		t.Fatalf("OpenAIMaxOutputTokens() = %d, want 8192", cfg.AI.OpenAIMaxOutputTokens())
	}
}

func TestLoad_openAIMaxRetriesZeroFromEnv(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_MAX_RETRIES", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OpenAI.MaxRetries != 0 {
		t.Fatalf("OpenAI.MaxRetries = %d, want 0", cfg.OpenAI.MaxRetries)
	}
}
