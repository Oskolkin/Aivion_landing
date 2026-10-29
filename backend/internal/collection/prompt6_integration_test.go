//go:build integration

package collection_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/advertising"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/price"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/stock"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPrompt6RepositoriesFinalizeAtomicSellerScopedHeads(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var user, seller, sku int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("prompt6_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'prompt6','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,101,NOW(),NOW(),'ACTIVE')RETURNING id`, seller).Scan(&sku); err != nil {
		t.Fatal(err)
	}

	newDomain := func(domain, profile string) (int64, int64) {
		var run, dr, payload int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','prompt6.integration')RETURNING id`, seller).Scan(&run); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,$3,'RUNNING',NOW())RETURNING id`, run, seller, domain).Scan(&dr); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON',$3,$4,1,'raw',$5,$6,200,NOW(),'{}',$7)RETURNING id`, seller, dr, "prompt6."+domain, fmt.Sprintf("fp-%d", dr), fmt.Sprintf("prompt6/%d", dr), fmt.Sprintf("%064x", dr), profile).Scan(&payload); e != nil {
			t.Fatal(e)
		}
		return dr, payload
	}
	amount := func(raw string) *sales.Decimal {
		v, e := sales.ParseAmount(json.Number(raw))
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}

	stockRun, stockPayload := newDomain("STOCK", stock.RedactionProfile)
	valid := int64(5)
	payloadID := stockPayload
	stockResult := stock.Result{ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1, CompletenessStatus: "COMPLETE", UniverseHash: fmt.Sprintf("%064x", 1), Positions: []stock.Position{{CanonicalSKUID: &sku, SourceSKU: 101, SKUResolutionStatus: "RESOLVED", ProviderPoolKey: "FBO:W1", FulfillmentType: "FBO", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED", ProviderLocationKey: "W1", LocationType: "WAREHOUSE", ProviderValidStockCount: &valid, QuantitySemanticStatus: "PENDING_ACCEPTANCE", SourcePayloadID: &payloadID}}}
	if err = stock.NewRepository(pool).Finalize(ctx, seller, stockRun, &stockResult); err != nil {
		t.Fatal(err)
	}

	priceRun, pricePayload := newDomain("PRICE", price.RedactionProfile)
	priceResult := price.Result{ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1, CompletenessStatus: "COMPLETE", UniverseHash: fmt.Sprintf("%064x", 2), Observations: []price.Observation{{CanonicalSKUID: &sku, SKUResolutionStatus: "RESOLVED", CurrencyCode: "RUB", ProviderCurrentPrice: amount("10.25"), CurrentPriceState: "KNOWN", SemanticMappingStatus: "PENDING_ACCEPTANCE", ReconciliationStatus: "NOT_APPLICABLE", SourcePayloadID: &pricePayload}}}
	if err = price.NewRepository(pool).Finalize(ctx, seller, priceRun, &priceResult); err != nil {
		t.Fatal(err)
	}

	adRun, adPayload := newDomain("ADVERTISING", advertising.RedactionProfile)
	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	adResult := advertising.Result{RequestedFrom: day, RequestedTo: day, CompletenessStatus: "COMPLETE", Campaigns: []advertising.CampaignObservation{{ProviderCampaignID: 77, ProviderState: "ACTIVE", CampaignFamily: "UNKNOWN", TargetScopeType: "UNKNOWN", CurrencyCode: "RUB", SourcePayloadID: adPayload}}, Daily: []advertising.DailyObservation{{ProviderCampaignID: 77, BusinessDate: day, PaidSpendAmount: amount("4"), CurrencyCode: "RUB", TrafficState: "USABLE", AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: adPayload}}}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, adRun, &adResult); err != nil {
		t.Fatal(err)
	}

	for domain, want := range map[string]int64{"STOCK": stockRun, "PRICE": priceRun, "ADVERTISING": adRun} {
		var got int64
		if err = pool.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain=$2`, seller, domain).Scan(&got); err != nil || got != want {
			t.Fatalf("%s head=%d err=%v", domain, got, err)
		}
	}
	var stockRows, priceRows, adRows int64
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM stock_snapshot_lines WHERE seller_account_id=$1),(SELECT count(*) FROM price_snapshots WHERE seller_account_id=$1),(SELECT count(*) FROM advertising_daily_facts WHERE seller_account_id=$1)`, seller).Scan(&stockRows, &priceRows, &adRows); err != nil || stockRows != 1 || priceRows != 1 || adRows != 1 {
		t.Fatalf("rows=%d/%d/%d err=%v", stockRows, priceRows, adRows, err)
	}
}
