package unifiedanalysis

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func TestProviderRequestIdentity_CanonicalHashes(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetAnalysisOutputJSON([]byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`))
	arts := &memoryArtifactStore{repo: repo}
	cfg := analysisTestConfig()
	svc := NewServiceWithStore(repo, stub, fake, arts, newMemoryPackObjectStore(), cfg)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	repo.mu.Lock()
	row := repo.runs[run.ID]
	row.PromptVersion = nullableText(PromptVersionAnalyst)
	repo.runs[run.ID] = row
	repo.mu.Unlock()

	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	reqs := arts.ListArtifactsByKind(ctx, run.ID, ArtifactUnifiedProviderRequestSummary)
	if len(reqs) == 0 {
		t.Fatal("expected request summary")
	}
	var summary providerRequestSummary
	if err := json.Unmarshal(reqs[0], &summary); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	wantPromptHash := "e7dc8b317cbb7eb7ce6dae632cd4487edd19db1a3685f6098ce4f9296ccb1a45"
	wantSchemaHash := contract.CanonicalSchemaHashV10

	if summary.PromptVersion != prompt.VersionAnalystV1_31 {
		t.Fatalf("prompt_version=%q", summary.PromptVersion)
	}
	if summary.PromptHash != wantPromptHash {
		t.Fatalf("prompt_hash=%q want %q", summary.PromptHash, wantPromptHash)
	}
	if summary.SchemaVersion != contract.UnifiedAnalysisOutputSchemaVersionV10 {
		t.Fatalf("schema_version=%q", summary.SchemaVersion)
	}
	if summary.SchemaHash != wantSchemaHash {
		t.Fatalf("schema_hash=%q want %q", summary.SchemaHash, wantSchemaHash)
	}
	if summary.ToolChoice != "auto" {
		t.Fatalf("tool_choice=%q", summary.ToolChoice)
	}

	// Bytes sent to provider must be LF-normalized. Prompt hash in the summary
	// is the immutable static template hash (runtime pack-identity appends are
	// not part of prompt_hash by design).
	if strings.Contains(fake.LastStartRequest.Instructions, "\r") {
		t.Fatal("provider instructions contain CR")
	}
	if strings.Contains(fake.LastStartRequest.Input, "\r") {
		t.Fatal("provider input contain CR")
	}
	tpl, err := prompt.TemplateByVersion(prompt.VersionAnalystV1_31)
	if err != nil {
		t.Fatal(err)
	}
	if fake.LastStartRequest.Instructions != tpl.Instructions {
		t.Fatal("provider instructions must equal canonical template instructions")
	}
	if !strings.HasPrefix(fake.LastStartRequest.Input, tpl.Input) {
		t.Fatal("provider input must start with canonical template input")
	}
	if prompt.Hash(tpl.Instructions, tpl.Input) != summary.PromptHash {
		t.Fatal("summary prompt_hash must equal Hash(canonical template)")
	}
	if fake.LastStartRequest.TextFormat == nil {
		t.Fatal("expected structured output schema on analysis request")
	}
	schemaBytes := []byte(fake.LastStartRequest.TextFormat.Schema)
	if strings.Contains(string(schemaBytes), "\r") {
		t.Fatal("provider schema contains CR")
	}
	canonical, err := contract.SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(schemaBytes) != string(canonical) {
		t.Fatal("provider schema bytes must equal SchemaJSON()")
	}
}
