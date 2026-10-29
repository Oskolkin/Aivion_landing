//go:build integration

package funnel

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFunnelRevisionCyclesAllowABAAndCutoffReads(t *testing.T) {
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

	var user, seller int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("funnel71_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'funnel71','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}

	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	contract := fmt.Sprintf("%064x", 71)
	newRun := func(label string) (int64, int64) {
		t.Helper()
		var parent, run, payload int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING',$2)RETURNING id`, seller, label).Scan(&parent); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'FUNNEL','RUNNING',NOW())RETURNING id`, parent, seller).Scan(&run); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.funnel.test',$3,1,'raw',$4,$5,200,NOW(),'{}','TEST_REDACTION_V1')RETURNING id`, seller, run, label, "funnel71/"+label, fmt.Sprintf("%064x", run)).Scan(&payload); e != nil {
			t.Fatal(e)
		}
		return run, payload
	}
	obs := func(value string, payload int64) Observation {
		return Observation{
			ScopeKind: "ACCOUNT", MetricKey: "SEARCH_IMPRESSIONS", PeriodStart: day, PeriodEnd: day,
			ValueNumeric: value, Unit: "COUNT", TemporalSemantics: "ADDITIVE_FLOW", EntityAggregationSemantics: "ADDITIVE",
			Surface: "SEARCH_CATALOG", ProviderMetricName: "hits_view_search", RegistryVersion: "test",
			SourceContractHash: contract, SourcePayloadID: payload, SKUResolutionStatus: "NOT_APPLICABLE",
		}
	}
	finalize := func(at time.Time, value string) {
		t.Helper()
		run, payload := newRun(fmt.Sprintf("funnel71.%s.%d", value, at.UnixNano()))
		result := Result{RequestedFrom: day, RequestedTo: day, RecordsReceived: 1, Pages: 1, Observations: []Observation{obs(value, payload)}}
		if e := NewRepositoryWithClock(pool, func() time.Time { return at }).Finalize(ctx, seller, run, &result); e != nil {
			t.Fatal(e)
		}
	}
	readAsOf := func(cutoff time.Time) (value string, revision int64) {
		t.Helper()
		if e := pool.QueryRow(ctx, `SELECT value_numeric::text,revision_no FROM funnel_observations WHERE seller_account_id=$1 AND scope_kind='ACCOUNT' AND metric_key='SEARCH_IMPRESSIONS' AND period_start=$2 AND period_end=$2 AND observed_at<=$3 ORDER BY observed_at DESC,revision_no DESC,id DESC LIMIT 1`, seller, day, cutoff).Scan(&value, &revision); e != nil {
			t.Fatal(e)
		}
		return value, revision
	}
	countRevisions := func() int64 {
		t.Helper()
		var n int64
		if e := pool.QueryRow(ctx, `SELECT count(*) FROM funnel_observations WHERE seller_account_id=$1 AND metric_key='SEARCH_IMPRESSIONS' AND period_start=$2`, seller, day).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}

	atA := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	finalize(atA, "100")
	if n := countRevisions(); n != 1 {
		t.Fatalf("A write revisions=%d", n)
	}
	v, rev := readAsOf(atA.Add(time.Second))
	if !(v == "100" || v == "100.000000") || rev != 1 {
		t.Fatalf("A as-of: value=%q rev=%d", v, rev)
	}

	// A→A: same semantic hash must skip insert.
	finalize(atA.Add(time.Minute), "100")
	if n := countRevisions(); n != 1 {
		t.Fatalf("A→A should not create revision: count=%d", n)
	}

	atB := atA.Add(2 * time.Minute)
	finalize(atB, "200")
	if n := countRevisions(); n != 2 {
		t.Fatalf("A→B revisions=%d", n)
	}
	v, rev = readAsOf(atB.Add(time.Second))
	if !(v == "200" || v == "200.000000") || rev != 2 {
		t.Fatalf("B as-of: value=%q rev=%d", v, rev)
	}
	v, rev = readAsOf(atA.Add(30 * time.Second))
	if !(v == "100" || v == "100.000000") || rev != 1 {
		t.Fatalf("cutoff before B must still see A: value=%q rev=%d", v, rev)
	}

	atA2 := atB.Add(time.Minute)
	finalize(atA2, "100")
	if n := countRevisions(); n != 3 {
		t.Fatalf("A→B→A revisions=%d want 3", n)
	}
	v, rev = readAsOf(atA2.Add(time.Second))
	if !(v == "100" || v == "100.000000") || rev != 3 {
		t.Fatalf("A→B→A latest: value=%q rev=%d", v, rev)
	}
	v, rev = readAsOf(atB.Add(30 * time.Second))
	if !(v == "200" || v == "200.000000") || rev != 2 {
		t.Fatalf("cutoff at B must see B: value=%q rev=%d", v, rev)
	}
}
