package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
)

func TestTokenLimit_CanonicalResolverMatchesDashboardGuardAndNarrativeWiring(t *testing.T) {
	tests := []struct {
		name       string
		perRequest string
		approx     string
	}{
		{name: "primary valid", perRequest: "48000", approx: "99000"},
		{name: "primary invalid fallback valid", perRequest: "bad", approx: "63000"},
		{name: "both absent", perRequest: "", approx: ""},
		{name: "both invalid", perRequest: "bad", approx: "also-bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", tt.perRequest)
			t.Setenv("AI_MAX_INPUT_TOKENS_APPROX", tt.approx)

			want := aitokens.MaxInputTokensPerRequest()
			if got := SynthesisInputTokenLimitForTest(); got != want {
				t.Fatalf("dashboard guard=%d want=%d", got, want)
			}

			setRequiredEnvForConfigLoad(t)
			t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", tt.perRequest)
			t.Setenv("AI_MAX_INPUT_TOKENS_APPROX", tt.approx)
			cfg, err := config.Load()
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

func setRequiredEnvForConfigLoad(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("BACKEND_PORT", "8081")
	t.Setenv("WORKER_METRICS_PORT", "9091")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("MIGRATIONS_PATH", "./migrations")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("S3_ENDPOINT", "localhost:19000")
	t.Setenv("S3_ACCESS_KEY", "minio")
	t.Setenv("S3_SECRET_KEY", "minio123")
	t.Setenv("S3_BUCKET_RAW", "raw-payloads")
	t.Setenv("S3_BUCKET_EXPORTS", "exports")
	t.Setenv("S3_BUCKET_ARTIFACTS", "artifacts")
	t.Setenv("AUTH_COOKIE_NAME", "session_token")
	t.Setenv("AUTH_SESSION_TTL_HOURS", "168")
	t.Setenv("OPENAI_MODEL", "gpt-4.1-mini")
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "30")
	t.Setenv("OPENAI_MAX_RETRIES", "2")
}
