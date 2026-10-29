package aipipeline

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

// EstimateJSONTokens returns a rough token estimate using len(json)/4 heuristic.
func EstimateJSONTokens(v any) (int, error) {
	return aitokens.EstimateJSONTokens(v)
}

// EstimateBytesTokens estimates tokens from serialized JSON bytes.
func EstimateBytesTokens(raw []byte) int {
	return aitokens.EstimateBytesTokens(raw)
}

// MaxAIInputTokensPerRequest delegates to internal/aitokens (canonical resolver).
func MaxAIInputTokensPerRequest() int {
	return aitokens.MaxInputTokensPerRequest()
}

// MaxAIOutputTokensPerRequest returns the configured max_output_tokens budget for OpenAI requests.
func MaxAIOutputTokensPerRequest() int {
	if v := envInt("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", 0); v > 0 {
		return v
	}
	if v := envInt("AI_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	return 4000
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
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
