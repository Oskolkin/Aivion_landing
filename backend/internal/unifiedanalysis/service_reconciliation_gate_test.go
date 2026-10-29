package unifiedanalysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
)

// reconFailSnapshot returns a snapshot whose SKU rollup does not reconcile with
// the account totals (SKU carries orders but zero revenue), which sourcerecon
// flags as both account_sku_revenue_mismatch and sku_orders_without_revenue.
func reconFailSnapshot(sellerID int64, metricDate time.Time) *analysisdata.AnalysisDataSnapshot {
	var accountRevenue pgtype.Numeric
	_ = accountRevenue.Scan("100.00")
	return &analysisdata.AnalysisDataSnapshot{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		AccountMetrics: []dbgen.DailyAccountMetric{
			{SellerAccountID: sellerID, MetricDate: pgtype.Date{Time: metricDate, Valid: true}, Revenue: accountRevenue, OrdersCount: 2},
		},
		SKUMetrics: []dbgen.DailySkuMetric{
			{
				SellerAccountID: sellerID,
				MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
				OzonProductID:   1001,
				OrdersCount:     2,
				// Revenue left as the zero-value (invalid) numeric on purpose.
			},
		},
	}
}

func errorCode(t *testing.T, err error) string {
	t.Helper()
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	t.Fatalf("error is not a typed *Error: %v", err)
	return ""
}

// TestGateSourceReconciliationBlocksAndPersistsReport verifies the pre-OpenAI
// gate fails the run with CodeSourceReconciliationFailed and persists the
// detailed (failing) report artifact when the pack snapshot does not reconcile.
func TestGateSourceReconciliationBlocksAndPersistsReport(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	fake := provider.NewFakeProvider(provider.FakeScenarioQueuedThenCompleted)
	svc := newTestService(repo, &stubPackBuilder{}, fake)

	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	run, err := svc.CreateRun(ctx, 1, metricDate, false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	result := &pack.BuildResult{Snapshot: reconFailSnapshot(1, metricDate)}

	gateErr := svc.gateSourceReconciliation(ctx, run.ID, result)
	if gateErr == nil {
		t.Fatal("expected gate to fail on an unreconciled snapshot")
	}
	if code := errorCode(t, gateErr); code != CodeSourceReconciliationFailed {
		t.Fatalf("gate error code=%q want %q", code, CodeSourceReconciliationFailed)
	}

	failed, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("status=%q want %q", failed.Status, StatusFailed)
	}
	if failed.ErrorCode != CodeSourceReconciliationFailed {
		t.Fatalf("run error code=%q want %q", failed.ErrorCode, CodeSourceReconciliationFailed)
	}

	report, ok := svc.artifacts.LoadSourceReconciliation(ctx, run.ID)
	if !ok {
		t.Fatal("expected a reconciliation report artifact to be persisted on failure")
	}
	if report.Valid {
		t.Fatal("persisted reconciliation report should be invalid")
	}
	if len(report.Errors) == 0 {
		t.Fatal("persisted reconciliation report should carry at least one error")
	}
}

// TestGateSourceReconciliationPassesOnCleanSnapshot verifies the gate is a
// no-op (and stores a passing report) when the snapshot reconciles.
func TestGateSourceReconciliationPassesOnCleanSnapshot(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	fake := provider.NewFakeProvider(provider.FakeScenarioQueuedThenCompleted)
	svc := newTestService(repo, &stubPackBuilder{}, fake)

	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	run, err := svc.CreateRun(ctx, 1, metricDate, false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	result := &pack.BuildResult{Snapshot: minimalTestSnapshot(1, metricDate)}
	if err := svc.gateSourceReconciliation(ctx, run.ID, result); err != nil {
		t.Fatalf("gate should pass on a reconciled snapshot: %v", err)
	}

	report, ok := svc.artifacts.LoadSourceReconciliation(ctx, run.ID)
	if !ok {
		t.Fatal("expected a reconciliation report artifact on success")
	}
	if !report.Valid {
		t.Fatalf("expected a valid reconciliation report, got errors: %v", report.Errors)
	}
}

// TestStartRunSkipsProviderWhenSnapshotUnreconciled proves the reconciliation
// math blocks the run before any OpenAI/provider interaction: an unreconciled
// snapshot must yield zero provider calls.
func TestStartRunSkipsProviderWhenSnapshotUnreconciled(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{
		result: validStubPack(packID),
		key:    "ai-analysis-packs/seller-1/2026-07-08/abc/analysis_pack.zip",
	}
	// Replace the (reconciled) default snapshot with one that fails.
	stub.result.Snapshot = reconFailSnapshot(1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))

	fake := provider.NewFakeProvider(provider.FakeScenarioQueuedThenCompleted)
	fake.SetSmokePackID(packID)
	svc := newTestService(repo, stub, fake)

	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	if err := svc.StartRun(ctx, run.ID); err == nil {
		t.Fatal("expected StartRun to fail on an unreconciled snapshot")
	}

	if fake.CreateContainerCount != 0 {
		t.Fatalf("CreateContainerCount=%d want 0", fake.CreateContainerCount)
	}
	if fake.UploadCount != 0 {
		t.Fatalf("UploadCount=%d want 0", fake.UploadCount)
	}
	if fake.StartResponseCount != 0 {
		t.Fatalf("StartResponseCount=%d want 0", fake.StartResponseCount)
	}

	failed, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("status=%q want %q", failed.Status, StatusFailed)
	}
}
