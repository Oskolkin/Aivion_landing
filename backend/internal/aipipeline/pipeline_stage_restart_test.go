package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type stageRestartStore struct {
	persistingArtifactStore
	runs              map[int64]dbgen.AiPipelineRun
	narratives        map[string]dbgen.AiDashboardNarrative
	snapshots         map[int64]dbgen.AiDashboardDataSnapshot
	snapshotByRun     map[int64]int64
	nextRunID         int64
	nextSnapshotID    int64
	cloneNarrativeErr error
	sourceSnapshot    map[string][]byte
}

func newStageRestartStore() *stageRestartStore {
	return &stageRestartStore{
		runs:           map[int64]dbgen.AiPipelineRun{},
		narratives:     map[string]dbgen.AiDashboardNarrative{},
		snapshots:      map[int64]dbgen.AiDashboardDataSnapshot{},
		snapshotByRun:  map[int64]int64{},
		sourceSnapshot: map[string][]byte{},
	}
}

func (s *stageRestartStore) CreatePipelineRun(ctx context.Context, params aipipeline.CreatePipelineRunParams) (int64, error) {
	s.nextRunID++
	id := s.nextRunID
	if id == 0 {
		id = 1
	}
	s.runs[id] = dbgen.AiPipelineRun{
		ID:              id,
		SellerAccountID: params.SellerAccountID,
		MetricDate:      pgtype.Date{Time: params.MetricDate, Valid: true},
		Status:          params.Status,
		DataPackHash:    params.DataPackHash,
		CreatedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	return id, nil
}

func (s *stageRestartStore) UpdatePipelineRun(ctx context.Context, params aipipeline.UpdatePipelineRunParams) error {
	run, ok := s.runs[params.ID]
	if !ok {
		return fmt.Errorf("pipeline run %d not found", params.ID)
	}
	if params.Status != "" {
		run.Status = params.Status
	}
	if params.GlobalContextHash != "" {
		run.GlobalContextHash = pgtype.Text{String: params.GlobalContextHash, Valid: true}
	}
	if params.GlobalContextStatus != "" {
		run.GlobalContextStatus = pgtype.Text{String: params.GlobalContextStatus, Valid: true}
	}
	if params.DashboardRunID != "" {
		var id pgtype.UUID
		_ = id.Scan(params.DashboardRunID)
		run.DashboardRunID = id
	}
	if params.ActionsRunID > 0 {
		run.ActionsRunID = pgtype.Int8{Int64: params.ActionsRunID, Valid: true}
	}
	if params.DashboardStatus != "" {
		run.DashboardStatus = pgtype.Text{String: params.DashboardStatus, Valid: true}
	}
	if params.ActionsStatus != "" {
		run.ActionsStatus = pgtype.Text{String: params.ActionsStatus, Valid: true}
	}
	if params.ErrorCode != "" {
		run.ErrorCode = pgtype.Text{String: params.ErrorCode, Valid: true}
	}
	if params.PublishedAt != nil {
		run.PublishedAt = pgtype.Timestamptz{Time: params.PublishedAt.UTC(), Valid: true}
	}
	s.runs[params.ID] = run
	return nil
}

func (s *stageRestartStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	run, ok := s.runs[id]
	if !ok {
		return dbgen.AiPipelineRun{}, fmt.Errorf("pipeline run %d not found", id)
	}
	return run, nil
}

func (s *stageRestartStore) GetDashboardDataSnapshotByPipelineRunID(ctx context.Context, pipelineRunID int64) (dbgen.AiDashboardDataSnapshot, error) {
	snapID, ok := s.snapshotByRun[pipelineRunID]
	if !ok {
		return dbgen.AiDashboardDataSnapshot{}, fmt.Errorf("snapshot not found")
	}
	snap, ok := s.snapshots[snapID]
	if !ok {
		return dbgen.AiDashboardDataSnapshot{}, fmt.Errorf("snapshot not found")
	}
	return snap, nil
}

func (s *stageRestartStore) CloneDashboardNarrativeForPipeline(
	ctx context.Context,
	sourceDashboardRunID string,
	sourcePipelineRunID int64,
	targetPipelineRunID int64,
	sellerID int64,
	dataPackHash string,
	globalContextHash string,
) (string, error) {
	if s.cloneNarrativeErr != nil {
		return "", s.cloneNarrativeErr
	}
	source, ok := s.narratives[sourceDashboardRunID]
	if !ok {
		return "", fmt.Errorf("source narrative not found")
	}
	newID := uuid.New()
	newKey := newID.String()
	clone := source
	clone.ID = pgtype.UUID{Bytes: newID, Valid: true}
	clone.AiPipelineRunID = pgtype.Int8{Int64: targetPipelineRunID, Valid: true}
	clone.StartedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	clone.FinishedAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	s.narratives[newKey] = clone
	s.sourceSnapshot[sourceDashboardRunID] = append([]byte(nil), source.ValidatedPayload...)
	return newKey, nil
}

func (s *stageRestartStore) CloneDashboardDataSnapshotForPipeline(
	ctx context.Context,
	sourceSnapshotID int64,
	sourcePipelineRunID int64,
	targetPipelineRunID int64,
	sellerID int64,
) (int64, error) {
	if s.cloneNarrativeErr != nil {
		return 0, s.cloneNarrativeErr
	}
	source, ok := s.snapshots[sourceSnapshotID]
	if !ok {
		return 0, fmt.Errorf("source snapshot not found")
	}
	s.nextSnapshotID++
	id := s.nextSnapshotID
	if id == 0 {
		id = 100
	}
	clone := source
	clone.ID = id
	clone.AiPipelineRunID = targetPipelineRunID
	s.snapshots[id] = clone
	s.snapshotByRun[targetPipelineRunID] = id
	run := s.runs[targetPipelineRunID]
	run.DashboardDataSnapshotID = pgtype.Int8{Int64: id, Valid: true}
	s.runs[targetPipelineRunID] = run
	return id, nil
}

func seedPublishedSourceRun(t *testing.T, store *stageRestartStore, sourceID int64) {
	t.Helper()
	sharedChunkPipelineEnv(t)
	pack := sharedChunkTestPack(t)
	packHash := aihash.HashDataPack(pack)
	metricDate, err := time.Parse("2006-01-02", pack.SellerContext.MetricDate)
	if err != nil {
		t.Fatal(err)
	}
	dashID := uuid.New()
	dashKey := dashID.String()
	store.nextRunID = sourceID // next CreatePipelineRun returns sourceID+1
	run := dbgen.AiPipelineRun{
		ID:                      sourceID,
		SellerAccountID:         1,
		MetricDate:              pgtype.Date{Time: metricDate, Valid: true},
		Status:                  aipipeline.PipelineStatusCompleted,
		DataPackHash:            packHash,
		GlobalContextStatus:     pgtype.Text{String: aipipeline.GlobalContextStatusCompleted, Valid: true},
		DashboardRunID:          pgtype.UUID{Bytes: dashID, Valid: true},
		DashboardDataSnapshotID: pgtype.Int8{Int64: 501, Valid: true},
		ActionsRunID:            pgtype.Int8{Int64: 900, Valid: true},
		DashboardStatus:         pgtype.Text{String: aipipeline.StepStatusCompleted, Valid: true},
		ActionsStatus:           pgtype.Text{String: actions.GenerationFinalStatusCompletedPartial, Valid: true},
		PublishedAt:             pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
		CreatedAt:               pgtype.Timestamptz{Time: time.Now().UTC().Add(-2 * time.Hour), Valid: true},
	}
	narrative := dbgen.AiDashboardNarrative{
		ID:               pgtype.UUID{Bytes: dashID, Valid: true},
		SellerAccountID:  1,
		MetricDate:       pgtype.Date{Time: metricDate, Valid: true},
		Status:           aipipeline.StepStatusCompleted,
		AiPipelineRunID:  pgtype.Int8{Int64: sourceID, Valid: true},
		DataPackHash:     pgtype.Text{String: packHash, Valid: true},
		ValidatedPayload: []byte(`{"headline":"Source","top_actions":[{"title":"Old Top"}]}`),
	}
	snapshot := dbgen.AiDashboardDataSnapshot{
		ID:              501,
		AiPipelineRunID: sourceID,
		SellerAccountID: 1,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		DataPackHash:    packHash,
		PayloadHash:     "snap-hash",
		Status:          "completed",
		PayloadJson:     []byte(`{"metric_date":"` + pack.SellerContext.MetricDate + `"}`),
	}
	store.snapshotByRun[sourceID] = 501

	// data pack artifact
	dpDoc, _ := json.Marshal(map[string]any{
		"meta": map[string]any{
			"seller_account_id": pack.SellerContext.SellerAccountID,
			"metric_date":       pack.SellerContext.MetricDate,
			"data_pack_hash":    packHash,
		},
		"payload": pack,
	})
	store.artifacts = append(store.artifacts, dbgen.AiPipelineArtifact{
		ID: 1, PipelineRunID: sourceID, Kind: aipipeline.ArtifactKindDataPack, PayloadJson: dpDoc,
	})

	chunkSet, _ := dashboardhome.BuildChunkSetForPack(pack, aidatapack.DefaultChunkOptions())
	chunkRun, _ := aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(context.Background(), pack, chunkSet, dashboardhome.NarrativeRunOptions{})
	chunkHashes, _ := aipipeline.OrderedChunkHashesForTest(chunkSet)
	sharedArtifact, _ := aipipeline.BuildSharedChunkAnalysisArtifactForTest(pack, packHash, chunkSet, chunkHashes, chunkRun, nil)
	sharedDoc, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": sharedArtifact})
	store.artifacts = append(store.artifacts, dbgen.AiPipelineArtifact{
		ID: 2, PipelineRunID: sourceID, Kind: aipipeline.ArtifactKindSharedChunkAnalysis, PayloadJson: sharedDoc,
	})

	gc := aipipeline.NewDeterministicGlobalContextFakeClient()
	gcInput, err := aipipeline.BuildGlobalContextGenerationInputForTest(pack, packHash, chunkSet, chunkRun)
	if err != nil {
		t.Fatal(err)
	}
	inputHash, err := aipipeline.HashGlobalContextGenerationInput(gcInput)
	if err != nil {
		t.Fatal(err)
	}
	ctxObj, _, err := gc.GenerateGlobalContext(context.Background(), gcInput)
	if err != nil {
		t.Fatal(err)
	}
	gcHash := aihash.HashGlobalSemanticContext(ctxObj)
	gcArtifact, err := aipipeline.BuildGlobalContextArtifactForTest(gcInput, ctxObj, inputHash, gcHash)
	if err != nil {
		t.Fatal(err)
	}
	run.GlobalContextHash = pgtype.Text{String: gcHash, Valid: true}
	narrative.GlobalContextHash = pgtype.Text{String: gcHash, Valid: true}
	snapshot.GlobalContextHash = gcHash
	store.runs[sourceID] = run
	store.narratives[dashKey] = narrative
	store.snapshots[501] = snapshot
	store.snapshotByRun[sourceID] = 501
	gcDoc, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": gcArtifact})
	store.artifacts = append(store.artifacts, dbgen.AiPipelineArtifact{
		ID: 3, PipelineRunID: sourceID, Kind: aipipeline.ArtifactKindGlobalContextFinal, PayloadJson: gcDoc,
	})
}

func stageRestartRequest(t *testing.T, sourceID int64) aipipeline.RunSellerAIPipelineRequest {
	t.Helper()
	pack := sharedChunkTestPack(t)
	metricDate, err := time.Parse("2006-01-02", pack.SellerContext.MetricDate)
	if err != nil {
		t.Fatal(err)
	}
	return aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:       pack.SellerContext.SellerAccountID,
		MetricDate:            metricDate,
		GenerateDashboard:     true,
		GenerateActions:       true,
		AIEnabled:             true,
		SourcePipelineRunID:   sourceID,
		RestartFromStage:      aipipeline.ResumeStageActions,
		StageRestartRequested: true,
		Trigger:               "devtool_stage_restart",
	}
}

func TestStageRestart_SuccessFromPublishedSource(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := newStageRestartStore()
	const sourceID int64 = 70
	seedPublishedSourceRun(t, store, sourceID)
	sourceBefore := store.runs[sourceID]

	chunkCalls := 0
	runner := &stubChunkAnalyzer{fn: func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
		chunkCalls++
		return aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
	}}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	act.calls = 0
	pub := &recordingSnapshotPublisher{}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		dash,
		act,
		store,
		gc,
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(runner)
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	req := stageRestartRequest(t, sourceID)
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.PipelineRunID == sourceID || res.PipelineRunID <= 0 {
		t.Fatalf("pipeline_run_id=%d source=%d", res.PipelineRunID, sourceID)
	}
	if !res.Published || pub.calls != 1 {
		t.Fatalf("published=%v pub_calls=%d", res.Published, pub.calls)
	}
	if chunkCalls != 0 || gc.calls != 0 || dash.calls != 0 || act.calls != 1 {
		t.Fatalf("calls chunk=%d gc=%d dash=%d act=%d", chunkCalls, gc.calls, dash.calls, act.calls)
	}
	sourceAfter := store.runs[sourceID]
	if sourceAfter.PublishedAt != sourceBefore.PublishedAt ||
		sourceAfter.DashboardRunID != sourceBefore.DashboardRunID ||
		sourceAfter.ActionsRunID != sourceBefore.ActionsRunID {
		t.Fatalf("source run mutated: before=%+v after=%+v", sourceBefore, sourceAfter)
	}
	if res.Restart == nil || !res.Restart.Requested || res.Restart.ActionsAICalls != 1 || res.Restart.SharedChunkAICalls != 0 {
		t.Fatalf("restart=%+v", res.Restart)
	}
	if res.ActionsRunID == 900 {
		t.Fatalf("reused source actions run id=%d", res.ActionsRunID)
	}
}

func TestStageRestart_ActionsFailureDoesNotPublish(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := newStageRestartStore()
	seedPublishedSourceRun(t, store, 70)
	act := &stageRestartFailingActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		act,
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())

	res, err := svc.Run(context.Background(), stageRestartRequest(t, 70))
	if err != nil {
		t.Fatal(err)
	}
	if res.Published || pub.calls != 0 || act.calls != 1 {
		t.Fatalf("published=%v pub=%d act=%d", res.Published, pub.calls, act.calls)
	}
}

func TestStageRestart_DashboardCloneFailure(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := newStageRestartStore()
	seedPublishedSourceRun(t, store, 70)
	store.cloneNarrativeErr = errors.New("clone failed")
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		act,
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())

	res, err := svc.Run(context.Background(), stageRestartRequest(t, 70))
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "stage_restart_dashboard_clone_failed" || act.calls != 0 || pub.calls != 0 {
		t.Fatalf("code=%q act=%d pub=%d", res.ErrorCode, act.calls, pub.calls)
	}
}

func TestStageRestart_MissingSharedArtifactPreflight(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := newStageRestartStore()
	seedPublishedSourceRun(t, store, 70)
	arts := append([]dbgen.AiPipelineArtifact(nil), store.artifacts...)
	store.artifacts = nil
	for _, art := range arts {
		if art.Kind == aipipeline.ArtifactKindDataPack {
			store.artifacts = append(store.artifacts, art)
		}
	}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		act,
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(pub)

	res, err := svc.Run(context.Background(), stageRestartRequest(t, 70))
	if err != nil {
		t.Fatal(err)
	}
	if res.PipelineRunID != 0 || res.ErrorCode != "stage_restart_shared_analysis_artifact_missing" || act.calls != 0 {
		t.Fatalf("run_id=%d code=%q act=%d", res.PipelineRunID, res.ErrorCode, act.calls)
	}
}

func TestStageRestart_ExplicitActionsRegeneration(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := newStageRestartStore()
	seedPublishedSourceRun(t, store, 70)
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		act,
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())

	res, err := svc.Run(context.Background(), stageRestartRequest(t, 70))
	if err != nil {
		t.Fatal(err)
	}
	if res.Restart == nil || res.Restart.ActionsReused || res.Restart.ActionsAICalls != 1 {
		t.Fatalf("restart=%+v", res.Restart)
	}
	if res.ActionsRunID == 900 {
		t.Fatal("reused source actions run")
	}
}

type stageRestartFailingActionsRunner struct {
	calls int
}

func (r *stageRestartFailingActionsRunner) BuildActionsAIFirstInputFromDataPack(pack *aidatapack.SellerAIDataPack, dataPackHash string, globalContext *aicontext.GlobalSemanticContext, opts actions.GenerateActionsOptions) (actions.ActionsAIFirstInput, error) {
	return actions.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalContext, opts)
}

func (r *stageRestartFailingActionsRunner) BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []actions.AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	return actions.BuildActionsAIFirstInputFromCandidates(pack, dataPackHash, globalContext, globalContextHash, candidates, failedChunkIDs, dataLimitations, opts)
}

func (r *stageRestartFailingActionsRunner) GenerateActionsAIFirstFromInput(ctx context.Context, input actions.ActionsAIFirstInput, opts actions.GenerateActionsOptions) (*actions.GenerateActionsResult, error) {
	r.calls++
	return nil, errors.New("actions provider failed")
}
