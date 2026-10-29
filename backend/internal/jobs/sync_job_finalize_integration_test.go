//go:build integration

package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func testSyncJobFinalizeDSN(t *testing.T) string {
	t.Helper()
	for _, k := range []string{"TEST_DATABASE_URL", "SYNC_JOB_FINALIZE_TEST_DATABASE_URL"} {
		if v := os.Getenv(k); v != "" {
			testutil.RequireSafeTestDatabaseURL(t, v)
			return v
		}
	}
	t.Skip("set TEST_DATABASE_URL or SYNC_JOB_FINALIZE_TEST_DATABASE_URL to run integration tests")
	return ""
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))
}

func openTestPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("integration db unavailable (connect): %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration db unavailable (ping): %v", err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return pool
}

func syncJobStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, syncJobID int64) string {
	t.Helper()
	var st string
	if err := pool.QueryRow(ctx, `SELECT status FROM sync_jobs WHERE id = $1`, syncJobID).Scan(&st); err != nil {
		t.Fatalf("read sync job status: %v", err)
	}
	return st
}

func seedUserSellerSync(t *testing.T, ctx context.Context, q *dbgen.Queries) (sellerID int64, syncID int64) {
	t.Helper()
	email := fmt.Sprintf("sync-finalize-%d-%d@test.invalid", time.Now().UnixNano(), time.Now().Nanosecond())
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: "x",
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{
		UserID: u.ID,
		Name:   "t",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("CreateSellerAccount: %v", err)
	}
	sj, err := q.CreateSyncJob(ctx, dbgen.CreateSyncJobParams{
		SellerAccountID: sa.ID,
		Type:            "initial_sync",
		Status:          "running",
		StartedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
		FinishedAt:      pgtype.Timestamptz{Valid: false},
		ErrorMessage:    pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("CreateSyncJob: %v", err)
	}
	return sa.ID, sj.ID
}

func createImportJob(t *testing.T, ctx context.Context, q *dbgen.Queries, sellerID, syncID int64, domain string) int64 {
	t.Helper()
	ij, err := q.CreateImportJob(ctx, dbgen.CreateImportJobParams{
		SellerAccountID: sellerID,
		SyncJobID:       syncID,
		Domain:          domain,
		Status:          "pending",
		SourceCursor:    pgtype.Text{Valid: false},
		RecordsReceived: 0,
		RecordsImported: 0,
		RecordsFailed:   0,
		StartedAt:       pgtype.Timestamptz{Valid: false},
		FinishedAt:      pgtype.Timestamptz{Valid: false},
		ErrorMessage:    pgtype.Text{Valid: false},
	})
	if err != nil {
		t.Fatalf("CreateImportJob: %v", err)
	}
	return ij.ID
}

func TestIntegration_syncJobFinalize_allImportJobsCompleted(t *testing.T) {
	dsn := testSyncJobFinalizeDSN(t)
	pool := openTestPool(t, dsn)
	ctx := context.Background()
	q := dbgen.New(pool)

	sellerID, syncID := seedUserSellerSync(t, ctx, q)
	_ = createImportJob(t, ctx, q, sellerID, syncID, "products")
	_ = createImportJob(t, ctx, q, sellerID, syncID, "orders")

	imports, err := q.ListImportJobsBySyncJobID(ctx, syncID)
	if err != nil || len(imports) != 2 {
		t.Fatalf("expected 2 import jobs, got %d err %v", len(imports), err)
	}
	for _, ij := range imports {
		if _, err := q.UpdateImportJobToFetching(ctx, ij.ID); err != nil {
			t.Fatalf("UpdateImportJobToFetching: %v", err)
		}
		if _, err := q.UpdateImportJobToImporting(ctx, ij.ID); err != nil {
			t.Fatalf("UpdateImportJobToImporting: %v", err)
		}
		if _, err := q.UpdateImportJobToCompleted(ctx, dbgen.UpdateImportJobToCompletedParams{
			ID: ij.ID, RecordsReceived: 0, RecordsImported: 0, RecordsFailed: 0,
		}); err != nil {
			t.Fatalf("UpdateImportJobToCompleted: %v", err)
		}
		if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
			t.Fatalf("finalize after import %d: %v", ij.ID, err)
		}
	}

	if st := syncJobStatus(t, ctx, pool, syncID); st != "completed" {
		t.Fatalf("want sync_job completed, got %q", st)
	}
}

func TestIntegration_syncJobFinalize_oneImportJobFailed(t *testing.T) {
	dsn := testSyncJobFinalizeDSN(t)
	pool := openTestPool(t, dsn)
	ctx := context.Background()
	q := dbgen.New(pool)

	sellerID, syncID := seedUserSellerSync(t, ctx, q)
	_ = createImportJob(t, ctx, q, sellerID, syncID, "products")
	_ = createImportJob(t, ctx, q, sellerID, syncID, "orders")

	imports, err := q.ListImportJobsBySyncJobID(ctx, syncID)
	if err != nil || len(imports) != 2 {
		t.Fatalf("expected 2 import jobs, got %d err %v", len(imports), err)
	}

	// Complete first domain
	ij0 := imports[0]
	if _, err := q.UpdateImportJobToFetching(ctx, ij0.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToImporting(ctx, ij0.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToCompleted(ctx, dbgen.UpdateImportJobToCompletedParams{
		ID: ij0.ID, RecordsReceived: 0, RecordsImported: 0, RecordsFailed: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
		t.Fatal(err)
	}
	if st := syncJobStatus(t, ctx, pool, syncID); st != "running" {
		t.Fatalf("want running, got %q", st)
	}

	// Fail second
	ij1 := imports[1]
	if _, err := q.UpdateImportJobToFetching(ctx, ij1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToImporting(ctx, ij1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToFailed(ctx, dbgen.UpdateImportJobToFailedParams{
		ID:           ij1.ID,
		ErrorMessage: pgtype.Text{String: "boom", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
		t.Fatal(err)
	}
	if st := syncJobStatus(t, ctx, pool, syncID); st != "failed" {
		t.Fatalf("want failed, got %q", st)
	}
}

func TestIntegration_syncJobFinalize_activeImportJobs_keepsRunning(t *testing.T) {
	dsn := testSyncJobFinalizeDSN(t)
	pool := openTestPool(t, dsn)
	ctx := context.Background()
	q := dbgen.New(pool)

	sellerID, syncID := seedUserSellerSync(t, ctx, q)
	_ = createImportJob(t, ctx, q, sellerID, syncID, "products")
	_ = createImportJob(t, ctx, q, sellerID, syncID, "orders")

	imports, err := q.ListImportJobsBySyncJobID(ctx, syncID)
	if err != nil || len(imports) != 2 {
		t.Fatalf("expected 2 import jobs, got %d err %v", len(imports), err)
	}

	ij0 := imports[0]
	if _, err := q.UpdateImportJobToFetching(ctx, ij0.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToImporting(ctx, ij0.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToCompleted(ctx, dbgen.UpdateImportJobToCompletedParams{
		ID: ij0.ID, RecordsReceived: 0, RecordsImported: 0, RecordsFailed: 0,
	}); err != nil {
		t.Fatal(err)
	}
	// Second stays pending
	if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
		t.Fatal(err)
	}
	if st := syncJobStatus(t, ctx, pool, syncID); st != "running" {
		t.Fatalf("want running, got %q", st)
	}
}

func TestIntegration_syncJobFinalize_repeatFinalizeTerminalSafe(t *testing.T) {
	dsn := testSyncJobFinalizeDSN(t)
	pool := openTestPool(t, dsn)
	ctx := context.Background()
	q := dbgen.New(pool)

	sellerID, syncID := seedUserSellerSync(t, ctx, q)
	_ = createImportJob(t, ctx, q, sellerID, syncID, "products")
	imports, err := q.ListImportJobsBySyncJobID(ctx, syncID)
	if err != nil || len(imports) != 1 {
		t.Fatalf("expected 1 import job, got %d err %v", len(imports), err)
	}
	ij := imports[0]
	if _, err := q.UpdateImportJobToFetching(ctx, ij.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToImporting(ctx, ij.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpdateImportJobToCompleted(ctx, dbgen.UpdateImportJobToCompletedParams{
		ID: ij.ID, RecordsReceived: 0, RecordsImported: 0, RecordsFailed: 0,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
		t.Fatal(err)
	}
	if st := syncJobStatus(t, ctx, pool, syncID); st != "completed" {
		t.Fatalf("want completed, got %q", st)
	}
	for i := 0; i < 3; i++ {
		if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
			t.Fatalf("repeated finalize %d: %v", i, err)
		}
	}
	if st := syncJobStatus(t, ctx, pool, syncID); st != "completed" {
		t.Fatalf("want completed after repeats, got %q", st)
	}
}

func TestIntegration_syncJobFinalize_zeroImportJobs_doesNotComplete(t *testing.T) {
	dsn := testSyncJobFinalizeDSN(t)
	pool := openTestPool(t, dsn)
	ctx := context.Background()
	q := dbgen.New(pool)

	_, syncID := seedUserSellerSync(t, ctx, q)
	if _, err := runParentSyncJobFinalization(ctx, q, syncID); err != nil {
		t.Fatal(err)
	}
	if st := syncJobStatus(t, ctx, pool, syncID); st != "running" {
		t.Fatalf("want running when no import jobs, got %q", st)
	}
}
