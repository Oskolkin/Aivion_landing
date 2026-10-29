package aipipeline

import (
	"fmt"
	"os"
	"strings"
)

const (
	AIProviderModeOpenAI = "openai"
	AIProviderModeReal   = "real"
	AIProviderModeFake   = "fake"
)

var placeholderOpenAIKeys = map[string]struct{}{
	"":                              {},
	"compose-local-fake-openai-key": {},
	"local-dev-placeholder":         {},
	"sk-local-compose-fake":         {},
	"fake":                          {},
	"test":                          {},
}

func openAIKeyConfigured() bool {
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY_FILE")) != "" {
		return true
	}
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
}

// AIProviderMode returns configured AI provider mode (openai or fake).
// AI_PROVIDER_MODE=real is normalized to openai.
func AIProviderMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER_MODE")))
	switch mode {
	case AIProviderModeFake:
		return AIProviderModeFake
	case AIProviderModeOpenAI, AIProviderModeReal:
		return AIProviderModeOpenAI
	case "":
		if !openAIKeyConfigured() {
			return AIProviderModeFake
		}
		return AIProviderModeOpenAI
	default:
		return mode
	}
}

// UseFakeAIProviders reports whether runtime should use deterministic fake AI providers.
func UseFakeAIProviders() bool {
	return AIProviderMode() == AIProviderModeFake
}

// ValidateAIProviderConfig returns an error when production config is unsafe.
func ValidateAIProviderConfig(appEnv, openAIKey string) error {
	rawMode := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER_MODE")))
	switch rawMode {
	case "", AIProviderModeFake, AIProviderModeOpenAI, AIProviderModeReal:
	default:
		return fmt.Errorf("unknown AI_PROVIDER_MODE %q", rawMode)
	}

	mode := AIProviderMode()
	env := strings.ToLower(strings.TrimSpace(appEnv))

	if mode == AIProviderModeFake {
		if env == "production" || env == "staging" {
			if os.Getenv("AI_PROVIDER_ALLOW_FAKE_IN_PRODUCTION") != "1" {
				return fmt.Errorf("AI_PROVIDER_MODE=fake is not allowed in %s", env)
			}
		}
		return nil
	}

	key := strings.TrimSpace(openAIKey)
	if key == "" {
		return fmt.Errorf("OpenAI API key is required when AI_PROVIDER_MODE is real or openai (set OPENAI_API_KEY_FILE or OPENAI_API_KEY)")
	}
	if _, ok := placeholderOpenAIKeys[strings.ToLower(key)]; ok {
		return fmt.Errorf("placeholder OpenAI API key is not allowed when AI_PROVIDER_MODE is real or openai")
	}
	if env == "production" && strings.HasPrefix(strings.ToLower(key), "compose-") {
		return fmt.Errorf("placeholder OpenAI API key is not allowed in production")
	}
	return nil
}

// ValidateRuntimeModeConfig ensures unified and legacy modes are not both enabled.
func ValidateRuntimeModeConfig() error {
	if err := ValidatePipelineConfig(); err != nil {
		return err
	}
	if SellerAIPipelineEnabled() && LegacyAIPostSyncEnabled() {
		return fmt.Errorf("SELLER_AI_PIPELINE_ENABLED and LEGACY_AI_POST_SYNC_ENABLED cannot both be true")
	}
	return nil
}
