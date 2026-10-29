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
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestValidatePipelineConfigRequiresAIFirst(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "0")
	if err := aipipeline.ValidatePipelineConfig(); !errors.Is(err, aipipeline.ErrPipelineRequiresAIFirst) {
		t.Fatalf("expected requires ai first, got %v", err)
	}
}

func TestPipelineBuildsDataPackOnce(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	pack := testSamplePack()
	var buildCount int
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack, count: &buildCount},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	res, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
		Trigger:           "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if buildCount != 1 {
		t.Fatalf("expected data pack built once, got %d", buildCount)
	}
	if res.DataPackHash == "" || res.GlobalContextHash == "" {
		t.Fatalf("expected hashes, got %+v", res)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("expected completed, got %s", res.Status)
	}
}

func TestRunPipelineOfflineNoReviewer(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	svc := newTestPipelineService()
	res, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompleted {
		t.Fatalf("status=%s", res.Status)
	}
	if res.Providers == nil {
		t.Fatal("expected providers")
	}
}

func TestRunPipelineRealProviderShapeNoReviewer(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	req := basePipelineRequest()
	req.AIEnabled = true
	req.ResolvedProviders = aipipeline.PipelineProviders{
		GlobalContext: aipipeline.ProviderOpenAI,
		Dashboard:     aipipeline.ProviderOpenAI,
		Actions:       aipipeline.ProviderOpenAI,
	}
	providers, err := aipipeline.ResolvePipelineProviders(req, true)
	if err != nil {
		t.Fatal(err)
	}
	if providers.GlobalContext != aipipeline.ProviderOpenAI {
		t.Fatalf("unexpected providers: %+v", providers)
	}
}

func TestPipelineSummaryHasNoReviewerFields(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	store := &artifactTrackingStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	req := basePipelineRequest()
	req.DebugArtifacts = true
	_, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := store.artifacts[aipipeline.ArtifactKindPipelineSummary]
	if !ok {
		t.Fatal("missing pipeline_summary artifact")
	}
	for _, key := range []string{
		"consistency_review_status", "consistency_review_id",
		"patches_proposed", "patches_applied", "consistency_patches_applied",
	} {
		if _, found := raw[key]; found {
			t.Fatalf("pipeline_summary must not contain %s", key)
		}
	}
}

func TestPipelineArtifactsContainOnlyStage3Kinds(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	store := &artifactTrackingStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	req := basePipelineRequest()
	req.DebugArtifacts = true
	_, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		aipipeline.ArtifactKindDataPack:                      true,
		aipipeline.ArtifactKindGlobalContextInput:            true,
		aipipeline.ArtifactKindGlobalContextPrompt:           true,
		aipipeline.ArtifactKindGlobalContextResponse:         true,
		aipipeline.ArtifactKindGlobalContextRawOutput:        true,
		aipipeline.ArtifactKindGlobalContextValidation:       true,
		aipipeline.ArtifactKindGlobalContextCallDiagnostics:  true,
		aipipeline.ArtifactKindGlobalContextParsedOutput:     true,
		aipipeline.ArtifactKindGlobalContextRawResponse:      true,
		aipipeline.ArtifactKindDashboardInput:                true,
		aipipeline.ArtifactKindDashboardOutput:               true,
		aipipeline.ArtifactKindDashboardRawOutput:            true,
		aipipeline.ArtifactKindDashboardValidation:           true,
		aipipeline.ArtifactKindDashboardCallDiagnostics:      true,
		aipipeline.ArtifactKindDashboardParsedOutput:         true,
		aipipeline.ArtifactKindDashboardRawResponse:          true,
		aipipeline.ArtifactKindActionsInput:                  true,
		aipipeline.ArtifactKindActionsOutput:                 true,
		aipipeline.ArtifactKindActionsRawOutput:              true,
		aipipeline.ArtifactKindActionsValidation:             true,
		aipipeline.ArtifactKindActionsFinalOutput:            true,
		aipipeline.ArtifactKindPipelineSummary:               true,
		aipipeline.ArtifactKindGlobalContextFinal:            true,
		aipipeline.ArtifactKindMetricSemanticsSnapshot:       true,
		aipipeline.ArtifactKindPublicationGuardDecision:      true,
		aipipeline.ArtifactKindSharedChunkRefMapping:         true,
		aipipeline.ArtifactKindFinalOutputSemanticValidation: true,
		aipipeline.ArtifactKindActionsExecutionDiagnostics:   true,
	}
	for kind := range store.artifactKinds {
		if !allowed[kind] {
			t.Fatalf("unexpected artifact kind %s", kind)
		}
	}
}

func TestProviderResolutionNoReviewer(t *testing.T) {
	t.Setenv("AI_PROVIDER_MODE", "openai")
	req := aipipeline.RunSellerAIPipelineRequest{AIEnabled: true}
	providers, err := aipipeline.ResolvePipelineProviders(req, true)
	if err != nil {
		t.Fatal(err)
	}
	if providers.GlobalContext != aipipeline.ProviderOpenAI {
		t.Fatalf("unexpected providers: %+v", providers)
	}
}

func TestPipelineContextFailureSkipsDownstream(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	pack := testSamplePack()
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		&fakeStore{},
		&failingGlobalContextClient{},
	)

	res, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusContextFailed {
		t.Fatalf("expected context_failed, got %s", res.Status)
	}
	if dash.calls != 0 || act.calls != 0 {
		t.Fatalf("expected no dashboard/actions calls on context failure")
	}
}

func TestPipelineCompletedPartial(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	pack := testSamplePack()
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		&fakeDashboardRunner{fail: true},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	res, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != aipipeline.PipelineStatusCompletedPartial {
		t.Fatalf("expected completed_partial when dashboard fails but actions run, got %s", res.Status)
	}
	if res.ActionsStatus == aipipeline.StepStatusSkipped {
		t.Fatalf("expected actions to run independently, got skipped")
	}
}

func newTestPipelineService() *aipipeline.SellerAIPipelineService {
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	return svc
}

type testSnapshotPublisher struct{}

func (testSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	return aipipeline.PublishResult{
		Snapshot:  publishedsnapshot.Snapshot{PipelineRunID: pipelineRunID, PublishedAt: time.Now().UTC()},
		IsCurrent: true,
	}, nil
}

func basePipelineRequest() aipipeline.RunSellerAIPipelineRequest {
	return aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
	}
}

type artifactTrackingStore struct {
	fakeStore
	artifactKinds map[string]bool
	artifacts     map[string]map[string]any
}

func (f *artifactTrackingStore) CreateArtifact(ctx context.Context, params aipipeline.CreateArtifactParams) (int64, error) {
	if f.artifactKinds == nil {
		f.artifactKinds = map[string]bool{}
	}
	if f.artifacts == nil {
		f.artifacts = map[string]map[string]any{}
	}
	f.artifactKinds[params.Kind] = true
	if params.Kind == aipipeline.ArtifactKindPipelineSummary && params.Payload != nil {
		var doc struct {
			Payload map[string]any `json:"payload"`
		}
		if raw, err := json.Marshal(params.Payload); err == nil {
			_ = json.Unmarshal(raw, &doc)
			f.artifacts[params.Kind] = doc.Payload
		}
	}
	return 1, nil
}

type fakeDataPackBuilder struct {
	pack  *aidatapack.SellerAIDataPack
	count *int
}

func (f *fakeDataPackBuilder) Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error) {
	if f.count != nil {
		*f.count++
	}
	return f.pack, nil
}

type fakeDashboardRunner struct {
	calls int
	fail  bool
}

func (f *fakeDashboardRunner) RunForSellerMetricDateWithOptions(ctx context.Context, sellerAccountID int64, metricDate time.Time, opts dashboardhome.NarrativeRunOptions) (*aipipeline.DashboardRunOutcome, error) {
	f.calls++
	if opts.DataPackOverride == nil || opts.GlobalContext == nil {
		return nil, errors.New("missing shared pack/context")
	}
	if f.fail {
		return &aipipeline.DashboardRunOutcome{
			Result: &dashboardhome.NarrativeGenerationResult{Status: "failed", ErrorMessage: "dashboard failed"},
		}, nil
	}
	return &aipipeline.DashboardRunOutcome{
		Result: &dashboardhome.NarrativeGenerationResult{
			Status:                         "completed",
			RunID:                          "uuid-dashboard",
			DashboardSynthesisInputTokens:  400,
			DashboardSynthesisOutputTokens: 40,
			DashboardSynthesisRequestID:    "req_dash_123",
			DashboardSynthesisLatencyMs:    0,
			InputTokens:                    400,
			OutputTokens:                   40,
			DashboardChunkUsageIncluded:    false,
		},
		DebugInput: map[string]any{
			"mode": "fake",
		},
		DebugOutput: map[string]any{
			"mode":    "fake",
			"model":   "fake-dashboard-deterministic",
			"payload": map[string]any{"executive_summary": "old"},
		},
	}, nil
}

type fakeActionsRunner struct {
	calls int
	fail  bool
	empty bool
}

func (f *fakeActionsRunner) BuildActionsAIFirstInputFromDataPack(pack *aidatapack.SellerAIDataPack, dataPackHash string, globalContext *aicontext.GlobalSemanticContext, opts actions.GenerateActionsOptions) (actions.ActionsAIFirstInput, error) {
	if pack == nil || globalContext == nil {
		return actions.ActionsAIFirstInput{}, errors.New("missing shared pack/context")
	}
	return actions.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalContext, opts)
}

func (f *fakeActionsRunner) BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []actions.AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	if pack == nil || globalContext == nil {
		return actions.ActionsAIFirstInput{}, errors.New("missing shared pack/context")
	}
	return actions.BuildActionsAIFirstInputFromCandidates(
		pack, dataPackHash, globalContext, globalContextHash, candidates, failedChunkIDs, dataLimitations, opts,
	)
}

func (f *fakeActionsRunner) GenerateActionsAIFirstFromInput(ctx context.Context, input actions.ActionsAIFirstInput, opts actions.GenerateActionsOptions) (*actions.GenerateActionsResult, error) {
	f.calls++
	if f.fail {
		return &actions.GenerateActionsResult{RunID: 42, FinalStatus: actions.GenerationFinalStatusFailed}, nil
	}
	if f.empty {
		return &actions.GenerateActionsResult{RunID: 42, FinalStatus: actions.GenerationFinalStatusEmpty}, nil
	}
	return &actions.GenerateActionsResult{
		RunID:        42,
		FinalStatus:  actions.GenerationFinalStatusCompleted,
		ComposeCalls: 1,
		InputTokens:  512,
		OutputTokens: 128,
	}, nil
}

type fakeStore struct {
	nextID int64
}

func (f *fakeStore) CreatePipelineRun(ctx context.Context, params aipipeline.CreatePipelineRunParams) (int64, error) {
	f.nextID++
	return f.nextID, nil
}
func (f *fakeStore) UpdatePipelineRun(ctx context.Context, params aipipeline.UpdatePipelineRunParams) error {
	return nil
}
func (f *fakeStore) CreateArtifact(ctx context.Context, params aipipeline.CreateArtifactParams) (int64, error) {
	return 1, nil
}
func (f *fakeStore) LinkDashboardRun(ctx context.Context, narrativeID string, sellerID, pipelineRunID int64, dataPackHash, globalContextHash string) error {
	return nil
}
func (f *fakeStore) GetDashboardNarrativeHashes(ctx context.Context, narrativeID string, sellerAccountID int64) (string, string, error) {
	return "", "", errors.New("dashboard narrative hashes not configured")
}
func (f *fakeStore) GetActionGenerationRunHashes(ctx context.Context, actionsRunID, sellerAccountID int64) (string, string, error) {
	return "", "", errors.New("action generation run hashes not configured")
}
func (f *fakeStore) GetActionGenerationRunFinalStatus(ctx context.Context, actionsRunID, sellerAccountID int64) (string, error) {
	return "", errors.New("action generation run final status not configured")
}
func (f *fakeStore) LinkActionsRun(ctx context.Context, actionsRunID, pipelineRunID int64, dataPackHash, globalContextHash string) error {
	return nil
}
func (f *fakeStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	return dbgen.AiPipelineRun{ID: id, Status: aipipeline.PipelineStatusRunning}, nil
}
func (f *fakeStore) ListArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	return nil, nil
}

type failingGlobalContextClient struct{}

func (f *failingGlobalContextClient) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	return nil, nil, errors.New("gc failed")
}
func (f *failingGlobalContextClient) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	return nil, nil, errors.New("gc retry failed")
}

type noopLeaseGuard struct{}

func (noopLeaseGuard) ValidateLease(ctx context.Context, dispatchID, pipelineRunID int64) error {
	return nil
}
func (noopLeaseGuard) RenewLease(ctx context.Context, dispatchID int64) error { return nil }

type countingGlobalContextClient struct {
	inner aipipeline.GlobalContextAIClient
	calls int
}

func (c *countingGlobalContextClient) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	c.calls++
	return c.inner.GenerateGlobalContext(ctx, input)
}
func (c *countingGlobalContextClient) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	c.calls++
	return c.inner.RetryGlobalContext(ctx, input)
}

type resumePipelineStore struct {
	fakeStore
	run       dbgen.AiPipelineRun
	artifacts []dbgen.AiPipelineArtifact
}

func (s *resumePipelineStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	run := s.run
	run.ID = id
	return run, nil
}
func (s *resumePipelineStore) ListArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	return s.artifacts, nil
}
func (s *resumePipelineStore) GetDashboardNarrativeHashes(ctx context.Context, narrativeID string, sellerAccountID int64) (string, string, error) {
	return aihash.HashDataPack(testSamplePack()), "", nil
}

func TestRun_ExistingPipelineResumesGlobalContext(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	gcClient := &countingGlobalContextClient{inner: aipipeline.NewDeterministicGlobalContextFakeClient()}
	dash := &fakeDashboardRunner{}
	store := &persistingArtifactStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		dash,
		&fakeActionsRunner{},
		store,
		gcClient,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	first, err := svc.Run(context.Background(), basePipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if gcClient.calls != 1 {
		t.Fatalf("first run gc calls=%d", gcClient.calls)
	}
	gcClient.calls = 0

	req := basePipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if gcClient.calls != 0 {
		t.Fatalf("global context calls=%d want 0 on resume", gcClient.calls)
	}
	if dash.calls != 2 {
		t.Fatalf("dashboard calls=%d want 2 (initial + resume)", dash.calls)
	}
}

func TestRun_ExistingPipelineResumesDashboard(t *testing.T) {
	gcClient := aipipeline.NewDeterministicGlobalContextFakeClient()
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	store := &resumePipelineStore{
		run: dbgen.AiPipelineRun{
			ID:                  99,
			Status:              aipipeline.PipelineStatusRunning,
			DataPackHash:        aihash.HashDataPack(testSamplePack()),
			GlobalContextStatus: pgtype.Text{String: aipipeline.GlobalContextStatusCompleted, Valid: true},
			DashboardRunID:      pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			DashboardStatus:     pgtype.Text{String: aipipeline.StepStatusCompleted, Valid: true},
		},
	}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		dash,
		act,
		store,
		gcClient,
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	req := basePipelineRequest()
	req.ExistingPipelineRunID = 99
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	if _, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if dash.calls != 0 {
		t.Fatalf("dashboard calls=%d want 0", dash.calls)
	}
	if act.calls != 1 {
		t.Fatalf("actions calls=%d want 1", act.calls)
	}
}

func testSamplePack() *aidatapack.SellerAIDataPack {
	return &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-24"},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {FactID: "account.current_day.revenue", FormattedValue: "1000"},
			"campaign.1.last_7d.drr":      {FactID: "campaign.1.last_7d.drr", FormattedValue: "12%"},
		},
		EntityRegistry: aidatapack.EntityRegistry{
			Campaigns: map[string]aidatapack.CampaignEntity{"campaign:1": {EntityID: "campaign:1", Name: "Test"}},
			SKUs:      map[string]aidatapack.SKUEntity{"sku:1": {EntityID: "sku:1", SKU: "1", Name: "SKU"}},
		},
		Tables: aidatapack.Tables{
			CampaignRollups: []aidatapack.CampaignRollupRow{{EntityID: "campaign:1", Name: "Test"}},
		},
	}
}
