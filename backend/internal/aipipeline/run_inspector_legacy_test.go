package aipipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestExportRunInspector_Run72LegacyFixture(t *testing.T) {
	dir := t.TempDir()
	validationPayload := map[string]any{
		"status": "invalid", "accepted_count": 0, "dropped_count": 1,
		"issues": []map[string]any{
			{"code": "entity_outside_candidate_scope_removed", "severity": "warning", "action_index": 0},
			{"code": "entity_outside_candidate_scope_removed", "severity": "warning", "action_index": 0},
			{"code": "unknown_object_metric_fact_id", "severity": "warning", "action_index": 0, "fact_id": "sku.1.idc_days"},
			{"code": "duplicate_scope_after_ai_order_normalization", "severity": "warning", "action_index": 0, "compared_action_index": 0, "same_action_comparison": true},
		},
		"dropped_actions": []map[string]any{{"reasons": []string{"actions_no_valid_actions"}, "failure_subreason": "actions_no_valid_actions"}},
	}
	rawOutputPayload := map[string]any{
		"actions": []map[string]any{{
			"type": "investigate_sales_drop", "scope": "account",
			"source_candidate_ids": []string{"account_rollup_001:candidate:01"},
		}},
	}
	rawOutputRaw, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": rawOutputPayload})
	validationRaw, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": validationPayload})
	outputRaw, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": map[string]any{
		"final_status": "failed", "accepted_count": 0, "dropped_count": 1, "actions": []any{},
	}})
	summaryRaw, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": map[string]any{
		"actions_candidate_coverage": map[string]any{
			"actions_candidate_total_count":                   35,
			"actions_candidate_referenced_persisted_count":    0,
			"actions_candidate_referenced_dropped_only_count": 8,
			"actions_candidate_not_referenced_by_ai_count":    27,
		},
	}})
	store := &fakeInspectorStore{
		run: dbgen.AiPipelineRun{
			ID: 72, SellerAccountID: 1, Status: "completed_partial",
			DataPackHash: "abc", MetricDate: pgtype.Date{Time: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		artifacts: []dbgen.AiPipelineArtifact{
			{ID: 1, PipelineRunID: 72, Kind: ArtifactKindDataPack, PayloadJson: []byte(`{"meta":{},"payload":{"fact_registry":{}}}`)},
			{ID: 2, PipelineRunID: 72, Kind: ArtifactKindActionsRawOutput, PayloadJson: rawOutputRaw},
			{ID: 3, PipelineRunID: 72, Kind: ArtifactKindActionsOutput, PayloadJson: outputRaw},
			{ID: 4, PipelineRunID: 72, Kind: ArtifactKindActionsValidation, PayloadJson: validationRaw},
			{ID: 5, PipelineRunID: 72, Kind: ArtifactKindPipelineSummary, PayloadJson: summaryRaw},
			{ID: 6, PipelineRunID: 72, Kind: ArtifactKindSharedChunkAnalysis, PayloadJson: []byte(`{"meta":{},"payload":{"validated_signals":[],"validated_action_candidates":[]}}`)},
		},
	}
	if err := ExportRunInspector(context.Background(), store, RunInspectorExportOptions{
		RunID: 72, OutputDir: dir, IncludeRaw: true, PrettyJSON: true,
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "inspector-data.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	views, _ := data["views"].(map[string]any)
	if views == nil {
		t.Fatal("missing views")
	}
	legacy, _ := views["legacy_actions"].(map[string]any)
	if legacy == nil {
		t.Fatal("missing legacy_actions view")
	}
	compose, _ := legacy["compose"].(map[string]any)
	if compose == nil || intValue(compose["returned_count"]) != 1 {
		t.Fatalf("compose=%v", compose)
	}
	diagnostics, _ := legacy["diagnostics"].(map[string]any)
	coverage, _ := diagnostics["candidate_coverage"].(map[string]any)
	if intValue(coverage["actions_candidate_referenced_persisted_count"]) != 0 ||
		intValue(coverage["actions_candidate_referenced_dropped_only_count"]) != 8 ||
		intValue(coverage["actions_candidate_not_referenced_by_ai_count"]) != 27 {
		t.Fatalf("coverage=%v", coverage)
	}
	final, _ := legacy["final_actions"].(map[string]any)
	if intValue(final["accepted_count"]) != 0 || intValue(final["dropped_count"]) != 1 {
		t.Fatalf("final=%v", final)
	}
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	for _, tab := range []string{"Signals", "Candidates", "Validation", "Legacy Actions", "Global Context"} {
		if !containsSubstring(body, tab) {
			t.Fatalf("html missing tab %q", tab)
		}
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
