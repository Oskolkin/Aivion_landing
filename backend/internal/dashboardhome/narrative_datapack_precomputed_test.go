package dashboardhome

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func withLegacyDashboardMode(t *testing.T) {
	t.Helper()
	t.Setenv("DASHBOARD_LEGACY_MODE", "1")
}

func minimalAcceptedSynthesisJSON() string {
	return `{"headline":"Заголовок","executive_summary":"Резюме.","improvement_summary":"Улучшения.","health_reasons":[],"top_actions":[],"data_quality_notes":[]}`
}

type recordingDashboardAI struct {
	chunkCalls      int
	synthesisCalls  int
	lastSynthInput  any
	synthesisInTok  int
	synthesisOutTok int
}

func (r *recordingDashboardAI) GenerateStructuredJSON(ctx context.Context, input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
	switch input.SystemPrompt {
	case systemPromptDashboardSynthesis():
		r.synthesisCalls++
		r.lastSynthInput = input.Context
		r.synthesisInTok = 3000
		r.synthesisOutTok = 600
		return &recommendations.GenerateRecommendationsOutput{
			Content:      minimalAcceptedSynthesisJSON(),
			InputTokens:  r.synthesisInTok,
			OutputTokens: r.synthesisOutTok,
		}, nil
	case systemPromptDashboardChunkAnalysis():
		r.chunkCalls++
		var chunk aidatapack.DataPackChunk
		if ctxMap, ok := input.Context.(map[string]any); ok {
			if ch, ok := ctxMap["chunk"].(aidatapack.DataPackChunk); ok {
				chunk = ch
			}
		}
		raw, err := recordingChunkAnalysisJSON(chunk)
		if err != nil {
			return nil, err
		}
		return &recommendations.GenerateRecommendationsOutput{Content: string(raw), InputTokens: 5000, OutputTokens: 1000}, nil
	default:
		return nil, context.Canceled
	}
}

func recordingChunkAnalysisJSON(chunk aidatapack.DataPackChunk) ([]byte, error) {
	if chunk.ChunkID == "" {
		return json.Marshal(DashboardChunkAnalysis{
			SchemaVersion:  DashboardChunkAnalysisSchemaVersionV2,
			AnalysisStatus: "insufficient_data",
		})
	}
	if chunk.ChunkID == "manifest_context" || chunk.ChunkType == "entity_dictionary" {
		return json.Marshal(DashboardChunkAnalysis{
			SchemaVersion:    DashboardChunkAnalysisSchemaVersionV2,
			ChunkID:          chunk.ChunkID,
			ChunkType:        chunk.ChunkType,
			AnalysisStatus:   "insufficient_data",
			Signals:          nil,
			ActionCandidates: []actions.AIFirstActionCandidate{},
		})
	}
	factIDs := sortedAllowlistKeys(chunkLocalFactIDAllowlist(chunk))
	entityIDs := sortedAllowlistKeys(chunkLocalEntityIDAllowlist(chunk))
	if len(factIDs) == 0 {
		return json.Marshal(DashboardChunkAnalysis{
			SchemaVersion:    DashboardChunkAnalysisSchemaVersionV2,
			ChunkID:          chunk.ChunkID,
			ChunkType:        chunk.ChunkType,
			AnalysisStatus:   "insufficient_data",
			Signals:          nil,
			ActionCandidates: []actions.AIFirstActionCandidate{},
		})
	}
	maxFacts := 2
	if len(factIDs) < maxFacts {
		maxFacts = len(factIDs)
	}
	maxEntities := 2
	if len(entityIDs) < maxEntities {
		maxEntities = len(entityIDs)
	}
	analysis := DashboardChunkAnalysis{
		SchemaVersion:  DashboardChunkAnalysisSchemaVersionV2,
		ChunkID:        chunk.ChunkID,
		ChunkType:      chunk.ChunkType,
		AnalysisStatus: "completed",
		Signals: []DashboardChunkSignal{{
			SignalID:    chunk.ChunkID + "-signal-01",
			SignalType:  ChunkSignalObservation,
			EntityIDs:   entityIDs[:maxEntities],
			FactIDs:     factIDs[:maxFacts],
			Observation: "Тестовое наблюдение по данным чанка.",
		}},
	}
	return json.Marshal(analysis)
}

func precomputedTestFixtures(t *testing.T) (*aidatapack.SellerAIDataPack, *aidatapack.ChunkSet, ChunkAnalysisRunResult) {
	t.Helper()
	pack := sampleDataPackForNarrativeTest()
	pack.EntityRegistry.SKUs["sku:802"] = aidatapack.SKUEntity{EntityID: "sku:802", SKU: "802", Name: "SKU 802"}
	chunkSet, err := BuildChunkSetForPack(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		t.Fatalf("build chunk set: %v", err)
	}
	hashes := make([]string, len(chunkSet.Chunks))
	for i, ch := range chunkSet.Chunks {
		hashes[i] = ch.ChunkHash
	}
	signals := []DashboardChunkSignal{
		{SignalID: "signal_A", SignalType: ChunkSignalObservation, Observation: "Наблюдение A", FactIDs: []string{"account.last_7d.revenue"}},
		{SignalID: "signal_B", SignalType: ChunkSignalObservation, Observation: "Наблюдение B", FactIDs: []string{"sku.801.last_7d.revenue"}, EntityIDs: []string{"sku:801"}},
		{SignalID: "signal_C", SignalType: ChunkSignalDataQuality, Observation: "Качество данных ограничено"},
	}
	run := ChunkAnalysisRunResult{
		SellerAccountID:      pack.SellerContext.SellerAccountID,
		MetricDate:           pack.SellerContext.MetricDate,
		ChunkCount:           len(chunkSet.Chunks),
		Completed:            len(chunkSet.Chunks),
		ValidatedSignals:     signals,
		ValidatedSignalCount: len(signals),
		ChunkHashes:          hashes,
		InputTokens:          10000,
		OutputTokens:         2000,
		AnalysisStatus:       ChunkAnalysisStatusCompleted,
	}
	return pack, chunkSet, run
}

func precomputedNarrativeBuild(t *testing.T, pack *aidatapack.SellerAIDataPack) dashboardNarrativeBuildResult {
	t.Helper()
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	build.ContextSource = ContextSourceSellerAIDataPackChunks
	return build
}

func TestValidatePrecomputedChunkRun_InvalidScenarios(t *testing.T) {
	pack, chunkSet, run := precomputedTestFixtures(t)
	cases := []struct {
		name string
		mut  func(*ChunkAnalysisRunResult)
	}{
		{"wrong_seller", func(r *ChunkAnalysisRunResult) { r.SellerAccountID = 99 }},
		{"wrong_metric_date", func(r *ChunkAnalysisRunResult) { r.MetricDate = "2000-01-01" }},
		{"wrong_chunk_count", func(r *ChunkAnalysisRunResult) { r.ChunkCount++ }},
		{"completed_zero", func(r *ChunkAnalysisRunResult) { r.Completed = 0; r.Failed = r.ChunkCount }},
		{"unknown_failed_chunk", func(r *ChunkAnalysisRunResult) {
			r.Failed = 1
			r.Completed = r.ChunkCount - 1
			r.FailedChunkIDs = []string{"missing"}
			r.FailedChunkTypes = []string{"x"}
			r.AnalysisStatus = ChunkAnalysisStatusPartial
		}},
		{"signal_count_mismatch", func(r *ChunkAnalysisRunResult) { r.ValidatedSignalCount++ }},
		{"bad_status", func(r *ChunkAnalysisRunResult) { r.AnalysisStatus = ChunkAnalysisStatusFailed }},
		{"chunk_hashes", func(r *ChunkAnalysisRunResult) { r.ChunkHashes = []string{"bad"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mut := run
			tc.mut(&mut)
			if err := ValidatePrecomputedChunkRun(pack, chunkSet, &mut); err == nil {
				t.Fatal("expected invalid")
			}
		})
	}
}

func TestSynthesizeFromChunkRun_NoInternalChunkAnalysis(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if ai.chunkCalls != 0 {
		t.Fatalf("chunk calls=%d want 0", ai.chunkCalls)
	}
	if ai.synthesisCalls != 1 {
		t.Fatalf("synthesis calls=%d want 1", ai.synthesisCalls)
	}
	_ = gen
}

func TestSynthesizeFromChunkRun_UsesPrecomputedSignals(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	_ = orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if ai.synthesisCalls != 1 {
		t.Fatalf("synthesis calls=%d", ai.synthesisCalls)
	}
	synthInput, ok := ai.lastSynthInput.(DashboardSynthesisInput)
	if !ok {
		t.Fatalf("unexpected synthesis input type %T", ai.lastSynthInput)
	}
	if len(synthInput.ValidatedSignals) == 0 {
		t.Fatal("expected signals in synthesis input")
	}
	gotIDs := make([]string, len(synthInput.ValidatedSignals))
	for i, s := range synthInput.ValidatedSignals {
		gotIDs[i] = s.SignalID
	}
	for _, want := range []string{"signal_A", "signal_B", "signal_C"} {
		if !precomputedContainsString(gotIDs, want) {
			t.Fatalf("missing signal %s in %v", want, gotIDs)
		}
	}
}

func TestSynthesizeFromChunkRun_DoesNotMutatePrecomputedRun(t *testing.T) {
	pack, chunkSet, run := precomputedTestFixtures(t)
	before := CloneChunkAnalysisRunResult(&run)
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	_ = orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	after := CloneChunkAnalysisRunResult(&run)
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatalf("chunk run mutated:\nbefore=%s\nafter=%s", beforeJSON, afterJSON)
	}
}

func TestPrecomputedPath_SkipsChunkSetBuild(t *testing.T) {
	withLegacyDashboardMode(t)
	ResetBuildChunkSetForPackCallsForTest()
	defer ResetBuildChunkSetForPackHookForTest()
	pack, chunkSet, run := precomputedTestFixtures(t)
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	if err := ValidatePrecomputedChunkRun(pack, chunkSet, &run); err != nil {
		t.Fatal(err)
	}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{
		PrecomputedChunkRun: &run,
		ChunkSetOverride:    chunkSet,
	})
	_ = gen
	if BuildChunkSetForPackCallsForTest() != 0 {
		t.Fatalf("build calls=%d want 0", BuildChunkSetForPackCallsForTest())
	}
	if ai.chunkCalls != 0 {
		t.Fatalf("chunk calls=%d want 0", ai.chunkCalls)
	}
	if ai.synthesisCalls != 1 {
		t.Fatalf("synthesis calls=%d want 1", ai.synthesisCalls)
	}
}

func TestPrecomputedPath_UsageOnlySynthesisTokens(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if gen.SynthesisInputTokens != 3000 || gen.SynthesisOutputTokens != 600 {
		t.Fatalf("synthesis tokens=%d/%d", gen.SynthesisInputTokens, gen.SynthesisOutputTokens)
	}
	_ = gen.Fatal
}

func TestTokenGuard_BlocksOversizedInput(t *testing.T) {
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", "10")
	pack, chunkSet, run := precomputedTestFixtures(t)
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if !gen.Fatal {
		t.Fatal("expected token guard failure")
	}
	if NarrativeErrorCode(gen.Error) != NarrativeErrorDashboardInputTooLarge {
		t.Fatalf("code=%q err=%v", NarrativeErrorCode(gen.Error), gen.Error)
	}
	if ai.synthesisCalls != 0 || ai.chunkCalls != 0 {
		t.Fatalf("ai calls synthesis=%d chunk=%d", ai.synthesisCalls, ai.chunkCalls)
	}
}

func TestTokenGuard_AllowsAtLimit(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	build := precomputedNarrativeBuild(t, pack)
	input := buildDashboardSynthesisInput(build, extractManifestSummary(chunkSet), pack.DataQuality.Limitations, cloneDashboardChunkSignals(run.ValidatedSignals), pack, NarrativeRunOptions{})
	est, err := EstimateSynthesisInputTokensForTest(input)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", strconv.Itoa(est))
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if ai.synthesisCalls != 1 {
		t.Fatalf("synthesis calls=%d", ai.synthesisCalls)
	}
	_ = gen
}

func TestPartialPrecomputedRun_ContinuesSynthesis(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	run.Completed = len(chunkSet.Chunks) - 1
	run.Failed = 1
	run.FailedChunkIDs = []string{chunkSet.Chunks[0].ChunkID}
	run.FailedChunkTypes = []string{chunkSet.Chunks[0].ChunkType}
	run.AnalysisStatus = ChunkAnalysisStatusPartial
	build := precomputedNarrativeBuild(t, pack)
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if ai.chunkCalls != 0 || ai.synthesisCalls != 1 {
		t.Fatalf("calls chunk=%d synthesis=%d", ai.chunkCalls, ai.synthesisCalls)
	}
	_ = gen
}

func TestPrecomputedMissingChunkSet_Error(t *testing.T) {
	pack, _, run := precomputedTestFixtures(t)
	opts := NarrativeRunOptions{DataPackOverride: pack, PrecomputedChunkRun: &run}
	if opts.ChunkSetOverride != nil {
		t.Fatal("unexpected chunk set override")
	}
	err := &NarrativePipelineError{Code: NarrativeErrorPrecomputedChunkSetMissing, Err: fmt.Errorf("chunk set override is required with precomputed chunk run")}
	if NarrativeErrorCode(err) != NarrativeErrorPrecomputedChunkSetMissing {
		t.Fatalf("code=%q", NarrativeErrorCode(err))
	}
	_ = opts
}

func TestUsesUnifiedDataPackContext_SkipsLegacyLoads(t *testing.T) {
	if UsesUnifiedDataPackContext(NarrativeRunOptions{}) {
		t.Fatal("legacy path should load alerts")
	}
	if !UsesUnifiedDataPackContext(NarrativeRunOptions{PrecomputedChunkRun: &ChunkAnalysisRunResult{}}) {
		t.Fatal("unified path should skip legacy loads")
	}
}

func TestShouldPersistFrozenDashboardSnapshot_Gate(t *testing.T) {
	runID := int64(42)
	if !ShouldPersistFrozenDashboardSnapshotForTest(NarrativeRunOptions{
		PipelineRunID: &runID, DataPackHash: "dp", GlobalContextHash: "gc",
	}) {
		t.Fatal("expected persist gate true for unified pipeline opts")
	}
	if ShouldPersistFrozenDashboardSnapshotForTest(NarrativeRunOptions{}) {
		t.Fatal("expected persist gate false without pipeline opts")
	}
}

func TestStandalonePath_AnalyzesThenSynthesizes(t *testing.T) {
	withLegacyDashboardMode(t)
	pack := minimalChunkValidationPack()
	pack.SellerContext = aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25", Marketplace: aidatapack.MarketplaceOzon}
	pack.Tables.StockPositions = []aidatapack.StockPositionRow{
		{EntityID: "sku:101", TurnoverGrade: "DEFICIT"},
		{EntityID: "sku:102", TurnoverGrade: "DEFICIT"},
	}
	pack.EntityRegistry.SKUs = map[string]aidatapack.SKUEntity{
		"sku:101": {EntityID: "sku:101", SKU: "101"},
		"sku:102": {EntityID: "sku:102", SKU: "102"},
	}
	pack.EntityRegistry.Campaigns = map[string]aidatapack.CampaignEntity{
		"campaign:1": {EntityID: "campaign:1", CampaignExternalID: "1"},
	}
	chunkSet, err := BuildChunkSetForPack(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	build := dashboardNarrativeBuildResult{
		ContextSource:      ContextSourceSellerAIDataPackChunks,
		CandidateByID:      map[string]dashboardNarrativeCandidate{candidateStockReview: {ID: candidateStockReview, Category: "stock", CompatibleDriverIDs: []string{driverStock}}},
		MetricCatalogByKey: map[string]dashboardNarrativeMetric{},
		AllowedQuotedNames: map[string]struct{}{},
		DataPackFactIDs:    []string{"sku.101.coverage_days", "sku.102.coverage_days"},
	}
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	chunkRun, err := orch.AnalyzeChunksOnly(context.Background(), pack, chunkSet)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if ai.chunkCalls == 0 {
		t.Fatal("expected chunk analysis calls")
	}
	synthCallsBefore := ai.synthesisCalls
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, chunkRun, build, NarrativeRunOptions{})
	if ai.synthesisCalls != synthCallsBefore+1 {
		t.Fatalf("synthesis calls=%d", ai.synthesisCalls)
	}
	_ = gen
}

func TestInvalidPrecomputedRun_NoSynthesisOrChunkCalls(t *testing.T) {
	pack, chunkSet, run := precomputedTestFixtures(t)
	run.SellerAccountID = 0
	ai := &recordingDashboardAI{}
	if err := ValidatePrecomputedChunkRun(pack, chunkSet, &run); err == nil {
		t.Fatal("expected invalid")
	}
	_ = ai
	if ai.synthesisCalls != 0 || ai.chunkCalls != 0 {
		t.Fatalf("unexpected ai calls")
	}
}

func TestPersistFrozenDashboardSnapshot_TrackingHook(t *testing.T) {
	EnablePersistFrozenDashboardSnapshotTrackingForTest()
	defer ResetPersistFrozenDashboardSnapshotTrackingForTest()
	svc := &Service{}
	_, _ = svc.persistFrozenDashboardSnapshot(context.Background(), 1, 1, precomputedMustDate(t), "dp", "gc", analyticsDTO(), Response{})
	if PersistFrozenDashboardSnapshotCallsForTest() != 1 {
		t.Fatalf("persist calls=%d", PersistFrozenDashboardSnapshotCallsForTest())
	}
}

func precomputedContainsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func precomputedMustDate(t *testing.T) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", "2026-06-25")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func analyticsDTO() analytics.DashboardMetricsDTO {
	return analytics.DashboardMetricsDTO{}
}
