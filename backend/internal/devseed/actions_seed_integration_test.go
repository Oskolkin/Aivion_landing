//go:build integration

package devseed

import (
	"context"
	"testing"
	"time"
)

func TestSeedActionsIntegration_IdempotentDataAppendOnlyRuns(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")
	dsn := testAdEfficiencyDSN(t)
	pool := openAdEfficiencyTestPool(t, dsn)
	ctx := context.Background()
	sellerID := createAdEfficiencyTestSeller(t, ctx, pool)
	metricDate := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)

	first, err := SeedActions(ctx, pool, SeedActionsOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        ActionsSeedScenarioChecked,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := SeedActions(ctx, pool, SeedActionsOptions{
		SellerAccountID: sellerID,
		MetricDate:      &metricDate,
		Scenario:        ActionsSeedScenarioChecked,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.GenerationRunID == second.GenerationRunID {
		t.Fatal("generation runs must be append-only history")
	}

	var actionCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_items WHERE seller_account_id=$1 AND metric_date=$2`, sellerID, metricDate).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 14 {
		t.Fatalf("expected 14 versioned action items across two runs (7 each), got %d", actionCount)
	}

	var latestRunActionCount int
	if err := pool.QueryRow(ctx, `
SELECT COUNT(*) FROM action_items
WHERE seller_account_id=$1 AND metric_date=$2 AND run_id=$3`, sellerID, metricDate, second.GenerationRunID).Scan(&latestRunActionCount); err != nil {
		t.Fatal(err)
	}
	if latestRunActionCount != 7 {
		t.Fatalf("expected 7 actions in latest run, got %d", latestRunActionCount)
	}

	var checkedCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_user_states WHERE seller_account_id=$1 AND status='checked'`, sellerID).Scan(&checkedCount); err != nil {
		t.Fatal(err)
	}
	if checkedCount != 2 {
		t.Fatalf("expected stable 2 checked states, got %d", checkedCount)
	}

	var runCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_generation_runs WHERE seller_account_id=$1 AND metric_date=$2`, sellerID, metricDate).Scan(&runCount); err != nil {
		t.Fatal(err)
	}
	if runCount < 2 {
		t.Fatalf("expected append-only run history, got %d", runCount)
	}

	var emptyCTA int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM action_items
		WHERE seller_account_id=$1 AND metric_date=$2
		  AND (cta_json IS NULL OR cta_json::text IN ('[]', 'null'))
	`, sellerID, metricDate).Scan(&emptyCTA); err != nil {
		t.Fatal(err)
	}
	if emptyCTA > 0 {
		t.Fatalf("expected non-empty cta_json for seeded actions, empty=%d", emptyCTA)
	}

	var missingEntityGroup int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM action_items
		WHERE seller_account_id=$1 AND metric_date=$2
		  AND type IN ('stock_replenishment','no_sales_with_stock','sku_orders_drop','key_sku_risk','ad_high_drr','ad_spend_no_orders')
		  AND (entity_group_ref IS NULL OR entity_group_ref = '')
	`, sellerID, metricDate).Scan(&missingEntityGroup); err != nil {
		t.Fatal(err)
	}
	if missingEntityGroup > 0 {
		t.Fatalf("expected entity_group_ref on SKU/campaign seeded actions, missing=%d", missingEntityGroup)
	}

	var groupCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM action_entity_groups WHERE seller_account_id=$1`, sellerID).Scan(&groupCount); err != nil {
		t.Fatal(err)
	}
	if groupCount < 6 {
		t.Fatalf("expected seeded entity groups, got %d", groupCount)
	}
}
