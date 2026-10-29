//go:build integration

package runtimeintegration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func TestLegacyContract_UnifiedModeAllowedTasks(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	metricDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	fake := testozon.Start(testozon.Options{MetricDate: metricDate})
	t.Cleanup(fake.Seller.Close)
	t.Cleanup(fake.Performance.Close)

	setupUnifiedRuntimeEnv(t, fake.Seller.URL, fake.Performance.URL)
	t.Setenv("DATABASE_URL", dsn)
	flushRedisDB(t, redisAddr(), testRedisDB)

	if err := aipipeline.ValidateRuntimeModeConfig(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	q := dbgen.New(pool)
	sellerID := seedContractSeller(t, ctx, q, metricDate)

	syncJob, err := q.CreateSyncJob(ctx, dbgen.CreateSyncJobParams{
		SellerAccountID: sellerID,
		Type:            syncdomain.SyncTypeInitial.String(),
		Status:          syncdomain.SyncStatusPending.String(),
		StartedAt:       pgtype.Timestamptz{Valid: false},
		FinishedAt:      pgtype.Timestamptz{Valid: false},
		ErrorMessage:    pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatal(err)
	}

	rec := NewRecordingEnqueuer(nil)
	coordinator := jobs.NewOzonSyncCoordinatorHandler(pool, rec, zap.NewNop())
	payload, err := json.Marshal(jobs.OzonSyncCoordinatorPayload{
		SellerAccountID: sellerID,
		SyncJobID:       syncJob.ID,
		SyncType:        syncdomain.SyncTypeInitial.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := coordinator.Handle(ctx, payload); err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	assertUnifiedEnqueueContract(t, rec.Types())

	// Simulate post-sync enqueue chain in unified mode.
	rec.Tasks = nil
	metricKey := metricDate.Format("2006-01-02")
	enrichmentTask, err := jobs.NewOzonEnrichmentP0Task(sellerID, syncJob.ID, metricKey, "p0")
	if err != nil {
		t.Fatal(err)
	}
	rec.Enqueue(enrichmentTask)
	recalcTask, err := jobs.NewRecalculateAfterSyncTask(sellerID, syncJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec.Enqueue(recalcTask)
	pipelineTask, err := jobs.NewRunSellerAIPipelineTask(sellerID, syncJob.ID, metricKey, "post_sync", false)
	if err != nil {
		t.Fatal(err)
	}
	rec.Enqueue(pipelineTask)
	assertUnifiedEnqueueContract(t, rec.Types())

	for _, typ := range rec.Types() {
		if typ == jobs.TaskTypeGenerateAIDailySummaries {
			t.Fatal("legacy summaries task must not be part of unified contract enqueue set")
		}
	}
}

func seedContractSeller(t *testing.T, ctx context.Context, q *dbgen.Queries, metricDate time.Time) int64 {
	t.Helper()
	email := fmt.Sprintf("legacy-contract-%d@test.invalid", time.Now().UnixNano())
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "contract", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if err := devseed.SeedOzonConnectionMock(ctx, q, sa.ID, time.Now().UTC(), testEncryptionKey); err != nil {
		t.Fatal(err)
	}
	return sa.ID
}
