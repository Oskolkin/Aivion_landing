package aipipeline

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func TestClassifyArtifactContentCompleteness_FullStructuredDocuments(t *testing.T) {
	cases := []struct {
		kind string
	}{
		{ArtifactKindDataPack},
		{ArtifactKindSharedChunkAnalysis},
		{ArtifactKindGlobalContextFinal},
		{ArtifactKindActionsInput},
		{ArtifactKindActionsOutput},
		{ArtifactKindActionsValidation},
		{ArtifactKindPipelineSummary},
	}
	for _, tc := range cases {
		got := ClassifyArtifactContentCompleteness(tc.kind, map[string]any{"schema_version": "v1", "payload": "x"})
		if got != ArtifactCompletenessFullStructured {
			t.Fatalf("kind=%s got=%s", tc.kind, got)
		}
	}
}

func TestClassifyArtifactContentCompleteness_ActionsRawOutputMetadataOnly(t *testing.T) {
	doc := map[string]any{
		"schema_version": "raw_output_v1",
		"component":      "actions_synthesis",
		"output_tokens":  8192,
		"status":         "validation_failed",
	}
	got := ClassifyArtifactContentCompleteness(ArtifactKindActionsRawOutput, doc)
	if got != ArtifactContentMetadataOnly {
		t.Fatalf("got=%s", got)
	}
}

func TestBuildAICallsTopologySummary_Run72DerivedRestart(t *testing.T) {
	summary := map[string]any{
		"source_pipeline_run_id": 71,
		"reused_stages":          []any{"data_pack", "chunk_set", "shared_chunk_analysis", "global_context"},
		"cloned_stages":          []any{"dashboard"},
		"ai_calls":               map[string]any{"actions": 1, "dashboard": 0, "global_context": 0, "shared_chunk_analysis": 0},
	}
	byKind := map[string]dbgen.AiPipelineArtifact{
		ArtifactKindActionsRawOutput: {ID: 565},
		ArtifactKindPipelineSummary:  {ID: 566},
	}
	legacy := LegacyActionsInspectorAdapter{ActionsRawOutput: mustJSON(map[string]any{"schema_version": "raw_output_v1", "output_tokens": 8192})}
	rows := buildAICallsTopologySummary(byKind, summary, legacy)
	if len(rows) != 4 {
		t.Fatalf("rows=%d want 4", len(rows))
	}
	byStage := map[string]map[string]any{}
	for _, row := range rows {
		byStage[stringValue(row["stage"])] = row
	}
	gc := byStage["global_context"]
	if gc == nil || stringValue(gc["execution"]) != "reused" || stringValue(gc["ai_call_in_current_run"]) != "not_executed" {
		t.Fatalf("global_context=%v", gc)
	}
	if intValue(gc["source_pipeline_run_id"]) != 71 || stringValue(gc["diagnostics"]) != DiagnosticStatusNotApplicable {
		t.Fatalf("global_context diagnostics=%v", gc["diagnostics"])
	}
	dash := byStage["dashboard"]
	if dash == nil || stringValue(dash["execution"]) != "cloned" || stringValue(dash["ai_call_in_current_run"]) != "not_executed" {
		t.Fatalf("dashboard=%v", dash)
	}
	chunk := byStage["shared_chunk_analysis"]
	if chunk == nil || stringValue(chunk["execution"]) != "reused" {
		t.Fatalf("shared_chunk=%v", chunk)
	}
	actions := byStage["actions_compose"]
	if actions == nil || intValue(actions["ai_calls"]) != 1 || stringValue(actions["stage_outcome"]) != "failed" {
		t.Fatalf("actions=%v", actions)
	}
	if stringValue(actions["diagnostics"]) != DiagnosticStatusPartial {
		t.Fatalf("actions diagnostics=%v", actions["diagnostics"])
	}
}

func TestProjectLegacyDropReasons_DuplicateScope(t *testing.T) {
	doc := map[string]any{
		"dropped_count": 1,
		"issues": []any{
			map[string]any{
				"code":         "duplicate_scope_after_ai_order_normalization",
				"action_index": 0,
			},
		},
	}
	got := projectLegacyDropReasons(doc)
	if len(got) != 1 {
		t.Fatalf("got=%+v", got)
	}
	if stringValue(got[0]["drop_code"]) != "duplicate_scope_after_ai_order_normalization" {
		t.Fatalf("drop=%+v", got[0])
	}
	if got[0]["comparison_details_available"] != false {
		t.Fatal("must not fabricate comparison details")
	}
}

func TestBuildCandidateSearchText_IncludesSignalAndSKU(t *testing.T) {
	text := buildCandidateSearchText(map[string]any{
		"candidate_id":      "cand-1",
		"chunk_id":          "chunk-a",
		"type":              "pricing",
		"source_signal_ids": []any{"sig-1"},
		"evidence_fact_ids": []any{"fact-1"},
		"affected_entities": []any{
			map[string]any{"entity_id": "sku:801", "sku": "801000100001"},
		},
	})
	for _, want := range []string{"cand-1", "chunk-a", "sig-1", "fact-1", "sku:801", "801000100001"} {
		if !strings.Contains(text, want) {
			t.Fatalf("search_text=%q missing %q", text, want)
		}
	}
}

func candidateRowMatchesSearch(searchText, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(searchText), q)
}
