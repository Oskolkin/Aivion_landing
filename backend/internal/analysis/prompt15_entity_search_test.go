//go:build integration

package analysis_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func prompt15SearchAPI(t *testing.T, f prompt81Fixture) *factual.Service {
	t.Helper()
	secret := strings.Repeat("t", 32)
	signer, err := factualapi.NewTokenSigner([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return factual.NewService(f.pool, signer, 0, 0)
}

func prompt15InsertFillerSKUs(t *testing.T, f prompt81Fixture, n int, targetAt int, targetName string) int64 {
	t.Helper()
	ctx := context.Background()
	obs := f.cut.Add(-2 * time.Hour)
	base := time.Now().UnixNano()%1_000_000 + 200000
	var targetID int64
	for i := 0; i < n; i++ {
		ozon := base + int64(i)
		name := fmt.Sprintf("Filler Search SKU %d", i)
		if i == targetAt {
			name = targetName
		}
		var id int64
		if err := f.pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)
			VALUES($1,$2,$3,$3,'ACTIVE')RETURNING id`, f.seller, ozon, f.cut.Add(-10*24*time.Hour)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO canonical_sku_versions(
			canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)
			VALUES($1,$2,$3,$4,'active','ACTIVE',$5,$6,$7)`,
			id, ozon+100, fmt.Sprintf("offer-fill-%d", ozon), name, fmt.Sprintf("%064x", ozon), obs, f.domainRun); err != nil {
			t.Fatal(err)
		}
		if i == targetAt {
			targetID = id
		}
	}
	if targetID == 0 {
		t.Fatal("target sku not inserted")
	}
	return targetID
}

func prompt15InsertFillerCampaigns(t *testing.T, f prompt81Fixture, n int, targetAt int, targetTitle string) int64 {
	t.Helper()
	ctx := context.Background()
	obs := f.cut.Add(-2 * time.Hour)
	var payloadID int64
	if err := f.pool.QueryRow(ctx, `SELECT id FROM source_payloads WHERE seller_account_id=$1 ORDER BY id DESC LIMIT 1`, f.seller).Scan(&payloadID); err != nil {
		t.Fatal(err)
	}
	var adsRun int64
	if err := f.pool.QueryRow(ctx, `SELECT id FROM collection_domain_runs WHERE seller_account_id=$1 AND domain='ADVERTISING' ORDER BY id DESC LIMIT 1`, f.seller).Scan(&adsRun); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UnixNano()%1_000_000 + 300000
	var targetID int64
	for i := 0; i < n; i++ {
		title := fmt.Sprintf("Filler Campaign %d", i)
		if i == targetAt {
			title = targetTitle
		}
		var id int64
		if err := f.pool.QueryRow(ctx, `INSERT INTO advertising_campaigns(seller_account_id,provider_campaign_id,first_observed_at,last_observed_at)
			VALUES($1,$2,$3,$3)RETURNING id`, f.seller, base+int64(i), f.cut.Add(-20*24*time.Hour)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO advertising_campaign_versions(
			seller_account_id,advertising_campaign_id,version_no,title,provider_state,campaign_family,currency_code,target_scope_type,
			state_hash,observed_at,source_payload_id,collection_domain_run_id)
			VALUES($1,$2,1,$3,'ACTIVE','PRODUCT_CPC','RUB','EXPLICIT_SKU_SET',$4,$5,$6,$7)`,
			f.seller, id, title, fmt.Sprintf("%064x", base+int64(i)), obs, payloadID, adsRun); err != nil {
			t.Fatal(err)
		}
		if i == targetAt {
			targetID = id
		}
	}
	return targetID
}

func prompt15InsertFillerCategories(t *testing.T, f prompt81Fixture, n int, targetAt int, targetName string) int64 {
	t.Helper()
	ctx := context.Background()
	obs := f.cut.Add(-30 * 24 * time.Hour)
	var taxSync int64
	if err := f.pool.QueryRow(ctx, `SELECT t.id FROM taxonomy_sync_runs t
		JOIN collection_domain_runs d ON d.id=t.collection_domain_run_id
		WHERE d.seller_account_id=$1 ORDER BY t.id DESC LIMIT 1`, f.seller).Scan(&taxSync); err != nil {
		t.Fatal(err)
	}
	var targetID int64
	base := time.Now().UnixNano()%1_000_000_000 + 70000
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("Filler Category %d", i)
		if i == targetAt {
			name = targetName
		}
		ext := base + int64(i)
		var pc, sc int64
		if err := f.pool.QueryRow(ctx, `INSERT INTO provider_categories(provider,provider_category_id,first_observed_at,last_observed_at)
			VALUES('OZON',$1,$2,$2)RETURNING id`, ext, f.cut.Add(-40*24*time.Hour)).Scan(&pc); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO provider_category_versions(
			provider_category_id,name,disabled,is_leaf,taxonomy_state,taxonomy_sync_run_id,observed_at,normalized_hash)
			VALUES($1,$2,false,true,'ACTIVE',$3,$4,$5)`,
			pc, name, taxSync, obs, fmt.Sprintf("%064x", ext)); err != nil {
			t.Fatal(err)
		}
		if err := f.pool.QueryRow(ctx, `INSERT INTO seller_categories(seller_account_id,provider_category_id,first_observed_at,last_observed_at)
			VALUES($1,$2,$3,$3)RETURNING id`, f.seller, pc, obs).Scan(&sc); err != nil {
			t.Fatal(err)
		}
		if i == targetAt {
			targetID = sc
		}
	}
	return targetID
}

func TestPrompt15SearchEntitiesSKUAfterFirst50(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	targetID := prompt15InsertFillerSKUs(t, f, 100, 79, "Prompt15 Search Target")
	api := prompt15SearchAPI(t, f)
	ctx := prompt15SealedFactual(t, f, pubID)
	page, err := api.ListSKUs(context.Background(), ctx, factualapi.SKUFilter{}, factualapi.PageRequest{PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, sku := range page.Items {
		if sku.ID == targetID {
			t.Fatal("old first-page ListSKUs must not include target at position >50")
		}
	}
	cnt := &countingFactual{API: api}
	found, err := cnt.SearchEntities(context.Background(), ctx, factualapi.EntitySearchRequest{
		Query: "Prompt15 Search Target", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cnt.searchCalls != 1 {
		t.Fatalf("SearchEntities calls=%d", cnt.searchCalls)
	}
	if cnt.listSKU != 0 {
		t.Fatalf("ListSKUs must not be used for search, got %d", cnt.listSKU)
	}
	if len(found.Items) == 0 || found.Items[0].CanonicalID != targetID {
		t.Fatalf("want canonical %d first got %+v", targetID, found.Items)
	}
}

func TestPrompt15SearchEntitiesCampaignAfterFirst50(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	targetID := prompt15InsertFillerCampaigns(t, f, 80, 70, "Prompt15 Search Campaign Target")
	api := prompt15SearchAPI(t, f)
	ctx := prompt15SealedFactual(t, f, pubID)
	found, err := api.SearchEntities(context.Background(), ctx, factualapi.EntitySearchRequest{
		Query: "Prompt15 Search Campaign Target", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchCampaign}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Items) == 0 || found.Items[0].CanonicalID != targetID {
		t.Fatalf("want campaign %d first got %+v", targetID, found.Items)
	}
}

func TestPrompt15SearchEntitiesCategoryAfterFirst50(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	targetID := prompt15InsertFillerCategories(t, f, 80, 70, "Prompt15 Search Category Target")
	api := prompt15SearchAPI(t, f)
	ctx := prompt15SealedFactual(t, f, pubID)
	found, err := api.SearchEntities(context.Background(), ctx, factualapi.EntitySearchRequest{
		Query: "Prompt15 Search Category Target", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchCategory}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Items) == 0 || found.Items[0].CanonicalID != targetID {
		t.Fatalf("want category %d first got %+v", targetID, found.Items)
	}
}

func TestPrompt15SearchEntitiesSellerIsolation(t *testing.T) {
	a, runA := prompt14FirstPublication(t)
	pubA := prompt14PubID(t, a, runA)
	b, runB := prompt14FirstPublication(t)
	pubB := prompt14PubID(t, b, runB)
	idA := prompt15InsertFillerSKUs(t, a, 1, 0, "Target A Unique Search")
	idB := prompt15InsertFillerSKUs(t, b, 1, 0, "Target B Unique Search")
	api := prompt15SearchAPI(t, a)
	found, err := api.SearchEntities(context.Background(), prompt15SealedFactual(t, a, pubA), factualapi.EntitySearchRequest{
		Query: "Target B Unique Search", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range found.Items {
		if it.CanonicalID == idB {
			t.Fatal("seller A search returned seller B SKU")
		}
	}
	foundA, err := api.SearchEntities(context.Background(), prompt15SealedFactual(t, a, pubA), factualapi.EntitySearchRequest{
		Query: "Target A Unique Search", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(foundA.Items) != 1 || foundA.Items[0].CanonicalID != idA {
		t.Fatalf("seller A target missing: %+v", foundA.Items)
	}
	foundB, err := api.SearchEntities(context.Background(), prompt15SealedFactual(t, b, pubB), factualapi.EntitySearchRequest{
		Query: "Target B Unique Search", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(foundB.Items) != 1 || foundB.Items[0].CanonicalID != idB {
		t.Fatalf("seller B target missing: %+v", foundB.Items)
	}
}

func TestPrompt15SearchEntitiesFrozenCutoff(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	ctxBG := context.Background()
	obsOld := f.cut.Add(-2 * time.Hour)
	var id int64
	if err := f.pool.QueryRow(ctxBG, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)
		VALUES($1,$2,$3,$3,'ACTIVE')RETURNING id`, f.seller, int64(93001), f.cut.Add(-10*24*time.Hour)).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctxBG, `INSERT INTO canonical_sku_versions(
		canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)
		VALUES($1,$2,'offer-cut','Polaris Frozen Old','active','ACTIVE',$3,$4,$5)`,
		id, int64(93001), fmt.Sprintf("%064x", 93001), obsOld, f.domainRun); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctxBG, `INSERT INTO canonical_sku_versions(
		canonical_sku_id,provider_product_id,offer_id,name,provider_status,catalog_state,normalized_hash,observed_at,collection_domain_run_id)
		VALUES($1,$2,'offer-cut','Polaris NEW Future UniqueRename','active','ACTIVE',$3,$4,$5)`,
		id, int64(93001), fmt.Sprintf("%064x", 93002), f.cut.Add(2*time.Hour), f.domainRun); err != nil {
		t.Fatal(err)
	}
	api := prompt15SearchAPI(t, f)
	ctx := prompt15SealedFactual(t, f, pubID)
	old, err := api.SearchEntities(ctxBG, ctx, factualapi.EntitySearchRequest{
		Query: "Polaris Frozen Old", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Items) != 1 || old.Items[0].CanonicalID != id {
		t.Fatalf("frozen old name missing: %+v", old.Items)
	}
	neu, err := api.SearchEntities(ctxBG, ctx, factualapi.EntitySearchRequest{
		Query: "NEW Future UniqueRename", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range neu.Items {
		if it.CanonicalID == id {
			t.Fatal("frozen search must not see future rename")
		}
	}
}

func TestPrompt15SearchEntitiesAmbiguous(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	prompt15InsertFillerSKUs(t, f, 1, 0, "Ambiguous Twin Widget")
	prompt15InsertFillerSKUs(t, f, 1, 0, "Ambiguous Twin Widget")
	api := prompt15SearchAPI(t, f)
	ex := &chatv2.ToolExecutor{API: api, Policy: chatv2.DefaultToolPolicy()}
	frozen := chatv2.FrozenTurnContext{HasFactual: true, Factual: prompt15SealedFactual(t, f, pubID)}
	out, err := ex.Execute(context.Background(), frozen, chatv2.NewGroundingRegistry(), chatv2.ToolSearchEntities, map[string]any{"query": "Ambiguous Twin Widget", "limit": 8})
	if err != nil {
		t.Fatal(err)
	}
	clarify, _ := out.Payload["needs_clarification"].(bool)
	matches, _ := out.Payload["matches"].([]map[string]any)
	if !clarify || len(matches) < 2 {
		t.Fatalf("expected ambiguity clarify=%v matches=%d", clarify, len(matches))
	}
}

func TestPrompt15SearchEntitiesNumericAndNaturalLanguage(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	prompt15Relabel(t, f)
	api := prompt15SearchAPI(t, f)
	ctx := prompt15SealedFactual(t, f, pubID)
	skuID := prompt15PolarisID(t, f)
	for _, q := range []string{"9105", "Polaris 417", "Какой остаток у Polaris 417?"} {
		found, err := api.SearchEntities(context.Background(), ctx, factualapi.EntitySearchRequest{
			Query: q, Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
		})
		if err != nil {
			t.Fatal(err)
		}
		ok := false
		for _, it := range found.Items {
			if it.CanonicalID == skuID {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("query %q did not find polaris canonical %d items=%+v", q, skuID, found.Items)
		}
	}
}

func TestPrompt15SearchEntitiesLargeCatalogPerformance(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	pubID := prompt14PubID(t, f, runID)
	targetID := prompt15InsertFillerSKUs(t, f, 400, 80, "Prompt15 Perf Search Target")
	api := prompt15SearchAPI(t, f)
	cnt := &countingFactual{API: api}
	ctx := prompt15SealedFactual(t, f, pubID)
	started := time.Now()
	found, err := cnt.SearchEntities(context.Background(), ctx, factualapi.EntitySearchRequest{
		Query: "Prompt15 Perf Search Target", Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU}, Limit: 8,
	})
	dur := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("PERF LARGE_ENTITY_SEARCH catalog+=400 duration=%s searchCalls=%d listSKU=%d skuGet=%d result=%d target=%d",
		dur, cnt.searchCalls, cnt.listSKU, cnt.skuGet, len(found.Items), targetID)
	if cnt.searchCalls != 1 || cnt.listSKU != 0 || cnt.skuGet != 0 {
		t.Fatalf("N+1 or page scan: search=%d list=%d get=%d", cnt.searchCalls, cnt.listSKU, cnt.skuGet)
	}
	if len(found.Items) == 0 || found.Items[0].CanonicalID != targetID {
		t.Fatalf("perf search miss first=%+v", found.Items)
	}
}

func TestPrompt15SearchPolarisP1P2ToolPreflight(t *testing.T) {
	f, r1, r2 := prompt15P1P2Stock(t)
	p1 := prompt14PubID(t, f, r1)
	p2 := prompt14PubID(t, f, r2)
	skuID := prompt15PolarisID(t, f)
	api := prompt15SearchAPI(t, f)
	ex := &chatv2.ToolExecutor{API: api, Policy: chatv2.DefaultToolPolicy()}
	for _, step := range []struct {
		pub   int64
		stock float64
	}{{p1, 0}, {p2, 300}} {
		frozen := chatv2.FrozenTurnContext{HasFactual: true, Factual: prompt15SealedFactual(t, f, step.pub)}
		search, err := ex.Execute(context.Background(), frozen, chatv2.NewGroundingRegistry(), chatv2.ToolSearchEntities, map[string]any{"query": "Какой остаток у Polaris 417?", "limit": nil})
		if err != nil || search.ErrorCode != "" {
			t.Fatalf("search pub=%d err=%v code=%s", step.pub, err, search.ErrorCode)
		}
		matches, _ := search.Payload["matches"].([]map[string]any)
		foundID := int64(0)
		for _, m := range matches {
			if m["kind"] == "SKU" {
				switch v := m["canonical_sku_id"].(type) {
				case int64:
					foundID = v
				case float64:
					foundID = int64(v)
				}
				break
			}
		}
		if foundID != skuID {
			t.Fatalf("pub=%d search canonical=%d want %d matches=%v", step.pub, foundID, skuID, matches)
		}
		st := prompt15DirectSellable(t, f, step.pub, skuID)
		if st != step.stock {
			t.Fatalf("pub=%d stock=%v want %v", step.pub, st, step.stock)
		}
		ctxs, err := ex.Execute(context.Background(), frozen, chatv2.NewGroundingRegistry(), chatv2.ToolGetSKUContexts, map[string]any{"sku_ids": []any{foundID}})
		if err != nil || ctxs.ErrorCode != "" {
			t.Fatalf("get_sku_contexts pub=%d err=%v code=%s", step.pub, err, ctxs.ErrorCode)
		}
		t.Logf("PREFLIGHT pub=%d sku=%d stock=%v search_ok get_sku_contexts_ok", step.pub, foundID, st)
	}
}
