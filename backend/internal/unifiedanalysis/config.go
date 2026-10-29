package unifiedanalysis

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var allowedMemoryLimits = map[string]struct{}{
	"1g":  {},
	"4g":  {},
	"16g": {},
	"64g": {},
}

const (
	defaultAnalysisMaxOutputTokens = 32768
	defaultSmokeMaxOutputTokens    = 2000
	defaultProviderMaxAttempts     = 2
	maxProviderMaxAttempts         = 5
)

// knownModelMaxOutputTokens maps confirmed model IDs to their documented max
// output token caps. Unknown models are not hard-capped here.
var knownModelMaxOutputTokens = map[string]int{
	"gpt-4.1":      32768,
	"gpt-4.1-mini": 32768,
	"gpt-4.1-nano": 32768,
}

// Config controls Unified Analysis orchestration (Stage 3 smoke + Stage 4 analysis).
type Config struct {
	Enabled                 bool
	Provider                string
	Model                   string
	MemoryLimit             string
	AnalysisMaxOutputTokens int
	SmokeMaxOutputTokens    int
	// MaxOutputTokens is the deprecated effective fallback retained for older
	// call sites. Prefer EffectiveMaxOutputTokens / profile-specific fields.
	MaxOutputTokens        int
	ProviderMaxAttempts    int
	PollIntervalSeconds    int
	MaxPollDurationSeconds int
	HTTPTimeoutSeconds     int
	TransportRetries       int
	CleanupEnabled         bool
	DebugArtifacts         bool

	// PromptProfile is "smoke" or "analysis". Empty defaults to smoke for backward compatibility.
	PromptProfile string
	// PromptVersion overrides the rendered prompt version when non-empty.
	PromptVersion string
	// OutputValidationEnabled controls Stage 4 technical validation after provider completion.
	OutputValidationEnabled bool
}

// ConfigFromEnv loads Unified Analysis settings from UNIFIED_ANALYSIS_* env vars.
//
// Max output token priority per profile:
//
//	profile-specific env → UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS (legacy) → profile default
func ConfigFromEnv() Config {
	enabled := envBool("UNIFIED_ANALYSIS_PROVIDER_ENABLED", false)
	if !enabled {
		// Backward-compatible alias used in early drafts.
		enabled = envBool("UNIFIED_ANALYSIS_ENABLED", false)
	}
	legacyMax := envIntOptional("UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS")
	analysisMax := envIntWithFallback("UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS", legacyMax, defaultAnalysisMaxOutputTokens)
	smokeMax := envIntWithFallback("UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS", legacyMax, defaultSmokeMaxOutputTokens)
	profile := envString("UNIFIED_ANALYSIS_PROMPT_PROFILE", "smoke")
	effective := smokeMax
	if isAnalysisProfileName(profile) {
		effective = analysisMax
	}
	return Config{
		Enabled:                 enabled,
		Provider:                envString("UNIFIED_ANALYSIS_PROVIDER", ProviderNameOpenAI),
		Model:                   firstNonEmptyEnv("UNIFIED_ANALYSIS_MODEL", "OPENAI_MODEL"),
		MemoryLimit:             envString("UNIFIED_ANALYSIS_CODE_INTERPRETER_MEMORY", envString("UNIFIED_ANALYSIS_MEMORY_LIMIT", "4g")),
		AnalysisMaxOutputTokens: analysisMax,
		SmokeMaxOutputTokens:    smokeMax,
		MaxOutputTokens:         effective,
		ProviderMaxAttempts:     envInt("UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS", defaultProviderMaxAttempts),
		PollIntervalSeconds:     envInt("UNIFIED_ANALYSIS_POLL_INTERVAL_SECONDS", 5),
		MaxPollDurationSeconds:  envInt("UNIFIED_ANALYSIS_MAX_POLL_DURATION_SECONDS", 1800),
		HTTPTimeoutSeconds:      envInt("UNIFIED_ANALYSIS_HTTP_TIMEOUT_SECONDS", 60),
		TransportRetries:        envInt("UNIFIED_ANALYSIS_TRANSPORT_RETRIES", 1),
		CleanupEnabled:          envBool("UNIFIED_ANALYSIS_CLEANUP_ENABLED", true),
		DebugArtifacts:          envBool("UNIFIED_ANALYSIS_DEBUG_ARTIFACTS", true),
		PromptProfile:           profile,
		PromptVersion:           envString("UNIFIED_ANALYSIS_PROMPT_VERSION", ""),
		OutputValidationEnabled: envBool("UNIFIED_ANALYSIS_OUTPUT_VALIDATION_ENABLED", true),
	}
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// EffectiveMaxOutputTokens returns the runtime output budget for a prompt profile.
func (c Config) EffectiveMaxOutputTokens(profile string) int {
	if isAnalysisProfileName(profile) {
		if c.AnalysisMaxOutputTokens > 0 {
			return c.AnalysisMaxOutputTokens
		}
		if c.MaxOutputTokens > 0 {
			return c.MaxOutputTokens
		}
		return defaultAnalysisMaxOutputTokens
	}
	if c.SmokeMaxOutputTokens > 0 {
		return c.SmokeMaxOutputTokens
	}
	if c.MaxOutputTokens > 0 {
		return c.MaxOutputTokens
	}
	return defaultSmokeMaxOutputTokens
}

// KnownModelMaxOutputTokens returns the documented max output cap for a model.
// known=false means no hard cap should be applied without an explicit policy.
func KnownModelMaxOutputTokens(model string) (cap int, known bool) {
	key := strings.ToLower(strings.TrimSpace(model))
	cap, known = knownModelMaxOutputTokens[key]
	return cap, known
}

func isAnalysisProfileName(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "analysis", "production", "analyst":
		return true
	default:
		return false
	}
}

// Validate checks configuration. When disabled, API key is not required.
func (c Config) Validate(apiKey string) error {
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("OPENAI_API_KEY is required when UNIFIED_ANALYSIS_ENABLED=true")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("UNIFIED_ANALYSIS_MODEL is required when UNIFIED_ANALYSIS_ENABLED=true")
	}
	providerName := strings.TrimSpace(c.Provider)
	if providerName == "" {
		return fmt.Errorf("UNIFIED_ANALYSIS_PROVIDER is required when UNIFIED_ANALYSIS_ENABLED=true")
	}
	if providerName != ProviderNameOpenAI {
		return fmt.Errorf("UNIFIED_ANALYSIS_PROVIDER must be %q", ProviderNameOpenAI)
	}
	mem := strings.ToLower(strings.TrimSpace(c.MemoryLimit))
	if _, ok := allowedMemoryLimits[mem]; !ok {
		return fmt.Errorf("UNIFIED_ANALYSIS_MEMORY_LIMIT must be one of: 1g, 4g, 16g, 64g")
	}
	if c.AnalysisMaxOutputTokens <= 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS must be greater than 0")
	}
	if c.SmokeMaxOutputTokens <= 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS must be greater than 0")
	}
	if c.MaxOutputTokens <= 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS must be greater than 0")
	}
	if c.ProviderMaxAttempts < 1 || c.ProviderMaxAttempts > maxProviderMaxAttempts {
		return fmt.Errorf("UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS must be between 1 and %d", maxProviderMaxAttempts)
	}
	if cap, known := KnownModelMaxOutputTokens(c.Model); known {
		if c.AnalysisMaxOutputTokens > cap {
			return fmt.Errorf("UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS %d exceeds model %s cap %d", c.AnalysisMaxOutputTokens, c.Model, cap)
		}
		if c.SmokeMaxOutputTokens > cap {
			return fmt.Errorf("UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS %d exceeds model %s cap %d", c.SmokeMaxOutputTokens, c.Model, cap)
		}
	}
	if c.PollIntervalSeconds <= 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_POLL_INTERVAL_SECONDS must be greater than 0")
	}
	if c.MaxPollDurationSeconds <= 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_MAX_POLL_DURATION_SECONDS must be greater than 0")
	}
	if c.HTTPTimeoutSeconds <= 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_HTTP_TIMEOUT_SECONDS must be greater than 0")
	}
	if c.TransportRetries < 0 {
		return fmt.Errorf("UNIFIED_ANALYSIS_TRANSPORT_RETRIES must be greater than or equal to 0")
	}
	return nil
}

func envString(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

// envIntOptional returns a pointer when the env var is set to a valid int.
func envIntOptional(key string) *int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &parsed
}

func envIntWithFallback(key string, legacy *int, profileDefault int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v != "" {
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed
		}
	}
	if legacy != nil {
		return *legacy
	}
	return profileDefault
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
