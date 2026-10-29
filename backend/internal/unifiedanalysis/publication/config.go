package publication

import (
	"os"
	"strconv"
	"strings"
)

// GlobalFlags are the global (deployment-wide) rollout kill switches for
// Stage 5 unified analysis publication. Per-seller behavior is always the
// AND of these global flags and the seller's own seller_analysis_settings row
// (see EffectiveSettings).
type GlobalFlags struct {
	// RolloutEnabled is the master switch for unified analysis rollout. When
	// false, no seller is auto-run or auto-published regardless of their own
	// settings (manual/admin-triggered publish calls are still allowed).
	RolloutEnabled bool
	// AutoRunEnabled gates automatic scheduling of unified analysis runs.
	AutoRunEnabled bool
	// AutoPublishEnabled gates automatic (non-admin-triggered) PublishRun calls.
	AutoPublishEnabled bool
	// LegacyFallbackEnabled gates whether the selector may fall back to the
	// legacy pipeline when unified is unavailable/stale (see selector.go).
	LegacyFallbackEnabled bool
}

// GlobalFlagsFromEnv loads GlobalFlags from UNIFIED_ANALYSIS_* env vars.
func GlobalFlagsFromEnv() GlobalFlags {
	return GlobalFlags{
		RolloutEnabled:        envBool("UNIFIED_ANALYSIS_ROLLOUT_ENABLED", false),
		AutoRunEnabled:        envBool("UNIFIED_ANALYSIS_AUTO_RUN_ENABLED", false),
		AutoPublishEnabled:    envBool("UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED", false),
		LegacyFallbackEnabled: envBool("UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED", true),
	}
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
