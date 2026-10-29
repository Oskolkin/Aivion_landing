package publication

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"

// EffectiveSettings resolves the effective Settings for a seller given their
// (possibly nil) seller_analysis_settings row and the deployment-wide
// GlobalFlags. A nil row means the seller has never been configured and is
// treated as legacy with every rollout flag off — this is the safe default
// so unconfigured sellers are never auto-run/auto-published.
func EffectiveSettings(row *dbgen.SellerAnalysisSetting, global GlobalFlags) Settings {
	if row == nil {
		return Settings{
			Mode:                  AnalysisModeLegacy,
			LegacyFallbackEnabled: true,
			Visibility:            VisibilityShadow,
		}
	}

	mode := AnalysisMode(row.AnalysisMode)
	if !mode.IsValid() {
		mode = AnalysisModeLegacy
	}

	settings := Settings{
		Mode:                  mode,
		AutoRunEnabled:        row.AutoRunEnabled,
		AutoPublishEnabled:    row.AutoPublishEnabled,
		LegacyFallbackEnabled: row.LegacyFallbackEnabled,
		Visibility:            VisibilityShadow,
	}

	switch mode {
	case AnalysisModeShadow:
		// Shadow mode still allows EffectiveAutoPublish: PublishRun uses it to
		// decide *when* to materialize the shadow projection automatically,
		// not whether the result is user-visible. Visibility stays shadow
		// regardless, so a shadow publish never becomes the seller's current
		// (visible) dashboard/actions source.
		settings.EffectiveAutoRun = global.RolloutEnabled && global.AutoRunEnabled && row.AutoRunEnabled
		settings.EffectiveAutoPublish = global.RolloutEnabled && global.AutoPublishEnabled && row.AutoPublishEnabled
		settings.Visibility = VisibilityShadow
	case AnalysisModeUnified:
		settings.EffectiveAutoRun = global.RolloutEnabled && global.AutoRunEnabled && row.AutoRunEnabled
		settings.EffectiveAutoPublish = global.RolloutEnabled && global.AutoPublishEnabled && row.AutoPublishEnabled
		settings.Visibility = VisibilityVisible
	default:
		// legacy: no auto-run, no auto-publish, regardless of stored flags.
	}

	return settings
}
