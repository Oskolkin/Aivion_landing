package config_test

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
)

func TestStagingRequiresAIHTTPSProxy(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("AI_HTTPS_PROXY", "")
	t.Setenv("AI_PROXY_URL", "")
	t.Setenv("DATABASE_URL", "postgres://aivion:aivion@localhost:5432/marketplace_ai?sslmode=disable")
	t.Setenv("MIGRATIONS_PATH", "./migrations")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://staging.aivion.pro")
	t.Setenv("METRICS_ALLOW_UNAUTHENTICATED", "false")
	t.Setenv("METRICS_AUTH_TOKEN", "metrics-token-0123456789abcdef")
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	t.Setenv("AUTH_PUBLIC_REGISTRATION", "false")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "gpt-4.1")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("S3_ACCESS_KEY", "minio")
	t.Setenv("S3_SECRET_KEY", "minio-secret-key-value")
	t.Setenv("S3_BUCKET_RAW", "raw")
	t.Setenv("S3_BUCKET_EXPORTS", "exports")
	t.Setenv("S3_BUCKET_ARTIFACTS", "artifacts")

	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "AI_HTTPS_PROXY") {
		t.Fatalf("expected AI_HTTPS_PROXY required error, got %v", err)
	}
}

func TestStagingDemoModeRequiresSellerIDs(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("AI_HTTPS_PROXY", "http://ai-egress:10809")
	t.Setenv("STAGING_DEMO_MODE", "true")
	t.Setenv("STAGING_DEMO_SELLER_ACCOUNT_IDS", "")
	t.Setenv("DATABASE_URL", "postgres://aivion:aivion@localhost:5432/marketplace_ai?sslmode=disable")
	t.Setenv("MIGRATIONS_PATH", "./migrations")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://staging.aivion.pro")
	t.Setenv("METRICS_ALLOW_UNAUTHENTICATED", "false")
	t.Setenv("METRICS_AUTH_TOKEN", "metrics-token-0123456789abcdef")
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	t.Setenv("AUTH_PUBLIC_REGISTRATION", "false")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "gpt-4.1")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("S3_ACCESS_KEY", "minio")
	t.Setenv("S3_SECRET_KEY", "minio-secret-key-value")
	t.Setenv("S3_BUCKET_RAW", "raw")
	t.Setenv("S3_BUCKET_EXPORTS", "exports")
	t.Setenv("S3_BUCKET_ARTIFACTS", "artifacts")

	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "STAGING_DEMO_SELLER_ACCOUNT_IDS") {
		t.Fatalf("expected seller ids required error, got %v", err)
	}
}

func TestIsStagingDemoSeller(t *testing.T) {
	cfg := &config.Config{}
	cfg.App.Env = "staging"
	cfg.App.StagingDemoMode = true
	cfg.App.StagingDemoSellerIDs = []int64{1, 7}
	if !cfg.IsStagingDemoSeller(1) || cfg.IsStagingDemoSeller(2) {
		t.Fatalf("demo seller matching failed: %+v", cfg.App)
	}
	cfg.App.Env = "production"
	if cfg.IsStagingDemoSeller(1) {
		t.Fatal("production must not treat sellers as demo")
	}
}

func TestPublicDemoModeRequiresStagingDemo(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("AI_HTTPS_PROXY", "http://ai-egress:10809")
	t.Setenv("STAGING_DEMO_MODE", "false")
	t.Setenv("PUBLIC_DEMO_MODE", "true")
	t.Setenv("DATABASE_URL", "postgres://aivion:aivion@localhost:5432/marketplace_ai?sslmode=disable")
	t.Setenv("MIGRATIONS_PATH", "./migrations")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://aivion.pro,https://staging.aivion.pro")
	t.Setenv("METRICS_ALLOW_UNAUTHENTICATED", "false")
	t.Setenv("METRICS_AUTH_TOKEN", "metrics-token-0123456789abcdef")
	t.Setenv("ADMIN_EMAILS", "admin@example.com")
	t.Setenv("AUTH_PUBLIC_REGISTRATION", "false")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "gpt-4.1")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("S3_ACCESS_KEY", "minio")
	t.Setenv("S3_SECRET_KEY", "minio-secret-key-value")
	t.Setenv("S3_BUCKET_RAW", "raw")
	t.Setenv("S3_BUCKET_EXPORTS", "exports")
	t.Setenv("S3_BUCKET_ARTIFACTS", "artifacts")

	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "PUBLIC_DEMO_MODE requires STAGING_DEMO_MODE") {
		t.Fatalf("expected PUBLIC_DEMO_MODE dependency error, got %v", err)
	}
}
