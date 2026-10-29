package aipipeline

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

// SelectGlobalContextOpenAIClient returns an OpenAI client with Global Context stage timeout.
func SelectGlobalContextOpenAIClient(
	narrativeSvc *dashboardhome.NarrativeService,
	cfg *config.Config,
) (*recommendations.OpenAIClient, int, error) {
	if narrativeSvc == nil || narrativeSvc.OpenAIClient() == nil {
		return nil, 0, fmt.Errorf("openai client is not configured")
	}
	timeoutSeconds := openAIGlobalContextTimeoutSeconds(cfg)
	if timeoutSeconds <= 0 {
		return nil, 0, fmt.Errorf("global context timeout must be positive")
	}
	base := narrativeSvc.OpenAIClient()
	if int(base.HTTPClientTimeout().Seconds()) == timeoutSeconds {
		return base, timeoutSeconds, nil
	}
	cloned, err := base.CloneWithTimeout(timeoutSeconds)
	if err != nil {
		return nil, 0, err
	}
	return cloned, timeoutSeconds, nil
}

func openAIGlobalContextTimeoutSeconds(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return cfg.OpenAI.GlobalContextRequestTimeoutSeconds()
}

// SelectGlobalContextClient wires the Global Context AI client for runtime/devtools.
func SelectGlobalContextClient(
	narrativeSvc *dashboardhome.NarrativeService,
	cfg *config.Config,
	provider string,
) (GlobalContextAIClient, error) {
	if UseRealProvider(provider) {
		ai, timeoutSeconds, err := SelectGlobalContextOpenAIClient(narrativeSvc, cfg)
		if err != nil {
			return nil, err
		}
		return &OpenAIGlobalContextClient{AI: ai, TimeoutSeconds: timeoutSeconds}, nil
	}
	return NewDeterministicGlobalContextFakeClient(), nil
}

// SelectActionsOpenAIClient returns an OpenAI client with Actions stage timeout.
func SelectActionsOpenAIClient(
	narrativeSvc *dashboardhome.NarrativeService,
	cfg *config.Config,
) (*recommendations.OpenAIClient, int, error) {
	if narrativeSvc == nil || narrativeSvc.OpenAIClient() == nil {
		return nil, 0, fmt.Errorf("openai client is not configured")
	}
	return SelectActionsOpenAIClientFromBase(narrativeSvc.OpenAIClient(), cfg)
}

// SelectActionsOpenAIClientFromBase clones the base client with Actions stage timeout when needed.
func SelectActionsOpenAIClientFromBase(
	base *recommendations.OpenAIClient,
	cfg *config.Config,
) (*recommendations.OpenAIClient, int, error) {
	if base == nil {
		return nil, 0, fmt.Errorf("openai client is not configured")
	}
	timeoutSeconds := openAIActionsTimeoutSeconds(cfg)
	if timeoutSeconds <= 0 {
		return nil, 0, fmt.Errorf("actions timeout must be positive")
	}
	if int(base.HTTPClientTimeout().Seconds()) == timeoutSeconds {
		return base, timeoutSeconds, nil
	}
	cloned, err := base.CloneWithTimeout(timeoutSeconds)
	if err != nil {
		return nil, 0, err
	}
	return cloned, timeoutSeconds, nil
}

func openAIActionsTimeoutSeconds(cfg *config.Config) int {
	if cfg == nil {
		return openaix.DefaultActionsTimeoutSeconds
	}
	return cfg.OpenAI.ActionsRequestTimeoutSeconds()
}

func openAIActionsDetailTimeoutSeconds(cfg *config.Config) int {
	if cfg == nil {
		return openaix.DefaultActionsTimeoutSeconds
	}
	return cfg.OpenAI.ActionObjectDetailRequestTimeoutSeconds()
}

// SelectActionsOpenAIClientsFromBase returns compose and detail clients with independent timeouts.
func SelectActionsOpenAIClientsFromBase(
	base *recommendations.OpenAIClient,
	cfg *config.Config,
) (composeClient, detailClient *recommendations.OpenAIClient, composeTimeout, detailTimeout int, err error) {
	composeClient, composeTimeout, err = SelectActionsOpenAIClientFromBase(base, cfg)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	detailTimeout = openAIActionsDetailTimeoutSeconds(cfg)
	if detailTimeout <= 0 {
		return nil, nil, 0, 0, fmt.Errorf("actions object detail timeout must be positive")
	}
	if int(composeClient.HTTPClientTimeout().Seconds()) == detailTimeout {
		detailClient = composeClient
		return composeClient, detailClient, composeTimeout, detailTimeout, nil
	}
	detailClient, err = base.CloneWithTimeout(detailTimeout)
	if err != nil {
		return nil, nil, 0, 0, err
	}
	return composeClient, detailClient, composeTimeout, detailTimeout, nil
}
