package eval

import (
	"os"
	"strconv"
	"strings"
)

func ResolveConfiguredModels() ResolvedModels {
	return ResolvedModels{
		Discovery: ConfiguredActualModel{
			Configured:     firstEnv("OPENAI_DISCOVERY_MODEL", "OPENAI_MODEL", "gpt-4.1-mini"),
			PolicyVersion:  "discovery_model_policy_v1",
			MaxOutput:      envInt("OPENAI_DISCOVERY_MAX_OUTPUT_TOKENS", 0),
			TimeoutSeconds: envInt("OPENAI_DISCOVERY_TIMEOUT_SECONDS", 0),
		},
		Deep: ConfiguredActualModel{
			Configured:      firstEnv("OPENAI_DEEP_MODEL", "OPENAI_MODEL", "gpt-5.4"),
			PolicyVersion:   "deep_model_policy_v1",
			ReasoningEffort: strings.TrimSpace(os.Getenv("OPENAI_DEEP_REASONING_EFFORT")),
			Verbosity:       strings.TrimSpace(os.Getenv("OPENAI_DEEP_VERBOSITY")),
			MaxOutput:       envInt("OPENAI_DEEP_MAX_OUTPUT_TOKENS", 0),
			TimeoutSeconds:  envInt("OPENAI_DEEP_TIMEOUT_SECONDS", 180),
		},
		Chat: ConfiguredActualModel{
			Configured:     firstEnv("OPENAI_CHAT_MODEL", "OPENAI_MODEL", "gpt-4.1-mini"),
			PolicyVersion:  "chat_model_policy_v1",
			TimeoutSeconds: envInt("OPENAI_TIMEOUT_SECONDS", 90),
		},
	}
}

func ApplyActualModel(cfg *ConfiguredActualModel, actual string) {
	if cfg == nil {
		return
	}
	cfg.Actual = strings.TrimSpace(actual)
	if cfg.Actual != "" && cfg.Configured != "" && cfg.Actual != cfg.Configured && !strings.HasPrefix(cfg.Actual, cfg.Configured) {
		cfg.Mismatch = true
		cfg.MismatchCode = ModelIdentityMismatch
	}
}

func firstEnv(keys ...string) string {
	for i, k := range keys {
		if i == len(keys)-1 && !strings.Contains(k, "_") {
			return k
		}
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
