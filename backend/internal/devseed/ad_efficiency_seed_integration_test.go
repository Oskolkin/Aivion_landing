//go:build integration

package devseed

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAdEfficiencyDSN(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Skip("set TEST_DATABASE_URL to run ad efficiency seed integration tests")
		return ""
	}
	testutil.RequireSafeTestDatabaseURL(t, v)
	return v
}

func openAdEfficiencyTestPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}
	return pool
}

func TestAdEfficiencySeedIntegration_Available(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")

	dsn := testAdEfficiencyDSN(t)
	pool := openAdEfficiencyTestPool(t, dsn)
	ctx := context.Background()

	sellerID := createAdEfficiencyTestSeller(t, ctx, pool)
	metricDate := time.Date(2026, 5, 23, 0, 0, 0, 0, time.UTC)
	cfg := testConfigFromEnv(t)

	withAI := false
	result, err := SeedAdEfficiency(ctx, pool, cfg, AdEfficiencySeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        AdEfficiencyScenarioAvailable,
		ResetFirst:      true,
		WithAINarrative: &withAI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasData {
		t.Fatal("expected HasData true")
	}
	if result.WidgetStatus != "available" {
		t.Fatalf("widget status got %q want available", result.WidgetStatus)
	}
	if result.CampaignsUpserted != 8 {
		t.Fatalf("campaigns got %d", result.CampaignsUpserted)
	}
	if result.DailyMetricsUpserted != 56 {
		t.Fatalf("metrics got %d want 56", result.DailyMetricsUpserted)
	}

	dto, err := verifyAdEfficiencyDashboard(ctx, pool, sellerID, metricDate)
	if err != nil {
		t.Fatal(err)
	}
	if !dto.HasData {
		t.Fatal("builder HasData false")
	}
	if dto.Summary.CampaignsHighDRRCount < 0 {
		t.Fatalf("high drr count %d", dto.Summary.CampaignsHighDRRCount)
	}
	if dto.Summary.CampaignsNoOrdersCount < 0 {
		t.Fatalf("no order count %d", dto.Summary.CampaignsNoOrdersCount)
	}
	if dto.Summary.SpendWithoutOrders <= 0 {
		t.Fatalf("spend without orders missing: %v", dto.Summary.SpendWithoutOrders)
	}
	if len(dto.Trend7D) != 7 {
		t.Fatalf("trend len %d", len(dto.Trend7D))
	}
	if dto.Period.SampleDays != 7 {
		t.Fatalf("sample days %d", dto.Period.SampleDays)
	}

	// Idempotent rerun.
	result2, err := SeedAdEfficiency(ctx, pool, cfg, AdEfficiencySeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        AdEfficiencyScenarioAvailable,
		ResetFirst:      false,
		WithAINarrative: &withAI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result2.CampaignsUpserted != 8 {
		t.Fatalf("idempotent campaigns got %d", result2.CampaignsUpserted)
	}

	assertNoSKUAdTablesWritten(t, ctx, pool, sellerID)
}

func TestAdEfficiencySeedIntegration_PartialAndReset(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")

	dsn := testAdEfficiencyDSN(t)
	pool := openAdEfficiencyTestPool(t, dsn)
	ctx := context.Background()
	sellerID := createAdEfficiencyTestSeller(t, ctx, pool)
	metricDate := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	cfg := testConfigFromEnv(t)

	partial, err := SeedAdEfficiency(ctx, pool, cfg, AdEfficiencySeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        AdEfficiencyScenarioPartial,
		ResetFirst:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if partial.WidgetStatus != "partial" {
		t.Fatalf("partial status got %q", partial.WidgetStatus)
	}
	if partial.SampleDaysExpected != 4 {
		t.Fatalf("sample days expected %d", partial.SampleDaysExpected)
	}

	dto, err := verifyAdEfficiencyDashboard(ctx, pool, sellerID, metricDate)
	if err != nil {
		t.Fatal(err)
	}
	if dto.Period.SampleDays >= 7 {
		t.Fatalf("sample days %d want < 7", dto.Period.SampleDays)
	}
	missing := 0
	for _, pt := range dto.Trend7D {
		if pt.DataStatus == "missing" {
			missing++
		}
	}
	if missing < 3 {
		t.Fatalf("missing trend days %d", missing)
	}

	_, err = SeedAdEfficiency(ctx, pool, cfg, AdEfficiencySeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        AdEfficiencyScenarioNotEnoughData,
		ResetFirst:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	dtoEmpty, err := verifyAdEfficiencyDashboard(ctx, pool, sellerID, metricDate)
	if err != nil {
		t.Fatal(err)
	}
	if dtoEmpty.HasData {
		t.Fatal("expected no ad efficiency data after not_enough_data seed")
	}
}

func createAdEfficiencyTestSeller(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	email := fmt.Sprintf("ad_eff_seed_%d@test.local", time.Now().UnixNano())
	hash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{Email: email, PasswordHash: hash, Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	sa, err := q.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{UserID: u.ID, Name: "AdEff Seed IT", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanupMVPSellerData(ctx, tx, sa.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return sa.ID
}

func testConfigFromEnv(t *testing.T) *config.Config {
	t.Helper()
	if os.Getenv("REDIS_ADDR") == "" {
		t.Setenv("REDIS_ADDR", "localhost:6379")
	}
	if os.Getenv("S3_ENDPOINT") == "" {
		t.Setenv("S3_ENDPOINT", "localhost:19000")
	}
	if os.Getenv("S3_ACCESS_KEY") == "" {
		t.Setenv("S3_ACCESS_KEY", "minio")
	}
	if os.Getenv("S3_SECRET_KEY") == "" {
		t.Setenv("S3_SECRET_KEY", "minio123")
	}
	if os.Getenv("S3_BUCKET_RAW") == "" {
		t.Setenv("S3_BUCKET_RAW", "raw-payloads")
	}
	if os.Getenv("S3_BUCKET_EXPORTS") == "" {
		t.Setenv("S3_BUCKET_EXPORTS", "exports")
	}
	if os.Getenv("S3_BUCKET_ARTIFACTS") == "" {
		t.Setenv("S3_BUCKET_ARTIFACTS", "artifacts")
	}
	if os.Getenv("ENCRYPTION_KEY") == "" {
		t.Setenv("ENCRYPTION_KEY", "01234567890123456789012345678901")
	}
	if os.Getenv("JWT_SECRET") == "" {
		t.Setenv("JWT_SECRET", "dev-jwt-secret")
	}
	if os.Getenv("AI_PROVIDER_MODE") == "" {
		t.Setenv("AI_PROVIDER_MODE", "fake")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Setenv("OPENAI_API_KEY", "fake")
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func assertNoSKUAdTablesWritten(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64) {
	t.Helper()
	var skuLinks int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM ad_campaign_skus
		WHERE seller_account_id = $1 AND campaign_external_id BETWEEN $2 AND $3
	`, sellerID, DevAdEfficiencyCampaignIDMin, DevAdEfficiencyCampaignIDMax).Scan(&skuLinks); err != nil {
		t.Fatal(err)
	}
	if skuLinks != 0 {
		t.Fatalf("ad_campaign_skus rows written: %d", skuLinks)
	}
}
