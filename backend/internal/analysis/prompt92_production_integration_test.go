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
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

// prompt92Seed extends prompt91 with price history, multi-family SEARCH watermarks, and lane-driving facts.
func prompt92Seed(t *testing.T) prompt81Fixture {
	t.Helper()
	f := prompt91Seed(t)
	ctx := context.Background()
	analysisDay := f.cut.AddDate(0, 0, -1)

	var skuIDs []int64
	rows, err := f.pool.Query(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 ORDER BY id`, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		skuIDs = append(skuIDs, id)
	}
	rows.Close()

	var priceRun int64
	if err = f.pool.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='PRICE'`, f.seller).Scan(&priceRun); err != nil {
		t.Fatal(err)
	}
	var priceColl int64
	if err = f.pool.QueryRow(ctx, `SELECT id FROM price_collection_snapshots WHERE seller_account_id=$1 ORDER BY id DESC LIMIT 1`, f.seller).Scan(&priceColl); err != nil {
		t.Fatal(err)
	}
	var pricePayload int64
	if err = f.pool.QueryRow(ctx, `SELECT id FROM source_payloads WHERE collection_domain_run_id=$1 ORDER BY id DESC LIMIT 1`, priceRun).Scan(&pricePayload); err != nil {
		t.Fatal(err)
	}
	olderObs := f.cut.Add(-72 * time.Hour)
	for i, skuID := range skuIDs {
		oldPrice := 200 + int64(i)*10
		if _, err = f.pool.Exec(ctx, `INSERT INTO price_snapshots(
			seller_account_id,price_collection_snapshot_id,canonical_sku_id,source_sku,provider_product_id,sku_resolution_status,
			collection_domain_run_id,observed_at,snapshot_at,time_quality,currency_code,provider_current_price,current_price_state,
			customer_price,semantic_mapping_status,state_hash,source_payload_id,reconciliation_status)
			VALUES($1,$2,$3,$4,$5,'RESOLVED',$6,$7,$7,'OBSERVED_APPROX','RUB',$8,'KNOWN',$8,'VALIDATED',$9,$10,'NOT_APPLICABLE')`,
			f.seller, priceColl, skuID, 9000+int64(i), 9100+int64(i), priceRun, olderObs, oldPrice,
			prompt81Hex(fmt.Sprintf("price-old-%d", skuID)), pricePayload); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO business_capability_assessments(
		seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
		contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
		reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
		VALUES($1,$2,'PRICE.HISTORY','HISTORICAL_QUERY','PRICE','READY',
		 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','AVAILABLE','NON_EMPTY',
		 'OK','{}','capability_policy_v3',$3,$4)
		ON CONFLICT DO NOTHING`, f.seller, f.dqRun, f.cut.Add(-time.Minute), priceRun); err != nil {
		t.Fatal(err)
	}

	var searchRun int64
	if err = f.pool.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='SEARCH'`, f.seller).Scan(&searchRun); err != nil {
		t.Fatal(err)
	}
	histFrom := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	histTo := analysisDay
	wh := prompt81Hex("wm-search-query")
	obs := f.cut.Add(-2 * time.Hour)
	if _, err = f.pool.Exec(ctx, `INSERT INTO domain_watermarks(
		seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
		latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
		source_contract_hash,state_hash,assessed_at)
		VALUES($1,'SEARCH','SEARCH.QUERY_DETAILS',$2,$3::date,$3::date,$2,$4::date,$3::date,$5,$6,$6,$2)
		ON CONFLICT(seller_account_id,domain,fact_family) DO UPDATE SET
		  latest_observed_at=EXCLUDED.latest_observed_at,state_hash=EXCLUDED.state_hash,assessed_at=EXCLUDED.assessed_at,
		  last_successful_domain_run_id=EXCLUDED.last_successful_domain_run_id`,
		f.seller, obs, histTo, histFrom, searchRun, wh); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
		seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
		latest_complete_snapshot_at,history_available_from,history_available_to,last_successful_domain_run_id,
		source_contract_hash,state_hash,assessed_at,knowledge_time)
		VALUES($1,'SEARCH','SEARCH.QUERY_DETAILS',$2,$3::date,$3::date,$2,$4::date,$3::date,$5,$6,$6,$2,$2)`,
		f.seller, obs, histTo, histFrom, searchRun, wh); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO business_capability_assessments(
		seller_account_id,data_quality_run_id,capability_key,capability_scope,domain,final_state,
		contract_state,access_state,source_state,semantic_state,coverage_state,temporal_state,history_state,population_state,
		reason_code,details_json,policy_version,assessed_at,source_domain_run_id)
		VALUES($1,$2,'SEARCH.QUERY_DETAILS','HISTORICAL_QUERY','SEARCH','READY',
		 'ACTIVE','ACCESSIBLE','SUCCESS','VALIDATED','FULL','CURRENT','AVAILABLE','NON_EMPTY',
		 'OK','{}','capability_policy_v3',$3,$4)
		ON CONFLICT DO NOTHING`, f.seller, f.dqRun, f.cut.Add(-time.Minute), searchRun); err != nil {
		t.Fatal(err)
	}

	// Lag SEARCH.SELLER_SUMMARY coverage behind analysis day so DomainWindow exposes provider lag.
	summaryTo := analysisDay.AddDate(0, 0, -2)
	summaryWh := prompt81Hex("wm-search-summary-lag")
	if _, err = f.pool.Exec(ctx, `UPDATE domain_watermarks SET
		latest_complete_business_date=$3::date,latest_mature_business_date=$3::date,
		history_available_to=$3::date,state_hash=$4,assessed_at=$2
		WHERE seller_account_id=$1 AND domain='SEARCH' AND fact_family='SEARCH.SELLER_SUMMARY'`,
		f.seller, f.cut.Add(-time.Hour), summaryTo, summaryWh); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO domain_watermark_revisions(
		seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
		history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at,knowledge_time)
		SELECT seller_account_id,domain,fact_family,latest_observed_at,latest_complete_business_date,latest_mature_business_date,
		history_available_from,history_available_to,last_successful_domain_run_id,source_contract_hash,state_hash,assessed_at,$2
		FROM domain_watermarks WHERE seller_account_id=$1 AND domain='SEARCH' AND fact_family='SEARCH.SELLER_SUMMARY'`,
		f.seller, f.cut.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	// prompt91 seeds summaries for analysis-day windows; V3 core resolves SEARCH.SELLER_SUMMARY
	// from lagged coverage_to, so realign stored period_start/period_end.
	anchor := summaryTo
	curFrom := anchor.AddDate(0, 0, -6)
	prevFrom := anchor.AddDate(0, 0, -13)
	prevTo := anchor.AddDate(0, 0, -7)
	if _, err = f.pool.Exec(ctx, `UPDATE search_sku_summaries SET period_start=$2, period_end=$3
		WHERE seller_account_id=$1 AND period_kind='CURRENT_7D'`, f.seller, curFrom, anchor); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE search_sku_summaries SET period_start=$2, period_end=$3
		WHERE seller_account_id=$1 AND period_kind='PREVIOUS_7D'`, f.seller, prevFrom, prevTo); err != nil {
		t.Fatal(err)
	}

	return f
}

// prompt93Seed extends prompt92 with facts that exercise every active materiality lane.
func prompt93Seed(t *testing.T) prompt81Fixture {
	t.Helper()
	f := prompt92Seed(t)
	ctx := context.Background()
	analysisDay := f.cut.AddDate(0, 0, -1)
	obs := f.cut.Add(-2 * time.Hour)

	var adsRun, adsPayload int64
	if err := f.pool.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='ADVERTISING'`, f.seller).Scan(&adsRun); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT id FROM source_payloads WHERE collection_domain_run_id=$1 ORDER BY id DESC LIMIT 1`, adsRun).Scan(&adsPayload); err != nil {
		t.Fatal(err)
	}

	var campIDs []int64
	rows, err := f.pool.Query(ctx, `SELECT id FROM advertising_campaigns WHERE seller_account_id=$1 ORDER BY provider_campaign_id`, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		campIDs = append(campIDs, id)
	}
	rows.Close()
	if len(campIDs) < 2 {
		t.Fatalf("expected at least 2 campaigns, got %d", len(campIDs))
	}

	if _, err = f.pool.Exec(ctx, `DELETE FROM advertising_daily_facts WHERE seller_account_id=$1`, f.seller); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `DELETE FROM advertising_product_daily_facts WHERE seller_account_id=$1`, f.seller); err != nil {
		t.Fatal(err)
	}

	prevFrom := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	prevTo := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	curFrom := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)

	insertCampDay := func(campID int64, d time.Time, spend, attr int64, impressions, clicks int64, drr int64) {
		t.Helper()
		if _, err = f.pool.Exec(ctx, `INSERT INTO advertising_daily_facts(
			seller_account_id,advertising_campaign_id,business_date,revision_no,impressions,clicks,paid_spend_amount,
			attributed_order_units,attributed_order_amount,attribution_kind,provider_drr,currency_code,traffic_state,attribution_state,
			normalized_day_hash,source_contract_key,source_payload_id,collection_domain_run_id,observed_at)
			VALUES($1,$2,$3,1,$4,$5,$6::numeric,2,$7::numeric,'PROVIDER_DEFAULT',$8,'RUB','USABLE','MATURE',
			$9,'ADVERTISING.CAMPAIGN_TRAFFIC',$10,$11,$12)`,
			f.seller, campID, d, impressions, clicks, spend, attr, drr,
			prompt81Hex(fmt.Sprintf("p93-ad-%d-%s", campID, d.Format("2006-01-02"))), adsPayload, adsRun, obs); err != nil {
			t.Fatal(err)
		}
	}

	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		insertCampDay(campIDs[0], d, 900, 1800, 10000, 500, 70)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertCampDay(campIDs[0], d, 400, 1600, 20000, 200, 30)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertCampDay(campIDs[1], d, 900, 1800, 8000, 400, 50)
	}
	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		insertCampDay(campIDs[1], d, 0, 0, 0, 0, 0)
	}

	var campIncrease int64
	if err = f.pool.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)
		VALUES($1,7704,$2,$2)RETURNING id`, f.seller, f.cut.Add(-20*24*time.Hour)).Scan(&campIncrease); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(
		seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,
		state_hash,observed_at,source_payload_id,collection_domain_run_id)
		VALUES($1,$2,1,'Campaign-7704','ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$3,$4,$5,$6)`,
		f.seller, campIncrease, prompt81Hex("camp-7704"), obs, adsPayload, adsRun); err != nil {
		t.Fatal(err)
	}
	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		insertCampDay(campIncrease, d, 200, 400, 4000, 200, 40)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertCampDay(campIncrease, d, 900, 1800, 8000, 400, 45)
	}

	var campToZero int64
	if err = f.pool.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)
		VALUES($1,7703,$2,$2)RETURNING id`, f.seller, f.cut.Add(-20*24*time.Hour)).Scan(&campToZero); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(
		seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,
		state_hash,observed_at,source_payload_id,collection_domain_run_id)
		VALUES($1,$2,1,'Campaign-7703','ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$3,$4,$5,$6)`,
		f.seller, campToZero, prompt81Hex("camp-7703"), obs, adsPayload, adsRun); err != nil {
		t.Fatal(err)
	}
	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		insertCampDay(campToZero, d, 900, 1800, 9000, 450, 55)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertCampDay(campToZero, d, 0, 0, 0, 0, 0)
	}

	var skuIDs []int64
	rows, err = f.pool.Query(ctx, `SELECT id FROM canonical_skus WHERE seller_account_id=$1 ORDER BY id`, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		skuIDs = append(skuIDs, id)
	}
	rows.Close()
	if len(skuIDs) < 9 {
		t.Fatalf("expected seeded SKUs, got %d", len(skuIDs))
	}

	insertSKUAdDay := func(campID, skuID, ozon, spend int64, d time.Time) {
		t.Helper()
		if _, err = f.pool.Exec(ctx, `INSERT INTO advertising_product_daily_facts(
			seller_account_id,advertising_campaign_id,business_date,promoted_canonical_sku_id,source_promoted_sku,sku_resolution_status,
			impressions,clicks,paid_spend_amount,attributed_order_units,attributed_order_amount,attribution_kind,currency_code,
			revision_no,state_hash,observed_at,source_payload_id,collection_domain_run_id)
			VALUES($1,$2,$3,$4,$5,'RESOLVED',$6,$7,$8::numeric,1,$9::numeric,'PROVIDER_DEFAULT','RUB',1,$10,$11,$12,$13)`,
			f.seller, campID, d, skuID, ozon, 50+spend, 5+spend/10, spend, spend*2,
			prompt81Hex(fmt.Sprintf("p93-adsku-%d-%s", skuID, d.Format("2006-01-02"))), obs, adsPayload, adsRun); err != nil {
			t.Fatal(err)
		}
	}

	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		insertSKUAdDay(campIDs[0], skuIDs[5], 9106, 120, d)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertSKUAdDay(campIDs[0], skuIDs[5], 9106, 15, d)
	}

	catBOzon := []int64{9105, 9106, 9107, 9108, 9109}
	for i, idx := range []int{4, 5, 6, 7, 8} {
		for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
			insertSKUAdDay(campIDs[i%2], skuIDs[idx], catBOzon[i], 80, d)
		}
		for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
			insertSKUAdDay(campIDs[i%2], skuIDs[idx], catBOzon[i], 20, d)
		}
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertSKUAdDay(campIDs[1], skuIDs[1], 9102, 25, d)
	}
	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		insertSKUAdDay(campIDs[0], skuIDs[0], 9101, 10, d)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		insertSKUAdDay(campIDs[0], skuIDs[0], 9101, 100, d)
	}

	var salesRun, salesPayload int64
	if err = f.pool.QueryRow(ctx, `SELECT collection_domain_run_id FROM collection_authoritative_heads WHERE seller_account_id=$1 AND domain='SALES_DEMAND'`, f.seller).Scan(&salesRun); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT id FROM source_payloads WHERE collection_domain_run_id=$1 ORDER BY id DESC LIMIT 1`, salesRun).Scan(&salesPayload); err != nil {
		t.Fatal(err)
	}
	upsertSalesDay := func(skuID, ozon, amt int64, d time.Time) {
		t.Helper()
		if _, err = f.pool.Exec(ctx, `DELETE FROM sales_demand_daily WHERE seller_account_id=$1 AND canonical_sku_id=$2 AND business_date=$3`,
			f.seller, skuID, d); err != nil {
			t.Fatal(err)
		}
		if amt < 0 {
			return
		}
		if _, err = f.pool.Exec(ctx, `INSERT INTO sales_demand_daily(
			seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,
			sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at)
			VALUES($1,$2,$3,$4,$5::numeric,$5::bigint,'RUB','RESOLVED',1,$6,$7,$8,$9)`,
			f.seller, skuID, ozon, d, amt, prompt81Hex(fmt.Sprintf("p93-sd-%d-%s", ozon, d.Format("2006-01-02"))), salesRun, salesPayload, obs); err != nil {
			t.Fatal(err)
		}
	}
	// FROM_ZERO / TO_ZERO need KNOWN zero periods (not missing).
	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		upsertSalesDay(skuIDs[2], 9103, 0, d)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		upsertSalesDay(skuIDs[3], 9104, 0, d)
	}
	// High current + 30d exposure + stock coverage lane floors.
	for dayOffset := -29; dayOffset <= 0; dayOffset++ {
		d := analysisDay.AddDate(0, 0, dayOffset)
		amt := int64(60)
		if !d.Before(curFrom) && !d.After(analysisDay) {
			amt = 200
		}
		upsertSalesDay(skuIDs[0], 9101, amt, d)
	}
	// Category-A absolute sales decrease (boost previous, cut current on anchor SKU).
	for d := prevFrom; !d.After(prevTo); d = d.AddDate(0, 0, 1) {
		upsertSalesDay(skuIDs[1], 9102, 300, d)
	}
	for d := curFrom; !d.After(analysisDay); d = d.AddDate(0, 0, 1) {
		upsertSalesDay(skuIDs[1], 9102, 20, d)
	}

	return f
}

func prompt92LaneFamily(lv materiality.LaneVersion) string {
	if lv.EntityKind == materiality.EntityCampaign {
		return "CAMPAIGN"
	}
	switch lv.Domain {
	case materiality.DomainSales:
		return "SALES"
	case materiality.DomainStock:
		return "STOCK"
	case materiality.DomainAdvertising:
		return "ADVERTISING"
	case materiality.DomainSearch:
		return "SEARCH"
	case materiality.DomainPrice:
		return "PRICE"
	case materiality.DomainSupply:
		return "SUPPLY"
	default:
		return ""
	}
}

func prompt92LaneHits(ranking materiality.RankingArtifact) map[string]bool {
	reg := materiality.LaneRegistryV1Current()
	laneDomain := map[string]string{}
	for _, lv := range reg.Versions {
		laneDomain[lv.LaneKey+"|"+string(lv.EntityKind)] = prompt92LaneFamily(lv)
	}
	hits := map[string]bool{}
	check := func(memberships []materiality.LaneMembershipArtifact, kind materiality.EntityKind) {
		for _, m := range memberships {
			if fam := laneDomain[m.LaneKey+"|"+string(kind)]; fam != "" {
				hits[fam] = true
			}
		}
	}
	for _, e := range ranking.SKUs {
		check(e.LaneMemberships, materiality.EntitySKU)
	}
	for _, e := range ranking.Categories {
		check(e.LaneMemberships, materiality.EntityCategory)
	}
	for _, e := range ranking.Campaigns {
		check(e.LaneMemberships, materiality.EntityCampaign)
	}
	return hits
}

func TestPrompt92ProductionLaneFamiliesStages1to6(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	f := prompt92Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt92-exec", nil)
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

	coreBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeCore))
	var core landscape.LandscapeSet
	if err = json.Unmarshal(coreBody, &core); err != nil {
		t.Fatal(err)
	}

	searchLagOK := false
	for _, w := range core.DomainWindows {
		if w.Domain == "search" && w.LagDays != nil {
			searchLagOK = true
			break
		}
	}
	if !searchLagOK {
		t.Fatalf("expected search DomainWindow with lag; windows=%#v", core.DomainWindows)
	}

	rankBody := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.MaterialityRanking))
	var ranking materiality.RankingArtifact
	if err = json.Unmarshal(rankBody, &ranking); err != nil {
		t.Fatal(err)
	}

	wantFamilies := []string{"SALES", "STOCK", "ADVERTISING", "SEARCH", "PRICE", "SUPPLY", "CAMPAIGN"}
	hits := prompt92LaneHits(ranking)
	for _, fam := range wantFamilies {
		if !hits[fam] {
			t.Errorf("lane family %s: FAIL (no ranking candidate)", fam)
		} else {
			t.Logf("lane family %s: PASS", fam)
		}
	}
	for _, fam := range wantFamilies {
		if !hits[fam] {
			t.Fatalf("missing lane family candidate: %s (hits=%#v)", fam, hits)
		}
	}

	selBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.RunMateriality))
	var sel materiality.SelectionArtifact
	if err = json.Unmarshal(selBody, &sel); err != nil {
		t.Fatal(err)
	}
	nonAnchor := false
	for _, e := range sel.SKUs {
		if !e.SelectedAsAnchor {
			nonAnchor = true
			break
		}
	}
	if !nonAnchor {
		for _, e := range ranking.SKUs {
			if len(e.LaneMemberships) > 0 && !e.IsAnchor {
				nonAnchor = true
				break
			}
		}
	}
	if !nonAnchor {
		t.Fatal("expected non-anchor SKU in selection or ranking with lane memberships")
	}

	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	campaignCtxOK := false
	for _, sk := range detail.SKUs {
		if sk.CampaignContext == nil || len(sk.CampaignContext.Items) == 0 {
			continue
		}
		campaignCtxOK = true
		break
	}
	if !campaignCtxOK {
		t.Fatal("expected SKU detail campaign_context when fixture has direct product ad facts")
	}

	for _, code := range []string{"FUNNEL_ACCOUNT_ROLLUP_NOT_PROJECTED", "AD_ACCOUNT_SUMMARY_PARTIAL", "IMPLEMENTATION_NOT_PROJECTED"} {
		if strings.Contains(string(coreBody), code) {
			t.Fatalf("core still contains legacy limitation code %q", code)
		}
	}
}
