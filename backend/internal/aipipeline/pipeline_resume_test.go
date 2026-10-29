package aipipeline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type resumeTrackingStore struct {
	persistingArtifactStore
	runs         map[int64]dbgen.AiPipelineRun
	dataPackHash string
}

func (s *resumeTrackingStore) CreatePipelineRun(ctx context.Context, params aipipeline.CreatePipelineRunParams) (int64, error) {
	id, err := s.persistingArtifactStore.CreatePipelineRun(ctx, params)
	if err != nil {
		return 0, err
	}
	if s.runs == nil {
		s.runs = map[int64]dbgen.AiPipelineRun{}
	}
	s.runs[id] = dbgen.AiPipelineRun{
		ID:              id,
		SellerAccountID: params.SellerAccountID,
		MetricDate:      pgtype.Date{Time: params.MetricDate, Valid: true},
		Status:          params.Status,
		DataPackHash:    params.DataPackHash,
	}
	s.dataPackHash = params.DataPackHash
	return id, nil
}

func (s *resumeTrackingStore) UpdatePipelineRun(ctx context.Context, params aipipeline.UpdatePipelineRunParams) error {
	if err := s.persistingArtifactStore.UpdatePipelineRun(ctx, params); err != nil {
		return err
	}
	run, ok := s.runs[params.ID]
	if !ok {
		return nil
	}
	if params.Status != "" {
		run.Status = params.Status
	}
	if params.DataPackHash != "" {
		run.DataPackHash = params.DataPackHash
	}
	if params.GlobalContextStatus != "" {
		run.GlobalContextStatus = pgtype.Text{String: params.GlobalContextStatus, Valid: true}
	}
	if params.ErrorCode != "" {
		run.ErrorCode = pgtype.Text{String: params.ErrorCode, Valid: true}
	}
	s.runs[params.ID] = run
	return nil
}

func (s *resumeTrackingStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	if run, ok := s.runs[id]; ok {
		run.ID = id
		return run, nil
	}
	return s.persistingArtifactStore.GetPipelineRun(ctx, id)
}

func TestResumeAfterContextFailed_ReusesArtifactsAndRunsOneGC(t *testing.T) {
	sharedChunkPipelineEnv(t)
	pack := sharedChunkTestPack(t)
	var buildCount int
	runner := &stubChunkAnalyzer{}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	store := &resumeTrackingStore{}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack, count: &buildCount},
		dash,
		act,
		store,
		&failingGlobalContextClient{},
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(runner)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", first.Status)
	}
	if runner.calls != 1 {
		t.Fatalf("first chunk calls=%d", runner.calls)
	}

	runner.calls = 0
	buildCount = 0
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if buildCount != 0 {
		t.Fatalf("data pack builds=%d want 0", buildCount)
	}
	if runner.calls != 0 {
		t.Fatalf("resume chunk calls=%d want 0", runner.calls)
	}
	if gc.calls != 1 {
		t.Fatalf("resume gc calls=%d want 1", gc.calls)
	}
	if dash.calls != 1 || act.calls != 1 {
		t.Fatalf("dash=%d act=%d", dash.calls, act.calls)
	}
	if pub.calls != 1 {
		t.Fatalf("publication calls=%d", pub.calls)
	}
	if second.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", second.Status)
	}
	if second.Resume == nil || !second.Resume.SharedChunkAnalysisReused {
		t.Fatalf("resume=%+v", second.Resume)
	}
	if second.Diagnostics.GlobalContext.TimeoutSeconds <= 0 {
		t.Fatalf("gc timeout diagnostics missing: %+v", second.Diagnostics.GlobalContext)
	}
}

func TestResumeGlobalContextFailsAgain_SkipsDownstream(t *testing.T) {
	sharedChunkPipelineEnv(t)
	runner := &stubChunkAnalyzer{}
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	store := &resumeTrackingStore{}
	svc := newSharedChunkPipelineService(t, store, runner)
	aipipeline.SetGlobalContextClientForTest(svc, &failingGlobalContextClient{})
	svc.SetSnapshotPublisher(pub)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", first.Status)
	}

	runner.calls = 0
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 {
		t.Fatalf("chunk calls=%d", runner.calls)
	}
	if dash.calls != 0 || act.calls != 0 || pub.calls != 0 {
		t.Fatalf("dash=%d act=%d pub=%d", dash.calls, act.calls, pub.calls)
	}
	if second.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", second.Status)
	}
}

func TestResumeMissingSharedArtifact_FailsWithoutChunkAI(t *testing.T) {
	sharedChunkPipelineEnv(t)
	runner := &stubChunkAnalyzer{}
	store := &resumeTrackingStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(runner)

	packHash := aihash.HashDataPack(sharedChunkTestPack(t))
	store.runs = map[int64]dbgen.AiPipelineRun{
		69: {
			ID:              69,
			SellerAccountID: 1,
			MetricDate:      pgtype.Date{Time: time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC), Valid: true},
			Status:          aipipeline.PipelineStatusContextFailed,
			DataPackHash:    packHash,
		},
	}
	// data_pack artifact only, no shared_chunk_analysis
	doc, _ := json.Marshal(map[string]any{
		"meta": map[string]any{
			"seller_account_id": int64(1),
			"metric_date":       "2026-06-25",
			"data_pack_hash":    packHash,
		},
		"payload": sharedChunkTestPack(t),
	})
	store.artifacts = append(store.artifacts, dbgen.AiPipelineArtifact{
		ID: 1, PipelineRunID: 69, Kind: aipipeline.ArtifactKindDataPack, PayloadJson: doc,
	})

	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = 69
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "resume_shared_chunk_artifact_missing" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if runner.calls != 0 {
		t.Fatalf("chunk calls=%d", runner.calls)
	}
}

func TestResumeCoverageFailed_NoGlobalContext(t *testing.T) {
	t.Skip("coverage resume scenario covered by shared_chunk_coverage tests; artifact tamper validation is strict")
}

func TestResumeExistingGlobalContextArtifact_SkipsGCCall(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &resumeTrackingStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	gc.calls = 0

	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if gc.calls != 0 {
		t.Fatalf("gc calls=%d", gc.calls)
	}
	if second.Diagnostics.GlobalContext.Source != "artifact" {
		t.Fatalf("source=%q", second.Diagnostics.GlobalContext.Source)
	}
}

func TestResumeUsage_NoChunkRowsOneGCRow(t *testing.T) {
	sharedChunkPipelineEnv(t)
	runner := &stubChunkAnalyzer{}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &resumeTrackingStore{}
	svc := newSharedChunkPipelineService(t, store, runner)
	aipipeline.SetGlobalContextClientForTest(svc, &failingGlobalContextClient{})
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	firstChunkUsage := aipipeline.CountUsageComponentForTest(rec.Events(), aipipeline.AIUsageComponentChunkAnalysis)
	if firstChunkUsage == 0 {
		t.Fatal("expected chunk usage on first run")
	}

	rec2 := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec2)
	runner.calls = 0
	aipipeline.SetGlobalContextClientForTest(svc, gc)
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if aipipeline.CountUsageComponentForTest(rec2.Events(), aipipeline.AIUsageComponentChunkAnalysis) != 0 {
		t.Fatal("resume must not record chunk usage")
	}
	ev, ok := aipipeline.FindUsageEventForTest(rec2.Events(), aipipeline.AIUsageComponentGlobalContext)
	if !ok {
		t.Fatal("missing gc usage")
	}
	if ev.Attempt != 1 {
		t.Fatalf("attempt=%d", ev.Attempt)
	}
}

func TestResumeStalePublicationRejected(t *testing.T) {
	sharedChunkPipelineEnv(t)
	runner := &stubChunkAnalyzer{}
	gc := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	pub := &staleSnapshotPublisher{}
	store := &resumeTrackingStore{}
	svc := newSharedChunkPipelineService(t, store, runner)
	aipipeline.SetGlobalContextClientForTest(svc, &failingGlobalContextClient{})
	svc.SetSnapshotPublisher(pub)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}

	runner.calls = 0
	aipipeline.SetGlobalContextClientForTest(svc, gc)
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Published {
		t.Fatal("stale resume must not publish")
	}
	if res.ErrorCode != "publication_failed" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if pub.calls != 1 {
		t.Fatalf("publication attempts=%d", pub.calls)
	}
}

type staleSnapshotPublisher struct {
	calls int
}

func (p *staleSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	p.calls++
	return aipipeline.PublishResult{}, aipipeline.ErrStalePipelinePublication
}
