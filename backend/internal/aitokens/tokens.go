package aitokens

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// DefaultMaxInputTokensPerRequest is the project-wide fallback when env limits are unset or invalid.
const DefaultMaxInputTokensPerRequest = 120000

const (
	envMaxInputTokensPerRequest = "AI_MAX_INPUT_TOKENS_PER_REQUEST"
	envMaxInputTokensApprox     = "AI_MAX_INPUT_TOKENS_APPROX"
)

// EstimateJSONTokens returns a rough token estimate using the project-wide len(json)/4 heuristic.
func EstimateJSONTokens(v any) (int, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	return EstimateBytesTokens(raw), nil
}

// EstimateBytesTokens estimates tokens from serialized JSON bytes.
func EstimateBytesTokens(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	// Common OpenAI pre-flight heuristic: ~4 chars per token.
	return (len(raw) + 3) / 4
}

// MaxInputTokensPerRequest reads AI_MAX_INPUT_TOKENS_PER_REQUEST with fallback to
// AI_MAX_INPUT_TOKENS_APPROX, then DefaultMaxInputTokensPerRequest.
func MaxInputTokensPerRequest() int {
	if v := envInt(envMaxInputTokensPerRequest, 0); v > 0 {
		return v
	}
	return envInt(envMaxInputTokensApprox, DefaultMaxInputTokensPerRequest)
}

func envInt(key string, fallback int) int {
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
