package runtime

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func NewDeepProviderFromEnv() (deep.Provider, error) {
	timeout := deep.CurrentModelPolicy().TimeoutSeconds
	if timeout <= 0 {
		timeout = 180
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_DEEP_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	key, _ := config.LoadSecretFromFileOrEnv("OPENAI_API_KEY_FILE", "OPENAI_API_KEY")
	return NewDeepProvider(DiscoveryProviderConfig{
		Mode:           os.Getenv("AI_PROVIDER_MODE"),
		APIKey:         key,
		Environment:    firstNonEmpty(os.Getenv("APP_ENV"), os.Getenv("ENVIRONMENT")),
		HTTPSProxy:     strings.TrimSpace(os.Getenv("AI_HTTPS_PROXY")),
		Model:          deep.CurrentModelPolicy().Model,
		TimeoutSeconds: timeout,
	})
}

func NewDeepProvider(cfg DiscoveryProviderConfig) (deep.Provider, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	env := strings.ToLower(strings.TrimSpace(cfg.Environment))
	build := cfg.BuildHTTPClient
	if build == nil {
		build = openaix.BuildHTTPClient
	}
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 180
	}
	switch mode {
	case "real", "openai":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, errors.New("OpenAI API key is required")
		}
		httpClient, err := build(timeout, cfg.HTTPSProxy)
		if err != nil {
			return nil, fmt.Errorf("build OpenAI client: %w", err)
		}
		return &deep.OpenAIProvider{
			Client: openaix.NewResponsesClient(httpClient, cfg.APIKey, ""),
			Model:  firstNonEmpty(cfg.Model, deep.CurrentModelPolicy().Model),
		}, nil
	case "fake":
		if env == "production" || env == "staging" {
			return nil, errors.New("fake Deep provider forbidden in production")
		}
		return &deep.FakeProvider{}, nil
	case "":
		if env == "production" || env == "staging" {
			return nil, errors.New("AI_PROVIDER_MODE is required in production")
		}
		return &deep.FakeProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported AI_PROVIDER_MODE %q", cfg.Mode)
	}
}
