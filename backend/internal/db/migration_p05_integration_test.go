//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db/migrationpreflight"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const migrationLatestVersion uint = 52

func TestMigrations_P05_Version42DirtyToLatest(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migration043Version-1); err != nil {
		t.Fatalf("migrate to 42: %v", err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	seedVersion42DirtyFixture(t, ctx, pool, sellerID)

	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatalf("migrate latest: %v", err)
	}
	assertPostRepairInvariants(t, ctx, pool, sellerID)
}

func TestMigrations_P05_Old043DedupRepairedBy051(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migration043Version-1); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	syncID, canonicalID, loserID := seedDuplicateImportJobsPre043(t, ctx, pool, sellerID)

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migration043Version); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migrationLatestVersion); err != nil {
		t.Fatalf("migrate to 51: %v", err)
	}

	var loserDomain, loserStatus string
	var supersededBy *int64
	if err := pool.QueryRow(ctx, `SELECT domain, status, superseded_by_import_job_id FROM import_jobs WHERE id=$1`, loserID).
		Scan(&loserDomain, &loserStatus, &supersededBy); err != nil {
		t.Fatal(err)
	}
	if loserStatus != "superseded" || loserDomain != "products" {
		t.Fatalf("loser status=%s domain=%s", loserStatus, loserDomain)
	}
	if supersededBy == nil || *supersededBy != canonicalID {
		t.Fatalf("superseded_by=%v want %d", supersededBy, canonicalID)
	}
	_ = syncID
}

func TestMigrations_P05_AmbiguousActionsPreflightBlocked(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	seedAmbiguousActionFixture(t, ctx, pool, sellerID)

	report, err := migrationpreflight.Run(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Blocked() || report.AmbiguousActionItems == 0 {
		t.Fatalf("expected ambiguous preflight block, got\n%s", report.String())
	}
}

func TestMigrations_P05_PreflightPassesAfterRepair(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migration043Version-1); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	seedVersion42DirtyFixture(t, ctx, pool, sellerID)
	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	report, err := migrationpreflight.Run(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if report.Blocked() {
		t.Fatalf("preflight blocked:\n%s", report.String())
	}
	_ = sellerID
}

func TestMigrations_P05_SchemaEquivalencePaths(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	clean := schemaFingerprint(t, dsn, 43_000_101, func(t *testing.T, sqlDB *sql.DB, pool *pgxpool.Pool) {
		if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
			t.Fatal(err)
		}
	})
	dirty := schemaFingerprint(t, dsn, 43_000_102, func(t *testing.T, sqlDB *sql.DB, pool *pgxpool.Pool) {
		ctx := context.Background()
		if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migration043Version-1); err != nil {
			t.Fatal(err)
		}
		sellerID := seedSellerAccount(t, ctx, pool)
		seedVersion42DirtyFixture(t, ctx, pool, sellerID)
		if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
			t.Fatal(err)
		}
	})
	if clean != dirty {
		t.Fatalf("schema mismatch\n clean=%s\n dirty=%s", clean, dirty)
	}
}

func TestMigrations_P05_AmbiguousActionsMigrationAbort(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), 50); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	if _, err := pool.Exec(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status) VALUES
($1, '2026-06-03', 'completed'),
($1, '2026-06-03', 'completed')`, sellerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO action_items (seller_account_id, run_id, action_key, type, category, priority, title, reason, next_step, metric_date)
VALUES ($1, NULL, 'abort_action', 'stock_replenishment', 'stock', 'high', 'x', 'x', 'x', '2026-06-03')`, sellerID); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migrationLatestVersion); err == nil {
		t.Fatal("expected migration 051 to abort on ambiguous action_items")
	}
}

func TestMigrations_P05_UnpublishedOrphanLinksRepaired(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), 50); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	var pipelineID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO ai_pipeline_runs (seller_account_id, metric_date, status, data_pack_hash)
VALUES ($1, '2026-06-04', 'failed', 'orphan-test') RETURNING id`, sellerID).Scan(&pipelineID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE ai_pipeline_runs DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_dashboard_run_id`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_runs
SET dashboard_run_id = '00000000-0000-0000-0000-000000000001'::uuid
WHERE id = $1`, pipelineID); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migrationLatestVersion); err != nil {
		t.Fatalf("migrate 051: %v", err)
	}
	var dashboardRunID *string
	if err := pool.QueryRow(ctx, `SELECT dashboard_run_id::text FROM ai_pipeline_runs WHERE id=$1`, pipelineID).Scan(&dashboardRunID); err != nil {
		t.Fatal(err)
	}
	if dashboardRunID != nil {
		t.Fatalf("expected orphan dashboard link cleared, got %v", *dashboardRunID)
	}
}

func TestMigrations_P05_PublishedOrphanLinksAbort(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), 50); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	var dashboardID string
	if err := pool.QueryRow(ctx, `
INSERT INTO ai_dashboard_narratives (seller_account_id, metric_date, status, prompt_version)
VALUES ($1, '2026-06-05', 'completed', 'test') RETURNING id::text`, sellerID).Scan(&dashboardID); err != nil {
		t.Fatal(err)
	}
	var actionsRunID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status)
VALUES ($1, '2026-06-05', 'completed') RETURNING id`, sellerID).Scan(&actionsRunID); err != nil {
		t.Fatal(err)
	}
	var pipelineID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO ai_pipeline_runs (
    seller_account_id, metric_date, status, data_pack_hash,
    published_at, dashboard_run_id, actions_run_id
) VALUES ($1, '2026-06-05', 'completed', 'orphan-test', now(), $2::uuid, $3) RETURNING id`,
		sellerID, dashboardID, actionsRunID).Scan(&pipelineID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE ai_pipeline_runs DROP CONSTRAINT IF EXISTS fk_ai_pipeline_runs_dashboard_run_id`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE ai_pipeline_runs
SET dashboard_run_id = '00000000-0000-0000-0000-000000000002'::uuid
WHERE id = $1`, pipelineID); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migrationLatestVersion); err == nil {
		t.Fatal("expected migration 051 to abort on published orphan link")
	}
}

func TestMigrations_P05_RepairIdempotencyPreflight(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	ctx := context.Background()

	if err := db.MigrateToVersion(sqlDB, migrationsDir(t), migration043Version-1); err != nil {
		t.Fatal(err)
	}
	sellerID := seedSellerAccount(t, ctx, pool)
	seedVersion42DirtyFixture(t, ctx, pool, sellerID)
	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	var auditBefore int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM migration_repair_audit WHERE migration_version='000051'`).Scan(&auditBefore); err != nil {
		t.Fatal(err)
	}
	if auditBefore == 0 {
		t.Fatal("expected repair audit rows")
	}
	report1, err := migrationpreflight.Run(ctx, pool)
	if err != nil || report1.Blocked() {
		t.Fatalf("preflight1: err=%v\n%s", err, report1.String())
	}
	report2, err := migrationpreflight.Run(ctx, pool)
	if err != nil || report2.Blocked() {
		t.Fatalf("preflight2: err=%v\n%s", err, report2.String())
	}
	var auditAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM migration_repair_audit WHERE migration_version='000051'`).Scan(&auditAfter); err != nil {
		t.Fatal(err)
	}
	if auditBefore != auditAfter {
		t.Fatalf("audit count changed %d -> %d", auditBefore, auditAfter)
	}
	_ = sellerID
}

func seedVersion42DirtyFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64) {
	t.Helper()
	var syncA, syncB int64
	for _, st := range []string{"running", "pending"} {
		var id int64
		if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, started_at, created_at)
VALUES ($1, 'incremental_sync', $2, now(), now()) RETURNING id`, sellerID, st).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if st == "running" {
			syncA = id
		} else {
			syncB = id
		}
	}
	for _, domain := range []string{"products", "products", "orders"} {
		if _, err := pool.Exec(ctx, `
INSERT INTO import_jobs (seller_account_id, sync_job_id, domain, status, created_at)
VALUES ($1, $2, $3, 'pending', now())`, sellerID, syncA, domain); err != nil {
			t.Fatal(err)
		}
	}
	var runID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status)
VALUES ($1, '2026-06-01', 'completed') RETURNING id`, sellerID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO action_items (seller_account_id, run_id, action_key, type, category, priority, title, reason, next_step, metric_date)
VALUES ($1, NULL, 'legacy_action', 'stock_replenishment', 'stock', 'high', 'legacy', 'legacy', 'legacy', '2026-06-01')`, sellerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO action_user_states (seller_account_id, action_key, status, checked_at)
VALUES ($1, 'legacy_action', 'checked', now())`, sellerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO chat_sessions (seller_account_id, title, action_key, status, created_at, updated_at)
VALUES ($1, 'legacy chat', 'legacy_action', 'active', now(), now())`, sellerID); err != nil {
		t.Fatal(err)
	}
	_ = syncB
	_ = runID
}

func seedDuplicateImportJobsPre043(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64) (syncID, canonicalID, loserID int64) {
	t.Helper()
	if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, created_at)
VALUES ($1, 'incremental_sync', 'running', now()) RETURNING id`, sellerID).Scan(&syncID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO import_jobs (seller_account_id, sync_job_id, domain, status, created_at)
VALUES ($1, $2, 'products', 'completed', now()) RETURNING id`, sellerID, syncID).Scan(&canonicalID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
INSERT INTO import_jobs (seller_account_id, sync_job_id, domain, status, created_at)
VALUES ($1, $2, 'products', 'pending', now()) RETURNING id`, sellerID, syncID).Scan(&loserID); err != nil {
		t.Fatal(err)
	}
	return syncID, canonicalID, loserID
}

func seedAmbiguousActionFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
INSERT INTO action_generation_runs (seller_account_id, metric_date, status) VALUES
($1, '2026-06-02', 'completed'),
($1, '2026-06-02', 'completed')`, sellerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO action_items (seller_account_id, run_id, action_key, type, category, priority, title, reason, next_step, metric_date)
VALUES ($1, NULL, 'ambiguous_action', 'stock_replenishment', 'stock', 'high', 'x', 'x', 'x', '2026-06-02')`, sellerID); err != nil {
		t.Fatal(err)
	}
}

func assertPostRepairInvariants(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64) {
	t.Helper()
	var activeDup int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM (
  SELECT seller_account_id FROM sync_jobs WHERE status IN ('pending','running')
  GROUP BY seller_account_id HAVING COUNT(*) > 1
) x`).Scan(&activeDup); err != nil {
		t.Fatal(err)
	}
	if activeDup != 0 {
		t.Fatalf("active sync dups=%d", activeDup)
	}
	var runID *int64
	if err := pool.QueryRow(ctx, `SELECT run_id FROM action_items WHERE seller_account_id=$1 AND action_key='legacy_action'`, sellerID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if runID == nil {
		t.Fatal("legacy action run_id not backfilled")
	}
	var userStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM action_user_states WHERE seller_account_id=$1 AND action_key='legacy_action'`, sellerID).Scan(&userStatus); err != nil {
		t.Fatal(err)
	}
	if userStatus != "checked" {
		t.Fatalf("user state=%s", userStatus)
	}
	var chatCount int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM chat_sessions WHERE seller_account_id=$1 AND action_key='legacy_action'`, sellerID).Scan(&chatCount); err != nil {
		t.Fatal(err)
	}
	if chatCount != 1 {
		t.Fatalf("chat sessions=%d want 1", chatCount)
	}
}

func schemaFingerprint(t *testing.T, dsn string, lockID int64, setup func(t *testing.T, sqlDB *sql.DB, pool *pgxpool.Pool)) string {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	if _, err := pool.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		pool.Close()
		t.Fatalf("advisory lock: %v", err)
	}
	if _, err := pool.Exec(ctx, `
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
GRANT ALL ON SCHEMA public TO CURRENT_USER;
`); err != nil {
		_, _ = pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, lockID)
		pool.Close()
		t.Fatalf("reset schema: %v", err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		_, _ = pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, lockID)
		pool.Close()
		t.Fatalf("sql.Open: %v", err)
	}
	setup(t, sqlDB, pool)
	_ = sqlDB.Close()
	rows, err := pool.Query(ctx, `
SELECT table_name, column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_schema='public'
ORDER BY table_name, ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var table, col, typ, nullable string
		if err := rows.Scan(&table, &col, &typ, &nullable); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "%s.%s:%s:%s\n", table, col, typ, nullable)
	}
	rows, err = pool.Query(ctx, `
SELECT indexname, indexdef FROM pg_indexes WHERE schemaname='public' ORDER BY indexname`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "idx:%s:%s\n", name, def)
	}
	_, _ = pool.Exec(ctx, `SELECT pg_advisory_unlock($1)`, lockID)
	pool.Close()
	return b.String()
}

func TestMigrations_P05_UpDownUpLatest(t *testing.T) {
	dsn := testutil.RequireIntegrationDatabase(t)
	pool := openMigrationPool(t, dsn)
	resetPublicSchema(t, pool)
	sqlDB := openMigrationSQLDB(t, dsn)
	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatal(err)
	}
	version, dirty, err := db.MigrationVersion(sqlDB, migrationsDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if dirty || version < migrationLatestVersion {
		t.Fatalf("version=%d dirty=%v", version, dirty)
	}
	if err := db.RunMigrationSteps(sqlDB, migrationsDir(t), -1); err != nil {
		t.Fatal(err)
	}
	if err := db.RunMigrationSteps(sqlDB, migrationsDir(t), 1); err != nil {
		t.Fatal(err)
	}
	after, dirty, err := db.MigrationVersion(sqlDB, migrationsDir(t))
	if err != nil || dirty || after != version {
		t.Fatalf("after cycle version=%d dirty=%v err=%v", after, dirty, err)
	}
	_ = time.Now()
}
