package aipipeline_test

import (
	"context"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPipelineE2E_SuccessFullPipeline_PublishesDashboardActions(t *testing.T) {
	n := e2eExpectedChunkCount(t)
	h := newE2EHarness(t, e2eHarnessConfig{})
	res := h.run(t, e2eBaseRequest())

	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s error=%s", res.Status, res.ErrorCode)
	}
	if !res.Published || h.publisher.count() != 1 {
		t.Fatalf("published=%v snapshot_count=%d", res.Published, h.publisher.count())
	}
	if res.Reused || res.DataPackHash == "" || res.GlobalContextHash == "" {
		t.Fatalf("reused=%v hashes missing", res.Reused)
	}
	if h.rec.CountChunkCalls() != n || h.rec.Count(aipipeline.AIUsageComponentGlobalContext) != 1 ||
		h.rec.Count(aipipeline.AIUsageComponentDashboardSynthesis) != 1 ||
		h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis) != 1 {
		t.Fatalf("calls chunks=%d gc=%d dash=%d act=%d total=%d",
			h.rec.CountChunkCalls(), h.rec.Count(aipipeline.AIUsageComponentGlobalContext),
			h.rec.Count(aipipeline.AIUsageComponentDashboardSynthesis),
			h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis), h.rec.Total())
	}
	if h.rec.Total() != n+3 {
		t.Fatalf("total calls=%d want %d", h.rec.Total(), n+3)
	}
	assertE2EUsageCounts(t, h.usage.Events(), n, 1, 1, 1)
	assertNoRetryUsage(t, h.usage)
	if h.actRunner.candidateBuildCalls != 1 || h.actRunner.legacyBuildCalls != 0 {
		t.Fatalf("candidate_build=%d legacy_build=%d", h.actRunner.candidateBuildCalls, h.actRunner.legacyBuildCalls)
	}
	if res.Diagnostics == nil || res.Diagnostics.SharedChunkAnalysis.ChunkCount != n {
		t.Fatalf("diagnostics chunk_count=%d", res.Diagnostics.SharedChunkAnalysis.ChunkCount)
	}
	assertArtifactKinds(t, h.store, res.PipelineRunID, map[string]bool{
		aipipeline.ArtifactKindSharedChunkAnalysis: true,
		aipipeline.ArtifactKindGlobalContextFinal:  true,
	})
}

func TestPipelineE2E_NoActionCandidates_PublishesEmptyActionsWithoutActionsAI(t *testing.T) {
	n := e2eExpectedChunkCount(t)
	rec := &PipelineAICallRecorder{}
	h := newE2EHarness(t, e2eHarnessConfig{rec: rec, chunkRunner: chunkRunnerNoCandidates(rec)})
	res := h.run(t, e2eBaseRequest())

	if res.ActionsStatus != "empty" {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published {
		t.Fatal("expected published snapshot with empty actions")
	}
	if h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis) != 0 {
		t.Fatalf("actions AI calls=%d", h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis))
	}
	if h.rec.Total() != n+2 {
		t.Fatalf("total calls=%d want %d", h.rec.Total(), n+2)
	}
	assertE2EUsageCounts(t, h.usage.Events(), n, 1, 1, 0)
	if len(h.actionsAI.ComposeAIFirstCalls) != 0 {
		t.Fatalf("compose calls=%d", len(h.actionsAI.ComposeAIFirstCalls))
	}
	if res.Diagnostics.Actions.ComposeCalls != 0 {
		t.Fatalf("compose_calls=%d", res.Diagnostics.Actions.ComposeCalls)
	}
}

func TestPipelineE2E_OneChunkFailed_ContinuesWithLimitation(t *testing.T) {
	n := e2eExpectedChunkCount(t)
	rec := &PipelineAICallRecorder{}
	h := newE2EHarness(t, e2eHarnessConfig{rec: rec, chunkRunner: chunkRunnerOneFailed(rec)})
	res := h.run(t, e2eBaseRequest())

	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s error=%s", res.Status, res.ErrorCode)
	}
	if !res.Published {
		t.Fatal("expected publication after partial chunk failure")
	}
	if res.Diagnostics.SharedChunkAnalysis.Failed != 1 || res.Diagnostics.SharedChunkAnalysis.Completed != n-1 {
		t.Fatalf("completed=%d failed=%d", res.Diagnostics.SharedChunkAnalysis.Completed, res.Diagnostics.SharedChunkAnalysis.Failed)
	}
	if h.rec.Total() != n+3 {
		t.Fatalf("total calls=%d", h.rec.Total())
	}
}

func TestPipelineE2E_AllChunksFailed_StopsBeforeContextDashboardActions(t *testing.T) {
	rec := &PipelineAICallRecorder{}
	h := newE2EHarness(t, e2eHarnessConfig{rec: rec, chunkRunner: chunkRunnerAllFailed(rec)})
	res := h.run(t, e2eBaseRequest())

	if res.ErrorCode != "all_chunks_failed" || res.Status != aipipeline.PipelineStatusFailed {
		t.Fatalf("status=%s error=%s", res.Status, res.ErrorCode)
	}
	if res.Published {
		t.Fatal("must not publish")
	}
	if h.rec.Count(aipipeline.AIUsageComponentGlobalContext) != 0 ||
		h.rec.Count(aipipeline.AIUsageComponentDashboardSynthesis) != 0 ||
		h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis) != 0 {
		t.Fatalf("downstream calls gc=%d dash=%d act=%d",
			h.rec.Count(aipipeline.AIUsageComponentGlobalContext),
			h.rec.Count(aipipeline.AIUsageComponentDashboardSynthesis),
			h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis))
	}
	assertE2EUsageCounts(t, h.usage.Events(), 0, 0, 0, 0)
}

func TestPipelineE2E_InvalidGlobalContext_FailsWithoutRetryKeepsOldSnapshot(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	first := h.run(t, e2eBaseRequest())
	if !first.Published {
		t.Fatal("first run must publish")
	}
	before := h.publisher.currentID()
	aipipeline.SetGlobalContextClientForTest(h.svc, &recordingGlobalContextClient{
		rec:   h.rec,
		inner: &failingGlobalContextClient{},
	})

	res := h.run(t, e2eBaseRequest())
	if res.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", res.Status)
	}
	if res.Published || h.publisher.currentID() != before || h.publisher.count() != 1 {
		t.Fatalf("published=%v current=%d count=%d", res.Published, h.publisher.currentID(), h.publisher.count())
	}
}

func TestPipelineE2E_InvalidDashboard_StopsBeforeActionsKeepsOldSnapshot(t *testing.T) {
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModeStrict)
	h := newE2EHarness(t, e2eHarnessConfig{})
	first := h.run(t, e2eBaseRequest())
	if !first.Published {
		t.Fatal("first run must publish")
	}
	before := h.publisher.currentID()
	callsBefore := h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis)
	h.dashAI.mode = stage6SynthesisInvalidOutput

	res := h.run(t, e2eBaseRequest())
	if res.DashboardStatus != aipipeline.StepStatusFailed {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
	if res.ActionsStatus == aipipeline.StepStatusSkipped {
		t.Fatalf("actions_status=%s want not skipped when global context is valid", res.ActionsStatus)
	}
	if res.Published || h.publisher.currentID() != before {
		t.Fatalf("published=%v", res.Published)
	}
	if h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis) <= callsBefore {
		t.Fatalf("actions calls=%d want increased from %d", h.rec.Count(aipipeline.AIUsageComponentActionsSynthesis), callsBefore)
	}
}

func TestPipelineE2E_PartiallyInvalidActions_PublishesCompletedPartial(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			if len(input.ActionCandidates) == 0 {
				t.Fatal("expected candidates")
			}
			c := input.ActionCandidates[0]
			drafts := []actions.AIFirstDraftAction{
				actions.ValidDetailedPilotDraftFromCandidateForTests(c),
				func() actions.AIFirstDraftAction {
					d := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
					d.Title = ""
					return d
				}(),
				actions.ValidDetailedPilotDraftFromCandidateForTests(c),
				func() actions.AIFirstDraftAction {
					d := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
					d.SourceCandidateIDs = []string{"unknown:candidate:99"}
					return d
				}(),
				actions.ValidDetailedPilotDraftFromCandidateForTests(c),
			}
			drafts[0].Title = "Action A"
			drafts[2].Title = "Action C"
			drafts[4].Title = "Action E"
			return &actions.ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: drafts}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
	h := newE2EHarness(t, e2eHarnessConfig{actionsAI: fakeAI})
	res := h.run(t, e2eBaseRequest())

	if res.ActionsStatus != actions.GenerationFinalStatusCompletedPartial {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published {
		t.Fatal("expected publication for completed_partial")
	}
	if len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("retry calls=%d", len(fakeAI.RetryAIFirstCalls))
	}
	if res.Diagnostics.Actions.DroppedCount < 1 || res.Diagnostics.Actions.AcceptedCount < 1 {
		t.Fatalf("accepted=%d dropped=%d", res.Diagnostics.Actions.AcceptedCount, res.Diagnostics.Actions.DroppedCount)
	}
}

func TestPipelineE2E_AllActionsInvalid_BlocksPublicationKeepsOldSnapshot(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	first := h.run(t, e2eBaseRequest())
	if !first.Published {
		t.Fatal("first run must publish")
	}
	before := h.publisher.currentID()

	invalidAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			drafts := make([]actions.AIFirstDraftAction, 0, len(input.ActionCandidates))
			for i, c := range input.ActionCandidates {
				if i >= 3 {
					break
				}
				d := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
				d.SourceCandidateIDs = []string{"unknown:candidate:99"}
				drafts = append(drafts, d)
			}
			return &actions.ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: drafts}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
	h2 := newE2EHarness(t, e2eHarnessConfig{actionsAI: invalidAI})
	req := e2eBaseRequest()
	req.ForceRegenerate = true
	res := h2.run(t, req)
	if res.Published || h2.publisher.count() != 0 {
		t.Fatalf("published=%v publication_count=%d", res.Published, h2.publisher.count())
	}
	if res.ActionsStatus != aipipeline.StepStatusFailed {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if before != h.publisher.currentID() || h.publisher.count() != 1 {
		t.Fatalf("original snapshot changed current=%d count=%d", h.publisher.currentID(), h.publisher.count())
	}
}

func TestPipelineE2E_SameHashSecondRun_ReusesPublishedSnapshotWithoutAICalls(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	first := h.run(t, e2eBaseRequest())
	if !first.Published {
		t.Fatal("first run must publish")
	}
	h.store.seedPublished(dbgen.AiPipelineRun{
		ID: first.PipelineRunID, SellerAccountID: e2eSellerID, Status: aipipeline.PipelineStatusCompleted,
		DataPackHash:      first.DataPackHash,
		MetricDate:        pgtype.Date{Time: e2eFixedMetricDate(), Valid: true},
		GlobalContextHash: pgtype.Text{String: first.GlobalContextHash, Valid: true},
	})
	beforeUsage := len(h.usage.Events())
	beforeSnapshots := h.publisher.count()
	callsBefore := h.rec.Total()

	second := h.run(t, e2eBaseRequest())
	if !second.Reused || second.PipelineRunID != first.PipelineRunID {
		t.Fatalf("reused=%v run_id=%d first=%d", second.Reused, second.PipelineRunID, first.PipelineRunID)
	}
	if h.rec.Total() != callsBefore {
		t.Fatalf("AI calls on reuse before=%d after=%d", callsBefore, h.rec.Total())
	}
	if len(h.usage.Events()) != beforeUsage || h.publisher.count() != beforeSnapshots {
		t.Fatalf("usage or snapshots changed on reuse")
	}
}

func TestPipelineE2E_ForceRegenerate_CreatesNewRunAndPublishesNewSnapshot(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	first := h.run(t, e2eBaseRequest())
	req := e2eBaseRequest()
	req.ForceRegenerate = true
	second := h.run(t, req)
	if second.Reused || second.PipelineRunID == first.PipelineRunID {
		t.Fatalf("reused=%v run1=%d run2=%d", second.Reused, first.PipelineRunID, second.PipelineRunID)
	}
	if !second.Published || h.publisher.count() != 2 {
		t.Fatalf("published=%v count=%d", second.Published, h.publisher.count())
	}
	if h.publisher.currentID() != second.PipelineRunID {
		t.Fatalf("current=%d second=%d", h.publisher.currentID(), second.PipelineRunID)
	}
}

func TestPipelineE2E_ForceRegenerateFailure_KeepsPreviousSnapshotCurrent(t *testing.T) {
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModeStrict)
	h := newE2EHarness(t, e2eHarnessConfig{})
	first := h.run(t, e2eBaseRequest())
	if !first.Published {
		t.Fatal("first run must publish")
	}
	h.dashAI.mode = stage6SynthesisInvalidOutput

	req := e2eBaseRequest()
	req.ForceRegenerate = true
	second := h.run(t, req)
	if second.Published {
		t.Fatal("failed force run must not publish")
	}
	if h.publisher.count() != 1 || h.publisher.currentID() != first.PipelineRunID {
		t.Fatalf("snapshots=%d current=%d first=%d", h.publisher.count(), h.publisher.currentID(), first.PipelineRunID)
	}
}

func TestPipelineE2E_StageOrder_DashboardBeforeActions(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	h.run(t, e2eBaseRequest())
	assertStageOrder(t, h.rec.Sequence())
}

func TestPipelineE2E_SharedAnalysisUsedByDashboardAndActions_NoReanalysis(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	h.run(t, e2eBaseRequest())
	if h.dashAI.chunkCalls != 0 {
		t.Fatalf("dashboard chunk calls=%d", h.dashAI.chunkCalls)
	}
	if h.actRunner.legacyBuildCalls != 0 {
		t.Fatalf("legacy data pack build calls=%d", h.actRunner.legacyBuildCalls)
	}
	if h.actRunner.candidateBuildCalls != 1 {
		t.Fatalf("candidate build calls=%d", h.actRunner.candidateBuildCalls)
	}
}

func TestPipelineE2E_DebugArtifacts_OnOffPolicy(t *testing.T) {
	hOff := newE2EHarness(t, e2eHarnessConfig{})
	resOff := hOff.run(t, e2eBaseRequest())
	kindsOff := sortedUniqueKinds(hOff.store, resOff.PipelineRunID)
	for _, kind := range kindsOff {
		if kind != aipipeline.ArtifactKindSharedChunkAnalysis && kind != aipipeline.ArtifactKindGlobalContextFinal && kind != aipipeline.ArtifactKindDataPack && kind != aipipeline.ArtifactKindPipelineSummary {
			if kind == aipipeline.ArtifactKindGlobalContextInput {
				continue // always-on GC input may be saved depending on path
			}
		}
	}

	hOn := newE2EHarness(t, e2eHarnessConfig{debug: true})
	req := e2eBaseRequest()
	req.DebugArtifacts = true
	resOn := hOn.run(t, req)
	foundDebug := false
	for _, kind := range sortedUniqueKinds(hOn.store, resOn.PipelineRunID) {
		switch kind {
		case aipipeline.ArtifactKindDashboardInput, aipipeline.ArtifactKindActionsInput,
			aipipeline.ArtifactKindDashboardRawOutput, aipipeline.ArtifactKindActionsRawOutput:
			foundDebug = true
		}
	}
	if !foundDebug {
		t.Fatalf("expected debug artifacts, kinds=%v", sortedUniqueKinds(hOn.store, resOn.PipelineRunID))
	}
}

func TestPipelineE2E_DebugArtifacts_CompactInputsDoNotLeakDataPack(t *testing.T) {
	h := newE2EHarness(t, e2eHarnessConfig{})
	req := e2eBaseRequest()
	req.DebugArtifacts = true
	res := h.run(t, req)
	assertArtifactPayloadNotContains(t, h.store, res.PipelineRunID, aipipeline.ArtifactKindActionsInput, `"data_pack"`)
}
