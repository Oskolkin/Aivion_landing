package unifiedanalysis

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func TestValidateOutputNoActionsPublicationReady(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack_example_v1"
	key := "ai-analysis-packs/seller-1/2026-07-08/hash/analysis_pack.zip"
	stub := &stubPackBuilder{
		result: validStubPack(packID),
		key:    key,
	}
	fixturePath := filepath.Join("contract", "testdata", "unified_output_v1.no_actions.json")
	fixture, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(fixture, &out); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	out.AnalysisMeta.PackID = packID
	out.AnalysisMeta.PackHash = stub.result.PackHash
	outBytes, _ := json.Marshal(out)

	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	fake.SetAnalysisOutputJSON(outBytes)

	cfg := testConfig()
	cfg.PromptProfile = "analysis"
	cfg.PromptVersion = PromptVersionAnalyst
	cfg.OutputValidationEnabled = true
	packStore := newMemoryPackObjectStore()
	packStore.Put(key, stub.result.ZipBytes)
	arts := &memoryArtifactStore{repo: repo}
	svc := NewServiceWithStore(repo, stub, fake, arts, packStore, cfg)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	started, _ := repo.GetRun(ctx, run.ID)
	fake.AdvanceResponse(started.ProviderResponseID)
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if poll.Run.Status != StatusOutputValidating {
		t.Fatalf("status=%q want output_validating", poll.Run.Status)
	}

	result, err := svc.ValidateOutput(ctx, run.ID)
	if err != nil {
		t.Fatalf("ValidateOutput: %v", err)
	}
	if result == nil || !result.Valid {
		t.Fatalf("expected valid result, report=%+v", result)
	}
	if result.Run.Status != StatusPublicationReady {
		t.Fatalf("status=%q want publication_ready", result.Run.Status)
	}
	if result.OutputHash == "" {
		t.Fatal("expected output hash")
	}
	if result.Outcome != contract.AnalysisOutcomeNoMaterialActions {
		t.Fatalf("outcome=%q", result.Outcome)
	}

	// Idempotent re-validate.
	again, err := svc.ValidateOutput(ctx, run.ID)
	if err != nil {
		t.Fatalf("ValidateOutput again: %v", err)
	}
	if again.OutputHash != result.OutputHash {
		t.Fatalf("hash changed on revalidate: %q vs %q", again.OutputHash, result.OutputHash)
	}

	vals := arts.ListArtifactsByKind(ctx, run.ID, ArtifactUnifiedAnalysisOutputValidation)
	if len(vals) == 0 {
		t.Fatal("expected output validation artifact")
	}
	var validationArt outputValidationArtifact
	if err := json.Unmarshal(vals[0], &validationArt); err != nil {
		t.Fatalf("unmarshal validation artifact: %v", err)
	}
	if validationArt.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionCurrent {
		t.Fatalf("validation report schema_version=%q want %q", validationArt.SchemaVersion, contract.UnifiedAnalysisOutputSchemaVersionCurrent)
	}
	if validationArt.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("runtime schema is v10; report must show aivion_unified_analysis_v10, got %q", validationArt.SchemaVersion)
	}
}

func minimalReferencePackZip(t *testing.T, packID string) []byte {
	t.Helper()
	zipBytes, _ := mustMinimalValidPackZip(packID, 1, "2026-07-08")
	return zipBytes
}
