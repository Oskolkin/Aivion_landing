package migrationpreflight_test

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db/migrationpreflight"
)

func TestImportJobStatusRank(t *testing.T) {
	if migrationpreflight.ImportJobStatusRank("completed") >= migrationpreflight.ImportJobStatusRank("failed") {
		t.Fatal("completed should outrank failed")
	}
}

func TestLegacyDedupBaseDomain(t *testing.T) {
	got := migrationpreflight.LegacyDedupBaseDomain("stocks__dedup_42")
	if got != "stocks" {
		t.Fatalf("got %q", got)
	}
}

func TestActionCandidateCount(t *testing.T) {
	if migrationpreflight.ActionCandidateCount(2) != "ambiguous" {
		t.Fatal("expected ambiguous")
	}
}

func TestActiveSyncStatusRank(t *testing.T) {
	if migrationpreflight.ActiveSyncStatusRank("running") >= migrationpreflight.ActiveSyncStatusRank("pending") {
		t.Fatal("running should outrank pending")
	}
}

func TestPickCanonicalImportJobDeterministic(t *testing.T) {
	now := time.Now().UTC()
	finished := now.Add(-time.Hour)
	rows := []migrationpreflight.ImportJobRankInput{
		{Status: "failed", ID: 1, FinishedAt: &finished, UpdatedAt: now, CreatedAt: now},
		{Status: "completed", ID: 2, FinishedAt: &finished, UpdatedAt: now, CreatedAt: now},
		{Status: "pending", ID: 3, FinishedAt: &finished, UpdatedAt: now, CreatedAt: now},
	}
	got := migrationpreflight.PickCanonicalImportJob(rows)
	if got.ID != 2 {
		t.Fatalf("canonical id=%d want 2", got.ID)
	}
}
