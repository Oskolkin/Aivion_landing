package actions

import (
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

const (
	defaultObjectDetailBatchSize       = 10
	defaultObjectDetailMaxConcurrency  = 2
	defaultObjectDetailMaxOutputTokens = 8192
	defaultComposeMaxOutputTokens      = 16384
)

// ActionsObjectDetailBatchSize returns entities per Object Detail batch (1–20).
func ActionsObjectDetailBatchSize() int {
	return clampInt(twoPhaseEnvInt("ACTIONS_OBJECT_DETAIL_BATCH_SIZE", defaultObjectDetailBatchSize), 1, 20)
}

// ActionsObjectDetailMaxConcurrency returns max parallel detail calls (1–4).
func ActionsObjectDetailMaxConcurrency() int {
	return clampInt(twoPhaseEnvInt("ACTIONS_OBJECT_DETAIL_MAX_CONCURRENCY", defaultObjectDetailMaxConcurrency), 1, 4)
}

// ActionsObjectDetailRequestedMaxOutputTokens returns max_output_tokens for detail batches.
func ActionsObjectDetailRequestedMaxOutputTokens() int {
	if v := twoPhaseEnvInt("OPENAI_ACTION_OBJECT_DETAIL_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	return defaultObjectDetailMaxOutputTokens
}

// ActionsComposeTimeoutSeconds returns HTTP timeout for Compose provider calls only.
func ActionsComposeTimeoutSeconds() int {
	if v := twoPhaseEnvInt("OPENAI_ACTIONS_TIMEOUT_SECONDS", 0); v > 0 {
		return v
	}
	return openaix.DefaultActionsTimeoutSeconds
}

// ActionsObjectDetailTimeoutSeconds returns HTTP timeout for detail batch calls.
func ActionsObjectDetailTimeoutSeconds() int {
	if v := twoPhaseEnvInt("OPENAI_ACTION_OBJECT_DETAIL_TIMEOUT_SECONDS", 0); v > 0 {
		return v
	}
	return openaix.DefaultActionsTimeoutSeconds
}

// ActionsComposeEffectiveMaxOutputTokens returns the max_output_tokens budget actually sent for Compose.
func ActionsComposeEffectiveMaxOutputTokens() int {
	return ActionsComposeRequestedMaxOutputTokens()
}

// ActionsComposeOutputLimitWiring describes which env keys resolved the compose output limit.
func ActionsComposeOutputLimitWiring() string {
	if v := strings.TrimSpace(os.Getenv("OPENAI_ACTIONS_MAX_OUTPUT_TOKENS")); v != "" {
		return "OPENAI_ACTIONS_MAX_OUTPUT_TOKENS"
	}
	if v := strings.TrimSpace(os.Getenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST")); v != "" {
		return "AI_MAX_OUTPUT_TOKENS_PER_REQUEST"
	}
	if v := strings.TrimSpace(os.Getenv("AI_MAX_OUTPUT_TOKENS")); v != "" {
		return "AI_MAX_OUTPUT_TOKENS"
	}
	return "default_compose_max_output_tokens"
}

// ActionsComposeRequestedMaxOutputTokens returns max_output_tokens for Compose phase.
func ActionsComposeRequestedMaxOutputTokens() int {
	if v := twoPhaseEnvInt("OPENAI_ACTIONS_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	if v := twoPhaseEnvInt("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", 0); v > 0 {
		return v
	}
	if v := twoPhaseEnvInt("AI_MAX_OUTPUT_TOKENS", 0); v > 0 {
		return v
	}
	return defaultComposeMaxOutputTokens
}

// IsTwoPhaseActionsPrompt reports whether the prompt version uses Compose + Object Detail.
func IsTwoPhaseActionsPrompt(version string) bool {
	return strings.TrimSpace(version) == actionsAIFirstV2PromptVersion
}

func twoPhaseEnvInt(key string, fallback int) int {
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

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
