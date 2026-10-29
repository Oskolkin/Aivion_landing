package config

import (
	"reflect"
	"testing"
)

func TestNormalizeAdminEmails(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "empty value",
			raw:  "",
			want: []string{},
		},
		{
			name: "single email",
			raw:  "admin@example.com",
			want: []string{"admin@example.com"},
		},
		{
			name: "multiple emails normalized and trimmed",
			raw:  "admin@example.com, Support@Example.com ",
			want: []string{"admin@example.com", "support@example.com"},
		},
		{
			name: "dedupe and ignore empty entries",
			raw:  "Admin@example.com, ,admin@example.com,support@example.com,",
			want: []string{"admin@example.com", "support@example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeAdminEmails(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeAdminEmails() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLoadAdminEmailsFromEnv(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("ADMIN_EMAILS", "admin@example.com, Support@Example.com ,admin@example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := []string{"admin@example.com", "support@example.com"}
	if !reflect.DeepEqual(cfg.Admin.Emails, want) {
		t.Fatalf("cfg.Admin.Emails = %#v, want %#v", cfg.Admin.Emails, want)
	}
}

func setRequiredEnvForLoad(t *testing.T) {
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
	// Clear process-level proxy env so Load() tests are isolated from local .env.local.
	t.Setenv("AI_PROXY_URL", "")
	t.Setenv("AI_HTTPS_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
}

func TestLoadOpenAITimeoutDefault(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.TimeoutSeconds != 120 {
		t.Fatalf("timeout=%d want 120", cfg.OpenAI.TimeoutSeconds)
	}
}

func TestLoadOpenAITimeoutOverride(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "45")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.TimeoutSeconds != 45 {
		t.Fatalf("timeout=%d want 45", cfg.OpenAI.TimeoutSeconds)
	}
}

func TestLoadOpenAIGlobalContextTimeoutDefault(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.GlobalContextTimeoutSeconds != 300 {
		t.Fatalf("global context timeout=%d want 300", cfg.OpenAI.GlobalContextTimeoutSeconds)
	}
	if cfg.OpenAI.GlobalContextRequestTimeoutSeconds() != 300 {
		t.Fatalf("effective=%d", cfg.OpenAI.GlobalContextRequestTimeoutSeconds())
	}
}

func TestLoadOpenAIGlobalContextTimeoutOverride(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "120")
	t.Setenv("OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS", "300")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.TimeoutSeconds != 120 {
		t.Fatalf("base timeout=%d", cfg.OpenAI.TimeoutSeconds)
	}
	if cfg.OpenAI.GlobalContextRequestTimeoutSeconds() != 300 {
		t.Fatalf("effective=%d", cfg.OpenAI.GlobalContextRequestTimeoutSeconds())
	}
}

func TestLoadOpenAIGlobalContextTimeoutFallbackToBase(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "90")
	t.Setenv("OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS", "0")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.GlobalContextRequestTimeoutSeconds() != 90 {
		t.Fatalf("effective=%d want 90", cfg.OpenAI.GlobalContextRequestTimeoutSeconds())
	}
}

func TestLoadOpenAIActionsTimeoutDefault(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_ACTIONS_TIMEOUT_SECONDS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.ActionsTimeoutSeconds != 900 {
		t.Fatalf("actions timeout=%d want 900", cfg.OpenAI.ActionsTimeoutSeconds)
	}
	if cfg.OpenAI.ActionsRequestTimeoutSeconds() != 900 {
		t.Fatalf("effective=%d want 900", cfg.OpenAI.ActionsRequestTimeoutSeconds())
	}
}

func TestLoadOpenAIActionsTimeoutOverride(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "120")
	t.Setenv("OPENAI_ACTIONS_TIMEOUT_SECONDS", "900")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.ActionsRequestTimeoutSeconds() != 900 {
		t.Fatalf("effective=%d want 900", cfg.OpenAI.ActionsRequestTimeoutSeconds())
	}
}

func TestLoadOpenAIActionsTimeoutZeroUsesDefault(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "120")
	t.Setenv("OPENAI_ACTIONS_TIMEOUT_SECONDS", "0")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.ActionsRequestTimeoutSeconds() != 900 {
		t.Fatalf("effective=%d want 900 (not base 120)", cfg.OpenAI.ActionsRequestTimeoutSeconds())
	}
}

func TestLoadOpenAIActionsTimeoutNegativeFails(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("OPENAI_ACTIONS_TIMEOUT_SECONDS", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadAIAndCleanupDefaults(t *testing.T) {
	setRequiredEnvForLoad(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI.RecommendationMaxContextItems <= 0 || cfg.AI.ChatMaxContextBytes <= 0 {
		t.Fatalf("unexpected AI defaults: %+v", cfg.AI)
	}
	if cfg.Cleanup.RetentionDays <= 0 {
		t.Fatalf("cleanup retention: %d", cfg.Cleanup.RetentionDays)
	}
}

func TestLoadAIProxyURLWinsOverHTTPSProxy(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("AI_PROXY_URL", "http://127.0.0.1:10809")
	t.Setenv("HTTPS_PROXY", "http://should-not-win:9999")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.AIHTTPSProxyURL() != "http://127.0.0.1:10809" {
		t.Fatalf("proxy=%q", cfg.OpenAI.ProxyURL)
	}
}

func TestLoadAIHTTPSProxyEmpty(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("AI_PROXY_URL", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("AI_HTTPS_PROXY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.AIHTTPSProxyURL() != "" {
		t.Fatalf("proxy=%q", cfg.OpenAI.ProxyURL)
	}
}

func TestLoadAIHTTPSProxySet(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("AI_HTTPS_PROXY", "http://127.0.0.1:10809")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAI.AIHTTPSProxyURL() != "http://127.0.0.1:10809" {
		t.Fatalf("proxy=%q", cfg.OpenAI.ProxyURL)
	}
}

func TestLoadAIHTTPSProxyInvalid(t *testing.T) {
	setRequiredEnvForLoad(t)
	t.Setenv("AI_HTTPS_PROXY", "socks5://127.0.0.1:10808")

	if _, err := Load(); err == nil {
		t.Fatal("expected load error for socks5 proxy")
	}
}
