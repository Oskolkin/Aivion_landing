package aipipeline_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestProviderSelectionRealVsFake(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	req := aipipeline.RunSellerAIPipelineRequest{
		AIEnabled: true,
	}
	providers, err := aipipeline.ResolvePipelineProviders(req, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{providers.GlobalContext, providers.Dashboard, providers.Actions} {
		if p != aipipeline.ProviderOpenAI {
			t.Fatalf("expected openai providers, got %+v", providers)
		}
	}

	req.AIEnabled = false
	providers, err = aipipeline.ResolvePipelineProviders(req, true)
	if err != nil {
		t.Fatal(err)
	}
	if providers.GlobalContext != aipipeline.ProviderFakeGlobalContext {
		t.Fatalf("expected fake global context, got %s", providers.GlobalContext)
	}
}

func TestRealRunDiagnosticsShape(t *testing.T) {
	providers := aipipeline.PipelineProviders{
		GlobalContext: aipipeline.ProviderOpenAI,
		Dashboard:     aipipeline.ProviderOpenAI,
		Actions:       aipipeline.ProviderOpenAI,
	}
	diag := aipipeline.NewPipelineDiagnosticsForTest(providers)
	if diag.GlobalContext.Provider != aipipeline.ProviderOpenAI {
		t.Fatal("missing global context provider")
	}
	if diag.Actions.Provider != aipipeline.ProviderOpenAI {
		t.Fatal("missing actions provider")
	}
}
