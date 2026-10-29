//go:build integration

package unifiedanalysis_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run unified analysis repository integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}
	return pool
}

func createSeller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	email := fmt.Sprintf("ua-provider-%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status) VALUES ($1, $2, 'active') RETURNING id`,
		userID, "ua-provider").Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	return sellerID
}

func TestRepositoryUnifiedAnalysisRunLifecycle(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	sellerID := createSeller(t, pool)
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)

	cfg := unifiedanalysis.Config{
		Enabled:     true,
		Provider:    unifiedanalysis.ProviderNameOpenAI,
		Model:       "gpt-test",
		MemoryLimit: "4g",
	}
	run, err := repo.CreateRun(ctx, sellerID, pgtype.Date{Time: metricDate, Valid: true}, cfg, pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}, unifiedanalysis.TriggerManual)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if run.Mode != unifiedanalysis.ModeUnifiedAnalysis {
		t.Fatalf("mode=%s", run.Mode)
	}
	if run.Status != unifiedanalysis.StatusCreated {
		t.Fatalf("status=%s", run.Status)
	}

	active, err := repo.FindActiveRun(ctx, sellerID, pgtype.Date{Time: metricDate, Valid: true})
	if err != nil {
		t.Fatalf("FindActiveRun: %v", err)
	}
	if active.ID != run.ID {
		t.Fatalf("active id=%d want %d", active.ID, run.ID)
	}

	if _, err := repo.TransitionStatus(ctx, run.ID, unifiedanalysis.StatusCreated, unifiedanalysis.StatusPackBuilding); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if _, err := repo.TransitionStatus(ctx, run.ID, unifiedanalysis.StatusCreated, unifiedanalysis.StatusPackReady); err == nil {
		t.Fatal("expected CAS conflict")
	}

	if _, err := repo.SavePack(ctx, run.ID, dbgen.SaveUnifiedAnalysisPackParams{
		PackID:                pgtype.Text{String: "pack:test", Valid: true},
		PackHash:              pgtype.Text{String: "abc", Valid: true},
		PackStorageKey:        pgtype.Text{String: "ai-analysis-packs/x/analysis_pack.zip", Valid: true},
		PackArchiveSha256:     pgtype.Text{String: "deadbeef", Valid: true},
		PackCompressedBytes:   pgtype.Int8{Int64: 10, Valid: true},
		PackUncompressedBytes: pgtype.Int8{Int64: 20, Valid: true},
		DataPackSchemaVersion: pgtype.Text{String: "aivion_analysis_pack_v1", Valid: true},
	}); err != nil {
		t.Fatalf("SavePack: %v", err)
	}

	if _, err := repo.SaveContainer(ctx, run.ID, "cntr_test_001"); err != nil {
		t.Fatalf("SaveContainer: %v", err)
	}
	if _, err := repo.SaveContainerFile(ctx, run.ID, "cfile_test_001", "/mnt/data/analysis_pack.zip"); err != nil {
		t.Fatalf("SaveContainerFile: %v", err)
	}
	respID := fmt.Sprintf("resp_test_%d", time.Now().UnixNano())
	if _, err := repo.SaveResponseStarted(ctx, run.ID, dbgen.SaveUnifiedAnalysisResponseStartedParams{
		ProviderResponseID: pgtype.Text{String: respID, Valid: true},
		ProviderStatus:     pgtype.Text{String: string(provider.ProviderResponseStatusInProgress), Valid: true},
		ProviderStartedAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}); err != nil {
		t.Fatalf("SaveResponseStarted: %v", err)
	}

	// Legacy-style run can coexist for same seller+date with mode=unified.
	_, err = pool.Exec(ctx, `
INSERT INTO ai_pipeline_runs (
  seller_account_id, metric_date, mode, trigger, status, data_pack_hash, started_at
) VALUES ($1, $2, 'unified', 'test', 'running', 'legacy-hash', now())`,
		sellerID, metricDate)
	if err != nil {
		t.Fatalf("insert legacy run: %v", err)
	}

	got, err := repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.ProviderContainerID != "cntr_test_001" || got.ProviderResponseID != respID {
		t.Fatalf("provider ids missing: %+v", got)
	}
}
