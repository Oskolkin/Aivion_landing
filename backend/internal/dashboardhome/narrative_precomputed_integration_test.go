package dashboardhome

import (
	"context"
	"testing"
)

func TestPrecomputedDashboardFlow_PersistFrozenSnapshotAndSynthesisOnly(t *testing.T) {
	withLegacyDashboardMode(t)
	EnablePersistFrozenDashboardSnapshotTrackingForTest()
	defer ResetPersistFrozenDashboardSnapshotTrackingForTest()
	ResetBuildChunkSetForPackCallsForTest()
	defer ResetBuildChunkSetForPackHookForTest()

	pack, chunkSet, run := precomputedTestFixtures(t)
	pipelineRunID := int64(77)
	opts := NarrativeRunOptions{
		DataPackOverride:    pack,
		ChunkSetOverride:    chunkSet,
		PrecomputedChunkRun: &run,
		PipelineRunID:       &pipelineRunID,
		DataPackHash:        "dp-hash",
		GlobalContextHash:   "gc-hash",
	}

	if !ShouldPersistFrozenDashboardSnapshotForTest(opts) {
		t.Fatal("expected persist gate for pipeline opts")
	}
	svc := &Service{}
	_, _ = svc.persistFrozenDashboardSnapshot(
		context.Background(),
		pipelineRunID,
		pack.SellerContext.SellerAccountID,
		precomputedMustDate(t),
		opts.DataPackHash,
		opts.GlobalContextHash,
		analyticsDTO(),
		Response{},
	)
	if PersistFrozenDashboardSnapshotCallsForTest() != 1 {
		t.Fatalf("persist calls=%d want 1", PersistFrozenDashboardSnapshotCallsForTest())
	}

	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	if err := ValidatePrecomputedChunkRun(pack, chunkSet, &run); err != nil {
		t.Fatal(err)
	}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, opts)
	if gen.Fatal {
		t.Fatalf("expected successful synthesis, err=%v", gen.Error)
	}
	if opts.ChunkSetOverride == nil || opts.PrecomputedChunkRun == nil {
		t.Fatal("expected overrides in unified flow")
	}
	if BuildChunkSetForPackCallsForTest() != 0 {
		t.Fatalf("chunk build calls=%d", BuildChunkSetForPackCallsForTest())
	}
	if ai.chunkCalls != 0 {
		t.Fatalf("internal chunk AI calls=%d", ai.chunkCalls)
	}
	if ai.synthesisCalls != 1 {
		t.Fatalf("synthesis calls=%d", ai.synthesisCalls)
	}
}
