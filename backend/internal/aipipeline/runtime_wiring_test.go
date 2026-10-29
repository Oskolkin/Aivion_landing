package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func TestSelectChunkAnalysisRunner_RealProviderUsesNarrativeAdapter(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	t.Setenv("OPENAI_API_KEY", "sk-real-key-not-placeholder")

	narrativeSvc := &dashboardhome.NarrativeService{}
	runner := SelectChunkAnalysisRunner(narrativeSvc, ProviderOpenAI)
	adapter, ok := runner.(*DashboardNarrativeAdapter)
	if !ok {
		t.Fatalf("runner type=%T", runner)
	}
	if adapter.NarrativeService != narrativeSvc {
		t.Fatal("expected same narrative service instance")
	}
}

func TestSelectChunkAnalysisRunner_FakeProviderUsesDeterministicRunner(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "fake")
	runner := SelectChunkAnalysisRunner(nil, ProviderFakeDashboard)
	if _, ok := runner.(*DeterministicChunkAnalysisRunner); !ok {
		t.Fatalf("runner type=%T", runner)
	}
}

func TestRuntimeComposition_InjectsChunkAnalyzer(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "fake")
	providers, err := ResolvePipelineProviders(RunSellerAIPipelineRequest{AIEnabled: false}, false)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewSellerAIPipelineService(
		nil,
		selectDashboardRunner(nil, nil, providers.Dashboard),
		nil,
		nil,
		nil,
	)
	svc.SetChunkAnalysisRunner(SelectChunkAnalysisRunner(nil, providers.Dashboard))
	if ChunkAnalyzerForTest(svc) == nil {
		t.Fatal("expected chunk analyzer to be configured")
	}
	if _, ok := ChunkAnalyzerForTest(svc).(*DeterministicChunkAnalysisRunner); !ok {
		t.Fatalf("runner type=%T", ChunkAnalyzerForTest(svc))
	}
}

func TestSelectChunkAnalysisRunner_UnknownProviderUsesFakeDashboardLabelPolicy(t *testing.T) {
	runner := SelectChunkAnalysisRunner(nil, "unknown-provider-label")
	if _, ok := runner.(*DeterministicChunkAnalysisRunner); !ok {
		t.Fatalf("runner type=%T", runner)
	}
}
