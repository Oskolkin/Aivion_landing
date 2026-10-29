package ingestion

import (
	"errors"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestNormalizeSyncType_emptyUsesIncremental(t *testing.T) {
	if got := syncdomain.NormalizeSyncType(""); got != syncdomain.SyncTypeIncremental {
		t.Fatalf("got %q want incremental_sync", got)
	}
	if got := syncdomain.NormalizeSyncType("periodic_sync"); got != syncdomain.SyncTypeIncremental {
		t.Fatalf("got %q want incremental_sync", got)
	}
}

func TestStartSync_enqueueDuplicateTaskIsSuccess(t *testing.T) {
	enq := &fakeEnqueuer{err: asynq.ErrDuplicateTask}
	_, err := enq.Enqueue(asynq.NewTask(syncdomain.TaskTypeOzonSyncCoordinator, []byte(`{}`)), asynq.TaskID("ozon-sync:1"))
	if !errors.Is(err, asynq.ErrDuplicateTask) {
		t.Fatalf("expected duplicate task error")
	}
}

func TestIsActiveSyncUniqueViolation(t *testing.T) {
	if isActiveSyncUniqueViolation(errors.New("other")) {
		t.Fatal("expected false")
	}
}

func TestSyncDomainConstants(t *testing.T) {
	if syncdomain.SyncTypeInitial.String() != "initial_sync" {
		t.Fatal(syncdomain.SyncTypeInitial)
	}
	if len(syncdomain.AllImportDomains) != 4 {
		t.Fatalf("expected 4 domains")
	}
}

func TestCreateSyncJobParamsTypes(t *testing.T) {
	_ = dbgen.CreateSyncJobParams{
		Type:      syncdomain.SyncTypeIncremental.String(),
		Status:    syncdomain.SyncStatusPending.String(),
		StartedAt: pgtype.Timestamptz{Valid: false},
	}
}
