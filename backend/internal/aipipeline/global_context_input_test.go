package aipipeline

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func TestBuildGlobalContextGenerationInputFromSharedRun(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	runner := NewDeterministicChunkAnalysisRunner()
	chunkRun, err := runner.AnalyzeDataPackChunks(t.Context(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildGlobalContextGenerationInput(pack, "hash-1", set, chunkRun)
	if err != nil {
		t.Fatal(err)
	}
	if input.SchemaVersion != GlobalContextInputSchemaVersionV2 {
		t.Fatalf("schema=%s", input.SchemaVersion)
	}
	if len(input.ValidatedSignals) != len(chunkRun.ValidatedSignals) {
		t.Fatalf("signals=%d run=%d", len(input.ValidatedSignals), len(chunkRun.ValidatedSignals))
	}
	for factID := range pack.FactRegistry {
		if _, ok := input.FactCatalog[factID]; ok && !signalReferencesFact(input.ValidatedSignals, factID) {
			t.Fatalf("unrelated fact %s in catalog", factID)
		}
	}
}

func signalReferencesFact(signals []dashboardhome.DashboardChunkSignal, factID string) bool {
	for _, signal := range signals {
		for _, id := range signal.FactIDs {
			if id == factID {
				return true
			}
		}
	}
	return false
}

func TestMarshalGlobalContextV2WireExcludesDataPack(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	runner := NewDeterministicChunkAnalysisRunner()
	chunkRun, err := runner.AnalyzeDataPackChunks(t.Context(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildGlobalContextGenerationInput(pack, "hash-1", set, chunkRun)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"data_pack"`)) {
		t.Fatalf("payload contains data_pack: %s", string(raw))
	}
	for _, key := range []string{`"validated_signals"`, `"fact_catalog"`, `"entity_catalog"`} {
		if !bytes.Contains(raw, []byte(key)) {
			t.Fatalf("missing %s in %s", key, string(raw))
		}
	}
}

func TestValidateGlobalContextV2RejectsHiddenEntityID(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hiddenEntityID := "sku:80100019999"
	pack.EntityRegistry.SKUs[hiddenEntityID] = aidatapack.SKUEntity{
		EntityID: hiddenEntityID, EntityType: "sku", SKU: "80100019999",
		OfferID: "OFFER-HIDDEN", Name: "Hidden SKU",
	}
	runner := NewDeterministicChunkAnalysisRunner()
	chunkRun, err := runner.AnalyzeDataPackChunks(t.Context(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildGlobalContextGenerationInput(pack, "hash-1", set, chunkRun)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := input.EntityCatalog[hiddenEntityID]; ok {
		t.Fatal("hidden entity must not be in catalog")
	}
	if _, ok := pack.EntityRegistry.SKUs[hiddenEntityID]; !ok {
		t.Fatal("hidden entity must exist in pack registry")
	}
	if len(input.FactCatalog) == 0 {
		t.Fatal("expected at least one fact in catalog")
	}
	var allowedFact string
	for id := range input.FactCatalog {
		allowedFact = id
		break
	}
	hiddenRef := aicontext.SemanticEntityRef{
		EntityType: "sku",
		EntityID:   hiddenEntityID,
		Name:       "Hidden SKU",
	}

	cases := []struct {
		name string
		mut  func(*aicontext.GlobalSemanticContext)
	}{
		{
			name: "primary_risks_entities",
			mut: func(ctx *aicontext.GlobalSemanticContext) {
				ctx.PrimaryRisks = []aicontext.SemanticRisk{{
					RiskID: "risk-hidden", Category: "sku", Severity: "high",
					Title: "Hidden entity risk", EvidenceFactIDs: []string{allowedFact},
					Entities: []aicontext.SemanticEntityRef{hiddenRef},
				}}
			},
		},
		{
			name: "secondary_risks_entities",
			mut: func(ctx *aicontext.GlobalSemanticContext) {
				ctx.SecondaryRisks = []aicontext.SemanticRisk{{
					RiskID: "risk-hidden-2", Category: "sku", Severity: "medium",
					Title: "Hidden entity secondary", EvidenceFactIDs: []string{allowedFact},
					Entities: []aicontext.SemanticEntityRef{hiddenRef},
				}}
			},
		},
		{
			name: "opportunities_entities",
			mut: func(ctx *aicontext.GlobalSemanticContext) {
				ctx.Opportunities = []aicontext.SemanticSignal{{
					SignalID: "opp-hidden", Category: "sku",
					Title: "Hidden entity opportunity", EvidenceFactIDs: []string{allowedFact},
					Entities: []aicontext.SemanticEntityRef{hiddenRef},
				}}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := aicontext.GlobalSemanticContext{
				SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
				OverallStory:  "test story",
				Confidence:    "medium",
				SourceFactIDs: []string{allowedFact},
				PriorityOrder: []string{"sales"},
			}
			tc.mut(&ctx)
			result := ValidateGlobalSemanticContext(input, ctx)
			if result.Status != aicontext.ValidationStatusInvalid {
				t.Fatalf("status=%s issues=%+v", result.Status, result.Issues)
			}
			found := false
			for _, issue := range result.Issues {
				if issue.Code == "unknown_entity" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected unknown_entity issue, got %+v", result.Issues)
			}
		})
	}
}

func TestValidateGlobalContextV2NilDataPackReturnsFatal(t *testing.T) {
	input := GlobalContextGenerationInput{
		SchemaVersion:    GlobalContextInputSchemaVersionV2,
		SellerAccountID:  1,
		MetricDate:       "2026-06-25",
		DataPackHash:     "hash-1",
		ValidatedSignals: []dashboardhome.DashboardChunkSignal{},
		FactCatalog:      map[string]aidatapack.CompactFact{},
		EntityCatalog:    map[string]aidatapack.CompactEntity{},
		ManifestSummary:  map[string]any{},
		DataPack:         nil,
	}
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		OverallStory:  "test",
		Confidence:    "medium",
		SourceFactIDs: []string{},
		PriorityOrder: []string{"sales"},
	}
	result := ValidateGlobalSemanticContext(input, ctx)
	if result.Status != aicontext.ValidationStatusInvalid {
		t.Fatalf("status=%s", result.Status)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "missing_data_pack" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing_data_pack, got %+v", result.Issues)
	}
}

func TestValidateGlobalContextV2RejectsHiddenFactID(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hidden := "hidden.fact"
	f := pack.FactRegistry["account.current_day.revenue"]
	f.FactID = hidden
	pack.FactRegistry[hidden] = f
	runner := NewDeterministicChunkAnalysisRunner()
	chunkRun, err := runner.AnalyzeDataPackChunks(t.Context(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildGlobalContextGenerationInput(pack, "hash-1", set, chunkRun)
	if err != nil {
		t.Fatal(err)
	}
	ctx := aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		OverallStory:  "test",
		Confidence:    "medium",
		SourceFactIDs: []string{hidden},
		PriorityOrder: []string{"sales"},
	}
	result := ValidateGlobalSemanticContext(input, ctx)
	if result.Status != aicontext.ValidationStatusInvalid {
		t.Fatalf("status=%s", result.Status)
	}
}

func TestOpenAIClientPayloadUsesWireWithoutDataPack(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	chunkRun, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(t.Context(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	input, err := BuildGlobalContextGenerationInput(pack, "hash-1", set, chunkRun)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["data_pack"]; ok {
		t.Fatal("wire payload must not include data_pack")
	}
	if strings.TrimSpace(payload["schema_version"].(string)) != GlobalContextInputSchemaVersionV2 {
		t.Fatalf("schema=%v", payload["schema_version"])
	}
}

func TestGlobalContextPromptV2MentionsSignals(t *testing.T) {
	prompt := GlobalContextSystemPromptV2ForTest()
	for _, needle := range []string{
		"ValidatedSignals",
		"FactCatalog",
		"EntityCatalog",
		"failed_chunk_ids",
		"не повторно анализировать",
	} {
		if !strings.Contains(prompt, needle) {
			t.Fatalf("prompt missing %q", needle)
		}
	}
}
