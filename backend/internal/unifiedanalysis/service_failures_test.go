package unifiedanalysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func TestServicePollRunFailsWithoutCodeInterpreter(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{
		result: validStubPack(packID),
		key:    "ai-analysis-packs/seller-1/2026-07-08/abc/analysis_pack.zip",
	}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithoutCodeInterpreter)
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
	if poll == nil || !poll.Failed {
		t.Fatal("expected failed poll result for smoke validation failure")
	}

	failed, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("status=%q", failed.Status)
	}
	if failed.ErrorCode != CodeSmokeValidationFailed {
		t.Fatalf("error_code=%q", failed.ErrorCode)
	}
}

func TestServiceStartRunFailsOnInvalidPack(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	invalid := validStubPack("pack:invalid")
	invalid.Validation.Valid = false
	stub := &stubPackBuilder{
		result: invalid,
		key:    "key",
		err:    pack.ErrValidationFailed,
	}
	fake := provider.NewFakeProvider(provider.FakeScenarioCompletedWithCodeInterpreter)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	err = svc.StartRun(ctx, run.ID)
	if err == nil {
		t.Fatal("expected pack validation failure")
	}
	var ue *Error
	if !errors.As(err, &ue) || ue.Code != CodePackValidationFailed {
		t.Fatalf("expected pack validation error, got %v", err)
	}
	if fake.CreateContainerCount != 0 {
		t.Fatalf("CreateContainerCount=%d", fake.CreateContainerCount)
	}
}
