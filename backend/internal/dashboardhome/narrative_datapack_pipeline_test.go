package dashboardhome

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestGenerationStatusShowsFallbackExplicitly(t *testing.T) {
	r := &NarrativeGenerationResult{
		Status:               "fallback",
		UsedLegacyFallback:   true,
		FinalPayloadSource:   FinalPayloadSourceLegacyDashboard,
		ChunkSynthesisStatus: ChunkSynthesisStatusFallback,
	}
	if r.IsSuccess() {
		t.Fatal("fallback must not be success")
	}
	if r.DevExitError() == nil {
		t.Fatal("expected dev exit error")
	}
}

func TestCompletedDoesNotHideLegacyFallback(t *testing.T) {
	r := &NarrativeGenerationResult{
		Status:             "completed",
		UsedLegacyFallback: true,
		FinalPayloadSource: FinalPayloadSourceLegacyDashboard,
	}
	if r.IsSuccess() {
		t.Fatal("completed with legacy fallback must not be success")
	}
}

func TestChunkPromptSeparatesFactIDsAndEntityIDs(t *testing.T) {
	chunk := aidatapack.DataPackChunk{
		ChunkID: "c1",
		LocalFactCatalog: map[string]aidatapack.CompactFact{
			"sku.1.last_7d.revenue": {FactID: "sku.1.last_7d.revenue", EntityID: "sku:1"},
		},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{"sku:1": {EntityID: "sku:1"}},
	}
	prompt := userPromptDashboardChunkAnalysis(chunk)
	if !strings.Contains(prompt, "allowed_fact_ids") || !strings.Contains(prompt, "allowed_entity_ids") {
		t.Fatal("prompt must include separate allowlists")
	}
	if !strings.Contains(prompt, "ID GUIDE") {
		t.Fatal("prompt must include ID GUIDE")
	}
}

func TestValidateChunkAnalysisRejectsEntityIDInFactIDs(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := aidatapack.DataPackChunk{
		ChunkID: "c1", ChunkType: "table_rows",
		LocalFactCatalog:   map[string]aidatapack.CompactFact{"f1": {FactID: "f1"}},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{"sku:1": {EntityID: "sku:1"}},
	}
	analysis := DashboardChunkAnalysis{
		SchemaVersion: DashboardChunkAnalysisSchemaVersion, ChunkID: "c1", AnalysisStatus: "completed",
		Signals: []DashboardChunkSignal{{
			SignalID: "s1", SignalType: ChunkSignalObservation, Observation: "ok",
			FactIDs: []string{"sku:1", "f1"}, EntityIDs: []string{"sku:1"},
		}},
	}
	v := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if !v.Valid || len(v.Signals) != 1 || len(v.Signals[0].FactIDs) != 1 || v.Signals[0].FactIDs[0] != "f1" {
		t.Fatalf("expected repaired signal, got %+v valid=%v", v.Signals, v.Valid)
	}
}

func TestValidateChunkAnalysisRejectsFactIDInEntityIDs(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := aidatapack.DataPackChunk{
		ChunkID:            "c1",
		LocalFactCatalog:   map[string]aidatapack.CompactFact{"sku.1.last_7d.revenue": {FactID: "sku.1.last_7d.revenue", EntityID: "sku:1"}},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{"sku:1": {EntityID: "sku:1"}},
	}
	analysis := DashboardChunkAnalysis{
		SchemaVersion: DashboardChunkAnalysisSchemaVersion, ChunkID: "c1", AnalysisStatus: "completed",
		Signals: []DashboardChunkSignal{{
			SignalID: "s1", SignalType: ChunkSignalObservation, Observation: "ok",
			FactIDs: []string{"sku.1.last_7d.revenue"}, EntityIDs: []string{"sku.1.last_7d.revenue", "sku:1"},
		}},
	}
	v := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if !v.Valid || len(v.Signals[0].EntityIDs) != 1 || v.Signals[0].EntityIDs[0] != "sku:1" {
		t.Fatalf("expected fact_id stripped from entity_ids: %+v", v.Signals)
	}
}

func TestValidateChunkAnalysisRejectsInventedFactID(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := aidatapack.DataPackChunk{
		ChunkID:          "c1",
		LocalFactCatalog: map[string]aidatapack.CompactFact{"f1": {FactID: "f1"}},
	}
	analysis := DashboardChunkAnalysis{
		SchemaVersion: DashboardChunkAnalysisSchemaVersion, ChunkID: "c1", AnalysisStatus: "completed",
		Signals: []DashboardChunkSignal{{
			SignalID: "s1", SignalType: ChunkSignalObservation, Observation: "ok",
			FactIDs: []string{"invented.fact.id"},
		}},
	}
	v := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if v.Valid || v.InventedFactRejected != 1 {
		t.Fatalf("expected invented fact rejection, valid=%v invented=%d", v.Valid, v.InventedFactRejected)
	}
}

func TestValidateChunkAnalysisRejectsFactIDNotInLocalCatalog(t *testing.T) {
	TestValidateChunkAnalysisRejectsInventedFactID(t)
}

func TestChunkAnalysisRepairDropsInvalidIDs(t *testing.T) {
	TestValidateChunkAnalysisRejectsEntityIDInFactIDs(t)
}

func TestSynthesisPromptRequiresRawJSONOnly(t *testing.T) {
	prompt := systemPromptDashboardSynthesis()
	lower := strings.ToLower(prompt)
	for _, needle := range []string{"valid json only", "do not wrap json in markdown", "json code fences"} {
		if !strings.Contains(lower, strings.ToLower(needle)) {
			t.Fatalf("missing %q in synthesis prompt", needle)
		}
	}
}

func TestSynthesisJSONCoercesWhatChangedStringItems(t *testing.T) {
	raw := `{"headline":"H","executive_summary":"S","health_reasons":[],"improvement_summary":"","top_actions":[],"data_quality_notes":[],"what_changed":{"summary":"Изменения","groups":[{"key":"worsened","label":"Ухудшилось","tone":"negative","items":["Выручка снизилась"]}]}}`
	result := parseDashboardSynthesisPayloadDetailed(raw)
	if result.ParseError != nil || result.Payload == nil || result.Payload.WhatChanged == nil {
		t.Fatalf("parse failed: %v payload=%+v", result.ParseError, result.Payload)
	}
	if len(result.Payload.WhatChanged.Groups) != 1 || len(result.Payload.WhatChanged.Groups[0].Items) != 1 {
		t.Fatalf("what_changed=%+v", result.Payload.WhatChanged)
	}
	if result.Payload.WhatChanged.Groups[0].Items[0].Text != "Выручка снизилась" {
		t.Fatalf("text=%q", result.Payload.WhatChanged.Groups[0].Items[0].Text)
	}
}

func TestSynthesisJSONExtractionAndRepair(t *testing.T) {
	raw := `{"headline":"H","executive_summary":"S","health_reasons":[],"improvement_summary":"","top_actions":[],"data_quality_notes":[],"ad_efficiency_narrative":"кратко"}`
	result := parseDashboardSynthesisPayloadDetailed(raw)
	if result.ParseError != nil || result.Payload == nil {
		t.Fatalf("parse failed: %v", result.ParseError)
	}
	if result.Payload.AdEfficiencyNarrative == nil || result.Payload.AdEfficiencyNarrative.Summary != "кратко" {
		t.Fatal("expected string ad_efficiency_narrative coerced to object")
	}

	notesRaw := `{"headline":"H","executive_summary":"S","health_reasons":[],"improvement_summary":"","top_actions":[],"data_quality_notes":[{"title":"A","summary":"B"}]}`
	result2 := parseDashboardSynthesisPayloadDetailed(notesRaw)
	if result2.ParseError != nil || len(result2.Payload.DataQualityNotes) != 1 || result2.Payload.DataQualityNotes[0] != "A: B" {
		t.Fatalf("data_quality_notes repair failed: %+v err=%v", result2.Payload, result2.ParseError)
	}
}

func TestSynthesisParseFailureSetsExplicitStatus(t *testing.T) {
	out := buildChunkPipelineOutcome(
		minimalChunkValidationPack(),
		&aidatapack.ChunkSet{},
		ChunkAnalysisRunResult{},
		ChunkSynthesisStatusParseFailed,
		"invalid character",
		"",
	)
	if out.ChunkSynthesisStatus != ChunkSynthesisStatusParseFailed || out.ChunkPipelineStatus != ChunkPipelineStatusFailed {
		t.Fatalf("pipeline=%+v", out)
	}
}

func TestFinalPayloadSourceDatapackChunksWhenNoFallback(t *testing.T) {
	out := buildChunkPipelineOutcome(
		minimalChunkValidationPack(),
		&aidatapack.ChunkSet{},
		ChunkAnalysisRunResult{AnalysisStatus: ChunkAnalysisStatusCompleted},
		ChunkSynthesisStatusCompleted,
		"",
		FinalPayloadSourceDatapackChunksSynthesis,
	)
	if out.FinalPayloadSource != FinalPayloadSourceDatapackChunksSynthesis || out.UsedLegacyFallback {
		t.Fatalf("unexpected pipeline outcome %+v", out)
	}
}

func TestFactCountDebugFieldsSeparated(t *testing.T) {
	pack := minimalChunkValidationPack()
	pack.FactRegistry["f2"] = aidatapack.Fact{FactID: "f2"}
	chunkSet := &aidatapack.ChunkSet{
		Chunks: []aidatapack.DataPackChunk{{
			LocalFactCatalog: map[string]aidatapack.CompactFact{
				"f1": {FactID: "f1"},
			},
		}},
	}
	signals := []DashboardChunkSignal{{FactIDs: []string{"f1"}, EntityIDs: []string{"sku:1"}}}
	out := buildChunkPipelineOutcome(pack, chunkSet, ChunkAnalysisRunResult{ValidatedSignals: signals}, ChunkSynthesisStatusCompleted, "", FinalPayloadSourceDatapackChunksSynthesis)
	if out.FullDataPackFactCount != 2 || out.ChunkLocalFactCount != 1 || out.ValidatedSignalFactCount != 1 {
		t.Fatalf("counts mismatch: %+v", out)
	}
}

func TestCampaignProductLinksDuplicateAudit(t *testing.T) {
	pack := &aidatapack.SellerAIDataPack{
		Tables: aidatapack.Tables{
			CampaignProductLinks: []aidatapack.CampaignProductLinkRow{
				{CampaignExternalID: "1", SKU: "A"},
				{CampaignExternalID: "1", SKU: "A"},
				{CampaignExternalID: "2", SKU: "B"},
			},
		},
	}
	report := aidatapack.AuditDataPack(pack)
	if report.CampaignProductLinks.SourceCount != 3 || report.CampaignProductLinks.DistinctByCampaignSKU != 2 || report.CampaignProductLinks.DuplicateCount != 1 {
		t.Fatalf("audit=%+v", report.CampaignProductLinks)
	}
}

func TestLooksLikeEntityAndFactID(t *testing.T) {
	if !looksLikeEntityID("sku:1") || looksLikeFactID("sku:1") {
		t.Fatal("sku:1 is entity")
	}
	if !looksLikeFactID("sku.1.last_7d.revenue") || looksLikeEntityID("sku.1.last_7d.revenue") {
		t.Fatal("dotted id is fact")
	}
}

func TestDashboardHomeResponseContractAfterChunksNarrative(t *testing.T) {
	payload := &dashboardNarrativePayload{
		Headline: "Заголовок", ExecutiveSummary: "Резюме.", ImprovementSummary: "Улучшения.",
		HealthReasons:    []dashboardNarrativeHealthReason{{DriverID: driverStock, Category: "stock", Title: "T", Reason: "R"}},
		TopActions:       []dashboardNarrativeTopAction{{DriverID: driverStock, SourceCandidateID: candidateStockReview, Priority: "high", Category: "stock", Title: "T", Reason: "R", NextStep: "N", ExpectedEffect: "E"}},
		DataQualityNotes: []string{"note"},
	}
	build := dashboardNarrativeBuildResult{
		ContextSource: ContextSourceSellerAIDataPackChunks,
		CandidateByID: map[string]dashboardNarrativeCandidate{
			candidateStockReview: {ID: candidateStockReview, Category: "stock", CompatibleDriverIDs: []string{driverStock}},
		},
		Context: dashboardNarrativeContext{MetricDate: "2026-06-25"},
	}
	accepted, repaired, _, hard := acceptGeneratedDashboardNarrative(payload, build)
	if !accepted || repaired == nil || len(hard) > 0 {
		t.Fatalf("acceptance failed accepted=%v hard=%v", accepted, hard)
	}
	if repaired.Headline == "" || repaired.ExecutiveSummary == "" {
		t.Fatal("seller-facing blocks must be populated")
	}
}
