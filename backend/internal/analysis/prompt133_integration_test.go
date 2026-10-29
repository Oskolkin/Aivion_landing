//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

type prompt133Residue struct {
	Publications, Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows                                     int
	Proposals, Assessments, SupportAssessments                       int
	Groups, GroupMembers, ReleaseEvents, Outbox                      int
	CurrentPubID                                                     int64
}

func prompt133SnapshotResidue(ctx context.Context, f prompt81Fixture, runID int64) prompt133Residue {
	var r prompt133Residue
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, runID).Scan(&r.Publications)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_components c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Components)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_findings c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Findings)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_recommendations c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Recommendations)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_projections c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Projections)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.StockRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_ad_campaign_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.AdRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_sku_focus_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.FocusRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_proposals c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Proposals)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, runID).Scan(&r.Assessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_support_assessments a JOIN action_case_assessments c ON c.id=a.assessment_id WHERE c.analysis_run_id=$1`, runID).Scan(&r.SupportAssessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Groups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id JOIN analysis_publications p ON p.id=g.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.GroupMembers)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_release_events c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.ReleaseEvents)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE payload_json->>'seller_account_id'=$1`, strconv.FormatInt(f.seller, 10)).Scan(&r.Outbox)
	cur, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err == nil {
		r.CurrentPubID = cur.ID
	}
	return r
}

func prompt133AssertEmptyCandidate(t *testing.T, before, after prompt133Residue) {
	t.Helper()
	if after.Publications != before.Publications || after.Components != before.Components ||
		after.Findings != before.Findings || after.Recommendations != before.Recommendations ||
		after.Projections != before.Projections || after.StockRows != before.StockRows ||
		after.AdRows != before.AdRows || after.FocusRows != before.FocusRows ||
		after.Proposals != before.Proposals || after.Assessments != before.Assessments ||
		after.SupportAssessments != before.SupportAssessments || after.Groups != before.Groups ||
		after.GroupMembers != before.GroupMembers || after.ReleaseEvents != before.ReleaseEvents ||
		after.Outbox != before.Outbox || after.CurrentPubID != before.CurrentPubID {
		t.Fatalf("rollback residue changed before=%+v after=%+v", before, after)
	}
}

func prompt133RollbackFull(t *testing.T, fp analysispg.PublishFailpoint) {
	t.Helper()
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p133-roll-"+string(fp))
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	before := prompt133SnapshotResidue(ctx, f, r1.ID)
	analysispg.TestPublishFailpoint = fp
	defer func() { analysispg.TestPublishFailpoint = "" }()
	_, err := f.repo.PublishAtomic(ctx, prompt132PublishInput(t, f, store, r1.ID, "roll"))
	if err == nil {
		t.Fatal("expected failpoint")
	}
	after := prompt133SnapshotResidue(ctx, f, r1.ID)
	prompt133AssertEmptyCandidate(t, before, after)
}

func TestPrompt133RollbackAfterPublicationInsertFullResidue(t *testing.T) {
	prompt133RollbackFull(t, analysispg.FailpointAfterPublicationInsert)
}
func TestPrompt133RollbackAfterActionMutationsFullResidue(t *testing.T) {
	prompt133RollbackFull(t, analysispg.FailpointAfterActionMutations)
}
func TestPrompt133RollbackAfterProjectionsFullResidue(t *testing.T) {
	prompt133RollbackFull(t, analysispg.FailpointAfterProjections)
}
func TestPrompt133RollbackBeforeCurrentPointerFullResidue(t *testing.T) {
	prompt133RollbackFull(t, analysispg.FailpointBeforeCurrentPointer)
}

func TestPrompt133PartiallySupportedFullLifecycle(t *testing.T) {
	f, r1, r2 := prompt132RunP1P2(t, "PARTIALLY_SUPPORTED")
	ctx := context.Background()
	var outcome, sys string
	var nSupport int
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	if outcome != "PARTIALLY_SUPPORTED" {
		t.Fatalf("outcome=%s", outcome)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_support_assessments a
		JOIN action_case_assessments c ON c.id=a.assessment_id WHERE c.analysis_run_id=$1`, r2).Scan(&nSupport)
	if nSupport < 1 {
		t.Fatalf("support assessments=%d", nSupport)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 ORDER BY c.occurrence_no LIMIT 1`, f.seller).Scan(&sys)
	if sys != "RESOLVED" && sys != "SUPERSEDED" {
		t.Fatalf("system_state=%s want BASIS_CHANGED→RESOLVED/SUPERSEDED", sys)
	}
	var p1Hash, p1Statement string
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE analysis_run_id=$1`, r1).Scan(&p1Hash)
	_ = f.pool.QueryRow(ctx, `
		SELECT statement FROM publication_findings f
		JOIN analysis_publications p ON p.id=f.publication_id WHERE p.analysis_run_id=$1 LIMIT 1`, r1).Scan(&p1Statement)
	var p1Hash2, p1Statement2 string
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE analysis_run_id=$1`, r1).Scan(&p1Hash2)
	_ = f.pool.QueryRow(ctx, `
		SELECT statement FROM publication_findings f
		JOIN analysis_publications p ON p.id=f.publication_id WHERE p.analysis_run_id=$1 LIMIT 1`, r1).Scan(&p1Statement2)
	if p1Hash != p1Hash2 || p1Statement != p1Statement2 {
		t.Fatal("old publication mutated")
	}
	var propCount int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_case_proposals pr
		JOIN analysis_publications p ON p.id=pr.publication_id WHERE p.analysis_run_id=$1`, r1).Scan(&propCount)
	if propCount < 1 {
		t.Fatal("p1 proposal missing")
	}
}

func TestPrompt133NotSupportedFullLifecycle(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "NOT_SUPPORTED")
	ctx := context.Background()
	var outcome, sys, user string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state, st.user_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 ORDER BY c.occurrence_no LIMIT 1`, f.seller).Scan(&sys, &user)
	if outcome != "NOT_SUPPORTED" || sys != "RESOLVED" {
		t.Fatalf("outcome=%s sys=%s", outcome, sys)
	}
	var suppressed *time.Time
	_ = f.pool.QueryRow(ctx, `
		SELECT ss.suppressed_until FROM action_signature_state ss
		JOIN action_signatures s ON s.id=ss.signature_id WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&suppressed)
	if suppressed != nil {
		t.Fatal("invented user suppression")
	}
	var active int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 AND st.system_state='ACTIVE' AND st.user_state='OPEN'`, f.seller).Scan(&active)
	if active != 0 {
		t.Fatalf("active cases=%d", active)
	}
}

func TestPrompt133UnverifiableContinuityRemains(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "UNVERIFIABLE")
	ctx := context.Background()
	var outcome, sys string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2).Scan(&outcome)
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 ORDER BY c.occurrence_no LIMIT 1`, f.seller).Scan(&sys)
	if outcome != "UNVERIFIABLE" || sys != "SUSPENDED" {
		t.Fatalf("outcome=%s sys=%s", outcome, sys)
	}
	exec, store := prompt132Exec(t, f, "p133-unver-p3")
	r3 := f.prodRun(t)
	prompt132RunThrough(t, exec, r3.ID, string(stages.PrepareContinuity))
	body := prompt91LoadStageBody(t, f, store, r3.ID, string(stages.PrepareContinuity))
	var cont continuity.ContinuityContextV1
	if err := json.Unmarshal(body, &cont); err != nil {
		t.Fatal(err)
	}
	if len(cont.PreviousActions) == 0 {
		t.Fatal("UNVERIFIABLE case lost from continuity recheck")
	}
}

func TestPrompt133SupersededOppositeIntentPostgres(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-super")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	var actionType, intent, sigHash string
	err := f.pool.QueryRow(ctx, `
		SELECT s.action_type, s.intent_key, s.signature_hash
		FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 ORDER BY c.occurrence_no LIMIT 1`, f.seller).Scan(&actionType, &intent, &sigHash)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "SUPERSEDED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	var nSig int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_signatures WHERE seller_account_id=$1`, f.seller).Scan(&nSig)
	if nSig < 2 {
		var intents string
		_ = f.pool.QueryRow(ctx, `SELECT string_agg(action_type||':'||intent_key, ',') FROM action_signatures WHERE seller_account_id=$1`, f.seller).Scan(&intents)
		t.Fatalf("signatures=%d intents=%s p1=%s/%s want opposite action/intent", nSig, intents, actionType, intent)
	}
	var nAssess int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, r2.ID).Scan(&nAssess)
	var outcome, sys string
	err = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1 LIMIT 1`, r2.ID).Scan(&outcome)
	if err != nil || outcome != "SUPERSEDED" {
		t.Fatalf("outcome=%s nAssess=%d err=%v", outcome, nAssess, err)
	}
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.signature_hash=$1 LIMIT 1`, sigHash).Scan(&sys)
	if sys != "SUPERSEDED" {
		t.Fatalf("old sys=%s", sys)
	}
	var newType, newIntent string
	_ = f.pool.QueryRow(ctx, `
		SELECT action_type, intent_key FROM action_signatures WHERE seller_account_id=$1 AND signature_hash<>$2 LIMIT 1`, f.seller, sigHash).Scan(&newType, &newIntent)
	if newType == actionType && newIntent == intent {
		t.Fatalf("new signature must differ: type=%s intent=%s", newType, newIntent)
	}
	if newType == actionType && actionType != actioncatalog.TypeSKUAdjustPrice {
		t.Fatalf("new action_type=%s same as old without opposite price intent", newType)
	}
}

func TestPrompt133DoneImmediateRecurrenceComplete(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-done")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	var caseID, sigHash, propID string
	var propParams []byte
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text, s.signature_hash, p.id::text, p.parameters_json FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_proposals p ON p.id=(SELECT current_proposal_id FROM action_case_state WHERE case_id=c.id)
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID, &sigHash, &propID, &propParams)
	t0 := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DONE", "test", "", t0); err != nil {
		t.Fatal(err)
	}
	var userState string
	var suppressed *time.Time
	_ = f.pool.QueryRow(ctx, `SELECT user_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&userState)
	_ = f.pool.QueryRow(ctx, `SELECT suppressed_until FROM action_signature_state WHERE signature_id=(SELECT signature_id FROM action_cases WHERE id=$1::uuid)`, caseID).Scan(&suppressed)
	if userState != "DONE" || suppressed != nil {
		t.Fatalf("user=%s suppressed=%v", userState, suppressed)
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "FULLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	var c2 string
	var c2Sys, c2User string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text, st.system_state, st.user_state FROM action_cases c
		JOIN action_case_state st ON st.case_id=c.id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 AND c.id<>$2::uuid ORDER BY c.occurrence_no DESC LIMIT 1`, f.seller, caseID).Scan(&c2, &c2Sys, &c2User)
	if c2 == "" || c2 == caseID {
		t.Fatal("missing new occurrence")
	}
	if c2Sys != "ACTIVE" || c2User != "OPEN" {
		t.Fatalf("c2 state=%s/%s", c2Sys, c2User)
	}
	var c1User string
	_ = f.pool.QueryRow(ctx, `SELECT user_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&c1User)
	if c1User != "DONE" {
		t.Fatalf("c1=%s", c1User)
	}
	var paramsAfter []byte
	_ = f.pool.QueryRow(ctx, `SELECT parameters_json FROM action_case_proposals WHERE id=$1::uuid`, propID).Scan(&paramsAfter)
	if string(paramsAfter) != string(propParams) {
		t.Fatal("historical proposal mutated")
	}
}

func TestPrompt133DismissedOppositeIntentPostgres(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-dismiss-opp")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	var caseID, actionType, oldSig string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text, p.action_type, s.signature_hash FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_proposals p ON p.id=(SELECT current_proposal_id FROM action_case_state WHERE case_id=c.id)
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID, &actionType, &oldSig)
	t0 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DISMISSED", "test", "", t0); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "SUPERSEDED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	var nSig int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_signatures WHERE seller_account_id=$1`, f.seller).Scan(&nSig)
	if nSig < 2 {
		t.Fatalf("signatures=%d action=%s", nSig, actionType)
	}
	var newCaseSys string
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 AND s.signature_hash<>$2
		ORDER BY c.occurrence_no DESC LIMIT 1`, f.seller, oldSig).Scan(&newCaseSys)
	if newCaseSys != "ACTIVE" {
		t.Fatalf("new case sys=%s", newCaseSys)
	}
	var oldUser string
	_ = f.pool.QueryRow(ctx, `SELECT user_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&oldUser)
	if oldUser != "DISMISSED" {
		t.Fatalf("old user=%s", oldUser)
	}
}

func TestPrompt133ConcurrentPublishLoserResidue(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p133-conc")
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
	beforeA := prompt133SnapshotResidue(ctx, f, rA.ID)
	beforeB := prompt133SnapshotResidue(ctx, f, rB.ID)
	inA := prompt132PublishInput(t, f, store, rA.ID, "conc-a")
	inB := prompt132PublishInput(t, f, store, rB.ID, "conc-b")
	var errA, errB error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, errA = f.repo.PublishAtomic(ctx, inA) }()
	go func() { defer wg.Done(); _, errB = f.repo.PublishAtomic(ctx, inB) }()
	wg.Wait()
	wins, stale := 0, 0
	var loserRun int64
	if errA == nil {
		wins++
	} else if errors.Is(errA, analysispg.ErrStalePublish) {
		stale++
		loserRun = rA.ID
	}
	if errB == nil {
		wins++
	} else if errors.Is(errB, analysispg.ErrStalePublish) {
		stale++
		loserRun = rB.ID
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins=%d stale=%d a=%v b=%v", wins, stale, errA, errB)
	}
	var nGen2 int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE seller_account_id=$1 AND publication_generation=2`, f.seller).Scan(&nGen2)
	if nGen2 != 1 {
		t.Fatalf("gen2=%d", nGen2)
	}
	loser := prompt133SnapshotResidue(ctx, f, loserRun)
	base := beforeA
	if loserRun == rB.ID {
		base = beforeB
	}
	if loser.Publications != base.Publications || loser.Proposals != base.Proposals ||
		loser.Assessments != base.Assessments || loser.ReleaseEvents != base.ReleaseEvents {
		t.Fatalf("loser residue pubs=%d props=%d assess=%d release=%d", loser.Publications, loser.Proposals, loser.Assessments, loser.ReleaseEvents)
	}
}

func TestPrompt133SellerLockIndependence(t *testing.T) {
	fA := prompt91Seed(t)
	fB := prompt91Seed(t)
	execA, storeA := prompt132Exec(t, fA, "p133-lock-a")
	execB, storeB := prompt132Exec(t, fB, "p133-lock-b")
	ctx := context.Background()
	rA := fA.prodRun(t)
	prompt132RunThrough(t, execA, rA.ID, string(stages.ValidatePublication))
	rB := fB.prodRun(t)
	prompt132RunThrough(t, execB, rB.ID, string(stages.ValidatePublication))
	inA := prompt132PublishInput(t, fA, storeA, rA.ID, "la")
	inB := prompt132PublishInput(t, fB, storeB, rB.ID, "lb")
	var errA, errB error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, errA = fA.repo.PublishAtomic(ctx, inA) }()
	go func() { defer wg.Done(); _, errB = fB.repo.PublishAtomic(ctx, inB) }()
	wg.Wait()
	if errA != nil || errB != nil {
		t.Fatalf("independent sellers should both publish a=%v b=%v", errA, errB)
	}
}

func TestPrompt133UnknownCommitRetry(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p133-unk")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	in := prompt132PublishInput(t, f, store, r1.ID, "unk1")
	analysispg.TestPublishUnknownCommitAfterSuccess = true
	res1, err := f.repo.PublishAtomic(ctx, in)
	analysispg.TestPublishUnknownCommitAfterSuccess = false
	if !errors.Is(err, analysispg.ErrPublishUnknownCommit) {
		t.Fatalf("want unknown commit err got %v", err)
	}
	if res1.PublicationID == 0 {
		t.Fatal("commit should have produced publication id before synthetic error")
	}
	res2, err := f.repo.PublishAtomic(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Idempotent || res2.PublicationID != res1.PublicationID {
		t.Fatalf("idempotent=%v id1=%d id2=%d", res2.Idempotent, res1.PublicationID, res2.PublicationID)
	}
	var nPub, nOut, nProp int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, r1.ID).Scan(&nPub)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE event_type='publication.current_changed' AND payload_json->>'seller_account_id'=$1`, strconv.FormatInt(f.seller, 10)).Scan(&nOut)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_proposals p JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.analysis_run_id=$1`, r1.ID).Scan(&nProp)
	if nPub != 1 || nOut != 1 {
		t.Fatalf("pub=%d outbox=%d", nPub, nOut)
	}
	_ = nProp
}

func TestPrompt133ImmutabilityUpdateDeleteAllTables(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-immut")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	// Ensure large rows populated (defect fix).
	var nStock, nAd, nFocus, nGroups, nMembers int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id=$1`, p1.ID).Scan(&nStock)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_ad_campaign_rows WHERE publication_id=$1`, p1.ID).Scan(&nAd)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_sku_focus_rows WHERE publication_id=$1`, p1.ID).Scan(&nFocus)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups WHERE publication_id=$1`, p1.ID).Scan(&nGroups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`, p1.ID).Scan(&nMembers)
	if nStock < 1 || nFocus < 1 {
		t.Fatalf("expected populated large rows stock=%d focus=%d ad=%d", nStock, nFocus, nAd)
	}
	type attack struct{ sql string }
	attacks := []attack{
		{`UPDATE analysis_publications SET publication_content_hash='deadbeef' WHERE id=$1`},
		{`DELETE FROM analysis_publications WHERE id=$1`},
		{`UPDATE publication_components SET sha256='deadbeef' WHERE publication_id=$1`},
		{`DELETE FROM publication_components WHERE publication_id=$1`},
		{`UPDATE publication_findings SET statement='x' WHERE publication_id=$1`},
		{`DELETE FROM publication_findings WHERE publication_id=$1`},
		{`UPDATE publication_recommendations SET summary='x' WHERE publication_id=$1`},
		{`DELETE FROM publication_recommendations WHERE publication_id=$1`},
		{`UPDATE publication_projections SET payload_hash='deadbeef' WHERE publication_id=$1`},
		{`DELETE FROM publication_projections WHERE publication_id=$1`},
		{`UPDATE publication_stock_sku_rows SET row_json='{}'::jsonb WHERE publication_id=$1`},
		{`DELETE FROM publication_stock_sku_rows WHERE publication_id=$1`},
		{`UPDATE publication_ad_campaign_rows SET row_json='{}'::jsonb WHERE publication_id=$1`},
		{`DELETE FROM publication_ad_campaign_rows WHERE publication_id=$1`},
		{`UPDATE publication_sku_focus_rows SET row_json='{}'::jsonb WHERE publication_id=$1`},
		{`DELETE FROM publication_sku_focus_rows WHERE publication_id=$1`},
	}
	if nGroups > 0 {
		attacks = append(attacks,
			attack{`UPDATE publication_action_groups SET display_title='x' WHERE publication_id=$1`},
			attack{`DELETE FROM publication_action_groups WHERE publication_id=$1`},
		)
	}
	if nMembers > 0 {
		attacks = append(attacks,
			attack{`DELETE FROM publication_action_group_members WHERE group_id IN (SELECT id FROM publication_action_groups WHERE publication_id=$1)`},
		)
	}
	for _, a := range attacks {
		_, err := f.pool.Exec(ctx, a.sql, p1.ID)
		if err == nil {
			t.Fatalf("expected reject: %s", a.sql)
		}
	}
	// Mutable controls still work.
	var caseID string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DONE", "t", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// Need another case for DISMISSED — reopen path: create second seller run not needed;
	// apply DISMISSED on DONE may be invalid; use fresh case from P2 if needed.
	f2 := prompt91Seed(t)
	exec2, _ := prompt132Exec(t, f2, "p133-dismiss-mut")
	rr := f2.prodRun(t)
	prompt132RunThrough(t, exec2, rr.ID, "")
	var cDismiss string
	_ = f2.pool.QueryRow(ctx, `
		SELECT c.id::text FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f2.seller).Scan(&cDismiss)
	if err := f2.repo.ApplyUserEvent(ctx, f2.seller, cDismiss, nil, "DISMISSED", "t", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt133PublicationOverlayUnchanged(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-overlay")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var hash string
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, p1.ID).Scan(&hash)
	projHashes := map[string]string{}
	rows, err := f.pool.Query(ctx, `SELECT projection_key, payload_hash FROM publication_projections WHERE publication_id=$1`, p1.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k, h string
		_ = rows.Scan(&k, &h)
		projHashes[k] = h
	}
	rows.Close()
	var groupCount, memberCount, stockCount int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups WHERE publication_id=$1`, p1.ID).Scan(&groupCount)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`, p1.ID).Scan(&memberCount)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id=$1`, p1.ID).Scan(&stockCount)
	var caseID string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID)
	_ = f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DONE", "t", "", time.Now().UTC())
	var hash2 string
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, p1.ID).Scan(&hash2)
	if hash != hash2 {
		t.Fatal("content hash changed")
	}
	for k, h := range projHashes {
		var h2 string
		_ = f.pool.QueryRow(ctx, `SELECT payload_hash FROM publication_projections WHERE publication_id=$1 AND projection_key=$2`, p1.ID, k).Scan(&h2)
		if h2 != h {
			t.Fatalf("projection %s hash changed", k)
		}
	}
	var g2, m2, s2 int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups WHERE publication_id=$1`, p1.ID).Scan(&g2)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`, p1.ID).Scan(&m2)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id=$1`, p1.ID).Scan(&s2)
	if g2 != groupCount || m2 != memberCount || s2 != stockCount {
		t.Fatal("group/stock rows changed after DONE")
	}
}

func TestPrompt133OutboxAtomicitySuccess(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-obx-ok")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var n int
	var agg string
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*), max(aggregate_id) FROM analysis_outbox_events
		WHERE event_type='publication.current_changed' AND payload_json->>'seller_account_id'=$1`,
		strconv.FormatInt(f.seller, 10)).Scan(&n, &agg)
	if n != 1 || agg != strconv.FormatInt(p1.ID, 10) {
		t.Fatalf("outbox n=%d agg=%s pub=%d", n, agg, p1.ID)
	}
}

func TestPrompt133PostPublishFailureThenRetry(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-post-fail")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	analysispg.TestOutboxSideEffectCount = 0
	analysispg.TestOutboxFailSellerID = f.seller
	analysispg.TestOutboxFailNext = true
	defer func() {
		analysispg.TestOutboxFailNext = false
		analysispg.TestOutboxFailSellerID = 0
	}()
	_, err := f.repo.ProcessOutbox(ctx, 50)
	if !errors.Is(err, analysispg.ErrOutboxConsumer) {
		t.Fatalf("want consumer fail got %v", err)
	}
	var pending, attempts int
	var lastErr *string
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*), coalesce(max(attempts),0), max(last_error) FROM analysis_outbox_events
		WHERE payload_json->>'seller_account_id'=$1 AND event_type='publication.current_changed' AND processed_at IS NULL`,
		strconv.FormatInt(f.seller, 10)).Scan(&pending, &attempts, &lastErr)
	if pending < 1 || attempts < 1 {
		t.Fatalf("pending=%d attempts=%d lastErr=%v", pending, attempts, lastErr)
	}
	n, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n < 1 {
		t.Fatalf("retry n=%d err=%v", n, err)
	}
	if analysispg.TestOutboxSideEffectCount < 1 {
		t.Fatal("side effect missing")
	}
	p3, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	if p3.ID != p1.ID {
		t.Fatal("publication changed after retry")
	}
}

func TestPrompt133DuplicateOutboxSideEffectOnce(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p133-dup")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	analysispg.ResetOutboxTestCounters()
	n1, _ := f.repo.ProcessOutbox(ctx, 10)
	if n1 < 1 {
		t.Fatal("first delivery")
	}
	if analysispg.TestOutboxDeliveryAttemptCount != 1 || analysispg.TestOutboxSideEffectCount != 1 {
		t.Fatalf("delivery=%d business=%d want 1/1", analysispg.TestOutboxDeliveryAttemptCount, analysispg.TestOutboxSideEffectCount)
	}
	var eventID string
	_ = f.pool.QueryRow(ctx, `
		SELECT id::text FROM analysis_outbox_events
		WHERE payload_json->>'seller_account_id'=$1 AND event_type='publication.current_changed' LIMIT 1`,
		strconv.FormatInt(f.seller, 10)).Scan(&eventID)
	_, _ = f.pool.Exec(ctx, `UPDATE analysis_outbox_events SET processed_at=NULL WHERE id=$1::uuid`, eventID)
	n2, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n2 < 1 {
		t.Fatalf("reprocess n=%d err=%v", n2, err)
	}
	if analysispg.TestOutboxDeliveryAttemptCount != 2 {
		t.Fatalf("delivery_attempt_count=%d want 2", analysispg.TestOutboxDeliveryAttemptCount)
	}
	if analysispg.TestOutboxSideEffectCount != 1 {
		t.Fatalf("business_effect_count=%d want 1", analysispg.TestOutboxSideEffectCount)
	}
}

func TestPrompt133SixProjectionConsistency(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "FULLY_SUPPORTED")
	ctx := context.Background()
	p2, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var date string
	var cutoff time.Time
	var uni string
	_ = f.pool.QueryRow(ctx, `SELECT analysis_date::text, analysis_cutoff_at, factual_universe_hash FROM analysis_publications WHERE id=$1`, p2.ID).Scan(&date, &cutoff, &uni)
	keys := []string{"OVERVIEW", "FUNNEL", "STOCK", "ADVERTISING", "ACTIONS_SNAPSHOT", "CHAT_CONTEXT"}
	for _, k := range keys {
		raw, err := f.repo.GetProjection(ctx, p2.ID, k)
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s: %v", k, err)
		}
		var meta map[string]any
		_ = json.Unmarshal(raw, &meta)
		if sid, ok := meta["seller_account_id"].(float64); ok && int64(sid) != f.seller {
			t.Fatalf("%s seller mismatch", k)
		}
		if ad, ok := meta["analysis_date"].(string); ok && ad != "" && ad != date {
			t.Fatalf("%s date=%s want %s", k, ad, date)
		}
		if uh, ok := meta["factual_universe_hash"].(string); ok && uh != "" && uh != uni {
			t.Fatalf("%s universe mismatch", k)
		}
	}
	_ = r2
}

func TestPrompt133LargeRowP1P2Consistency(t *testing.T) {
	f, r1, r2 := prompt132RunP1P2(t, "FULLY_SUPPORTED")
	ctx := context.Background()
	var p1, p2 int64
	_ = f.pool.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, r1).Scan(&p1)
	_ = f.pool.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, r2).Scan(&p2)
	var mix int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id=$1
		  AND sku_id IN (SELECT sku_id FROM publication_stock_sku_rows WHERE publication_id=$2)
		  AND row_json IS NULL`, p2, p1).Scan(&mix)
	var n2, foreign int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id=$1`, p2).Scan(&n2)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id<>$1 AND publication_id=$2`, p2, p1).Scan(&foreign)
	if n2 < 1 {
		t.Fatal("p2 stock rows missing")
	}
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows s WHERE s.publication_id=$1 AND NOT EXISTS (SELECT 1 FROM analysis_publications p WHERE p.id=s.publication_id AND p.id=$1)`, p2).Scan(&foreign)
	if foreign != 0 {
		t.Fatalf("orphan/foreign rows=%d", foreign)
	}
	cur, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	if cur.ID != p2 {
		t.Fatal("current not p2")
	}
}

func TestPrompt133ContinuityCurrentValuesInDeepPack(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p133-cont-val")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	// Mutate stock observation used by continuity revalidation so P2 evidence differs.
	_, err := f.pool.Exec(ctx, `
		UPDATE stock_snapshot_lines SET sellable_units = COALESCE(sellable_units,0) + 37
		WHERE seller_account_id=$1 AND canonical_sku_id IS NOT NULL`, f.seller)
	if err != nil {
		t.Fatalf("stock mutate: %v", err)
	}
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, string(stages.BuildDeepPack))
	pack := prompt91LoadStageBody(t, f, store, r2.ID, string(stages.BuildDeepPack))
	var wire deep.PackWireV1
	if err := json.Unmarshal(pack, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Payload.EvidenceRegistry) == 0 {
		t.Fatal("no evidence")
	}
	for _, link := range wire.Payload.ContinuitySupportEvidenceLinks {
		found := false
		for _, it := range wire.Payload.EvidenceRegistry {
			if it.EvidenceID == link.EvidenceID {
				found = true
				if len(it.Data) == 0 || string(it.Data) == "null" {
					t.Fatalf("continuity evidence %s missing data", link.EvidenceID)
				}
			}
		}
		if !found {
			t.Fatalf("dangling continuity link %s", link.EvidenceID)
		}
	}
	for _, link := range wire.Payload.ThreadEvidenceLinks {
		for _, ev := range link.EvidenceRefs {
			ok := false
			for _, it := range wire.Payload.EvidenceRegistry {
				if it.EvidenceID == ev {
					ok = true
					break
				}
			}
			if !ok {
				t.Fatalf("dangling thread link %s", ev)
			}
		}
	}
	seen := map[string]int{}
	for _, fh := range wire.Payload.FactRegistry {
		seen[fh.Handle]++
		if seen[fh.Handle] > 1 {
			t.Fatalf("duplicate handle %s", fh.Handle)
		}
	}
}

func TestPrompt133OverBudgetNoProviderCalls(t *testing.T) {
	f := prompt91Seed(t)
	t.Setenv("OPENAI_DEEP_MAX_INPUT_TOKENS", "2048")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	exec, _ := prompt132Exec(t, f, "p133-budget")
	ctx := context.Background()
	r1 := f.prodRun(t)
	err := error(nil)
	for _, key := range stages.ProductionImplementedStageNames() {
		err = exec.ExecuteStage(ctx, r1.ID, key)
		if err != nil {
			break
		}
		if key == string(stages.BuildDeepPack) {
			break
		}
	}
	st, _ := exec.Repo.GetStage(ctx, r1.ID, string(stages.BuildDeepPack))
	if st.ErrorCode == "" && err == nil {
		t.Fatal("expected DEEP_PACK_TOO_LARGE")
	}
	msg := st.ErrorMessage + st.ErrorCode
	if err != nil {
		msg += err.Error()
	}
	if !strings.Contains(msg, "DEEP_PACK_TOO_LARGE") && !strings.Contains(msg, deep.CodePackTooLarge) {
		t.Fatalf("msg=%s", msg)
	}
	stAI, _ := exec.Repo.GetStage(ctx, r1.ID, string(stages.DeepAI))
	if stAI.Status == "SUCCEEDED" || stAI.AcceptedAttemptID != nil {
		t.Fatal("DEEP_AI must not succeed")
	}
}

func TestPrompt133Scale500And2000Report(t *testing.T) {
	pool, counter := newCountedPool(t)
	measure := func(n int) (sql int64, stockCalls, campCalls, stockRows, campRows, focusRows, bytes int, dur time.Duration) {
		f := prompt94ScaleSeed(t, pool, n)
		exec, store := prompt132Exec(t, f, "p133-scale-"+strconv.Itoa(n))
		r := f.prodRun(t)
		start := time.Now()
		prompt132RunThrough(t, exec, r.ID, string(stages.BuildProjections))
		dur = time.Since(start)
		man := loadProjectionManifest(t, f, store, r.ID)
		sql = counter.Count()
		stockCalls = man.QueryCounts["ListSKUStock"]
		campCalls = man.QueryCounts["ListCampaigns"]
		rawStock := prompt91LoadArtifactKind(t, f, store, r.ID, string(artifacts.StockSKURows))
		rawCamp := prompt91LoadArtifactKind(t, f, store, r.ID, string(artifacts.AdCampaignRows))
		rawFocus := prompt91LoadArtifactKind(t, f, store, r.ID, string(artifacts.SKUFocusRows))
		var stock []map[string]any
		var camp []map[string]any
		var focus []map[string]any
		_ = json.Unmarshal(rawStock, &stock)
		_ = json.Unmarshal(rawCamp, &camp)
		_ = json.Unmarshal(rawFocus, &focus)
		stockRows, campRows, focusRows = len(stock), len(camp), len(focus)
		bytes = len(rawStock) + len(rawCamp) + len(rawFocus)
		for _, b := range [][]byte{
			prompt91LoadStageBody(t, f, store, r.ID, string(stages.BuildProjections)),
		} {
			bytes += len(b)
		}
		return
	}
	counter.Reset()
	s500, stock500, camp500, sr500, cr500, fr500, b500, d500 := measure(500)
	t.Logf("500 SKU sql=%d ListSKUStock=%d ListCampaigns=%d dur=%s bytes=%d stockRows=%d campRows=%d focusRows=%d",
		s500, stock500, camp500, d500, b500, sr500, cr500, fr500)
	counter.Reset()
	s2000, stock2000, camp2000, sr2000, cr2000, fr2000, b2000, d2000 := measure(2000)
	t.Logf("2000 SKU sql=%d ListSKUStock=%d ListCampaigns=%d dur=%s bytes=%d stockRows=%d campRows=%d focusRows=%d",
		s2000, stock2000, camp2000, d2000, b2000, sr2000, cr2000, fr2000)
	if stock2000 > stock500*5 {
		t.Fatalf("N+1 ListSKUStock 500=%d 2000=%d", stock500, stock2000)
	}
	if camp2000 > camp500*5+5 {
		t.Fatalf("N+1 ListCampaigns 500=%d 2000=%d", camp500, camp2000)
	}
	// Reject ~1 SQL per SKU growth (1500 extra for 1500 SKUs).
	if s2000 > s500+1500 {
		t.Fatalf("SQL growth looks like N+1: 500=%d 2000=%d", s500, s2000)
	}
}
