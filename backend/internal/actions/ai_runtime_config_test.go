package actions

import "testing"

func TestGenerateOptionsFromEnv(t *testing.T) {
	t.Setenv("ACTIONS_AI_ENABLED", "false")
	t.Setenv("ACTIONS_AI_REPAIR_ENABLED", "false")
	t.Setenv("ACTIONS_LEGACY_GLOBAL_CHECK_ENABLED", "true")
	t.Setenv("ACTIONS_AI_FIRST", "true")
	opts := GenerateOptionsFromEnv()
	if opts.AIEnabled {
		t.Fatal("AIEnabled should be false")
	}
	if !opts.AIFirst {
		t.Fatal("AIFirst should be true")
	}
	if opts.EnableAIRepair {
		t.Fatal("EnableAIRepair should be false")
	}
	if !opts.EnableLegacyGlobalCheck {
		t.Fatal("EnableLegacyGlobalCheck should be true")
	}
}

func TestGenerateOptionsFromEnv_AIFirstDefaultFalse(t *testing.T) {
	t.Setenv("ACTIONS_AI_ENABLED", "")
	t.Setenv("ACTIONS_AI_REPAIR_ENABLED", "")
	t.Setenv("ACTIONS_LEGACY_GLOBAL_CHECK_ENABLED", "")
	t.Setenv("ACTIONS_AI_FIRST", "")
	opts := GenerateOptionsFromEnv()
	if opts.AIFirst {
		t.Fatal("AIFirst should be false by default")
	}
}
