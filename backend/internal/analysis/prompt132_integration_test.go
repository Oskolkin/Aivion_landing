//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func prompt132Exec(t *testing.T, f prompt81Fixture, worker string) (*runtime.Executor, artifacts.ArtifactStore) {
	t.Helper()
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	store := f.store
	if store == nil {
		store = artifacts.NewMemoryStore()
	}
	exec, err := runtime.NewProductionExecutor(f.pool, store, worker, nil)
	if err != nil {
		t.Fatal(err)
	}
	return exec, store
}

func prompt132RunThrough(t *testing.T, exec *runtime.Executor, runID int64, through string) {
	ctx := context.Background()
	for _, key := range stages.ProductionImplementedStageNames() {
		if err := exec.ExecuteStage(ctx, runID, key); err != nil {
			var code, msg string
			if st, stErr := exec.Repo.GetStage(ctx, runID, key); stErr == nil {
				code = st.ErrorCode
				msg = st.ErrorMessage
			}
			if code == "" {
				if prev, stErr := exec.Repo.GetStage(ctx, runID, prevFailedStage(key)); stErr == nil {
					code = prev.ErrorCode
					msg = prev.ErrorMessage
				}
			}
			t.Fatalf("run %d stage %s: %v code=%s msg=%s", runID, key, err, code, msg)
		}
		if through != "" && key == through {
			return
		}
	}
}

func prevFailedStage(key string) string {
	order := stages.ProductionImplementedStageNames()
	for i, k := range order {
		if k == key && i > 0 {
			return order[i-1]
		}
	}
	return key
}

func prompt132DeactivateRun(t *testing.T, f prompt81Fixture, runID int64) {
	_, err := f.pool.Exec(context.Background(), `UPDATE analysis_runs SET status='STALE' WHERE id=$1`, runID)
	if err != nil {
		t.Fatal(err)
	}
}

func prompt132PublishInput(t *testing.T, f prompt81Fixture, store artifacts.ArtifactStore, runID int64, worker string) analysispg.PublishInput {
	ctx := context.Background()
	run, err := f.repo.GetRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := f.repo.ClaimByRunStage(ctx, runID, string(stages.Publish), worker, 5*time.Minute, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	var vc publication.ValidatedCandidateV1
	if err = json.Unmarshal(prompt91LoadStageBody(t, f, store, runID, string(stages.ValidatePublication)), &vc); err != nil {
		t.Fatal(err)
	}
	var plan actionreconcile.PlanV1
	if err = json.Unmarshal(prompt91LoadStageBody(t, f, store, runID, string(stages.PlanActionReconciliation)), &plan); err != nil {
		t.Fatal(err)
	}
	var validated deep.ValidatedOutputV1
	if err = json.Unmarshal(prompt91LoadStageBody(t, f, store, runID, string(stages.ValidateDeep)), &validated); err != nil {
		t.Fatal(err)
	}
	projStage, err := f.repo.GetStage(ctx, runID, string(stages.BuildProjections))
	if err != nil || projStage.AcceptedAttemptID == nil {
		t.Fatal("projections missing")
	}
	projBytes := map[string][]byte{}
	projHashes := map[string]string{}
	kindBy := map[string]string{
		string(artifacts.OverviewProjection):        projection.KeyOverview,
		string(artifacts.FunnelProjection):          projection.KeyFunnel,
		string(artifacts.StockProjection):           projection.KeyStock,
		string(artifacts.AdvertisingProjection):     projection.KeyAdvertising,
		string(artifacts.ActionsSnapshotProjection): projection.KeyActionsSnapshot,
		string(artifacts.ChatContextProjection):     projection.KeyChatContext,
		string(artifacts.StockSKURows):              projection.KeyStockSKURows,
		string(artifacts.AdCampaignRows):            projection.KeyAdCampaignRows,
		string(artifacts.SKUFocusRows):              projection.KeySKUFocusRows,
	}
	for kind, key := range kindBy {
		_, loc, sha, e := f.repo.ArtifactByStageAttemptAndKind(ctx, runID, *projStage.AcceptedAttemptID, kind)
		if e != nil {
			continue
		}
		raw, _ := store.Get(ctx, loc)
		projBytes[key] = raw
		projHashes[key] = sha
	}
	return analysispg.PublishInput{
		Run: run, Lease: lease, Candidate: vc.Candidate, Plan: plan, Validated: validated,
		ProjectionBytes: projBytes, ProjectionHashes: projHashes,
		AllowedNext: stages.ProductionImplementedStageNames(), Now: time.Now().UTC(),
	}
}

func prompt132OutboxCountForSeller(ctx context.Context, f prompt81Fixture) int {
	var n int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM analysis_outbox_events
		WHERE payload_json->>'seller_account_id' = $1::text`, strconv.FormatInt(f.seller, 10)).Scan(&n)
	return n
}

func prompt132RollbackNoWrites(t *testing.T, f prompt81Fixture, store artifacts.ArtifactStore, runID int64, fp analysispg.PublishFailpoint) {
	t.Helper()
	ctx := context.Background()
	analysispg.TestPublishFailpoint = fp
	defer func() { analysispg.TestPublishFailpoint = "" }()
	_, err := f.repo.PublishAtomic(ctx, prompt132PublishInput(t, f, store, runID, "roll-"+string(fp)))
	if err == nil {
		t.Fatal("expected failpoint error")
	}
	var n int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, runID).Scan(&n)
	if n != 0 {
		t.Fatalf("publication rows=%d", n)
	}
	if prompt132OutboxCountForSeller(ctx, f) != 0 {
		t.Fatalf("outbox for seller=%d", prompt132OutboxCountForSeller(ctx, f))
	}
}

func prompt132RunP1P2(t *testing.T, assessment string) (prompt81Fixture, int64, int64) {
	t.Setenv("DEEP_FAKE_ASSESSMENT", assessment)
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-"+assessment)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	return f, r1.ID, r2.ID
}

func TestPrompt132PartiallySupportedPostgres(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "PARTIALLY_SUPPORTED")
	ctx := context.Background()
	var outcome string
	var sysState string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	if outcome != "PARTIALLY_SUPPORTED" {
		t.Fatalf("outcome=%s", outcome)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&sysState)
	if sysState != "RESOLVED" {
		t.Fatalf("system_state=%s", sysState)
	}
}

func TestPrompt132NotSupportedPostgres(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "NOT_SUPPORTED")
	ctx := context.Background()
	var outcome, sysState string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	if outcome != "NOT_SUPPORTED" {
		t.Fatalf("outcome=%s", outcome)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&sysState)
	if sysState != "RESOLVED" {
		t.Fatalf("system_state=%s", sysState)
	}
}

func TestPrompt132UnverifiablePostgres(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "UNVERIFIABLE")
	ctx := context.Background()
	var outcome, sysState string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	if outcome != "UNVERIFIABLE" {
		t.Fatalf("outcome=%s", outcome)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&sysState)
	if sysState != "SUSPENDED" {
		t.Fatalf("system_state=%s", sysState)
	}
}

func TestPrompt132SupersededPostgres(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "SUPERSEDED")
	ctx := context.Background()
	var outcome, sysState string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	if outcome != "SUPERSEDED" {
		t.Fatalf("outcome=%s", outcome)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&sysState)
	if sysState != "SUPERSEDED" {
		t.Fatalf("system_state=%s", sysState)
	}
}

func TestPrompt132SupersededOppositeIntentSignatures(t *testing.T) {
	menu := actioncatalog.Menu{Items: []actioncatalog.MenuItem{{
		ActionType:        actioncatalog.TypeSKUAdjustPrice,
		TargetKind:        "SKU",
		EligibleTargetIDs: []string{"417"},
		HardConstraints:   []string{"current_price:417=2300RUB"},
	}}}
	skuID := "417"
	hint := actioncatalog.PriceHintFromMenu(menu, "SKU", skuID)
	if hint == nil {
		t.Fatal("missing price hint")
	}
	incSig := actionreconcile.HashSignature(actionreconcile.SignatureInput{
		SellerID: 1, ActionType: actioncatalog.TypeSKUAdjustPrice,
		Target:          actionreconcile.EntityRef{Kind: "SKU", ID: skuID},
		IntentSignature: "INCREASE",
	})
	decSig := actionreconcile.HashSignature(actionreconcile.SignatureInput{
		SellerID: 1, ActionType: actioncatalog.TypeSKUAdjustPrice,
		Target:          actionreconcile.EntityRef{Kind: "SKU", ID: skuID},
		IntentSignature: "DECREASE",
	})
	if incSig == decSig {
		t.Fatal("opposite intents must produce distinct signatures")
	}
}

func TestPrompt132DoneImmediateRecurrencePostgres(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-done")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	var caseID string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID)
	t0 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DONE", "test", "", t0); err != nil {
		t.Fatal(err)
	}
	var userState string
	var suppressed *time.Time
	_ = f.pool.QueryRow(ctx, `SELECT user_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&userState)
	_ = f.pool.QueryRow(ctx, `
		SELECT ss.suppressed_until FROM action_signature_state ss
		JOIN action_cases c ON c.signature_id=ss.signature_id WHERE c.id=$1::uuid`, caseID).Scan(&suppressed)
	if userState != "DONE" {
		t.Fatalf("user_state=%s", userState)
	}
	if suppressed != nil {
		t.Fatalf("suppressed_until=%v want null", suppressed)
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "FULLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	var nCases int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, f.seller).Scan(&nCases)
	if nCases < 2 {
		t.Fatalf("cases=%d want >=2", nCases)
	}
	var c1State string
	_ = f.pool.QueryRow(ctx, `SELECT user_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&c1State)
	if c1State != "DONE" {
		t.Fatalf("c1 user_state=%s", c1State)
	}
}

func TestPrompt132DismissedOppositeIntent(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-opp")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	var caseID, actionType, skuID string
	var paramsRaw []byte
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text, p.action_type, s.target_id, p.parameters_json FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_proposals p ON p.id=(SELECT current_proposal_id FROM action_case_state WHERE case_id=c.id)
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID, &actionType, &skuID, &paramsRaw)
	t0 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DISMISSED", "test", "", t0); err != nil {
		t.Fatal(err)
	}
	var params actioncatalog.Parameters
	_ = json.Unmarshal(paramsRaw, &params)
	if params.RecommendedPrice != nil {
		params.RecommendedPrice.Amount = params.RecommendedPrice.Amount - 100
	}
	plan := actionreconcile.PlanInput{
		SellerID: f.seller, Now: t0.AddDate(0, 0, 5),
		Continuity: continuityFromFixture(f),
		Menu: actioncatalog.Menu{Items: []actioncatalog.MenuItem{{
			ActionType: actionType, TargetKind: "SKU", EligibleTargetIDs: []string{skuID},
		}}},
		Catalog: actioncatalog.Catalog(),
		Validated: deep.ValidatedOutputV1{
			Output: deep.OutputV1{
				ActionProposals: []deep.ActionProposal{{
					Ordinal: 1, ActionType: actionType,
					Target: discoveryEntitySKU(skuID), Parameters: params,
					RecommendationOrdinal: 1, FindingOrdinals: []int{1}, FactHandles: []string{"ef_001"},
				}},
			},
		},
	}
	p, rep, err := actionreconcile.Plan(plan)
	if err != nil || !rep.Valid || len(p.NewProposalIntents) == 0 {
		t.Fatalf("plan err=%v valid=%v", err, rep.Valid)
	}
	if p.NewProposalIntents[0].Disposition == "SUPPRESSED_USER_DISMISSED" {
		t.Fatal("opposite intent should not be suppressed")
	}
}

func TestPrompt132ConcurrentPublishAtomic(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-conc")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	prompt132DeactivateRun(t, f, r1.ID)
	rA := f.prodRun(t)
	prompt132RunThrough(t, exec, rA.ID, string(stages.ValidatePublication))
	prompt132DeactivateRun(t, f, rA.ID)
	rB := f.prodRun(t)
	prompt132RunThrough(t, exec, rB.ID, string(stages.ValidatePublication))
	prompt132DeactivateRun(t, f, rB.ID)
	inA := prompt132PublishInput(t, f, store, rA.ID, "conc-a")
	inB := prompt132PublishInput(t, f, store, rB.ID, "conc-b")
	var errA, errB error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var res analysispg.PublishResult
		res, errA = f.repo.PublishAtomic(ctx, inA)
		_ = res
	}()
	go func() {
		defer wg.Done()
		var res analysispg.PublishResult
		res, errB = f.repo.PublishAtomic(ctx, inB)
		_ = res
	}()
	wg.Wait()
	wins := 0
	stale := 0
	if errA == nil {
		wins++
	}
	if errB == nil {
		wins++
	}
	if errors.Is(errA, analysispg.ErrStalePublish) {
		stale++
	}
	if errors.Is(errB, analysispg.ErrStalePublish) {
		stale++
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins=%d stale=%d errA=%v errB=%v", wins, stale, errA, errB)
	}
	var nPub int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE seller_account_id=$1 AND publication_generation=2`, f.seller).Scan(&nPub)
	if nPub != 1 {
		t.Fatalf("p2 count=%d", nPub)
	}
}

func TestPrompt132PublishRollbackAfterPublicationInsert(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-roll-ins")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	prompt132RollbackNoWrites(t, f, store, r1.ID, analysispg.FailpointAfterPublicationInsert)
}

func TestPrompt132PublishRollbackAfterActionMutations(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-roll-act")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	prompt132RollbackNoWrites(t, f, store, r1.ID, analysispg.FailpointAfterActionMutations)
}

func TestPrompt132PublishRollbackAfterProjections(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-roll-proj")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	prompt132RollbackNoWrites(t, f, store, r1.ID, analysispg.FailpointAfterProjections)
}

func TestPrompt132PublishRollbackNoBusinessWrites(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-roll")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	prompt132RollbackNoWrites(t, f, store, r1.ID, analysispg.FailpointBeforeCurrentPointer)
}

func TestPrompt132PublishIdempotentRetry(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-idem")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	in := prompt132PublishInput(t, f, store, r1.ID, "idem1")
	res1, err := f.repo.PublishAtomic(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	res2, err := f.repo.PublishAtomic(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Idempotent || res1.PublicationID != res2.PublicationID {
		t.Fatalf("idempotent=%v id1=%d id2=%d", res2.Idempotent, res1.PublicationID, res2.PublicationID)
	}
	var nPub int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, r1.ID).Scan(&nPub)
	if nPub != 1 {
		t.Fatalf("pub count=%d", nPub)
	}
}

func TestPrompt132PublicationImmutabilitySQL(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-immut")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	immutableTables := []struct{ table, col, val string }{
		{"analysis_publications", "publication_content_hash", "deadbeef"},
		{"publication_components", "sha256", "deadbeef"},
		{"publication_findings", "statement", "x"},
		{"publication_recommendations", "summary", "x"},
		{"publication_projections", "payload_hash", "deadbeef"},
	}
	for _, it := range immutableTables {
		_, err := f.pool.Exec(ctx, `UPDATE `+it.table+` SET `+it.col+`=$2 WHERE publication_id=$1`, p1.ID, it.val)
		if err == nil {
			t.Fatalf("expected immutability reject on %s", it.table)
		}
	}
}

func TestPrompt132PublicationHashStableAfterDone(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-hash")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var hashBefore string
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, p1.ID).Scan(&hashBefore)
	var caseID string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID)
	_ = f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DONE", "test", "", time.Now().UTC())
	var hashAfter string
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, p1.ID).Scan(&hashAfter)
	if hashBefore != hashAfter {
		t.Fatalf("hash changed %s -> %s", hashBefore, hashAfter)
	}
}

func TestPrompt132OutboxAtomicityOnRollback(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-outbox")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	prompt132RollbackNoWrites(t, f, store, r1.ID, analysispg.FailpointBeforeCurrentPointer)
}

func TestPrompt132PostPublishOutboxRetry(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-post")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	p2, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var pending int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM analysis_outbox_events
		WHERE payload_json->>'seller_account_id'=$1 AND processed_at IS NULL`,
		strconv.FormatInt(f.seller, 10)).Scan(&pending)
	if pending < 1 {
		t.Fatalf("pending outbox=%d", pending)
	}
	n1, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n1 < 1 {
		t.Fatalf("processed=%d err=%v", n1, err)
	}
	n2, _ := f.repo.ProcessOutbox(ctx, 10)
	if p2.ID == 0 {
		t.Fatal("publication lost")
	}
	_ = n2
}

func TestPrompt132DuplicateOutboxProcessing(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p132-dup-obx")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	var eventID string
	_ = f.pool.QueryRow(ctx, `
		SELECT id::text FROM analysis_outbox_events
		WHERE payload_json->>'seller_account_id'=$1 AND event_type='publication.current_changed' LIMIT 1`,
		strconv.FormatInt(f.seller, 10)).Scan(&eventID)
	_, _ = f.repo.ProcessOutbox(ctx, 10)
	_, err := f.pool.Exec(ctx, `UPDATE analysis_outbox_events SET processed_at=NULL WHERE id=$1::uuid`, eventID)
	if err != nil {
		t.Fatal(err)
	}
	n, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n < 1 {
		t.Fatalf("reprocess n=%d err=%v", n, err)
	}
	var pubCount int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE seller_account_id=$1`, f.seller).Scan(&pubCount)
	if pubCount != 1 {
		t.Fatalf("duplicate publication count=%d", pubCount)
	}
}

func TestPrompt132ProjectionScaleNoNPlusOne(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p132-scale")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.BuildProjections))
	man := loadProjectionManifest(t, f, store, r1.ID)
	if man.QueryCounts["ListSKUStock"] > 2 {
		t.Fatalf("ListSKUStock=%d want <=2 for ~12 SKUs", man.QueryCounts["ListSKUStock"])
	}
	if man.QueryCounts["ListCampaigns"] > 2 {
		t.Fatalf("ListCampaigns=%d", man.QueryCounts["ListCampaigns"])
	}
}

func TestPrompt132ProjectionScale500And2000SKU(t *testing.T) {
	pool, counter := newCountedPool(t)
	f500 := prompt94ScaleSeed(t, pool, 500)
	exec500, store500 := prompt132Exec(t, f500, "p132-scale-500")
	r500 := f500.prodRun(t)
	start := time.Now()
	prompt132RunThrough(t, exec500, r500.ID, string(stages.BuildProjections))
	dur500 := time.Since(start)
	man500 := loadProjectionManifest(t, f500, store500, r500.ID)
	sql500 := counter.Count()
	t.Logf("500 SKU: sql=%d duration=%s stock_calls=%d campaigns=%d",
		sql500, dur500, man500.QueryCounts["ListSKUStock"], man500.QueryCounts["ListCampaigns"])

	counter.Reset()
	f2000 := prompt94ScaleSeed(t, pool, 2000)
	exec2000, store2000 := prompt132Exec(t, f2000, "p132-scale-2000")
	r2000 := f2000.prodRun(t)
	start = time.Now()
	prompt132RunThrough(t, exec2000, r2000.ID, string(stages.BuildProjections))
	dur2000 := time.Since(start)
	man2000 := loadProjectionManifest(t, f2000, store2000, r2000.ID)
	sql2000 := counter.Count()
	t.Logf("2000 SKU: sql=%d duration=%s stock_calls=%d campaigns=%d",
		sql2000, dur2000, man2000.QueryCounts["ListSKUStock"], man2000.QueryCounts["ListCampaigns"])

	if man2000.QueryCounts["ListSKUStock"] > man500.QueryCounts["ListSKUStock"]*4+4 {
		t.Fatalf("N+1 ListSKUStock 500=%d 2000=%d", man500.QueryCounts["ListSKUStock"], man2000.QueryCounts["ListSKUStock"])
	}
}

func loadProjectionManifest(t *testing.T, f prompt81Fixture, store artifacts.ArtifactStore, runID int64) projection.Manifest {
	t.Helper()
	body := prompt91LoadStageBody(t, f, store, runID, string(stages.BuildProjections))
	var man projection.Manifest
	if err := json.Unmarshal(body, &man); err != nil {
		t.Fatal(err)
	}
	return man
}

func continuityFromFixture(f prompt81Fixture) continuity.ContinuityContextV1 {
	return continuity.ContinuityContextV1{Mode: continuity.ModeFromPublication, ContinuityContractVersion: continuity.ContractFromPublicationV1}
}

func discoveryEntitySKU(id string) deep.EntityRef {
	return deep.EntityRef{Kind: "SKU", ID: id}
}
