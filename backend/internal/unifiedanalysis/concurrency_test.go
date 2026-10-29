package unifiedanalysis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func TestDuplicateCleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil || !poll.Completed {
		t.Fatalf("PollRun: err=%v completed=%v", err, poll != nil && poll.Completed)
	}

	if err := svc.CleanupRun(ctx, run.ID); err != nil {
		t.Fatalf("CleanupRun first: %v", err)
	}
	if err := svc.CleanupRun(ctx, run.ID); err != nil {
		t.Fatalf("CleanupRun second: %v", err)
	}
	final, _ := repo.GetRun(ctx, run.ID)
	if final.Status != StatusProviderCompleted {
		t.Fatalf("status=%q", final.Status)
	}
	if final.CleanupStatus != CleanupStatusCompleted {
		t.Fatalf("cleanup_status=%q", final.CleanupStatus)
	}
	if fake.DeleteFileCount > 2 || fake.DeleteContainerCount > 2 {
		t.Fatalf("delete counts file=%d container=%d", fake.DeleteFileCount, fake.DeleteContainerCount)
	}
}

func TestCleanupRetryAfterTemporaryFailure(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	fake.CleanupFailRemaining = 1
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if _, err := svc.PollRun(ctx, run.ID); err != nil {
		t.Fatalf("PollRun: %v", err)
	}

	if err := svc.CleanupRun(ctx, run.ID); err == nil {
		t.Fatal("expected first cleanup to fail")
	}
	mid, _ := repo.GetRun(ctx, run.ID)
	if mid.Status != StatusProviderCompleted {
		t.Fatalf("status after cleanup fail=%q", mid.Status)
	}
	if mid.CleanupStatus != CleanupStatusFailed {
		t.Fatalf("cleanup_status=%q", mid.CleanupStatus)
	}

	if err := svc.CleanupRun(ctx, run.ID); err != nil {
		t.Fatalf("CleanupRun retry: %v", err)
	}
	final, _ := repo.GetRun(ctx, run.ID)
	if final.Status != StatusProviderCompleted {
		t.Fatalf("final status=%q", final.Status)
	}
	if final.CleanupStatus != CleanupStatusCompleted {
		t.Fatalf("final cleanup=%q", final.CleanupStatus)
	}
}

func TestPollTimeoutCancelsAndFails(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioInProgressUntilTimeout)
	fake.SetSmokePackID(packID)
	cfg := testConfig()
	cfg.MaxPollDurationSeconds = 1
	cfg.PollIntervalSeconds = 1
	svc := NewServiceWithStore(repo, stub, fake, &memoryArtifactStore{repo: repo}, newMemoryPackObjectStore(), cfg)
	svc.clock = fixedClock{t: time.Now().UTC()}

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	// Force provider_started_at into the past for timeout.
	row := repo.runs[run.ID]
	row.ProviderStartedAt = pgtype.Timestamptz{Time: time.Now().UTC().Add(-3 * time.Second), Valid: true}
	repo.runs[run.ID] = row

	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if !poll.Failed {
		t.Fatalf("expected failed poll, status=%q", poll.Run.Status)
	}
	if poll.Run.ErrorCode != CodeProviderPollTimeout {
		t.Fatalf("error_code=%q", poll.Run.ErrorCode)
	}
	if fake.CancelCount != 1 {
		t.Fatalf("CancelCount=%d", fake.CancelCount)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
}

func TestConcurrentPollCompletesOnce(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.PollRun(ctx, run.ID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("PollRun: %v", err)
		}
	}
	final, _ := repo.GetRun(ctx, run.ID)
	if final.Status != StatusProviderCompleted {
		t.Fatalf("status=%q", final.Status)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
}

func TestPollCompletedWinsOverCancel(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil || !poll.Completed {
		t.Fatalf("PollRun: err=%v", err)
	}
	if err := svc.CancelRun(ctx, run.ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	final, _ := repo.GetRun(ctx, run.ID)
	if final.Status != StatusProviderCompleted {
		t.Fatalf("status=%q want provider_completed", final.Status)
	}
}

func TestCancelBeforePollMarksCancelled(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioInProgressUntilTimeout)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if err := svc.CancelRun(ctx, run.ID); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	final, _ := repo.GetRun(ctx, run.ID)
	if final.Status != StatusCancelled {
		t.Fatalf("status=%q", final.Status)
	}
	if fake.CancelCount < 1 {
		t.Fatalf("CancelCount=%d", fake.CancelCount)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
}

func TestPlanRecoveryProviderRunningAndCleanup(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioInProgressUntilTimeout)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	plans, err := svc.PlanRecovery(ctx, 10)
	if err != nil {
		t.Fatalf("PlanRecovery: %v", err)
	}
	found := false
	for _, p := range plans {
		if p.RunID == run.ID && p.Action == RecoveryActionPoll {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected poll recovery for run %d, plans=%v", run.ID, plans)
	}

	row := repo.runs[run.ID]
	row.Status = StatusProviderCompleted
	row.CleanupStatus = pgtype.Text{String: CleanupStatusPending, Valid: true}
	repo.runs[run.ID] = row

	plans, err = svc.PlanRecovery(ctx, 10)
	if err != nil {
		t.Fatalf("PlanRecovery cleanup: %v", err)
	}
	found = false
	for _, p := range plans {
		if p.RunID == run.ID && p.Action == RecoveryActionCleanup {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cleanup recovery, plans=%v", plans)
	}
}

func TestPlanRecoveryPublicationReadyAndPublishing(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioInProgressUntilTimeout)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	row := repo.runs[run.ID]
	row.Status = StatusPublicationReady
	row.CleanupStatus = pgtype.Text{String: CleanupStatusCompleted, Valid: true}
	repo.runs[run.ID] = row

	plans, err := svc.PlanRecovery(ctx, 10)
	if err != nil {
		t.Fatalf("PlanRecovery publication_ready: %v", err)
	}
	found := false
	for _, p := range plans {
		if p.RunID == run.ID && p.Action == RecoveryActionPublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected publish recovery for publication_ready run %d, plans=%v", run.ID, plans)
	}

	row = repo.runs[run.ID]
	row.Status = StatusPublishing
	repo.runs[run.ID] = row

	plans, err = svc.PlanRecovery(ctx, 10)
	if err != nil {
		t.Fatalf("PlanRecovery publishing: %v", err)
	}
	found = false
	for _, p := range plans {
		if p.RunID == run.ID && p.Action == RecoveryActionPublish {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected publish recovery for publishing run %d, plans=%v", run.ID, plans)
	}

	row = repo.runs[run.ID]
	row.Status = StatusPublished
	repo.runs[run.ID] = row
	plans, err = svc.PlanRecovery(ctx, 10)
	if err != nil {
		t.Fatalf("PlanRecovery published: %v", err)
	}
	for _, p := range plans {
		if p.RunID == run.ID && p.Action == RecoveryActionPublish {
			t.Fatalf("published run must not schedule publish recovery, plans=%v", plans)
		}
	}
}

func TestRetrieveTemporaryErrorThenComplete(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioRetrieveThenComplete)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if _, err := svc.PollRun(ctx, run.ID); err == nil {
		t.Fatal("expected first poll retrieve transport error")
	}
	mid, _ := repo.GetRun(ctx, run.ID)
	if mid.Status != StatusProviderRunning {
		t.Fatalf("status after transport error=%q", mid.Status)
	}
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil || !poll.Completed {
		t.Fatalf("second PollRun: err=%v completed=%v", err, poll != nil && poll.Completed)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
}

func TestPermanentProviderFailureCleansUpPending(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioFailed)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if !poll.Failed || poll.Run.Status != StatusFailed {
		t.Fatalf("expected failed, got completed=%v status=%q", poll.Completed, poll.Run.Status)
	}
	if poll.Run.CleanupStatus != CleanupStatusPending {
		t.Fatalf("cleanup_status=%q", poll.Run.CleanupStatus)
	}
	if err := svc.CleanupRun(ctx, run.ID); err != nil {
		t.Fatalf("CleanupRun: %v", err)
	}
}
