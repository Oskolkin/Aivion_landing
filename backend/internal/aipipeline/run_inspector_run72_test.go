package aipipeline

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestExportRunInspector_Run72ActualFixture(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "debug", "run-72-final")
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skip("run-72-final fixture directory not available")
	}
	store := run72InspectorStore(fixtureDir)

	dir := t.TempDir()
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

	// Actions input
	input, _ := views["actions_input"].(map[string]any)
	if intValue(input["candidate_count"]) != 35 {
		t.Fatalf("candidate_count=%v want 35", input["candidate_count"])
	}
	if intValue(input["fact_count"]) != 156 {
		t.Fatalf("fact_count=%v want 156", input["fact_count"])
	}
	if intValue(input["entity_count"]) != 78 {
		t.Fatalf("entity_count=%v want 78", input["entity_count"])
	}
	if stringValue(input["schema_version"]) != "actions_ai_first_v2" {
		t.Fatalf("schema=%v", input["schema_version"])
	}

	// Legacy compose - no raw body
	legacy, _ := views["legacy_actions"].(map[string]any)
	compose, _ := legacy["compose"].(map[string]any)
	if compose["returned_count"] != nil {
		t.Fatalf("returned_count should be nil when raw body missing, got %v", compose["returned_count"])
	}
	if compose["raw_body_available"] != false {
		t.Fatalf("raw_body_available=%v", compose["raw_body_available"])
	}

	// Raw output metadata only
	artifacts, _ := views["artifacts"].([]any)
	foundRaw := false
	for _, item := range artifacts {
		row, _ := item.(map[string]any)
		if stringValue(row["kind"]) != ArtifactKindActionsRawOutput {
			continue
		}
		foundRaw = true
		if row["raw_body_available"] != false {
			t.Fatalf("actions_raw_output raw_body_available=%v", row["raw_body_available"])
		}
		if stringValue(row["content_completeness"]) != ArtifactContentMetadataOnly {
			t.Fatalf("content_completeness=%v", row["content_completeness"])
		}
	}
	if !foundRaw {
		t.Fatal("missing actions_raw_output artifact row")
	}

	// Candidate coverage not reconstructable
	lineage, _ := legacy["lineage"].(map[string]any)
	if stringValue(lineage["coverage_status"]) != DiagnosticStatusNotReconstructable {
		t.Fatalf("coverage_status=%v", lineage["coverage_status"])
	}

	// Entities flattened
	entities, _ := views["entities"].([]any)
	if len(entities) <= 5 {
		t.Fatalf("entities count=%d want >5", len(entities))
	}
	hasSKU := false
	for _, item := range entities {
		row, _ := item.(map[string]any)
		if stringValue(row["entity_id"]) == "skus" {
			t.Fatal("container key skus must not appear as entity")
		}
		if stringValue(row["entity_id"]) == "sku:801000100001" {
			hasSKU = true
		}
	}
	if !hasSKU {
		t.Fatal("missing sku:801000100001 in flattened entities")
	}

	// Stages - dashboard cloned, actions failed
	stages, _ := data["stages"].([]any)
	stageByName := map[string]map[string]any{}
	for _, item := range stages {
		row, _ := item.(map[string]any)
		stageByName[stringValue(row["stage"])] = row
	}
	if dash := stageByName["Dashboard"]; dash == nil || stringValue(dash["execution"]) != "cloned" {
		t.Fatalf("dashboard stage=%v", dash)
	}
	if actions := stageByName["Actions"]; actions == nil || stringValue(actions["outcome"]) != "failed" {
		t.Fatalf("actions stage=%v", actions)
	}

	// HTML search wiring
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	if !strings.Contains(body, `addEventListener('input'`) {
		t.Fatal("html missing search input listener")
	}
	if !strings.Contains(body, "Clear filters") {
		t.Fatal("html missing clear filters button")
	}
	if !strings.Contains(body, "Diagnostic Completeness") {
		t.Fatal("html missing diagnostic completeness section")
	}
	if !strings.Contains(body, `id="actions_input_candidates"`) {
		t.Fatal("html missing actions_input_candidates container")
	}
	if !strings.Contains(body, `data-filter="actions_input_candidates"`) {
		t.Fatal("html missing actions input search wiring")
	}
	if !strings.Contains(body, `id="ai_calls"`) {
		t.Fatal("html missing ai_calls panel")
	}
	if !strings.Contains(body, "hashchange") {
		t.Fatal("html missing hashchange handler")
	}
	if !strings.Contains(body, "LINEAGE_PREFIX_TO_PANEL") {
		t.Fatal("html missing lineage prefix mapping")
	}

	// AI Calls topology — no duplicate Actions rows; reused/cloned not marked not_captured.
	diag, _ := views["diagnostics"].(map[string]any)
	aiCalls, _ := diag["ai_calls"].(map[string]any)
	summaryRows, _ := aiCalls["summary"].([]any)
	actionRows := 0
	for _, item := range summaryRows {
		row, _ := item.(map[string]any)
		if stringValue(row["stage"]) == "actions_compose" {
			actionRows++
		}
		if stringValue(row["execution"]) == "reused" || stringValue(row["execution"]) == "cloned" {
			if stringValue(row["diagnostics"]) == DiagnosticStatusNotCaptured {
				t.Fatalf("reused/cloned must not be not_captured: %+v", row)
			}
		}
	}
	if actionRows != 1 {
		t.Fatalf("actions rows in ai_calls summary=%d want 1", actionRows)
	}

	// Artifact completeness for full documents.
	wantFull := map[string]bool{
		ArtifactKindDataPack: true, ArtifactKindSharedChunkAnalysis: true,
		ArtifactKindGlobalContextFinal: true, ArtifactKindActionsInput: true,
		ArtifactKindActionsOutput: true, ArtifactKindActionsValidation: true,
		ArtifactKindPipelineSummary: true,
	}
	for _, item := range artifacts {
		row, _ := item.(map[string]any)
		kind := stringValue(row["kind"])
		if !wantFull[kind] {
			continue
		}
		if stringValue(row["content_completeness"]) != ArtifactCompletenessFullStructured {
			t.Fatalf("kind=%s completeness=%v", kind, row["content_completeness"])
		}
	}

	// Drop reasons projection.
	validation, _ := views["validation"].(map[string]any)
	drops, _ := validation["drop_reasons"].([]any)
	if len(drops) != 1 {
		t.Fatalf("drop_reasons=%v", drops)
	}
	drop0, _ := drops[0].(map[string]any)
	if stringValue(drop0["drop_code"]) != "duplicate_scope_after_ai_order_normalization" {
		t.Fatalf("drop=%v", drop0)
	}

	// Failure focus honest wording.
	failureFocus, _ := diag["failure_focus"].(map[string]any)
	if failureFocus["applicable"] != true {
		t.Fatalf("failure_focus=%v", failureFocus)
	}
	primary := stringValue(failureFocus["primary_failure"])
	if !strings.Contains(primary, "Raw provider Action count cannot be confirmed") {
		t.Fatalf("primary_failure=%q", primary)
	}
	if strings.Contains(primary, "All returned Actions were dropped") {
		t.Fatalf("must not claim all returned when raw body missing: %q", primary)
	}

	// chunk_id_source on signals.
	signals, _ := views["signals"].([]any)
	if len(signals) == 0 {
		t.Fatal("missing signals")
	}
	hasChunkSource := false
	for _, item := range signals {
		row, _ := item.(map[string]any)
		if stringValue(row["chunk_id_source"]) != "" {
			hasChunkSource = true
			break
		}
	}
	if !hasChunkSource {
		t.Fatal("signals missing chunk_id_source")
	}

	// Actions input search: filter hides non-matching candidate rows.
	candidates, _ := input["candidates"].([]any)
	if len(candidates) < 2 {
		t.Fatalf("need >=2 candidates for filter test, got %d", len(candidates))
	}
	var matchText, otherText string
	for _, item := range candidates {
		row, _ := item.(map[string]any)
		text := stringValue(row["search_text"])
		if strings.Contains(text, "801000100001") {
			matchText = text
		} else if otherText == "" {
			otherText = text
		}
	}
	if matchText == "" {
		t.Fatal("expected sku 801000100001 in candidate search_text")
	}
	if candidateRowMatchesSearch(otherText, "801000100001") {
		t.Fatal("control candidate unexpectedly matches sku filter")
	}
	if !candidateRowMatchesSearch(matchText, "801000100001") {
		t.Fatal("target candidate must match sku filter")
	}
}

func TestExportRun72DiagnosticsV4(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "debug", "run-72-final")
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		t.Skip("run-72-final fixture directory not available")
	}
	outDir := filepath.Join("..", "..", "debug", "run-72-diagnostics-v4")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := run72InspectorStore(fixtureDir)
	if err := ExportRunInspector(context.Background(), store, RunInspectorExportOptions{
		RunID: 72, OutputDir: outDir, IncludeRaw: true, PrettyJSON: true, DiagnosticSummary: true,
	}); err != nil {
		t.Fatal(err)
	}
}

func run72InspectorStore(fixtureDir string) *fakeInspectorStore {
	load := func(name string) []byte {
		raw, err := os.ReadFile(filepath.Join(fixtureDir, name))
		if err != nil {
			panic(err)
		}
		return raw
	}
	wrap := func(payload []byte) []byte {
		var doc map[string]any
		if err := json.Unmarshal(payload, &doc); err != nil {
			panic(err)
		}
		wrapped, _ := json.Marshal(map[string]any{"meta": map[string]any{}, "payload": doc})
		return wrapped
	}
	return &fakeInspectorStore{
		run: dbgen.AiPipelineRun{
			ID: 72, SellerAccountID: 1, Status: "completed_partial",
			DataPackHash: "abc", MetricDate: pgtype.Date{Time: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		artifacts: []dbgen.AiPipelineArtifact{
			{ID: 558, PipelineRunID: 72, Kind: ArtifactKindDataPack, PayloadJson: wrap(load("data-pack.json"))},
			{ID: 559, PipelineRunID: 72, Kind: ArtifactKindSharedChunkAnalysis, PayloadJson: wrap(load("shared-chunk-analysis.json"))},
			{ID: 561, PipelineRunID: 72, Kind: ArtifactKindGlobalContextFinal, PayloadJson: wrap(load("global-context.json"))},
			{ID: 562, PipelineRunID: 72, Kind: ArtifactKindActionsInput, PayloadJson: wrap(load("actions-input.json"))},
			{ID: 563, PipelineRunID: 72, Kind: ArtifactKindActionsOutput, PayloadJson: wrap(json.RawMessage(`{"final_status":"failed","accepted_count":0,"dropped_count":1,"actions":[]}`))},
			{ID: 564, PipelineRunID: 72, Kind: ArtifactKindActionsValidation, PayloadJson: wrap(load("actions-validation.json"))},
			{ID: 565, PipelineRunID: 72, Kind: ArtifactKindActionsRawOutput, PayloadJson: wrap(load("actions-raw-output.json"))},
			{ID: 566, PipelineRunID: 72, Kind: ArtifactKindPipelineSummary, PayloadJson: wrap(load("pipeline-summary.json"))},
		},
	}
}

func TestClassifyLegacyRawOutputArtifact_MetadataOnly(t *testing.T) {
	doc := map[string]any{
		"schema_version": "raw_output_v1",
		"component":      "actions_synthesis",
		"output_tokens":  8192,
		"status":         "validation_failed",
	}
	contentType, available := ClassifyLegacyRawOutputArtifact(doc)
	if contentType != ArtifactContentMetadataOnly || available {
		t.Fatalf("contentType=%s available=%v", contentType, available)
	}
}

func TestBuildDiagnosticCompletenessMatrix_Run72Shape(t *testing.T) {
	legacy := LegacyActionsInspectorAdapter{
		ActionsRawOutput: mustJSON(map[string]any{
			"schema_version": "raw_output_v1", "output_tokens": 8192,
		}),
		ActionsValidation: mustJSON(map[string]any{"accepted_count": 0, "dropped_count": 1}),
	}
	byKind := map[string]dbgen.AiPipelineArtifact{
		ArtifactKindActionsInput:      {ID: 562},
		ArtifactKindActionsRawOutput:  {ID: 565},
		ArtifactKindActionsValidation: {ID: 564},
		ArtifactKindActionsOutput:     {ID: 563},
	}
	matrix := buildDiagnosticCompletenessMatrix(byKind, legacy)
	foundRawNotCaptured := false
	for _, entry := range matrix {
		if entry.Dimension == "Raw provider body" && entry.Status == DiagnosticStatusNotCaptured {
			foundRawNotCaptured = true
		}
	}
	if !foundRawNotCaptured {
		t.Fatalf("matrix=%+v", matrix)
	}
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}
