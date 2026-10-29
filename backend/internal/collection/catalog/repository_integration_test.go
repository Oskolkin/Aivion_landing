//go:build integration

package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCanonicalCatalogRepositoryIdentityVersioningCategoryAndFinalization(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run canonical catalog integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	newSeller := func(suffix string) int64 {
		var userID, sellerID int64
		email := fmt.Sprintf("canonical_catalog_%s_%d@test.local", suffix, time.Now().UnixNano())
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&userID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
		if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,$2,'active') RETURNING id`, userID, suffix).Scan(&sellerID); err != nil {
			t.Fatal(err)
		}
		return sellerID
	}
	newDomain := func(sellerID int64, domain collection.CollectionDomain) int64 {
		var runID, domainID int64
		if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','catalog.integration') RETURNING id`, sellerID).Scan(&runID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at) VALUES($1,$2,$3,'RUNNING',NOW()) RETURNING id`, runID, sellerID, string(domain)).Scan(&domainID); err != nil {
			t.Fatal(err)
		}
		return domainID
	}

	seller1 := newSeller("one")
	seller2 := newSeller("two")
	taxonomyRun := newDomain(seller1, collection.DomainTaxonomy)
	catalogRun1 := newDomain(seller1, collection.DomainCatalog)
	catalogRun2 := newDomain(seller2, collection.DomainCatalog)
	repo := NewRepositoryWithPool(pool)

	typeNode, _ := json.Marshal(ozon.DescriptionCategoryTreeJSONNode{TypeID: 900, TypeName: "Type"})
	leafNode, _ := json.Marshal(ozon.DescriptionCategoryTreeJSONNode{DescriptionCategoryID: 20, CategoryName: "Leaf", Children: []json.RawMessage{typeNode}})
	tree := []ozon.DescriptionCategoryTreeJSONNode{{DescriptionCategoryID: 10, CategoryName: "Parent", Children: []json.RawMessage{leafNode}}}
	if _, err := repo.PersistTaxonomy(ctx, seller1, taxonomyRun, tree, now); err != nil {
		t.Fatal(err)
	}

	base := SKUObservation{OzonSKU: 777, ProductID: 10, OfferID: "OLD", Name: "Name", State: "ACTIVE", CategoryID: 20, ProductTypeID: 900}
	id1, changed, err := repo.PersistSKU(ctx, seller1, catalogRun1, base, now)
	if err != nil || !changed {
		t.Fatalf("first persist: id=%d changed=%v err=%v", id1, changed, err)
	}
	if _, changed, err = repo.PersistSKU(ctx, seller1, catalogRun1, base, now.Add(time.Second)); err != nil || changed {
		t.Fatalf("unchanged metadata/content: changed=%v err=%v", changed, err)
	}
	changedObservation := base
	changedObservation.OfferID = "NEW"
	changedObservation.ProductID = 11
	if id, changed, err := repo.PersistSKU(ctx, seller1, catalogRun1, changedObservation, now.Add(2*time.Second)); err != nil || !changed || id != id1 {
		t.Fatalf("mutable metadata created wrong identity: id=%d changed=%v err=%v", id, changed, err)
	}
	id2, _, err := repo.PersistSKU(ctx, seller2, catalogRun2, base, now)
	if err != nil || id2 == id1 {
		t.Fatalf("seller-scoped identity: seller1=%d seller2=%d err=%v", id1, id2, err)
	}
	var versionCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canonical_sku_versions WHERE canonical_sku_id=$1`, id1).Scan(&versionCount); err != nil || versionCount != 2 {
		t.Fatalf("version count=%d err=%v", versionCount, err)
	}

	if changed, err := repo.PersistAssignment(ctx, seller1, id1, catalogRun1, base, now, true); err != nil || !changed {
		t.Fatalf("leaf assignment: changed=%v err=%v", changed, err)
	}
	if changed, err := repo.PersistAssignment(ctx, seller1, id1, catalogRun1, base, now.Add(time.Second), true); err != nil || changed {
		t.Fatalf("unchanged assignment duplicated: changed=%v err=%v", changed, err)
	}
	unresolvedType := base
	unresolvedType.ProductTypeID = 999999
	if changed, err := repo.PersistAssignment(ctx, seller1, id1, catalogRun1, unresolvedType, now.Add(1500*time.Millisecond), true); err != nil || !changed {
		t.Fatalf("unresolved product type assignment: changed=%v err=%v", changed, err)
	}
	var categoryResolution, productTypeResolution string
	if err := pool.QueryRow(ctx, `SELECT category_resolution_status,product_type_resolution_status FROM sku_category_assignments WHERE canonical_sku_id=$1 AND observed_to IS NULL`, id1).Scan(&categoryResolution, &productTypeResolution); err != nil || categoryResolution != "RESOLVED" || productTypeResolution != "UNRESOLVED" {
		t.Fatalf("independent resolution category=%q product_type=%q err=%v", categoryResolution, productTypeResolution, err)
	}
	parent := base
	parent.CategoryID = 10
	if changed, err := repo.PersistAssignment(ctx, seller1, id1, catalogRun1, parent, now.Add(2*time.Second), true); err != nil || !changed {
		t.Fatalf("parent must become UNMAPPED: changed=%v err=%v", changed, err)
	}
	var assignments, openAssignments, sellerCategories int
	if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE observed_to IS NULL) FROM sku_category_assignments WHERE canonical_sku_id=$1`, id1).Scan(&assignments, &openAssignments); err != nil || assignments != 3 || openAssignments != 1 {
		t.Fatalf("assignment history=%d open=%d err=%v", assignments, openAssignments, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM seller_categories WHERE seller_account_id=$1`, seller1).Scan(&sellerCategories); err != nil || sellerCategories != 1 {
		t.Fatalf("only leaf seller category expected: count=%d err=%v", sellerCategories, err)
	}
	taxonomyRun2 := newDomain(seller1, collection.DomainTaxonomy)
	if _, err := repo.PersistTaxonomy(ctx, seller1, taxonomyRun2, []ozon.DescriptionCategoryTreeJSONNode{{DescriptionCategoryID: 10, CategoryName: "Parent"}}, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	var missingState string
	if err := pool.QueryRow(ctx, `SELECT v.taxonomy_state FROM provider_category_versions v JOIN provider_categories c ON c.id=v.provider_category_id WHERE c.provider_category_id=20 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1`).Scan(&missingState); err != nil || missingState != "UNOBSERVED" {
		t.Fatalf("missing taxonomy state=%q err=%v", missingState, err)
	}
	var sellerCategoryID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM seller_categories WHERE seller_account_id=$1`, seller1).Scan(&sellerCategoryID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sku_category_assignments(canonical_sku_id,seller_category_id,raw_provider_category_id,mapping_status,observed_from,collection_domain_run_id) VALUES($1,$2,20,'MAPPED',NOW(),$3)`, id2, sellerCategoryID, catalogRun2); err == nil {
		t.Fatal("database accepted cross-seller category assignment")
	}

	archived := SKUObservation{OzonSKU: 888, ProductID: 12, OfferID: "ARCHIVED", Name: "Archived", State: "ARCHIVED", Archived: true, CategoryID: 20, ProductTypeID: 900}
	result := Result{
		RecordsReceived: 1,
		Pages:           2,
		ObservedSKUs:    []int64{888},
		observations:    []SKUObservation{archived},
		taxonomyReady:   true,
	}
	if err := repo.FinalizeCompleteRun(ctx, seller1, catalogRun1, &result, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(pool)
	row, err := q.GetCanonicalSKUForSeller(ctx, dbgen.GetCanonicalSKUForSellerParams{ID: id1, SellerAccountID: seller1})
	if err != nil || row.CatalogState != "UNOBSERVED" || result.MarkedUnobserved != 1 {
		t.Fatalf("complete reconciliation: row=%+v result=%+v err=%v", row, result, err)
	}
	var archivedState, domainStatus, completeness string
	if err := pool.QueryRow(ctx, `SELECT catalog_state FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=888`, seller1).Scan(&archivedState); err != nil || archivedState != "ARCHIVED" {
		t.Fatalf("ARCHIVED stream SKU was not preserved: state=%q err=%v", archivedState, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status,completeness_status FROM collection_domain_runs WHERE id=$1`, catalogRun1).Scan(&domainStatus, &completeness); err != nil || domainStatus != "COMPLETED" || completeness != "COMPLETE" {
		t.Fatalf("finalization state status=%q completeness=%q err=%v", domainStatus, completeness, err)
	}
	if _, err := q.GetCanonicalSKUForSeller(ctx, dbgen.GetCanonicalSKUForSellerParams{ID: id2, SellerAccountID: seller1}); err == nil {
		t.Fatal("seller 1 read seller 2 SKU")
	}
}

func TestCompleteCatalogFixturePersistsAllArchivedUnionAndLeafAssignments(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run canonical catalog integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}

	var userID, sellerID, parentRunID, taxonomyRunID, catalogRunID int64
	email := fmt.Sprintf("catalog_full_fixture_%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'catalog-full-fixture','active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','catalog.full.integration') RETURNING id`, sellerID).Scan(&parentRunID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at) VALUES($1,$2,'TAXONOMY','RUNNING',NOW()) RETURNING id`, parentRunID, sellerID).Scan(&taxonomyRunID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at) VALUES($1,$2,'CATALOG','RUNNING',NOW()) RETURNING id`, parentRunID, sellerID).Scan(&catalogRunID); err != nil {
		t.Fatal(err)
	}

	typeX, _ := json.Marshal(ozon.DescriptionCategoryTreeJSONNode{TypeID: 900, TypeName: "Type X"})
	typeY, _ := json.Marshal(ozon.DescriptionCategoryTreeJSONNode{TypeID: 901, TypeName: "Type Y"})
	leafX, _ := json.Marshal(ozon.DescriptionCategoryTreeJSONNode{DescriptionCategoryID: 20, CategoryName: "Leaf X", Children: []json.RawMessage{typeX}})
	leafY, _ := json.Marshal(ozon.DescriptionCategoryTreeJSONNode{DescriptionCategoryID: 30, CategoryName: "Leaf Y", Children: []json.RawMessage{typeY}})
	tree := []ozon.DescriptionCategoryTreeJSONNode{{DescriptionCategoryID: 10, CategoryName: "Parent", Children: []json.RawMessage{leafX, leafY}}}
	repo := NewRepositoryWithPool(pool)
	if _, err := repo.PersistTaxonomy(ctx, sellerID, taxonomyRunID, tree, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	details := map[int64]ozon.ProductInfoListItem{
		101: {ID: 101, SKU: 10001, DescriptionCategoryID: 20, TypeID: 900},
		102: {ID: 102, SKU: 10002, DescriptionCategoryID: 20, TypeID: 900},
		103: {ID: 103, SKU: 10003, DescriptionCategoryID: 999, TypeID: 999},
		104: {ID: 104, SKU: 10004, DescriptionCategoryID: 30, TypeID: 901, Archived: true},
	}
	api := &scriptedAPI{}
	api.list = func(request ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error) {
		switch request.Filter.Visibility + ":" + request.LastID {
		case "ALL:":
			return productListResponse([]ozon.ProductItem{{ProductID: 101, SKU: 10001}, {ProductID: 102, SKU: 10002}}, "all-next"), nil
		case "ALL:all-next":
			return productListResponse([]ozon.ProductItem{{ProductID: 103, SKU: 10003}}, ""), nil
		case "ARCHIVED:":
			return productListResponse([]ozon.ProductItem{{ProductID: 104, SKU: 10004, Archived: true}}, ""), nil
		default:
			return nil, fmt.Errorf("unexpected list request: %+v", request)
		}
	}
	api.info = func(request ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
		items := make([]ozon.ProductInfoListItem, 0, len(request.ProductID))
		for _, productID := range request.ProductID {
			items = append(items, details[productID])
		}
		return &ozon.TypedResponse[ozon.ProductInfoListResult]{Data: ozon.ProductInfoListResult{Items: items}}, nil
	}

	result, err := (&Runner{API: api, TaxonomyReady: true}).RunCatalog(ctx, sellerID, catalogRunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.FinalizeCompleteRun(ctx, sellerID, catalogRunID, &result, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `SELECT ozon_sku,catalog_state FROM canonical_skus WHERE seller_account_id=$1 ORDER BY ozon_sku`, sellerID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	states := map[int64]string{}
	for rows.Next() {
		var sku int64
		var state string
		if err := rows.Scan(&sku, &state); err != nil {
			t.Fatal(err)
		}
		states[sku] = state
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantStates := map[int64]string{10001: "ACTIVE", 10002: "ACTIVE", 10003: "ACTIVE", 10004: "ARCHIVED"}
	if fmt.Sprint(states) != fmt.Sprint(wantStates) {
		t.Fatalf("canonical union states=%v want=%v", states, wantStates)
	}
	var sellerCategories, parentTargets, unmappedC int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM seller_categories WHERE seller_account_id=$1`, sellerID).Scan(&sellerCategories); err != nil || sellerCategories != 2 {
		t.Fatalf("leaf seller categories=%d err=%v", sellerCategories, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM seller_categories sc JOIN provider_categories pc ON pc.id=sc.provider_category_id WHERE sc.seller_account_id=$1 AND pc.provider_category_id=10`, sellerID).Scan(&parentTargets); err != nil || parentTargets != 0 {
		t.Fatalf("parent seller category targets=%d err=%v", parentTargets, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sku_category_assignments a JOIN canonical_skus s ON s.id=a.canonical_sku_id WHERE s.seller_account_id=$1 AND s.ozon_sku=10003 AND a.observed_to IS NULL AND a.mapping_status='UNMAPPED'`, sellerID).Scan(&unmappedC); err != nil || unmappedC != 1 {
		t.Fatalf("unknown category SKU assignment=%d err=%v", unmappedC, err)
	}
}

func TestCatalogFinalizationOrderingSupersessionAndIdempotence(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run canonical catalog integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}

	newSeller := func(label string) (int64, int64) {
		var userID, sellerID int64
		email := fmt.Sprintf("catalog_ordering_%s_%d@test.local", label, time.Now().UnixNano())
		if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&userID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
		if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,$2,'active') RETURNING id`, userID, label).Scan(&sellerID); err != nil {
			t.Fatal(err)
		}
		return userID, sellerID
	}
	newRun := func(sellerID int64) int64 {
		var parentID, domainID int64
		if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','catalog.ordering.integration') RETURNING id`, sellerID).Scan(&parentID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at) VALUES($1,$2,'CATALOG','RUNNING',NOW()) RETURNING id`, parentID, sellerID).Scan(&domainID); err != nil {
			t.Fatal(err)
		}
		return domainID
	}
	result := func(state string, productID int64) Result {
		return Result{RecordsReceived: 1, Pages: 2, ObservedSKUs: []int64{50001}, observations: []SKUObservation{{OzonSKU: 50001, ProductID: productID, OfferID: state, Name: state, State: state, Archived: state == "ARCHIVED"}}}
	}
	repo := NewRepositoryWithPool(pool)

	_, sellerNewFirst := newSeller("new-first")
	oldRun := newRun(sellerNewFirst)
	newRunID := newRun(sellerNewFirst)
	newResult := result("ACTIVE", 200)
	if err := repo.FinalizeCompleteRun(ctx, sellerNewFirst, newRunID, &newResult, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	oldResult := result("ARCHIVED", 100)
	err = repo.FinalizeCompleteRun(ctx, sellerNewFirst, oldRun, &oldResult, time.Now().UTC().Add(time.Second))
	var superseded *SupersededRunError
	if !errors.As(err, &superseded) || superseded.SupersededBy != newRunID {
		t.Fatalf("old finalization error=%v", err)
	}
	var state, oldStatus, oldCode string
	if err := pool.QueryRow(ctx, `SELECT catalog_state FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=50001`, sellerNewFirst).Scan(&state); err != nil || state != "ACTIVE" {
		t.Fatalf("new state overwritten: state=%q err=%v", state, err)
	}
	if err := pool.QueryRow(ctx, `SELECT status,error_code FROM collection_domain_runs WHERE id=$1`, oldRun).Scan(&oldStatus, &oldCode); err != nil || oldStatus != "CANCELLED" || oldCode != "SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN" {
		t.Fatalf("old run status=%q code=%q err=%v", oldStatus, oldCode, err)
	}

	_, sellerOldFirst := newSeller("old-first")
	firstRun := newRun(sellerOldFirst)
	secondRun := newRun(sellerOldFirst)
	firstResult := result("ARCHIVED", 100)
	if err := repo.FinalizeCompleteRun(ctx, sellerOldFirst, firstRun, &firstResult, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	secondResult := result("ACTIVE", 200)
	if err := repo.FinalizeCompleteRun(ctx, sellerOldFirst, secondRun, &secondResult, time.Now().UTC().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var versionsBefore, versionsAfter int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canonical_sku_versions v JOIN canonical_skus s ON s.id=v.canonical_sku_id WHERE s.seller_account_id=$1 AND s.ozon_sku=50001`, sellerOldFirst).Scan(&versionsBefore); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinalizeCompleteRun(ctx, sellerOldFirst, secondRun, &secondResult, time.Now().UTC().Add(2*time.Second)); err != nil {
		t.Fatalf("idempotent finalize: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM canonical_sku_versions v JOIN canonical_skus s ON s.id=v.canonical_sku_id WHERE s.seller_account_id=$1 AND s.ozon_sku=50001`, sellerOldFirst).Scan(&versionsAfter); err != nil || versionsAfter != versionsBefore {
		t.Fatalf("retry changed versions: before=%d after=%d err=%v", versionsBefore, versionsAfter, err)
	}
	if err := pool.QueryRow(ctx, `SELECT catalog_state FROM canonical_skus WHERE seller_account_id=$1 AND ozon_sku=50001`, sellerOldFirst).Scan(&state); err != nil || state != "ACTIVE" {
		t.Fatalf("newer state=%q err=%v", state, err)
	}
}

func TestConcurrentTaxonomyRefreshKeepsNewestGlobalHead(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run canonical catalog integration tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("integration database unavailable: %v", err)
	}
	var userID, sellerID int64
	email := fmt.Sprintf("taxonomy_concurrency_%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'taxonomy-concurrency','active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatal(err)
	}
	newTaxonomyRun := func() int64 {
		var parentID, domainID int64
		if err := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','taxonomy.concurrent') RETURNING id`, sellerID).Scan(&parentID); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at) VALUES($1,$2,'TAXONOMY','RUNNING',NOW()) RETURNING id`, parentID, sellerID).Scan(&domainID); err != nil {
			t.Fatal(err)
		}
		return domainID
	}
	oldRun, newRunID := newTaxonomyRun(), newTaxonomyRun()
	repo := NewRepositoryWithPool(pool)
	start := make(chan struct{})
	var wg sync.WaitGroup
	errorsByRun := make(map[int64]error)
	var mu sync.Mutex
	for runID, name := range map[int64]string{oldRun: "Old", newRunID: "New"} {
		wg.Add(1)
		go func(runID int64, name string) {
			defer wg.Done()
			<-start
			_, persistErr := repo.PersistTaxonomy(ctx, sellerID, runID, []ozon.DescriptionCategoryTreeJSONNode{{DescriptionCategoryID: 980001, CategoryName: name}}, time.Now().UTC())
			mu.Lock()
			errorsByRun[runID] = persistErr
			mu.Unlock()
		}(runID, name)
	}
	close(start)
	wg.Wait()
	if errorsByRun[newRunID] != nil {
		t.Fatalf("new taxonomy run failed: %v", errorsByRun[newRunID])
	}
	if oldErr := errorsByRun[oldRun]; oldErr != nil {
		var superseded *SupersededTaxonomyRunError
		if !errors.As(oldErr, &superseded) || superseded.SupersededBy != newRunID {
			t.Fatalf("old taxonomy error=%v", oldErr)
		}
	}
	var head int64
	var latestName string
	if err := pool.QueryRow(ctx, `SELECT collection_domain_run_id FROM provider_taxonomy_heads WHERE provider='OZON' AND market='RU'`).Scan(&head); err != nil || head != newRunID {
		t.Fatalf("taxonomy head=%d want=%d err=%v", head, newRunID, err)
	}
	if err := pool.QueryRow(ctx, `SELECT v.name FROM provider_category_versions v JOIN provider_categories c ON c.id=v.provider_category_id WHERE c.provider_category_id=980001 ORDER BY v.observed_at DESC,v.id DESC LIMIT 1`).Scan(&latestName); err != nil || latestName != "New" {
		t.Fatalf("latest taxonomy name=%q err=%v", latestName, err)
	}
}
