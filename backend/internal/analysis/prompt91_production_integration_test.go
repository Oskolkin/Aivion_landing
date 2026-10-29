//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/hibiken/asynq"
)

// prompt91Seed inserts enough canonical facts for one seller that the production
// Factual API → Landscape Core → Materiality → Detail path yields non-empty lanes.
// It reuses prompt81Fixture helpers (pool/repo/evalRun) from the same package.
func prompt91Seed(t *testing.T) prompt81Fixture {
	t.Helper()
	return prompt91SeedForSeller(t, 0)
}

func prompt91SeedForSeller(t *testing.T, seller int64) prompt81Fixture {
	t.Helper()
	_ = os.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	pool := prompt81Pool(t)
	ctx := context.Background()
	if seller <= 0 {
		seller = prompt81Seller(t, pool)
	} else {
		var ok bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seller_accounts WHERE id=$1)`, seller).Scan(&ok); err != nil || !ok {
			t.Fatalf("seller %d not found", seller)
		}
	}
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	analysisDay := cut.AddDate(0, 0, -1) // 2026-08-24
	obs := cut.Add(-2 * time.Hour)
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
			VALUES($1,'MANUAL','COMPLETED','prompt91',$2,$2)RETURNING id`, seller, at).Scan(&parent); err != nil {
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
			seller, run, endpoint, fmt.Sprintf("p91-fp-%d-%d", run, seq), seq,
			fmt.Sprintf("prompt91/%d/%d", run, seq), fmt.Sprintf("%064x", run*1000+int64(seq)), obs).Scan(&id); err != nil {
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

	for domain, run := range map[string]int64{
		"CATALOG": catalogRun, "SALES_DEMAND": salesRun, "STOCK": stockRun,
		"PRICE": priceRun, "ADVERTISING": adsRun, "SEARCH": searchRun, "SUPPLY": supplyRun,
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
	supplyPayload := payload(supplyRun, "supply", 1)
	_ = catalogPayload

	var taxSync int64
	if err := pool.QueryRow(ctx, `INSERT INTO taxonomy_sync_runs(collection_domain_run_id,taxonomy_hash,status,observed_at)
		VALUES($1,$2,'COMPLETED',$3)RETURNING id`, taxonomyRun, strings.Repeat("a", 64), cut.Add(-4*time.Hour)).Scan(&taxSync); err != nil {
		t.Fatal(err)
	}

	catBase := time.Now().UnixNano() % 1_000_000_000
	newLeaf := func(ext int64, name string) int64 {
		t.Helper()
		ext += catBase
		var pc, sc int64
		if err := pool.QueryRow(ctx, `INSERT INTO provider_categories(provider,provider_category_id,first_observed_at,last_observed_at)
			VALUES('OZON',$1,$2,$2)RETURNING id`, ext, cut.Add(-40*24*time.Hour)).Scan(&pc); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO provider_category_versions(
			provider_category_id,name,disabled,is_leaf,taxonomy_state,taxonomy_sync_run_id,observed_at,normalized_hash)
			VALUES($1,$2,false,true,'ACTIVE',$3,$4,$5)`,
			pc, name, taxSync, cut.Add(-30*24*time.Hour), fmt.Sprintf("%064x", ext)); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO seller_categories(seller_account_id,provider_category_id,first_observed_at,last_observed_at)
			VALUES($1,$2,$3,$3)RETURNING id`, seller, pc, cut.Add(-30*24*time.Hour)).Scan(&sc); err != nil {
			t.Fatal(err)
		}
		return sc
	}
	catA := newLeaf(5001, "LeafA")
	catB := newLeaf(5002, "LeafB")

	type skuSpec struct {
		ozon       int64
		name       string
		cat        *int64
		unmapped   bool
		curAmt     int64 // current-window daily amount (applied to each current day)
		prevAmt    int64
		stockUnits int64
		zeroStock  bool
	}
	specs := []skuSpec{
		{ozon: 9101, name: "SKU-A1", cat: &catA, curAmt: 100, prevAmt: 40, stockUnits: 50},
		{ozon: 9102, name: "SKU-A2", cat: &catA, curAmt: 80, prevAmt: 90, stockUnits: 20},
		{ozon: 9103, name: "SKU-A3", cat: &catA, curAmt: 60, prevAmt: 0, stockUnits: 10}, // from zero
		{ozon: 9104, name: "SKU-A4", cat: &catA, curAmt: 0, prevAmt: 70, stockUnits: 5},  // to zero
		{ozon: 9105, name: "SKU-B1", cat: &catB, curAmt: 120, prevAmt: 50, stockUnits: 0, zeroStock: true},
		{ozon: 9106, name: "SKU-B2", cat: &catB, curAmt: 55, prevAmt: 55, stockUnits: 15},
		{ozon: 9107, name: "SKU-B3", cat: &catB, curAmt: 45, prevAmt: 30, stockUnits: 8},
		{ozon: 9108, name: "SKU-B4", cat: &catB, curAmt: 35, prevAmt: 20, stockUnits: 12},
		{ozon: 9109, name: "SKU-B5", cat: &catB, curAmt: 25, prevAmt: 15, stockUnits: 6},
		{ozon: 9110, name: "SKU-U1", unmapped: true, curAmt: 40, prevAmt: 10, stockUnits: 4},
		{ozon: 9111, name: "SKU-A5", cat: &catA, curAmt: 15, prevAmt: 5, stockUnits: 3},
		{ozon: 9112, name: "SKU-B6", cat: &catB, curAmt: 10, prevAmt: 8, stockUnits: 2},
	}

	skuIDs := make([]int64, len(specs))
	for i, sp := range specs {
		var id int64
		if err := pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)
			VALUES($1,$2,$3,$3,'ACTIVE')RETURNING id`, seller, sp.ozon, cut.Add(-10*24*time.Hour)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		skuIDs[i] = id
		if _, err := pool.Exec(ctx, `INSERT INTO canonical_sku_versions(
			canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)
			VALUES($1,$2,$3,$4,'active','ACTIVE',$5,$6,$7)`,
			id, sp.ozon+100, fmt.Sprintf("offer-%d", sp.ozon), sp.name, fmt.Sprintf("%064x", sp.ozon), obs, catalogRun); err != nil {
			t.Fatal(err)
		}
		if sp.unmapped {
			if _, err := pool.Exec(ctx, `INSERT INTO sku_category_assignments(
				canonical_sku_id,seller_category_id,mapping_status,category_resolution_status,product_type_resolution_status,
				observed_from,observed_to,collection_domain_run_id)
				VALUES($1,NULL,'UNMAPPED','UNRESOLVED','NOT_APPLICABLE',$2,NULL,$3)`,
				id, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), catalogRun); err != nil {
				t.Fatal(err)
			}
		} else if sp.cat != nil {
			if _, err := pool.Exec(ctx, `INSERT INTO sku_category_assignments(
				canonical_sku_id,seller_category_id,mapping_status,category_resolution_status,product_type_resolution_status,
				observed_from,observed_to,collection_domain_run_id)
				VALUES($1,$2,'MAPPED','RESOLVED','NOT_APPLICABLE',$3,NULL,$4)`,
				id, *sp.cat, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), catalogRun); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Sales: previous7 (Aug 11–17), current7 (Aug 18–24), plus sparse 30d history.
	accountByDay := map[string]int64{}
	for i, sp := range specs {
		id := skuIDs[i]
		for dayOffset := 0; dayOffset < 30; dayOffset++ {
			d := analysisDay.AddDate(0, 0, -29+dayOffset)
			key := d.Format("2006-01-02")
			var amt int64
			switch {
			case !d.Before(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)) && !d.After(analysisDay):
				amt = sp.curAmt
			case !d.Before(time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)) && d.Before(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)):
				amt = sp.prevAmt
			default:
				if dayOffset%5 == 0 {
					amt = 1 + int64(i)
				}
			}
			if amt == 0 && (sp.curAmt == 0 && !d.Before(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC))) {
				continue
			}
			if amt == 0 {
				continue
			}
			accountByDay[key] += amt
			if _, err := pool.Exec(ctx, `INSERT INTO sales_demand_daily(
				seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,
				sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)
				VALUES($1,$2,$3,$4,$5::numeric,$5::bigint,'RUB','RESOLVED',1,$6,$7,$8,$9)
				ON CONFLICT DO NOTHING`,
				seller, id, sp.ozon, d, amt, prompt81Hex(fmt.Sprintf("sd-%d-%s", sp.ozon, key)), salesRun, salesPayload, obs); err != nil {
				t.Fatal(err)
			}
		}
	}
	for day, amt := range accountByDay {
		if _, err := pool.Exec(ctx, `INSERT INTO sales_account_daily(
			seller_account_id,business_date,ordered_amount,ordered_units,currency_code,revision_no,fact_hash,
			collection_domain_run_id,source_payload_id,observed_at)
			VALUES($1,$2::date,$3::numeric,$3::bigint,'RUB',1,$4,$5,$6,$7)
			ON CONFLICT DO NOTHING`,
			seller, day, amt, prompt81Hex("sa-"+day), salesRun, salesPayload, obs); err != nil {
			t.Fatal(err)
		}
	}

	var poolID int64
	if err := pool.QueryRow(ctx, `INSERT INTO inventory_pools(
		seller_account_id,provider_pool_key,fulfillment_type,pool_type,independence_status,first_observed_at,last_observed_at)
		VALUES($1,'FBO:P91','FBO','PROVIDER_PHYSICAL','VERIFIED_INDEPENDENT',$2,$2)RETURNING id`, seller, obs).Scan(&poolID); err != nil {
		t.Fatal(err)
	}
	var snapID int64
	if err := pool.QueryRow(ctx, `INSERT INTO stock_snapshots(
		seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,
		requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,
		position_count,batches_expected,batches_completed,batches_failed,completeness_status,semantic_mapping_status,state_hash)
		VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,$5,$5,$5,$5,0,$5,1,1,0,'COMPLETE','VALIDATED',$6)RETURNING id`,
		seller, stockRun, obs, strings.Repeat("1", 64), len(specs), strings.Repeat("2", 64)).Scan(&snapID); err != nil {
		t.Fatal(err)
	}
	for i, sp := range specs {
		units := sp.stockUnits
		if sp.zeroStock {
			units = 0
		}
		if _, err := pool.Exec(ctx, `INSERT INTO stock_snapshot_lines(
			seller_account_id,snapshot_id,canonical_sku_id,source_sku,sku_resolution_status,inventory_pool_id,fulfillment_type,
			sellable_units,provider_valid_stock_count,quantity_semantic_status,source_payload_id,observed_at)
			VALUES($1,$2,$3,$4,'RESOLVED',$5,'FBO',$6,$6,'VALIDATED',$7,$8)`,
			seller, snapID, skuIDs[i], sp.ozon, poolID, units, stockPayload, obs); err != nil {
			t.Fatal(err)
		}
	}

	var priceColl int64
	if err := pool.QueryRow(ctx, `INSERT INTO price_collection_snapshots(
		seller_account_id,collection_domain_run_id,snapshot_at,collection_started_at,collection_completed_at,time_quality,
		requested_sku_universe_hash,expected_sku_count,requested_sku_count,received_sku_count,resolved_sku_count,unresolved_sku_count,
		batches_expected,batches_completed,batches_failed,completeness_status,state_hash)
		VALUES($1,$2,$3,$3,$3,'OBSERVED_APPROX',$4,$5,$5,$5,$5,0,1,1,0,'COMPLETE',$6)RETURNING id`,
		seller, priceRun, obs, strings.Repeat("3", 64), len(specs), strings.Repeat("4", 64)).Scan(&priceColl); err != nil {
		t.Fatal(err)
	}
	for i, sp := range specs {
		price := 100 + int64(i)*10
		oldRef := price + 20 // observed change vs current (SKU0 and others)
		if _, err := pool.Exec(ctx, `INSERT INTO price_snapshots(
			seller_account_id,price_collection_snapshot_id,canonical_sku_id,source_sku,provider_product_id,sku_resolution_status,
			collection_domain_run_id,observed_at,snapshot_at,time_quality,currency_code,provider_current_price,current_price_state,
			old_reference_price,customer_price,semantic_mapping_status,state_hash,source_payload_id,reconciliation_status)
			VALUES($1,$2,$3,$4,$5,'RESOLVED',$6,$7,$7,'OBSERVED_APPROX','RUB',$8,'KNOWN',$9,$8,'VALIDATED',$10,$11,'NOT_APPLICABLE')`,
			seller, priceColl, skuIDs[i], sp.ozon, sp.ozon+100, priceRun, obs, price, oldRef,
			prompt81Hex(fmt.Sprintf("price-%d", sp.ozon)), pricePayload); err != nil {
			t.Fatal(err)
		}
	}

	campIDs := make([]int64, 2)
	for i, providerID := range []int64{7701, 7702} {
		var cid int64
		if err := pool.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)
			VALUES($1,$2,$3,$3)RETURNING id`, seller, providerID, cut.Add(-20*24*time.Hour)).Scan(&cid); err != nil {
			t.Fatal(err)
		}
		campIDs[i] = cid
		if _, err := pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(
			seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,
			state_hash,observed_at,source_payload_id,collection_domain_run_id)
			VALUES($1,$2,1,$3,'ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$4,$5,$6,$7)`,
			seller, cid, fmt.Sprintf("Campaign-%d", providerID), prompt81Hex(fmt.Sprintf("camp-%d", providerID)), obs, adsPayload, adsRun); err != nil {
			t.Fatal(err)
		}
	}
	// Campaign daily facts for previous + current windows.
	for _, campID := range campIDs {
		for dayOffset := -13; dayOffset <= 0; dayOffset++ {
			d := analysisDay.AddDate(0, 0, dayOffset)
			spend := 5 + int64(dayOffset+13)
			attr := spend * 2
			if _, err := pool.Exec(ctx, `INSERT INTO advertising_daily_facts(
				seller_account_id,advertising_campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,
				attributed_order_units,attributed_order_amount,attribution_kind,provider_drr,currency_code,traffic_state,attribution_state,
				normalized_day_hash,source_contract_key,source_payload_id,collection_domain_run_id,observed_at)
				VALUES($1,$2,$3,1,$4,$5,$6::numeric,2,$7::numeric,'PROVIDER_DEFAULT',50,'RUB','USABLE','MATURE',
				$8,'ADVERTISING.CAMPAIGN_TRAFFIC',$9,$10,$11)`,
				seller, campID, d, 100+spend, 10+spend/2, spend, attr,
				prompt81Hex(fmt.Sprintf("ad-%d-%s", campID, d.Format("2006-01-02"))), adsPayload, adsRun, obs); err != nil {
				t.Fatal(err)
			}
		}
	}
	// SKU advertising product facts for a few promoted SKUs.
	for _, idx := range []int{0, 1, 4, 5} {
		for dayOffset := -13; dayOffset <= 0; dayOffset++ {
			d := analysisDay.AddDate(0, 0, dayOffset)
			spend := 2 + int64(idx)
			if _, err := pool.Exec(ctx, `INSERT INTO advertising_product_daily_facts(
				seller_account_id,advertising_campaign_id,business_date,promoted_canonical_sku_id,source_promoted_sku,sku_resolution_status,
				impressions,clicks,paid_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,currency_code,
				revision_no,state_hash,observed_at,source_payload_id,collection_domain_run_id)
				VALUES($1,$2,$3,$4,$5,'RESOLVED',$6,$7,$8::numeric,1,$9::numeric,'PROVIDER_DEFAULT','RUB',1,$10,$11,$12,$13)`,
				seller, campIDs[idx%2], d, skuIDs[idx], specs[idx].ozon, 50+spend, 5+spend, spend, spend*2,
				prompt81Hex(fmt.Sprintf("adsku-%d-%s", specs[idx].ozon, d.Format("2006-01-02"))), obs, adsPayload, adsRun); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Search SKU summaries current + previous windows.
	for i, sp := range specs[:6] {
		for _, win := range []struct {
			from, to time.Time
			kind     string
		}{
			{time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), analysisDay, "CURRENT_7D"},
			{time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC), "PREVIOUS_7D"},
		} {
			pos := float64(5 + i)
			if win.kind == "PREVIOUS_7D" {
				pos += 4 // materiality SEARCH_POSITION_CHANGE needs prev≠current
			}
			if _, err := pool.Exec(ctx, `INSERT INTO search_sku_summaries(
				seller_account_id,canonical_sku_id,source_sku,sku_resolution_status,period_start,period_end,period_kind,
				provider_search_gmv,orders,unique_search_users,unique_view_users,position,view_conversion,
				revision_no,state_hash,collection_domain_run_id,source_payload_id,observed_at)
				VALUES($1,$2,$3,'RESOLVED',$4,$5,$6,$7::numeric,$8::numeric,$9::numeric,$10::numeric,$11::numeric,0.12,
				1,$12,$13,$14,$15)`,
				seller, skuIDs[i], sp.ozon, win.from, win.to, win.kind,
				100+int64(i)*10, 10+int64(i), 20+int64(i), 15+int64(i), pos,
				prompt81Hex(fmt.Sprintf("search-%d-%s", sp.ozon, win.kind)), searchRun, searchPayload, obs); err != nil {
				t.Fatal(err)
			}
		}
	}

	// One open (overdue) supply with planned inbound for SKU-B1.
	var supplyOrder, supplyID, bundleID int64
	if err := pool.QueryRow(ctx, `INSERT INTO supply_orders(seller_account_id,provider_order_id,first_observed_at,last_observed_at)
		VALUES($1,'P91-ORDER-1',$2,$2)RETURNING id`, seller, cut.Add(-10*24*time.Hour)).Scan(&supplyOrder); err != nil {
		t.Fatal(err)
	}
	overdueTo := analysisDay.AddDate(0, 0, -3)
	if _, err := pool.Exec(ctx, `INSERT INTO supply_order_versions(
		seller_account_id,supply_order_id,version_no,provider_state_code,canonical_state,state_mapping_version,state_mapping_status,
		timeslot_from,timeslot_to,state_hash,observed_at,source_payload_id,collection_domain_run_id)
		VALUES($1,$2,1,'IN_TRANSIT','IN_TRANSIT','supply_state_mapping_v1','MAPPED',$3,$4,$5,$6,$7,$8)`,
		seller, supplyOrder, overdueTo.Add(-24*time.Hour), overdueTo, prompt81Hex("supply-order-v1"), obs, supplyPayload, supplyRun); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO supplies(seller_account_id,supply_order_id,provider_supply_id,first_observed_at,last_observed_at)
		VALUES($1,$2,'P91-SUPPLY-1',$3,$3)RETURNING id`, seller, supplyOrder, cut.Add(-10*24*time.Hour)).Scan(&supplyID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO supply_versions(
		seller_account_id,supply_id,version_no,provider_state_code,canonical_state,state_mapping_version,state_mapping_status,
		state_hash,observed_at,source_payload_id,collection_domain_run_id)
		VALUES($1,$2,1,'IN_TRANSIT','IN_TRANSIT','supply_state_mapping_v1','MAPPED',$3,$4,$5,$6)`,
		seller, supplyID, prompt81Hex("supply-v1"), obs, supplyPayload, supplyRun); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO supply_bundle_versions(
		seller_account_id,supply_id,version_no,provider_bundle_id,completeness_status,page_count,item_count,composition_hash,
		observed_at,source_payload_id,collection_domain_run_id)
		VALUES($1,$2,1,'BUNDLE-1','COMPLETE',1,1,$3,$4,$5,$6)RETURNING id`,
		seller, supplyID, prompt81Hex("bundle-1"), obs, supplyPayload, supplyRun).Scan(&bundleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO supply_planned_items(
		seller_account_id,supply_bundle_version_id,line_ordinal,canonical_sku_id,source_sku,sku_resolution_status,declared_units,source_payload_id)
		VALUES($1,$2,1,$3,$4,'RESOLVED',25,$5)`, seller, bundleID, skuIDs[4], specs[4].ozon, supplyPayload); err != nil {
		t.Fatal(err)
	}

	// Watermarks — fact families used by readiness / resolve source bindings.
	histFrom := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	histTo := analysisDay
	wm := func(domain, family string, runID int64, state string) {
		t.Helper()
		wh := prompt81Hex(state)
		_, err := pool.Exec(ctx, `INSERT INTO domain_watermarks(
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
			seller, domain, family, obs, histTo, histFrom, runID, wh)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
			seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
			latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
			source_contract_hash,state_hash,assessed_at,knowledge_time)
			VALUES($1,$2,$3,$4,$5::date,$5::date,$4,$6::date,$5::date,$7,$8,$8,$4,$4)`,
			seller, domain, family, obs, histTo, histFrom, runID, wh)
		if err != nil {
			t.Fatal(err)
		}
	}
	wm("CATALOG", "CATALOG.SKU_UNIVERSE", catalogRun, "wm-catalog")
	wm("SALES_DEMAND", "SALES.ACCOUNT_DEMAND", salesRun, "wm-sales-acct")
	wm("SALES_DEMAND", "SALES.SKU_DEMAND", salesRun, "wm-sales-sku")
	wm("STOCK", "STOCK.SNAPSHOT", stockRun, "wm-stock")
	wm("PRICE", "PRICE.SNAPSHOT", priceRun, "wm-price")
	wm("ADVERTISING", "ADVERTISING.TRAFFIC", adsRun, "wm-ads-traffic")
	wm("ADVERTISING", "ADVERTISING.ATTRIBUTION", adsRun, "wm-ads-attr")
	wm("SEARCH", "SEARCH.SELLER_SUMMARY", searchRun, "wm-search")
	wm("SUPPLY", "SUPPLY.STATE", supplyRun, "wm-supply")

	var dqRun int64
	if err := pool.QueryRow(ctx, `INSERT INTO data_quality_runs(
		seller_account_id,cutoff_at,policy_version,input_hash,result_hash,status,started_at,completed_at)
		VALUES($1,$2,'dq_policy_v3',$3,$4,'COMPLETED',$2,$2)RETURNING id`,
		seller, cut, prompt81Hex("p91-dq-in"), prompt81Hex("p91-dq-res")).Scan(&dqRun); err != nil {
		t.Fatal(err)
	}

	type capSpec struct {
		key, domain string
		runID       int64
	}
	caps := []capSpec{
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
		{"SUPPLY.STATE", "SUPPLY", supplyRun},
		{"SUPPLY.BUNDLE", "SUPPLY", supplyRun},
		{"SUPPLY.SKU_INBOUND", "SUPPLY", supplyRun},
	}
	for _, c := range caps {
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

func prompt91LoadStageBody(t *testing.T, f prompt81Fixture, store artifacts.ArtifactStore, runID int64, stageKey string) []byte {
	t.Helper()
	st, err := f.repo.GetStage(context.Background(), runID, stageKey)
	if err != nil {
		t.Fatal(err)
	}
	if st.OutputHash == "" {
		t.Fatalf("stage %s missing output hash (status=%s)", stageKey, st.Status)
	}
	key, err := f.repo.ArtifactStorageKey(context.Background(), runID, st.OutputHash)
	if err != nil {
		t.Fatal(err)
	}
	body, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func prompt91LoadArtifactKind(t *testing.T, f prompt81Fixture, store artifacts.ArtifactStore, runID int64, kind string) []byte {
	t.Helper()
	var key string
	err := f.pool.QueryRow(context.Background(),
		`SELECT storage_key FROM analysis_artifacts WHERE run_id=$1 AND kind=$2 ORDER BY id DESC LIMIT 1`,
		runID, kind).Scan(&key)
	if err != nil {
		t.Fatalf("artifact kind %s: %v", kind, err)
	}
	body, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPrompt91ProductionFactualPathStages1to6(t *testing.T) {
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt91-exec", nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
		st, err := f.repo.GetStage(ctx, run.ID, key)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != "SUCCEEDED" && st.Status != "SUCCEEDED_WITH_LIMITATIONS" {
			t.Fatalf("stage %s status=%s err=%s", key, st.Status, st.ErrorMessage)
		}
	}

	// --- Landscape Core ---
	coreBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeCore))
	var core landscape.LandscapeSet
	if err = json.Unmarshal(coreBody, &core); err != nil {
		t.Fatal(err)
	}
	if len(core.GlobalCapabilities) == 0 {
		t.Fatal("GlobalCapabilities empty — sealed readiness keys did not propagate")
	}
	readyKeys := map[string]bool{}
	for _, c := range core.GlobalCapabilities {
		if c.FinalState == materiality.ReadinessReady || c.FinalState == "READY" {
			readyKeys[c.Key] = true
		}
	}
	for _, want := range []string{
		"SALES.SKU_DEMAND", "SALES.ACCOUNT_DEMAND", "STOCK.SKU_CURRENT",
		"ADVERTISING.CAMPAIGN_TRAFFIC", "PRICE.CUSTOMER_PRICE",
	} {
		if !readyKeys[want] {
			t.Fatalf("expected READY capability %s in GlobalCapabilities; got %#v", want, readyKeys)
		}
	}
	domains := map[string]bool{}
	for _, w := range core.DomainWindows {
		domains[w.Domain] = true
	}
	if len(domains) < 2 {
		t.Fatalf("DomainWindows expected >=2 domains, got %#v", domains)
	}
	if core.Account.Sales == nil || (core.Account.Sales.State != landscape.BlockReady && core.Account.Sales.State != landscape.BlockPartial) {
		t.Fatalf("Account.Sales missing/unusable: %+v", core.Account.Sales)
	}
	if len(core.SKUs) < 5 {
		t.Fatalf("SKU count=%d want >=5", len(core.SKUs))
	}
	campPerfOK := false
	for _, c := range core.Campaigns {
		if c.Performance != nil && c.Performance.State != landscape.BlockUnavailable {
			campPerfOK = true
			break
		}
	}
	if !campPerfOK {
		t.Fatalf("Campaign Performance all UNAVAILABLE despite ads seed; campaigns=%d", len(core.Campaigns))
	}

	// --- Materiality ranking / selection ---
	rankBody := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.MaterialityRanking))
	var ranking materiality.RankingArtifact
	if err = json.Unmarshal(rankBody, &ranking); err != nil {
		t.Fatal(err)
	}
	laneHit := false
	anchorHit := false
	for _, e := range ranking.SKUs {
		if len(e.LaneMemberships) > 0 {
			laneHit = true
		}
		if e.IsAnchor {
			anchorHit = true
		}
	}
	if !laneHit {
		t.Fatal("expected nonempty lane memberships for some SKU")
	}

	selBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.RunMateriality))
	var sel materiality.SelectionArtifact
	if err = json.Unmarshal(selBody, &sel); err != nil {
		t.Fatal(err)
	}
	if len(sel.SKUs) == 0 && !anchorHit {
		t.Fatal("expected SKU selection or sales-exposure anchors")
	}
	if len(sel.Campaigns) == 0 {
		// Campaigns with spend should enter selection or at least ranking anchors.
		campRanked := false
		for _, c := range ranking.Campaigns {
			if len(c.LaneMemberships) > 0 || c.IsAnchor {
				campRanked = true
				break
			}
		}
		if !campRanked {
			t.Fatal("expected Campaign selection or ranked campaigns with spend seed")
		}
	}

	var rankingHash, selectionHash string
	if err = f.pool.QueryRow(ctx, `SELECT ranking_hash,selection_hash FROM materiality_runs WHERE analysis_run_id=$1`, run.ID).
		Scan(&rankingHash, &selectionHash); err != nil {
		t.Fatal(err)
	}
	if rankingHash == "" || selectionHash == "" || rankingHash == selectionHash {
		t.Fatalf("unstable/identical materiality hashes r=%s s=%s", rankingHash, selectionHash)
	}

	// --- Detail ---
	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Account.Identity.SellerAccountID != f.seller {
		t.Fatalf("detail account seller=%d want %d", detail.Account.Identity.SellerAccountID, f.seller)
	}
	if len(detail.SKUs) == 0 && len(detail.Campaigns) == 0 && len(detail.Categories) == 0 {
		t.Fatal("detail has no selected entities")
	}
	for _, banned := range []string{"fusion_utility", "lane_rank", "materiality_score"} {
		if strings.Contains(string(detailBody), banned) {
			t.Fatalf("Detail leaked materiality field %q", banned)
		}
	}

	// Stage 7 must not become READY.
	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status == "READY" {
			t.Fatal("stage 11 BUILD_DEEP_PACK must not become READY")
		}
	}

	// Duplicate RUN_MATERIALITY: idempotent ExecuteStage + stable hashes / no conflict reuse.
	if err = exec.ExecuteStage(ctx, run.ID, string(stages.RunMateriality)); err != nil {
		t.Fatalf("duplicate ExecuteStage RUN_MATERIALITY: %v", err)
	}
	var count int
	var rh2, sh2 string
	if err = f.pool.QueryRow(ctx, `SELECT count(*),max(ranking_hash),max(selection_hash) FROM materiality_runs WHERE analysis_run_id=$1`, run.ID).
		Scan(&count, &rh2, &sh2); err != nil {
		t.Fatal(err)
	}
	if count != 1 || rh2 != rankingHash || sh2 != selectionHash {
		t.Fatalf("materiality run mutated on duplicate: count=%d r=%s/%s s=%s/%s", count, rankingHash, rh2, selectionHash, sh2)
	}
	// Explicit same-result PutMaterialityRun must reuse, not CONFLICT.
	got, err := f.repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	sealStage, err := f.repo.GetStage(ctx, run.ID, string(stages.SealUniverse))
	if err != nil {
		t.Fatal(err)
	}
	coreStage, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeCore))
	if err != nil {
		t.Fatal(err)
	}
	policyV := materiality.CurrentPolicy()
	budget := materiality.DiscoveryStandardBudget()
	id, err := f.repo.PutMaterialityRun(ctx, analysispg.MaterialityRunInput{
		AnalysisRunID: got.ID, SellerID: got.SellerID, UniverseID: got.UniverseID,
		UniverseManifestHash: sealStage.OutputHash, LandscapeCoreProfile: core.Context.LandscapeProfileVersion,
		InputLandscapeCoreHash: coreStage.OutputHash, PolicyVersion: policyV.Version, PolicyHash: policyV.Hash(),
		BudgetVersion: budget.Version, BudgetHash: budget.Hash(),
		RankingHash: rankingHash, SelectionHash: selectionHash,
	})
	if err != nil {
		t.Fatalf("same-result PutMaterialityRun: %v", err)
	}
	if id <= 0 {
		t.Fatal("expected reused materiality_run id")
	}
}

func TestPrompt91ProductionAsynqStages1to6(t *testing.T) {
	redisURL := integrationRedisURL(t)
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := asynq.NewClient(opt)
	t.Cleanup(func() { _ = client.Close() })

	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt91-asynq", runtime.AsynqEnqueueFunc(client))
	if err != nil {
		t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	runtime.RegisterStageExecuteHandler(mux, exec)
	srv := asynq.NewServer(opt, asynq.Config{Concurrency: 1})
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(mux) }()
	t.Cleanup(func() {
		srv.Shutdown()
		select {
		case <-errCh:
		case <-time.After(2 * time.Second):
		}
	})

	if err = runtime.EnqueueStageExecute(ctx, client, run.ID, string(stages.ResolveContext)); err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		waitStageSucceeded(t, f, run.ID, key, 90*time.Second)
	}

	coreBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeCore))
	var core landscape.LandscapeSet
	if err = json.Unmarshal(coreBody, &core); err != nil {
		t.Fatal(err)
	}
	if len(core.GlobalCapabilities) == 0 || len(core.SKUs) < 5 || core.Account.Sales == nil {
		t.Fatalf("asynq core thin: caps=%d skus=%d sales=%v", len(core.GlobalCapabilities), len(core.SKUs), core.Account.Sales != nil)
	}
	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status != "SUCCEEDED" {
			t.Fatalf("stage 11 must succeed; status=%s", s.Status)
		}
		if s.Key == string(stages.PostPublish) && s.Status != "SUCCEEDED" {
			t.Fatalf("stage 19 must succeed; status=%s", s.Status)
		}
	}
}
