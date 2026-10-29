//go:build integration

package runtimeintegration

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestFullPublishedSnapshotContract_NoPublishedSnapshot(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	runID := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, h.MetricDate, time.Now().UTC())
	_, err := publishedsnapshot.NewSQLCResolver(h.Queries).GetCurrentPublishedSnapshot(ctx, sellerID)
	if err == nil {
		t.Fatal("expected no current published snapshot")
	}
	if !isNoRows(err) {
		t.Fatalf("expected no rows, got %v", err)
	}
	_ = runID
}

func TestFullPublishedSnapshotContract_P1UnifiedSnapshot(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	p1 := publishFixturePipeline(t, ctx, h, sellerID, h.MetricDate, "p1")
	fp := loadSellerFacingFingerprint(t, ctx, h, sellerID)
	if fp.PipelineRunID != p1 {
		t.Fatalf("pipeline=%d want %d", fp.PipelineRunID, p1)
	}
	if fp.DashboardDataSnapshotID <= 0 || fp.DashboardPayloadHash == "" {
		t.Fatal("dashboard data snapshot must be linked with payload hash")
	}
	if fp.ActionsRunID <= 0 || fp.ActionTitle == "" {
		t.Fatal("actions run must be published")
	}
	if len(fp.EntityGroupKeys) == 0 {
		t.Fatal("entity groups must belong to published actions run")
	}
}

func TestFullPublishedSnapshotContract_MutableMetricsDoNotChangeDashboard(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()

	_ = publishFixturePipeline(t, ctx, h, sellerID, h.MetricDate, "p1")
	before := loadSellerFacingFingerprint(t, ctx, h, sellerID)

	if _, err := h.Pool.Exec(ctx, `
INSERT INTO daily_account_metrics (seller_account_id, metric_date, revenue, orders_count, updated_at)
VALUES ($1, $2, 999999, 999, now())
ON CONFLICT (seller_account_id, metric_date) DO UPDATE
SET revenue = EXCLUDED.revenue, orders_count = EXCLUDED.orders_count, updated_at = now()
`, sellerID, pgtype.Date{Time: h.MetricDate, Valid: true}); err != nil {
		t.Fatalf("mutate metrics: %v", err)
	}

	after := loadSellerFacingFingerprint(t, ctx, h, sellerID)
	assertSellerFacingFingerprintEqual(t, before, after)
}

func TestFullPublishedSnapshotContract_HistoricalRepublishIsNotCurrent(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)

	p1 := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, h.MetricDate, time.Now().UTC().Add(-2*time.Minute))
	p2 := createPublishablePipelineRun(t, ctx, h.Pool, h.Queries, sellerID, h.MetricDate, time.Now().UTC().Add(-1*time.Minute))
	if _, err := publisher.PublishPipelineSnapshot(ctx, p1); err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.PublishPipelineSnapshot(ctx, p2); err != nil {
		t.Fatal(err)
	}
	republish, err := publisher.PublishPipelineSnapshot(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}
	if !republish.AlreadyPublished || republish.IsCurrent {
		t.Fatalf("historical republish semantics: %+v", republish)
	}
}

func isNoRows(err error) bool {
	return err != nil && (err == pgx.ErrNoRows || err.Error() == "no rows in result set")
}
