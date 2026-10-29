package analysissource

import (
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// Local mirrors of unifiedanalysis/publication's mode/visibility/status
// string values. Duplicated (rather than imported) because that package
// imports dashboardhome/actions, and this package must be importable from
// both without creating a cycle. The underlying DB columns are the single
// source of truth; these constants just need to match those string values.
const (
	analysisModeLegacy  = "legacy"
	analysisModeShadow  = "shadow"
	analysisModeUnified = "unified"

	visibilityVisible = "visible"

	publicationStatusPublished = "published"
)

// GlobalFlags are the deployment-wide rollout kill switches consulted when
// resolving a seller's current analysis source. Mirrors the subset of
// unifiedanalysis/publication.GlobalFlags relevant to the read path.
type GlobalFlags struct {
	// RolloutEnabled is the master switch for unified analysis rollout. When
	// false, unified is never surfaced as the seller-facing read source
	// regardless of seller settings (manual/admin publish calls still work).
	RolloutEnabled bool
	// LegacyFallbackEnabled gates whether the resolver may fall back to the
	// legacy pipeline when unified is unavailable/invalid.
	LegacyFallbackEnabled bool
}

// GlobalFlagsFromEnv loads GlobalFlags from the same UNIFIED_ANALYSIS_* env
// vars unifiedanalysis/publication.GlobalFlagsFromEnv reads, so both
// packages observe identical rollout configuration without importing each
// other.
func GlobalFlagsFromEnv() GlobalFlags {
	return GlobalFlags{
		RolloutEnabled:        envBool("UNIFIED_ANALYSIS_ROLLOUT_ENABLED", false),
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

// effectiveSettings resolves the effective per-seller settings given the
// (possibly nil) seller_analysis_settings row and GlobalFlags. A nil row
// means the seller has never been configured and is treated as legacy with
// fallback enabled — the safe default for unconfigured sellers.
type effectiveSettings struct {
	Mode                  string
	Visibility            string
	LegacyFallbackEnabled bool
}

func resolveEffectiveSettings(row *dbgen.SellerAnalysisSetting) effectiveSettings {
	if row == nil {
		return effectiveSettings{
			Mode:                  analysisModeLegacy,
			Visibility:            "shadow",
			LegacyFallbackEnabled: true,
		}
	}
	mode := strings.TrimSpace(row.AnalysisMode)
	switch mode {
	case analysisModeLegacy, analysisModeShadow, analysisModeUnified:
	default:
		mode = analysisModeLegacy
	}
	settings := effectiveSettings{
		Mode:                  mode,
		LegacyFallbackEnabled: row.LegacyFallbackEnabled,
		Visibility:            "shadow",
	}
	if mode == analysisModeUnified {
		settings.Visibility = visibilityVisible
	}
	return settings
}
