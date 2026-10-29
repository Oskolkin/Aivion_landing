package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type countingRetryGlobalContextClient struct {
	inner         aipipeline.GlobalContextAIClient
	generateCalls int
	retryCalls    int
}

func (c *countingRetryGlobalContextClient) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	c.generateCalls++
	return c.inner.GenerateGlobalContext(ctx, input)
}

func (c *countingRetryGlobalContextClient) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	c.retryCalls++
	return c.inner.RetryGlobalContext(ctx, input)
}

func TestGlobalContextGenerationNoRetryOnValidResponse(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	gc := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	if gc.generateCalls != 1 || gc.retryCalls != 0 {
		t.Fatalf("generate=%d retry=%d", gc.generateCalls, gc.retryCalls)
	}
}

func TestGlobalContextGenerationRetriesOnValidationFailure(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	gc := &countingRetryGlobalContextClient{inner: &invalidThenValidGlobalContextClient{}}
	store := &persistingArtifactStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", res.Status)
	}
	if gc.generateCalls != 1 || gc.retryCalls != 1 {
		t.Fatalf("generate=%d retry=%d", gc.generateCalls, gc.retryCalls)
	}
}

type invalidThenValidGlobalContextClient struct{}

func (invalidThenValidGlobalContextClient) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	ctxOut := validInvalidGlobalContextDraft(input)
	return &ctxOut, &aipipeline.GlobalContextRawResponse{Content: "invalid-draft"}, nil
}

func (invalidThenValidGlobalContextClient) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	return aipipeline.NewDeterministicGlobalContextFakeClient().GenerateGlobalContext(ctx, input.Input)
}

func validInvalidGlobalContextDraft(input aipipeline.GlobalContextGenerationInput) aicontext.GlobalSemanticContext {
	ctx, _, _ := aipipeline.NewDeterministicGlobalContextFakeClient().GenerateGlobalContext(context.Background(), input)
	ctx.OverallStory = "Маржинальность требует внимания."
	return *ctx
}

func TestGlobalContextGenerationNoRetryOnInvalidJSON(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	gc := &countingRetryGlobalContextClient{inner: &invalidJSONGlobalContextClient{}}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", res.Status)
	}
	if gc.generateCalls != 1 || gc.retryCalls != 0 {
		t.Fatalf("generate=%d retry=%d", gc.generateCalls, gc.retryCalls)
	}
}

func TestGlobalContextInvalidJSONPersistsDiagnosticsArtifacts(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	store := &persistingArtifactStore{}
	dashboard := &fakeDashboardRunner{}
	actions := &fakeActionsRunner{}
	gc := &countingRetryGlobalContextClient{inner: &invalidJSONGlobalContextClient{}}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		dashboard,
		actions,
		store,
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	req := basePipelineRequest()
	req.DebugArtifacts = true
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", res.Status)
	}
	if dashboard.calls != 0 || actions.calls != 0 {
		t.Fatalf("dashboard calls=%d actions calls=%d want 0/0", dashboard.calls, actions.calls)
	}

	loaded, err := store.ListArtifacts(context.Background(), res.PipelineRunID)
	if err != nil {
		t.Fatal(err)
	}
	hasCallDiag, hasRaw := false, false
	var rawBody string
	for _, art := range loaded {
		switch art.Kind {
		case aipipeline.ArtifactKindGlobalContextCallDiagnostics:
			hasCallDiag = true
			var doc aipipeline.ArtifactDocument
			if err := json.Unmarshal(art.PayloadJson, &doc); err != nil {
				t.Fatal(err)
			}
			env, _ := doc.Payload.(map[string]any)
			if env["parse_status"] != aipipeline.ParseStatusInvalidJSON {
				t.Fatalf("parse_status=%v", env["parse_status"])
			}
			if env["raw_response_captured"] != true {
				t.Fatalf("raw_response_captured=%v", env["raw_response_captured"])
			}
		case aipipeline.ArtifactKindGlobalContextRawResponse:
			hasRaw = true
			var doc aipipeline.ArtifactDocument
			if err := json.Unmarshal(art.PayloadJson, &doc); err != nil {
				t.Fatal(err)
			}
			payload, _ := doc.Payload.(map[string]any)
			rawBody, _ = payload["body"].(string)
		case aipipeline.ArtifactKindGlobalContextParsedOutput:
			t.Fatal("parsed output must not exist on invalid json")
		}
	}
	if !hasCallDiag || !hasRaw {
		t.Fatalf("hasCallDiag=%v hasRaw=%v artifacts=%d", hasCallDiag, hasRaw, len(loaded))
	}
	if rawBody != "not-json" {
		t.Fatalf("raw body=%q", rawBody)
	}
}

func TestGlobalContextInvalidJSONPersistsDiagnosticsWithoutRawBody(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	store := &persistingArtifactStore{}
	gc := &countingRetryGlobalContextClient{inner: &invalidJSONGlobalContextClient{}}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	req := basePipelineRequest()
	req.DebugArtifacts = false
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("status=%s", res.Status)
	}
	loaded, err := store.ListArtifacts(context.Background(), res.PipelineRunID)
	if err != nil {
		t.Fatal(err)
	}
	hasCallDiag, hasRaw := false, false
	for _, art := range loaded {
		switch art.Kind {
		case aipipeline.ArtifactKindGlobalContextCallDiagnostics:
			hasCallDiag = true
			var doc aipipeline.ArtifactDocument
			if err := json.Unmarshal(art.PayloadJson, &doc); err != nil {
				t.Fatal(err)
			}
			env, _ := doc.Payload.(map[string]any)
			if env["parse_status"] != aipipeline.ParseStatusInvalidJSON {
				t.Fatalf("parse_status=%v", env["parse_status"])
			}
			if env["raw_body_head"] == "" || env["raw_body_tail"] == "" {
				t.Fatal("expected head/tail preview on compact diagnostics")
			}
			if env["parse_error"] == "" {
				t.Fatal("expected compact parse error")
			}
		case aipipeline.ArtifactKindGlobalContextRawResponse:
			hasRaw = true
		}
	}
	if !hasCallDiag || hasRaw {
		t.Fatalf("hasCallDiag=%v hasRaw=%v want true/false", hasCallDiag, hasRaw)
	}
}

type invalidJSONGlobalContextClient struct{}

func (invalidJSONGlobalContextClient) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	return nil, &aipipeline.GlobalContextRawResponse{
		Content:     "not-json",
		ParseStatus: aipipeline.ParseStatusInvalidJSON,
		ParseError:  "invalid_json: bad json",
	}, errors.New("invalid_json: bad json")
}

func (invalidJSONGlobalContextClient) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	panic("retry must not be called")
}

func TestGlobalContextRequiredArtifactSavedWithoutDebug(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	store := &persistingArtifactStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	req := basePipelineRequest()
	req.DebugArtifacts = false
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, art := range store.artifacts {
		if art.Kind == aipipeline.ArtifactKindGlobalContextFinal {
			found = true
		}
		if art.Kind == aipipeline.ArtifactKindGlobalContextResponse {
			t.Fatal("debug response artifact must not be required")
		}
	}
	if !found {
		t.Fatal("missing required global_context artifact")
	}
}

func TestGlobalContextResumeSkipsGeneration(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	gc := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	gc.generateCalls = 0

	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if gc.generateCalls != 0 || gc.retryCalls != 0 {
		t.Fatalf("generate=%d retry=%d", gc.generateCalls, gc.retryCalls)
	}
	if second.Diagnostics.GlobalContext.Source != "artifact" {
		t.Fatalf("source=%q", second.Diagnostics.GlobalContext.Source)
	}
}

func TestGlobalContextLoadErrorStopsPipeline(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	gc := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &persistingArtifactStore{listErr: errors.New("db down")}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		gc,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "global_context_load_failed" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if gc.generateCalls != 0 {
		t.Fatalf("generate=%d", gc.generateCalls)
	}
}

func TestSharedGlobalContextStageOrder(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	rec := &stageEventRecorder{}
	store := &eventPersistingStore{rec: rec}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: sharedChunkTestPack(t)},
		&recordingDashboardRunner{rec: rec},
		&recordingActionsRunner{rec: rec},
		store,
		&stageGlobalContextWrapper{inner: &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}, rec: rec},
	)
	svc.SetSnapshotPublisher(stagePublisher{rec: rec})
	svc.SetChunkAnalysisRunner(&stageChunkAnalyzer{rec: rec, inner: aipipeline.NewDeterministicChunkAnalysisRunner()})

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
	sharedAnalysis := idx("shared_chunk_analysis")
	sharedSave := idx("shared_artifact_save")
	globalContext := idx("global_context")
	dashboard := idx("dashboard")
	if sharedAnalysis < 0 || sharedSave < 0 || globalContext < 0 || dashboard < 0 {
		t.Fatalf("events=%v", rec.events)
	}
	if !(sharedAnalysis < sharedSave && sharedSave < globalContext && globalContext < dashboard) {
		t.Fatalf("invalid order: %v", rec.events)
	}
	for _, art := range store.artifacts {
		if art.Kind == aipipeline.ArtifactKindGlobalContextFinal {
			return
		}
	}
	t.Fatal("missing global_context required artifact")
}

func TestGlobalContextInputHashMismatchReanalyzes(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")

	gc := &countingRetryGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())
	aipipeline.SetGlobalContextClientForTest(svc, gc)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	for i := range store.artifacts {
		if store.artifacts[i].Kind != aipipeline.ArtifactKindGlobalContextFinal {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal(store.artifacts[i].PayloadJson, &doc); err != nil {
			t.Fatal(err)
		}
		payload, _ := doc["payload"].(map[string]any)
		payload["global_context_input_hash"] = "tampered"
		doc["payload"] = payload
		raw, _ := json.Marshal(doc)
		store.artifacts[i].PayloadJson = raw
	}
	gc.generateCalls = 0
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if gc.generateCalls != 1 {
		t.Fatalf("generate=%d want 1", gc.generateCalls)
	}
}

func TestGlobalContextPersistErrorStopsDownstream(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	dash := &fakeDashboardRunner{}
	store := &kindFailingArtifactStore{err: errors.New("disk full")}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		dash,
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.ErrorCode != "global_context_persist_failed" {
		t.Fatalf("error_code=%q", res.ErrorCode)
	}
	if dash.calls != 0 {
		t.Fatalf("dashboard calls=%d", dash.calls)
	}
}

type kindFailingArtifactStore struct {
	persistingArtifactStore
	failKind string
	err      error
}

func (s *kindFailingArtifactStore) CreateArtifact(ctx context.Context, params aipipeline.CreateArtifactParams) (int64, error) {
	if s.failKind == "" {
		s.failKind = aipipeline.ArtifactKindGlobalContextFinal
	}
	if params.Kind == s.failKind {
		return 0, s.err
	}
	return s.persistingArtifactStore.CreateArtifact(ctx, params)
}

// Ensure resume artifact helper compiles with real hash path.
func TestGlobalContextArtifactRoundTripHelper(t *testing.T) {
	pack := testSamplePack()
	hash := aihash.HashDataPack(pack)
	input := aipipeline.DefaultGlobalContextGenerationInput(pack, hash)
	ctx, _, err := aipipeline.NewDeterministicGlobalContextFakeClient().GenerateGlobalContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	inputHash, err := aipipeline.HashGlobalContextGenerationInput(input)
	if err != nil {
		t.Fatal(err)
	}
	gcHash := aihash.HashGlobalSemanticContext(ctx)
	artifact, err := aipipeline.BuildGlobalContextArtifactForTest(input, ctx, inputHash, gcHash)
	if err != nil {
		t.Fatal(err)
	}
	raw, docHash, err := aipipeline.MarshalGlobalContextArtifactDocumentForTest(1, artifact)
	if err != nil || len(raw) == 0 || docHash == "" {
		t.Fatalf("raw=%d hash=%s err=%v", len(raw), docHash, err)
	}
	_ = dashboardhome.ChunkAnalysisStatusCompleted
}
