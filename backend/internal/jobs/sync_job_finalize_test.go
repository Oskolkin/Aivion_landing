package jobs

import (
	"context"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
)

type stubParentSyncFinalizer struct {
	failResult dbgen.SyncJob
	failErr    error
	okResult   dbgen.SyncJob
	okErr      error
	nFailCalls int
	nOKCalls   int
	lastFailID int64
	lastOKID   int64
}

func (s *stubParentSyncFinalizer) TryFinalizeSyncJobFailedIfNonTerminal(ctx context.Context, arg dbgen.TryFinalizeSyncJobFailedIfNonTerminalParams) (dbgen.SyncJob, error) {
	s.nFailCalls++
	s.lastFailID = arg.ID
	return s.failResult, s.failErr
}

func (s *stubParentSyncFinalizer) TryFinalizeSyncJobCompletedIfNonTerminal(ctx context.Context, id int64) (dbgen.SyncJob, error) {
	s.nOKCalls++
	s.lastOKID = id
	return s.okResult, s.okErr
}

func TestRunParentSyncJobFinalization_skipsNonPositiveSyncJobID(t *testing.T) {
	t.Parallel()
	stub := &stubParentSyncFinalizer{}
	res, err := runParentSyncJobFinalization(context.Background(), stub, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SyncJobJustCompleted {
		t.Fatal("unexpected completed flag")
	}
	if stub.nFailCalls != 0 || stub.nOKCalls != 0 {
		t.Fatalf("expected no calls, got fail=%d ok=%d", stub.nFailCalls, stub.nOKCalls)
	}
}

func TestRunParentSyncJobFinalization_propagatesRealErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	want := context.DeadlineExceeded
	stub := &stubParentSyncFinalizer{failErr: want}
	if _, err := runParentSyncJobFinalization(ctx, stub, 42); err == nil {
		t.Fatal("expected error")
	}
	if stub.nFailCalls != 1 || stub.nOKCalls != 0 {
		t.Fatalf("expected fail only, got fail=%d ok=%d", stub.nFailCalls, stub.nOKCalls)
	}
}

func TestRunParentSyncJobFinalization_callsTryCompletedAfterTryFailedNoRows(t *testing.T) {
	t.Parallel()
	stub := &stubParentSyncFinalizer{
		failErr: pgx.ErrNoRows,
		okErr:   pgx.ErrNoRows,
	}
	res, err := runParentSyncJobFinalization(context.Background(), stub, 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SyncJobJustCompleted {
		t.Fatal("unexpected SyncJobJustCompleted")
	}
	if stub.nFailCalls != 1 || stub.nOKCalls != 1 {
		t.Fatalf("expected one call each, got fail=%d ok=%d", stub.nFailCalls, stub.nOKCalls)
	}
	if stub.lastFailID != 7 || stub.lastOKID != 7 {
		t.Fatalf("wrong ids: fail=%d ok=%d", stub.lastFailID, stub.lastOKID)
	}
}

func TestRunParentSyncJobFinalization_tryCompletedAfterTryFailedSucceeds(t *testing.T) {
	t.Parallel()
	stub := &stubParentSyncFinalizer{
		failErr: pgx.ErrNoRows,
		okResult: dbgen.SyncJob{
			ID:     9,
			Status: "completed",
		},
	}
	res, err := runParentSyncJobFinalization(context.Background(), stub, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.SyncJobJustCompleted {
		t.Fatal("expected SyncJobJustCompleted")
	}
	if stub.nFailCalls != 1 || stub.nOKCalls != 1 {
		t.Fatalf("expected one call each, got fail=%d ok=%d", stub.nFailCalls, stub.nOKCalls)
	}
}

func TestRunParentSyncJobFinalization_tryFailedSucceedsThenCompletedNoRows(t *testing.T) {
	t.Parallel()
	stub := &stubParentSyncFinalizer{
		failResult: dbgen.SyncJob{ID: 3, Status: "failed"},
		okErr:      pgx.ErrNoRows,
	}
	res, err := runParentSyncJobFinalization(context.Background(), stub, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SyncJobJustCompleted {
		t.Fatal("unexpected SyncJobJustCompleted when sync failed path")
	}
	if stub.nFailCalls != 1 || stub.nOKCalls != 1 {
		t.Fatalf("expected try failed then try completed, got fail=%d ok=%d", stub.nFailCalls, stub.nOKCalls)
	}
}

func TestRunParentSyncJobFinalization_tryCompletedPropagatesNonNoRows(t *testing.T) {
	t.Parallel()
	want := context.Canceled
	stub := &stubParentSyncFinalizer{
		failErr: pgx.ErrNoRows,
		okErr:   want,
	}
	if _, err := runParentSyncJobFinalization(context.Background(), stub, 2); err == nil {
		t.Fatal("expected error")
	}
}
