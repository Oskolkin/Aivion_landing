//go:build integration

package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestResolveImportJobExecutionTarget_SupersededPointer(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	ctx := context.Background()
	pool := openImportResolvePool(t, dsn)
	q := dbgen.New(pool)

	sellerID := seedImportResolveSeller(t, ctx, pool)
	var syncID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, created_at, updated_at)
VALUES ($1, 'incremental_sync', 'running', now(), now()) RETURNING id`, sellerID).Scan(&syncID); err != nil {
		t.Fatal(err)
	}
	var canonicalID, loserID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO import_jobs (seller_account_id, sync_job_id, domain, status, created_at, updated_at)
VALUES ($1, $2, 'products', 'completed', now(), now()) RETURNING id`, sellerID, syncID).Scan(&canonicalID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO import_jobs (seller_account_id, sync_job_id, domain, status, superseded_by_import_job_id, deduplicated_at, created_at, updated_at)
VALUES ($1, $2, 'products', 'superseded', $3, now(), now(), now()) RETURNING id`, sellerID, syncID, canonicalID).Scan(&loserID); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveImportJobExecutionTarget(ctx, q, loserID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != canonicalID {
		t.Fatalf("resolved id=%d want canonical=%d", resolved.ID, canonicalID)
	}
}

func openImportResolvePool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedImportResolveSeller(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()
	var userID int64
	email := fmt.Sprintf("import-resolve-%d@test.invalid", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts (user_id, name, status) VALUES ($1, 'it', 'active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatal(err)
	}
	return sellerID
}
