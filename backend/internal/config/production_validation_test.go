package config

import (
	"strings"
	"testing"
)

func TestValidateDeployedEnv_requiresHardenedAuth(t *testing.T) {
	cfg := &Config{
		App: AppConfig{Env: "staging"},
		Auth: AuthConfig{
			JWTSecret:           "abcdefghijklmnopqrstuvwxyz012345",
			EncryptionKey:       "0123456789abcdef0123456789abcdef",
			SessionCookieSecure: true,
			PublicRegistration:  false,
		},
		Admin: AdminConfig{Emails: []string{"admin@example.com"}},
		Server: ServerConfig{
			CORSAllowedOrigins: []string{"https://staging.aivion.pro"},
			HTTP: HTTPConfig{
				MetricsAllowUnauthenticated: false,
				MetricsAuthToken:            "metrics-token",
			},
		},
		OpenAI: OpenAIConfig{ProxyURL: "http://ai-egress:10809"},
	}
	if err := validateDeployedEnv(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDeployedEnv_rejectsPublicRegistration(t *testing.T) {
	cfg := &Config{
		App: AppConfig{Env: "production"},
		Auth: AuthConfig{
			JWTSecret:           "abcdefghijklmnopqrstuvwxyz012345",
			EncryptionKey:       "0123456789abcdef0123456789abcdef",
			SessionCookieSecure: true,
			PublicRegistration:  true,
		},
		Admin: AdminConfig{Emails: []string{"admin@example.com"}},
		Server: ServerConfig{
			CORSAllowedOrigins: []string{"https://app.aivion.pro"},
			HTTP: HTTPConfig{
				MetricsAllowUnauthenticated: false,
				MetricsAuthToken:            "metrics-token",
			},
		},
	}
	err := validateDeployedEnv(cfg)
	if err == nil || !strings.Contains(err.Error(), "AUTH_PUBLIC_REGISTRATION") {
		t.Fatalf("expected public registration error, got %v", err)
	}
}

func TestResolvePublicRegistration_explicitValues(t *testing.T) {
	t.Setenv("AUTH_PUBLIC_REGISTRATION", "false")
	if resolvePublicRegistration("staging") {
		t.Fatal("expected false")
	}
	t.Setenv("AUTH_PUBLIC_REGISTRATION", "true")
	if !resolvePublicRegistration("local") {
		t.Fatal("expected true when explicitly enabled")
	}
}

func TestResolveWorkerConcurrency_clamps(t *testing.T) {
	t.Setenv("WORKER_CONCURRENCY", "1")
	if got := resolveWorkerConcurrency(); got != 1 {
		t.Fatalf("got %d", got)
	}
	t.Setenv("WORKER_CONCURRENCY", "100")
	if got := resolveWorkerConcurrency(); got != 32 {
		t.Fatalf("got %d want 32", got)
	}
}
