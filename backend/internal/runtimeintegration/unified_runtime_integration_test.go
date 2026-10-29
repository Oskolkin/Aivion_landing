//go:build integration

package runtimeintegration

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func TestUnifiedRuntime_FullFlow(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigratedPool(t, dsn)
	metricDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	h := NewHarness(t, dsn, metricDate)
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	syncJob, err := h.Orchestrate.StartSync(ctx, sellerID, syncdomain.SyncTypeInitial.String())
	if err != nil {
		t.Fatalf("StartSync: %v", err)
	}

	h.WaitForPipelinePublished(t, sellerID, syncJob.ID, 3*time.Minute)

	t.Run("sync_job_completed", func(t *testing.T) {
		got, err := h.Queries.GetSyncJobByID(ctx, syncJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "completed" {
			t.Fatalf("sync status=%s", got.Status)
		}
	})

	t.Run("four_imports_completed", func(t *testing.T) {
		imports, err := h.Queries.ListImportJobsBySyncJobID(ctx, syncJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(imports) != 4 {
			t.Fatalf("import count=%d", len(imports))
		}
		seen := map[string]bool{}
		for _, ij := range imports {
			seen[ij.Domain] = true
			if ij.Status != "completed" {
				t.Fatalf("domain=%s status=%s", ij.Domain, ij.Status)
			}
		}
		for _, domain := range []string{"products", "orders", "stocks", "ads"} {
			if !seen[domain] {
				t.Fatalf("missing import domain %s", domain)
			}
		}
	})

	t.Run("enrichment_completed", func(t *testing.T) {
		run, err := h.Queries.GetLatestOzonEnrichmentSyncRunBySellerAndDate(ctx, dbgen.GetLatestOzonEnrichmentSyncRunBySellerAndDateParams{
			SellerAccountID: sellerID,
			MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != "completed" && run.Status != "completed_partial" {
			t.Fatalf("enrichment status=%s", run.Status)
		}
	})

	t.Run("pipeline_published", func(t *testing.T) {
		run, err := h.Queries.GetLatestPublishedAIPipelineRunBySeller(ctx, sellerID)
		if err != nil {
			t.Fatal(err)
		}
		if !run.PublishedAt.Valid {
			t.Fatal("pipeline not published")
		}
		if !run.MetricDate.Valid || run.MetricDate.Time.UTC().Format("2006-01-02") != metricDate.Format("2006-01-02") {
			t.Fatalf("pipeline metric_date=%v want %s", run.MetricDate, metricDate.Format("2006-01-02"))
		}
	})

	t.Run("actions_generated", func(t *testing.T) {
		var runCount int64
		if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM action_generation_runs
WHERE seller_account_id = $1
  AND metric_date = $2::date
  AND status IN ('completed', 'completed_repaired', 'completed_partial', 'completed_after_retry', 'empty')`,
			sellerID, metricDate).Scan(&runCount); err != nil {
			t.Fatal(err)
		}
		if runCount < 1 {
			t.Fatalf("action_generation_runs=%d", runCount)
		}
	})

	t.Run("no_legacy_ai_tasks", func(t *testing.T) {
		types := []string{
			jobs.TaskTypeOzonSyncCoordinator,
			jobs.TaskTypeOzonImportProducts,
			jobs.TaskTypeOzonImportOrders,
			jobs.TaskTypeOzonImportStocks,
			jobs.TaskTypeOzonImportAds,
			jobs.TaskTypeOzonEnrichmentP0,
			jobs.TaskTypePostSyncRecalculation,
			jobs.TaskTypeRunSellerAIPipeline,
		}
		assertUnifiedEnqueueContract(t, types)
	})

	t.Run("coordinator_retry_idempotent", func(t *testing.T) {
		coordinator := jobs.NewOzonSyncCoordinatorHandler(h.Pool, h.AsynqClient, zap.NewNop())
		task, err := jobs.NewOzonSyncCoordinatorTask(sellerID, syncJob.ID, syncdomain.SyncTypeInitial.String())
		if err != nil {
			t.Fatal(err)
		}
		if err := coordinator.Handle(ctx, task.Payload()); err != nil {
			t.Fatalf("coordinator retry: %v", err)
		}
		got, err := h.Queries.GetSyncJobByID(ctx, syncJob.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "completed" {
			t.Fatalf("sync status after coordinator retry=%s", got.Status)
		}
	})
}
