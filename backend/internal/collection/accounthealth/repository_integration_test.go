//go:build integration

package accounthealth

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountHealthFinalizePreservesTemporalAndIdentityRules(t *testing.T) {
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
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("health_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'health','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	newRun := func() (int64, int64) {
		var parent, run, payload int64
		if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','health.integration')RETURNING id`, seller).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'ACCOUNT_HEALTH','RUNNING',NOW())RETURNING id`, parent, seller).Scan(&run); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.account_health.test',$3,1,'raw',$4,$5,200,NOW(),'{}',$6)RETURNING id`, seller, run, fmt.Sprintf("fp-%d", run), fmt.Sprintf("health/%d", run), fmt.Sprintf("%064x", run), RedactionProfile).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		return run, payload
	}
	at1 := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	run1, payload1 := newRun()
	result1 := Result{
		StartedAt: at1, CompletedAt: at1.Add(time.Minute), CompletenessStatus: "COMPLETE", SummaryPayloadID: payload1,
		Metrics:    []MetricObservation{{ProviderRatingKey: "new_key", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", GroupName: "A", Value: "95", ProviderPreviousValue: "92", ProviderStatus: "CRITICAL", SourcePayloadID: payload1}},
		History:    []PeriodFact{{ProviderRatingKey: "new_key", PeriodFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), PeriodTo: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), Value: "91", ProviderStatus: "WARNING", SourcePayloadID: payload1}},
		Thresholds: []ThresholdObservation{{ProviderRatingKey: "new_key", ProviderStatus: "WARNING", From: "80", To: "94", SourcePayloadID: payload1}},
		FBSIndex:   &FBSIndexFact{Value: "0.25", ProviderStatus: "OK", SourcePayloadID: payload1},
		FBSErrors:  []FBSErrorEvent{{ProviderEventKey: "e1", SourcePostingNumber: "missing", ResolutionStatus: "UNRESOLVED", ErrorCode: "SELLER_DELAY", SourcePayloadID: payload1}},
	}
	if err = NewRepository(pool).Finalize(ctx, seller, run1, &result1); err != nil {
		t.Fatal(err)
	}
	at2 := at1.Add(24 * time.Hour)
	run2, payload2 := newRun()
	result2 := Result{
		StartedAt: at2, CompletedAt: at2.Add(time.Minute), CompletenessStatus: "COMPLETE", SummaryPayloadID: payload2,
		Metrics:    []MetricObservation{{ProviderRatingKey: "new_key", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", GroupName: "B", Value: "96", ProviderPreviousValue: "95", ProviderStatus: "OK", SourcePayloadID: payload2}},
		Thresholds: []ThresholdObservation{{ProviderRatingKey: "new_key", ProviderStatus: "WARNING", From: "85", To: "94", SourcePayloadID: payload2}},
	}
	if err = NewRepository(pool).Finalize(ctx, seller, run2, &result2); err != nil {
		t.Fatal(err)
	}
	var metrics, versions, thresholds, periods, postings int64
	var historicalStatus string
	if err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM account_health_metrics WHERE seller_account_id=$1),
		(SELECT count(*) FROM account_health_metric_versions WHERE seller_account_id=$1),
		(SELECT count(*) FROM account_rating_threshold_observations WHERE seller_account_id=$1),
		(SELECT count(*) FROM account_rating_period_facts WHERE seller_account_id=$1),
		(SELECT count(*) FROM sales_postings WHERE seller_account_id=$1),
		(SELECT provider_status FROM account_rating_period_facts WHERE seller_account_id=$1 LIMIT 1)`, seller).Scan(&metrics, &versions, &thresholds, &periods, &postings, &historicalStatus); err != nil {
		t.Fatal(err)
	}
	if metrics != 1 || versions != 2 || thresholds != 2 || periods != 1 {
		t.Fatalf("identity/version counts=%d/%d/%d/%d", metrics, versions, thresholds, periods)
	}
	if historicalStatus != "WARNING" {
		t.Fatalf("old history was recalculated: %s", historicalStatus)
	}
	if postings != 0 {
		t.Fatalf("unresolved rating event created %d fake postings", postings)
	}
}

func TestAccountHealthPointerHashesDedupeOnReread(t *testing.T) {
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
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("health_hash_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'health_hash','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	newRun := func() (int64, int64) {
		var parent, run, payload int64
		if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','health.hash')RETURNING id`, seller).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'ACCOUNT_HEALTH','RUNNING',NOW())RETURNING id`, parent, seller).Scan(&run); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.account_health.hash',$3,1,'raw',$4,$5,200,NOW(),'{}',$6)RETURNING id`, seller, run, fmt.Sprintf("fp-%d", run), fmt.Sprintf("health-hash/%d", run), fmt.Sprintf("%064x", run), RedactionProfile).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		return run, payload
	}
	at := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	occurred1 := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	occurred2 := time.Date(2026, 8, 20, 8, 0, 0, 0, time.UTC)
	run1, payload1 := newRun()
	first := Result{
		StartedAt: at, CompletedAt: at.Add(time.Minute), CompletenessStatus: "COMPLETE", SummaryPayloadID: payload1,
		Metrics:   []MetricObservation{{ProviderRatingKey: "hash_metric", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", Value: "1", SourcePayloadID: payload1}},
		Penalties: []PenaltyEvent{{ProviderEventKey: "penalty-hash-1", ProviderRatingKey: "hash_metric", Amount: "10", Currency: "RUB", OccurredAt: &occurred1, SourcePayloadID: payload1}},
		FBSErrors: []FBSErrorEvent{{ProviderEventKey: "fbs-hash-1", SourcePostingNumber: "P-1", ResolutionStatus: "UNRESOLVED", OccurredAt: &occurred1, ErrorCode: "DELAY", SourcePayloadID: payload1}},
	}
	if err = NewRepository(pool).Finalize(ctx, seller, run1, &first); err != nil {
		t.Fatal(err)
	}
	run2, payload2 := newRun()
	second := Result{
		StartedAt: at.Add(2 * time.Hour), CompletedAt: at.Add(3 * time.Hour), CompletenessStatus: "COMPLETE", SummaryPayloadID: payload2,
		Metrics:   []MetricObservation{{ProviderRatingKey: "hash_metric", CanonicalMetricKey: "UNMAPPED", MappingStatus: "UNMAPPED", Value: "1", SourcePayloadID: payload2}},
		Penalties: []PenaltyEvent{{ProviderEventKey: "penalty-hash-1", ProviderRatingKey: "hash_metric", Amount: "10", Currency: "RUB", OccurredAt: &occurred2, SourcePayloadID: payload2}},
		FBSErrors: []FBSErrorEvent{{ProviderEventKey: "fbs-hash-1", SourcePostingNumber: "P-1", ResolutionStatus: "UNRESOLVED", OccurredAt: &occurred2, ErrorCode: "DELAY", SourcePayloadID: payload2}},
	}
	if err = NewRepository(pool).Finalize(ctx, seller, run2, &second); err != nil {
		t.Fatal(err)
	}
	var penaltyRevs, fbsRevs int64
	if err = pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM account_penalty_event_facts WHERE seller_account_id=$1 AND provider_event_key='penalty-hash-1'),
		(SELECT count(*) FROM fbs_posting_error_events WHERE seller_account_id=$1 AND provider_event_key='fbs-hash-1')`, seller).Scan(&penaltyRevs, &fbsRevs); err != nil {
		t.Fatal(err)
	}
	if penaltyRevs != 1 || fbsRevs != 1 {
		t.Fatalf("identical pointer-allocated events must dedupe: penalties=%d fbs=%d", penaltyRevs, fbsRevs)
	}
	if second.PenaltiesWritten != 0 || second.FBSErrorEventsWritten != 0 {
		t.Fatalf("reread wrote new revisions: penalties=%d fbs=%d", second.PenaltiesWritten, second.FBSErrorEventsWritten)
	}
}
