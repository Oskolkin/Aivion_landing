package aipipeline_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func TestRun_Stage8_PartialValidResponsePublishes(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			if len(input.ActionCandidates) == 0 {
				t.Fatal("expected candidates")
			}
			c := input.ActionCandidates[0]
			actionsOut := make([]actions.AIFirstDraftAction, 0, 10)
			for i := 0; i < 8; i++ {
				d := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
				d.Title = fmt.Sprintf("Valid action %d", i+1)
				actionsOut = append(actionsOut, d)
			}
			bad := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
			bad.Title = ""
			actionsOut = append(actionsOut, bad, bad)
			return &actions.ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: actionsOut}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ActionsStatus != actions.GenerationFinalStatusCompletedPartial {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published || publisher.calls != 1 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
	if len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("retry calls=%d", len(fakeAI.RetryAIFirstCalls))
	}
}

func TestRun_Stage8_AllInvalidBlocksPublication(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			drafts := make([]actions.AIFirstDraftAction, 0, 4)
			for i, c := range input.ActionCandidates {
				if i >= 4 {
					break
				}
				d := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
				d.SourceCandidateIDs = []string{"unknown:candidate:99"}
				drafts = append(drafts, d)
			}
			return &actions.ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: drafts}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published || publisher.calls != 0 || res.ActionsStatus != aipipeline.StepStatusFailed {
		t.Fatalf("published=%v calls=%d status=%s", res.Published, publisher.calls, res.ActionsStatus)
	}
}

func TestRun_Stage8_UnsupportedClientBlocksPublication(t *testing.T) {
	legacyOnly := legacyOnlyActionAIClient{}
	svc := actions.NewGenerationService(nil, legacyOnly, &stage7MinimalGenRepo{})
	runner := &stage7CapturingActionsRunner{inner: &aipipeline.ActionsGenerationAdapter{GenerationService: svc}}
	publisher := &recordingSnapshotPublisher{}
	pipeline := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), runner, &fakeStore{}, publisher, nil)

	res, err := pipeline.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published || publisher.calls != 0 || res.ActionsStatus != aipipeline.StepStatusFailed {
		t.Fatalf("published=%v calls=%d status=%s", res.Published, publisher.calls, res.ActionsStatus)
	}
}

type legacyOnlyActionAIClient struct{}

func (legacyOnlyActionAIClient) ComposeDomainActions(ctx context.Context, input actions.DomainComposerInput) (*actions.DomainComposerOutput, *actions.AIRawResponse, error) {
	return &actions.DomainComposerOutput{}, &actions.AIRawResponse{}, nil
}

func (legacyOnlyActionAIClient) RepairDomainActions(ctx context.Context, input actions.DomainRepairInput) (*actions.DomainComposerOutput, *actions.AIRawResponse, error) {
	return &actions.DomainComposerOutput{}, &actions.AIRawResponse{}, nil
}

func (legacyOnlyActionAIClient) RunLegacyGlobalCheck(ctx context.Context, input actions.LegacyGlobalCheckInput) (*actions.LegacyGlobalCheckOutput, *actions.AIRawResponse, error) {
	return &actions.LegacyGlobalCheckOutput{}, &actions.AIRawResponse{}, nil
}

func TestRun_Stage8_PartialTopActionKeysPreserveOrder(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			if len(input.ActionCandidates) == 0 {
				t.Fatal("expected candidates")
			}
			c := input.ActionCandidates[0]
			drafts := []actions.AIFirstDraftAction{
				stage8TitledDraft(c, "A"),
				stage8TitledDraft(c, ""),
				stage8TitledDraft(c, "C"),
				stage8TitledDraft(c, "D"),
				stage8TitledDraft(c, "E"),
			}
			return &actions.ActionsAIFirstOutput{SchemaVersion: input.SchemaVersion, Actions: drafts}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)
	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Published || res.Diagnostics.Actions.TopActionKeysCount != 3 {
		t.Fatalf("published=%v top_count=%d", res.Published, res.Diagnostics.Actions.TopActionKeysCount)
	}
}

func stage8TitledDraft(c actions.AIFirstActionCandidate, title string) actions.AIFirstDraftAction {
	d := actions.ValidDetailedPilotDraftFromCandidateForTests(c)
	d.Title = title
	return d
}

func TestRun_Stage8_AIDisabledWithCandidates_FailsWithoutCompose(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			t.Fatal("compose must not be called when AI is disabled")
			return nil, nil, nil
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	req := sharedChunkPipelineRequest()
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(fakeAI.ComposeAIFirstCalls) != 0 || len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("compose=%d retry=%d", len(fakeAI.ComposeAIFirstCalls), len(fakeAI.RetryAIFirstCalls))
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
	if res.ActionsStatus != aipipeline.StepStatusFailed {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if res.ErrorCode != actions.ActionsErrorAIDisabled {
		t.Fatalf("error_code=%q want %s", res.ErrorCode, actions.ActionsErrorAIDisabled)
	}
	if res.Diagnostics == nil || res.Diagnostics.Actions.ComposeCalls != 0 {
		t.Fatalf("compose_calls=%d", res.Diagnostics.Actions.ComposeCalls)
	}
}

func TestRun_Stage8_AIDisabledZeroCandidates_RemainsBusinessEmpty(t *testing.T) {
	runner := &stubChunkAnalyzer{fn: func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
		res, err := aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
		if err != nil {
			return nil, err
		}
		res.ValidatedActionCandidates = []actions.AIFirstActionCandidate{}
		res.ValidatedCandidateCount = 0
		return res, nil
	}}
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			t.Fatal("compose must not be called for zero candidates")
			return nil, nil, nil
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, runner)

	req := sharedChunkPipelineRequest()
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(fakeAI.ComposeAIFirstCalls) != 0 {
		t.Fatalf("compose calls=%d", len(fakeAI.ComposeAIFirstCalls))
	}
	if res.ActionsStatus != "empty" {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published || publisher.calls != 1 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
}
