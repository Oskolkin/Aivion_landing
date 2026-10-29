package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

type stage6SynthesisMode string

const (
	stage6SynthesisOK            stage6SynthesisMode = "ok"
	stage6SynthesisProviderError stage6SynthesisMode = "provider_error"
	stage6SynthesisInvalidOutput stage6SynthesisMode = "invalid_output"
)

type stage6SynthesisAI struct {
	mode                   stage6SynthesisMode
	chunkCalls             int
	synthCalls             int
	lastSynthesisSignalIDs []string
}

func (a *stage6SynthesisAI) GenerateStructuredJSON(ctx context.Context, input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
	if _, ok := input.Context.(dashboardhome.DashboardSynthesisInput); ok {
		a.synthCalls++
		if synthInput, ok := input.Context.(dashboardhome.DashboardSynthesisInput); ok {
			ids := make([]string, 0, len(synthInput.ValidatedSignals))
			for _, sig := range synthInput.ValidatedSignals {
				ids = append(ids, sig.SignalID)
			}
			sort.Strings(ids)
			a.lastSynthesisSignalIDs = ids
		}
		switch a.mode {
		case stage6SynthesisProviderError:
			return nil, errors.New("synthesis provider unavailable")
		case stage6SynthesisInvalidOutput:
			return &recommendations.GenerateRecommendationsOutput{
				Content:      `{"headline":"","executive_summary":"","improvement_summary":"","health_reasons":[],"top_actions":[],"data_quality_notes":[]}`,
				InputTokens:  100,
				OutputTokens: 20,
			}, nil
		default:
			return &recommendations.GenerateRecommendationsOutput{
				Content:      `{"headline":"Заголовок","executive_summary":"Резюме.","improvement_summary":"Улучшения.","health_reasons":[],"top_actions":[],"data_quality_notes":[]}`,
				InputTokens:  3000,
				OutputTokens: 600,
			}, nil
		}
	}
	a.chunkCalls++
	return nil, errors.New("unexpected chunk analysis in precomputed dashboard path")
}

type precomputedSynthesisDashboardRunner struct {
	ai *stage6SynthesisAI
}

func (r *precomputedSynthesisDashboardRunner) RunForSellerMetricDateWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts dashboardhome.NarrativeRunOptions,
) (*aipipeline.DashboardRunOutcome, error) {
	if opts.PrecomputedChunkRun == nil || opts.ChunkSetOverride == nil || opts.DataPackOverride == nil {
		return nil, errors.New("precomputed dashboard path requires overrides")
	}
	pack := opts.DataPackOverride
	if err := dashboardhome.ValidatePrecomputedChunkRun(pack, opts.ChunkSetOverride, opts.PrecomputedChunkRun); err != nil {
		return &aipipeline.DashboardRunOutcome{
			Result: &dashboardhome.NarrativeGenerationResult{
				Status:       "failed",
				ErrorCode:    dashboardhome.NarrativeErrorPrecomputedChunkRunInvalid,
				ErrorMessage: err.Error(),
			},
		}, nil
	}
	build, err := dashboardhome.BuildPrecomputedSynthesisNarrativeContext(pack)
	if err != nil {
		return nil, err
	}
	runCopy := dashboardhome.CloneChunkAnalysisRunResult(opts.PrecomputedChunkRun)
	orch := &dashboardhome.DataPackChunksOrchestrator{AI: r.ai, Model: "test-model"}
	gen := orch.SynthesizeFromChunkRun(ctx, pack, opts.ChunkSetOverride, runCopy, build, opts)
	if gen.Fatal {
		code := dashboardhome.NarrativeErrorCode(gen.Error)
		if code == "" {
			code = "dashboard_synthesis_failed"
		}
		return &aipipeline.DashboardRunOutcome{
			Result: &dashboardhome.NarrativeGenerationResult{
				Status:       "failed",
				ErrorCode:    code,
				ErrorMessage: gen.Error.Error(),
			},
		}, nil
	}
	return &aipipeline.DashboardRunOutcome{
		Result: &dashboardhome.NarrativeGenerationResult{
			Status:               "completed",
			InputTokens:          gen.SynthesisInputTokens,
			OutputTokens:         gen.SynthesisOutputTokens,
			DashboardChunkSource: "precomputed",
		},
	}, nil
}

func newStage6SharedPipelineService(
	t *testing.T,
	dash aipipeline.DashboardRunner,
	actions aipipeline.ActionsRunner,
	store aipipeline.PipelineStore,
	publisher aipipeline.SnapshotPublisher,
) *aipipeline.SellerAIPipelineService {
	t.Helper()
	sharedChunkPipelineEnv(t)
	pack, _, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		actions,
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(publisher)
	svc.SetChunkAnalysisRunner(aipipeline.NewDeterministicChunkAnalysisRunner())
	return svc
}

func TestRun_SharedFlag_PrecomputedDashboardProviderError_NoPublication(t *testing.T) {
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
	ai := &stage6SynthesisAI{mode: stage6SynthesisProviderError}
	dash := &precomputedSynthesisDashboardRunner{ai: ai}
	actRec := &stageEventRecorder{}
	act := &recordingActionsRunner{rec: actRec}
	publisher := &recordingSnapshotPublisher{}
	svc := newStage6SharedPipelineService(t, dash, act, &fakeStore{}, publisher)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if ai.chunkCalls != 0 {
		t.Fatalf("dashboard chunk calls=%d", ai.chunkCalls)
	}
	if ai.synthCalls != 1 {
		t.Fatalf("dashboard synthesis calls=%d want 1", ai.synthCalls)
	}
	if len(actRec.events) == 0 {
		t.Fatalf("actions should run independently after dashboard failure")
	}
	if res.ActionsStatus == aipipeline.StepStatusSkipped {
		t.Fatalf("actions_status=%s want not skipped", res.ActionsStatus)
	}
	if res.Published {
		t.Fatal("dashboard provider error must not publish")
	}
	if publisher.calls != 0 {
		t.Fatalf("publication calls=%d", publisher.calls)
	}
	if res.DashboardStatus != aipipeline.StepStatusFailed {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
}

func TestRun_SharedFlag_PrecomputedDashboardInvalidOutput_NoRetryNoPublication(t *testing.T) {
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
	ai := &stage6SynthesisAI{mode: stage6SynthesisInvalidOutput}
	dash := &precomputedSynthesisDashboardRunner{ai: ai}
	publisher := &recordingSnapshotPublisher{}
	svc := newStage6SharedPipelineService(t, dash, &recordingActionsRunner{rec: &stageEventRecorder{}}, &fakeStore{}, publisher)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if ai.chunkCalls != 0 || ai.synthCalls != 1 {
		t.Fatalf("chunk=%d synthesis=%d", ai.chunkCalls, ai.synthCalls)
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
	if res.DashboardStatus != aipipeline.StepStatusFailed {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
}

func TestRun_SharedFlag_PrecomputedDashboardOversizedInput_NoSynthesisNoPublication(t *testing.T) {
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", "10")
	ai := &stage6SynthesisAI{mode: stage6SynthesisOK}
	dash := &precomputedSynthesisDashboardRunner{ai: ai}
	publisher := &recordingSnapshotPublisher{}
	svc := newStage6SharedPipelineService(t, dash, &recordingActionsRunner{rec: &stageEventRecorder{}}, &fakeStore{}, publisher)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if ai.chunkCalls != 0 || ai.synthCalls != 0 {
		t.Fatalf("chunk=%d synthesis=%d", ai.chunkCalls, ai.synthCalls)
	}
	if res.Published || publisher.calls != 0 {
		t.Fatalf("published=%v publication_calls=%d", res.Published, publisher.calls)
	}
	if res.DashboardStatus != aipipeline.StepStatusFailed {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
	if res.ErrorCode != dashboardhome.NarrativeErrorDashboardInputTooLarge {
		t.Fatalf("error_code=%q want %s", res.ErrorCode, dashboardhome.NarrativeErrorDashboardInputTooLarge)
	}
}

func TestRun_SharedArtifactResume_DashboardUsesArtifactWithoutChunkAnalysis(t *testing.T) {
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
	sharedChunkPipelineEnv(t)
	rec := &stageEventRecorder{}
	pack, _, err := aipipeline.PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	ai := &stage6SynthesisAI{mode: stage6SynthesisOK}
	dash := &precomputedSynthesisDashboardRunner{ai: ai}
	store := &persistingArtifactStore{}
	runner := aipipeline.NewDeterministicChunkAnalysisRunner()
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		&recordingActionsRunner{rec: rec},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(stagePublisher{rec: rec})
	svc.SetChunkAnalysisRunner(&stageChunkAnalyzer{rec: rec, inner: runner})

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if len(store.sharedChunkArtifacts()) != 1 {
		t.Fatalf("shared artifacts=%d", len(store.sharedChunkArtifacts()))
	}

	ai.chunkCalls = 0
	ai.synthCalls = 0
	rec.events = nil
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	if second, err := svc.Run(context.Background(), req); err != nil {
		t.Fatal(err)
	} else if second.Diagnostics.SharedChunkAnalysis.Source != "artifact" {
		t.Fatalf("shared source=%q", second.Diagnostics.SharedChunkAnalysis.Source)
	}
	if countEvents(rec.events, "shared_chunk_analysis") != 0 {
		t.Fatalf("shared analyzer calls on resume: %v", rec.events)
	}
	if ai.chunkCalls != 0 {
		t.Fatalf("dashboard chunk calls=%d", ai.chunkCalls)
	}
	if ai.synthCalls != 1 {
		t.Fatalf("dashboard synthesis calls=%d", ai.synthCalls)
	}
	var artifact struct {
		Payload struct {
			ValidatedSignals []dashboardhome.DashboardChunkSignal `json:"validated_signals"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(store.sharedChunkArtifacts()[0].PayloadJson, &artifact); err != nil {
		t.Fatal(err)
	}
	artifactIDs := make([]string, 0, len(artifact.Payload.ValidatedSignals))
	for _, sig := range artifact.Payload.ValidatedSignals {
		artifactIDs = append(artifactIDs, sig.SignalID)
	}
	sort.Strings(artifactIDs)
	if len(artifactIDs) == 0 {
		t.Fatal("expected validated signals in shared artifact")
	}
	if len(ai.lastSynthesisSignalIDs) != len(artifactIDs) {
		t.Fatalf("synthesis signals=%v artifact=%v", ai.lastSynthesisSignalIDs, artifactIDs)
	}
	for i := range artifactIDs {
		if ai.lastSynthesisSignalIDs[i] != artifactIDs[i] {
			t.Fatalf("signal mismatch synthesis=%v artifact=%v", ai.lastSynthesisSignalIDs, artifactIDs)
		}
	}
}

func countEvents(events []string, name string) int {
	n := 0
	for _, ev := range events {
		if ev == name {
			n++
		}
	}
	return n
}

var _ aipipeline.DashboardRunner = (*precomputedSynthesisDashboardRunner)(nil)
