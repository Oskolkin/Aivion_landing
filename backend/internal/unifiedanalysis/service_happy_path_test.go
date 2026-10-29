package unifiedanalysis

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func TestServiceHappyPathCreateStartPollCleanup(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{
		result: validStubPack(packID),
		key:    "ai-analysis-packs/seller-1/2026-07-08/abc/analysis_pack.zip",
	}
	fake := provider.NewFakeProvider(provider.FakeScenarioQueuedThenCompleted)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if fake.CreateContainerCount != 1 {
		t.Fatalf("CreateContainerCount=%d", fake.CreateContainerCount)
	}
	if fake.UploadCount != 1 {
		t.Fatalf("UploadCount=%d", fake.UploadCount)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}

	started, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if started.Status != StatusProviderRunning {
		t.Fatalf("status after start=%q", started.Status)
	}

	fake.AdvanceResponse(started.ProviderResponseID)
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if !poll.Completed {
		t.Fatalf("expected completed poll result, status=%q", poll.Run.Status)
	}
	if poll.Run.Status != StatusProviderCompleted {
		t.Fatalf("status after poll=%q", poll.Run.Status)
	}
	if poll.Run.CleanupStatus != CleanupStatusPending {
		t.Fatalf("cleanup_status=%q", poll.Run.CleanupStatus)
	}

	if err := svc.CleanupRun(ctx, run.ID); err != nil {
		t.Fatalf("CleanupRun: %v", err)
	}
	if fake.DeleteFileCount != 1 {
		t.Fatalf("DeleteFileCount=%d", fake.DeleteFileCount)
	}
	if fake.DeleteContainerCount != 1 {
		t.Fatalf("DeleteContainerCount=%d", fake.DeleteContainerCount)
	}
}
