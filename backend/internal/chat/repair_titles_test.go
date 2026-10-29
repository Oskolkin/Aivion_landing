package chat

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
)

func repairTitlesDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run repair_titles integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func repairTitlesMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))
}

func openRepairTitlesPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.RunMigrations(sqlDB, repairTitlesMigrationsDir(t)); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return pool
}

func TestRepairChatTitles_repairsFromActionTitle(t *testing.T) {
	pool := openRepairTitlesPool(t, repairTitlesDSN(t))
	ctx := context.Background()
	q := dbgen.New(pool)

	email := fmt.Sprintf("repair-titles-%d@test.invalid", time.Now().UnixNano())
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "repair", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	metricDate := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	run, err := q.CreateActionGenerationRun(ctx, dbgen.CreateActionGenerationRunParams{
		SellerAccountID: sa.ID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		Status:          "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	actionKey := "pricing.stock_risk"
	if _, err := q.UpsertActionItem(ctx, dbgen.UpsertActionItemParams{
		SellerAccountID:        sa.ID,
		RunID:                  pgtype.Int8{Int64: run.ID, Valid: true},
		ActionKey:              actionKey,
		Type:                   "pricing",
		Category:               "stock",
		Priority:               "high",
		Title:                  "Снизить остатки SKU-1001",
		Summary:                pgtype.Text{String: "summary", Valid: true},
		Reason:                 "reason",
		NextStep:               "next",
		EvidenceJson:           []byte("[]"),
		LimitationsJson:        []byte("[]"),
		CtaJson:                []byte("[]"),
		AffectedEntitiesJson:   []byte("[]"),
		SourceCandidateIdsJson: []byte("[]"),
		EvidenceFactIdsJson:    []byte("[]"),
		PayloadJson:            []byte("{}"),
		MetricDate:             pgtype.Date{Time: metricDate, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	var sessionID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO chat_sessions (seller_account_id, title, action_key, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())
RETURNING id`, sa.ID, "????????", actionKey).Scan(&sessionID); err != nil {
		t.Fatal(err)
	}

	result, err := RepairChatTitlesForSeller(ctx, pool, sa.ID, false, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.CorruptedFound < 1 || result.Repaired < 1 {
		t.Fatalf("result=%+v", result)
	}

	var title string
	if err := pool.QueryRow(ctx, `SELECT title FROM chat_sessions WHERE id = $1`, sessionID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if IsCorruptedChatTitle(title) {
		t.Fatalf("title still corrupted: %q", title)
	}
	if title == "" {
		t.Fatal("empty repaired title")
	}
}

func TestRepairChatTitles_dryRunDoesNotWrite(t *testing.T) {
	pool := openRepairTitlesPool(t, repairTitlesDSN(t))
	ctx := context.Background()
	q := dbgen.New(pool)

	email := fmt.Sprintf("repair-dry-%d@test.invalid", time.Now().UnixNano())
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: "x", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "repair-dry", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO chat_sessions (seller_account_id, title, created_at, updated_at)
VALUES ($1, '????????', NOW(), NOW())`, sa.ID); err != nil {
		t.Fatal(err)
	}

	result, err := RepairChatTitlesForSeller(ctx, pool, sa.ID, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.DryRun != true || result.Repaired != 0 {
		t.Fatalf("result=%+v", result)
	}
}
