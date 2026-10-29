package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// TestDevtoolsRunAIPipelineAssembly_ConfiguresChunkRunner mirrors cmd/devtools/ai_pipeline.go
// wiring and ensures shared chunk analysis gets a non-nil runner when enabled.
func TestDevtoolsRunAIPipelineAssembly_ConfiguresChunkRunner(t *testing.T) {
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")
	t.Setenv("AI_PROVIDER_MODE", "real")
	t.Setenv("OPENAI_API_KEY", "sk-real-key-not-placeholder")

	if !SharedChunkAnalysisEnabled() {
		t.Fatal("expected shared chunk analysis enabled")
	}

	providers, err := ResolvePipelineProviders(RunSellerAIPipelineRequest{AIEnabled: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !UseRealProvider(providers.Dashboard) {
		t.Fatalf("dashboard provider=%q want real", providers.Dashboard)
	}

	narrativeSvc := &dashboardhome.NarrativeService{}
	svc := NewSellerAIPipelineService(nil, nil, nil, nil, nil)
	svc.SetChunkAnalysisRunner(SelectChunkAnalysisRunner(narrativeSvc, providers.Dashboard))

	if ChunkAnalyzerForTest(svc) == nil {
		t.Fatal("expected non-nil ChunkAnalysisRunner for devtools assembly")
	}
	if _, ok := ChunkAnalyzerForTest(svc).(*DashboardNarrativeAdapter); !ok {
		t.Fatalf("runner type=%T want *DashboardNarrativeAdapter", ChunkAnalyzerForTest(svc))
	}
}
