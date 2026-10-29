package aipipeline

import (
	"os"
	"strconv"
	"strings"
)

const defaultGlobalContextMaxOutputTokens = 16384

// GlobalContextRequestedMaxOutputTokens returns max_output_tokens for Global Context AI calls.
// Priority: OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS > default 16384.
func GlobalContextRequestedMaxOutputTokens() int {
	if v := globalContextEnvInt("OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	return defaultGlobalContextMaxOutputTokens
}

// GlobalContextEffectiveMaxOutputTokens is the effective limit used at runtime (same as requested).
func GlobalContextEffectiveMaxOutputTokens() int {
	return GlobalContextRequestedMaxOutputTokens()
}

func globalContextEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
