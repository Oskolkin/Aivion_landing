//go:build integration

package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCanonicalLifecycleIdentityVersionTemporalLineageReturnsStaleAndDemandIsolation(t *testing.T) {
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
	defer pool.Close()
	if e = pool.Ping(ctx); e != nil {
		t.Skip(e)
	}
	var user, seller int64
	e = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, fmt.Sprintf("lifecycle_%d@test.local", time.Now().UnixNano())).Scan(&user)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	e = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'lifecycle','active') RETURNING id`, user).Scan(&seller)
	if e != nil {
		t.Fatal(e)
	}
	var sku101 int64
	e = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state) VALUES($1,101,NOW(),NOW(),'ACTIVE') RETURNING id`, seller).Scan(&sku101)
	if e != nil {
		t.Fatal(e)
	}
	day := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newDomain := func(domain string) (int64, int64) {
		var parent, run int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger) VALUES($1,'MANUAL','RUNNING','lifecycle.integration') RETURNING id`, seller).Scan(&parent); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,requested_from,requested_to,started_at) VALUES($1,$2,$3,'RUNNING',$4,$4,NOW()) RETURNING id`, parent, seller, domain, day).Scan(&run); e != nil {
			t.Fatal(e)
		}
		return parent, run
	}
	payload := func(run int64, domain string) (int64, time.Time) {
		profile, endpoint := "OZON_POSTINGS_FBS_REDACTION_V1", FBSListEndpoint
		if domain == "RETURNS" {
			profile, endpoint = "OZON_RETURNS_REDACTION_V1", ReturnsEndpoint
		}
		received := time.Date(2026, 8, 1, 10, 0, int(run%50), 0, time.UTC)
		var id int64
		e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,redaction_profile) VALUES($1,$2,'OZON',$3,$4,1,'raw',$5,$6,200,$7,$8) RETURNING id`, seller, run, endpoint, fmt.Sprintf("fp-%d", run), fmt.Sprintf("lifecycle/%d", run), fmt.Sprintf("%064d", run), received, profile).Scan(&id)
		if e != nil {
			t.Fatal(e)
		}
		return id, received
	}
	money := func(v string) sales.Decimal {
		d, e := sales.ParseAmount(json.Number(v))
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	postingResult := func(run int64, status, qty string, parent string) PostingsResult {
		p, received := payload(run, "POSTINGS")
		q := int64(3)
		if qty == "2" {
			q = 2
		}
		parentCandidate := PostingCandidate{OrderID: 1, OrderNumber: "ORDER-1", PostingNumber: "PARENT", Scheme: "FBS", Status: "awaiting_deliver", ProviderCreatedAt: timePtr(time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)), Products: []ProductCandidate{{101, "known", "Known", 3, money("12.345678"), "RUB", p}}, PayloadID: p, SourceReceivedAt: received}
		parentCandidate.StateHash = postingState(parentCandidate)
		child := PostingCandidate{OrderID: 1, OrderNumber: "ORDER-1", PostingNumber: "CHILD", Scheme: "FBS", Status: status, ParentPostingNumber: parent, Products: []ProductCandidate{{999, "unknown", "Unknown", q, money("1"), "RUB", p}}, PayloadID: p, SourceReceivedAt: received}
		child.StateHash = postingState(child)
		return PostingsResult{RequestedFrom: day, RequestedTo: day, FBO: StreamResult{Status: "COMPLETE"}, FBS: StreamResult{Status: "COMPLETE", Pages: 1, Records: 2}, Postings: 2, postings: []PostingCandidate{parentCandidate, child}}
	}
	repoAt := func(at time.Time) *Repository { return NewRepositoryWithClock(pool, func() time.Time { return at }) }
	t1 := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	_, run1 := newDomain("POSTINGS")
	r1 := postingResult(run1, "awaiting_deliver", "3", "PARENT")
	if e = repoAt(t1).FinalizePostings(ctx, seller, run1, &r1); e != nil {
		t.Fatal(e)
	}
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM sales_orders WHERE seller_account_id=$1`, seller).Scan(&count); e != nil || count != 1 {
		t.Fatalf("orders=%d err=%v", count, e)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM sales_postings WHERE seller_account_id=$1`, seller).Scan(&count); e != nil || count != 2 {
		t.Fatalf("postings=%d err=%v", count, e)
	}
	var acceptance time.Time
	var distinctAcceptance int
	var sourceReceiptSeparate bool
	if e = pool.QueryRow(ctx, `SELECT count(DISTINCT observed_at),min(observed_at),bool_and(source_received_at<>observed_at) FROM sales_posting_versions WHERE seller_account_id=$1 AND collection_domain_run_id=$2`, seller, run1).Scan(&distinctAcceptance, &acceptance, &sourceReceiptSeparate); e != nil || distinctAcceptance != 1 || !acceptance.Equal(t1) || !sourceReceiptSeparate {
		t.Fatalf("acceptance semantics distinct=%d at=%v source_separate=%v err=%v", distinctAcceptance, acceptance, sourceReceiptSeparate, e)
	}
	var createdQuality string
	var createdEffective *time.Time
	if e = pool.QueryRow(ctx, `SELECT time_quality,effective_at FROM sales_lifecycle_events WHERE seller_account_id=$1 AND event_type='POSTING_CREATED' ORDER BY id LIMIT 1`, seller).Scan(&createdQuality, &createdEffective); e != nil || createdQuality != "PROVIDER_EXACT" || createdEffective == nil {
		t.Fatalf("posting created temporal=%s/%v err=%v", createdQuality, createdEffective, e)
	}
	// The seller-safe composite FK must reject a SKU identity owned by another seller.
	var otherUser, otherSeller, otherSKU, postingVersion int64
	if e = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,'integration','active') RETURNING id`, fmt.Sprintf("lifecycle_other_%d@test.local", time.Now().UnixNano())).Scan(&otherUser); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, otherUser) })
	if e = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,'other lifecycle','active') RETURNING id`, otherUser).Scan(&otherSeller); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state) VALUES($1,202,NOW(),NOW(),'ACTIVE') RETURNING id`, otherSeller).Scan(&otherSKU); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `SELECT id FROM sales_posting_versions WHERE seller_account_id=$1 AND collection_domain_run_id=$2 ORDER BY id LIMIT 1`, seller, run1).Scan(&postingVersion); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `INSERT INTO sales_posting_products(seller_account_id,posting_version_id,line_no,canonical_sku_id,source_sku,sku_resolution_status,quantity,unit_price,currency_code,source_payload_id) VALUES($1,$2,99,$3,202,'RESOLVED',1,1,'RUB',$4)`, seller, postingVersion, otherSKU, r1.postings[0].PayloadID); e == nil {
		t.Fatal("cross-seller canonical SKU relation was accepted")
	}
	var resolution string
	if e = pool.QueryRow(ctx, `SELECT resolution_status FROM sales_posting_relations WHERE seller_account_id=$1`, seller).Scan(&resolution); e != nil || resolution != "RESOLVED" {
		t.Fatalf("relation=%s err=%v", resolution, e)
	}
	if r1.ResolvedSKUs != 1 || r1.UnresolvedSKUs != 1 {
		t.Fatalf("result=%+v", r1)
	}
	var demandCount int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM sales_demand_daily WHERE seller_account_id=$1`, seller).Scan(&demandCount); e != nil || demandCount != 0 {
		t.Fatalf("demand changed=%d", demandCount)
	}
	_, run2 := newDomain("POSTINGS")
	r2 := postingResult(run2, "awaiting_deliver", "3", "PARENT")
	if e = repoAt(t1.Add(time.Hour)).FinalizePostings(ctx, seller, run2, &r2); e != nil {
		t.Fatal(e)
	}
	if r2.VersionsWritten != 0 || r2.VersionsReused != 2 {
		t.Fatalf("identical=%+v", r2)
	}
	_, run3 := newDomain("POSTINGS")
	r3 := postingResult(run3, "delivered", "2", "PARENT")
	if e = repoAt(t1.Add(2*time.Hour)).FinalizePostings(ctx, seller, run3, &r3); e != nil {
		t.Fatal(e)
	}
	var partialEvents int
	e = pool.QueryRow(ctx, `SELECT count(*) FROM sales_lifecycle_events WHERE seller_account_id=$1 AND event_type='POSTING_CANCELLED_PARTIAL'`, seller).Scan(&partialEvents)
	if e != nil || partialEvents != 0 {
		t.Fatalf("partial cancellation invented=%d", partialEvents)
	}
	var quality string
	var effective *time.Time
	e = pool.QueryRow(ctx, `SELECT time_quality,effective_at FROM sales_lifecycle_events WHERE seller_account_id=$1 AND event_type='POSTING_DELIVERED' ORDER BY id DESC LIMIT 1`, seller).Scan(&quality, &effective)
	if e != nil || quality != "OBSERVED_APPROX" || effective != nil {
		t.Fatalf("delivery temporal=%s/%v err=%v", quality, effective, e)
	}
	_, run4 := newDomain("POSTINGS")
	r4 := postingResult(run4, "awaiting_deliver", "3", "PARENT")
	if e = repoAt(t1.Add(3*time.Hour)).FinalizePostings(ctx, seller, run4, &r4); e != nil {
		t.Fatal(e)
	}
	var hashes []string
	rows, e := pool.Query(ctx, `SELECT state_hash FROM sales_posting_versions v JOIN sales_postings p ON p.id=v.canonical_posting_id WHERE p.seller_account_id=$1 AND p.posting_number='CHILD' ORDER BY version_no`, seller)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var h string
		_ = rows.Scan(&h)
		hashes = append(hashes, h)
	}
	rows.Close()
	if len(hashes) != 3 || hashes[0] != hashes[2] || hashes[0] == hashes[1] {
		t.Fatalf("A-B-A=%v", hashes)
	}
	var originalObserved time.Time
	if e = pool.QueryRow(ctx, `SELECT observed_at FROM sales_posting_versions v JOIN sales_postings p ON p.id=v.canonical_posting_id WHERE p.seller_account_id=$1 AND p.posting_number='CHILD' AND version_no=1`, seller).Scan(&originalObserved); e != nil || !originalObserved.Equal(t1) {
		t.Fatalf("historical version mutated at=%v err=%v", originalObserved, e)
	}
	// An unseen split parent remains explicit and resolves when that stable identity appears later.
	_, run5 := newDomain("POSTINGS")
	p5, received5 := payload(run5, "POSTINGS")
	orphan := PostingCandidate{OrderID: 2, OrderNumber: "ORDER-2", PostingNumber: "ORPHAN-CHILD", Scheme: "FBS", Status: "new", ParentPostingNumber: "LATE-PARENT", Products: []ProductCandidate{{101, "", "", 1, money("1"), "RUB", p5}}, PayloadID: p5, SourceReceivedAt: received5}
	orphan.StateHash = postingState(orphan)
	orphanResult := PostingsResult{RequestedFrom: day, RequestedTo: day, FBO: StreamResult{Status: "COMPLETE"}, FBS: StreamResult{Status: "COMPLETE"}, Postings: 1, postings: []PostingCandidate{orphan}}
	if e = repoAt(t1.Add(4*time.Hour)).FinalizePostings(ctx, seller, run5, &orphanResult); e != nil {
		t.Fatal(e)
	}
	e = pool.QueryRow(ctx, `SELECT resolution_status FROM sales_posting_relations WHERE seller_account_id=$1 AND parent_posting_number='LATE-PARENT'`, seller).Scan(&resolution)
	if e != nil || resolution != "UNRESOLVED" {
		t.Fatalf("unseen parent=%s", resolution)
	}
	_, run6 := newDomain("POSTINGS")
	p6, received6 := payload(run6, "POSTINGS")
	late := PostingCandidate{OrderID: 2, OrderNumber: "ORDER-2", PostingNumber: "LATE-PARENT", Scheme: "FBS", Status: "new", Products: []ProductCandidate{{101, "", "", 1, money("1"), "RUB", p6}}, PayloadID: p6, SourceReceivedAt: received6}
	late.StateHash = postingState(late)
	lateResult := PostingsResult{RequestedFrom: day, RequestedTo: day, FBO: StreamResult{Status: "COMPLETE"}, FBS: StreamResult{Status: "COMPLETE"}, Postings: 1, postings: []PostingCandidate{late}}
	if e = repoAt(t1.Add(5*time.Hour)).FinalizePostings(ctx, seller, run6, &lateResult); e != nil {
		t.Fatal(e)
	}
	e = pool.QueryRow(ctx, `SELECT resolution_status FROM sales_posting_relations WHERE seller_account_id=$1 AND parent_posting_number='LATE-PARENT'`, seller).Scan(&resolution)
	if e != nil || resolution != "RESOLVED" {
		t.Fatalf("late parent=%s", resolution)
	}
	// Identity conflicts and explicit lineage cycles fail the whole transaction.
	_, conflictRun := newDomain("POSTINGS")
	cp, cr := payload(conflictRun, "POSTINGS")
	conflict := PostingCandidate{OrderID: 1, OrderNumber: "DIFFERENT", PostingNumber: "PARENT", Scheme: "FBS", Status: "new", Products: []ProductCandidate{{101, "", "", 1, money("1"), "RUB", cp}}, PayloadID: cp, SourceReceivedAt: cr}
	conflict.StateHash = postingState(conflict)
	conflictResult := PostingsResult{RequestedFrom: day, RequestedTo: day, FBO: StreamResult{Status: "COMPLETE"}, FBS: StreamResult{Status: "COMPLETE"}, Postings: 1, postings: []PostingCandidate{conflict}}
	if e = repoAt(t1.Add(5*time.Hour+time.Minute)).FinalizePostings(ctx, seller, conflictRun, &conflictResult); !errors.Is(e, ErrOrderIdentityConflict) {
		t.Fatalf("order conflict=%v", e)
	}
	_, schemeRun := newDomain("POSTINGS")
	sp, sr := payload(schemeRun, "POSTINGS")
	scheme := PostingCandidate{OrderID: 1, OrderNumber: "ORDER-1", PostingNumber: "PARENT", Scheme: "FBO", Status: "new", Products: []ProductCandidate{{101, "", "", 1, money("1"), "RUB", sp}}, PayloadID: sp, SourceReceivedAt: sr}
	scheme.StateHash = postingState(scheme)
	schemeResult := PostingsResult{RequestedFrom: day, RequestedTo: day, FBO: StreamResult{Status: "COMPLETE"}, FBS: StreamResult{Status: "COMPLETE"}, Postings: 1, postings: []PostingCandidate{scheme}}
	if e = repoAt(t1.Add(5*time.Hour+2*time.Minute)).FinalizePostings(ctx, seller, schemeRun, &schemeResult); e != nil {
		t.Fatalf("scheme version=%v", e)
	}
	var latestScheme string
	if e = pool.QueryRow(ctx, `SELECT v.fulfillment_scheme FROM sales_posting_versions v JOIN sales_postings p ON p.id=v.canonical_posting_id WHERE p.seller_account_id=$1 AND p.posting_number='PARENT' ORDER BY v.version_no DESC LIMIT 1`, seller).Scan(&latestScheme); e != nil || latestScheme != "FBO" {
		t.Fatalf("latest scheme=%q err=%v", latestScheme, e)
	}
	_, cycleRun := newDomain("POSTINGS")
	cyp, cyr := payload(cycleRun, "POSTINGS")
	cycleA := PostingCandidate{OrderID: 3, OrderNumber: "ORDER-3", PostingNumber: "CYCLE-A", Scheme: "FBS", Status: "new", ParentPostingNumber: "CYCLE-B", Products: []ProductCandidate{{101, "", "", 1, money("1"), "RUB", cyp}}, PayloadID: cyp, SourceReceivedAt: cyr}
	cycleB := PostingCandidate{OrderID: 3, OrderNumber: "ORDER-3", PostingNumber: "CYCLE-B", Scheme: "FBS", Status: "new", ParentPostingNumber: "CYCLE-A", Products: []ProductCandidate{{101, "", "", 1, money("1"), "RUB", cyp}}, PayloadID: cyp, SourceReceivedAt: cyr}
	cycleA.StateHash, cycleB.StateHash = postingState(cycleA), postingState(cycleB)
	cycleResult := PostingsResult{RequestedFrom: day, RequestedTo: day, FBO: StreamResult{Status: "COMPLETE"}, FBS: StreamResult{Status: "COMPLETE"}, Postings: 2, postings: []PostingCandidate{cycleA, cycleB}}
	if e = repoAt(t1.Add(5*time.Hour+3*time.Minute)).FinalizePostings(ctx, seller, cycleRun, &cycleResult); !errors.Is(e, ErrPostingLineageCycle) {
		t.Fatalf("cycle=%v", e)
	}
	// Return identity/history preserves provider semantics and unresolved links.
	returnResult := func(run int64, visual string) ReturnsResult {
		p, received := payload(run, "RETURNS")
		c := ReturnCandidate{ProviderReturnID: 77, SourceID: 900, SourceOrderID: 999, SourcePostingNumber: "MISSING", SourceSKU: 999, ProviderType: "provider_type", ProviderSchema: "FBO", Reason: "raw reason", Quantity: 1, UnitPrice: money("5.5"), Currency: "RUB", VisualStatus: visual, ProviderReturnDate: timePtr(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)), PayloadID: p, SourceReceivedAt: received}
		c.StateHash = returnState(c)
		return ReturnsResult{RequestedFrom: day, RequestedTo: day, Stream: StreamResult{Status: "COMPLETE", Pages: 1, Records: 1}, Returns: 1, returns: []ReturnCandidate{c}}
	}
	_, rr1 := newDomain("RETURNS")
	ret1 := returnResult(rr1, "moving")
	if e = repoAt(t1.Add(6*time.Hour)).FinalizeReturns(ctx, seller, rr1, &ret1); e != nil {
		t.Fatal(e)
	}
	_, rr2 := newDomain("RETURNS")
	ret2 := returnResult(rr2, "moving")
	if e = repoAt(t1.Add(7*time.Hour)).FinalizeReturns(ctx, seller, rr2, &ret2); e != nil || ret2.VersionsWritten != 0 {
		t.Fatalf("return identical=%+v err=%v", ret2, e)
	}
	_, rr3 := newDomain("RETURNS")
	ret3 := returnResult(rr3, "received")
	if e = repoAt(t1.Add(8*time.Hour)).FinalizeReturns(ctx, seller, rr3, &ret3); e != nil {
		t.Fatal(e)
	}
	_, rr4 := newDomain("RETURNS")
	ret4 := returnResult(rr4, "moving")
	if e = repoAt(t1.Add(9*time.Hour)).FinalizeReturns(ctx, seller, rr4, &ret4); e != nil {
		t.Fatal(e)
	}
	hashes = nil
	rows, e = pool.Query(ctx, `SELECT state_hash FROM sales_return_events e JOIN sales_returns r ON r.id=e.sales_return_id WHERE r.seller_account_id=$1 AND r.provider_return_id=77 ORDER BY version_no`, seller)
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var h string
		_ = rows.Scan(&h)
		hashes = append(hashes, h)
	}
	rows.Close()
	if len(hashes) != 3 || hashes[0] != hashes[2] {
		t.Fatalf("return A-B-A=%v", hashes)
	}
	var providerType, providerSchema, reason string
	e = pool.QueryRow(ctx, `SELECT provider_type,provider_schema,return_reason_name FROM sales_return_events WHERE seller_account_id=$1 ORDER BY id LIMIT 1`, seller).Scan(&providerType, &providerSchema, &reason)
	if e != nil || providerType != "provider_type" || providerSchema != "FBO" || reason != "raw reason" {
		t.Fatalf("return semantics=%s/%s/%s", providerType, providerSchema, reason)
	}
	var business *time.Time
	e = pool.QueryRow(ctx, `SELECT time_quality,effective_at,business_date FROM sales_lifecycle_events WHERE seller_account_id=$1 AND event_type='RETURN_CREATED'`, seller).Scan(&quality, &effective, &business)
	if e != nil || quality != "UNKNOWN" || effective != nil || business != nil {
		t.Fatalf("return temporal=%s/%v/%v", quality, effective, business)
	}
	// Newer authoritative runs supersede older workers without writes.
	_, oldRun := newDomain("POSTINGS")
	old := postingResult(oldRun, "delivering", "3", "PARENT")
	_, newRun := newDomain("POSTINGS")
	newer := postingResult(newRun, "delivered", "3", "PARENT")
	if e = repoAt(t1.Add(10*time.Hour)).FinalizePostings(ctx, seller, newRun, &newer); e != nil {
		t.Fatal(e)
	}
	before := versionCount(t, ctx, pool, seller, "PARENT")
	e = repoAt(t1.Add(11*time.Hour)).FinalizePostings(ctx, seller, oldRun, &old)
	var stale *SupersededRunError
	if !errors.As(e, &stale) {
		t.Fatalf("stale err=%v", e)
	}
	after := versionCount(t, ctx, pool, seller, "PARENT")
	if before != after {
		t.Fatalf("stale wrote versions %d->%d", before, after)
	}

	// RETURNS has an independent authoritative head and the same stale-run protection.
	_, oldReturnRun := newDomain("RETURNS")
	oldReturn := returnResult(oldReturnRun, "moving")
	_, newReturnRun := newDomain("RETURNS")
	newReturn := returnResult(newReturnRun, "completed")
	if e = repoAt(t1.Add(12*time.Hour)).FinalizeReturns(ctx, seller, newReturnRun, &newReturn); e != nil {
		t.Fatal(e)
	}
	beforeReturns := returnVersionCount(t, ctx, pool, seller, 77)
	e = repoAt(t1.Add(13*time.Hour)).FinalizeReturns(ctx, seller, oldReturnRun, &oldReturn)
	if !errors.As(e, &stale) || stale.Domain != "RETURNS" {
		t.Fatalf("stale return err=%v", e)
	}
	afterReturns := returnVersionCount(t, ctx, pool, seller, 77)
	if beforeReturns != afterReturns {
		t.Fatalf("stale return wrote versions %d->%d", beforeReturns, afterReturns)
	}
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM sales_demand_daily WHERE seller_account_id=$1`, seller).Scan(&demandCount); e != nil || demandCount != 0 {
		t.Fatalf("lifecycle changed demand after postings/returns: count=%d err=%v", demandCount, e)
	}
	var piiColumns int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name IN ('sales_orders','sales_postings','sales_posting_versions','sales_posting_products','sales_posting_relations','sales_returns','sales_return_events','sales_lifecycle_events') AND column_name ~ '(customer|phone|email|address|addressee|imei|digital_code)'`).Scan(&piiColumns); e != nil || piiColumns != 0 {
		t.Fatalf("canonical lifecycle PII columns=%d err=%v", piiColumns, e)
	}
}
func timePtr(v time.Time) *time.Time { return &v }
func postingState(c PostingCandidate) string {
	parts := []string{c.Status, c.Scheme}
	for _, p := range c.Products {
		parts = append(parts, fmt.Sprintf("%d/%d/%s", p.SourceSKU, p.Quantity, p.UnitPrice.String()))
	}
	return hash(parts...)
}
func returnState(c ReturnCandidate) string {
	return hash(c.VisualStatus, c.ProviderType, c.ProviderSchema, c.Reason)
}
func versionCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seller int64, posting string) int {
	t.Helper()
	var n int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM sales_posting_versions v JOIN sales_postings p ON p.id=v.canonical_posting_id WHERE p.seller_account_id=$1 AND p.posting_number=$2`, seller, posting).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}

func returnVersionCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seller, providerReturnID int64) int {
	t.Helper()
	var n int
	if e := pool.QueryRow(ctx, `SELECT count(*) FROM sales_return_events e JOIN sales_returns r ON r.id=e.sales_return_id WHERE r.seller_account_id=$1 AND r.provider_return_id=$2`, seller, providerReturnID).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
