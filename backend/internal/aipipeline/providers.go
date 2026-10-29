package aipipeline

import (
	"fmt"
	"strings"
)

const (
	ProviderModeReal = "real"
	ProviderModeFake = "fake"
	ProviderModeAuto = "auto"

	ProviderOpenAI            = "openai"
	ProviderFakeGlobalContext = "fake-global-context-deterministic"
	ProviderFakeDashboard     = "fake-dashboard-deterministic"
	ProviderFakeActions       = "fake-ai-first-deterministic"
)

// PipelineProviderOverrides allows per-component provider selection in devtools.
type PipelineProviderOverrides struct {
	GlobalContext string `json:"global_context,omitempty"`
	Dashboard     string `json:"dashboard,omitempty"`
	Actions       string `json:"actions,omitempty"`
}

// PipelineProviders reports which provider backed each pipeline step.
type PipelineProviders struct {
	GlobalContext string `json:"global_context"`
	Dashboard     string `json:"dashboard"`
	Actions       string `json:"actions"`
}

// ResolvePipelineProviders picks provider labels for the run.
func ResolvePipelineProviders(req RunSellerAIPipelineRequest, hasOpenAI bool) (PipelineProviders, error) {
	if UseFakeAIProviders() {
		return PipelineProviders{
			GlobalContext: ProviderFakeGlobalContext,
			Dashboard:     ProviderFakeDashboard,
			Actions:       ProviderFakeActions,
		}, nil
	}
	over := req.ProviderOverrides
	gcReal, gcLabel, err := resolveComponentProvider(over.GlobalContext, req.AIEnabled, hasOpenAI, ProviderFakeGlobalContext)
	if err != nil {
		return PipelineProviders{}, fmt.Errorf("global_context provider: %w", err)
	}
	dashReal, dashLabel, err := resolveComponentProvider(over.Dashboard, req.AIEnabled, hasOpenAI, ProviderFakeDashboard)
	if err != nil {
		return PipelineProviders{}, fmt.Errorf("dashboard provider: %w", err)
	}
	actReal, actLabel, err := resolveComponentProvider(over.Actions, req.AIEnabled, hasOpenAI, ProviderFakeActions)
	if err != nil {
		return PipelineProviders{}, fmt.Errorf("actions provider: %w", err)
	}

	out := PipelineProviders{
		GlobalContext: gcLabel,
		Dashboard:     dashLabel,
		Actions:       actLabel,
	}
	_ = gcReal
	_ = dashReal
	_ = actReal
	return out, nil
}

func resolveComponentProvider(override string, aiEnabled, hasOpenAI bool, fakeLabel string) (useReal bool, label string, err error) {
	switch strings.ToLower(strings.TrimSpace(override)) {
	case "", ProviderModeAuto:
		if aiEnabled {
			if !hasOpenAI {
				return false, fakeLabel, fmt.Errorf("openai client is not configured")
			}
			return true, ProviderOpenAI, nil
		}
		return false, fakeLabel, nil
	case ProviderModeReal, ProviderOpenAI:
		if !hasOpenAI {
			return false, "", fmt.Errorf("openai client is not configured")
		}
		return true, ProviderOpenAI, nil
	case ProviderModeFake:
		return false, fakeLabel, nil
	default:
		return false, "", fmt.Errorf("unknown provider mode %q", override)
	}
}

// UseRealProvider reports whether the resolved label means OpenAI.
func UseRealProvider(label string) bool {
	return label == ProviderOpenAI
}

// DefaultProvidersFromRequest infers provider labels when ResolvedProviders is empty (tests).
func DefaultProvidersFromRequest(req RunSellerAIPipelineRequest) PipelineProviders {
	if req.AIEnabled {
		return PipelineProviders{
			GlobalContext: ProviderOpenAI,
			Dashboard:     ProviderOpenAI,
			Actions:       ProviderOpenAI,
		}
	}
	return PipelineProviders{
		GlobalContext: ProviderFakeGlobalContext,
		Dashboard:     ProviderFakeDashboard,
		Actions:       ProviderFakeActions,
	}
}
