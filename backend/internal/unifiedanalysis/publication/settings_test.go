package publication

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func allEnabledGlobalFlags() GlobalFlags {
	return GlobalFlags{
		RolloutEnabled:        true,
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
}

func TestEffectiveSettings_NilRow_IsSafeLegacyDefault(t *testing.T) {
	settings := EffectiveSettings(nil, allEnabledGlobalFlags())
	if settings.Mode != AnalysisModeLegacy {
		t.Fatalf("mode = %q, want legacy", settings.Mode)
	}
	if settings.EffectiveAutoRun || settings.EffectiveAutoPublish {
		t.Fatalf("unconfigured seller must never auto-run/auto-publish: got %+v", settings)
	}
	if settings.Visibility != VisibilityShadow {
		t.Fatalf("visibility = %q, want shadow", settings.Visibility)
	}
}

func TestEffectiveSettings_ShadowMode_AllowsAutoPublish(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          string(AnalysisModeShadow),
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
	settings := EffectiveSettings(row, allEnabledGlobalFlags())

	if !settings.EffectiveAutoRun {
		t.Fatal("shadow mode with global+seller auto_run enabled must have EffectiveAutoRun=true")
	}
	// The core Stage 5 fix under test: shadow mode + global+seller auto_publish
	// enabled MUST allow EffectiveAutoPublish=true. PublishRun uses this to decide
	// *when* to materialize the shadow projection automatically; it never affects
	// visibility, which stays shadow regardless (checked below).
	if !settings.EffectiveAutoPublish {
		t.Fatal("shadow mode with global+seller auto_publish enabled must have EffectiveAutoPublish=true")
	}
	if settings.Visibility != VisibilityShadow {
		t.Fatalf("shadow mode visibility must stay shadow even when auto-publish is effective, got %q", settings.Visibility)
	}
}

func TestEffectiveSettings_ShadowMode_SellerFlagOffDisablesAutoPublish(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          string(AnalysisModeShadow),
		AutoRunEnabled:        true,
		AutoPublishEnabled:    false,
		LegacyFallbackEnabled: true,
	}
	settings := EffectiveSettings(row, allEnabledGlobalFlags())
	if settings.EffectiveAutoPublish {
		t.Fatal("seller auto_publish_enabled=false must yield EffectiveAutoPublish=false even in shadow mode")
	}
}

func TestEffectiveSettings_ShadowMode_GlobalFlagOffDisablesAutoPublish(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          string(AnalysisModeShadow),
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
	global := allEnabledGlobalFlags()
	global.AutoPublishEnabled = false
	settings := EffectiveSettings(row, global)
	if settings.EffectiveAutoPublish {
		t.Fatal("global AUTO_PUBLISH_ENABLED=false must yield EffectiveAutoPublish=false even in shadow mode")
	}
}

func TestEffectiveSettings_ShadowMode_RolloutOffDisablesEverything(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          string(AnalysisModeShadow),
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
	global := allEnabledGlobalFlags()
	global.RolloutEnabled = false
	settings := EffectiveSettings(row, global)
	if settings.EffectiveAutoRun || settings.EffectiveAutoPublish {
		t.Fatalf("global rollout kill switch must disable both auto-run and auto-publish, got %+v", settings)
	}
}

func TestEffectiveSettings_UnifiedMode_VisibleAndAutoPublish(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          string(AnalysisModeUnified),
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
	settings := EffectiveSettings(row, allEnabledGlobalFlags())
	if !settings.EffectiveAutoRun || !settings.EffectiveAutoPublish {
		t.Fatalf("unified mode with everything enabled must have both effective flags true, got %+v", settings)
	}
	if settings.Visibility != VisibilityVisible {
		t.Fatalf("unified mode visibility must be visible, got %q", settings.Visibility)
	}
}

func TestEffectiveSettings_LegacyMode_NeverAutoRunOrPublish(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          string(AnalysisModeLegacy),
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
	settings := EffectiveSettings(row, allEnabledGlobalFlags())
	if settings.EffectiveAutoRun || settings.EffectiveAutoPublish {
		t.Fatalf("legacy mode must never auto-run/auto-publish regardless of stored flags, got %+v", settings)
	}
}

func TestEffectiveSettings_InvalidMode_TreatedAsLegacy(t *testing.T) {
	row := &dbgen.SellerAnalysisSetting{
		AnalysisMode:          "not_a_real_mode",
		AutoRunEnabled:        true,
		AutoPublishEnabled:    true,
		LegacyFallbackEnabled: true,
	}
	settings := EffectiveSettings(row, allEnabledGlobalFlags())
	if settings.Mode != AnalysisModeLegacy {
		t.Fatalf("invalid stored mode must fall back to legacy, got %q", settings.Mode)
	}
	if settings.EffectiveAutoRun || settings.EffectiveAutoPublish {
		t.Fatalf("invalid mode fallback must not auto-run/auto-publish, got %+v", settings)
	}
}
