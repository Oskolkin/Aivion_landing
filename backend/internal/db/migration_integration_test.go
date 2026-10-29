//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const migration043Version uint = 43

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))
}

func openMigrationSQLDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return sqlDB
}

func openMigrationPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	return pool
}

func resetPublicSchema(t *testing.T, pool *pgxpool.Pool) {
	resetPublicSchemaWithLock(t, pool, 43_000_043)
}

func resetPublicSchemaWithLock(t *testing.T, pool *pgxpool.Pool, lockID int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockID)
	})
	if _, err := pool.Exec(ctx, `
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
GRANT ALL ON SCHEMA public TO CURRENT_USER;
`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

func TestMigrations_UpDownUp(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)

	sqlDB := openMigrationSQLDB(t, dsn)
	migrationsPath := migrationsDir(t)

	if err := db.RunMigrations(sqlDB, migrationsPath); err != nil {
		t.Fatalf("RunMigrations up: %v", err)
	}
	version, dirty, err := db.MigrationVersion(sqlDB, migrationsPath)
	if err != nil {
		t.Fatalf("MigrationVersion: %v", err)
	}
	if dirty {
		t.Fatal("schema is dirty after up")
	}
	if version < migration043Version {
		t.Fatalf("version=%d want at least %d", version, migration043Version)
	}

	if err := db.RunMigrationSteps(sqlDB, migrationsPath, -1); err != nil {
		t.Fatalf("RunMigrationSteps down: %v", err)
	}
	if err := db.RunMigrationSteps(sqlDB, migrationsPath, 1); err != nil {
		t.Fatalf("RunMigrationSteps up: %v", err)
	}

	afterVersion, dirty, err := db.MigrationVersion(sqlDB, migrationsPath)
	if err != nil {
		t.Fatalf("MigrationVersion after cycle: %v", err)
	}
	if dirty {
		t.Fatal("schema is dirty after up/down/up")
	}
	if afterVersion != version {
		t.Fatalf("version after cycle=%d want %d", afterVersion, version)
	}
}

func TestMigrations_Pre043FixtureThenMigrate43Plus(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)

	sqlDB := openMigrationSQLDB(t, dsn)
	migrationsPath := migrationsDir(t)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsPath, migration043Version-1); err != nil {
		t.Fatalf("MigrateToVersion 42: %v", err)
	}

	sellerID, runID := seedPre043ActionFixture(t, ctx, pool)

	var syncType string
	if _, err := pool.Exec(ctx, `ALTER TABLE sync_jobs DROP CONSTRAINT IF EXISTS sync_jobs_type_check`); err != nil {
		t.Fatalf("drop sync_jobs type check: %v", err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status)
VALUES ($1, 'periodic_sync', 'completed')
RETURNING type`,
		sellerID,
	).Scan(&syncType); err != nil {
		t.Fatalf("insert legacy periodic_sync: %v", err)
	}
	if syncType != "periodic_sync" {
		t.Fatalf("sync type=%q", syncType)
	}

	if err := db.MigrateToVersion(sqlDB, migrationsPath, migration043Version); err != nil {
		t.Fatalf("MigrateToVersion 43: %v", err)
	}

	if err := pool.QueryRow(ctx, `SELECT type FROM sync_jobs WHERE seller_account_id=$1`, sellerID).Scan(&syncType); err != nil {
		t.Fatalf("load sync job: %v", err)
	}
	if syncType != "incremental_sync" {
		t.Fatalf("normalized sync type=%q want incremental_sync", syncType)
	}

	var backfilledRunID *int64
	if err := pool.QueryRow(ctx, `SELECT run_id FROM action_items WHERE seller_account_id=$1 AND action_key='legacy_action'`, sellerID).Scan(&backfilledRunID); err != nil {
		t.Fatalf("load action item run_id: %v", err)
	}
	if backfilledRunID == nil || *backfilledRunID != runID {
		t.Fatalf("backfilled run_id=%v want %d", backfilledRunID, runID)
	}

	var tableExists bool
	if err := pool.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema='public' AND table_name='ai_usage_events'
)`).Scan(&tableExists); err != nil {
		t.Fatalf("check ai_usage_events: %v", err)
	}
	if !tableExists {
		t.Fatal("ai_usage_events table missing after migration 43")
	}
}

func TestMigrations_Down43KeepsLatestActionPerActionKey(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)

	sqlDB := openMigrationSQLDB(t, dsn)
	migrationsPath := migrationsDir(t)
	ctx := context.Background()

	// Apply through 043 only so the next down step exercises 043 rollback semantics.
	if err := db.MigrateToVersion(sqlDB, migrationsPath, migration043Version); err != nil {
		t.Fatalf("MigrateToVersion(%d): %v", migration043Version, err)
	}

	sellerID := seedSellerAccount(t, ctx, pool)
	var oldRunID, newRunID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status, created_at)
VALUES ($1, '2026-06-01', 'completed', now() - interval '2 days')
RETURNING id`, sellerID).Scan(&oldRunID); err != nil {
		t.Fatalf("insert old run: %v", err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status, created_at)
VALUES ($1, '2026-06-02', 'completed', now())
RETURNING id`, sellerID).Scan(&newRunID); err != nil {
		t.Fatalf("insert new run: %v", err)
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO action_items (
	seller_account_id, run_id, action_key, type, category, priority,
	title, reason, next_step, metric_date, created_at
) VALUES
($1, $2, 'same_action', 'stock_replenishment', 'stock', 'high', 'old', 'old', 'old', '2026-06-01', now() - interval '2 days'),
($1, $3, 'same_action', 'stock_replenishment', 'stock', 'high', 'new', 'new', 'new', '2026-06-02', now())`,
		sellerID, oldRunID, newRunID,
	); err != nil {
		t.Fatalf("insert versioned action items: %v", err)
	}

	if err := db.RunMigrationSteps(sqlDB, migrationsPath, -1); err != nil {
		t.Fatalf("down migration 43: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM action_items WHERE seller_account_id=$1 AND action_key='same_action'`, sellerID).Scan(&count); err != nil {
		t.Fatalf("count action items: %v", err)
	}
	if count != 1 {
		t.Fatalf("action item count=%d want 1", count)
	}

	var keptTitle string
	var keptRunID *int64
	if err := pool.QueryRow(ctx, `
SELECT title, run_id FROM action_items WHERE seller_account_id=$1 AND action_key='same_action'`, sellerID).
		Scan(&keptTitle, &keptRunID); err != nil {
		t.Fatalf("load kept action item: %v", err)
	}
	if keptTitle != "new" {
		t.Fatalf("kept title=%q want new", keptTitle)
	}
	if keptRunID == nil || *keptRunID != newRunID {
		t.Fatalf("kept run_id=%v want %d", keptRunID, newRunID)
	}

	var constraintExists bool
	if err := pool.QueryRow(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM pg_constraint
	WHERE conname = 'action_items_seller_account_id_action_key_key'
)`).Scan(&constraintExists); err != nil {
		t.Fatalf("check unique constraint: %v", err)
	}
	if !constraintExists {
		t.Fatal("expected seller+action_key unique constraint after down migration")
	}
}

func seedSellerAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()
	var userID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash)
VALUES ($1, 'hash')
RETURNING id`, time.Now().Format("migration-test-%d@example.com")).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status)
VALUES ($1, 'migration-test-seller', 'active')
RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	return sellerID
}

func seedPre043ActionFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (sellerID, runID int64) {
	t.Helper()
	sellerID = seedSellerAccount(t, ctx, pool)

	if err := pool.QueryRow(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status)
VALUES ($1, '2026-06-01', 'completed')
RETURNING id`, sellerID).Scan(&runID); err != nil {
		t.Fatalf("insert generation run: %v", err)
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO action_items (
	seller_account_id, run_id, action_key, type, category, priority,
	title, reason, next_step, metric_date
) VALUES ($1, NULL, 'legacy_action', 'stock_replenishment', 'stock', 'high', 'legacy', 'legacy', 'legacy', '2026-06-01')`,
		sellerID,
	); err != nil {
		t.Fatalf("insert legacy action item: %v", err)
	}
	return sellerID, runID
}
