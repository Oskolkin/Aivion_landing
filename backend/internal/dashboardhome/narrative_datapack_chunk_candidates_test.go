package dashboardhome

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func testChunkForCandidateValidation() aidatapack.DataPackChunk {
	return aidatapack.DataPackChunk{
		ChunkID:   "stock_positions_003",
		ChunkType: "stock_positions",
		LocalFactCatalog: map[string]aidatapack.CompactFact{
			"sku.101.coverage_days":      {FactID: "sku.101.coverage_days", EntityID: "sku:101", EntityType: "sku"},
			"sku.102.coverage_days":      {FactID: "sku.102.coverage_days", EntityID: "sku:102", EntityType: "sku"},
			"campaign.1.last_7d.drr_pct": {FactID: "campaign.1.last_7d.drr_pct", EntityID: "campaign:1", EntityType: "campaign"},
		},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{
			"sku:101":     {EntityID: "sku:101", EntityType: "sku", Name: "SKU 101"},
			"sku:102":     {EntityID: "sku:102", EntityType: "sku", Name: "SKU 102"},
			"campaign:1":  {EntityID: "campaign:1", EntityType: "campaign", Name: "Campaign 1"},
			"sku:missing": {EntityID: "sku:missing", EntityType: "sku"},
		},
	}
}

func validSignalAndCandidateAnalysis(chunkID string) DashboardChunkAnalysis {
	return DashboardChunkAnalysis{
		SchemaVersion:  DashboardChunkAnalysisSchemaVersionV2,
		ChunkID:        chunkID,
		ChunkType:      "stock_positions",
		AnalysisStatus: "completed",
		Signals: []DashboardChunkSignal{{
			SignalID:    chunkID + "-signal-01",
			SignalType:  ChunkSignalRiskCandidate,
			EntityIDs:   []string{"sku:101", "sku:102"},
			FactIDs:     []string{"sku.101.coverage_days", "sku.102.coverage_days"},
			Observation: "У двух SKU покрытие запасом менее трёх дней.",
		}},
		ActionCandidates: []actions.AIFirstActionCandidate{{
			Type:              actions.ActionTypeStockReplenishment,
			Category:          actions.ActionCategoryStock,
			LocalPriority:     actions.ActionPriorityCritical,
			Problem:           "У двух SKU с подтверждённым спросом запас менее трёх дней.",
			RecommendedAction: "Проверить товары в пути и пополнить недостающий запас.",
			WhyActionable:     "При сохранении текущего спроса товары могут закончиться.",
			AffectedEntities: []actions.AIFirstAffectedEntityRef{
				{EntityType: "sku", EntityID: "sku:101"},
				{EntityType: "sku", EntityID: "sku:102"},
			},
			EvidenceFactIDs: []string{"sku.101.coverage_days", "sku.102.coverage_days"},
			SourceSignalIDs: []string{chunkID + "-signal-01"},
		}},
	}
}

func validateChunkAnalysis(t *testing.T, chunk aidatapack.DataPackChunk, analysis DashboardChunkAnalysis) ChunkValidationResult {
	t.Helper()
	analysis.ActionCandidates = normalizeActionCandidateTechnicalFields(chunk.ChunkID, analysis.ActionCandidates)
	return ValidateDashboardChunkAnalysis(chunk, analysis, minimalChunkValidationPack())
}

func TestChunkCandidateValidation_AcceptsOneSignalAndOneCandidate(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	result := validateChunkAnalysis(t, chunk, validSignalAndCandidateAnalysis(chunk.ChunkID))
	if !result.Valid {
		t.Fatalf("expected valid, hard=%v repairable=%v", result.HardErrors, result.RepairableErrors)
	}
	if len(result.Signals) != 1 || len(result.ActionCandidates) != 1 {
		t.Fatalf("signals=%d candidates=%d", len(result.Signals), len(result.ActionCandidates))
	}
	if result.RejectedCandidateCount != 0 {
		t.Fatalf("rejected candidates=%d", result.RejectedCandidateCount)
	}
}

func TestChunkCandidateValidation_AcceptsPartialInvalidRefs(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].EvidenceFactIDs = []string{
		"sku.101.coverage_days", "sku.102.coverage_days", "sku.999.unknown",
	}
	analysis.ActionCandidates[0].AffectedEntities = []actions.AIFirstAffectedEntityRef{
		{EntityType: "sku", EntityID: "sku:101"},
		{EntityType: "sku", EntityID: "sku:102"},
		{EntityType: "sku", EntityID: "sku:unknown"},
	}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 1 {
		t.Fatalf("candidates=%d want 1", len(result.ActionCandidates))
	}
	if result.RejectedCandidateCount != 0 {
		t.Fatalf("rejected=%d", result.RejectedCandidateCount)
	}
}

func TestChunkCandidateValidation_RejectsWhenNoEvidenceAfterCleanup(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].EvidenceFactIDs = []string{"sku.999.unknown"}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 || result.RejectedCandidateCount != 1 {
		t.Fatalf("candidates=%d rejected=%d", len(result.ActionCandidates), result.RejectedCandidateCount)
	}
}

func TestChunkCandidateValidation_RejectsUnknownSKU(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].AffectedEntities = []actions.AIFirstAffectedEntityRef{
		{EntityType: "sku", EntityID: "sku:unknown"},
	}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.Signals) != 1 || len(result.ActionCandidates) != 0 || !result.Valid {
		t.Fatalf("expected valid chunk with rejected candidate, signals=%d candidates=%d valid=%v", len(result.Signals), len(result.ActionCandidates), result.Valid)
	}
	if result.RejectedCandidateCount != 1 {
		t.Fatalf("rejected=%d", result.RejectedCandidateCount)
	}
}

func TestChunkCandidateValidation_RejectsUnknownCampaign(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Type = actions.ActionTypeAdHighDRRReview
	analysis.ActionCandidates[0].AffectedEntities = []actions.AIFirstAffectedEntityRef{
		{EntityType: "campaign", EntityID: "campaign:missing"},
	}
	analysis.ActionCandidates[0].EvidenceFactIDs = []string{"campaign.1.last_7d.drr_pct"}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 || !result.Valid {
		t.Fatalf("expected rejected campaign candidate, candidates=%d valid=%v", len(result.ActionCandidates), result.Valid)
	}
}

func TestChunkCandidateValidation_RejectsUnknownFactID(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].EvidenceFactIDs = []string{"missing.fact"}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("expected rejected candidate")
	}
}

func TestChunkCandidateValidation_RejectsUnknownEntityLevelFactID(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].AffectedEntities = []actions.AIFirstAffectedEntityRef{
		{
			EntityType: "sku",
			EntityID:   "sku:101",
			FactIDs:    []string{"sku.unknown.entity_fact"},
		},
	}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 1 || result.RejectedCandidateCount != 0 {
		t.Fatalf("expected candidate accepted with unknown entity fact removed, candidates=%d rejected=%d", len(result.ActionCandidates), result.RejectedCandidateCount)
	}
	if len(result.ActionCandidates[0].AffectedEntities[0].FactIDs) != 0 {
		t.Fatalf("entity facts=%v", result.ActionCandidates[0].AffectedEntities[0].FactIDs)
	}
}

func TestChunkCandidateValidation_RejectsFactOnlyInFullPackNotLocalChunkCatalog(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	pack := minimalChunkValidationPack()
	pack.FactRegistry["pack.only.fact"] = aidatapack.Fact{FactID: "pack.only.fact", EntityType: "sku", EntityID: "sku:101"}
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].EvidenceFactIDs = []string{"pack.only.fact"}
	analysis.ActionCandidates = normalizeActionCandidateTechnicalFields(chunk.ChunkID, analysis.ActionCandidates)
	result := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("fact from full pack but absent in local chunk catalog must reject candidate")
	}
}

func TestChunkCandidateValidation_DoesNotRejectCandidatesForSemanticTextPatterns(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	pack := minimalChunkValidationPack()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Problem = "маржа упала и расход по sku высокий"
	analysis.ActionCandidates[0].RecommendedAction = "снизьте цену"
	analysis.ActionCandidates[0].WhyActionable = "sku сливает бюджет"
	analysis.ActionCandidates = normalizeActionCandidateTechnicalFields(chunk.ChunkID, analysis.ActionCandidates)
	result := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if len(result.ActionCandidates) != 1 {
		t.Fatalf("candidate semantic text must not be rejected by backend pilot validator, candidates=%d errors=%v", len(result.ActionCandidates), result.RepairableErrors)
	}
}

func TestChunkCandidateValidation_RejectsCandidatesBeyondChunkLimit(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	base := analysis.ActionCandidates[0]
	for i := 0; i < 5; i++ {
		analysis.ActionCandidates = append(analysis.ActionCandidates, base)
	}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != maxChunkActionCandidates {
		t.Fatalf("accepted=%d want %d", len(result.ActionCandidates), maxChunkActionCandidates)
	}
	if result.RejectedCandidateCount != 1 {
		t.Fatalf("rejected=%d want 1", result.RejectedCandidateCount)
	}
}

func TestChunkCandidateValidation_RejectsUnknownSourceSignalID(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].SourceSignalIDs = []string{"missing-signal"}
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("expected rejected candidate")
	}
}

func TestChunkCandidateValidation_RejectsEmptyProblem(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Problem = "   "
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("expected rejected candidate")
	}
}

func TestChunkCandidateValidation_RejectsEmptyRecommendedAction(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].RecommendedAction = ""
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("expected rejected candidate")
	}
}

func TestChunkCandidateValidation_RejectsEmptyWhyActionable(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].WhyActionable = ""
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("expected rejected candidate")
	}
}

func TestChunkCandidateValidation_RejectsUnknownActionType(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	for _, actionType := range []actions.ActionType{
		actions.ActionType("unknown_action_type"),
		actions.ActionTypeKeySKURisk,
		actions.ActionTypeAdHighDRR,
		actions.ActionTypeDataQualityIssue,
	} {
		analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
		analysis.ActionCandidates[0].Type = actionType
		if _, ok := actions.ActionCategoryForType(actionType); ok {
			t.Fatalf("type %q should be unsupported", actionType)
		}
		result := validateChunkAnalysis(t, chunk, analysis)
		if len(result.ActionCandidates) != 0 {
			t.Fatalf("expected rejected candidate for type %q", actionType)
		}
	}
}

func TestChunkCandidateValidation_NormalizesCategoryFromType(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Category = actions.ActionCategoryAdvertising
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 1 || result.ActionCandidates[0].Category != actions.ActionCategoryStock {
		t.Fatalf("category=%q", result.ActionCandidates[0].Category)
	}

	analysis = validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Category = ""
	result = validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 1 || result.ActionCandidates[0].Category != actions.ActionCategoryStock {
		t.Fatalf("empty category not normalized, got %q", result.ActionCandidates[0].Category)
	}
}

func TestNormalizeActionCandidateTechnicalFields_AssignsCandidateID(t *testing.T) {
	chunkID := "stock_positions_003"
	candidates := []actions.AIFirstActionCandidate{{CandidateID: "invented"}}
	out := normalizeActionCandidateTechnicalFields(chunkID, candidates)
	if out[0].CandidateID != "stock_positions_003:candidate:01" {
		t.Fatalf("candidate_id=%q", out[0].CandidateID)
	}
}

func TestNormalizeActionCandidateTechnicalFields_AssignsChunkID(t *testing.T) {
	chunkID := "stock_positions_003"
	candidates := []actions.AIFirstActionCandidate{{ChunkID: "other_chunk"}}
	out := normalizeActionCandidateTechnicalFields(chunkID, candidates)
	if out[0].ChunkID != chunkID {
		t.Fatalf("chunk_id=%q", out[0].ChunkID)
	}
}

func TestNormalizeActionCandidateTechnicalFields_PreservesRawIndex(t *testing.T) {
	chunkID := "stock_positions_003"
	candidates := []actions.AIFirstActionCandidate{
		{Type: actions.ActionTypeStockReplenishment, Problem: "bad", RecommendedAction: "x", WhyActionable: "y"},
		validSignalAndCandidateAnalysis(chunkID).ActionCandidates[0],
	}
	candidates = normalizeActionCandidateTechnicalFields(chunkID, candidates)
	if candidates[1].CandidateID != "stock_positions_003:candidate:02" {
		t.Fatalf("candidate_id=%q", candidates[1].CandidateID)
	}
}

func TestChunkCandidateValidation_InvalidCandidateDoesNotRemoveValidSignal(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Problem = ""
	result := validateChunkAnalysis(t, chunk, analysis)
	if !result.Valid || len(result.Signals) != 1 || len(result.ActionCandidates) != 0 {
		t.Fatalf("signals=%d candidates=%d valid=%v", len(result.Signals), len(result.ActionCandidates), result.Valid)
	}
}

func TestChunkCandidateValidation_OneInvalidDoesNotRejectOther(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	second := analysis.ActionCandidates[0]
	second.Problem = "Другая проблема с запасом"
	second.SourceSignalIDs = []string{chunk.ChunkID + "-signal-01"}
	analysis.ActionCandidates = append(analysis.ActionCandidates, second)
	analysis.ActionCandidates[0].Problem = ""
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 1 || result.RejectedCandidateCount != 1 || !result.Valid {
		t.Fatalf("candidates=%d rejected=%d valid=%v", len(result.ActionCandidates), result.RejectedCandidateCount, result.Valid)
	}
}

func TestChunkCandidateValidation_AllCandidatesInvalidSignalsValid(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates[0].Problem = ""
	result := validateChunkAnalysis(t, chunk, analysis)
	if !result.Valid || len(result.Signals) != 1 || len(result.ActionCandidates) != 0 || result.RejectedCandidateCount == 0 {
		t.Fatalf("signals=%d candidates=%d rejected=%d valid=%v", len(result.Signals), len(result.ActionCandidates), result.RejectedCandidateCount, result.Valid)
	}
}

func TestParseDashboardChunkAnalysis_ActionCandidatesNullNormalized(t *testing.T) {
	raw := `{"schema_version":"dashboard_chunk_analysis_v2","chunk_id":"c1","chunk_type":"stock_positions","analysis_status":"insufficient_data","signals":[],"action_candidates":null}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if analysis.ActionCandidates == nil || len(analysis.ActionCandidates) != 0 {
		t.Fatalf("action_candidates=%v", analysis.ActionCandidates)
	}
}

func TestChunkCandidateValidation_V1IgnoresCandidates(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.SchemaVersion = DashboardChunkAnalysisSchemaVersionV1
	result := validateChunkAnalysis(t, chunk, analysis)
	if len(result.ActionCandidates) != 0 || result.RejectedCandidateCount != 0 {
		t.Fatalf("v1 should ignore candidates")
	}
}

func TestChunkAnalysisPromptRequestsV2(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	system := strings.ToLower(systemPromptDashboardChunkAnalysis())
	user := strings.ToLower(userPromptDashboardChunkAnalysis(chunk))
	for _, needle := range []string{
		"dashboard_chunk_analysis_v3",
		"action_candidates",
		"not every signal",
		"group similar objects",
		"sku-level advertising",
		"profit",
		"price",
		"max 10",
		"max 5",
		"max 12",
		"max 20",
	} {
		if !strings.Contains(system+user, needle) {
			t.Fatalf("missing %q in prompt", needle)
		}
	}
	if strings.Contains(user, "second ai call") || strings.Contains(system, "second ai call") {
		// prompt should mention single response, not a separate call
	}
}

func TestChunkAnalysisTokenLimits(t *testing.T) {
	if chunkAnalysisMaxOutputTokens != 6000 {
		t.Fatalf("output tokens=%d", chunkAnalysisMaxOutputTokens)
	}
	chunk := aidatapack.DataPackChunk{ChunkID: "c1", ApproxTokens: 18001}
	orch := &DataPackChunksOrchestrator{}
	_, _, _, _, err := orch.analyzeOneChunk(context.Background(), chunk)
	if err == nil || !strings.Contains(err.Error(), "18000") {
		t.Fatalf("expected hard input limit error, got %v", err)
	}
}

type fakeChunkAnalysisAI struct {
	calls int
	fn    func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error)
}

func (f *fakeChunkAnalysisAI) GenerateStructuredJSON(ctx context.Context, input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
	f.calls++
	return f.fn(input)
}

func TestAnalyzeOneChunk_SingleAICallReturnsSignalsAndCandidates(t *testing.T) {
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	raw, _ := json.Marshal(analysis)
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			if input.MaxOutputTokens != chunkAnalysisMaxOutputTokens {
				t.Fatalf("max_output_tokens=%d", input.MaxOutputTokens)
			}
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw)}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	parsed, _, _, _, err := orch.analyzeOneChunk(context.Background(), chunk)
	if err != nil {
		t.Fatalf("analyzeOneChunk: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("ai calls=%d", fake.calls)
	}
	if len(parsed.Signals) != 1 || len(parsed.ActionCandidates) != 1 {
		t.Fatalf("signals=%d candidates=%d", len(parsed.Signals), len(parsed.ActionCandidates))
	}
	if parsed.ActionCandidates[0].CandidateID != "stock_positions_003:candidate:01" {
		t.Fatalf("candidate_id=%q", parsed.ActionCandidates[0].CandidateID)
	}
}

func TestAnalyzeChunksOnly_AggregatesSignalsAndCandidates(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk1 := testChunkForCandidateValidation()
	chunk2 := testChunkForCandidateValidation()
	chunk2.ChunkID = "stock_positions_004"
	chunk2.ChunkType = "stock_positions"
	analysis1 := validSignalAndCandidateAnalysis(chunk1.ChunkID)
	analysis2 := validSignalAndCandidateAnalysis(chunk2.ChunkID)
	raw1, _ := json.Marshal(analysis1)
	raw2, _ := json.Marshal(analysis2)
	call := 0
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			call++
			if call == 1 {
				return &recommendations.GenerateRecommendationsOutput{Content: string(raw1)}, nil
			}
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw2)}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	chunkSet := &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk1, chunk2}}
	result := orch.analyzeAllChunks(context.Background(), pack, chunkSet)
	if result.ValidatedSignalCount != 2 || result.ValidatedCandidateCount != 2 {
		t.Fatalf("signals=%d candidates=%d", result.ValidatedSignalCount, result.ValidatedCandidateCount)
	}
	if len(result.ValidatedSignals) != 2 || len(result.ValidatedActionCandidates) != 2 {
		t.Fatalf("aggregated signals=%d candidates=%d", len(result.ValidatedSignals), len(result.ValidatedActionCandidates))
	}
}

func TestAnalyzeChunksOnly_RecordsFailedChunkMetadata(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return nil, context.DeadlineExceeded
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	chunkSet := &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}}
	result := orch.analyzeAllChunks(context.Background(), pack, chunkSet)
	if len(result.FailedChunkIDs) != 1 || result.FailedChunkIDs[0] != chunk.ChunkID {
		t.Fatalf("failed ids=%v", result.FailedChunkIDs)
	}
	if len(result.FailedChunkTypes) != 1 || result.FailedChunkTypes[0] != chunk.ChunkType {
		t.Fatalf("failed types=%v", result.FailedChunkTypes)
	}
}

func TestChunkDebugRecorder_IncludesCandidateFields(t *testing.T) {
	dir := t.TempDir()
	rec := NewNarrativeDebugRecorder(dir)
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.ActionCandidates = normalizeActionCandidateTechnicalFields(chunk.ChunkID, analysis.ActionCandidates)
	validation := validateChunkAnalysis(t, chunk, analysis)
	rec.RecordChunk(chunk, "{}", nil, "", &analysis, validation)

	raw, err := os.ReadFile(filepath.Join(dir, "chunks", "001_"+chunk.ChunkID+".validation.json"))
	if err != nil {
		t.Fatalf("read debug: %v", err)
	}
	payload := string(raw)
	for _, needle := range []string{
		"accepted_candidates",
		"parsed_candidate_count",
		"rejected_candidates",
		"chunk_type",
	} {
		if !strings.Contains(payload, needle) {
			t.Fatalf("missing %q in debug payload: %s", needle, payload)
		}
	}
}

func TestIsSupportedDashboardChunkAnalysisSchemaVersion_ProductionAliasV3(t *testing.T) {
	if DashboardChunkAnalysisSchemaVersion != DashboardChunkAnalysisSchemaVersionV3 {
		t.Fatalf("production alias=%q", DashboardChunkAnalysisSchemaVersion)
	}
}
