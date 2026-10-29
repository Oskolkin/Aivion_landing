package main

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func TestWireDevtoolsPipelineChunkRunner_RealProviderNonNil(t *testing.T) {
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")
	t.Setenv("AI_PROVIDER_MODE", "real")
	t.Setenv("OPENAI_API_KEY", "sk-real-key-not-placeholder")

	providers, err := aipipeline.ResolvePipelineProviders(aipipeline.RunSellerAIPipelineRequest{AIEnabled: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	narrativeSvc := &dashboardhome.NarrativeService{}
	svc := aipipeline.NewSellerAIPipelineService(nil, nil, nil, nil, nil)
	wireDevtoolsPipelineChunkRunner(svc, narrativeSvc, providers)

	runner := aipipeline.SelectChunkAnalysisRunner(narrativeSvc, providers.Dashboard)
	if runner == nil {
		t.Fatal("expected non-nil chunk analysis runner")
	}
	if _, ok := runner.(*aipipeline.DashboardNarrativeAdapter); !ok {
		t.Fatalf("runner type=%T", runner)
	}
}

func TestValidateRunAIPipelineFlagsResumeAndForce(t *testing.T) {
	if code := runAIPipeline([]string{"--seller-id=1", "--metric-date=2026-07-08", "--resume-run-id=69", "--force=true"}); code != 2 {
		t.Fatalf("exit=%d want 2", code)
	}
}

func TestResolveRunAIPipelineAIEnabledResumeDefault(t *testing.T) {
	if !resolveRunAIPipelineAIEnabled(true, "") {
		t.Fatal("resume should default ai-enabled true")
	}
	if resolveRunAIPipelineAIEnabled(false, "") {
		t.Fatal("new run should default ai-enabled false")
	}
}

func TestDevtoolsPipelineProxyDiagnostics_AIProxyURL(t *testing.T) {
	t.Setenv("AI_PROXY_URL", "http://127.0.0.1:10809")
	t.Setenv("AI_HTTPS_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")

	summary := openaix.ProxySummaryFromEnv()
	if !summary.AIProxyConfigured {
		t.Fatalf("summary=%+v", summary)
	}
	if summary.EffectiveProxySource != openaix.ProxySourceAIProxyURL {
		t.Fatalf("source=%q", summary.EffectiveProxySource)
	}
	if summary.EffectiveProxyScheme != "http" || summary.EffectiveProxyHost != "127.0.0.1:10809" {
		t.Fatalf("summary=%+v", summary)
	}
}
