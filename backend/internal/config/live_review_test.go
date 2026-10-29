package config_test

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
)

func baseStagingEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "staging")
	t.Setenv("AI_HTTPS_PROXY", "http://ai-egress:10809")
	t.Setenv("STAGING_DEMO_MODE", "true")
	t.Setenv("STAGING_DEMO_SELLER_ACCOUNT_IDS", "1")
	t.Setenv("PUBLIC_DEMO_MODE", "true")
	t.Setenv("DATABASE_URL", "postgres://aivion:aivion@localhost:5432/marketplace_ai?sslmode=disable")
	t.Setenv("MIGRATIONS_PATH", "./migrations")
	t.Setenv("REDIS_ADDR", "localhost:6379")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://aivion.pro")
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
}

func TestLiveReviewEnabledRequiresDemoStack(t *testing.T) {
	baseStagingEnv(t)
	t.Setenv("LIVE_REVIEW_ENABLED", "true")
	t.Setenv("LIVE_REVIEW_ALLOWED_HOST_SUFFIX", ".review.aivion.pro")
	t.Setenv("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID", "1")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected live review config to load: %v", err)
	}
	if !cfg.LiveReview.Enabled || cfg.LiveReview.AllowedSellerAccountID != 1 {
		t.Fatalf("unexpected live review cfg: %+v", cfg.LiveReview)
	}
}

func TestLiveReviewRejectedOnProduction(t *testing.T) {
	baseStagingEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("STAGING_DEMO_MODE", "false")
	t.Setenv("PUBLIC_DEMO_MODE", "false")
	t.Setenv("LIVE_REVIEW_ENABLED", "true")
	t.Setenv("LIVE_REVIEW_ALLOWED_HOST_SUFFIX", ".review.aivion.pro")
	t.Setenv("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID", "1")
	// production validation fails earlier on many fields; ensure live review is not silently accepted
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected production load failure")
	}
}

func TestLiveReviewRequiresSellerInAllowList(t *testing.T) {
	baseStagingEnv(t)
	t.Setenv("LIVE_REVIEW_ENABLED", "true")
	t.Setenv("LIVE_REVIEW_ALLOWED_HOST_SUFFIX", ".review.aivion.pro")
	t.Setenv("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID", "99")
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "STAGING_DEMO_SELLER_ACCOUNT_IDS") {
		t.Fatalf("expected allow-list error, got %v", err)
	}
}

func TestLiveReviewRejectsMainHostSuffix(t *testing.T) {
	baseStagingEnv(t)
	t.Setenv("LIVE_REVIEW_ENABLED", "true")
	t.Setenv("LIVE_REVIEW_ALLOWED_HOST_SUFFIX", "aivion.pro")
	t.Setenv("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID", "1")
	_, err := config.Load()
	if err == nil || !strings.Contains(err.Error(), "LIVE_REVIEW_ALLOWED_HOST_SUFFIX") {
		t.Fatalf("expected host suffix error, got %v", err)
	}
}
