//go:build integration

package promotions

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPromotionMembershipTemporalBoundariesAndPartialClosure(t *testing.T) {
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

	var user, seller, sku int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status)VALUES($1,'integration','active')RETURNING id`, fmt.Sprintf("promo_temporal_%d@test.local", time.Now().UnixNano())).Scan(&user); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user) })
	if err = pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status)VALUES($1,'promo_temporal','active')RETURNING id`, user).Scan(&seller); err != nil {
		t.Fatal(err)
	}
	sourceSKU := int64(88001)
	if err = pool.QueryRow(ctx, `INSERT INTO canonical_skus(seller_account_id,ozon_sku,first_observed_at,last_observed_at,catalog_state)VALUES($1,$2,NOW()-INTERVAL '1 day',NOW(),'ACTIVE')RETURNING id`, seller, sourceSKU).Scan(&sku); err != nil {
		t.Fatal(err)
	}

	newRun := func(at time.Time) (int64, int64) {
		var parent, run, payload int64
		if err = pool.QueryRow(ctx, `INSERT INTO collection_runs(seller_account_id,kind,status,trigger)VALUES($1,'MANUAL','RUNNING','promo.temporal')RETURNING id`, seller).Scan(&parent); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO collection_domain_runs(collection_run_id,seller_account_id,domain,status,started_at)VALUES($1,$2,'PROMOTIONS','RUNNING',$3)RETURNING id`, parent, seller, at).Scan(&run); err != nil {
			t.Fatal(err)
		}
		if err = pool.QueryRow(ctx, `INSERT INTO source_payloads(seller_account_id,collection_domain_run_id,provider,endpoint_key,request_fingerprint,page_seq,storage_bucket,storage_object_key,sha256,http_status,received_at,metadata_json,redaction_profile)VALUES($1,$2,'OZON','ozon.promotions.temporal',$3,1,'raw',$4,$5,200,$6,'{}','TEST_REDACTION_V1')RETURNING id`, seller, run, fmt.Sprintf("fp-%d", run), fmt.Sprintf("promo/%d", run), fmt.Sprintf("%064x", run), at).Scan(&payload); err != nil {
			t.Fatal(err)
		}
		return run, payload
	}

	openAt := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	closeAt := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)

	openRun, openPayload := newRun(openAt)
	openResult := Result{
		StartedAt: openAt, CompletedAt: openAt.Add(time.Minute), CompletenessStatus: "COMPLETE",
		Definitions:             []DefinitionObservation{{Origin: "OZON", ProviderActionID: 501, Title: "Promo A", ProviderState: "ACTIVE", SourcePayloadID: openPayload}},
		Participants:            []SKUObservation{{Origin: "OZON", ProviderActionID: 501, CanonicalSKUID: &sku, SourceSKU: &sourceSKU, SKUResolutionStatus: "RESOLVED", SourcePayloadID: openPayload}},
		CompleteParticipantKeys: []PromotionKey{{Origin: "OZON", ProviderActionID: 501}},
	}
	if err = NewRepository(pool).Finalize(ctx, seller, openRun, &openResult); err != nil {
		t.Fatal(err)
	}

	partialRun, partialPayload := newRun(openAt.Add(time.Hour))
	partialResult := Result{
		StartedAt: openAt.Add(time.Hour), CompletedAt: openAt.Add(90 * time.Minute), CompletenessStatus: "PARTIAL",
		Definitions:  []DefinitionObservation{{Origin: "OZON", ProviderActionID: 501, Title: "Promo A", ProviderState: "ACTIVE", SourcePayloadID: partialPayload}},
		Participants: []SKUObservation{}, // incomplete participant page — must not close open membership
	}
	if err = NewRepository(pool).Finalize(ctx, seller, partialRun, &partialResult); err != nil {
		t.Fatal(err)
	}
	var openAfterPartial int64
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM promotion_sku_memberships WHERE seller_account_id=$1 AND known_to IS NULL`, seller).Scan(&openAfterPartial); err != nil {
		t.Fatal(err)
	}
	if openAfterPartial != 1 {
		t.Fatalf("partial collection fabricated closure: open=%d", openAfterPartial)
	}

	closeRun, closePayload := newRun(closeAt)
	closeResult := Result{
		StartedAt: closeAt, CompletedAt: closeAt.Add(time.Minute), CompletenessStatus: "COMPLETE",
		Definitions:             []DefinitionObservation{{Origin: "OZON", ProviderActionID: 501, Title: "Promo A", ProviderState: "ACTIVE", SourcePayloadID: closePayload}},
		Participants:            []SKUObservation{}, // authoritative empty complete set closes SKU membership
		CompleteParticipantKeys: []PromotionKey{{Origin: "OZON", ProviderActionID: 501}},
	}
	if err = NewRepository(pool).Finalize(ctx, seller, closeRun, &closeResult); err != nil {
		t.Fatal(err)
	}

	signer, err := factualapi.NewTokenSigner([]byte(strings.Repeat("p", 32)))
	if err != nil {
		t.Fatal(err)
	}
	api := factual.NewService(pool, signer, 50, 500)
	makeCtx := func(cutoff time.Time, run int64) factualapi.FactualReadContext {
		t.Helper()
		var capabilities []factualapi.CapabilityBinding
		for i, key := range readiness.CapabilityKeys() {
			capabilities = append(capabilities, factualapi.CapabilityBinding{CapabilityKey: key, Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: int64(i + 1), AssessedAt: cutoff})
		}
		analysisDay := factualapi.LastCompletedBusinessDate(cutoff)
		c, e := factualapi.NewReadContext(factualapi.ContextSpec{
			SellerAccountID: seller, AnalysisDate: analysisDay, CutoffAt: cutoff, Mode: factualapi.ModeLatestStable, Policies: factualapi.V1Policies(),
			Domains:      []factualapi.DomainBinding{{Domain: "PROMOTIONS", FactFamily: "PROMOTIONS.MEMBERSHIP", DomainRunID: run, FinalizedAt: cutoff, ContractHash: "promo.temporal"}},
			Capabilities: capabilities,
		})
		if e != nil {
			t.Fatal(e)
		}
		return c
	}

	oldCtx := makeCtx(openAt.Add(30*time.Minute), openRun)
	active, err := api.GetSKUPromotions(ctx, oldCtx, sku, factualapi.PageRequest{PageSize: 10})
	if err != nil || len(active.Items) != 1 || active.Items[0].ID == 0 {
		t.Fatalf("old context must see active Promo A membership: items=%+v err=%v", active.Items, err)
	}
	promotionID := active.Items[0].ID

	newCtx := makeCtx(closeAt.Add(time.Minute), closeRun)
	inactive, err := api.GetSKUPromotions(ctx, newCtx, sku, factualapi.PageRequest{PageSize: 10})
	if err != nil || len(inactive.Items) != 0 {
		t.Fatalf("new context must see inactive membership: items=%+v err=%v", inactive.Items, err)
	}

	history, err := api.GetPromotionMembershipHistory(ctx, newCtx, promotionID, factualapi.PromotionMembershipFilter{CanonicalSKUID: &sku}, factualapi.PageRequest{PageSize: 10})
	if err != nil || len(history.Items) != 1 {
		t.Fatalf("membership history must expose one closed interval: items=%+v err=%v", history.Items, err)
	}
	interval := history.Items[0]
	if interval.KnownFrom.IsZero() || interval.KnownTo == nil || !interval.KnownTo.Equal(closeAt) {
		t.Fatalf("history must show both boundaries: from=%v to=%v", interval.KnownFrom, interval.KnownTo)
	}

	eligibility, err := api.GetPromotionEligibility(ctx, newCtx, promotionID, factualapi.PageRequest{PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if eligibility.SnapshotID != 0 {
		t.Fatalf("no eligibility snapshots were written; got snapshot=%d", eligibility.SnapshotID)
	}
}
