package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type stage7CapturingActionsRunner struct {
	legacyBuildCalls    int
	candidateBuildCalls int
	lastInput           actions.ActionsAIFirstInput
	lastCandidates      []actions.AIFirstActionCandidate
	inner               aipipeline.ActionsRunner
}

func (r *stage7CapturingActionsRunner) BuildActionsAIFirstInputFromDataPack(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	r.legacyBuildCalls++
	if r.inner == nil {
		return actions.ActionsAIFirstInput{}, errors.New("inner runner missing")
	}
	return r.inner.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalContext, opts)
}

func (r *stage7CapturingActionsRunner) BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []actions.AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	r.candidateBuildCalls++
	r.lastCandidates = append([]actions.AIFirstActionCandidate(nil), candidates...)
	if r.inner == nil {
		return actions.ActionsAIFirstInput{}, errors.New("inner runner missing")
	}
	input, err := r.inner.BuildActionsAIFirstInputFromCandidates(
		pack, dataPackHash, globalContext, globalContextHash, candidates, failedChunkIDs, dataLimitations, opts,
	)
	if err == nil {
		r.lastInput = input
	}
	return input, err
}

func (r *stage7CapturingActionsRunner) GenerateActionsAIFirstFromInput(
	ctx context.Context,
	input actions.ActionsAIFirstInput,
	opts actions.GenerateActionsOptions,
) (*actions.GenerateActionsResult, error) {
	if r.inner == nil {
		return nil, errors.New("inner runner missing")
	}
	return r.inner.GenerateActionsAIFirstFromInput(ctx, input, opts)
}

type stage7MinimalGenRepo struct {
	nextRunID int64
}

func (r *stage7MinimalGenRepo) GetLatestRun(ctx context.Context, sellerAccountID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetLatestPublishableRun(ctx context.Context, sellerAccountID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetPublishableRunByPipelineID(ctx context.Context, sellerAccountID, pipelineRunID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetLatestMetricDate(ctx context.Context, sellerAccountID int64) (*time.Time, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) ListActionsByRunID(ctx context.Context, runID int64, params actions.ListActionsParams) ([]actions.ActionItemRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) ListActions(ctx context.Context, params actions.ListActionsParams) ([]actions.ActionItemRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetActionByKey(ctx context.Context, sellerAccountID int64, actionKey string) (*actions.ActionItemRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetActionByID(ctx context.Context, sellerAccountID, actionID int64) (*actions.ActionItemRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetHistoricalActionByID(ctx context.Context, sellerAccountID, actionID int64) (*actions.ActionItemRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetActionEntityGroup(ctx context.Context, sellerAccountID, actionsRunID int64, groupKey string) (*actions.ActionEntityGroup, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) GetGenerationRunByID(ctx context.Context, runID int64) (*actions.ActionGenerationRunRecord, error) {
	return nil, nil
}
func (r *stage7MinimalGenRepo) SetActionChecked(ctx context.Context, sellerAccountID int64, actionKey string) error {
	return nil
}
func (r *stage7MinimalGenRepo) SetActionOpen(ctx context.Context, sellerAccountID int64, actionKey string) error {
	return nil
}
func (r *stage7MinimalGenRepo) SetActionDismissed(ctx context.Context, sellerAccountID int64, actionKey string) error {
	return nil
}

func (r *stage7MinimalGenRepo) CreateActionGenerationRun(ctx context.Context, sellerAccountID int64, metricDate time.Time, status string) (int64, error) {
	r.nextRunID++
	return r.nextRunID, nil
}
func (r *stage7MinimalGenRepo) UpdateActionGenerationRun(ctx context.Context, params actions.UpdateGenerationRunParams) error {
	return nil
}
func (r *stage7MinimalGenRepo) CreateActionGenerationCategoryRun(ctx context.Context, params actions.CreateCategoryRunParams) (int64, error) {
	return 1, nil
}
func (r *stage7MinimalGenRepo) UpdateActionGenerationCategoryRun(ctx context.Context, params actions.UpdateCategoryRunParams) error {
	return nil
}
func (r *stage7MinimalGenRepo) UpsertActionItem(ctx context.Context, params actions.UpsertActionItemParams) error {
	return nil
}
func (r *stage7MinimalGenRepo) DeleteStaleActionItemsForDate(ctx context.Context, sellerAccountID int64, metricDate time.Time, keepKeys []string) error {
	return nil
}
func (r *stage7MinimalGenRepo) LinkActionGenerationRunToPipeline(ctx context.Context, pipelineRunID, actionsRunID int64, dataPackHash, globalContextHash string) error {
	return nil
}
func (r *stage7MinimalGenRepo) PersistGenerationAtomic(ctx context.Context, params actions.PersistGenerationParams) error {
	return nil
}

func newStage7RealActionsRunner(fakeAI actions.ActionAIClient) *stage7CapturingActionsRunner {
	svc := actions.NewGenerationService(nil, fakeAI, &stage7MinimalGenRepo{})
	inner := &aipipeline.ActionsGenerationAdapter{GenerationService: svc}
	return &stage7CapturingActionsRunner{inner: inner}
}

func newStage7PipelineService(
	t *testing.T,
	dash aipipeline.DashboardRunner,
	actionsRunner aipipeline.ActionsRunner,
	store aipipeline.PipelineStore,
	publisher aipipeline.SnapshotPublisher,
	chunkRunner aipipeline.ChunkAnalysisRunner,
) *aipipeline.SellerAIPipelineService {
	t.Helper()
	sharedChunkPipelineEnv(t)
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
	pack := sharedChunkTestPack(t)
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		actionsRunner,
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(publisher)
	if chunkRunner != nil {
		svc.SetChunkAnalysisRunner(chunkRunner)
	} else {
		svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())
	}
	return svc
}

func stage7SuccessfulDashboardRunner() aipipeline.DashboardRunner {
	return &precomputedSynthesisDashboardRunner{ai: &stage6SynthesisAI{mode: stage6SynthesisOK}}
}

func stage7ValidatingFakeAI() *actions.FakeActionAIClient {
	return &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			drafts := make([]actions.AIFirstDraftAction, 0, len(input.ActionCandidates))
			for _, c := range input.ActionCandidates {
				drafts = append(drafts, actions.ValidV2DraftFromCandidateForTests(c))
			}
			return &actions.ActionsAIFirstOutput{
				SchemaVersion: input.SchemaVersion,
				Actions:       drafts,
			}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
}

func TestRun_Stage7_SharedCandidatesUseActionsV2_NoLegacyBuilder(t *testing.T) {
	fakeAI := stage7ValidatingFakeAI()
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if actionsRunner.legacyBuildCalls != 0 {
		t.Fatalf("legacy build calls=%d want 0", actionsRunner.legacyBuildCalls)
	}
	if actionsRunner.candidateBuildCalls != 1 {
		t.Fatalf("candidate build calls=%d want 1", actionsRunner.candidateBuildCalls)
	}
	if len(actionsRunner.lastCandidates) == 0 {
		t.Fatal("expected candidates from shared chunk run")
	}
	if actionsRunner.lastInput.SchemaVersion != actions.ActionsAIFirstSchemaVersionV2 {
		t.Fatalf("schema=%s", actionsRunner.lastInput.SchemaVersion)
	}
	wire, err := actions.MarshalActionsAIFirstInputWire(actionsRunner.lastInput)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), `"data_pack"`) {
		t.Fatal("wire must not contain data_pack")
	}
	if len(fakeAI.ComposeAIFirstCalls) != 1 {
		t.Fatalf("compose calls=%d want 1", len(fakeAI.ComposeAIFirstCalls))
	}
	if len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("retry calls=%d", len(fakeAI.RetryAIFirstCalls))
	}
	if !res.Published || publisher.calls != 1 {
		t.Fatalf("published=%v publication_calls=%d status=%s", res.Published, publisher.calls, res.Status)
	}
	for _, item := range actionsRunner.lastCandidates {
		found := false
		for _, c := range actionsRunner.lastInput.ActionCandidates {
			if c.CandidateID == item.CandidateID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("candidate %s missing from V2 input", item.CandidateID)
		}
	}
}

func TestRun_Stage7_ArtifactResume_PassesSameCandidatesToActionsV2(t *testing.T) {
	fakeAI := stage7ValidatingFakeAI()
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	store := &persistingArtifactStore{}
	runner := &stubChunkAnalyzer{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, store, &recordingSnapshotPublisher{}, runner)

	first, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("first analyzer calls=%d", runner.calls)
	}
	firstIDs := candidateIDs(actionsRunner.lastCandidates)

	runner.calls = 0
	actionsRunner.legacyBuildCalls = 0
	actionsRunner.candidateBuildCalls = 0
	fakeAI.ComposeAIFirstCalls = nil

	req := sharedChunkPipelineActionsAIRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 {
		t.Fatalf("resume analyzer calls=%d want 0", runner.calls)
	}
	if actionsRunner.legacyBuildCalls != 0 {
		t.Fatalf("legacy build calls=%d", actionsRunner.legacyBuildCalls)
	}
	if actionsRunner.candidateBuildCalls != 1 {
		t.Fatalf("candidate build calls=%d", actionsRunner.candidateBuildCalls)
	}
	secondIDs := candidateIDs(actionsRunner.lastCandidates)
	if len(secondIDs) == 0 || len(firstIDs) == 0 {
		t.Fatalf("candidate ids first=%v second=%v", firstIDs, secondIDs)
	}
	for i := range firstIDs {
		if firstIDs[i] != secondIDs[i] {
			t.Fatalf("candidate ids changed: first=%v second=%v", firstIDs, secondIDs)
		}
	}
	if second.Diagnostics.SharedChunkAnalysis.Source != "artifact" {
		t.Fatalf("source=%q", second.Diagnostics.SharedChunkAnalysis.Source)
	}
	if len(fakeAI.ComposeAIFirstCalls) != 1 {
		t.Fatalf("compose calls=%d", len(fakeAI.ComposeAIFirstCalls))
	}
}

func TestRun_Stage7_ZeroCandidates_NoProviderBusinessEmptyPublishes(t *testing.T) {
	fakeAI := actions.NewDeterministicAIFirstFakeClient()
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	runner := &stubChunkAnalyzer{fn: func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
		res, err := aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
		if err != nil {
			return nil, err
		}
		res.ValidatedActionCandidates = []actions.AIFirstActionCandidate{}
		res.ValidatedCandidateCount = 0
		return res, nil
	}}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(fakeAI.ComposeAIFirstCalls) != 0 || len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("compose=%d retry=%d", len(fakeAI.ComposeAIFirstCalls), len(fakeAI.RetryAIFirstCalls))
	}
	if res.ActionsStatus != "empty" {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published || publisher.calls != 1 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
}

func TestRun_Stage7_ActionsProviderError_NoPublication(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			return nil, &actions.AIRawResponse{}, errors.New("provider down")
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
	if res.ActionsStatus != aipipeline.StepStatusFailed {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
}

func TestRun_Stage7_ActionsInvalidJSON_NoPublication(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			return nil, &actions.AIRawResponse{RawText: "not-json"}, errors.New("invalid_json: broken")
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
}

func TestRun_Stage7_UnknownSourceCandidate_NoPublication(t *testing.T) {
	fakeAI := &actions.FakeActionAIClient{
		ComposeAIFirstFn: func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
			return &actions.ActionsAIFirstOutput{
				SchemaVersion: input.SchemaVersion,
				Actions: []actions.AIFirstDraftAction{{
					Type: actions.ActionTypeStockReplenishment, Category: actions.ActionCategoryStock, Priority: actions.ActionPriorityCritical,
					Title: "t", Reason: "r", NextStep: "n", CompletionCriteria: "c",
					EvidenceFactIDs:    []string{"fact_1"},
					AffectedEntities:   []actions.AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku_1"}},
					ObjectActions:      []actions.AIFirstObjectAction{{EntityType: "sku", EntityID: "sku_1", RecommendedAction: "a", EvidenceFactIDs: []string{"fact_1"}}},
					SourceCandidateIDs: []string{"unknown:candidate:99"},
				}},
			}, &actions.AIRawResponse{RawText: "{}"}, nil
		},
	}
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
	if len(fakeAI.RetryAIFirstCalls) != 0 {
		t.Fatalf("retry calls=%d", len(fakeAI.RetryAIFirstCalls))
	}
}

func TestRun_Stage7_InputTooLarge_NoPublication(t *testing.T) {
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", "100")
	fakeAI := actions.NewDeterministicAIFirstFakeClient()
	actionsRunner := newStage7RealActionsRunner(fakeAI)
	publisher := &recordingSnapshotPublisher{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), actionsRunner, &fakeStore{}, publisher, nil)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(fakeAI.ComposeAIFirstCalls) != 0 {
		t.Fatalf("compose calls=%d want 0", len(fakeAI.ComposeAIFirstCalls))
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d code=%s", res.Published, publisher.calls, res.ErrorCode)
	}
	if res.ErrorCode != dashboardhome.NarrativeErrorDashboardInputTooLarge {
		t.Fatalf("error_code=%s", res.ErrorCode)
	}
	if res.ActionsStatus == aipipeline.StepStatusSkipped {
		t.Fatalf("actions_status=%s want not skipped when global context is valid", res.ActionsStatus)
	}
}

func candidateIDs(candidates []actions.AIFirstActionCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.CandidateID)
	}
	return out
}

// Ensure artifact payload contains candidates for resume assertions.
func TestRun_Stage7_ArtifactPayloadContainsCandidates(t *testing.T) {
	store := &persistingArtifactStore{}
	svc := newStage7PipelineService(t, stage7SuccessfulDashboardRunner(), &fakeActionsRunner{}, store, &recordingSnapshotPublisher{}, nil)
	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	arts := store.sharedChunkArtifacts()
	if len(arts) != 1 {
		t.Fatalf("artifacts=%d", len(arts))
	}
	var doc struct {
		Payload struct {
			ValidatedActionCandidates []actions.AIFirstActionCandidate `json:"validated_action_candidates"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(arts[0].PayloadJson, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Payload.ValidatedActionCandidates) == 0 {
		t.Fatal("artifact missing validated_action_candidates")
	}
}
