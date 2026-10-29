//go:build integration

package analysis_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	prompt94ScaleCategories = 25
	prompt94ScaleCampaigns  = 25
	prompt94ScaleOzonBase   = int64(920_000)
)

// prompt94ScaleSeed inserts a seller with skuCount eligible SKUs, prompt94ScaleCategories leaf
// categories, and prompt94ScaleCampaigns campaigns — enough factual surface for V3 BuildCore gates.
func prompt94ScaleSeed(t *testing.T, pool *pgxpool.Pool, skuCount int) prompt81Fixture {
	t.Helper()
	if skuCount < 1 {
		t.Fatalf("skuCount must be >= 1, got %d", skuCount)
	}
	_ = os.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	ctx := context.Background()
	seller := prompt81Seller(t, pool)
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	analysisDay := cut.AddDate(0, 0, -1)
	obs := cut.Add(-2 * time.Hour)
	curFrom := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	prevFrom := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	prevTo := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	repo := analysispg.New(pool)
	store := artifacts.NewMemoryStore()
	bundle, err := contracts.Current()
	if err != nil {
		t.Fatal(err)
	}
	body, _ := bundle.MarshalCanonical()
	if err = repo.PutContractBundle(ctx, bundle.Hash(), "", body); err != nil {
		t.Fatal(err)
	}

	newRun := func(domain string, at time.Time) int64 {
		t.Helper()
		var parent, run int64
		if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)
			VALUES($1,'MANUAL','COMPLETED','prompt94.scale',$2,$2)RETURNING id`, seller, at).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(
			collection_run_id,seller_account_id,domain,status,requested_from,requested_to,coverage_from,coverage_to,
			records_received,records_written,pages_received,completeness_status,
			source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,started_at,finished_at)
			VALUES($1,$2,$3,'COMPLETED','2026-07-26','2026-08-24','2026-07-26','2026-08-24',1,1,1,'COMPLETE',
			$4,'v1',$4,'v1',$5,$5)RETURNING id`, parent, seller, domain, "contract."+strings.ToLower(domain), at).Scan(&run); err != nil {
			t.Fatal(err)
		}
		return run
	}
	payload := func(run int64, endpoint string, seq int) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO source_payloads(
			seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,
			storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile)
			VALUES($1,$2,'OZON',$3,$4,$5,'raw',$6,$7,200,$8,'TEST_REDACTION_V1')RETURNING id`,
			seller, run, endpoint, fmt.Sprintf("p94-fp-%d-%d", run, seq), seq,
			fmt.Sprintf("prompt94/%d/%d", run, seq), fmt.Sprintf("%064x", run*1000+int64(seq)), obs).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}

	taxonomyRun := newRun("TAXONOMY", cut.Add(-5*time.Hour))
	catalogRun := newRun("CATALOG", cut.Add(-4*time.Hour))
	salesRun := newRun("SALES_DEMAND", cut.Add(-3*time.Hour))
	stockRun := newRun("STOCK", cut.Add(-3*time.Hour))
	priceRun := newRun("PRICE", cut.Add(-3*time.Hour))
	adsRun := newRun("ADVERTISING", cut.Add(-3*time.Hour))
	searchRun := newRun("SEARCH", cut.Add(-3*time.Hour))
	supplyRun := newRun("SUPPLY", cut.Add(-3*time.Hour))
	funnelRun := newRun("FUNNEL", cut.Add(-3*time.Hour))
	promoRun := newRun("PROMOTIONS", cut.Add(-3*time.Hour))
	financeRun := newRun("FINANCE", cut.Add(-3*time.Hour))
	healthRun := newRun("ACCOUNT_HEALTH", cut.Add(-3*time.Hour))

	for domain, run := range map[string]int64{
		"CATALOG": catalogRun, "SALES_DEMAND": salesRun, "STOCK": stockRun,
		"PRICE": priceRun, "ADVERTISING": adsRun, "SEARCH": searchRun, "SUPPLY": supplyRun,
		"FUNNEL": funnelRun, "PROMOTIONS": promoRun, "FINANCE": financeRun, "ACCOUNT_HEALTH": healthRun,
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)
			VALUES($1,$2,$3,$4)ON CONFLICT DO NOTHING`, seller, domain, run, obs); err != nil {
			t.Fatal(err)
		}
	}
	catalogPayload := payload(catalogRun, "catalog", 1)
	salesPayload := payload(salesRun, "sales", 1)
	stockPayload := payload(stockRun, "stock", 1)
	pricePayload := payload(priceRun, "price", 1)
	adsPayload := payload(adsRun, "advertising", 1)
	searchPayload := payload(searchRun, "search", 1)
	_ = catalogPayload

	var taxSync int64
	if err := pool.QueryRow(ctx, `INSERT INTO taxonomy_sync_runs(collection_domain_run_id,taxonomy_hash,status,observed_at)
		VALUES($1,$2,'COMPLETED',$3)RETURNING id`, taxonomyRun, strings.Repeat("a", 64), cut.Add(-4*time.Hour)).Scan(&taxSync); err != nil {
		t.Fatal(err)
	}

	catBase := time.Now().UnixNano() % 1_000_000_000
	catIDs := make([]int64, 0, prompt94ScaleCategories)
	for i := 0; i < prompt94ScaleCategories; i++ {
		ext := catBase + int64(i)
		var pc, sc int64
		if err := pool.QueryRow(ctx, `INSERT INTO provider_categories(provider,provider_category_id,first_observed_at,last_observed_at)
			VALUES('OZON',$1,$2,$2)RETURNING id`, ext, cut.Add(-40*24*time.Hour)).Scan(&pc); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO provider_category_versions(
			provider_category_id,name,disabled,is_leaf,taxonomy_state,taxonomy_sync_run_id,observed_at,normalized_hash)
			VALUES($1,$2,false,true,'ACTIVE',$3,$4,$5)`,
			pc, fmt.Sprintf("ScaleCat-%d", i), taxSync, cut.Add(-30*24*time.Hour), fmt.Sprintf("%064x", ext)); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO seller_categories(seller_account_id,provider_category_id,first_observed_at,last_observed_at)
			VALUES($1,$2,$3,$3)RETURNING id`, seller, pc, cut.Add(-30*24*time.Hour)).Scan(&sc); err != nil {
			t.Fatal(err)
		}
		catIDs = append(catIDs, sc)
	}

	campIDs := make([]int64, 0, prompt94ScaleCampaigns)
	for i := 0; i < prompt94ScaleCampaigns; i++ {
		providerID := int64(8800 + i)
		var cid int64
		if err := pool.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)
			VALUES($1,$2,$3,$3)RETURNING id`, seller, providerID, cut.Add(-20*24*time.Hour)).Scan(&cid); err != nil {
			t.Fatal(err)
		}
		campIDs = append(campIDs, cid)
		if _, err := pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(
			seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,
			state_hash,observed_at,source_payload_id,collection_domain_run_id)
			VALUES($1,$2,1,$3,'ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$4,$5,$6,$7)`,
			seller, cid, fmt.Sprintf("ScaleCamp-%d", providerID), prompt81Hex(fmt.Sprintf("scale-camp-%d", providerID)), obs, adsPayload, adsRun); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := pool.Exec(ctx, `
WITH inserted AS (
	INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)
	SELECT $1, $4::bigint + g, $2, $2, 'ACTIVE'
	FROM generate_series(1, $3::int) AS g
	RETURNING id, ozon_sku
),
numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx FROM inserted
),
cats AS (
	SELECT id, (row_number() OVER (ORDER BY id) - 1) AS cat_idx
	FROM seller_categories WHERE seller_account_id = $1 ORDER BY id LIMIT $5
)
INSERT INTO canonical_sku_versions(
	canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)
SELECT n.id, n.ozon_sku + 100, 'offer-' || n.ozon_sku::text, 'ScaleSKU-' || n.ozon_sku::text,
	'active', 'ACTIVE', encode(digest(n.ozon_sku::text, 'sha256'), 'hex'), $2, $6
FROM numbered n`,
		seller, cut.Add(-10*24*time.Hour), skuCount, prompt94ScaleOzonBase, prompt94ScaleCategories, catalogRun); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
),
cats AS (
	SELECT id, (row_number() OVER (ORDER BY id) - 1) AS cat_idx
	FROM seller_categories WHERE seller_account_id = $1 ORDER BY id LIMIT $3
)
INSERT INTO sku_category_assignments(
	canonical_sku_id,seller_category_id,mapping_status,category_resolution_status,product_type_resolution_status,
	observed_from,observed_to,collection_domain_run_id)
SELECT n.id, c.id, 'MAPPED', 'RESOLVED', 'NOT_APPLICABLE', $4, NULL, $5
FROM numbered n
JOIN cats c ON c.cat_idx = (n.idx % $3)`,
		seller, prompt94ScaleOzonBase, prompt94ScaleCategories, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), catalogRun); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
),
days AS (
	SELECT d::date AS business_date, CASE WHEN d::date >= $3::date THEN 'cur' ELSE 'prev' END AS win
	FROM generate_series($4::date, $5::date, '1 day') AS d
)
INSERT INTO sales_demand_daily(
	seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,
	sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)
SELECT $1, n.id, n.ozon_sku, d.business_date,
	(10 + (n.idx % 7))::numeric, (10 + (n.idx % 7))::bigint, 'RUB', 'RESOLVED', 1,
	encode(digest(n.id::text || d.business_date::text, 'sha256'), 'hex'), $6, $7, $8
FROM numbered n CROSS JOIN days d`,
		seller, prompt94ScaleOzonBase, curFrom, prevFrom, analysisDay, salesRun, salesPayload, obs); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO sales_account_daily(
	seller_account_id,business_date,ordered_amount,ordered_units,currency_code,revision_no,fact_hash,
	collection_domain_run_id,source_payload_id,observed_at)
SELECT $1, business_date, SUM(ordered_amount), SUM(ordered_units), 'RUB', 1,
	encode(digest(business_date::text, 'sha256'), 'hex'), $2, $3, $4
FROM sales_demand_daily WHERE seller_account_id = $1
GROUP BY business_date`,
		seller, salesRun, salesPayload, obs); err != nil {
		t.Fatal(err)
	}

	var poolID int64
	if err := pool.QueryRow(ctx, `INSERT INTO inventory_pools(
		seller_account_id,provider_pool_key,fulfillment_type,pool_type,independence_status,first_observed_at,last_observed_at)
		VALUES($1,'FBO:P94','FBO','PROVIDER_PHYSICAL','VERIFIED_INDEPENDENT',$2,$2)RETURNING id`, seller, obs).Scan(&poolID); err != nil {
		t.Fatal(err)
	}
	var snapID int64
	if err := pool.QueryRow(ctx, `INSERT INTO stock_snapshots(
		seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,
		requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,
		position_count,batches_expected,batches_completed,batches_failed,completeness_status,semantic_mapping_status,state_hash)
		VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,$5,$5,$5,$5,0,$5,1,1,0,'COMPLETE','VALIDATED',$6)RETURNING id`,
		seller, stockRun, obs, strings.Repeat("1", 64), skuCount, strings.Repeat("2", 64)).Scan(&snapID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
)
INSERT INTO stock_snapshot_lines(
	seller_account_id,snapshot_id,canonical_sku_id,source_sku,sku_resolution_status,inventory_pool_id,fulfillment_type,
	sellable_units,provider_valid_stock_count,quantity_semantic_status,source_payload_id,observed_at)
SELECT $1, $3, n.id, n.ozon_sku, 'RESOLVED', $4, 'FBO',
	5 + (n.idx % 20), 5 + (n.idx % 20), 'VALIDATED', $5, $6
FROM numbered n`,
		seller, prompt94ScaleOzonBase, snapID, poolID, stockPayload, obs); err != nil {
		t.Fatal(err)
	}

	var priceColl int64
	if err := pool.QueryRow(ctx, `INSERT INTO price_collection_snapshots(
		seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,
		requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,
		batches_expected,batches_completed,batches_failed,completeness_status,state_hash)
		VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,$5,$5,$5,$5,0,1,1,0,'COMPLETE',$6)RETURNING id`,
		seller, priceRun, obs, strings.Repeat("3", 64), skuCount, strings.Repeat("4", 64)).Scan(&priceColl); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
)
INSERT INTO price_snapshots(
	seller_account_id,price_collection_snapshot_id,canonical_sku_id,source_sku,provider_product_id,sku_resolution_status,
	collection_domain_run_id,observed_at,snapshot_at,time_quality,currency_code,provider_current_price,current_price_state,
	old_reference_price,customer_price,semantic_mapping_status,state_hash,source_payload_id,reconciliation_status)
SELECT $1, $3, n.id, n.ozon_sku, n.ozon_sku + 100, 'RESOLVED', $4, $5, $5, 'OBSERVED_APPROX', 'RUB',
	(100 + n.idx * 3), 'KNOWN', (120 + n.idx * 3), (100 + n.idx * 3), 'VALIDATED',
	encode(digest(n.id::text, 'sha256'), 'hex'), $6, 'NOT_APPLICABLE'
FROM numbered n`,
		seller, prompt94ScaleOzonBase, priceColl, priceRun, obs, pricePayload); err != nil {
		t.Fatal(err)
	}

	for _, campID := range campIDs {
		for dayOffset := -13; dayOffset <= 0; dayOffset++ {
			d := analysisDay.AddDate(0, 0, dayOffset)
			spend := 5 + int64(dayOffset+13)
			if _, err := pool.Exec(ctx, `INSERT INTO advertising_daily_facts(
				seller_account_id,advertising_campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,
				attributed_order_units,attributed_order_amount,attribution_kind,provider_drr,currency_code,traffic_state,attribution_state,
				normalized_day_hash,source_contract_key,source_payload_id,collection_domain_run_id,observed_at)
				VALUES($1,$2,$3,1,$4,$5,$6::numeric,2,$7::numeric,'PROVIDER_DEFAULT',50,'RUB','USABLE','MATURE',
				$8,'ADVERTISING.CAMPAIGN_TRAFFIC',$9,$10,$11)`,
				seller, campID, d, 100+spend, 10+spend/2, spend, spend*2,
				prompt81Hex(fmt.Sprintf("scale-ad-%d-%s", campID, d.Format("2006-01-02"))), adsPayload, adsRun, obs); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
),
days AS (
	SELECT d::date AS business_date FROM generate_series($3::date, $4::date, '1 day') AS d
),
camps AS (
	SELECT id, (row_number() OVER (ORDER BY id) - 1) AS camp_idx
	FROM advertising_campaigns WHERE seller_account_id = $1 ORDER BY id LIMIT $5
)
INSERT INTO advertising_product_daily_facts(
	seller_account_id,advertising_campaign_id,business_date,promoted_canonical_sku_id,source_promoted_sku,sku_resolution_status,
	impressions,clicks,paid_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,currency_code,
	revision_no,state_hash,observed_at,source_payload_id,collection_domain_run_id)
SELECT $1, c.id, d.business_date, n.id, n.ozon_sku, 'RESOLVED',
	50, 5, 2::numeric, 1, 4::numeric, 'PROVIDER_DEFAULT', 'RUB', 1,
	encode(digest(n.id::text || d.business_date::text, 'sha256'), 'hex'), $6, $7, $8
FROM numbered n
JOIN camps c ON c.camp_idx = (n.idx % $5)
CROSS JOIN days d
WHERE n.idx < 200`,
		seller, prompt94ScaleOzonBase, prevFrom, analysisDay, prompt94ScaleCampaigns, obs, adsPayload, adsRun); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
)
INSERT INTO search_sku_summaries(
	seller_account_id,canonical_sku_id,source_sku,sku_resolution_status,period_start,period_end,period_kind,
	provider_search_gmv,orders,unique_search_users,unique_view_users,position,view_conversion,
	revision_no,state_hash,collection_domain_run_id,source_payload_id,observed_at)
SELECT $1, n.id, n.ozon_sku, 'RESOLVED', $3::date, $4::date, 'CURRENT_7D',
	100::numeric, 10::numeric, 20::numeric, 15::numeric, (5 + (n.idx % 10))::numeric, 0.12::numeric,
	1, lpad(to_hex(n.id), 64, '0'), $5, $6, $7
FROM numbered n`,
		seller, prompt94ScaleOzonBase, curFrom, analysisDay, searchRun, searchPayload, obs); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
WITH numbered AS (
	SELECT id, ozon_sku, (row_number() OVER (ORDER BY id) - 1) AS idx
	FROM canonical_skus WHERE seller_account_id = $1 AND ozon_sku >= $2
)
INSERT INTO search_sku_summaries(
	seller_account_id,canonical_sku_id,source_sku,sku_resolution_status,period_start,period_end,period_kind,
	provider_search_gmv,orders,unique_search_users,unique_view_users,position,view_conversion,
	revision_no,state_hash,collection_domain_run_id,source_payload_id,observed_at)
SELECT $1, n.id, n.ozon_sku, 'RESOLVED', $3::date, $4::date, 'PREVIOUS_7D',
	90::numeric, 9::numeric, 18::numeric, 14::numeric, (9 + (n.idx % 10))::numeric, 0.11::numeric,
	1, lpad(to_hex(n.id + 1000000), 64, '0'), $5, $6, $7
FROM numbered n`,
		seller, prompt94ScaleOzonBase, prevFrom, prevTo, searchRun, searchPayload, obs); err != nil {
		t.Fatal(err)
	}

	histFrom := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	histTo := analysisDay
	wm := func(domain, family string, runID int64, state string) {
		t.Helper()
		wh := prompt81Hex(state)
		if _, err := pool.Exec(ctx, `INSERT INTO domain_watermarks(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
			source_contract_hash,state_hash,assessed_at)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$4,$6::date,$5::date,$7,$8,$8,$4)
			ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET
			  latest_observed_at=EXCLUDED.latest_observed_at,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at,
			  last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id,source_contract_hash=EXCLUDED.source_contract_hash,
			  history_available_from=EXCLUDED.history_available_from,history_available_to=EXCLUDED.history_available_to,
			  latest_complete_business_date=EXCLUDED.latest_complete_business_date,latest_mature_business_date=EXCLUDED.latest_mature_business_date,
			  latest_complete_snapshot_at=EXCLUDED.latest_complete_snapshot_at`,
			seller, domain, family, obs, histTo, histFrom, runID, wh); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
			source_contract_hash,state_hash,assessed_at,knowledge_time)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$4,$6::date,$5::date,$7,$8,$8,$4,$4)`,
			seller, domain, family, obs, histTo, histFrom, runID, wh); err != nil {
			t.Fatal(err)
		}
	}
	wm("CATALOG", "CATALOG.SKU_UNIVERSE", catalogRun, "wm-catalog")
	wm("SALES_DEMAND", "SALES.ACCOUNT_DEMAND", salesRun, "wm-sales-acct")
	wm("SALES_DEMAND", "SALES.SKU_DEMAND", salesRun, "wm-sales-sku")
	wm("SALES_DEMAND", "SALES.CATEGORY_DEMAND", salesRun, "wm-sales-cat")
	wm("STOCK", "STOCK.SNAPSHOT", stockRun, "wm-stock")
	wm("PRICE", "PRICE.SNAPSHOT", priceRun, "wm-price")
	wm("ADVERTISING", "ADVERTISING.TRAFFIC", adsRun, "wm-ads-traffic")
	wm("ADVERTISING", "ADVERTISING.ATTRIBUTION", adsRun, "wm-ads-attr")
	wm("SEARCH", "SEARCH.SELLER_SUMMARY", searchRun, "wm-search")
	wm("SEARCH", "SEARCH.QUERY_DETAILS", searchRun, "wm-search-query")
	wm("SUPPLY", "SUPPLY.STATE", supplyRun, "wm-supply")
	wm("FUNNEL", "FUNNEL.DAILY_ADDITIVE", funnelRun, "wm-funnel")
	wm("PROMOTIONS", "PROMOTIONS.DEFINITIONS", promoRun, "wm-promo-def")
	wm("PROMOTIONS", "PROMOTIONS.MEMBERSHIP", promoRun, "wm-promo")
	wm("FINANCE", "FINANCE.DAILY_LEDGER", financeRun, "wm-finance")
	wm("ACCOUNT_HEALTH", "CURRENT_SNAPSHOT", healthRun, "wm-health")

	var dqRun int64
	if err := pool.QueryRow(ctx, `INSERT INTO data_quality_runs(
		seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`,
		seller, cut, prompt81Hex("p94-dq-in"), prompt81Hex("p94-dq-res")).Scan(&dqRun); err != nil {
		t.Fatal(err)
	}

	type capSpec struct {
		key, domain string
		runID       int64
	}
	for _, c := range []capSpec{
		{"CATALOG.SKU_UNIVERSE", "CATALOG", catalogRun},
		{"CATALOG.CATEGORY_ATTRIBUTION", "CATALOG", catalogRun},
		{"SALES.SKU_DEMAND", "SALES_DEMAND", salesRun},
		{"SALES.CATEGORY_DEMAND", "SALES_DEMAND", salesRun},
		{"SALES.ACCOUNT_DEMAND", "SALES_DEMAND", salesRun},
		{"STOCK.SKU_CURRENT", "STOCK", stockRun},
		{"STOCK.COVERAGE_7D", "STOCK", stockRun},
		{"ADVERTISING.SKU_TRAFFIC", "ADVERTISING", adsRun},
		{"ADVERTISING.SKU_ATTRIBUTION", "ADVERTISING", adsRun},
		{"ADVERTISING.CAMPAIGN_TRAFFIC", "ADVERTISING", adsRun},
		{"ADVERTISING.CAMPAIGN_ATTRIBUTION", "ADVERTISING", adsRun},
		{"ADVERTISING.CAMPAIGN_DRR", "ADVERTISING", adsRun},
		{"ADVERTISING.ACCOUNT_SPEND", "ADVERTISING", adsRun},
		{"ADVERTISING.CAMPAIGN_CONFIG", "ADVERTISING", adsRun},
		{"PRICE.CUSTOMER_PRICE", "PRICE", priceRun},
		{"SEARCH.SELLER_SUMMARY", "SEARCH", searchRun},
		{"SEARCH.QUERY_DETAILS", "SEARCH", searchRun},
		{"SEARCH.CATEGORY_SUMMARY", "SEARCH", searchRun},
		{"SUPPLY.STATE", "SUPPLY", supplyRun},
		{"SUPPLY.BUNDLE", "SUPPLY", supplyRun},
		{"FUNNEL.ACCOUNT", "FUNNEL", funnelRun},
		{"FUNNEL.SKU", "FUNNEL", funnelRun},
		{"FUNNEL.CATEGORY_ADDITIVE", "FUNNEL", funnelRun},
		{"FINANCE.ACCOUNT_NET", "FINANCE", financeRun},
		{"PROMOTIONS.DEFINITIONS", "PROMOTIONS", promoRun},
		{"PROMOTIONS.MEMBERSHIP", "PROMOTIONS", promoRun},
		{"HEALTH.CURRENT_RATINGS", "ACCOUNT_HEALTH", healthRun},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO business_capability_assessments(
			seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
			contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
			reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
			VALUES($1,$2,$3,'HISTORICAL_QUERY',$4,'READY',
			 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','AVAILABLE','NON_EMPTY',
			 'OK','{}','capability_policy_v3',$5,$6)
			ON CONFLICT DO NOTHING`,
			seller, dqRun, c.key, c.domain, cut.Add(-time.Minute), c.runID); err != nil {
			t.Fatal(err)
		}
	}

	return prompt81Fixture{
		pool: pool, repo: repo, store: store, sealer: universe.Sealer{Repo: repo, Store: store},
		seller: seller, cut: cut, bundle: bundle, domainRun: catalogRun, dqRun: dqRun,
	}
}

func prompt94FactualAPI(t *testing.T, pool *pgxpool.Pool) *factual.Service {
	t.Helper()
	signer, err := factualapi.NewTokenSigner([]byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatal(err)
	}
	return factual.NewService(pool, signer, 200, 1000)
}

func prompt94SealFixture(t *testing.T, f prompt81Fixture) (resolve.Manifest, universe.Binding, factualapi.FactualReadContext) {
	t.Helper()
	ctx := context.Background()
	run := f.evalRun(t)
	resolved, hash := f.resolve(t)
	readCtx, err := universe.ReadContextFromResolved(resolved)
	if err != nil {
		t.Fatal(err)
	}
	api := prompt94FactualAPI(t, f.pool)
	entitySets, err := universe.BuildEntitySets(ctx, api, readCtx)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := f.sealer.Seal(ctx, universe.Input{
		SellerID: f.seller, RunID: run.ID, ResolvedContextHash: hash, Resolved: &resolved, EntitySets: entitySets,
	})
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := universe.NewSealedContext(binding.Record, binding)
	if err != nil {
		t.Fatal(err)
	}
	return resolved, binding, sealed
}
