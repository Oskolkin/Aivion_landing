package actions

import (
	"os"
	"strings"
)

func GenerateOptionsFromEnv() GenerateActionsOptions {
	opts := DefaultGenerateActionsOptions()
	opts.AIEnabled = envBool("ACTIONS_AI_ENABLED", opts.AIEnabled)
	opts.AIFirst = envBool("ACTIONS_AI_FIRST", false)
	opts.EnableAIRepair = envBool("ACTIONS_AI_REPAIR_ENABLED", opts.EnableAIRepair)
	opts.EnableLegacyGlobalCheck = envBool("ACTIONS_LEGACY_GLOBAL_CHECK_ENABLED", opts.EnableLegacyGlobalCheck)
	return opts
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch raw {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}
