//go:build integration

package finance

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFinanceDayRevisionCyclesAllowABAAndMultiplicity(t *testing.T) {
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
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("finance71_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'finance71','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}

	day := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	newRun := func(label string) (int64, int64) {
		t.Helper()
		var parent, run, payload int64
		if e := pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING',$2)RETURNING id`, seller, label).Scan(&parent); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'FINANCE','RUNNING',NOW())RETURNING id`, parent, seller).Scan(&run); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.finance.test',$3,1,'raw',$4,$5,200,NOW(),'{}','TEST_REDACTION_V1')RETURNING id`, seller, run, label, "finance71/"+label, fmt.Sprintf("%064x", run)).Scan(&payload); e != nil {
			t.Fatal(e)
		}
		return run, payload
	}
	dayWith := func(payload int64, amount string, duplicate bool) DayRevision {
		records := []Record{{
			RowOrdinal: 1, ProviderTypeID: "op", TotalAmountSigned: amount, CurrencyCode: "RUB",
			Lines: []Line{{Ordinal: 1, SourceComponentPath: "fees", SignedAmount: amount, MappingStatus: "UNMAPPED"}},
		}}
		if duplicate {
			records = append(records, Record{
				RowOrdinal: 2, ProviderTypeID: "op", TotalAmountSigned: amount, CurrencyCode: "RUB",
				Lines: []Line{{Ordinal: 1, SourceComponentPath: "fees", SignedAmount: amount, MappingStatus: "UNMAPPED"}},
			})
		}
		rev := DayRevision{BusinessDate: day, CompletenessStatus: "COMPLETE", SourcePayloadID: payload, Records: records}
		rev.StateHash = dayHash(rev)
		return rev
	}
	finalize := func(label string, amount string, duplicate bool) {
		t.Helper()
		run, payload := newRun(label)
		result := Result{CompletenessStatus: "COMPLETE", PagesReceived: 1, Days: []DayRevision{dayWith(payload, amount, duplicate)}}
		if e := NewRepository(pool).Finalize(ctx, seller, run, &result); e != nil {
			t.Fatal(e)
		}
	}
	countDays := func() (n, maxRev int64) {
		t.Helper()
		if e := pool.QueryRow(ctx, `SELECT count(*),COALESCE(max(revision_no),0) FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date=$2`, seller, day).Scan(&n, &maxRev); e != nil {
			t.Fatal(e)
		}
		return n, maxRev
	}
	latestAmount := func() string {
		t.Helper()
		var amount string
		if e := pool.QueryRow(ctx, `SELECT r.total_amount_signed::text FROM finance_accrual_records r JOIN finance_accrual_day_revisions d ON d.id=r.day_revision_id WHERE d.seller_account_id=$1 AND d.business_date=$2 ORDER BY d.revision_no DESC,r.row_ordinal LIMIT 1`, seller, day).Scan(&amount); e != nil {
			t.Fatal(e)
		}
		return amount
	}
	recordCountOnLatest := func() int64 {
		t.Helper()
		var n int64
		if e := pool.QueryRow(ctx, `SELECT count(*) FROM finance_accrual_records r JOIN finance_accrual_day_revisions d ON d.id=r.day_revision_id WHERE d.seller_account_id=$1 AND d.business_date=$2 AND d.revision_no=(SELECT max(revision_no) FROM finance_accrual_day_revisions WHERE seller_account_id=$1 AND business_date=$2)`, seller, day).Scan(&n); e != nil {
			t.Fatal(e)
		}
		return n
	}

	finalize("finance71.A", "10", false)
	n, rev := countDays()
	if n != 1 || rev != 1 {
		t.Fatalf("A: count=%d rev=%d", n, rev)
	}
	finalize("finance71.A.again", "10", false)
	n, rev = countDays()
	if n != 1 || rev != 1 {
		t.Fatalf("A→A must skip: count=%d rev=%d", n, rev)
	}

	finalize("finance71.B", "20", false)
	n, rev = countDays()
	if n != 2 || rev != 2 {
		t.Fatalf("A→B: count=%d rev=%d", n, rev)
	}
	if amt := latestAmount(); amt != "20" && amt != "20.000000" {
		t.Fatalf("latest B amount=%q", amt)
	}

	finalize("finance71.A2", "10", false)
	n, rev = countDays()
	if n != 3 || rev != 3 {
		t.Fatalf("A→B→A blocked or wrong: count=%d rev=%d", n, rev)
	}
	if amt := latestAmount(); amt != "10" && amt != "10.000000" {
		t.Fatalf("latest A again amount=%q", amt)
	}

	finalize("finance71.multi", "10", true)
	n, rev = countDays()
	if n != 4 || rev != 4 {
		t.Fatalf("multiplicity revision: count=%d rev=%d", n, rev)
	}
	if got := recordCountOnLatest(); got != 2 {
		t.Fatalf("identical provider rows lost multiplicity: records=%d", got)
	}
}
