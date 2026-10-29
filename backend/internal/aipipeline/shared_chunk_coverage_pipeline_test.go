package aipipeline_test

import (
	"context"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type countingSnapshotPublisher struct {
	calls int
}

func (c *countingSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	c.calls++
	return aipipeline.PublishResult{}, nil
}

func TestRun_CoverageFailureBlocksDownstream(t *testing.T) {
	sharedChunkPipelineEnv(t)
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	pub := &countingSnapshotPublisher{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		dash,
		act,
		&fakeStore{},
		gc,
	)
	aipipeline.SetChunkSetBuilderForTest(svc, func(pack *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
		return aipipeline.Run68StyleChunkSetForTests(), nil
	})
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(&stubChunkAnalyzer{
		fn: func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			run := aipipeline.Run68StyleFailedChunkRunForTests(chunkSet)
			hashes := make([]string, 0, len(chunkSet.Chunks))
			for _, ch := range chunkSet.Chunks {
				hashes = append(hashes, ch.ChunkHash)
			}
			run.ChunkHashes = hashes
			return run, nil
		},
	})

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "shared_chunk_coverage_insufficient" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if gc.calls != 0 || dash.calls != 0 || act.calls != 0 {
		t.Fatalf("downstream gc=%d dash=%d act=%d", gc.calls, dash.calls, act.calls)
	}
	if pub.calls != 0 {
		t.Fatalf("publication calls=%d", pub.calls)
	}
	if res.Diagnostics == nil || res.Diagnostics.SharedChunkAnalysis.CoverageGatePassed {
		t.Fatalf("diagnostics=%+v", res.Diagnostics)
	}
}

func TestRun_CoveragePassInvokesGlobalContextOnce(t *testing.T) {
	sharedChunkPipelineEnv(t)
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s error=%s", res.Status, res.ErrorCode)
	}
	if gc.calls != 1 {
		t.Fatalf("global context calls=%d", gc.calls)
	}
	if res.Diagnostics == nil || !res.Diagnostics.SharedChunkAnalysis.CoverageGatePassed {
		t.Fatalf("coverage diagnostics=%+v", res.Diagnostics)
	}
}
