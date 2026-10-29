package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func sharedChunkPipelineEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")
}

func sharedChunkTestPack(t *testing.T) *aidatapack.SellerAIDataPack {
	t.Helper()
	pack, _, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	return pack
}

func sharedChunkPipelineRequest() aipipeline.RunSellerAIPipelineRequest {
	return aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
		Trigger:           "test",
	}
}

// sharedChunkPipelineActionsAIRequest enables production AI for the Actions stage
// while keeping fake providers available for other components in tests.
func sharedChunkPipelineActionsAIRequest() aipipeline.RunSellerAIPipelineRequest {
	req := sharedChunkPipelineRequest()
	req.AIEnabled = true
	return req
}

type stubChunkAnalyzer struct {
	calls int
	fn    func(
		ctx context.Context,
		pack *aidatapack.SellerAIDataPack,
		chunkSet *aidatapack.ChunkSet,
		opts dashboardhome.NarrativeRunOptions,
	) (*dashboardhome.ChunkAnalysisRunResult, error)
}

func (s *stubChunkAnalyzer) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts dashboardhome.NarrativeRunOptions,
) (*dashboardhome.ChunkAnalysisRunResult, error) {
	s.calls++
	if s.fn != nil {
		return s.fn(ctx, pack, chunkSet, opts)
	}
	return aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
}

type persistingArtifactStore struct {
	fakeStore
	artifacts []dbgen.AiPipelineArtifact
	listErr   error
	createErr error
}

func (s *persistingArtifactStore) CreateArtifact(ctx context.Context, params aipipeline.CreateArtifactParams) (int64, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
	raw, err := json.Marshal(params.Payload)
	if err != nil {
		return 0, err
	}
	id := int64(len(s.artifacts) + 1)
	s.artifacts = append(s.artifacts, dbgen.AiPipelineArtifact{
		ID:            id,
		PipelineRunID: params.PipelineRunID,
		Kind:          params.Kind,
		Hash:          pgtype.Text{String: params.Hash, Valid: params.Hash != ""},
		PayloadJson:   raw,
	})
	return id, nil
}

func (s *persistingArtifactStore) ListArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]dbgen.AiPipelineArtifact, 0, len(s.artifacts))
	for _, art := range s.artifacts {
		if art.PipelineRunID == pipelineRunID {
			out = append(out, art)
		}
	}
	return out, nil
}

func (s *persistingArtifactStore) sharedChunkArtifacts() []dbgen.AiPipelineArtifact {
	out := make([]dbgen.AiPipelineArtifact, 0)
	for _, art := range s.artifacts {
		if art.Kind == aipipeline.ArtifactKindSharedChunkAnalysis {
			out = append(out, art)
		}
	}
	return out
}

func newSharedChunkPipelineService(
	t *testing.T,
	store aipipeline.PipelineStore,
	runner aipipeline.ChunkAnalysisRunner,
) *aipipeline.SellerAIPipelineService {
	t.Helper()
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)
	return svc
}

func TestRun_SharedChunkFlagTrue_AnalyzerCalledOnceOnArtifactMiss(t *testing.T) {
	sharedChunkPipelineEnv(t)
	runner := &stubChunkAnalyzer{}
	svc := newSharedChunkPipelineService(t, &fakeStore{}, runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s error=%s", res.Status, res.ErrorCode)
	}
	if runner.calls != 1 {
		t.Fatalf("analyzer calls=%d want 1", runner.calls)
	}
	if res.Diagnostics == nil || res.Diagnostics.SharedChunkAnalysis.Source != "generated" {
		t.Fatalf("diagnostics=%+v", res.Diagnostics)
	}
}

func TestRun_SharedChunkFlagTrue_NilResultFailsPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	runner := &stubChunkAnalyzer{
		fn: func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			return nil, nil
		},
	}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "shared_chunk_analysis_empty_result" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if gc.calls != 0 {
		t.Fatalf("global context calls=%d", gc.calls)
	}
}

func TestRun_SharedChunkFlagTrue_CompleteResultContinuesPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", res.Status)
	}
	arts := store.sharedChunkArtifacts()
	if len(arts) != 1 {
		t.Fatal("expected artifact")
	}
	var doc struct {
		Payload struct {
			AnalysisStatus string   `json:"analysis_status"`
			Limitations    []string `json:"limitations"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(arts[0].PayloadJson, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Payload.AnalysisStatus != dashboardhome.ChunkAnalysisStatusCompleted {
		t.Fatalf("status=%s", doc.Payload.AnalysisStatus)
	}
	if len(doc.Payload.Limitations) != 0 {
		t.Fatalf("limitations=%v", doc.Payload.Limitations)
	}
}

func TestRun_SharedChunkFlagTrue_ChunkSetBuiltOnce(t *testing.T) {
	sharedChunkPipelineEnv(t)
	var buildCount int
	pack := sharedChunkTestPack(t)
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetChunkSetBuilderForTest(svc, func(p *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
		buildCount++
		return dashboardhome.BuildChunkSetForPack(p, opts)
	})

	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	if buildCount != 1 {
		t.Fatalf("chunk set build count=%d want 1", buildCount)
	}
}

func TestRun_SharedChunkFlagTrue_ValidArtifactSkipsAnalyzer(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{}
	runner := &stubChunkAnalyzer{}
	svc := newSharedChunkPipelineService(t, store, runner)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("first run analyzer calls=%d want 1", runner.calls)
	}
	if len(store.sharedChunkArtifacts()) != 1 {
		t.Fatalf("expected shared artifact saved, got %d", len(store.sharedChunkArtifacts()))
	}

	runner.calls = 0
	req := sharedChunkPipelineRequest()
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
	if second.Diagnostics.SharedChunkAnalysis.Source != "artifact" {
		t.Fatalf("source=%q", second.Diagnostics.SharedChunkAnalysis.Source)
	}
	if second.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", second.Status)
	}
}

func TestRun_SharedChunkFlagTrue_RequiredArtifactSavedWithoutDebug(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())

	req := sharedChunkPipelineRequest()
	req.DebugArtifacts = false
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(store.sharedChunkArtifacts()) != 1 {
		t.Fatalf("shared artifacts=%d want 1", len(store.sharedChunkArtifacts()))
	}
	requiredKinds := map[string]bool{
		aipipeline.ArtifactKindSharedChunkAnalysis:      true,
		aipipeline.ArtifactKindGlobalContextFinal:       true,
		aipipeline.ArtifactKindDataPack:                 true,
		aipipeline.ArtifactKindDashboardValidation:      true,
		aipipeline.ArtifactKindMetricSemanticsSnapshot:  true,
		aipipeline.ArtifactKindGlobalContextValidation:  true,
		aipipeline.ArtifactKindPublicationGuardDecision: true,
	}
	for _, art := range store.artifacts {
		if requiredKinds[art.Kind] {
			delete(requiredKinds, art.Kind)
		}
	}
	if len(requiredKinds) != 0 {
		t.Fatalf("missing required artifacts: %v", requiredKinds)
	}
	debugOnly := 0
	for _, art := range store.artifacts {
		switch art.Kind {
		case aipipeline.ArtifactKindSharedChunkRefMapping,
			aipipeline.ArtifactKindFinalOutputSemanticValidation,
			aipipeline.ArtifactKindActionsExecutionDiagnostics:
			debugOnly++
		}
	}
	if debugOnly != 0 {
		t.Fatalf("unexpected debug artifacts: %d", debugOnly)
	}
}

func TestRun_SharedChunkFlagTrue_AllChunksFailedStopsDownstream(t *testing.T) {
	sharedChunkPipelineEnv(t)
	pack, chunkSet, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	runner := &stubChunkAnalyzer{
		fn: func(ctx context.Context, p *aidatapack.SellerAIDataPack, cs *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			failedIDs := make([]string, len(cs.Chunks))
			failedTypes := make([]string, len(cs.Chunks))
			for i, ch := range cs.Chunks {
				failedIDs[i] = ch.ChunkID
				failedTypes[i] = ch.ChunkType
			}
			return &dashboardhome.ChunkAnalysisRunResult{
				SellerAccountID:  pack.SellerContext.SellerAccountID,
				MetricDate:       pack.SellerContext.MetricDate,
				ChunkCount:       len(cs.Chunks),
				Completed:        0,
				Failed:           len(cs.Chunks),
				FailedChunkIDs:   failedIDs,
				FailedChunkTypes: failedTypes,
				AnalysisStatus:   dashboardhome.ChunkAnalysisStatusFailed,
			}, nil
		},
	}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		&fakeStore{},
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "all_chunks_failed" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if res.Status != aipipeline.PipelineStatusFailed {
		t.Fatalf("status=%s", res.Status)
	}
	if gc.calls != 0 || dash.calls != 0 || act.calls != 0 {
		t.Fatalf("downstream gc=%d dash=%d act=%d", gc.calls, dash.calls, act.calls)
	}
	_ = chunkSet
}

func TestRun_SharedChunkFlagTrue_PartialResultContinuesPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	pack := sharedChunkTestPack(t)
	runner := &stubChunkAnalyzer{
		fn: func(ctx context.Context, p *aidatapack.SellerAIDataPack, cs *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			result, err := aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, p, cs, opts)
			if err != nil || result == nil || len(cs.Chunks) < 2 {
				return result, err
			}
			result.Completed = len(cs.Chunks) - 1
			result.Failed = 1
			result.FailedChunkIDs = []string{cs.Chunks[0].ChunkID}
			result.FailedChunkTypes = []string{cs.Chunks[0].ChunkType}
			result.AnalysisStatus = dashboardhome.ChunkAnalysisStatusPartial
			return result, nil
		},
	}
	store := &persistingArtifactStore{}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s error=%s", res.Status, res.ErrorCode)
	}
	if runner.calls != 1 {
		t.Fatalf("analyzer calls=%d", runner.calls)
	}
	if gc.calls != 1 {
		t.Fatalf("global context calls=%d", gc.calls)
	}
	if len(store.sharedChunkArtifacts()) != 1 {
		t.Fatal("expected shared artifact")
	}
	var doc struct {
		Payload struct {
			AnalysisStatus string   `json:"analysis_status"`
			Limitations    []string `json:"limitations"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(store.sharedChunkArtifacts()[0].PayloadJson, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Payload.AnalysisStatus != dashboardhome.ChunkAnalysisStatusPartial {
		t.Fatalf("status=%s", doc.Payload.AnalysisStatus)
	}
	if len(doc.Payload.Limitations) == 0 {
		t.Fatal("expected partial limitation")
	}
}

func TestRun_SharedChunkFlagTrue_DataPackHashMismatchReanalyzes(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{}
	runner := &stubChunkAnalyzer{}
	svc := newSharedChunkPipelineService(t, store, runner)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	runner.calls = 0

	// Tamper stored artifact hash metadata.
	arts := store.sharedChunkArtifacts()
	if len(arts) != 1 {
		t.Fatal("expected one shared artifact")
	}
	var doc map[string]any
	if err := json.Unmarshal(arts[0].PayloadJson, &doc); err != nil {
		t.Fatal(err)
	}
	payload, _ := doc["payload"].(map[string]any)
	payload["data_pack_hash"] = "tampered"
	doc["payload"] = payload
	raw, _ := json.Marshal(doc)
	for i := range store.artifacts {
		if store.artifacts[i].ID == arts[0].ID {
			store.artifacts[i].PayloadJson = raw
			break
		}
	}

	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("analyzer calls=%d want 1 on hash mismatch", runner.calls)
	}
	if second.Diagnostics.SharedChunkAnalysis.Source != "generated" {
		t.Fatalf("source=%q", second.Diagnostics.SharedChunkAnalysis.Source)
	}
	if second.Diagnostics.SharedChunkAnalysis.ArtifactMissReason != "data_pack_hash_mismatch" {
		t.Fatalf("miss reason=%q", second.Diagnostics.SharedChunkAnalysis.ArtifactMissReason)
	}
}

func TestRun_SharedChunkFlagTrue_LoadErrorStopsPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{listErr: errors.New("db unavailable")}
	runner := &stubChunkAnalyzer{}
	svc := newSharedChunkPipelineService(t, store, runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "shared_chunk_analysis_load_failed" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if runner.calls != 0 {
		t.Fatalf("analyzer calls=%d want 0", runner.calls)
	}
}

func TestRun_SharedChunkFlagTrue_PersistErrorStopsPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{createErr: errors.New("disk full")}
	runner := &stubChunkAnalyzer{}
	svc := newSharedChunkPipelineService(t, store, runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "artifact_save_failed" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
}

func TestRun_SharedChunkFlagFalse_SkipsSharedStage(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	var buildCount int
	pack := sharedChunkTestPack(t)
	runner := &stubChunkAnalyzer{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&persistingArtifactStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)
	aipipeline.SetChunkSetBuilderForTest(svc, func(p *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
		buildCount++
		return dashboardhome.BuildChunkSetForPack(p, opts)
	})

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", res.Status)
	}
	if buildCount != 0 || runner.calls != 0 {
		t.Fatalf("build=%d runner=%d", buildCount, runner.calls)
	}
}

type publishedReuseStore struct {
	fakeStore
	published dbgen.AiPipelineRun
}

func (s *publishedReuseStore) GetPublishedAIPipelineRunBySellerDateAndDataPackHash(
	ctx context.Context,
	arg dbgen.GetPublishedAIPipelineRunBySellerDateAndDataPackHashParams,
) (dbgen.AiPipelineRun, error) {
	return s.published, nil
}

func TestRun_SharedChunkFlagTrue_PublishedReuseSkipsSharedStage(t *testing.T) {
	sharedChunkPipelineEnv(t)
	var buildCount int
	runner := &stubChunkAnalyzer{}
	store := &publishedReuseStore{
		published: dbgen.AiPipelineRun{
			ID:     77,
			Status: aipipeline.PipelineStatusCompleted,
		},
	}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)
	aipipeline.SetChunkSetBuilderForTest(svc, func(p *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
		buildCount++
		return dashboardhome.BuildChunkSetForPack(p, opts)
	})

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Reused || res.PipelineRunID != 77 {
		t.Fatalf("reused=%v run_id=%d", res.Reused, res.PipelineRunID)
	}
	if buildCount != 0 || runner.calls != 0 {
		t.Fatalf("build=%d runner=%d", buildCount, runner.calls)
	}
}

type eventPersistingStore struct {
	persistingArtifactStore
	rec *stageEventRecorder
}

func (s *eventPersistingStore) CreateArtifact(ctx context.Context, params aipipeline.CreateArtifactParams) (int64, error) {
	if params.Kind == aipipeline.ArtifactKindSharedChunkAnalysis {
		s.rec.events = append(s.rec.events, "shared_artifact_save")
	}
	return s.persistingArtifactStore.CreateArtifact(ctx, params)
}

type stageEventRecorder struct {
	events []string
}

type recordingDashboardRunner struct {
	rec      *stageEventRecorder
	inner    *fakeDashboardRunner
	lastOpts dashboardhome.NarrativeRunOptions
}

func (r *recordingDashboardRunner) RunForSellerMetricDateWithOptions(ctx context.Context, sellerAccountID int64, metricDate time.Time, opts dashboardhome.NarrativeRunOptions) (*aipipeline.DashboardRunOutcome, error) {
	r.rec.events = append(r.rec.events, "dashboard")
	r.lastOpts = opts
	if r.inner == nil {
		r.inner = &fakeDashboardRunner{}
	}
	return r.inner.RunForSellerMetricDateWithOptions(ctx, sellerAccountID, metricDate, opts)
}

type recordingActionsRunner struct {
	rec   *stageEventRecorder
	inner *fakeActionsRunner
}

func (r *recordingActionsRunner) BuildActionsAIFirstInputFromDataPack(pack *aidatapack.SellerAIDataPack, dataPackHash string, globalContext *aicontext.GlobalSemanticContext, opts actions.GenerateActionsOptions) (actions.ActionsAIFirstInput, error) {
	if r.inner == nil {
		r.inner = &fakeActionsRunner{}
	}
	return r.inner.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalContext, opts)
}

func (r *recordingActionsRunner) BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []actions.AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	if r.inner == nil {
		r.inner = &fakeActionsRunner{}
	}
	return r.inner.BuildActionsAIFirstInputFromCandidates(
		pack, dataPackHash, globalContext, globalContextHash, candidates, failedChunkIDs, dataLimitations, opts,
	)
}

func (r *recordingActionsRunner) GenerateActionsAIFirstFromInput(ctx context.Context, input actions.ActionsAIFirstInput, opts actions.GenerateActionsOptions) (*actions.GenerateActionsResult, error) {
	r.rec.events = append(r.rec.events, "actions")
	if r.inner == nil {
		r.inner = &fakeActionsRunner{}
	}
	return r.inner.GenerateActionsAIFirstFromInput(ctx, input, opts)
}

func TestRun_SharedChunkFlagTrue_StageOrder(t *testing.T) {
	sharedChunkPipelineEnv(t)
	rec := &stageEventRecorder{}
	pack := sharedChunkTestPack(t)
	gcInner := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &eventPersistingStore{rec: rec}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		&recordingDashboardRunner{rec: rec},
		&recordingActionsRunner{rec: rec},
		store,
		&stageGlobalContextWrapper{inner: gcInner, rec: rec},
	)
	svc.SetSnapshotPublisher(stagePublisher{rec: rec})
	svc.SetChunkAnalysisRunner(&stageChunkAnalyzer{rec: rec, inner: aipipeline.NewDeterministicChunkAnalysisRunner()})
	aipipeline.SetChunkSetBuilderForTest(svc, func(p *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
		rec.events = append(rec.events, "shared_chunk_set_build")
		return dashboardhome.BuildChunkSetForPack(p, opts)
	})

	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	idx := func(name string) int {
		for i, ev := range rec.events {
			if ev == name {
				return i
			}
		}
		return -1
	}
	chunkBuild := idx("shared_chunk_set_build")
	chunkAnalysis := idx("shared_chunk_analysis")
	artifactSave := idx("shared_artifact_save")
	globalContext := idx("global_context")
	dashboard := idx("dashboard")
	actions := idx("actions")
	if chunkBuild < 0 || chunkAnalysis < 0 || artifactSave < 0 || globalContext < 0 || dashboard < 0 || actions < 0 {
		t.Fatalf("missing stage events: %v", rec.events)
	}
	if !(chunkBuild < chunkAnalysis && chunkAnalysis < artifactSave && artifactSave < globalContext && globalContext < dashboard && dashboard < actions) {
		t.Fatalf("invalid order: %v", rec.events)
	}
}

func TestRun_SharedChunkFlagTrue_DashboardReceivesPrecomputedOverrides(t *testing.T) {
	sharedChunkPipelineEnv(t)
	rec := &stageEventRecorder{}
	pack := sharedChunkTestPack(t)
	dash := &recordingDashboardRunner{rec: rec}
	store := &persistingArtifactStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		&recordingActionsRunner{rec: rec},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(stagePublisher{rec: rec})
	svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())

	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	if dash.lastOpts.ChunkSetOverride == nil {
		t.Fatal("expected chunk set override")
	}
	if dash.lastOpts.PrecomputedChunkRun == nil {
		t.Fatal("expected precomputed chunk run")
	}
	if dash.lastOpts.DataPackOverride == nil {
		t.Fatal("expected data pack override")
	}
}

type stageChunkAnalyzer struct {
	rec   *stageEventRecorder
	inner aipipeline.ChunkAnalysisRunner
}

func (s *stageChunkAnalyzer) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts dashboardhome.NarrativeRunOptions,
) (*dashboardhome.ChunkAnalysisRunResult, error) {
	s.rec.events = append(s.rec.events, "shared_chunk_analysis")
	return s.inner.AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
}

type stagePublisher struct {
	rec *stageEventRecorder
}

func (p stagePublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	p.rec.events = append(p.rec.events, "publication")
	return aipipeline.PublishResult{}, nil
}

type stageGlobalContextWrapper struct {
	inner *countingGlobalContextClient
	rec   *stageEventRecorder
}

func (w *stageGlobalContextWrapper) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	w.rec.events = append(w.rec.events, "global_context")
	return w.inner.GenerateGlobalContext(ctx, input)
}

func (w *stageGlobalContextWrapper) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	w.rec.events = append(w.rec.events, "global_context")
	return w.inner.RetryGlobalContext(ctx, input)
}
