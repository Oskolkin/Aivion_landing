package unifiedanalysis

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func TestServiceStartRunIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{
		result: validStubPack(packID),
		key:    "ai-analysis-packs/seller-1/2026-07-08/abc/analysis_pack.zip",
	}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun first: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun second: %v", err)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d, want 1", fake.StartResponseCount)
	}
	if fake.CreateContainerCount != 1 {
		t.Fatalf("CreateContainerCount=%d, want 1", fake.CreateContainerCount)
	}
	if fake.UploadCount != 1 {
		t.Fatalf("UploadCount=%d, want 1", fake.UploadCount)
	}
}
