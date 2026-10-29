package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestIsSupportedDashboardChunkAnalysisSchemaVersion(t *testing.T) {
	if !IsSupportedDashboardChunkAnalysisSchemaVersion(DashboardChunkAnalysisSchemaVersionV1) {
		t.Fatal("v1 should be supported")
	}
	if !IsSupportedDashboardChunkAnalysisSchemaVersion(DashboardChunkAnalysisSchemaVersionV2) {
		t.Fatal("v2 should be supported")
	}
	if DashboardChunkAnalysisSchemaVersion != DashboardChunkAnalysisSchemaVersionV3 {
		t.Fatal("production default schema version should alias v3")
	}
	if !IsSupportedDashboardChunkAnalysisSchemaVersion(DashboardChunkAnalysisSchemaVersionV3) {
		t.Fatal("v3 should be supported")
	}
	if IsSupportedDashboardChunkAnalysisSchemaVersion("dashboard_chunk_analysis_v4") {
		t.Fatal("unknown version should not be supported")
	}
}

func TestParseDashboardChunkAnalysis_MissingSchemaVersionDefaultsToV2AndValidates(t *testing.T) {
	raw := `{
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "insufficient_data",
  "signals": [],
  "action_candidates": []
}`

	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if analysis.SchemaVersion != "" {
		t.Fatalf("parser should not invent schema_version, got %q", analysis.SchemaVersion)
	}

	if analysis.SchemaVersion == "" {
		analysis.SchemaVersion = DashboardChunkAnalysisSchemaVersion
	}
	if analysis.SchemaVersion != DashboardChunkAnalysisSchemaVersionV3 {
		t.Fatalf("schema_version=%q want %q", analysis.SchemaVersion, DashboardChunkAnalysisSchemaVersionV3)
	}

	pack := minimalChunkValidationPack()
	chunk := aidatapack.DataPackChunk{ChunkID: "c1", ChunkType: "table_rows"}
	result := ValidateDashboardChunkAnalysis(chunk, *analysis, pack)
	if !result.Valid {
		t.Fatalf("expected valid after v2 default, hard_errors=%v repairable=%v", result.HardErrors, result.RepairableErrors)
	}
}

func TestParseDashboardChunkAnalysis_V1WithoutActionCandidates(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v1",
  "chunk_id": "test_chunk",
  "chunk_type": "stock_positions",
  "analysis_status": "completed",
  "signals": [],
  "data_quality_notes": [],
  "limitations_acknowledged": []
}`

	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if analysis.SchemaVersion != DashboardChunkAnalysisSchemaVersionV1 {
		t.Fatalf("schema_version=%q", analysis.SchemaVersion)
	}
	if analysis.Signals == nil {
		t.Fatal("signals should be normalized to empty slice")
	}
	if len(analysis.Signals) != 0 {
		t.Fatalf("signals=%d", len(analysis.Signals))
	}
	if analysis.ActionCandidates == nil {
		t.Fatal("action_candidates should be normalized to empty slice")
	}
	if len(analysis.ActionCandidates) != 0 {
		t.Fatalf("action_candidates=%d", len(analysis.ActionCandidates))
	}
}

func TestParseDashboardChunkAnalysis_V2WithEmptyActionCandidates(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "test_chunk",
  "chunk_type": "stock_positions",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [],
  "data_quality_notes": [],
  "limitations_acknowledged": []
}`

	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if analysis.SchemaVersion != DashboardChunkAnalysisSchemaVersionV2 {
		t.Fatalf("schema_version=%q", analysis.SchemaVersion)
	}
	if analysis.Signals == nil || len(analysis.Signals) != 0 {
		t.Fatalf("signals=%v", analysis.Signals)
	}
	if analysis.ActionCandidates == nil || len(analysis.ActionCandidates) != 0 {
		t.Fatalf("action_candidates=%v", analysis.ActionCandidates)
	}
}

func TestParseDashboardChunkAnalysis_V2NullNormalization(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "test_chunk",
  "chunk_type": "stock_positions",
  "analysis_status": "completed",
  "signals": null,
  "action_candidates": null
}`

	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if analysis.Signals == nil {
		t.Fatal("signals should be normalized from null")
	}
	if analysis.ActionCandidates == nil {
		t.Fatal("action_candidates should be normalized from null")
	}
}

func TestParseDashboardChunkAnalysis_V2WithCandidate(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "test_chunk",
  "chunk_type": "stock_positions",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [{
    "candidate_id": "candidate-1",
    "chunk_id": "chunk-1",
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "У нескольких SKU низкий остаток",
    "recommended_action": "Проверить поставки и пополнить остаток",
    "why_actionable": "Товары могут закончиться",
    "affected_entities": [],
    "evidence_fact_ids": [],
    "source_signal_ids": [],
    "limitations": []
  }]
}`

	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(analysis.ActionCandidates) != 1 {
		t.Fatalf("action_candidates=%d", len(analysis.ActionCandidates))
	}
	candidate := analysis.ActionCandidates[0]
	if candidate.CandidateID != "candidate-1" {
		t.Fatalf("candidate_id=%q", candidate.CandidateID)
	}
	if candidate.Type != actions.ActionTypeStockReplenishment {
		t.Fatalf("type=%q", candidate.Type)
	}
	if candidate.Category != actions.ActionCategoryStock {
		t.Fatalf("category=%q", candidate.Category)
	}
	if candidate.LocalPriority != actions.ActionPriorityCritical {
		t.Fatalf("local_priority=%q", candidate.LocalPriority)
	}
}

func TestValidateDashboardChunkAnalysis_RejectsUnknownSchemaVersion(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := aidatapack.DataPackChunk{ChunkID: "c1", ChunkType: "table_rows"}
	analysis := DashboardChunkAnalysis{
		SchemaVersion:  "dashboard_chunk_analysis_v4",
		ChunkID:        "c1",
		AnalysisStatus: "completed",
		Signals:        []DashboardChunkSignal{},
	}
	result := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if result.Valid {
		t.Fatal("expected invalid for unknown schema version")
	}
	if !containsString(result.HardErrors, "invalid_schema_version") {
		t.Fatalf("hard_errors=%v", result.HardErrors)
	}
}

func TestValidateDashboardChunkAnalysis_AcceptsV2SchemaVersion(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := aidatapack.DataPackChunk{
		ChunkID: "c1", ChunkType: "table_rows",
		LocalFactCatalog: map[string]aidatapack.CompactFact{"f1": {FactID: "f1"}},
	}
	analysis := DashboardChunkAnalysis{
		SchemaVersion:  DashboardChunkAnalysisSchemaVersionV2,
		ChunkID:        "c1",
		AnalysisStatus: "insufficient_data",
		Signals:        []DashboardChunkSignal{},
	}
	result := ValidateDashboardChunkAnalysis(chunk, analysis, pack)
	if !result.Valid {
		t.Fatalf("expected valid v2 insufficient_data, hard_errors=%v repairable=%v", result.HardErrors, result.RepairableErrors)
	}
	if result.ActionCandidates == nil {
		t.Fatal("action_candidates should be initialized")
	}
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("action_candidates=%d", len(result.ActionCandidates))
	}
	if result.RejectedCandidateCount != 0 {
		t.Fatalf("rejected_candidate_count=%d", result.RejectedCandidateCount)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestChunkAnalysisParser_AffectedEntitiesArrayObjects(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": [
      {"entity_type": "sku", "entity_id": "sku:101", "role": "affected"}
    ],
    "evidence_fact_ids": ["sku.101.coverage_days"],
    "source_signal_ids": ["signal-1"]
  }]
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(analysis.ActionCandidates) != 1 || len(analysis.ActionCandidates[0].AffectedEntities) != 1 {
		t.Fatalf("candidates=%+v", analysis.ActionCandidates)
	}
}

func TestChunkAnalysisParser_AffectedEntitiesSingleObjectNormalized(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": {"entity_type": "sku", "entity_id": "sku:101"},
    "evidence_fact_ids": ["sku.101.coverage_days"],
    "source_signal_ids": ["signal-1"]
  }]
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(analysis.ActionCandidates[0].AffectedEntities) != 1 {
		t.Fatalf("entities=%+v", analysis.ActionCandidates[0].AffectedEntities)
	}
}

func TestChunkAnalysisParser_AffectedEntitiesArrayStringsNormalized(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": ["sku:101", "campaign:9"],
    "evidence_fact_ids": ["sku.101.coverage_days"],
    "source_signal_ids": ["signal-1"]
  }]
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ents := analysis.ActionCandidates[0].AffectedEntities
	if len(ents) != 2 || ents[0].EntityType != "sku" || ents[1].EntityType != "campaign" {
		t.Fatalf("entities=%+v", ents)
	}
}

func TestChunkAnalysisParser_AffectedEntitiesStringNormalized(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": "sku:101",
    "evidence_fact_ids": ["sku.101.coverage_days"],
    "source_signal_ids": ["signal-1"]
  }]
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(analysis.ActionCandidates[0].AffectedEntities) != 1 {
		t.Fatalf("entities=%+v", analysis.ActionCandidates[0].AffectedEntities)
	}
}

func TestChunkAnalysisParser_InvalidAffectedEntitiesRejectsCandidateNotChunk(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [{
    "signal_id": "signal-1",
    "signal_type": "observation",
    "entity_ids": ["sku:101"],
    "fact_ids": ["sku.101.coverage_days"],
    "observation": "stock low"
  }],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": [123, true],
    "evidence_fact_ids": ["sku.101.coverage_days"],
    "source_signal_ids": ["signal-1"]
  }]
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse should not fail chunk: %v", err)
	}
	chunk := aidatapack.DataPackChunk{
		ChunkID: "c1", ChunkType: "table_rows",
		LocalFactCatalog:   map[string]aidatapack.CompactFact{"sku.101.coverage_days": {FactID: "sku.101.coverage_days", EntityID: "sku:101"}},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{"sku:101": {EntityID: "sku:101", EntityType: "sku"}},
	}
	result := ValidateDashboardChunkAnalysis(chunk, *analysis, minimalChunkValidationPack())
	if len(result.Signals) != 1 {
		t.Fatalf("signals=%d rejected=%d", len(result.Signals), result.RejectedSignalCount)
	}
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("expected invalid candidate rejected, got %d", len(result.ActionCandidates))
	}
	if result.RejectedCandidateCount == 0 {
		t.Fatal("expected rejected candidate count > 0")
	}
}

func TestChunkAnalysisParser_InvalidCandidateDoesNotDropValidSignals(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [{
    "signal_id": "signal-1",
    "signal_type": "observation",
    "entity_ids": "sku:101",
    "fact_ids": "sku.101.coverage_days",
    "observation": "stock low"
  }],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": "unknown_entity_without_prefix",
    "evidence_fact_ids": "sku.101.coverage_days",
    "source_signal_ids": "signal-1"
  }]
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	chunk := aidatapack.DataPackChunk{
		ChunkID: "c1", ChunkType: "table_rows",
		LocalFactCatalog:   map[string]aidatapack.CompactFact{"sku.101.coverage_days": {FactID: "sku.101.coverage_days", EntityID: "sku:101"}},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{"sku:101": {EntityID: "sku:101", EntityType: "sku"}},
	}
	result := ValidateDashboardChunkAnalysis(chunk, *analysis, minimalChunkValidationPack())
	if len(result.Signals) != 1 {
		t.Fatalf("signals=%d", len(result.Signals))
	}
	if len(result.ActionCandidates) != 0 {
		t.Fatalf("bad candidate should be rejected, got %d", len(result.ActionCandidates))
	}
}

func TestChunkAnalysisParser_LimitationsAcknowledgedBoolNormalized(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "c1",
  "chunk_type": "table_rows",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [],
  "limitations_acknowledged": false
}`
	analysis, err := parseDashboardChunkAnalysis(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if analysis.LimitationsAcknowledged == nil {
		t.Fatal("expected empty slice, not nil")
	}
	if len(analysis.LimitationsAcknowledged) != 0 {
		t.Fatalf("limitations=%v", analysis.LimitationsAcknowledged)
	}
}

func TestChunkAnalysisParser_RegressionCannotUnmarshalAffectedEntitiesString(t *testing.T) {
	raw := `{
  "schema_version": "dashboard_chunk_analysis_v2",
  "chunk_id": "manifest_context",
  "chunk_type": "manifest",
  "analysis_status": "completed",
  "signals": [],
  "action_candidates": [{
    "type": "stock_replenishment",
    "category": "stock",
    "local_priority": "critical",
    "problem": "low stock",
    "recommended_action": "replenish",
    "why_actionable": "risk",
    "affected_entities": "sku:101",
    "evidence_fact_ids": ["sku.101.coverage_days"],
    "source_signal_ids": ["signal-1"]
  }]
}`
	if _, err := parseDashboardChunkAnalysis(raw); err != nil {
		t.Fatalf("regression: parse must not fail with string affected_entities: %v", err)
	}
}
