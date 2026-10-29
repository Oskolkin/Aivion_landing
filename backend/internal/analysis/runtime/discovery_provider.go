package runtime

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

type DiscoveryProviderConfig struct {
	Mode            string
	APIKey          string
	Environment     string
	HTTPSProxy      string
	Model           string
	TimeoutSeconds  int
	BuildHTTPClient func(timeoutSeconds int, httpsProxy string) (*http.Client, error)
}

func NewDiscoveryProvider(cfg DiscoveryProviderConfig) (discovery.Provider, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	env := strings.ToLower(strings.TrimSpace(cfg.Environment))
	build := cfg.BuildHTTPClient
	if build == nil {
		build = openaix.BuildHTTPClient
	}
	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 120
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
		return &discovery.OpenAIProvider{
			Client: openaix.NewResponsesClient(httpClient, cfg.APIKey, ""),
			Model:  firstNonEmpty(cfg.Model, discoveryModelFromEnv()),
		}, nil
	case "fake":
		if env == "production" || env == "staging" {
			return nil, errors.New("fake Discovery provider forbidden in production")
		}
		return &discovery.FakeProvider{}, nil
	case "":
		if env == "production" || env == "staging" {
			return nil, errors.New("AI_PROVIDER_MODE is required in production")
		}
		return &discovery.FakeProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported AI_PROVIDER_MODE %q", cfg.Mode)
	}
}

func NewDiscoveryProviderFromEnv() (discovery.Provider, error) {
	timeout := 120
	if v := strings.TrimSpace(os.Getenv("OPENAI_DISCOVERY_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	key, _ := config.LoadSecretFromFileOrEnv("OPENAI_API_KEY_FILE", "OPENAI_API_KEY")
	return NewDiscoveryProvider(DiscoveryProviderConfig{
		Mode:           os.Getenv("AI_PROVIDER_MODE"),
		APIKey:         key,
		Environment:    firstNonEmpty(os.Getenv("APP_ENV"), os.Getenv("ENVIRONMENT")),
		HTTPSProxy:     strings.TrimSpace(os.Getenv("AI_HTTPS_PROXY")),
		Model:          discoveryModelFromEnv(),
		TimeoutSeconds: timeout,
	})
}
