//go:build integration

package sales

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
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSalesFinalizationRevisionReconciliationIsolationAndStaleProtection(t *testing.T) {
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
	if err = pool.Ping(ctx); err != nil {
		t.Skip(err)
	}
	var user, seller, otherSeller int64
	email := fmt.Sprintf("canonical_sales_%d@test.local", time.Now().UnixNano())
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, email).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'sales-one','active') RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	var otherUser int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, fmt.Sprintf("canonical_sales_other_%d@test.local", time.Now().UnixNano())).Scan(&otherUser); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, otherUser) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'sales-two','active') RETURNING id`, otherUser).Scan(&otherSeller); err != nil {
		t.Fatal(err)
	}
	var canonical int64
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state) VALUES($1,101,NOW(),NOW(),'ACTIVE') RETURNING id`, seller).Scan(&canonical); err != nil {
		t.Fatal(err)
	}
	var otherCanonical int64
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state) VALUES($1,101,NOW(),NOW(),'ACTIVE') RETURNING id`, otherSeller).Scan(&otherCanonical); err != nil {
		t.Fatal(err)
	}

	newDomain := func() int64 {
		var cr, dr int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','sales.integration') RETURNING id`, seller).Scan(&cr); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,requested_from,requested_to,started_at) VALUES($1,$2,'SALES_DEMAND','RUNNING','2026-08-01','2026-08-02',NOW()) RETURNING id`, cr, seller).Scan(&dr); e != nil {
			t.Fatal(e)
		}
		return dr
	}
	payload := func(dr int64, seq int64) int64 {
		var id int64
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile) VALUES($1,$2,'OZON','analytics','fingerprint',$3,'raw',$4,$5,200,NOW(),'OZON_SALES_DEMAND_REDACTION_V1') RETURNING id`, seller, dr, seq, fmt.Sprintf("sales/%d/%d", dr, seq), fmt.Sprintf("%064d", seq+1)).Scan(&id); e != nil {
			t.Fatal(e)
		}
		return id
	}
	amount := func(v string) Decimal {
		d, e := ParseAmount(json.Number(v))
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	day1, _ := time.Parse("2006-01-02", "2026-08-01")
	day2 := day1.AddDate(0, 0, 1)
	repo := NewRepository(pool)
	makeResult := func(dr int64) *Result {
		p1, p2 := payload(dr, 1), payload(dr, 2)
		return &Result{RequestedFrom: day1, RequestedTo: day2, Currency: "RUB", RecordsReceived: 4, Pages: 2, sku: []candidate{{101, day1, amount("8"), 8, p1, time.Now()}, {999, day1, amount("2"), 2, p1, time.Now()}}, account: []accountCandidate{{day1, amount("10"), 10, p2, time.Now()}, {day2, Zero(), 0, p2, time.Now()}}}
	}

	run1 := newDomain()
	r1 := makeResult(run1)
	if err = repo.Finalize(ctx, seller, run1, r1); err != nil {
		t.Fatal(err)
	}
	if r1.UnresolvedSKUs != 1 || r1.ReconciliationExplained != 1 || r1.ReconciliationPASS != 1 {
		t.Fatalf("result=%+v", r1)
	}
	var status string
	if err = pool.QueryRow(ctx, `SELECT status FROM collection_domain_runs WHERE id=$1`, run1).Scan(&status); err != nil || status != "PARTIAL" {
		t.Fatalf("status=%s err=%v", status, err)
	}
	var coverageFrom, coverageTo *time.Time
	if err = pool.QueryRow(ctx, `SELECT coverage_from,coverage_to FROM collection_domain_runs WHERE id=$1`, run1).Scan(&coverageFrom, &coverageTo); err != nil || coverageFrom == nil || coverageTo == nil || coverageFrom.Format("2006-01-02") != "2026-08-01" || coverageTo.Format("2006-01-02") != "2026-08-02" {
		t.Fatalf("continuous/explicit-zero coverage=%v..%v err=%v", coverageFrom, coverageTo, err)
	}
	var resolvedID *int64
	if err = pool.QueryRow(ctx, `SELECT canonical_sku_id FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=101`, seller).Scan(&resolvedID); err != nil || resolvedID == nil || *resolvedID != canonical {
		t.Fatalf("seller resolution=%v err=%v", resolvedID, err)
	}

	run2 := newDomain()
	r2 := makeResult(run2)
	if err = repo.Finalize(ctx, seller, run2, r2); err != nil {
		t.Fatal(err)
	}
	if r2.RevisionsWritten != 0 || r2.ReusedFacts != 4 {
		t.Fatalf("idempotency=%+v", r2)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM sales_demand_daily WHERE seller_account_id=$1`, seller).Scan(&count); err != nil || count != 2 {
		t.Fatalf("facts=%d err=%v", count, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state) VALUES($1,999,NOW(),NOW(),'ACTIVE')`, seller); err != nil {
		t.Fatal(err)
	}

	older := newDomain()
	newer := newDomain()
	rn := makeResult(newer)
	rn.sku[0].Amount = amount("9")
	if err = repo.Finalize(ctx, seller, newer, rn); err != nil {
		t.Fatal(err)
	}
	ro := makeResult(older)
	if err = repo.Finalize(ctx, seller, older, ro); err == nil {
		t.Fatal("stale run accepted")
	} else {
		var stale *SupersededRunError
		if !errors.As(err, &stale) {
			t.Fatalf("stale err=%v", err)
		}
	}
	if err = pool.QueryRow(ctx, `SELECT status FROM collection_domain_runs WHERE id=$1`, older).Scan(&status); err != nil || status != "CANCELLED" {
		t.Fatalf("older status=%s err=%v", status, err)
	}
	var maxRevision int64
	if err = pool.QueryRow(ctx, `SELECT max(revision_no) FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=101`, seller).Scan(&maxRevision); err != nil || maxRevision != 2 {
		t.Fatalf("revision=%d err=%v", maxRevision, err)
	}
	if err = pool.QueryRow(ctx, `SELECT max(revision_no) FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=999`, seller).Scan(&maxRevision); err != nil || maxRevision != 2 {
		t.Fatalf("resolution revision=%d err=%v", maxRevision, err)
	}
	missingRun := newDomain()
	missing := makeResult(missingRun)
	missing.sku[0].Amount = amount("9")
	missing.account = missing.account[:1]
	missing.RecordsReceived = 3
	if err = repo.Finalize(ctx, seller, missingRun, missing); err != nil {
		t.Fatal(err)
	}
	if missing.ReconciliationNotComparable != 1 {
		t.Fatalf("missing account reconciliation=%+v", missing)
	}
	concurrentA, concurrentB := newDomain(), newDomain()
	concurrentResult := func(dr int64) *Result {
		p := payload(dr, 20)
		return &Result{RequestedFrom: day1, RequestedTo: day1, Currency: "RUB", RecordsReceived: 2, Pages: 2, sku: []candidate{{555, day1, amount("1"), 1, p, time.Now()}}, account: []accountCandidate{{day1, amount("10"), 10, p, time.Now()}}}
	}
	results := []*Result{concurrentResult(concurrentA), concurrentResult(concurrentB)}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, dr := range []int64{concurrentA, concurrentB} {
		wg.Add(1)
		go func(i int, dr int64) { defer wg.Done(); errs[i] = repo.Finalize(ctx, seller, dr, results[i]) }(i, dr)
	}
	wg.Wait()
	if errs[0] != nil && errs[1] != nil {
		t.Fatalf("both concurrent finalizers failed: %v / %v", errs[0], errs[1])
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=555`, seller).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent duplicate count=%d err=%v finalizers=%v", count, err, errs)
	}

	// Historical hash reuse is valid knowledge history: SKU and Account A -> B -> A.
	acceptB := time.Date(2030, 8, 24, 12, 0, 0, 0, time.UTC)
	acceptAAgain := acceptB.Add(time.Hour)
	correction := newDomain()
	correctionResult := makeResult(correction)
	correctionResult.sku[0].Amount = amount("12")
	correctionResult.account[0].Amount = amount("11")
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptB }).Finalize(ctx, seller, correction, correctionResult); err != nil {
		t.Fatal(err)
	}
	repeatB := newDomain()
	repeatBResult := makeResult(repeatB)
	repeatBResult.sku[0].Amount = amount("12")
	repeatBResult.account[0].Amount = amount("11")
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptB.Add(30 * time.Minute) }).Finalize(ctx, seller, repeatB, repeatBResult); err != nil {
		t.Fatal(err)
	}
	if repeatBResult.RevisionsWritten != 0 || repeatBResult.ReusedFacts != 4 {
		t.Fatalf("A-B-B latest dedup=%+v", repeatBResult)
	}
	returnToA := newDomain()
	returnResult := makeResult(returnToA)
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptAAgain }).Finalize(ctx, seller, returnToA, returnResult); err != nil {
		t.Fatal(err)
	}
	var skuHashes, accountHashes []string
	var skuObserved, accountObserved []time.Time
	rows, err := pool.Query(ctx, `SELECT fact_hash,observed_at FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=101 AND business_date=$2 ORDER BY revision_no`, seller, day1)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var hash string
		var observed time.Time
		if err = rows.Scan(&hash, &observed); err != nil {
			t.Fatal(err)
		}
		skuHashes, skuObserved = append(skuHashes, hash), append(skuObserved, observed)
	}
	rows.Close()
	rows, err = pool.Query(ctx, `SELECT fact_hash,observed_at FROM sales_account_daily WHERE seller_account_id=$1 AND business_date=$2 ORDER BY revision_no`, seller, day1)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var hash string
		var observed time.Time
		if err = rows.Scan(&hash, &observed); err != nil {
			t.Fatal(err)
		}
		accountHashes, accountObserved = append(accountHashes, hash), append(accountObserved, observed)
	}
	rows.Close()
	if len(skuHashes) != 4 || skuHashes[0] != skuHashes[3] || skuHashes[2] == skuHashes[3] {
		t.Fatalf("SKU revision hashes=%v", skuHashes)
	}
	if len(accountHashes) != 3 || accountHashes[0] != accountHashes[2] || accountHashes[1] == accountHashes[2] {
		t.Fatalf("Account revision hashes=%v", accountHashes)
	}
	if !skuObserved[2].Equal(acceptB) || !accountObserved[1].Equal(acceptB) || !skuObserved[3].Equal(acceptAAgain) || !accountObserved[2].Equal(acceptAAgain) {
		t.Fatalf("acceptance timestamps sku=%v account=%v", skuObserved, accountObserved)
	}
	var resolutionStatuses []string
	var resolutionObserved []time.Time
	rows, err = pool.Query(ctx, `SELECT sku_resolution_status,observed_at FROM sales_demand_daily WHERE seller_account_id=$1 AND source_sku=999 AND business_date=$2 ORDER BY revision_no`, seller, day1)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var resolution string
		var observed time.Time
		if err = rows.Scan(&resolution, &observed); err != nil {
			t.Fatal(err)
		}
		resolutionStatuses, resolutionObserved = append(resolutionStatuses, resolution), append(resolutionObserved, observed)
	}
	rows.Close()
	if len(resolutionStatuses) != 2 || resolutionStatuses[0] != "UNRESOLVED" || resolutionStatuses[1] != "RESOLVED" || !resolutionObserved[1].After(resolutionObserved[0]) {
		t.Fatalf("late resolution statuses=%v observed=%v", resolutionStatuses, resolutionObserved)
	}
	// Repeating the latest A does not append another revision.
	repeat := newDomain()
	repeatResult := makeResult(repeat)
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptAAgain.Add(time.Hour) }).Finalize(ctx, seller, repeat, repeatResult); err != nil {
		t.Fatal(err)
	}
	if repeatResult.RevisionsWritten != 0 || repeatResult.ReusedFacts != 4 {
		t.Fatalf("latest-only dedup=%+v", repeatResult)
	}

	// Coverage is the outer bound of actual Account observations, while holes
	// remain explicit NOT_COMPARABLE reconciliation rows.
	coverageRun := newDomain()
	_, err = pool.Exec(ctx, `UPDATE collection_domain_runs SET requested_from='2026-08-01',requested_to='2026-08-10' WHERE id=$1`, coverageRun)
	if err != nil {
		t.Fatal(err)
	}
	coveragePayload := payload(coverageRun, 30)
	coverageResult := &Result{RequestedFrom: day1, RequestedTo: day1.AddDate(0, 0, 9), Currency: "RUB", Pages: 1}
	for _, n := range []int{2, 3, 4, 6, 7} { // observed Aug 3-5 and 7-8; Aug 6 is a hole
		coverageResult.account = append(coverageResult.account, accountCandidate{day1.AddDate(0, 0, n), Zero(), 0, coveragePayload, time.Now()})
	}
	coverageResult.RecordsReceived = len(coverageResult.account)
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptAAgain.Add(2 * time.Hour) }).Finalize(ctx, seller, coverageRun, coverageResult); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT coverage_from,coverage_to FROM collection_domain_runs WHERE id=$1`, coverageRun).Scan(&coverageFrom, &coverageTo); err != nil {
		t.Fatal(err)
	}
	if coverageFrom == nil || coverageTo == nil || coverageFrom.Format("2006-01-02") != "2026-08-03" || coverageTo.Format("2006-01-02") != "2026-08-08" {
		t.Fatalf("coverage=%v..%v", coverageFrom, coverageTo)
	}
	if err = pool.QueryRow(ctx, `SELECT overall_status FROM sales_reconciliations WHERE collection_domain_run_id=$1 AND business_date='2026-08-06'`, coverageRun).Scan(&status); err != nil || status != "NOT_COMPARABLE" {
		t.Fatalf("hole status=%s err=%v", status, err)
	}
	noAccountRun := newDomain()
	noAccountResult := &Result{RequestedFrom: day1, RequestedTo: day2, Currency: "RUB"}
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptAAgain.Add(3 * time.Hour) }).Finalize(ctx, seller, noAccountRun, noAccountResult); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT coverage_from,coverage_to FROM collection_domain_runs WHERE id=$1`, noAccountRun).Scan(&coverageFrom, &coverageTo); err != nil || coverageFrom != nil || coverageTo != nil {
		t.Fatalf("empty coverage=%v..%v err=%v", coverageFrom, coverageTo, err)
	}

	// Canonical facts/reconciliation share acceptance time; source receipt is
	// independent provenance and remains unchanged.
	var factObserved, reconciliationObserved, payloadReceived time.Time
	if err = pool.QueryRow(ctx, `SELECT observed_at FROM sales_demand_daily WHERE collection_domain_run_id=$1 LIMIT 1`, returnToA).Scan(&factObserved); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT observed_at FROM sales_reconciliations WHERE collection_domain_run_id=$1 LIMIT 1`, returnToA).Scan(&reconciliationObserved); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT received_at FROM source_payloads WHERE collection_domain_run_id=$1 ORDER BY id LIMIT 1`, returnToA).Scan(&payloadReceived); err != nil {
		t.Fatal(err)
	}
	if !factObserved.Equal(acceptAAgain) || !reconciliationObserved.Equal(acceptAAgain) || payloadReceived.Equal(acceptAAgain) {
		t.Fatalf("knowledge/provenance fact=%s reconciliation=%s source=%s", factObserved, reconciliationObserved, payloadReceived)
	}

	badRun := newDomain()
	badPayload := payload(badRun, 9)
	if _, err = pool.Exec(ctx, `INSERT INTO sales_demand_daily(seller_account_id,canonical_sku_id,source_sku,business_date,ordered_amount,ordered_units,currency_code,sku_resolution_status,revision_no,fact_hash,collection_domain_run_id,source_payload_id,observed_at) VALUES($1,$2,101,'2026-08-01',1,1,'RUB','RESOLVED',99,$3,$4,$5,NOW())`, seller, otherCanonical, fmt.Sprintf("%064d", 99), badRun, badPayload); err == nil {
		t.Fatal("cross-seller canonical SKU accepted")
	}
}

func TestSalesSameRunResumeFromPersistedSourcePayloadsAndObjectStorage(t *testing.T) {
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
	if err = pool.Ping(ctx); err != nil {
		t.Skip(err)
	}

	var userID, sellerID int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, fmt.Sprintf("sales_resume_%d@test.local", time.Now().UnixNano())).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'sales-resume','active') RETURNING id`, userID).Scan(&sellerID); err != nil {
		t.Fatal(err)
	}

	foundation := collection.NewRepository(dbgen.New(pool))
	lifecycle := collection.NewService(foundation)
	d, _ := time.Parse("2006-01-02", "2026-08-01")
	run, err := foundation.CreateRun(ctx, sellerID, collection.CollectionRunKindManual, "sales.resume.integration", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.TransitionRun(ctx, sellerID, run.ID, collection.CollectionRunStatusRunning); err != nil {
		t.Fatal(err)
	}
	domain, err := foundation.CreateDomainRunWithContract(ctx, run.ID, sellerID, collection.DomainSalesDemand, &DomainSourceContract, &d, &d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = lifecycle.TransitionDomainRun(ctx, sellerID, domain.ID, collection.DomainRunStatusRunning); err != nil {
		t.Fatal(err)
	}

	var payloadStore *collection.SourcePayloadStore
	if endpoint := os.Getenv("TEST_MINIO_ENDPOINT"); endpoint != "" {
		client, storageErr := storage.New(ctx, storage.S3Config{Endpoint: endpoint, AccessKey: "minio", SecretKey: "minio123", BucketRaw: "raw-payloads", BucketExports: "exports", BucketArtifacts: "artifacts"})
		if storageErr != nil {
			t.Fatal(storageErr)
		}
		payloadStore = collection.NewSourcePayloadStore(client, "raw-payloads")
	} else {
		payloadStore = collection.NewSourcePayloadStoreWithObjects(&replayObjects{}, "raw")
	}
	firstAPI := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 && req.Offset < 3000 {
			return response(skuPage(req.Offset, PageSize)), nil
		}
		return nil, ozon.ErrNetworkError
	}}
	_, err = (&Runner{API: firstAPI, Collection: lifecycle, Payloads: payloadStore, Cursors: lifecycle, Recovery: foundation}).Collect(ctx, sellerID, domain.ID, d, d)
	if !errors.Is(err, ozon.ErrNetworkError) {
		t.Fatalf("interruption=%v", err)
	}
	var payloadCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM source_payloads WHERE collection_domain_run_id=$1`, domain.ID).Scan(&payloadCount); err != nil || payloadCount != 3 {
		t.Fatalf("prefix payloads=%d err=%v", payloadCount, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE collection_domain_runs SET error_code='SALES_COLLECTION_INTERRUPTED',error_message='test interruption' WHERE id=$1`, domain.ID); err != nil {
		t.Fatal(err)
	}

	resumeAPI := &fakeAPI{fn: func(req ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		if len(req.Dimension) == 2 {
			return response(skuPage(req.Offset, 1)), nil
		}
		return response([]ozon.CanonicalAnalyticsRow{row([]string{"2026-08-01"}, "4504501", "3001")}), nil
	}}
	result, err := (&Runner{API: resumeAPI, Collection: lifecycle, Payloads: payloadStore, Cursors: lifecycle, Recovery: foundation}).Collect(ctx, sellerID, domain.ID, d, d)
	if err != nil {
		t.Fatal(err)
	}
	if len(resumeAPI.requests) != 2 || resumeAPI.requests[0].Offset != 3000 || resumeAPI.requests[1].Offset != 0 {
		t.Fatalf("resume requests=%+v", resumeAPI.requests)
	}
	terminalAPI := &fakeAPI{fn: func(ozon.AnalyticsDataRequest) (*ozon.CanonicalAnalyticsResponse, error) {
		return nil, errors.New("terminal replay must not call provider")
	}}
	replayedAgain, err := (&Runner{API: terminalAPI, Collection: lifecycle, Payloads: payloadStore, Cursors: lifecycle, Recovery: foundation}).Collect(ctx, sellerID, domain.ID, d, d)
	if err != nil || len(terminalAPI.requests) != 0 || replayedAgain.SKURecords != result.SKURecords || replayedAgain.AccountRecords != result.AccountRecords {
		t.Fatalf("terminal replay err=%v requests=%d result=%+v", err, len(terminalAPI.requests), replayedAgain)
	}
	acceptance := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptance }).Finalize(ctx, sellerID, domain.ID, &result); err != nil {
		t.Fatal(err)
	}
	if err = NewRepositoryWithClock(pool, func() time.Time { return acceptance.Add(time.Hour) }).Finalize(ctx, sellerID, domain.ID, &replayedAgain); err != nil {
		t.Fatalf("idempotent refinalization: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM source_payloads WHERE collection_domain_run_id=$1`, domain.ID).Scan(&payloadCount); err != nil || payloadCount != 5 {
		t.Fatalf("final payloads=%d err=%v", payloadCount, err)
	}
	var skuFacts, accountFacts, maxSKURevision int
	var accountAmount string
	if err = pool.QueryRow(ctx, `SELECT count(*),max(revision_no) FROM sales_demand_daily WHERE seller_account_id=$1`, sellerID).Scan(&skuFacts, &maxSKURevision); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*),max(ordered_amount)::text FROM sales_account_daily WHERE seller_account_id=$1`, sellerID).Scan(&accountFacts, &accountAmount); err != nil {
		t.Fatal(err)
	}
	if skuFacts != 3001 || maxSKURevision != 1 || accountFacts != 1 || accountAmount != "4504501.000000" {
		t.Fatalf("canonical sku=%d rev=%d account=%d/%s", skuFacts, maxSKURevision, accountFacts, accountAmount)
	}
	var errorCode *string
	if err = pool.QueryRow(ctx, `SELECT error_code FROM collection_domain_runs WHERE id=$1`, domain.ID).Scan(&errorCode); err != nil || errorCode != nil {
		t.Fatalf("successful resume retained error_code=%v err=%v", errorCode, err)
	}
}
