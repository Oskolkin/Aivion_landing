//go:build integration

package runtimeintegration

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
)

func TestFullPreviousSnapshotSurvival_DashboardFailurePreservesFingerprint(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := h.MetricDate

	_ = publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSellerFacingFingerprint(t, ctx, h, sellerID)

	svc := buildSurvivalPipelineService(h, survivalOptions{dashboardFail: true})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID: sellerID, MetricDate: metricDate,
		GenerateDashboard: true, GenerateActions: true, ForceRegenerate: true, AIEnabled: false, Trigger: "integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Published {
		t.Fatal("failed P2 must not publish")
	}
	assertCurrentPipeline(t, ctx, h, sellerID, before.PipelineRunID)
	after := loadSellerFacingFingerprint(t, ctx, h, sellerID)
	assertSellerFacingFingerprintEqual(t, before, after)
}

func TestFullPreviousSnapshotSurvival_ActionsFailurePreservesFingerprint(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := h.MetricDate

	_ = publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSellerFacingFingerprint(t, ctx, h, sellerID)

	svc := buildSurvivalPipelineService(h, survivalOptions{actionsFail: true})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID: sellerID, MetricDate: metricDate,
		GenerateDashboard: true, GenerateActions: true, ForceRegenerate: true, AIEnabled: false, Trigger: "integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Published {
		t.Fatal("failed P2 must not publish")
	}
	assertCurrentPipeline(t, ctx, h, sellerID, before.PipelineRunID)
	after := loadSellerFacingFingerprint(t, ctx, h, sellerID)
	assertSellerFacingFingerprintEqual(t, before, after)
}

func TestFullPreviousSnapshotSurvival_HashMismatchPreservesFingerprint(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := h.MetricDate

	_ = publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSellerFacingFingerprint(t, ctx, h, sellerID)

	p2 := createCompletedPipelineAttempt(t, ctx, h, sellerID, metricDate, "p2", false, false, false)
	if _, err := h.Pool.Exec(ctx, `UPDATE action_generation_runs SET data_pack_hash = 'sha256:broken' WHERE ai_pipeline_run_id = $1`, p2); err != nil {
		t.Fatal(err)
	}
	publisher := aipipeline.NewSQLCSnapshotPublisher(h.Pool)
	if _, err := publisher.PublishPipelineSnapshot(ctx, p2); err == nil {
		t.Fatal("expected publication rejection")
	}
	assertCurrentPipeline(t, ctx, h, sellerID, before.PipelineRunID)
	after := loadSellerFacingFingerprint(t, ctx, h, sellerID)
	assertSellerFacingFingerprintEqual(t, before, after)
}

func TestFullPreviousSnapshotSurvival_SuccessfulSwitchUpdatesFingerprint(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	h := NewHarness(t, dsn, time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC))
	sellerID := h.SeedSeller(t)
	ctx := context.Background()
	metricDate := h.MetricDate

	p1 := publishFixturePipeline(t, ctx, h, sellerID, metricDate, "p1")
	before := loadSellerFacingFingerprint(t, ctx, h, sellerID)

	svc := buildSurvivalPipelineService(h, survivalOptions{})
	svc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(h.Pool))
	res, err := svc.Run(ctx, aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID: sellerID, MetricDate: metricDate,
		GenerateDashboard: true, GenerateActions: true, ForceRegenerate: true, AIEnabled: false, Trigger: "integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Published {
		t.Fatal("expected successful P2 publication")
	}
	after := loadSellerFacingFingerprint(t, ctx, h, sellerID)
	if after.PipelineRunID == p1 || after.PipelineRunID == before.PipelineRunID {
		t.Fatalf("current must switch away from P1, got %d", after.PipelineRunID)
	}
	if after.PipelineRunID != res.PipelineRunID {
		t.Fatalf("current pipeline=%d run result=%d", after.PipelineRunID, res.PipelineRunID)
	}
}
