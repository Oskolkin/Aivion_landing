package aipipeline_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func TestGlobalContextRequestMetricsPresent(t *testing.T) {
	input := sampleGlobalContextMetricsInput()
	metrics, err := aipipeline.EstimateGlobalContextRequestMetricsForTest(
		input,
		"gpt-4.1-mini",
		300,
		aipipeline.MaxAIInputTokensPerRequestForTest(),
		aipipeline.MaxAIOutputTokensPerRequestForTest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.InputJSONBytes <= 0 || metrics.InputEstimatedTokens <= 0 {
		t.Fatalf("input metrics=%+v", metrics)
	}
	if metrics.SystemPromptBytes <= 0 || metrics.TaskPromptBytes <= 0 {
		t.Fatalf("prompt metrics=%+v", metrics)
	}
	if metrics.TotalRequestBytes <= metrics.InputJSONBytes {
		t.Fatalf("total_bytes=%d input_json_bytes=%d", metrics.TotalRequestBytes, metrics.InputJSONBytes)
	}
	if metrics.TotalEstimatedInputTokens <= 0 {
		t.Fatalf("total_estimated=%d", metrics.TotalEstimatedInputTokens)
	}
	if metrics.TimeoutSeconds != 300 {
		t.Fatalf("timeout=%d", metrics.TimeoutSeconds)
	}
	if metrics.MaxInputTokens <= 0 || metrics.MaxOutputTokens <= 0 {
		t.Fatalf("limits=%d/%d", metrics.MaxInputTokens, metrics.MaxOutputTokens)
	}
}

func TestGlobalContextRequestMetricsInputNotShrunk(t *testing.T) {
	input := sampleGlobalContextMetricsInput()
	wire, err := aipipeline.MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatal(err)
	}
	var signals []dashboardhome.DashboardChunkSignal
	if err := json.Unmarshal(payload["validated_signals"], &signals); err != nil {
		t.Fatal(err)
	}
	var facts map[string]aidatapack.CompactFact
	if err := json.Unmarshal(payload["fact_catalog"], &facts); err != nil {
		t.Fatal(err)
	}
	var entities map[string]aidatapack.CompactEntity
	if err := json.Unmarshal(payload["entity_catalog"], &entities); err != nil {
		t.Fatal(err)
	}
	if len(signals) != len(input.ValidatedSignals) {
		t.Fatalf("signals=%d want %d", len(signals), len(input.ValidatedSignals))
	}
	if len(facts) != len(input.FactCatalog) {
		t.Fatalf("facts=%d want %d", len(facts), len(input.FactCatalog))
	}
	if len(entities) != len(input.EntityCatalog) {
		t.Fatalf("entities=%d want %d", len(entities), len(input.EntityCatalog))
	}
}

func TestGlobalContextRequestDiagnosticsNoContentLeak(t *testing.T) {
	input := sampleGlobalContextMetricsInput()
	metrics, err := aipipeline.EstimateGlobalContextRequestMetricsForTest(
		input,
		"gpt-4.1-mini",
		300,
		120000,
		8000,
	)
	if err != nil {
		t.Fatal(err)
	}
	diag := aipipeline.NewPipelineDiagnosticsForTest(aipipeline.PipelineProviders{})
	aipipeline.PopulateGlobalContextRequestDiagnosticsForTest(&diag, metrics)
	raw, err := json.Marshal(diag)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "SECRET-SKU-NAME-XYZ") {
		t.Fatal("diagnostics leaked SKU content")
	}
	if strings.Contains(text, aipipeline.GlobalContextSystemPromptV2ForTest()) {
		t.Fatal("diagnostics leaked full system prompt")
	}
}

func sampleGlobalContextMetricsInput() aipipeline.GlobalContextGenerationInput {
	return aipipeline.GlobalContextGenerationInput{
		SchemaVersion:   aipipeline.GlobalContextInputSchemaVersion,
		SellerAccountID: 1,
		MetricDate:      "2026-07-08",
		ValidatedSignals: []dashboardhome.DashboardChunkSignal{
			{SignalID: "s1", Observation: "revenue down"},
			{SignalID: "s2", Observation: "stock low"},
		},
		FactCatalog: map[string]aidatapack.CompactFact{
			"f1": {FactID: "f1", Metric: "revenue"},
			"f2": {FactID: "f2", Metric: "margin"},
		},
		EntityCatalog: map[string]aidatapack.CompactEntity{
			"sku:1": {EntityID: "sku:1", Name: "SECRET-SKU-NAME-XYZ"},
		},
	}
}
