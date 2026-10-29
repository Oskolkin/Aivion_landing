//go:build integration

package factual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	ready "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type factualFixture struct {
	pool                                                                                *pgxpool.Pool
	seller, other, sku1, sku2, catA, catB, order, posting, ret, campaign, stockSnapshot int64
	cutoff                                                                              time.Time
	ctx                                                                                 factualapi.FactualReadContext
	api                                                                                 *Service
	signer                                                                              factualapi.TokenSigner
	salesRuns                                                                           [2]int64
	salesPayload                                                                        int64
}

func setupFactual(t *testing.T) *factualFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, e := pgxpool.New(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	f := &factualFixture{pool: pool, cutoff: time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)}
	newSeller := func(name string) int64 {
		var user, seller int64
		e := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("factual_%s_%d@test.local", name, time.Now().UnixNano())).Scan(&user)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
		if e = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,$2,'active')RETURNING id`, user, name).Scan(&seller); e != nil {
			t.Fatal(e)
		}
		return seller
	}
	f.seller = newSeller("primary")
	f.other = newSeller("other")
	newRun := func(domain string, at time.Time) (int64, int64) {
		var parent, run int64
		if e = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)VALUES($1,'MANUAL','COMPLETED','factual.integration',$2,$2)RETURNING id`, f.seller, at).Scan(&parent); e != nil {
			t.Fatal(e)
		}
		if e = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,requested_from,requested_to,coverage_from,coverage_to,records_received,records_written,pages_received,completeness_status,source_contract_key,source_contract_version,source_endpoint_family,semantic_mapping_version,started_at,finished_at)VALUES($1,$2,$3,'COMPLETED','2026-08-01','2026-08-24','2026-08-01','2026-08-24',1,1,1,'COMPLETE',$4,'v1',$4,'v1',$5,$5)RETURNING id`, parent, f.seller, domain, "contract."+strings.ToLower(domain), at).Scan(&run); e != nil {
			t.Fatal(e)
		}
		return parent, run
	}
	payload := func(run int64, endpoint string, seq int) int64 {
		var id int64
		if e = pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile)VALUES($1,$2,'OZON',$3,$4,$5,'raw',$6,$7,200,$8,'TEST_REDACTION_V1')RETURNING id`, f.seller, run, endpoint, fmt.Sprintf("fp-%d-%d", run, seq), seq, fmt.Sprintf("factual/%d/%d", run, seq), fmt.Sprintf("%064x", run*100+int64(seq)), f.cutoff.Add(-2*time.Hour)).Scan(&id); e != nil {
			t.Fatal(e)
		}
		return id
	}
	_, taxonomy := newRun("TAXONOMY", f.cutoff.Add(-5*time.Hour))
	var tax int64
	if e = pool.QueryRow(ctx, `INSERT INTO taxonomy_sync_runs(collection_domain_run_id,taxonomy_hash,status,observed_at)VALUES($1,$2,'COMPLETED',$3)RETURNING id`, taxonomy, strings.Repeat("a", 64), f.cutoff.Add(-4*time.Hour)).Scan(&tax); e != nil {
		t.Fatal(e)
	}
	_, catalog := newRun("CATALOG", f.cutoff.Add(-4*time.Hour))
	_, sales1 := newRun("SALES_DEMAND", f.cutoff.Add(-4*time.Hour))
	_, sales2 := newRun("SALES_DEMAND", f.cutoff.Add(-3*time.Hour))
	f.salesRuns = [2]int64{sales1, sales2}
	_, postings := newRun("POSTINGS", f.cutoff.Add(-3*time.Hour))
	_, returns := newRun("RETURNS", f.cutoff.Add(-3*time.Hour))
	_, stockRun := newRun("STOCK", f.cutoff.Add(-3*time.Hour))
	_, priceRun := newRun("PRICE", f.cutoff.Add(-3*time.Hour))
	_, advertisingRun := newRun("ADVERTISING", f.cutoff.Add(-3*time.Hour))
	for domain, run := range map[string]int64{"CATALOG": catalog, "SALES_DEMAND": sales2, "POSTINGS": postings, "RETURNS": returns, "STOCK": stockRun, "PRICE": priceRun, "ADVERTISING": advertisingRun} {
		if _, e = pool.Exec(ctx, `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,$2,$3,$4)`, f.seller, domain, run, f.cutoff.Add(-2*time.Hour)); e != nil {
			t.Fatal(e)
		}
	}
	catPayload := payload(catalog, "catalog", 1)
	_ = catPayload
	salesPayload1 := payload(sales1, "sales", 1)
	f.salesPayload = payload(sales2, "sales", 1)
	postPayload := payload(postings, "postings", 1)
	returnPayload := payload(returns, "returns", 1)
	stockPayload := payload(stockRun, "stock", 1)
	pricePayload := payload(priceRun, "price", 1)
	advertisingPayload := payload(advertisingRun, "advertising", 1)
	categoryBase := time.Now().UnixNano() % 1_000_000_000
	newCategory := func(external int64, name string) int64 {
		external += categoryBase
		var pc, sc int64
		if e = pool.QueryRow(ctx, `INSERT INTO provider_categories(provider,provider_category_id,first_observed_at,last_observed_at)VALUES('OZON',$1,$2,$2)RETURNING id`, external, f.cutoff.Add(-30*24*time.Hour)).Scan(&pc); e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `INSERT INTO provider_category_versions(provider_category_id,name,disabled,is_leaf,taxonomy_state,taxonomy_sync_run_id,observed_at,normalized_hash)VALUES($1,$2,false,true,'ACTIVE',$3,$4,$5)`, pc, name, tax, f.cutoff.Add(-20*24*time.Hour), fmt.Sprintf("%064x", external)); e != nil {
			t.Fatal(e)
		}
		if e = pool.QueryRow(ctx, `INSERT INTO seller_categories(seller_account_id,provider_category_id,first_observed_at,last_observed_at)VALUES($1,$2,$3,$3)RETURNING id`, f.seller, pc, f.cutoff.Add(-20*24*time.Hour)).Scan(&sc); e != nil {
			t.Fatal(e)
		}
		return sc
	}
	f.catA = newCategory(1001, "A")
	f.catB = newCategory(1002, "B")
	newSKU := func(ozon int64, name string, first time.Time) int64 {
		var id int64
		if e = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,$2,$3,$3,'ACTIVE')RETURNING id`, f.seller, ozon, first).Scan(&id); e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, `INSERT INTO canonical_sku_versions(canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)VALUES($1,$2,$3,$4,'active','ACTIVE',$5,$6,$7)`, id, ozon+100, "offer", name, fmt.Sprintf("%064x", ozon), first, catalog); e != nil {
			t.Fatal(e)
		}
		return id
	}
	f.sku1 = newSKU(101, "SKU V1", f.cutoff.Add(-2*time.Hour))
	f.sku2 = newSKU(102, "UNMAPPED", f.cutoff.Add(-2*time.Hour))
	if _, e = pool.Exec(ctx, `INSERT INTO canonical_sku_versions(canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)VALUES($1,201,'offer','SKU V2','active','ACTIVE',$2,$3,$4)`, f.sku1, strings.Repeat("b", 64), f.cutoff.Add(time.Hour), catalog); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO sku_category_assignments(canonical_sku_id,seller_category_id,mapping_status,category_resolution_status,product_type_resolution_status,observed_from,observed_to,collection_domain_run_id)VALUES($1,$2,'MAPPED','RESOLVED','NOT_APPLICABLE','2026-08-01T00:00:00Z','2026-08-18T00:00:00Z',$4),($1,$3,'MAPPED','RESOLVED','NOT_APPLICABLE','2026-08-18T00:00:00Z',NULL,$4)`, f.sku1, f.catA, f.catB, catalog); e != nil {
		t.Fatal(e)
	}
	var inventoryPool int64
	if e = pool.QueryRow(ctx, `INSERT INTO inventory_pools(seller_account_id,provider_pool_key,fulfillment_type,pool_type,independence_status,first_observed_at,last_observed_at)VALUES($1,'FBO:W1','FBO','PROVIDER_PHYSICAL','VERIFIED_INDEPENDENT',$2,$2)RETURNING id`, f.seller, f.cutoff.Add(-2*time.Hour)).Scan(&inventoryPool); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `INSERT INTO stock_snapshots(seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,position_count,batches_expected,batches_completed,batches_failed,completeness_status,semantic_mapping_status,state_hash)VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,1,1,1,1,0,1,1,1,0,'COMPLETE','VALIDATED',$5)RETURNING id`, f.seller, stockRun, f.cutoff.Add(-2*time.Hour), strings.Repeat("1", 64), strings.Repeat("2", 64)).Scan(&f.stockSnapshot); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO stock_snapshot_lines(seller_account_id,snapshot_id,canonical_sku_id,source_sku,sku_resolution_status,inventory_pool_id,fulfillment_type,sellable_units,provider_valid_stock_count,quantity_semantic_status,source_payload_id,observed_at)VALUES($1,$2,$3,101,'RESOLVED',$4,'FBO',30,30,'VALIDATED',$5,$6)`, f.seller, f.stockSnapshot, f.sku1, inventoryPool, stockPayload, f.cutoff.Add(-2*time.Hour)); e != nil {
		t.Fatal(e)
	}
	var priceCollection int64
	if e = pool.QueryRow(ctx, `INSERT INTO price_collection_snapshots(seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,batches_expected,batches_completed,batches_failed,completeness_status,state_hash)VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,1,1,1,1,0,1,1,0,'COMPLETE',$5)RETURNING id`, f.seller, priceRun, f.cutoff.Add(-2*time.Hour), strings.Repeat("3", 64), strings.Repeat("4", 64)).Scan(&priceCollection); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO price_snapshots(seller_account_id,price_collection_snapshot_id,canonical_sku_id,source_sku,provider_product_id,sku_resolution_status,collection_domain_run_id,observed_at,snapshot_at,time_quality,currency_code,provider_current_price,current_price_state,old_reference_price,semantic_mapping_status,state_hash,source_payload_id,reconciliation_status)VALUES($1,$2,$3,101,201,'RESOLVED',$4,$5,$5,'OBSERVED_APPROX','RUB',100,'KNOWN',120,'VALIDATED',$6,$7,'NOT_APPLICABLE')`, f.seller, priceCollection, f.sku1, priceRun, f.cutoff.Add(-2*time.Hour), strings.Repeat("5", 64), pricePayload); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)VALUES($1,77,$2,$2)RETURNING id`, f.seller, f.cutoff.Add(-2*time.Hour)).Scan(&f.campaign); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,1,'Campaign V1','ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$3,$4,$5,$6),($1,$2,2,'Campaign FUTURE','PAUSED','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$7,$8,$5,$6)`, f.seller, f.campaign, strings.Repeat("6", 64), f.cutoff.Add(-2*time.Hour), advertisingPayload, advertisingRun, strings.Repeat("7", 64), f.cutoff.Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO advertising_daily_facts(seller_account_id,advertising_campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,currency_code,traffic_state,attribution_state,normalized_day_hash,source_contract_key,source_payload_id,collection_domain_run_id,observed_at)VALUES($1,$2,'2026-08-24',1,100,10,4,2,8,'PROVIDER_DEFAULT','RUB','USABLE','PENDING_ACCEPTANCE',$3,'ADVERTISING.CAMPAIGN_TRAFFIC',$4,$5,$6)`, f.seller, f.campaign, strings.Repeat("8", 64), advertisingPayload, advertisingRun, f.cutoff.Add(-2*time.Hour)); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 15; i++ {
		d := time.Date(2026, 8, 10+i, 0, 0, 0, 0, time.UTC)
		amount := int64(i + 1)
		if _, e = pool.Exec(ctx, `INSERT INTO sales_demand_daily(seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)VALUES($1,$2,101,$3,$4::numeric,$4::bigint,'RUB','RESOLVED',1,$5,$6,$7,$8)`, f.seller, f.sku1, d, amount, fmt.Sprintf("%064x", 10000+i), sales2, f.salesPayload, f.cutoff.Add(-2*time.Hour)); e != nil {
			t.Fatal(e)
		}
		account := amount
		if i == 14 {
			if _, e = pool.Exec(ctx, `INSERT INTO sales_demand_daily(seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)VALUES($1,$2,102,$3,5,1,'RUB','RESOLVED',1,$4,$5,$6,$7)`, f.seller, f.sku2, d, fmt.Sprintf("%064x", 20000+i), sales2, f.salesPayload, f.cutoff.Add(-2*time.Hour)); e != nil {
				t.Fatal(e)
			}
			account += 5
		}
		if _, e = pool.Exec(ctx, `INSERT INTO sales_account_daily(seller_account_id,business_date,ordered_amount,ordered_units,currency_code,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)VALUES($1,$2,$3::numeric,$3::bigint,'RUB',1,$4,$5,$6,$7)`, f.seller, d, account, fmt.Sprintf("%064x", 30000+i), sales2, f.salesPayload, f.cutoff.Add(-2*time.Hour)); e != nil {
			t.Fatal(e)
		}
		hash := fmt.Sprintf("%064x", 40000+i)
		for j, r := range []int64{sales1, sales2} {
			p := salesPayload1
			if j == 1 {
				p = f.salesPayload
			}
			_ = p
			if _, e = pool.Exec(ctx, `INSERT INTO sales_reconciliations(seller_account_id,business_date,currency_code,provider_account_ordered_amount,provider_account_ordered_units,source_sku_ordered_amount,source_sku_ordered_units,resolved_sku_ordered_amount,resolved_sku_ordered_units,unresolved_ordered_amount,unresolved_ordered_units,source_sku_row_count,resolved_sku_row_count,unresolved_sku_row_count,amount_status,units_status,overall_status,collection_domain_run_id,observed_at,normalized_day_hash)VALUES($1,$2,'RUB',$3::numeric,$3::bigint,$3::numeric,$3::bigint,$3::numeric,$3::bigint,0,0,1,1,0,'PASS','PASS','PASS',$4,$5,$6)`, f.seller, d, account, r, f.cutoff.Add(time.Duration(-4+j)*time.Hour), hash); e != nil {
				t.Fatal(e)
			}
		}
	}
	if e = pool.QueryRow(ctx, `INSERT INTO sales_orders(seller_account_id,ozon_order_id,order_number,first_observed_at,last_observed_at)VALUES($1,501,'ORDER-501',$2,$2)RETURNING id`, f.seller, f.cutoff.Add(-2*time.Hour)).Scan(&f.order); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `INSERT INTO sales_postings(seller_account_id,canonical_order_id,posting_number,fulfillment_scheme,order_resolution_status,first_observed_at,last_observed_at)VALUES($1,$2,'POST-501','FBS','RESOLVED',$3,$3)RETURNING id`, f.seller, f.order, f.cutoff.Add(-2*time.Hour)).Scan(&f.posting); e != nil {
		t.Fatal(e)
	}
	var pv1, pv2 int64
	if e = pool.QueryRow(ctx, `INSERT INTO sales_posting_versions(seller_account_id,canonical_posting_id,version_no,collection_domain_run_id,source_payload_id,source_received_at,observed_at,provider_status,state_hash)VALUES($1,$2,1,$3,$4,$5,$6,'delivering',$7)RETURNING id`, f.seller, f.posting, postings, postPayload, f.cutoff.Add(-90*time.Minute), f.cutoff.Add(-time.Hour), strings.Repeat("c", 64)).Scan(&pv1); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `INSERT INTO sales_posting_versions(seller_account_id,canonical_posting_id,version_no,collection_domain_run_id,source_payload_id,source_received_at,observed_at,provider_status,state_hash)VALUES($1,$2,2,$3,$4,$5,$6,'delivered',$7)RETURNING id`, f.seller, f.posting, postings, postPayload, f.cutoff.Add(30*time.Minute), f.cutoff.Add(time.Hour), strings.Repeat("d", 64)).Scan(&pv2); e != nil {
		t.Fatal(e)
	}
	_ = pv2
	if _, e = pool.Exec(ctx, `INSERT INTO sales_posting_products(seller_account_id,posting_version_id,line_no,canonical_sku_id,source_sku,sku_resolution_status,quantity,unit_price,currency_code,source_payload_id)VALUES($1,$2,1,$3,101,'RESOLVED',2,10,'RUB',$4)`, f.seller, pv1, f.sku1, postPayload); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `INSERT INTO sales_returns(seller_account_id,provider_return_id,first_observed_at,last_observed_at)VALUES($1,701,$2,$2)RETURNING id`, f.seller, f.cutoff.Add(-2*time.Hour)).Scan(&f.ret); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO sales_return_events(sales_return_id,seller_account_id,version_no,canonical_order_id,canonical_posting_id,canonical_sku_id,order_resolution_status,posting_resolution_status,sku_resolution_status,provider_type,provider_schema,return_reason_name,quantity,unit_price,currency_code,provider_visual_status,state_hash,source_received_at,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,1,$3,$4,$5,'RESOLVED','RESOLVED','RESOLVED','provider','FBS','reason',1,5,'RUB','moving',$6,$7,$8,$9,$10)`, f.ret, f.seller, f.order, f.posting, f.sku1, strings.Repeat("e", 64), f.cutoff.Add(-90*time.Minute), f.cutoff.Add(-time.Hour), returnPayload, returns); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO sales_return_events(sales_return_id,seller_account_id,version_no,canonical_order_id,canonical_posting_id,canonical_sku_id,order_resolution_status,posting_resolution_status,sku_resolution_status,provider_type,provider_schema,return_reason_name,quantity,unit_price,currency_code,provider_visual_status,state_hash,source_received_at,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,2,$3,$4,$5,'RESOLVED','RESOLVED','RESOLVED','provider','FBS','reason',1,5,'RUB','received',$6,$7,$8,$9,$10)`, f.ret, f.seller, f.order, f.posting, f.sku1, strings.Repeat("f", 64), f.cutoff.Add(30*time.Minute), f.cutoff.Add(time.Hour), returnPayload, returns); e != nil {
		t.Fatal(e)
	}
	for i, q := range []struct {
		quality   string
		business  any
		effective any
		observed  time.Time
	}{{"PROVIDER_EXACT", time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC), f.cutoff.Add(-time.Hour)}, {"PROVIDER_DATE", time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), nil, f.cutoff.Add(-time.Hour)}, {"UNKNOWN", nil, nil, f.cutoff.Add(-time.Hour)}, {"OBSERVED_APPROX", nil, nil, f.cutoff.Add(-time.Hour)}, {"PROVIDER_EXACT", time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC), f.cutoff.Add(time.Hour)}} {
		if _, e = pool.Exec(ctx, `INSERT INTO sales_lifecycle_events(seller_account_id,canonical_order_id,canonical_posting_id,event_type,effective_at,business_date,time_quality,source_type,source_posting_version_id,source_payload_id,collection_domain_run_id,dedupe_key,observed_at)VALUES($1,$2,$3,'POSTING_DELIVERED',$4,$5,$6,'POSTING_VERSION',$7,$8,$9,$10,$11)`, f.seller, f.order, f.posting, q.effective, q.business, q.quality, pv1, postPayload, postings, fmt.Sprintf("%064x", 50000+i), q.observed); e != nil {
			t.Fatal(e)
		}
	}
	pol := factualapi.V1Policies()
	domains := []factualapi.DomainBinding{}
	for _, b := range []struct {
		domain, family string
		run            int64
	}{
		{"CATALOG", "CATALOG.SKU_UNIVERSE", catalog},
		{"SALES_DEMAND", "SALES.ACCOUNT_DEMAND", sales2},
		{"POSTINGS", "LIFECYCLE.POSTINGS", postings},
		{"RETURNS", "LIFECYCLE.RETURNS", returns},
		{"STOCK", "STOCK.SNAPSHOT", stockRun},
		{"PRICE", "PRICE.SNAPSHOT", priceRun},
		{"ADVERTISING", "ADVERTISING.TRAFFIC", advertisingRun},
	} {
		domains = append(domains, factualapi.DomainBinding{Domain: b.domain, FactFamily: b.family, DomainRunID: b.run, FinalizedAt: factualapi.FinalizedAtPtr(f.cutoff.Add(-2 * time.Hour)), ContractHash: "test"})
	}
	caps := []factualapi.CapabilityBinding{}
	for i, key := range ready.CapabilityKeys() {
		caps = append(caps, factualapi.CapabilityBinding{CapabilityKey: key, Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: int64(i + 1), AssessedAt: f.cutoff.Add(-time.Hour)})
	}
	f.ctx, e = factualapi.NewReadContext(factualapi.ContextSpec{SellerAccountID: f.seller, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), CutoffAt: f.cutoff, Mode: factualapi.ModeLatestStable, Policies: pol, Domains: domains, Capabilities: caps})
	if e != nil {
		t.Fatal(e)
	}
	signer, _ := factualapi.NewTokenSigner([]byte(strings.Repeat("s", 32)))
	f.signer = signer
	f.api = NewService(pool, signer, 200, 600)
	return f
}

func TestFactualAsKnownCategoryLifecyclePaginationBatchTraceAndIsolation(t *testing.T) {
	f := setupFactual(t)
	ctx := context.Background()
	sku, e := f.api.GetSKU(ctx, f.ctx, f.sku1)
	if e != nil || sku.Name != "SKU V1" {
		t.Fatalf("old SKU=%+v err=%v", sku, e)
	}
	skuTrace, e := f.api.TraceFact(ctx, f.ctx, sku.Meta.FactRef)
	if e != nil || len(skuTrace.Sources) != 1 || skuTrace.Sources[0].EndpointKey != "CANONICAL_CATALOG" {
		t.Fatalf("SKU trace=%+v err=%v", skuTrace, e)
	}
	mismatchedRef, e := f.signer.FactRef(f.ctx, factualapi.TraceInput{Kind: "SKU", EntityID: f.sku1, ResultHash: strings.Repeat("0", 64)})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.api.TraceFact(ctx, f.ctx, mismatchedRef); e == nil {
		t.Fatal("mismatched FactRef result hash accepted")
	} else {
		var factualErr *factualapi.Error
		if !errors.As(e, &factualErr) || factualErr.Code != factualapi.CodeFactTraceResultMismatch {
			t.Fatalf("mismatched FactRef error=%v", e)
		}
	}
	day := factualapi.Period{From: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)}
	d, e := f.api.GetAccountDemand(ctx, f.ctx, "", &day)
	if e != nil || d.OrderedAmount.State != factualapi.ValueKnown || d.OrderedAmount.Value.Amount.String() != "20" {
		t.Fatalf("account demand=%+v err=%v", d, e)
	}
	zeroDay := factualapi.Period{From: time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)}
	missing, e := f.api.GetSKUDemand(ctx, f.ctx, f.sku1, "", &zeroDay)
	if e != nil || missing.Demand.OrderedAmount.State == factualapi.ValueKnown {
		t.Fatalf("missing became known zero: %+v", missing)
	}
	prev, e := f.api.ListCategoryDemand(ctx, f.ctx, factualapi.PeriodPrevious7D, nil, factualapi.PageRequest{PageSize: 20})
	if e != nil {
		t.Fatal(e)
	}
	current, e := f.api.ListCategoryDemand(ctx, f.ctx, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: 20})
	if e != nil {
		t.Fatal(e)
	}
	if !hasCategory(prev.Items, f.catA) || hasCategory(prev.Items, f.catB) || !hasCategory(current.Items, f.catB) || !hasUnmapped(current.Items) {
		t.Fatalf("historical category mapping prev=%+v current=%+v", prev.Items, current.Items)
	}
	seenCategory, seenUnmapped, categoryToken := false, false, ""
	for {
		page, pageErr := f.api.ListCategoryDemand(ctx, f.ctx, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: 1, PageToken: categoryToken})
		if pageErr != nil || len(page.Items) > 1 {
			t.Fatalf("category keyset page=%+v err=%v", page, pageErr)
		}
		seenCategory = seenCategory || hasCategory(page.Items, f.catB)
		seenUnmapped = seenUnmapped || hasUnmapped(page.Items)
		if page.NextPageToken == "" {
			break
		}
		categoryToken = page.NextPageToken
	}
	if !seenCategory || !seenUnmapped {
		t.Fatal("category pagination lost a category or UNMAPPED bucket")
	}
	p, e := f.api.GetPosting(ctx, f.ctx, f.posting)
	if e != nil || p.ProviderStatus != "delivering" || len(p.Products) != 1 {
		t.Fatalf("posting=%+v err=%v", p, e)
	}
	r, e := f.api.GetReturn(ctx, f.ctx, f.ret)
	if e != nil || r.VisualStatus != "moving" {
		t.Fatalf("return=%+v err=%v", r, e)
	}
	events, e := f.api.ListLifecycleEvents(ctx, f.ctx, factualapi.LifecycleEventFilter{BusinessPeriod: &day}, factualapi.PageRequest{PageSize: 20})
	if e != nil || len(events.Items) != 2 || !hasEventQuality(events.Items, "PROVIDER_EXACT") || !hasEventQuality(events.Items, "PROVIDER_DATE") {
		t.Fatalf("daily events=%+v err=%v", events, e)
	}
	history, e := f.api.ListLifecycleEvents(ctx, f.ctx, factualapi.LifecycleEventFilter{}, factualapi.PageRequest{PageSize: 20})
	if e != nil || len(history.Items) != 4 {
		t.Fatalf("history events=%d err=%v", len(history.Items), e)
	}
	page1, e := f.api.ListSKUs(ctx, f.ctx, factualapi.SKUFilter{}, factualapi.PageRequest{PageSize: 1})
	if e != nil || len(page1.Items) != 1 || page1.NextPageToken == "" {
		t.Fatalf("page1=%+v err=%v", page1, e)
	}
	var late int64
	e = f.pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,999,$2,$2,'ACTIVE')RETURNING id`, f.seller, f.cutoff.Add(time.Hour)).Scan(&late)
	if e != nil {
		t.Fatal(e)
	}
	page2, e := f.api.ListSKUs(ctx, f.ctx, factualapi.SKUFilter{}, factualapi.PageRequest{PageSize: 1, PageToken: page1.NextPageToken})
	if e != nil || len(page2.Items) != 1 || page2.Items[0].ID == late {
		t.Fatalf("late pagination leak=%+v err=%v", page2, e)
	}
	otherCtx, _ := factualapi.NewReadContext(factualapi.ContextSpec{SellerAccountID: f.other, AnalysisDate: f.ctx.AnalysisDate(), CutoffAt: f.ctx.CutoffAt(), Mode: factualapi.ModeLatestStable, Policies: factualapi.V1Policies(), Domains: []factualapi.DomainBinding{{Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", DomainRunID: 999, FinalizedAt: factualapi.FinalizedAtPtr(f.cutoff.Add(-time.Hour)), ContractHash: "x"}}, Capabilities: []factualapi.CapabilityBinding{{CapabilityKey: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 999, AssessedAt: f.cutoff.Add(-time.Hour)}}})
	if _, e = f.api.GetSKU(ctx, otherCtx, f.sku1); e == nil {
		t.Fatal("cross seller SKU leaked")
	}
	if _, e = f.api.ListSKUs(ctx, otherCtx, factualapi.SKUFilter{}, factualapi.PageRequest{PageToken: page1.NextPageToken}); e == nil {
		t.Fatal("cross context token accepted")
	}
	ids := make([]int64, 500)
	for i := range ids {
		ids[i] = int64(1_000_000 + i)
	}
	ids[0] = f.sku1
	skuBatch, e := f.api.BatchGetSKUs(ctx, f.ctx, []int64{f.sku1, 9_999_999})
	if e != nil || len(skuBatch.Items) != 2 || skuBatch.Items[0].Status != factualapi.BatchFound || skuBatch.Items[1].Status != factualapi.BatchNotFound {
		t.Fatalf("mixed SKU batch=%+v err=%v", skuBatch, e)
	}
	batch, e := f.api.BatchGetSKUDemand(ctx, f.ctx, ids, factualapi.PeriodAnalysisDay, nil)
	if e != nil || len(batch.Items) != 500 || batch.Items[0].Status == factualapi.BatchNotFound {
		t.Fatalf("batch len=%d first=%+v err=%v", len(batch.Items), batch.Items[0], e)
	}
	trace, e := f.api.TraceFact(ctx, f.ctx, d.Meta.FactRef)
	if e != nil || len(trace.Sources) == 0 {
		t.Fatalf("trace=%+v err=%v", trace, e)
	}
	category, e := f.api.GetCategoryDemand(ctx, f.ctx, &f.catB, factualapi.PeriodCurrent7D, nil)
	if e != nil {
		t.Fatal(e)
	}
	categoryTrace, e := f.api.TraceFact(ctx, f.ctx, category.Demand.Meta.FactRef)
	if e != nil || !hasTraceRole(categoryTrace.Sources, "HISTORICAL_CATEGORY_ASSIGNMENT") {
		t.Fatalf("category trace missing historical assignment: %+v err=%v", categoryTrace, e)
	}
	if _, e = f.pool.Exec(ctx, `INSERT INTO sales_account_daily(seller_account_id,business_date,ordered_amount,ordered_units,currency_code,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)VALUES($1,'2026-08-24',999,999,'RUB',2,$2,$3,$4,$5)`, f.seller, strings.Repeat("9", 64), f.salesRuns[1], f.salesPayload, f.cutoff.Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	again, _ := f.api.GetAccountDemand(ctx, f.ctx, "", &day)
	if again.OrderedAmount.Value.Amount.String() != "20" {
		t.Fatalf("late revision leaked: %+v", again)
	}
}

func TestPrompt6FactualCutoffTraceAndCrossDomainFormulas(t *testing.T) {
	f := setupFactual(t)
	ctx := context.Background()
	stockFact, err := f.api.GetSKUStock(ctx, f.ctx, f.sku1)
	if err != nil || stockFact.SellableUnits.Value == nil || *stockFact.SellableUnits.Value != 30 {
		t.Fatalf("stock=%+v err=%v", stockFact, err)
	}
	if trace, e := f.api.TraceFact(ctx, f.ctx, stockFact.Meta.FactRef); e != nil || len(trace.Sources) != 1 {
		t.Fatalf("stock trace=%+v err=%v", trace, e)
	}
	coverage, err := f.api.GetStockCoverage(ctx, f.ctx, f.sku1)
	if err != nil || coverage.CoverageDays7D.Value == nil || coverage.CoverageDays7D.Value.String() != "2.5" {
		t.Fatalf("coverage=%+v err=%v", coverage, err)
	}

	priceFact, err := f.api.GetSKUPrice(ctx, f.ctx, f.sku1)
	if err != nil || priceFact.ProviderCurrentPrice.Value == nil || priceFact.ProviderCurrentPrice.Value.Amount.String() != "100" {
		t.Fatalf("price=%+v err=%v", priceFact, err)
	}
	if trace, e := f.api.TraceFact(ctx, f.ctx, priceFact.Meta.FactRef); e != nil || len(trace.Sources) != 1 {
		t.Fatalf("price trace=%+v err=%v", trace, e)
	}

	campaign, err := f.api.GetCampaign(ctx, f.ctx, f.campaign)
	if err != nil || campaign.Title != "Campaign V1" || campaign.ProviderState != "ACTIVE" {
		t.Fatalf("campaign cutoff leak=%+v err=%v", campaign, err)
	}
	period := factualapi.Period{From: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)}
	drr, err := f.api.GetCampaignDRR(ctx, f.ctx, f.campaign, "", &period)
	if err != nil || drr.CalculatedDRRPct.Value == nil || drr.CalculatedDRRPct.Value.String() != "50" {
		t.Fatalf("drr=%+v err=%v", drr, err)
	}
	share, err := f.api.GetAdvertisingSpendShare(ctx, f.ctx, "", &period)
	if err != nil || share.SpendSharePct.Value == nil || share.SpendSharePct.Value.String() != "20" {
		t.Fatalf("spend share=%+v err=%v", share, err)
	}
	if trace, e := f.api.TraceFact(ctx, f.ctx, share.Meta.FactRef); e != nil || len(trace.Inputs) != 2 {
		t.Fatalf("share trace=%+v err=%v", trace, e)
	}

	// Unified AS-KNOWN-AT future-leak: later stock/price/ad corrections must not alter Context C.
	stockBefore := stockFact.Meta.FactRef
	priceBefore := priceFact.Meta.FactRef
	campaignBefore := campaign.Title
	var lateCollectionRun, lateStockRun, latePriceRun, lateAdRun, lateStockPayload, latePricePayload, lateAdPayload int64
	if e := f.pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','COMPLETED','prompt6.future')RETURNING id`, f.seller).Scan(&lateCollectionRun); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at,finished_at)VALUES($1,$2,'STOCK','COMPLETED',$3,$3)RETURNING id`, lateCollectionRun, f.seller, f.cutoff.Add(time.Hour)).Scan(&lateStockRun); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile)VALUES($1,$2,'OZON','future.stock','fp-future-stock',1,'raw','future/stock',$3,200,$4,'TEST_REDACTION_V1')RETURNING id`, f.seller, lateStockRun, strings.Repeat("a", 64), f.cutoff.Add(time.Hour)).Scan(&lateStockPayload); e != nil {
		t.Fatal(e)
	}
	var lateSnapshot, latePool int64
	if e := f.pool.QueryRow(ctx, `INSERT INTO inventory_pools(seller_account_id,provider_pool_key,fulfillment_type,pool_type,independence_status,first_observed_at,last_observed_at)VALUES($1,'FBO:W-LATE','FBO','PROVIDER_PHYSICAL','VERIFIED_INDEPENDENT',$2,$2)RETURNING id`, f.seller, f.cutoff.Add(time.Hour)).Scan(&latePool); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO stock_snapshots(seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,position_count,batches_expected,batches_completed,batches_failed,completeness_status,semantic_mapping_status,state_hash)VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,1,1,1,1,0,1,1,1,0,'COMPLETE','VALIDATED',$5)RETURNING id`, f.seller, lateStockRun, f.cutoff.Add(time.Hour), strings.Repeat("b", 64), strings.Repeat("c", 64)).Scan(&lateSnapshot); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, `INSERT INTO stock_snapshot_lines(seller_account_id,snapshot_id,canonical_sku_id,source_sku,sku_resolution_status,inventory_pool_id,fulfillment_type,sellable_units,provider_valid_stock_count,quantity_semantic_status,source_payload_id,observed_at)VALUES($1,$2,$3,101,'RESOLVED',$4,'FBO',999,999,'VALIDATED',$5,$6)`, f.seller, lateSnapshot, f.sku1, latePool, lateStockPayload, f.cutoff.Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at,finished_at)VALUES($1,$2,'PRICE','COMPLETED',$3,$3)RETURNING id`, lateCollectionRun, f.seller, f.cutoff.Add(time.Hour)).Scan(&latePriceRun); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile)VALUES($1,$2,'OZON','future.price','fp-future-price',1,'raw','future/price',$3,200,$4,'TEST_REDACTION_V1')RETURNING id`, f.seller, latePriceRun, strings.Repeat("9", 64), f.cutoff.Add(time.Hour)).Scan(&latePricePayload); e != nil {
		t.Fatal(e)
	}
	var latePriceCollection int64
	if e := f.pool.QueryRow(ctx, `INSERT INTO price_collection_snapshots(seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,batches_expected,batches_completed,batches_failed,completeness_status,state_hash)VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,1,1,1,1,0,1,1,0,'COMPLETE',$5)RETURNING id`, f.seller, latePriceRun, f.cutoff.Add(time.Hour), strings.Repeat("d", 64), strings.Repeat("e", 64)).Scan(&latePriceCollection); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, `INSERT INTO price_snapshots(seller_account_id,price_collection_snapshot_id,canonical_sku_id,source_sku,provider_product_id,sku_resolution_status,collection_domain_run_id,observed_at,snapshot_at,time_quality,currency_code,provider_current_price,current_price_state,old_reference_price,semantic_mapping_status,state_hash,source_payload_id,reconciliation_status)VALUES($1,$2,$3,101,201,'RESOLVED',$4,$5,$5,'OBSERVED_APPROX','RUB',777,'KNOWN',120,'VALIDATED',$6,$7,'NOT_APPLICABLE')`, f.seller, latePriceCollection, f.sku1, latePriceRun, f.cutoff.Add(time.Hour), strings.Repeat("f", 64), latePricePayload); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at,finished_at)VALUES($1,$2,'ADVERTISING','COMPLETED',$3,$3)RETURNING id`, lateCollectionRun, f.seller, f.cutoff.Add(time.Hour)).Scan(&lateAdRun); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile)VALUES($1,$2,'OZON','future.ad','fp-future-ad',1,'raw','future/ad',$3,200,$4,'TEST_REDACTION_V1')RETURNING id`, f.seller, lateAdRun, strings.Repeat("0", 64), f.cutoff.Add(time.Hour)).Scan(&lateAdPayload); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, `INSERT INTO advertising_daily_facts(seller_account_id,advertising_campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,currency_code,traffic_state,attribution_state,normalized_day_hash,source_contract_key,source_payload_id,collection_domain_run_id,observed_at)VALUES($1,$2,'2026-08-24',2,999,99,400,20,800,'PROVIDER_DEFAULT','RUB','USABLE','PENDING_ACCEPTANCE',$3,'ADVERTISING.CAMPAIGN_TRAFFIC',$4,$5,$6)`, f.seller, f.campaign, strings.Repeat("1", 64), lateAdPayload, lateAdRun, f.cutoff.Add(time.Hour)); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,state_hash,observed_at,source_payload_id,collection_domain_run_id)VALUES($1,$2,3,'Campaign AFTER CUTOFF','ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$3,$4,$5,$6)`, f.seller, f.campaign, strings.Repeat("2", 64), f.cutoff.Add(2*time.Hour), lateAdPayload, lateAdRun); e != nil {
		t.Fatal(e)
	}

	stockAfter, err := f.api.GetSKUStock(ctx, f.ctx, f.sku1)
	if err != nil || stockAfter.SellableUnits.Value == nil || *stockAfter.SellableUnits.Value != 30 || stockAfter.Meta.FactRef != stockBefore {
		t.Fatalf("stock future leak=%+v err=%v", stockAfter, err)
	}
	priceAfter, err := f.api.GetSKUPrice(ctx, f.ctx, f.sku1)
	if err != nil || priceAfter.ProviderCurrentPrice.Value == nil || priceAfter.ProviderCurrentPrice.Value.Amount.String() != "100" || priceAfter.Meta.FactRef != priceBefore {
		t.Fatalf("price future leak=%+v err=%v", priceAfter, err)
	}
	campaignAfter, err := f.api.GetCampaign(ctx, f.ctx, f.campaign)
	if err != nil || campaignAfter.Title != campaignBefore {
		t.Fatalf("campaign future leak=%+v err=%v", campaignAfter, err)
	}
	drrAfter, err := f.api.GetCampaignDRR(ctx, f.ctx, f.campaign, "", &period)
	if err != nil || drrAfter.CalculatedDRRPct.Value == nil || drrAfter.CalculatedDRRPct.Value.String() != "50" {
		t.Fatalf("ad daily future leak=%+v err=%v", drrAfter, err)
	}
}
func hasCategory(v []factualapi.CategoryDemand, id int64) bool {
	for _, x := range v {
		if x.CategoryID != nil && *x.CategoryID == id {
			return true
		}
	}
	return false
}
func hasUnmapped(v []factualapi.CategoryDemand) bool {
	for _, x := range v {
		if x.CategoryID == nil && x.Bucket == "UNMAPPED" {
			return true
		}
	}
	return false
}
func hasTraceRole(v []factualapi.TraceSource, role string) bool {
	for _, x := range v {
		if x.SemanticRole == role {
			return true
		}
	}
	return false
}
func hasEventQuality(v []factualapi.LifecycleEvent, quality string) bool {
	for _, x := range v {
		if x.TimeQuality == quality {
			return true
		}
	}
	return false
}

type countClock struct {
	at time.Time
	n  int
}

func (c *countClock) Now() time.Time { c.n++; return c.at }
func TestReadinessSelects14DayMatureWindowAndCapturesClockOnce(t *testing.T) {
	f := setupFactual(t)
	var failedParent int64
	if e := f.pool.QueryRow(context.Background(), `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)VALUES($1,'MANUAL','FAILED','factual.current.failure',$2,$2)RETURNING id`, f.seller, f.cutoff.Add(-30*time.Minute)).Scan(&failedParent); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(context.Background(), `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,requested_from,requested_to,records_received,records_written,pages_received,completeness_status,source_contract_key,source_contract_version,started_at,finished_at)VALUES($1,$2,'SALES_DEMAND','FAILED','2026-08-24','2026-08-24',0,0,0,'PARTIAL','contract.sales','v1',$3,$3)`, failedParent, f.seller, f.cutoff.Add(-30*time.Minute)); e != nil {
		t.Fatal(e)
	}
	clock := &countClock{at: f.cutoff}
	svc := ready.NewService(f.pool, clock, ready.StaticSemanticRegistry{})
	before := countRows(t, f.pool, `SELECT count(*) FROM sales_demand_daily WHERE seller_account_id=$1`, f.seller)
	actionsBefore := countRows(t, f.pool, `SELECT count(*) FROM action_items WHERE seller_account_id=$1`, f.seller)
	r, e := svc.Evaluate(context.Background(), f.seller)
	if e != nil {
		t.Fatal(e)
	}
	if clock.n != 1 {
		t.Fatalf("clock calls=%d", clock.n)
	}
	if r.AnalysisDate == nil || r.AnalysisDate.Format("2006-01-02") != "2026-08-24" || r.Context == nil {
		t.Fatalf("readiness=%+v", r.Candidates)
	}
	if capabilityState(r.Capabilities, "SALES.ACCOUNT_DEMAND", "CURRENT_COLLECTION") != "FAILED" || capabilityState(r.Capabilities, "SALES.ACCOUNT_DEMAND", "HISTORICAL_QUERY") != "READY" {
		t.Fatalf("current failure leaked into historical capability: %+v", r.Capabilities)
	}
	if before != countRows(t, f.pool, `SELECT count(*) FROM sales_demand_daily WHERE seller_account_id=$1`, f.seller) || actionsBefore != countRows(t, f.pool, `SELECT count(*) FROM action_items WHERE seller_account_id=$1`, f.seller) {
		t.Fatal("readiness mutated business facts/actions")
	}
	var hash string
	if e = f.pool.QueryRow(context.Background(), `SELECT result_hash FROM data_quality_runs WHERE id=$1`, r.DQRunID).Scan(&hash); e != nil || hash != r.ResultHash {
		t.Fatalf("DQ hash=%s err=%v", hash, e)
	}
	var salesSnapshot, catalogSnapshot []byte
	if e = f.pool.QueryRow(context.Background(), `SELECT sales_watermark_snapshot_json,catalog_watermark_snapshot_json FROM analysis_readiness_evaluations WHERE id=$1`, r.Candidates[len(r.Candidates)-1].EvaluationID).Scan(&salesSnapshot, &catalogSnapshot); e != nil || string(salesSnapshot) == "{}" || string(catalogSnapshot) == "{}" {
		t.Fatalf("readiness watermark snapshots sales=%s catalog=%s err=%v", salesSnapshot, catalogSnapshot, e)
	}
	repeated, e := svc.Evaluate(context.Background(), f.seller)
	if e != nil || repeated.DQRunID != r.DQRunID || repeated.ResultHash != r.ResultHash {
		t.Fatalf("same inputs were not idempotent: first=%d/%s repeat=%d/%s err=%v", r.DQRunID, r.ResultHash, repeated.DQRunID, repeated.ResultHash, e)
	}
	beforeFrozen, e := f.api.GetAccountDemand(context.Background(), *r.Context, factualapi.PeriodCurrent7D, nil)
	if e != nil {
		t.Fatal(e)
	}
	var newerDQ int64
	newerAt := f.cutoff.Add(30 * time.Second)
	if e = f.pool.QueryRow(context.Background(), `INSERT INTO data_quality_runs(seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)VALUES($1,$2,$3,$4,$5,'COMPLETED',$2,$2)RETURNING id`, f.seller, newerAt, factualapi.DQPolicyV1, strings.Repeat("6", 64), strings.Repeat("7", 64)).Scan(&newerDQ); e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(context.Background(), `INSERT INTO business_capability_assessments(seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,reason_code,policy_version,assessed_at)VALUES($1,$2,'SALES.ACCOUNT_DEMAND','HISTORICAL_QUERY','SALES_DEMAND','FAILED','ACTIVE','ACCESSIBLE','FAILED','VALIDATED','FULL','CURRENT','AVAILABLE','NON_EMPTY','NEWER_FAILURE',$3,$4)`, f.seller, newerDQ, factualapi.CapabilityPolicyV1, newerAt); e != nil {
		t.Fatal(e)
	}
	afterFrozen, e := f.api.GetAccountDemand(context.Background(), *r.Context, factualapi.PeriodCurrent7D, nil)
	beforeJSON, _ := json.Marshal(beforeFrozen)
	afterJSON, _ := json.Marshal(afterFrozen)
	if e != nil || string(beforeJSON) != string(afterJSON) {
		t.Fatalf("newer capability assessment changed frozen context: before=%s after=%s err=%v", beforeJSON, afterJSON, e)
	}
	var correctionParent, correctionRun int64
	if e = f.pool.QueryRow(context.Background(), `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)VALUES($1,'MANUAL','COMPLETED','factual.correction',$2,$2)RETURNING id`, f.seller, f.cutoff.Add(time.Minute)).Scan(&correctionParent); e != nil {
		t.Fatal(e)
	}
	if e = f.pool.QueryRow(context.Background(), `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,requested_from,requested_to,coverage_from,coverage_to,records_received,records_written,pages_received,completeness_status,source_contract_key,source_contract_version,started_at,finished_at)VALUES($1,$2,'SALES_DEMAND','COMPLETED','2026-08-24','2026-08-24','2026-08-24','2026-08-24',1,1,1,'COMPLETE','contract.sales','v1',$3,$3)RETURNING id`, correctionParent, f.seller, f.cutoff.Add(time.Minute)).Scan(&correctionRun); e != nil {
		t.Fatal(e)
	}
	if _, e = f.pool.Exec(context.Background(), `INSERT INTO sales_reconciliations(seller_account_id,business_date,currency_code,provider_account_ordered_amount,provider_account_ordered_units,source_sku_ordered_amount,source_sku_ordered_units,resolved_sku_ordered_amount,resolved_sku_ordered_units,unresolved_ordered_amount,unresolved_ordered_units,source_sku_row_count,resolved_sku_row_count,unresolved_sku_row_count,amount_status,units_status,overall_status,collection_domain_run_id,observed_at,normalized_day_hash)VALUES($1,'2026-08-24','RUB',20,16,20,16,20,16,0,0,2,2,0,'PASS','PASS','PASS',$2,$3,$4)`, f.seller, correctionRun, f.cutoff.Add(time.Minute), strings.Repeat("8", 64)); e != nil {
		t.Fatal(e)
	}
	clock.at = f.cutoff.Add(2 * time.Minute)
	r2, e := svc.Evaluate(context.Background(), f.seller)
	if e != nil {
		t.Fatal(e)
	}
	if r2.AnalysisDate == nil || r2.AnalysisDate.Format("2006-01-02") != "2026-08-23" {
		t.Fatalf("resolver did not fall back to newest mature window: %+v", r2.Candidates)
	}
	for _, w := range r2.Watermarks {
		if w.FactFamily == "SALES.ACCOUNT_DEMAND" && w.LatestMatureBusinessDate != nil && w.LatestMatureBusinessDate.Format("2006-01-02") == "2026-08-24" {
			t.Fatal("correction did not reset mature watermark")
		}
	}
}
func TestReadinessEmptyInsufficientAndInnerGap(t *testing.T) {
	t.Run("empty seller is not a failure", func(t *testing.T) {
		f := setupFactual(t)
		var parent, catalog int64
		if e := f.pool.QueryRow(context.Background(), `INSERT INTO collection_runs(seller_account_id,kind,status,trigger,started_at,finished_at)VALUES($1,'MANUAL','COMPLETED','factual.empty',$2,$2)RETURNING id`, f.other, f.cutoff.Add(-time.Hour)).Scan(&parent); e != nil {
			t.Fatal(e)
		}
		if e := f.pool.QueryRow(context.Background(), `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,requested_from,requested_to,coverage_from,coverage_to,records_received,records_written,pages_received,completeness_status,source_contract_key,source_contract_version,started_at,finished_at)VALUES($1,$2,'CATALOG','COMPLETED','2026-08-24','2026-08-24','2026-08-24','2026-08-24',0,0,1,'COMPLETE','contract.catalog','v1',$3,$3)RETURNING id`, parent, f.other, f.cutoff.Add(-time.Hour)).Scan(&catalog); e != nil {
			t.Fatal(e)
		}
		if _, e := f.pool.Exec(context.Background(), `INSERT INTO collection_authoritative_heads(seller_account_id,domain,collection_domain_run_id,finalized_at)VALUES($1,'CATALOG',$2,$3)`, f.other, catalog, f.cutoff.Add(-time.Hour)); e != nil {
			t.Fatal(e)
		}
		r, e := ready.NewService(f.pool, &countClock{at: f.cutoff}, ready.StaticSemanticRegistry{}).Evaluate(context.Background(), f.other)
		if e != nil || r.Context != nil || len(r.Candidates) != 1 || r.Candidates[0].Status != ready.GateEmpty {
			t.Fatalf("empty readiness=%+v err=%v", r, e)
		}
		if capabilityState(r.Capabilities, "CATALOG.SKU_UNIVERSE", "HISTORICAL_QUERY") != "READY" {
			t.Fatalf("successful empty catalog must remain READY with empty population: %+v", r.Capabilities)
		}
	})
	t.Run("thirteen days still selects candidate day", func(t *testing.T) {
		f := setupFactual(t)
		for _, table := range []string{"sales_reconciliations", "sales_demand_daily", "sales_account_daily"} {
			if _, e := f.pool.Exec(context.Background(), `DELETE FROM `+table+` WHERE seller_account_id=$1 AND business_date IN('2026-08-10','2026-08-11')`, f.seller); e != nil {
				t.Fatal(e)
			}
		}
		r, e := ready.NewService(f.pool, &countClock{at: f.cutoff}, ready.StaticSemanticRegistry{}).Evaluate(context.Background(), f.seller)
		if e != nil || r.AnalysisDate == nil || r.AnalysisDate.Format("2006-01-02") != "2026-08-24" {
			t.Fatalf("13-day history must still select analysis_date: %+v err=%v", r.Candidates, e)
		}
		if capabilityState(r.Capabilities, "SALES.ACCOUNT_DEMAND", "HISTORICAL_QUERY") != "PARTIAL" {
			t.Fatalf("13-day sales capability must stay PARTIAL: %+v", r.Capabilities)
		}
	})
	t.Run("inner gap does not invalidate ready candidate day", func(t *testing.T) {
		f := setupFactual(t)
		if _, e := f.pool.Exec(context.Background(), `DELETE FROM sales_account_daily WHERE seller_account_id=$1 AND business_date='2026-08-17'`, f.seller); e != nil {
			t.Fatal(e)
		}
		r, e := ready.NewService(f.pool, &countClock{at: f.cutoff}, ready.StaticSemanticRegistry{}).Evaluate(context.Background(), f.seller)
		if e != nil || r.AnalysisDate == nil || r.AnalysisDate.Format("2006-01-02") != "2026-08-24" {
			t.Fatalf("inner gap blocked ready D: %+v err=%v", r.Candidates, e)
		}
		for _, w := range r.Watermarks {
			if w.FactFamily == "SALES.ACCOUNT_DEMAND" && (w.LatestCompleteBusinessDate == nil || w.LatestCompleteBusinessDate.Format("2006-01-02") != "2026-08-16") {
				t.Fatalf("gap watermark=%+v", w)
			}
		}
	})
}
func TestPrompt98AccountCategoryPeriodCompleteness(t *testing.T) {
	f := setupFactual(t)
	ctx := context.Background()
	incomplete := factualapi.Period{
		From: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC),
	}
	acct, e := f.api.GetAccountDemand(ctx, f.ctx, "", &incomplete)
	if e != nil {
		t.Fatal(e)
	}
	if acct.OrderedAmount.State != factualapi.ValueUnknown || acct.OrderedAmount.ReasonCode != "MISSING_DAILY_FACT" {
		t.Fatalf("account 4/7 must be UNKNOWN MISSING_DAILY_FACT: %+v", acct.OrderedAmount)
	}
	cat, e := f.api.GetCategoryDemand(ctx, f.ctx, &f.catA, "", &incomplete)
	if e != nil {
		t.Fatal(e)
	}
	if cat.Demand.OrderedAmount.State == factualapi.ValueKnown {
		t.Fatalf("category summed incomplete source period as known: %+v", cat.Demand.OrderedAmount)
	}
	if cat.Demand.OrderedAmount.ReasonCode != "MISSING_DAILY_FACT" {
		t.Fatalf("category incomplete reason=%q", cat.Demand.OrderedAmount.ReasonCode)
	}

	complete := factualapi.Period{
		From: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
	}
	acct7, e := f.api.GetAccountDemand(ctx, f.ctx, "", &complete)
	if e != nil || acct7.OrderedAmount.State != factualapi.ValueKnown {
		t.Fatalf("account 7/7 must be KNOWN: %+v err=%v", acct7.OrderedAmount, e)
	}
	early := factualapi.Period{
		From: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC),
	}
	zeroCat, e := f.api.GetCategoryDemand(ctx, f.ctx, &f.catB, "", &early)
	if e != nil {
		t.Fatal(e)
	}
	if zeroCat.Demand.OrderedAmount.State != factualapi.ValueKnown || zeroCat.Demand.OrderedAmount.Value == nil || zeroCat.Demand.OrderedAmount.Value.Amount.String() != "0" {
		t.Fatalf("full source period + no category rows must be known zero: %+v", zeroCat.Demand.OrderedAmount)
	}
}

func capabilityState(v []ready.Capability, key, scope string) string {
	for _, x := range v {
		if x.Key == key && x.Scope == scope {
			return x.FinalState
		}
	}
	return ""
}
func countRows(t *testing.T, p *pgxpool.Pool, q string, args ...any) int {
	t.Helper()
	var n int
	if e := p.QueryRow(context.Background(), q, args...).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

var _ = errors.Is
