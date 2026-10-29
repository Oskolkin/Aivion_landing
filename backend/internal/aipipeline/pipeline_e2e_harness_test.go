package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	e2eSellerID   int64 = 1
	e2eMetricDate       = "2026-06-25"
)

func e2ePipelineEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AI_PROVIDER_MODE", "fake")
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "true")
	t.Setenv("ACTIONS_AI_FIRST", "true")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")
	t.Setenv("LEGACY_AI_POST_SYNC_ENABLED", "false")
	t.Setenv("OPENAI_MAX_RETRIES", "0")
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
}

func e2eFixedMetricDate() time.Time {
	return time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
}

func e2eExpectedChunkCount(t *testing.T) int {
	t.Helper()
	_, set, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	return len(set.Chunks)
}

func e2eBaseRequest() aipipeline.RunSellerAIPipelineRequest {
	return aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   e2eSellerID,
		MetricDate:        e2eFixedMetricDate(),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         true,
		Trigger:           "e2e",
	}
}

// PipelineAICallRecorder records component-level AI calls for E2E assertions.
type PipelineAICallRecorder struct {
	mu    sync.Mutex
	Calls []PipelineAICall
}

type PipelineAICall struct {
	Component string
	ChunkID   string
	Status    string
	RequestID string
}

func (r *PipelineAICallRecorder) Record(component, chunkID, status, requestID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Calls = append(r.Calls, PipelineAICall{
		Component: component,
		ChunkID:   chunkID,
		Status:    status,
		RequestID: requestID,
	})
}

func (r *PipelineAICallRecorder) Count(component string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.Calls {
		if c.Component == component {
			n++
		}
	}
	return n
}

func (r *PipelineAICallRecorder) CountChunkCalls() int {
	return r.Count(aipipeline.AIUsageComponentChunkAnalysis)
}

func (r *PipelineAICallRecorder) Total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Calls)
}

func (r *PipelineAICallRecorder) Sequence() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.Calls))
	for i, c := range r.Calls {
		if c.ChunkID != "" {
			out[i] = c.Component + ":" + c.ChunkID
		} else {
			out[i] = c.Component
		}
	}
	return out
}

type recordingChunkRunner struct {
	rec   *PipelineAICallRecorder
	inner aipipeline.ChunkAnalysisRunner
}

func (r *recordingChunkRunner) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts dashboardhome.NarrativeRunOptions,
) (*dashboardhome.ChunkAnalysisRunResult, error) {
	result, err := r.inner.AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
	if err != nil {
		return nil, err
	}
	if result != nil {
		for _, call := range result.ChunkCalls {
			status := call.Status
			if status == "" {
				status = "completed"
			}
			r.rec.Record(aipipeline.AIUsageComponentChunkAnalysis, call.ChunkID, status, call.RequestID)
		}
	}
	return result, err
}

type recordingGlobalContextClient struct {
	rec   *PipelineAICallRecorder
	inner aipipeline.GlobalContextAIClient
}

func (c *recordingGlobalContextClient) GenerateGlobalContext(ctx context.Context, input aipipeline.GlobalContextGenerationInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	c.rec.Record(aipipeline.AIUsageComponentGlobalContext, "", "completed", "gc-req")
	return c.inner.GenerateGlobalContext(ctx, input)
}

func (c *recordingGlobalContextClient) RetryGlobalContext(ctx context.Context, input aipipeline.GlobalContextRetryInput) (*aicontext.GlobalSemanticContext, *aipipeline.GlobalContextRawResponse, error) {
	c.rec.Record("global_context_retry", "", "retry", "gc-retry-req")
	return c.inner.RetryGlobalContext(ctx, input)
}

type e2eSnapshotPublisher struct {
	mu        sync.Mutex
	published []int64
	current   int64
}

func (p *e2eSnapshotPublisher) PublishPipelineSnapshot(ctx context.Context, pipelineRunID int64) (aipipeline.PublishResult, error) {
	_ = ctx
	p.mu.Lock()
	defer p.mu.Unlock()
	p.published = append(p.published, pipelineRunID)
	p.current = pipelineRunID
	return aipipeline.PublishResult{
		Snapshot: publishedsnapshot.Snapshot{
			PipelineRunID: pipelineRunID,
			PublishedAt:   time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		},
		IsCurrent: true,
	}, nil
}

func (p *e2eSnapshotPublisher) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.published)
}

func (p *e2eSnapshotPublisher) currentID() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.current
}

type e2ePipelineStore struct {
	persistingArtifactStore
	publishedRun   *dbgen.AiPipelineRun
	publishedByKey map[string]dbgen.AiPipelineRun
	runs           map[int64]dbgen.AiPipelineRun
	nextID         int64
}

func newE2EPipelineStore() *e2ePipelineStore {
	return &e2ePipelineStore{
		publishedByKey: map[string]dbgen.AiPipelineRun{},
		runs:           map[int64]dbgen.AiPipelineRun{},
	}
}

func (s *e2ePipelineStore) CreatePipelineRun(ctx context.Context, params aipipeline.CreatePipelineRunParams) (int64, error) {
	s.nextID++
	id := s.nextID
	s.runs[id] = dbgen.AiPipelineRun{
		ID:              id,
		SellerAccountID: params.SellerAccountID,
		Status:          params.Status,
		DataPackHash:    params.DataPackHash,
	}
	return id, nil
}

func (s *e2ePipelineStore) UpdatePipelineRun(ctx context.Context, params aipipeline.UpdatePipelineRunParams) error {
	run := s.runs[params.ID]
	run.Status = params.Status
	if params.GlobalContextHash != "" {
		run.GlobalContextHash = pgtype.Text{String: params.GlobalContextHash, Valid: true}
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
	s.runs[params.ID] = run
	return nil
}

func (s *e2ePipelineStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	run, ok := s.runs[id]
	if !ok {
		return dbgen.AiPipelineRun{ID: id, Status: aipipeline.PipelineStatusRunning}, nil
	}
	return run, nil
}

func (s *e2ePipelineStore) seedPublished(run dbgen.AiPipelineRun) {
	key := fmt.Sprintf("%d:%s:%s", run.SellerAccountID, run.MetricDate.Time.Format("2006-01-02"), run.DataPackHash)
	s.publishedByKey[key] = run
	s.publishedRun = &run
}

func (s *e2ePipelineStore) GetPublishedAIPipelineRunBySellerDateAndDataPackHash(
	ctx context.Context,
	arg dbgen.GetPublishedAIPipelineRunBySellerDateAndDataPackHashParams,
) (dbgen.AiPipelineRun, error) {
	key := fmt.Sprintf("%d:%s:%s", arg.SellerAccountID, arg.MetricDate.Time.Format("2006-01-02"), arg.DataPackHash)
	if run, ok := s.publishedByKey[key]; ok {
		return run, nil
	}
	if s.publishedRun != nil {
		return *s.publishedRun, nil
	}
	return dbgen.AiPipelineRun{}, errors.New("not found")
}

type e2eHarness struct {
	svc       *aipipeline.SellerAIPipelineService
	rec       *PipelineAICallRecorder
	usage     *aipipeline.CaptureUsageRecorder
	store     *e2ePipelineStore
	publisher *e2eSnapshotPublisher
	dashAI    *stage6SynthesisAI
	actionsAI *actions.FakeActionAIClient
	actRunner *stage7CapturingActionsRunner
}

type e2eHarnessConfig struct {
	rec         *PipelineAICallRecorder
	chunkRunner aipipeline.ChunkAnalysisRunner
	gcClient    aipipeline.GlobalContextAIClient
	dashRunner  aipipeline.DashboardRunner
	actionsAI   *actions.FakeActionAIClient
	debug       bool
}

func newE2EHarness(t *testing.T, cfg e2eHarnessConfig) *e2eHarness {
	t.Helper()
	e2ePipelineEnv(t)

	rec := cfg.rec
	if rec == nil {
		rec = &PipelineAICallRecorder{}
	}
	usage := aipipeline.NewCaptureUsageRecorderForTest()
	store := newE2EPipelineStore()
	publisher := &e2eSnapshotPublisher{}

	pack, _, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}

	chunkRunner := cfg.chunkRunner
	if chunkRunner == nil {
		chunkRunner = &recordingChunkRunner{
			rec:   rec,
			inner: aipipeline.NewDeterministicChunkAnalysisRunner(),
		}
	}

	gcClient := cfg.gcClient
	if gcClient == nil {
		gcClient = &recordingGlobalContextClient{
			rec:   rec,
			inner: aipipeline.NewDeterministicGlobalContextFakeClient(),
		}
	}

	dashAI := &stage6SynthesisAI{mode: stage6SynthesisOK}
	dashRunner := cfg.dashRunner
	if dashRunner == nil {
		dashRunner = wrapPrecomputedDashWithRecorder(rec, dashAI)
	}

	actionsAI := cfg.actionsAI
	if actionsAI == nil {
		actionsAI = stage7ValidatingFakeAI()
	}
	wrapActionsAIWithRecorder(rec, actionsAI)
	actRunner := newStage7RealActionsRunner(actionsAI)

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dashRunner,
		actRunner,
		store,
		gcClient,
	)
	svc.SetSnapshotPublisher(publisher)
	svc.SetChunkAnalysisRunner(chunkRunner)
	svc.SetUsageRecorder(usage)

	return &e2eHarness{
		svc:       svc,
		rec:       rec,
		usage:     usage,
		store:     store,
		publisher: publisher,
		dashAI:    dashAI,
		actionsAI: actionsAI,
		actRunner: actRunner,
	}
}

func (h *e2eHarness) run(t *testing.T, req aipipeline.RunSellerAIPipelineRequest) *aipipeline.RunSellerAIPipelineResult {
	t.Helper()
	res, err := h.svc.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("pipeline run: %v", err)
	}
	return res
}

func assertE2EUsageCounts(t *testing.T, events []aicost.UsageEventInput, chunk, gc, dash, act int) {
	t.Helper()
	gotChunk := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentChunkAnalysis)
	gotGC := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentGlobalContext)
	gotDash := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentDashboardSynthesis)
	gotAct := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentActionsSynthesis)
	if gotChunk != chunk || gotGC != gc || gotDash != dash || gotAct != act {
		t.Fatalf("usage counts chunk=%d/%d gc=%d/%d dash=%d/%d act=%d/%d",
			gotChunk, chunk, gotGC, gc, gotDash, dash, gotAct, act)
	}
}

func assertNoRetryUsage(t *testing.T, rec *aipipeline.CaptureUsageRecorder) {
	t.Helper()
	for _, ev := range rec.Events() {
		if ev.Attempt != 1 {
			t.Fatalf("attempt=%d component=%s", ev.Attempt, ev.Component)
		}
		if strings.Contains(ev.Component, "retry") || strings.Contains(ev.Component, "_initial") {
			t.Fatalf("legacy component=%s", ev.Component)
		}
	}
}

func assertArtifactKinds(t *testing.T, store *e2ePipelineStore, runID int64, want map[string]bool) {
	t.Helper()
	arts, err := store.ListArtifacts(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, art := range arts {
		got[art.Kind] = true
	}
	for kind, required := range want {
		if required && !got[kind] {
			t.Fatalf("missing artifact kind %s (have %v)", kind, got)
		}
		if !required && got[kind] {
			t.Fatalf("unexpected artifact kind %s", kind)
		}
	}
}

func assertArtifactPayloadNotContains(t *testing.T, store *e2ePipelineStore, runID int64, kind, needle string) {
	t.Helper()
	arts, err := store.ListArtifacts(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, art := range arts {
		if art.Kind != kind {
			continue
		}
		if strings.Contains(strings.ToLower(string(art.PayloadJson)), strings.ToLower(needle)) {
			t.Fatalf("artifact %s contains %q", kind, needle)
		}
	}
}

func chunkRunnerNoCandidates(rec *PipelineAICallRecorder) aipipeline.ChunkAnalysisRunner {
	if rec == nil {
		rec = &PipelineAICallRecorder{}
	}
	return &recordingChunkRunner{
		rec: rec,
		inner: chunkRunnerFunc(func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			res, err := aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
			if err != nil {
				return nil, err
			}
			res.ValidatedActionCandidates = nil
			res.ValidatedCandidateCount = 0
			return res, nil
		}),
	}
}

type chunkRunnerFunc func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error)

func (f chunkRunnerFunc) AnalyzeDataPackChunks(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
	return f(ctx, pack, chunkSet, opts)
}

func chunkRunnerOneFailed(rec *PipelineAICallRecorder) aipipeline.ChunkAnalysisRunner {
	return &recordingChunkRunner{
		rec: rec,
		inner: chunkRunnerFunc(func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			result, err := aipipeline.NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
			if err != nil || result == nil || len(chunkSet.Chunks) < 1 {
				return result, err
			}
			result.Completed = len(chunkSet.Chunks) - 1
			result.Failed = 1
			result.FailedChunkIDs = []string{chunkSet.Chunks[0].ChunkID}
			result.FailedChunkTypes = []string{chunkSet.Chunks[0].ChunkType}
			result.AnalysisStatus = dashboardhome.ChunkAnalysisStatusPartial
			if len(result.ChunkCalls) > 0 {
				result.ChunkCalls[0].Status = dashboardhome.ChunkCallStatusFailed
			}
			return result, nil
		}),
	}
}

func chunkRunnerAllFailed(rec *PipelineAICallRecorder) aipipeline.ChunkAnalysisRunner {
	return &recordingChunkRunner{
		rec: rec,
		inner: chunkRunnerFunc(func(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, opts dashboardhome.NarrativeRunOptions) (*dashboardhome.ChunkAnalysisRunResult, error) {
			failedIDs := make([]string, len(chunkSet.Chunks))
			failedTypes := make([]string, len(chunkSet.Chunks))
			for i, ch := range chunkSet.Chunks {
				failedIDs[i] = ch.ChunkID
				failedTypes[i] = ch.ChunkType
				rec.Record(aipipeline.AIUsageComponentChunkAnalysis, ch.ChunkID, dashboardhome.ChunkCallStatusFailed, "")
			}
			return &dashboardhome.ChunkAnalysisRunResult{
				SellerAccountID:  pack.SellerContext.SellerAccountID,
				MetricDate:       pack.SellerContext.MetricDate,
				ChunkCount:       len(chunkSet.Chunks),
				Failed:           len(chunkSet.Chunks),
				FailedChunkIDs:   failedIDs,
				FailedChunkTypes: failedTypes,
				AnalysisStatus:   dashboardhome.ChunkAnalysisStatusFailed,
			}, nil
		}),
	}
}

func wrapPrecomputedDashWithRecorder(rec *PipelineAICallRecorder, ai *stage6SynthesisAI) aipipeline.DashboardRunner {
	return &precomputedDashRecorder{rec: rec, inner: &precomputedSynthesisDashboardRunner{ai: ai}}
}

type precomputedDashRecorder struct {
	rec   *PipelineAICallRecorder
	inner aipipeline.DashboardRunner
}

func (r *precomputedDashRecorder) RunForSellerMetricDateWithOptions(ctx context.Context, sellerAccountID int64, metricDate time.Time, opts dashboardhome.NarrativeRunOptions) (*aipipeline.DashboardRunOutcome, error) {
	out, err := r.inner.RunForSellerMetricDateWithOptions(ctx, sellerAccountID, metricDate, opts)
	if out != nil && out.Result != nil && out.Result.IsSuccess() {
		r.rec.Record(aipipeline.AIUsageComponentDashboardSynthesis, "", "completed", out.Result.DashboardSynthesisRequestID)
	} else if out != nil && out.Result != nil {
		r.rec.Record(aipipeline.AIUsageComponentDashboardSynthesis, "", "failed", out.Result.DashboardSynthesisRequestID)
	}
	return out, err
}

func wrapActionsAIWithRecorder(rec *PipelineAICallRecorder, ai *actions.FakeActionAIClient) *actions.FakeActionAIClient {
	orig := ai.ComposeAIFirstFn
	ai.ComposeAIFirstFn = func(ctx context.Context, input actions.ActionsAIFirstInput) (*actions.ActionsAIFirstOutput, *actions.AIRawResponse, error) {
		rec.Record(aipipeline.AIUsageComponentActionsSynthesis, "", "completed", "actions-req")
		if orig != nil {
			return orig(ctx, input)
		}
		return nil, nil, errors.New("compose not configured")
	}
	return ai
}

func assertStageOrder(t *testing.T, seq []string) {
	t.Helper()
	idx := func(prefix string) int {
		for i, s := range seq {
			if strings.HasPrefix(s, prefix) {
				return i
			}
		}
		return -1
	}
	gc := idx(aipipeline.AIUsageComponentGlobalContext)
	dash := idx(aipipeline.AIUsageComponentDashboardSynthesis)
	act := idx(aipipeline.AIUsageComponentActionsSynthesis)
	if gc < 0 || dash < 0 || act < 0 {
		t.Fatalf("missing stages in %v", seq)
	}
	if !(gc < dash && dash < act) {
		t.Fatalf("stage order invalid: %v", seq)
	}
	chunkIdxs := []int{}
	for i, s := range seq {
		if strings.HasPrefix(s, aipipeline.AIUsageComponentChunkAnalysis) {
			chunkIdxs = append(chunkIdxs, i)
		}
	}
	if len(chunkIdxs) == 0 {
		t.Fatal("expected chunk calls")
	}
	for _, ci := range chunkIdxs {
		if ci >= gc {
			t.Fatalf("chunk call after global context: %v", seq)
		}
	}
}

func sortedUniqueKinds(store *e2ePipelineStore, runID int64) []string {
	arts, _ := store.ListArtifacts(context.Background(), runID)
	kinds := make([]string, 0, len(arts))
	for _, art := range arts {
		kinds = append(kinds, art.Kind)
	}
	sort.Strings(kinds)
	return kinds
}

func decodeArtifactPayload(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var doc struct {
		Payload map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Payload
}
