package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
)

type Config struct {
	App             AppConfig
	Server          ServerConfig
	DB              DBConfig
	Redis           RedisConfig
	S3              S3Config
	Auth            AuthConfig
	Admin           AdminConfig
	LiveReview      LiveReviewConfig
	Sentry          SentryConfig
	Telemetry       TelemetryConfig
	OpenAI          OpenAIConfig
	UnifiedAnalysis UnifiedAnalysisConfig
	AI              AIContextLimitsConfig
	Cleanup         CleanupConfig
	Ozon            OzonConfig
	Sync            SyncConfig
}

// LiveReviewConfig enables temporary header-based live seller review for external auditors.
// Raw invite tokens are never stored in runtime.env; only Caddy holds them.
type LiveReviewConfig struct {
	Enabled                bool
	AllowedHostSuffix      string
	AllowedSellerAccountID int64
}

// OzonConfig controls Ozon integration behavior.
type OzonConfig struct {
	PerformanceAuthBaseURL            string
	DisconnectRetentionDays           int
	DisconnectedAccountCleanupEnabled bool
}

// SyncConfig controls background Ozon sync scheduling.
type SyncConfig struct {
	Interval          time.Duration
	SchedulerEnabled  bool
	WorkerConcurrency int
}

// DisconnectedAccountCleanupConfig controls retention purge for disconnected cabinets.
type DisconnectedAccountCleanupConfig struct {
	Enabled       bool
	RetentionDays int
}

type AppConfig struct {
	Env string
	// StagingDemoMode enables staging-only demo data access (no live Ozon).
	// Default false. Forbidden in production.
	StagingDemoMode bool
	// StagingDemoSellerIDs are seller_account IDs treated as source_mode=demo when StagingDemoMode is on.
	StagingDemoSellerIDs []int64
	// PublicDemoMode marks the shared staging stack as the public demo entry
	// (aivion.pro without Basic Auth). Forbidden in production. Requires
	// StagingDemoMode. Does not create a separate database or publication.
	PublicDemoMode bool
	// PublicDemoUserEmail is the canonical non-admin demo login (informational).
	PublicDemoUserEmail string
}

type ServerConfig struct {
	Port               string
	WorkerMetricsPort  string
	CORSAllowedOrigins []string
	HTTP               HTTPConfig
	AuthRateLimit      AuthRateLimitConfig
}

type HTTPConfig struct {
	ReadHeaderTimeout           time.Duration
	ReadTimeout                 time.Duration
	WriteTimeout                time.Duration
	IdleTimeout                 time.Duration
	ShutdownTimeout             time.Duration
	MaxHeaderBytes              int
	JSONBodyLimitBytes          int64
	ChatBodyLimitBytes          int64
	MetricsAuthToken            string
	MetricsAllowUnauthenticated bool
}

type AuthRateLimitConfig struct {
	LoginIPLimit    int
	LoginEmailLimit int
	RegisterIPLimit int
	Window          time.Duration
	RedisFailOpen   bool
}

type DBConfig struct {
	URL            string
	MigrationsPath string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type S3Config struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	BucketRaw       string
	BucketExports   string
	BucketArtifacts string
	UseSSL          bool
}

type AuthConfig struct {
	JWTSecret           string
	EncryptionKey       string
	CookieName          string
	SessionTTLHours     int
	SessionCookieSecure bool
	PublicRegistration  bool
}

type AdminConfig struct {
	Emails []string
}

type SentryConfig struct {
	DSN     string
	Release string
}

type TelemetryConfig struct {
	Enabled      bool
	OTLPEndpoint string
	ServiceName  string
}

// UnifiedAnalysisConfig controls Stage 3 unified analysis provider orchestration.
type UnifiedAnalysisConfig = unifiedanalysis.Config

type OpenAIConfig struct {
	APIKey                           string
	Model                            string
	TimeoutSeconds                   int
	GlobalContextTimeoutSeconds      int
	GlobalContextMaxOutputTokens     int
	ActionsTimeoutSeconds            int
	ActionObjectDetailTimeoutSeconds int
	MaxRetries                       int
	ProxyURL                         string
}

// AIHTTPSProxyURL returns the effective OpenAI proxy URL resolved from env (empty = direct).
func (c OpenAIConfig) AIHTTPSProxyURL() string {
	return strings.TrimSpace(c.ProxyURL)
}

// GlobalContextRequestedMaxOutputTokens returns max_output_tokens for Global Context synthesis.
func (c OpenAIConfig) GlobalContextRequestedMaxOutputTokens() int {
	if c.GlobalContextMaxOutputTokens > 0 {
		return c.GlobalContextMaxOutputTokens
	}
	return 16384
}

// GlobalContextRequestTimeoutSeconds returns the effective Global Context HTTP request timeout.
// Priority: OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS > OPENAI_TIMEOUT_SECONDS > default 120.
func (c OpenAIConfig) GlobalContextRequestTimeoutSeconds() int {
	if c.GlobalContextTimeoutSeconds > 0 {
		return c.GlobalContextTimeoutSeconds
	}
	if c.TimeoutSeconds > 0 {
		return c.TimeoutSeconds
	}
	return openaix.DefaultRequestTimeoutSeconds
}

// ActionsRequestTimeoutSeconds returns the effective Actions HTTP request timeout.
// Priority: OPENAI_ACTIONS_TIMEOUT_SECONDS if > 0, else DefaultActionsTimeoutSeconds (900).
// Unlike Global Context, Actions never falls back to the base OPENAI_TIMEOUT_SECONDS.
func (c OpenAIConfig) ActionsRequestTimeoutSeconds() int {
	if c.ActionsTimeoutSeconds > 0 {
		return c.ActionsTimeoutSeconds
	}
	return openaix.DefaultActionsTimeoutSeconds
}

// ActionObjectDetailRequestTimeoutSeconds returns per Object Detail batch HTTP timeout.
func (c OpenAIConfig) ActionObjectDetailRequestTimeoutSeconds() int {
	if c.ActionObjectDetailTimeoutSeconds > 0 {
		return c.ActionObjectDetailTimeoutSeconds
	}
	return openaix.DefaultActionsTimeoutSeconds
}

// AIContextLimitsConfig caps AI payloads before OpenAI (MVP byte/token heuristics).
type AIContextLimitsConfig struct {
	RecommendationMaxContextItems int
	RecommendationMaxContextBytes int
	ChatMaxContextItems           int
	ChatMaxContextBytes           int
	MaxInputTokensApprox          int
	MaxInputTokensPerRequest      int
	MaxOutputTokens               int
	MaxOutputTokensPerRequest     int
	SummaryMaxCategoriesPerRun    int
	ChatMaxQuestionsPerDay        int
	RecommendationMaxRunsPerDay   int
	SummaryMaxRunsPerDay          int
}

// OpenAIMaxInputTokensApprox returns the per-request input token budget for OpenAI clients.
func (c AIContextLimitsConfig) OpenAIMaxInputTokensApprox() int {
	if c.MaxInputTokensPerRequest > 0 {
		return c.MaxInputTokensPerRequest
	}
	if c.MaxInputTokensApprox > 0 {
		return c.MaxInputTokensApprox
	}
	return aitokens.DefaultMaxInputTokensPerRequest
}

// OpenAIMaxOutputTokens returns max_output_tokens for OpenAI requests.
func (c AIContextLimitsConfig) OpenAIMaxOutputTokens() int {
	if c.MaxOutputTokensPerRequest > 0 {
		return c.MaxOutputTokensPerRequest
	}
	if c.MaxOutputTokens > 0 {
		return c.MaxOutputTokens
	}
	return 4000
}

// CleanupConfig controls optional background maintenance (MVP: archive stale chat sessions only).
type CleanupConfig struct {
	Enabled       bool
	RetentionDays int
	Schedule      time.Duration
}

func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", "")

	openAIKey, err := LoadSecretFromFileOrEnv("OPENAI_API_KEY_FILE", "OPENAI_API_KEY")
	if err != nil {
		return nil, err
	}
	databaseURL, err := LoadSecretFromFileOrEnv("DATABASE_URL_FILE", "DATABASE_URL")
	if err != nil {
		return nil, err
	}
	jwtSecret, err := LoadSecretFromFileOrEnv("JWT_SECRET_FILE", "JWT_SECRET")
	if err != nil {
		return nil, err
	}
	encryptionKey, err := LoadSecretFromFileOrEnv("ENCRYPTION_KEY_FILE", "ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}
	metricsToken, err := LoadSecretFromFileOrEnv("METRICS_AUTH_TOKEN_FILE", "METRICS_AUTH_TOKEN")
	if err != nil {
		return nil, err
	}
	redisPassword, err := LoadSecretFromFileOrEnv("REDIS_PASSWORD_FILE", "REDIS_PASSWORD")
	if err != nil {
		return nil, err
	}
	s3AccessKey, err := LoadSecretFromFileOrEnv("S3_ACCESS_KEY_FILE", "S3_ACCESS_KEY")
	if err != nil {
		return nil, err
	}
	s3SecretKey, err := LoadSecretFromFileOrEnv("S3_SECRET_KEY_FILE", "S3_SECRET_KEY")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		App: AppConfig{
			Env:                  appEnv,
			StagingDemoMode:      getEnvAsBool("STAGING_DEMO_MODE", false),
			StagingDemoSellerIDs: parseInt64CSV(getEnv("STAGING_DEMO_SELLER_ACCOUNT_IDS", "")),
			PublicDemoMode:       getEnvAsBool("PUBLIC_DEMO_MODE", false),
			PublicDemoUserEmail:  strings.TrimSpace(getEnv("PUBLIC_DEMO_USER_EMAIL", "")),
		},
		LiveReview: LiveReviewConfig{
			Enabled:                getEnvAsBool("LIVE_REVIEW_ENABLED", false),
			AllowedHostSuffix:      strings.TrimSpace(getEnv("LIVE_REVIEW_ALLOWED_HOST_SUFFIX", ".review.aivion.pro")),
			AllowedSellerAccountID: getEnvAsInt64("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID", 1),
		},
		Server: ServerConfig{
			Port:               getEnv("BACKEND_PORT", "8080"),
			WorkerMetricsPort:  getEnv("WORKER_METRICS_PORT", "9091"),
			CORSAllowedOrigins: splitCommaList(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
			HTTP: HTTPConfig{
				ReadHeaderTimeout:           getEnvAsDuration("HTTP_READ_HEADER_TIMEOUT", 10*time.Second),
				ReadTimeout:                 getEnvAsDuration("HTTP_READ_TIMEOUT", 30*time.Second),
				WriteTimeout:                getEnvAsDuration("HTTP_WRITE_TIMEOUT", 180*time.Second),
				IdleTimeout:                 getEnvAsDuration("HTTP_IDLE_TIMEOUT", 120*time.Second),
				ShutdownTimeout:             getEnvAsDuration("HTTP_SHUTDOWN_TIMEOUT", 30*time.Second),
				MaxHeaderBytes:              getEnvAsInt("HTTP_MAX_HEADER_BYTES", 1<<20),
				JSONBodyLimitBytes:          int64(getEnvAsInt("HTTP_JSON_BODY_LIMIT_BYTES", 1<<20)),
				ChatBodyLimitBytes:          int64(getEnvAsInt("HTTP_CHAT_BODY_LIMIT_BYTES", 2<<20)),
				MetricsAuthToken:            metricsToken,
				MetricsAllowUnauthenticated: getEnvAsBool("METRICS_ALLOW_UNAUTHENTICATED", appEnvAllowsOpenMetrics(appEnv)),
			},
			AuthRateLimit: AuthRateLimitConfig{
				LoginIPLimit:    getEnvAsInt("AUTH_LOGIN_IP_LIMIT", 20),
				LoginEmailLimit: getEnvAsInt("AUTH_LOGIN_EMAIL_LIMIT", 10),
				RegisterIPLimit: getEnvAsInt("AUTH_REGISTER_IP_LIMIT", 10),
				Window:          getEnvAsDuration("AUTH_RATE_LIMIT_WINDOW", 15*time.Minute),
				RedisFailOpen:   getEnvAsBool("AUTH_RATE_LIMIT_REDIS_FAIL_OPEN", true),
			},
		},
		DB: DBConfig{
			URL:            databaseURL,
			MigrationsPath: getEnv("MIGRATIONS_PATH", "./migrations"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", ""),
			Password: redisPassword,
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		S3: S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", ""),
			AccessKey:       s3AccessKey,
			SecretKey:       s3SecretKey,
			BucketRaw:       getEnv("S3_BUCKET_RAW", ""),
			BucketExports:   getEnv("S3_BUCKET_EXPORTS", ""),
			BucketArtifacts: getEnv("S3_BUCKET_ARTIFACTS", ""),
			UseSSL:          getEnvAsBool("S3_USE_SSL", false),
		},
		Auth: AuthConfig{
			JWTSecret:           jwtSecret,
			EncryptionKey:       encryptionKey,
			CookieName:          getEnv("AUTH_COOKIE_NAME", "session_token"),
			SessionTTLHours:     getEnvAsInt("AUTH_SESSION_TTL_HOURS", 168),
			SessionCookieSecure: getEnvAsBool("SESSION_COOKIE_SECURE", false),
			PublicRegistration:  resolvePublicRegistration(appEnv),
		},
		Admin: AdminConfig{
			Emails: NormalizeAdminEmails(getEnv("ADMIN_EMAILS", "")),
		},
		Sentry: SentryConfig{
			DSN:     getEnv("SENTRY_DSN", ""),
			Release: getEnv("SENTRY_RELEASE", "dev"),
		},
		Telemetry: TelemetryConfig{
			Enabled:      getEnvAsBool("OTEL_ENABLED", false),
			OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			ServiceName:  getEnv("OTEL_SERVICE_NAME", "marketplace-ai-backend"),
		},
		OpenAI: OpenAIConfig{
			APIKey:                           openAIKey,
			Model:                            getEnv("OPENAI_MODEL", "gpt-4.1-mini"),
			TimeoutSeconds:                   getEnvAsInt("OPENAI_TIMEOUT_SECONDS", openaix.DefaultRequestTimeoutSeconds),
			GlobalContextTimeoutSeconds:      getEnvAsInt("OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS", openaix.DefaultGlobalContextTimeoutSeconds),
			GlobalContextMaxOutputTokens:     getEnvAsInt("OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS", 16384),
			ActionsTimeoutSeconds:            getEnvAsInt("OPENAI_ACTIONS_TIMEOUT_SECONDS", openaix.DefaultActionsTimeoutSeconds),
			ActionObjectDetailTimeoutSeconds: getEnvAsInt("OPENAI_ACTION_OBJECT_DETAIL_TIMEOUT_SECONDS", openaix.DefaultActionsTimeoutSeconds),
			MaxRetries:                       getEnvAsInt("OPENAI_MAX_RETRIES", 2),
			ProxyURL:                         resolveOpenAIProxyURLFromEnv(),
		},
		UnifiedAnalysis: unifiedanalysis.ConfigFromEnv(),
		AI: AIContextLimitsConfig{
			RecommendationMaxContextItems: getEnvAsInt("AI_RECOMMENDATION_MAX_CONTEXT_ITEMS", 50),
			RecommendationMaxContextBytes: getEnvAsInt("AI_RECOMMENDATION_MAX_CONTEXT_BYTES", 480*1024),
			ChatMaxContextItems:           getEnvAsInt("AI_CHAT_MAX_CONTEXT_ITEMS", 20),
			ChatMaxContextBytes:           getEnvAsInt("AI_CHAT_MAX_CONTEXT_BYTES", 50*1024),
			MaxInputTokensApprox:          getEnvAsInt("AI_MAX_INPUT_TOKENS_APPROX", aitokens.DefaultMaxInputTokensPerRequest),
			MaxInputTokensPerRequest:      resolveMaxInputTokensPerRequest(),
			MaxOutputTokens:               getEnvAsInt("AI_MAX_OUTPUT_TOKENS", 4096),
			MaxOutputTokensPerRequest:     resolveMaxOutputTokensPerRequest(),
			SummaryMaxCategoriesPerRun:    resolveSummaryMaxCategoriesPerRun(getEnvAsInt("AI_SUMMARY_MAX_CATEGORIES_PER_RUN", 10)),
			ChatMaxQuestionsPerDay:        getEnvAsInt("AI_CHAT_MAX_QUESTIONS_PER_DAY", 50),
			RecommendationMaxRunsPerDay:   getEnvAsInt("AI_RECOMMENDATION_MAX_RUNS_PER_DAY", 3),
			SummaryMaxRunsPerDay:          getEnvAsInt("AI_SUMMARY_MAX_RUNS_PER_DAY", 3),
		},
		Cleanup: CleanupConfig{
			Enabled:       getEnvAsBool("CLEANUP_ENABLED", false),
			RetentionDays: getEnvAsInt("CLEANUP_RETENTION_DAYS", 90),
			Schedule:      getEnvAsDuration("CLEANUP_SCHEDULE", 168*time.Hour),
		},
		Ozon: OzonConfig{
			PerformanceAuthBaseURL:            getEnv("OZON_PERFORMANCE_AUTH_URL", "https://api-performance.ozon.ru"),
			DisconnectRetentionDays:           getEnvAsInt("DISCONNECTED_ACCOUNT_RETENTION_DAYS", 90),
			DisconnectedAccountCleanupEnabled: getEnvAsBool("DISCONNECTED_ACCOUNT_CLEANUP_ENABLED", false),
		},
		Sync: SyncConfig{
			Interval:          getEnvAsDuration("SYNC_INTERVAL", 5*time.Minute),
			SchedulerEnabled:  getEnvAsBool("WORKER_SCHEDULER_ENABLED", true),
			WorkerConcurrency: resolveWorkerConcurrency(),
		},
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validate(cfg *Config) error {

	if cfg.Auth.CookieName == "" {
		return fmt.Errorf("AUTH_COOKIE_NAME is required")
	}
	if cfg.Auth.SessionTTLHours <= 0 {
		return fmt.Errorf("AUTH_SESSION_TTL_HOURS must be greater than 0")
	}
	if cfg.Server.WorkerMetricsPort == "" {
		return fmt.Errorf("WORKER_METRICS_PORT is required")
	}
	if cfg.App.Env == "" {
		return fmt.Errorf("APP_ENV is required")
	}
	if cfg.Server.Port == "" {
		return fmt.Errorf("BACKEND_PORT is required")
	}
	if cfg.DB.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.DB.MigrationsPath == "" {
		return fmt.Errorf("MIGRATIONS_PATH is required")
	}
	if cfg.Redis.Addr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	if cfg.S3.Endpoint == "" {
		return fmt.Errorf("S3_ENDPOINT is required")
	}
	if cfg.S3.AccessKey == "" {
		return fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if cfg.S3.SecretKey == "" {
		return fmt.Errorf("S3_SECRET_KEY is required")
	}
	if cfg.S3.BucketRaw == "" {
		return fmt.Errorf("S3_BUCKET_RAW is required")
	}
	if cfg.S3.BucketExports == "" {
		return fmt.Errorf("S3_BUCKET_EXPORTS is required")
	}
	if cfg.S3.BucketArtifacts == "" {
		return fmt.Errorf("S3_BUCKET_ARTIFACTS is required")
	}
	if cfg.OpenAI.Model == "" {
		return fmt.Errorf("OPENAI_MODEL is required")
	}
	if cfg.OpenAI.TimeoutSeconds <= 0 {
		return fmt.Errorf("OPENAI_TIMEOUT_SECONDS must be greater than 0")
	}
	if cfg.OpenAI.GlobalContextTimeoutSeconds < 0 {
		return fmt.Errorf("OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS must be greater than or equal to 0")
	}
	if cfg.OpenAI.ActionsTimeoutSeconds < 0 {
		return fmt.Errorf("OPENAI_ACTIONS_TIMEOUT_SECONDS must be greater than or equal to 0")
	}
	if cfg.OpenAI.MaxRetries < 0 {
		return fmt.Errorf("OPENAI_MAX_RETRIES must be greater than or equal to 0")
	}
	if _, err := openaix.ParseProxyURL(cfg.OpenAI.ProxyURL); err != nil {
		return fmt.Errorf("%w (%s)", err, openaix.RedactProxyURL(cfg.OpenAI.ProxyURL))
	}
	if cfg.Cleanup.Enabled && cfg.Cleanup.RetentionDays <= 0 {
		return fmt.Errorf("CLEANUP_RETENTION_DAYS must be greater than 0 when CLEANUP_ENABLED is true")
	}
	if cfg.Cleanup.Enabled && cfg.Cleanup.Schedule <= 0 {
		return fmt.Errorf("CLEANUP_SCHEDULE must be a positive duration when CLEANUP_ENABLED is true")
	}

	switch cfg.App.Env {
	case "local", "test", "staging", "production":
	default:
		return fmt.Errorf("APP_ENV must be one of: local, test, staging, production")
	}

	if cfg.Sync.WorkerConcurrency < 1 {
		return fmt.Errorf("WORKER_CONCURRENCY must be greater than 0")
	}

	if isDeployedEnv(cfg.App.Env) {
		if err := validateDeployedEnv(cfg); err != nil {
			return err
		}
	}

	return nil
}

func isDeployedEnv(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "staging", "production":
		return true
	default:
		return false
	}
}

func validateDeployedEnv(cfg *Config) error {
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return fmt.Errorf("JWT_SECRET is required in %s", cfg.App.Env)
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters in %s", cfg.App.Env)
	}
	if len(cfg.Auth.EncryptionKey) != 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes in %s", cfg.App.Env)
	}
	if !cfg.Auth.SessionCookieSecure {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true in %s", cfg.App.Env)
	}
	if len(cfg.Server.CORSAllowedOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS is required in %s", cfg.App.Env)
	}
	for _, origin := range cfg.Server.CORSAllowedOrigins {
		if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
			return fmt.Errorf("CORS_ALLOWED_ORIGINS must not include localhost in %s", cfg.App.Env)
		}
	}
	if cfg.Server.HTTP.MetricsAllowUnauthenticated {
		return fmt.Errorf("METRICS_ALLOW_UNAUTHENTICATED must be false in %s", cfg.App.Env)
	}
	if strings.TrimSpace(cfg.Server.HTTP.MetricsAuthToken) == "" {
		return fmt.Errorf("METRICS_AUTH_TOKEN is required in %s", cfg.App.Env)
	}
	if len(cfg.Admin.Emails) == 0 {
		return fmt.Errorf("ADMIN_EMAILS is required in %s", cfg.App.Env)
	}
	if cfg.Auth.PublicRegistration {
		return fmt.Errorf("AUTH_PUBLIC_REGISTRATION must be false in %s until self-service onboarding is approved", cfg.App.Env)
	}
	if cfg.App.Env == "staging" {
		if strings.TrimSpace(cfg.OpenAI.ProxyURL) == "" {
			return fmt.Errorf("AI_HTTPS_PROXY is required in staging (OpenAI must egress via ai-egress; no direct fallback)")
		}
	}
	if cfg.App.StagingDemoMode {
		if cfg.App.Env != "staging" {
			return fmt.Errorf("STAGING_DEMO_MODE is only allowed when APP_ENV=staging")
		}
		if len(cfg.App.StagingDemoSellerIDs) == 0 {
			return fmt.Errorf("STAGING_DEMO_SELLER_ACCOUNT_IDS is required when STAGING_DEMO_MODE=true")
		}
	}
	if cfg.App.PublicDemoMode {
		if cfg.App.Env != "staging" {
			return fmt.Errorf("PUBLIC_DEMO_MODE is only allowed when APP_ENV=staging (shared public-demo stack)")
		}
		if !cfg.App.StagingDemoMode {
			return fmt.Errorf("PUBLIC_DEMO_MODE requires STAGING_DEMO_MODE=true on the shared stack")
		}
	}
	if err := validateLiveReviewConfig(cfg); err != nil {
		return err
	}
	return nil
}

func validateLiveReviewConfig(cfg *Config) error {
	if cfg == nil || !cfg.LiveReview.Enabled {
		return nil
	}
	if isProductionLikeEnv(cfg.App.Env) && !cfg.App.PublicDemoMode {
		return fmt.Errorf("LIVE_REVIEW_ENABLED is not allowed in production client environments")
	}
	if cfg.App.Env != "staging" && cfg.App.Env != "local" && cfg.App.Env != "test" && cfg.App.Env != "development" {
		return fmt.Errorf("LIVE_REVIEW_ENABLED requires APP_ENV=staging (or local/test for development)")
	}
	if !cfg.App.StagingDemoMode {
		return fmt.Errorf("LIVE_REVIEW_ENABLED requires STAGING_DEMO_MODE=true")
	}
	if !cfg.App.PublicDemoMode && cfg.App.Env == "staging" {
		return fmt.Errorf("LIVE_REVIEW_ENABLED requires PUBLIC_DEMO_MODE=true on the shared demo stack")
	}
	if cfg.LiveReview.AllowedSellerAccountID <= 0 {
		return fmt.Errorf("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID must be > 0 when LIVE_REVIEW_ENABLED=true")
	}
	if !cfg.IsStagingDemoSeller(cfg.LiveReview.AllowedSellerAccountID) {
		return fmt.Errorf("LIVE_REVIEW_ALLOWED_SELLER_ACCOUNT_ID must be present in STAGING_DEMO_SELLER_ACCOUNT_IDS")
	}
	suffix := strings.TrimSpace(cfg.LiveReview.AllowedHostSuffix)
	if suffix == "" {
		return fmt.Errorf("LIVE_REVIEW_ALLOWED_HOST_SUFFIX is required when LIVE_REVIEW_ENABLED=true")
	}
	normalized := strings.ToLower(strings.TrimPrefix(suffix, "."))
	if normalized == "aivion.pro" || normalized == "www.aivion.pro" || normalized == "staging.aivion.pro" {
		return fmt.Errorf("LIVE_REVIEW_ALLOWED_HOST_SUFFIX must not equal the main application host")
	}
	if !strings.Contains(normalized, "review.") {
		return fmt.Errorf("LIVE_REVIEW_ALLOWED_HOST_SUFFIX must be a dedicated review host suffix")
	}
	return nil
}

func isProductionLikeEnv(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "production", "prod":
		return true
	default:
		return false
	}
}

// IsStagingDemoSeller reports whether sellerID may use staging demo source_mode.
func (c *Config) IsStagingDemoSeller(sellerID int64) bool {
	if c == nil || !c.App.StagingDemoMode || c.App.Env != "staging" || sellerID <= 0 {
		return false
	}
	for _, id := range c.App.StagingDemoSellerIDs {
		if id == sellerID {
			return true
		}
	}
	return false
}

func parseInt64CSV(raw string) []int64 {
	parts := splitCommaList(raw)
	if len(parts) == 0 {
		return nil
	}
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || n <= 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func resolvePublicRegistration(appEnv string) bool {
	if _, ok := os.LookupEnv("AUTH_PUBLIC_REGISTRATION"); ok {
		return getEnvAsBool("AUTH_PUBLIC_REGISTRATION", false)
	}
	// Default: open in local/test, closed in staging/production.
	return !isDeployedEnv(appEnv)
}

func resolveWorkerConcurrency() int {
	v := getEnvAsInt("WORKER_CONCURRENCY", 10)
	if v < 1 {
		return 1
	}
	if v > 32 {
		return 32
	}
	return v
}

func appEnvAllowsOpenMetrics(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "local", "test", "development":
		return true
	default:
		return false
	}
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func resolveOpenAIProxyURLFromEnv() string {
	proxyURL, _ := openaix.ResolveAIProxyURL()
	return proxyURL
}

func getEnvAsInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvAsDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func resolveSummaryMaxCategoriesPerRun(v int) int {
	if v <= 0 {
		return 10
	}
	return v
}

func resolveMaxInputTokensPerRequest() int {
	return aitokens.MaxInputTokensPerRequest()
}

func resolveMaxOutputTokensPerRequest() int {
	if v := getEnvAsInt("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", 0); v > 0 {
		return v
	}
	if v := getEnvAsInt("AI_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	return 4000
}

func splitCommaList(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func NormalizeAdminEmails(raw string) []string {
	if raw == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	emails := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))

	for _, part := range parts {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		emails = append(emails, email)
	}

	return emails
}
