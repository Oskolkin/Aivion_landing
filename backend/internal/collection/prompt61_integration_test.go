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
	ready "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPrompt61StockSparseMissingResponsePersists(t *testing.T) {
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

	var user, seller, sku1, sku2 int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("prompt61_stock_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'prompt61-stock','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,101,NOW(),NOW(),'ACTIVE')RETURNING id`, seller).Scan(&sku1); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,102,NOW(),NOW(),'ACTIVE')RETURNING id`, seller).Scan(&sku2); err != nil {
		t.Fatal(err)
	}

	var run, dr, payload int64
	if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','prompt61.stock')RETURNING id`, seller).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'STOCK','RUNNING',NOW())RETURNING id`, run, seller).Scan(&dr); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.stock','fp-stock',1,'raw',$3,$4,200,NOW(),'{}',$5)RETURNING id`, seller, dr, fmt.Sprintf("prompt61/stock/%d", dr), fmt.Sprintf("%064x", dr), stock.RedactionProfile).Scan(&payload); err != nil {
		t.Fatal(err)
	}

	ten := int64(10)
	zero := int64(0)
	payloadID := payload
	result := stock.Result{
		ExpectedSKUCount: 2, RequestedSKUCount: 2, ReceivedSKUCount: 1,
		MissingResponseSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1,
		CompletenessStatus: "PARTIAL", UniverseHash: fmt.Sprintf("%064x", 61),
		Positions: []stock.Position{
			{CanonicalSKUID: &sku1, SourceSKU: 101, SKUResolutionStatus: "RESOLVED", ProviderPoolKey: "FBO:W1", FulfillmentType: "FBO", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED", ProviderLocationKey: "W1", LocationType: "WAREHOUSE", ProviderValidStockCount: &ten, QuantitySemanticStatus: "PENDING_ACCEPTANCE", SourcePayloadID: &payloadID},
			{CanonicalSKUID: &sku2, SourceSKU: 102, SKUResolutionStatus: "RESOLVED", ProviderPoolKey: "UNKNOWN:ABSENCE", FulfillmentType: "UNKNOWN", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED", QuantitySemanticStatus: "PENDING_ACCEPTANCE", AbsenceReason: "MISSING_PROVIDER_RESPONSE_ROW"},
		},
	}
	if err = stock.NewRepository(pool).Finalize(ctx, seller, dr, &result); err != nil {
		t.Fatalf("sparse finalize failed: %v", err)
	}
	var received, missing, resolved, unresolved int64
	var completeness string
	if err = pool.QueryRow(ctx, `SELECT received_sku_count,missing_response_sku_count,resolved_sku_count,unresolved_sku_count,completeness_status FROM stock_snapshots WHERE id=$1`, result.SnapshotID).Scan(&received, &missing, &resolved, &unresolved, &completeness); err != nil {
		t.Fatal(err)
	}
	if received != 1 || missing != 1 || resolved != 1 || unresolved != 0 || completeness != "PARTIAL" {
		t.Fatalf("snapshot counts received=%d missing=%d resolved=%d unresolved=%d completeness=%s", received, missing, resolved, unresolved, completeness)
	}
	var absence string
	var absencePayload *int64
	if err = pool.QueryRow(ctx, `SELECT COALESCE(absence_reason,''),source_payload_id FROM stock_snapshot_lines WHERE snapshot_id=$1 AND source_sku=102`, result.SnapshotID).Scan(&absence, &absencePayload); err != nil {
		t.Fatal(err)
	}
	if absence != "MISSING_PROVIDER_RESPONSE_ROW" || absencePayload != nil {
		t.Fatalf("absence line reason=%s payload=%v", absence, absencePayload)
	}

	// Case B: known zero is distinct from missing.
	var run2, dr2, payload2 int64
	if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','prompt61.stock.zero')RETURNING id`, seller).Scan(&run2); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'STOCK','RUNNING',NOW())RETURNING id`, run2, seller).Scan(&dr2); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.stock','fp-stock-zero',1,'raw',$3,$4,200,NOW(),'{}',$5)RETURNING id`, seller, dr2, fmt.Sprintf("prompt61/stock-zero/%d", dr2), fmt.Sprintf("%064x", dr2+1), stock.RedactionProfile).Scan(&payload2); err != nil {
		t.Fatal(err)
	}
	p2 := payload2
	zeroResult := stock.Result{
		ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1,
		CompletenessStatus: "COMPLETE", UniverseHash: fmt.Sprintf("%064x", 62),
		Positions: []stock.Position{{CanonicalSKUID: &sku2, SourceSKU: 102, SKUResolutionStatus: "RESOLVED", ProviderPoolKey: "FBO:W1", FulfillmentType: "FBO", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED", ProviderLocationKey: "W1", LocationType: "WAREHOUSE", ProviderValidStockCount: &zero, QuantitySemanticStatus: "PENDING_ACCEPTANCE", SourcePayloadID: &p2}},
	}
	if err = stock.NewRepository(pool).Finalize(ctx, seller, dr2, &zeroResult); err != nil {
		t.Fatal(err)
	}
	var knownZero *int64
	if err = pool.QueryRow(ctx, `SELECT provider_valid_stock_count FROM stock_snapshot_lines WHERE snapshot_id=$1`, zeroResult.SnapshotID).Scan(&knownZero); err != nil || knownZero == nil || *knownZero != 0 {
		t.Fatalf("known zero lost: %v err=%v", knownZero, err)
	}
}

func TestPrompt61AdvertisingTargetEmptyClosureAndCPOIdempotency(t *testing.T) {
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
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("prompt61_ad_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'prompt61-ad','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,101,NOW(),NOW(),'ACTIVE')RETURNING id`, seller).Scan(&sku); err != nil {
		t.Fatal(err)
	}
	amount := func(raw string) *sales.Decimal {
		v, e := sales.ParseAmount(json.Number(raw))
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	newDomain := func(label string) (int64, int64) {
		var run, dr, payload int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING',$2)RETURNING id`, seller, label).Scan(&run); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'ADVERTISING','RUNNING',NOW())RETURNING id`, run, seller).Scan(&dr); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.ad',$3,1,'raw',$4,$5,200,NOW(),'{}',$6)RETURNING id`, seller, dr, label, fmt.Sprintf("prompt61/ad/%s", label), fmt.Sprintf("%064x", time.Now().UnixNano()), advertising.RedactionProfile).Scan(&payload); e != nil {
			t.Fatal(e)
		}
		return dr, payload
	}

	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	sourceSKU := int64(101)
	run1, payload1 := newDomain("prompt61.target.r1")
	r1 := advertising.Result{
		RequestedFrom: day, RequestedTo: day, CompletenessStatus: "COMPLETE",
		Campaigns:                      []advertising.CampaignObservation{{ProviderCampaignID: 9, ProviderState: "ACTIVE", CampaignFamily: "PRODUCT_CPC", TargetScopeType: "EXPLICIT_SKU_SET", CurrencyCode: "RUB", SourcePayloadID: payload1}},
		AuthoritativeTargetCampaignIDs: []int64{9},
		Targets:                        []advertising.TargetObservation{{ProviderCampaignID: 9, CanonicalSKUID: &sku, SourceSKU: &sourceSKU, SKUResolutionStatus: "RESOLVED", AssignmentState: "ACTIVE", SourcePayloadID: payload1}},
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, run1, &r1); err != nil {
		t.Fatal(err)
	}
	var openCount int64
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM advertising_target_intervals WHERE seller_account_id=$1 AND observed_to IS NULL`, seller).Scan(&openCount); err != nil || openCount != 1 {
		t.Fatalf("open targets after run1=%d err=%v", openCount, err)
	}

	run2, payload2 := newDomain("prompt61.target.r2")
	r2 := advertising.Result{
		RequestedFrom: day, RequestedTo: day, CompletenessStatus: "COMPLETE",
		Campaigns:                      []advertising.CampaignObservation{{ProviderCampaignID: 9, ProviderState: "ACTIVE", CampaignFamily: "PRODUCT_CPC", TargetScopeType: "EXPLICIT_SKU_SET", CurrencyCode: "RUB", SourcePayloadID: payload2}},
		AuthoritativeTargetCampaignIDs: []int64{9},
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, run2, &r2); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM advertising_target_intervals WHERE seller_account_id=$1 AND observed_to IS NULL`, seller).Scan(&openCount); err != nil || openCount != 0 {
		t.Fatalf("empty authoritative set must close intervals, open=%d err=%v", openCount, err)
	}

	run3, payload3 := newDomain("prompt61.target.r3.fail")
	r3 := advertising.Result{
		RequestedFrom: day, RequestedTo: day, CompletenessStatus: "PARTIAL",
		Campaigns: []advertising.CampaignObservation{{ProviderCampaignID: 9, ProviderState: "ACTIVE", CampaignFamily: "PRODUCT_CPC", TargetScopeType: "UNKNOWN", CurrencyCode: "RUB", SourcePayloadID: payload3}},
		// no AuthoritativeTargetCampaignIDs → failed target collection must not reopen/close
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, run3, &r3); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM advertising_target_intervals WHERE seller_account_id=$1 AND observed_to IS NULL`, seller).Scan(&openCount); err != nil || openCount != 0 {
		t.Fatalf("failed target collection altered closed intervals, open=%d", openCount)
	}

	// CPO idempotency + revision
	runA, payloadA := newDomain("prompt61.cpo.a")
	orderID := int64(555)
	cpoA := advertising.Result{
		RequestedFrom: day, RequestedTo: day, CompletenessStatus: "COMPLETE",
		Campaigns: []advertising.CampaignObservation{{ProviderCampaignID: 9, ProviderState: "ACTIVE", CampaignFamily: "SEARCH_PROMO_CPO", TargetScopeType: "UNKNOWN", CurrencyCode: "RUB", SourcePayloadID: payloadA}},
		CPO: []advertising.CPOObservation{{
			ProviderCampaignID: 9, BusinessDate: day, SourceOrderID: &orderID, SourcePromotedSKU: &sourceSKU, SourceOrderedSKU: &sourceSKU,
			PromotedSKUResolutionStatus: "UNRESOLVED", OrderedSKUResolutionStatus: "UNRESOLVED", OrderResolutionStatus: "UNRESOLVED",
			AttributedOrderAmount: amount("100.00"), CurrencyCode: "RUB", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: payloadA,
		}},
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, runA, &cpoA); err != nil {
		t.Fatal(err)
	}
	runB, payloadB := newDomain("prompt61.cpo.b")
	cpoB := cpoA
	cpoB.Campaigns[0].SourcePayloadID = payloadB
	cpoB.CPO[0].SourcePayloadID = payloadB
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, runB, &cpoB); err != nil {
		t.Fatal(err)
	}
	var cpoCount, maxRev int64
	if err = pool.QueryRow(ctx, `SELECT count(*),max(revision_no) FROM advertising_cpo_facts WHERE seller_account_id=$1`, seller).Scan(&cpoCount, &maxRev); err != nil || cpoCount != 1 || maxRev != 1 {
		t.Fatalf("idempotent reread expected 1/1 got count=%d rev=%d err=%v", cpoCount, maxRev, err)
	}
	runC, payloadC := newDomain("prompt61.cpo.c")
	cpoC := cpoA
	cpoC.Campaigns[0].SourcePayloadID = payloadC
	cpoC.CPO[0].SourcePayloadID = payloadC
	cpoC.CPO[0].AttributedOrderAmount = amount("150.00")
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, runC, &cpoC); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*),max(revision_no) FROM advertising_cpo_facts WHERE seller_account_id=$1`, seller).Scan(&cpoCount, &maxRev); err != nil || cpoCount != 2 || maxRev != 2 {
		t.Fatalf("correction expected 2/2 got count=%d rev=%d err=%v", cpoCount, maxRev, err)
	}

	// Spend reconciliation latest-as-known-at
	runD, payloadD := newDomain("prompt61.recon.d")
	reconDiff := advertising.Result{
		RequestedFrom: day, RequestedTo: day, CompletenessStatus: "COMPLETE",
		Campaigns: []advertising.CampaignObservation{{ProviderCampaignID: 9, ProviderState: "ACTIVE", CampaignFamily: "PRODUCT_CPC", TargetScopeType: "UNKNOWN", CurrencyCode: "RUB", SourcePayloadID: payloadD}},
		Daily:     []advertising.DailyObservation{{ProviderCampaignID: 9, BusinessDate: day, PaidSpendAmount: amount("10"), CurrencyCode: "RUB", TrafficState: "USABLE", AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: payloadD}},
		Expense:   []advertising.ExpenseObservation{{BusinessDate: day, PaidSpendAmount: amount("9"), CurrencyCode: "RUB", SourcePayloadID: payloadD}},
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, runD, &reconDiff); err != nil {
		t.Fatal(err)
	}
	runE, payloadE := newDomain("prompt61.recon.e")
	reconPass := advertising.Result{
		RequestedFrom: day, RequestedTo: day, CompletenessStatus: "COMPLETE",
		Campaigns: []advertising.CampaignObservation{{ProviderCampaignID: 9, ProviderState: "ACTIVE", CampaignFamily: "PRODUCT_CPC", TargetScopeType: "UNKNOWN", CurrencyCode: "RUB", SourcePayloadID: payloadE}},
		Daily:     []advertising.DailyObservation{{ProviderCampaignID: 9, BusinessDate: day, PaidSpendAmount: amount("10"), CurrencyCode: "RUB", TrafficState: "USABLE", AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: payloadE}},
		Expense:   []advertising.ExpenseObservation{{BusinessDate: day, PaidSpendAmount: amount("10"), CurrencyCode: "RUB", SourcePayloadID: payloadE}},
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, runE, &reconPass); err != nil {
		t.Fatal(err)
	}
	var latestStatus string
	if err = pool.QueryRow(ctx, `SELECT reconciliation_status FROM advertising_spend_reconciliations WHERE seller_account_id=$1 AND business_date=$2 ORDER BY observed_at DESC,id DESC LIMIT 1`, seller, day).Scan(&latestStatus); err != nil || latestStatus != "PASS" {
		t.Fatalf("latest recon=%s err=%v", latestStatus, err)
	}
	var hist int64
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM advertising_spend_reconciliations WHERE seller_account_id=$1 AND business_date=$2 AND reconciliation_status='DIFFERENT'`, seller, day).Scan(&hist); err != nil || hist < 1 {
		t.Fatalf("history DIFFERENT should remain hist=%d err=%v", hist, err)
	}
}

func TestPrompt61WatermarkCompleteVsPartial(t *testing.T) {
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
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("prompt61_wm_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'prompt61-wm','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,101,NOW(),NOW(),'ACTIVE')RETURNING id`, seller).Scan(&sku); err != nil {
		t.Fatal(err)
	}
	newDomain := func(domain, profile string) (int64, int64) {
		var run, dr, payload int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','prompt61.wm')RETURNING id`, seller).Scan(&run); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,$3,'RUNNING',NOW())RETURNING id`, run, seller, domain).Scan(&dr); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON',$3,$4,1,'raw',$5,$6,200,NOW(),'{}',$7)RETURNING id`, seller, dr, "prompt61.wm."+domain, fmt.Sprintf("fp-wm-%d", time.Now().UnixNano()), fmt.Sprintf("prompt61/wm/%d", time.Now().UnixNano()), fmt.Sprintf("%064x", time.Now().UnixNano()), profile).Scan(&payload); e != nil {
			t.Fatal(e)
		}
		return dr, payload
	}
	ten := int64(10)
	completeAt := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	partialAt := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	stockCompleteRun, stockCompletePayload := newDomain("STOCK", stock.RedactionProfile)
	p1 := stockCompletePayload
	completeResult := stock.Result{
		StartedAt: completeAt, CompletedAt: completeAt,
		ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1,
		CompletenessStatus: "COMPLETE", UniverseHash: fmt.Sprintf("%064x", 6101),
		Positions: []stock.Position{{CanonicalSKUID: &sku, SourceSKU: 101, SKUResolutionStatus: "RESOLVED", ProviderPoolKey: "FBO:W1", FulfillmentType: "FBO", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED", ProviderLocationKey: "W1", LocationType: "WAREHOUSE", ProviderValidStockCount: &ten, QuantitySemanticStatus: "PENDING_ACCEPTANCE", SourcePayloadID: &p1}},
	}
	if err = stock.NewRepository(pool).Finalize(ctx, seller, stockCompleteRun, &completeResult); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE stock_snapshots SET snapshot_at=$2 WHERE id=$1`, completeResult.SnapshotID, completeAt)

	stockPartialRun, stockPartialPayload := newDomain("STOCK", stock.RedactionProfile)
	p2 := stockPartialPayload
	partialResult := stock.Result{
		StartedAt: partialAt, CompletedAt: partialAt,
		ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, MissingResponseSKUCount: 0, BatchesExpected: 1, BatchesCompleted: 1, BatchesFailed: 0,
		CompletenessStatus: "PARTIAL", UniverseHash: fmt.Sprintf("%064x", 6102),
		Positions: []stock.Position{{CanonicalSKUID: &sku, SourceSKU: 101, SKUResolutionStatus: "RESOLVED", ProviderPoolKey: "FBO:W1", FulfillmentType: "FBO", PoolType: "PROVIDER_PHYSICAL", PoolIndependenceStatus: "UNVERIFIED", ProviderLocationKey: "W1", LocationType: "WAREHOUSE", ProviderValidStockCount: &ten, QuantitySemanticStatus: "PENDING_ACCEPTANCE", SourcePayloadID: &p2}},
	}
	if err = stock.NewRepository(pool).Finalize(ctx, seller, stockPartialRun, &partialResult); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE stock_snapshots SET snapshot_at=$2 WHERE id=$1`, partialResult.SnapshotID, partialAt)

	amount := func(raw string) *sales.Decimal {
		v, e := sales.ParseAmount(json.Number(raw))
		if e != nil {
			t.Fatal(e)
		}
		return &v
	}
	priceCompleteRun, priceCompletePayload := newDomain("PRICE", price.RedactionProfile)
	pc := priceCompletePayload
	priceComplete := price.Result{
		StartedAt: completeAt, CompletedAt: completeAt,
		ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1,
		CompletenessStatus: "COMPLETE", UniverseHash: fmt.Sprintf("%064x", 6201),
		Observations: []price.Observation{{CanonicalSKUID: &sku, SourceSKU: &[]int64{101}[0], SKUResolutionStatus: "RESOLVED", CurrencyCode: "RUB", ProviderCurrentPrice: amount("10.00"), CurrentPriceState: "KNOWN", SemanticMappingStatus: "PENDING_ACCEPTANCE", ReconciliationStatus: "NOT_APPLICABLE", SourcePayloadID: &pc}},
	}
	if err = price.NewRepository(pool).Finalize(ctx, seller, priceCompleteRun, &priceComplete); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE price_collection_snapshots SET snapshot_at=$2 WHERE collection_domain_run_id=$1`, priceCompleteRun, completeAt)
	_, _ = pool.Exec(ctx, `UPDATE price_snapshots SET observed_at=$2,snapshot_at=$2 WHERE collection_domain_run_id=$1`, priceCompleteRun, completeAt)

	pricePartialRun, pricePartialPayload := newDomain("PRICE", price.RedactionProfile)
	pp := pricePartialPayload
	pricePartial := price.Result{
		StartedAt: partialAt, CompletedAt: partialAt,
		ExpectedSKUCount: 1, RequestedSKUCount: 1, ReceivedSKUCount: 1, BatchesExpected: 1, BatchesCompleted: 1,
		CompletenessStatus: "PARTIAL", UniverseHash: fmt.Sprintf("%064x", 6202),
		Observations: []price.Observation{{CanonicalSKUID: &sku, SourceSKU: &[]int64{101}[0], SKUResolutionStatus: "RESOLVED", CurrencyCode: "RUB", ProviderCurrentPrice: amount("11.00"), CurrentPriceState: "KNOWN", SemanticMappingStatus: "PENDING_ACCEPTANCE", ReconciliationStatus: "NOT_APPLICABLE", SourcePayloadID: &pp}},
	}
	if err = price.NewRepository(pool).Finalize(ctx, seller, pricePartialRun, &pricePartial); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE price_collection_snapshots SET snapshot_at=$2 WHERE collection_domain_run_id=$1`, pricePartialRun, partialAt)
	_, _ = pool.Exec(ctx, `UPDATE price_snapshots SET observed_at=$2,snapshot_at=$2 WHERE collection_domain_run_id=$1`, pricePartialRun, partialAt)

	day1 := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	adRun, adPayload := newDomain("ADVERTISING", advertising.RedactionProfile)
	adResult := advertising.Result{
		RequestedFrom: day1, RequestedTo: day3, CompletenessStatus: "COMPLETE",
		Campaigns: []advertising.CampaignObservation{{ProviderCampaignID: 88, ProviderState: "ACTIVE", CampaignFamily: "PRODUCT_CPC", TargetScopeType: "UNKNOWN", CurrencyCode: "RUB", SourcePayloadID: adPayload}},
		Daily: []advertising.DailyObservation{
			{ProviderCampaignID: 88, BusinessDate: day1, PaidSpendAmount: amount("1"), CurrencyCode: "RUB", TrafficState: "USABLE", AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: adPayload},
			{ProviderCampaignID: 88, BusinessDate: day2, PaidSpendAmount: amount("1"), CurrencyCode: "RUB", TrafficState: "PARTIAL", AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: adPayload},
			{ProviderCampaignID: 88, BusinessDate: day3, PaidSpendAmount: amount("1"), CurrencyCode: "RUB", TrafficState: "USABLE", AttributionState: "PENDING_ACCEPTANCE", AttributionKind: "PROVIDER_DEFAULT", SourcePayloadID: adPayload},
		},
	}
	if err = advertising.NewRepository(pool).Finalize(ctx, seller, adRun, &adResult); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `UPDATE advertising_daily_facts SET observed_at=$2 WHERE seller_account_id=$1`, seller, partialAt)
	_, _ = pool.Exec(ctx, `UPDATE advertising_campaign_versions SET observed_at=$2 WHERE seller_account_id=$1`, seller, completeAt)
	_, _ = pool.Exec(ctx, `UPDATE advertising_campaigns SET first_observed_at=$2,last_observed_at=$2 WHERE seller_account_id=$1`, seller, completeAt)

	cutoff := partialAt.Add(time.Hour)
	svc := ready.NewService(pool, &wmClock{at: cutoff}, ready.StaticSemanticRegistry{})
	result, err := svc.Evaluate(ctx, seller)
	if err != nil {
		t.Fatal(err)
	}
	var stockWM, priceWM, adTraffic ready.Watermark
	for _, w := range result.Watermarks {
		switch w.FactFamily {
		case "STOCK.SNAPSHOT":
			stockWM = w
		case "PRICE.SNAPSHOT":
			priceWM = w
		case "ADVERTISING.TRAFFIC":
			adTraffic = w
		}
	}
	if stockWM.LatestObservedAt == nil || !stockWM.LatestObservedAt.Equal(partialAt) {
		t.Fatalf("stock latest observed want %v got %v", partialAt, stockWM.LatestObservedAt)
	}
	if stockWM.LatestCompleteSnapshotAt == nil || !stockWM.LatestCompleteSnapshotAt.Equal(completeAt) {
		t.Fatalf("stock latest complete want %v got %v", completeAt, stockWM.LatestCompleteSnapshotAt)
	}
	if priceWM.LatestObservedAt == nil || !priceWM.LatestObservedAt.Equal(partialAt) {
		t.Fatalf("price latest observed want %v got %v", partialAt, priceWM.LatestObservedAt)
	}
	if priceWM.LatestCompleteSnapshotAt == nil || !priceWM.LatestCompleteSnapshotAt.Equal(completeAt) {
		t.Fatalf("price latest complete want %v got %v", completeAt, priceWM.LatestCompleteSnapshotAt)
	}
	if adTraffic.LatestCompleteBusinessDate == nil || adTraffic.LatestCompleteBusinessDate.Format("2006-01-02") != "2026-08-22" {
		t.Fatalf("ad traffic complete-through must stop before incomplete day, got %v", adTraffic.LatestCompleteBusinessDate)
	}
}

type wmClock struct{ at time.Time }

func (c *wmClock) Now() time.Time { return c.at }
