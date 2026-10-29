package aipipeline

import (
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

// AIUsage captures normalized provider usage for billing and diagnostics.
type AIUsage struct {
	Provider     string
	Model        string
	RequestID    string
	InputTokens  int
	OutputTokens int
	LatencyMs    int64
	UsageSource  string
}

func normalizeTokenCount(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func usageFromGlobalContextRaw(providerLabel, defaultModel string, raw *GlobalContextRawResponse, err error) AIUsage {
	u := AIUsage{
		Provider:    usageProviderName(providerLabel),
		Model:       defaultModel,
		UsageSource: UsageSourceProviderMissing,
	}
	if raw != nil {
		if raw.Model != "" {
			u.Model = raw.Model
		}
		u.RequestID = raw.RequestID
		u.InputTokens = normalizeTokenCount(raw.InputTokens)
		u.OutputTokens = normalizeTokenCount(raw.OutputTokens)
		u.LatencyMs = raw.LatencyMs
		if u.InputTokens > 0 || u.OutputTokens > 0 {
			u.UsageSource = usageSourceForProvider(providerLabel, true)
		}
	}
	if !UseRealProvider(providerLabel) {
		u.UsageSource = UsageSourceFake
	}
	if err != nil && u.UsageSource == UsageSourceProviderMissing {
		u.UsageSource = UsageSourceProviderMissing
	}
	return u
}

func usageFromAIRaw(providerLabel, defaultModel string, raw *actions.AIRawResponse) AIUsage {
	u := AIUsage{
		Provider:    usageProviderName(providerLabel),
		Model:       defaultModel,
		UsageSource: UsageSourceProviderMissing,
	}
	if raw != nil {
		if raw.Model != "" {
			u.Model = raw.Model
		}
		u.RequestID = raw.RequestID
		u.InputTokens = normalizeTokenCount(raw.InputTokens)
		u.OutputTokens = normalizeTokenCount(raw.OutputTokens)
		u.LatencyMs = raw.LatencyMs
		if u.InputTokens > 0 || u.OutputTokens > 0 {
			u.UsageSource = usageSourceForProvider(providerLabel, true)
		}
	}
	if !UseRealProvider(providerLabel) {
		if u.InputTokens > 0 || u.OutputTokens > 0 {
			u.UsageSource = UsageSourceFake
		}
	}
	return u
}

func usageSourceForProvider(providerLabel string, hasProviderTokens bool) string {
	if !UseRealProvider(providerLabel) {
		return UsageSourceFake
	}
	if hasProviderTokens {
		return UsageSourceProvider
	}
	return UsageSourceProviderMissing
}

func usageProviderName(providerLabel string) string {
	if UseRealProvider(providerLabel) {
		return ProviderOpenAI
	}
	return ProviderModeFake
}

func usageBillingModel(providerLabel, model string) string {
	if strings.TrimSpace(model) != "" {
		return model
	}
	if UseRealProvider(providerLabel) {
		return ProviderOpenAI
	}
	return ProviderModeFake
}
