package aipipeline_test

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type countingChunkAnalyzer struct {
	calls int
}

func (c *countingChunkAnalyzer) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts dashboardhome.NarrativeRunOptions,
) (*dashboardhome.ChunkAnalysisRunResult, error) {
	c.calls++
	return aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
}

func TestSetChunkAnalysisRunner(t *testing.T) {
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	runner := &countingChunkAnalyzer{}
	svc.SetChunkAnalysisRunner(runner)
	if aipipeline.ChunkAnalyzerForTest(svc) != runner {
		t.Fatal("expected runner to be set")
	}
}

func TestRun_SharedChunkFlagFalse_AllowsNilRunner(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	svc := newTestPipelineService()
	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode == "shared_chunk_analysis_runner_not_configured" {
		t.Fatal("unexpected shared chunk runner error with flag disabled")
	}
}

func TestRun_SharedChunkFlagTrue_NilRunnerFailsBeforeDataPack(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	var buildCount int
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack(), count: &buildCount},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
	if res.ErrorCode != "shared_chunk_analysis_runner_not_configured" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if buildCount != 0 {
		t.Fatalf("data pack build count=%d want 0", buildCount)
	}
}

func TestRun_SharedChunkFlagTrue_ConfiguredRunnerCalledOnce(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	runner := &countingChunkAnalyzer{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPackForGuard(t)},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode == "shared_chunk_analysis_runner_not_configured" {
		t.Fatal("unexpected shared chunk runner error")
	}
	if runner.calls != 1 {
		t.Fatalf("chunk analyzer calls=%d want 1", runner.calls)
	}
}

func sharedChunkTestPackForGuard(t *testing.T) *aidatapack.SellerAIDataPack {
	t.Helper()
	pack, _, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	return pack
}
