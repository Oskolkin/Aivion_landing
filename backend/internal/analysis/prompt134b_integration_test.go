//go:build integration

package analysis_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt134BConcurrentFullLoserState(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134b-conc")
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
	before := prompt134BSnapshotSeller(ctx, f)
	beforeCaseOverlay := prompt134BSnapshotCaseStateOverlay(ctx, f)
	beforeSigOverlay := prompt134BSnapshotSigStateOverlay(ctx, f)
	inA := prompt132PublishInput(t, f, store, rA.ID, "conc-a")
	inB := prompt132PublishInput(t, f, store, rB.ID, "conc-b")
	var errA, errB error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, errA = f.repo.PublishAtomic(ctx, inA) }()
	go func() { defer wg.Done(); _, errB = f.repo.PublishAtomic(ctx, inB) }()
	wg.Wait()
	wins, stale := 0, 0
	var loserRun, winnerRun int64
	if errA == nil {
		wins++
		winnerRun = rA.ID
	} else if errors.Is(errA, analysispg.ErrStalePublish) {
		stale++
		loserRun = rA.ID
	}
	if errB == nil {
		wins++
		winnerRun = rB.ID
	} else if errors.Is(errB, analysispg.ErrStalePublish) {
		stale++
		loserRun = rB.ID
	}
	if wins != 1 || stale != 1 {
		t.Fatalf("wins=%d stale=%d a=%v b=%v", wins, stale, errA, errB)
	}
	after := prompt134BSnapshotSeller(ctx, f)
	var winnerPub int64
	_ = f.pool.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, winnerRun).Scan(&winnerPub)
	if winnerPub == 0 {
		t.Fatal("winner publication missing")
	}
	prompt134BAssertLoserZero(t, ctx, f, loserRun, winnerRun, winnerPub, before, after, beforeCaseOverlay, beforeSigOverlay)
	t.Logf("winner_run=%d winner_pub=%d loser_run=%d before_hash=%s after_hash=%s case_digest %s→%s sig_digest %s→%s pubs %d→%d outbox %d→%d current %d→%d",
		winnerRun, winnerPub, loserRun, before.Digest, after.Digest,
		before.CaseStateDigest, after.CaseStateDigest, before.SigStateDigest, after.SigStateDigest,
		before.Publications, after.Publications, before.Outbox, after.Outbox, before.CurrentPubID, after.CurrentPubID)
}

func TestPrompt134BSellerLockIndependence(t *testing.T) {
	TestPrompt133SellerLockIndependence(t)
}

func TestPrompt134BContinuityGroundingFactRef(t *testing.T) {
	prompt134Setup(t)
	t.Setenv("DEEP_FAKE_STOCK_SUPPORT", "1")
	t.Setenv("DEEP_FAKE_PRICE_MENU", "1")
	t.Setenv("DEEP_FAKE_PRICE_INTENT", "DECREASE")
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134b-ground")
	ctx := context.Background()
	prompt134SetStockUnits(ctx, f, 2)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1g := prompt134BStockGrounding(t, f, store, r1.ID, 2)
	if p1g.Handle == "" || p1g.EvidenceID == "" {
		t.Fatalf("P1 grounding incomplete %+v", p1g)
	}
	if p1g.Units != 2 {
		t.Fatalf("P1 units=%d want 2", p1g.Units)
	}
	prompt134SetStockUnits(ctx, f, 40)
	t.Setenv("DEEP_FAKE_ASSESSMENT", "PARTIALLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	p2g := prompt134BStockGrounding(t, f, store, r2.ID, 40)
	if p2g.Handle == "" || p2g.EvidenceID == "" {
		t.Fatalf("P2 grounding incomplete %+v", p2g)
	}
	if p2g.Units != 40 {
		t.Fatalf("P2 current units=%d want 40", p2g.Units)
	}
	if p2g.Units == 2 {
		t.Fatal("stale P1 value 2 used as current grounding")
	}
	if p2g.EvidenceID == p1g.EvidenceID {
		t.Fatalf("current evidence still P1 evidence_id=%s", p1g.EvidenceID)
	}
	// Evidence handles are lineage/fingerprint specific (new FactRef → new ef_ handle).
	// Discovery f_* handles are reused only when lineage fingerprint matches.
	if p2g.Handle == p1g.Handle {
		if len(p2g.FactRefs) == 0 || strings.Join(p2g.FactRefs, ",") == strings.Join(p1g.FactRefs, ",") {
			t.Fatal("stable handle without P2 FactRef/revision distinction")
		}
	}
	p2Handles := map[string]bool{p2g.Handle: true}
	pack2 := prompt91LoadStageBody(t, f, store, r2.ID, string(stages.BuildDeepPack))
	var wire2 deep.PackWireV1
	if err := json.Unmarshal(pack2, &wire2); err != nil {
		t.Fatal(err)
	}
	for _, fh := range wire2.Payload.FactRegistry {
		if fh.Handle != "" {
			p2Handles[fh.Handle] = true
		}
	}
	regRaw := prompt91LoadArtifactKind(t, f, store, r2.ID, string(artifacts.AnalysisFactHandleRegistry))
	var reg evidence.UnifiedHandleRegistryV1
	if err := json.Unmarshal(regRaw, &reg); err != nil {
		t.Fatal(err)
	}
	p2FactRefs := map[string]bool{}
	for _, ref := range p2g.FactRefs {
		if ref != "" {
			p2FactRefs[ref] = true
		}
	}
	for _, e := range reg.Entries {
		if e.EvidenceID == p2g.EvidenceID || e.Handle == p2g.Handle {
			for _, ref := range e.FactRefs {
				if ref != "" {
					p2FactRefs[ref] = true
				}
			}
		}
	}
	var supportRows int
	var groundedP2Assessment bool
	rows, err := f.pool.Query(ctx, `
		SELECT sa.current_fact_refs_json::text
		FROM action_support_assessments sa
		JOIN action_case_assessments a ON a.id=sa.assessment_id
		WHERE a.analysis_run_id=$1`, r2.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var currentRefs string
		if err := rows.Scan(&currentRefs); err != nil {
			t.Fatal(err)
		}
		supportRows++
		if strings.TrimSpace(currentRefs) == "" || currentRefs == "[]" || currentRefs == "null" {
			t.Fatalf("current_fact_refs_json empty for support assessment row=%d refs=%s", supportRows, currentRefs)
		}
		var refs []string
		if err := json.Unmarshal([]byte(currentRefs), &refs); err != nil {
			t.Fatalf("current_fact_refs_json invalid: %s err=%v", currentRefs, err)
		}
		if len(refs) == 0 {
			t.Fatalf("current_fact_refs_json has zero handles: %s", currentRefs)
		}
		rowGroundedP2 := false
		hasP1Handle := false
		hasP2Handle := false
		for _, ref := range refs {
			if ref == p1g.Handle {
				hasP1Handle = true
			}
			if ref == p2g.Handle || p2Handles[ref] {
				hasP2Handle = true
				rowGroundedP2 = true
			}
			for p2Ref := range p2FactRefs {
				if strings.Contains(currentRefs, p2Ref) {
					rowGroundedP2 = true
				}
			}
		}
		if !rowGroundedP2 {
			t.Fatalf("support assessment not grounded to P2 current Evidence/FactRef refs=%s p2_handle=%s p2_evidence=%s", currentRefs, p2g.Handle, p2g.EvidenceID)
		}
		if hasP1Handle && !hasP2Handle {
			t.Fatalf("P1-only stale grounding refs=%s p1_handle=%s", currentRefs, p1g.Handle)
		}
		groundedP2Assessment = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if supportRows == 0 {
		t.Fatal("no action_support_assessments for P2 run")
	}
	if !groundedP2Assessment {
		t.Fatal("no support assessment with non-empty P2-grounded current_fact_refs_json")
	}
	t.Logf("contract=lineage-specific P1 evidence=%s handle=%s refs=%v units=2 P2 evidence=%s handle=%s refs=%v units=40 current_grounding=P2",
		p1g.EvidenceID, p1g.Handle, p1g.FactRefs, p2g.EvidenceID, p2g.Handle, p2g.FactRefs)
}

func TestPrompt134BPartiallySupportedP1FullContentHash(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134b-p1hash")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	hBefore := prompt134BImmutableContentHash(ctx, f, p1.ID)
	if hBefore == "" {
		t.Fatal("empty P1 immutable hash")
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "PARTIALLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	hAfter := prompt134BImmutableContentHash(ctx, f, p1.ID)
	if hBefore != hAfter {
		t.Fatalf("P1 immutable hash changed before=%s after=%s", hBefore, hAfter)
	}
	var outcome string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2.ID).Scan(&outcome)
	if outcome != "PARTIALLY_SUPPORTED" {
		t.Fatalf("outcome=%s", outcome)
	}
	t.Logf("H_before=%s H_after=%s equal=YES", hBefore, hAfter)
}

func TestPrompt134BLargeRowSentinelCurrentRead(t *testing.T) {
	prompt134Setup(t)
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134b-rows")
	ctx := context.Background()
	const p1Stock, p2Stock int64 = 111, 222
	const p1Budget, p2Budget = "333", "444"
	prompt134SetStockUnits(ctx, f, int(p1Stock))
	prompt134BSetCampaignBudget(ctx, f, p1Budget)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	prompt134SetStockUnits(ctx, f, int(p2Stock))
	prompt134BSetCampaignBudget(ctx, f, p2Budget)
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	p2, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	if p2.ID == p1.ID {
		t.Fatal("current still P1")
	}
	cur, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil || cur.ID != p2.ID {
		t.Fatalf("current-read pointer %v id=%d want P2=%d", err, cur.ID, p2.ID)
	}
	prompt134BAssertFamilySentinel(t, ctx, f, cur.ID, p1.ID, p2.ID,
		"publication_stock_sku_rows", "sku_id", fmt.Sprintf(`"sellable_units":%d`, p1Stock), fmt.Sprintf(`"sellable_units":%d`, p2Stock))
	prompt134BAssertFamilySentinel(t, ctx, f, cur.ID, p1.ID, p2.ID,
		"publication_ad_campaign_rows", "campaign_id", p1Budget, p2Budget)
	prompt134BAssertFamilySentinel(t, ctx, f, cur.ID, p1.ID, p2.ID,
		"publication_sku_focus_rows", "sku_id", fmt.Sprintf(`"sellable_units":%d`, p1Stock), fmt.Sprintf(`"sellable_units":%d`, p2Stock))
}

func TestPrompt134BOverviewCutoffLiteral(t *testing.T) {
	TestPrompt134ASixViewPublicationIdentity(t)
}

type prompt134BSellerSnap struct {
	Publications, Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows                                     int
	Signatures, SigStates, Cases, CaseStates                         int
	Proposals, Assessments, SupportAssessments                       int
	UserEvents, SystemEvents                                         int
	Groups, GroupMembers, ReleaseEvents, Outbox                      int
	CurrentPubID                                                     int64
	SigIDs, CaseIDs, ProposalIDs, AssessmentIDs                      string
	SupportIDs, UserEventIDs, SystemEventIDs                         string
	GroupIDs, MemberIDs, ReleaseIDs, OutboxIDs                       string
	CaseStateDigest, SigStateDigest                                  string
	Digest                                                           string
}

func prompt134BSnapshotSeller(ctx context.Context, f prompt81Fixture) prompt134BSellerSnap {
	var s prompt134BSellerSnap
	sid := f.seller
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE seller_account_id=$1`, sid).Scan(&s.Publications)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_components c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.Components)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_findings c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.Findings)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_recommendations c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.Recommendations)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_projections c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.Projections)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.StockRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_ad_campaign_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.AdRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_sku_focus_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.FocusRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_signatures WHERE seller_account_id=$1`, sid).Scan(&s.Signatures)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_signature_state ss JOIN action_signatures s ON s.id=ss.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.SigStates)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.Cases)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_state st JOIN action_cases c ON c.id=st.case_id JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.CaseStates)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_proposals p JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.seller_account_id=$1`, sid).Scan(&s.Proposals)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments a JOIN analysis_publications pub ON pub.id=a.new_publication_id WHERE pub.seller_account_id=$1`, sid).Scan(&s.Assessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_support_assessments sa JOIN action_case_assessments a ON a.id=sa.assessment_id JOIN analysis_publications pub ON pub.id=a.new_publication_id WHERE pub.seller_account_id=$1`, sid).Scan(&s.SupportAssessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_user_events e JOIN action_cases c ON c.id=e.case_id JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.UserEvents)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_system_events e JOIN action_cases c ON c.id=e.case_id JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.SystemEvents)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups g JOIN analysis_publications p ON p.id=g.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.Groups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id JOIN analysis_publications p ON p.id=g.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.GroupMembers)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_release_events e JOIN analysis_publications p ON p.id=e.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.ReleaseEvents)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE payload_json->>'seller_account_id'=$1`, strconv.FormatInt(sid, 10)).Scan(&s.Outbox)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(id::text, ',' ORDER BY id), '') FROM action_signatures WHERE seller_account_id=$1`, sid).Scan(&s.SigIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(c.id::text, ',' ORDER BY c.id::text), '') FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.CaseIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(p.id::text, ',' ORDER BY p.id::text), '') FROM action_case_proposals p JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.seller_account_id=$1`, sid).Scan(&s.ProposalIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(a.id::text, ',' ORDER BY a.id::text), '') FROM action_case_assessments a JOIN analysis_publications pub ON pub.id=a.new_publication_id WHERE pub.seller_account_id=$1`, sid).Scan(&s.AssessmentIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(sa.id::text, ',' ORDER BY sa.id), '') FROM action_support_assessments sa JOIN action_case_assessments a ON a.id=sa.assessment_id JOIN analysis_publications pub ON pub.id=a.new_publication_id WHERE pub.seller_account_id=$1`, sid).Scan(&s.SupportIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(e.id::text, ',' ORDER BY e.id::text), '') FROM action_user_events e JOIN action_cases c ON c.id=e.case_id JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.UserEventIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(e.id::text, ',' ORDER BY e.id::text), '') FROM action_system_events e JOIN action_cases c ON c.id=e.case_id JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.SystemEventIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(g.id::text, ',' ORDER BY g.id::text), '') FROM publication_action_groups g JOIN analysis_publications p ON p.id=g.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.GroupIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(m.id::text, ',' ORDER BY m.id), '') FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id JOIN analysis_publications p ON p.id=g.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.MemberIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(e.id::text, ',' ORDER BY e.id), '') FROM publication_release_events e JOIN analysis_publications p ON p.id=e.publication_id WHERE p.seller_account_id=$1`, sid).Scan(&s.ReleaseIDs)
	_ = f.pool.QueryRow(ctx, `SELECT coalesce(string_agg(id::text, ',' ORDER BY id::text), '') FROM analysis_outbox_events WHERE payload_json->>'seller_account_id'=$1`, strconv.FormatInt(sid, 10)).Scan(&s.OutboxIDs)
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(md5(string_agg(st.system_state||':'||st.user_state||':'||coalesce(st.current_proposal_id::text,'')||':'||c.id::text, ',' ORDER BY c.id::text)), '')
		FROM action_case_state st JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.CaseStateDigest)
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(md5(string_agg(coalesce(ss.suppression_source,'')||':'||coalesce(ss.suppressed_until::text,'')||':'||ss.last_occurrence_no::text||':'||s.signature_hash, ',' ORDER BY s.signature_hash)), '')
		FROM action_signature_state ss JOIN action_signatures s ON s.id=ss.signature_id
		WHERE s.seller_account_id=$1`, sid).Scan(&s.SigStateDigest)
	cur, err := f.repo.GetCurrentPublication(ctx, sid)
	if err == nil {
		s.CurrentPubID = cur.ID
	}
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	s.Digest = hex.EncodeToString(sum[:])
	return s
}

func prompt134BSnapshotCaseStateOverlay(ctx context.Context, f prompt81Fixture) map[string]string {
	out := map[string]string{}
	rows, err := f.pool.Query(ctx, `
		SELECT c.id::text, st.system_state, st.user_state, coalesce(st.current_proposal_id::text,'')
		FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1
		ORDER BY c.id::text`, f.seller)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, sys, user, prop string
		if err := rows.Scan(&id, &sys, &user, &prop); err != nil {
			panic(err)
		}
		out[id] = sys + "|" + user + "|" + prop
	}
	return out
}

func prompt134BSnapshotSigStateOverlay(ctx context.Context, f prompt81Fixture) map[string]string {
	out := map[string]string{}
	rows, err := f.pool.Query(ctx, `
		SELECT s.id::text,
			coalesce(ss.active_case_id::text,''),
			coalesce(ss.suppressed_until::text,''),
			coalesce(ss.suppression_source,''),
			coalesce(ss.suppression_reason,''),
			ss.last_occurrence_no::text
		FROM action_signature_state ss
		JOIN action_signatures s ON s.id=ss.signature_id
		WHERE s.seller_account_id=$1
		ORDER BY s.id::text`, f.seller)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, active, supUntil, supSrc, supReason, occ string
		if err := rows.Scan(&id, &active, &supUntil, &supSrc, &supReason, &occ); err != nil {
			panic(err)
		}
		out[id] = active + "|" + supUntil + "|" + supSrc + "|" + supReason + "|" + occ
	}
	return out
}

func prompt134BWinnerAffectedOverlay(ctx context.Context, f prompt81Fixture, winnerRun, winnerPub int64) (map[string]bool, map[string]bool) {
	cases := map[string]bool{}
	sigs := map[string]bool{}
	rows, err := f.pool.Query(ctx, `
		SELECT DISTINCT c.id::text, c.signature_id::text FROM action_cases c
			JOIN action_signatures s ON s.id=c.signature_id
			WHERE s.seller_account_id=$1 AND c.opened_by_publication_id=$2
		UNION
		SELECT DISTINCT c.id::text, c.signature_id::text FROM action_case_proposals p
			JOIN action_cases c ON c.id=p.case_id
			JOIN action_signatures s ON s.id=c.signature_id
			WHERE s.seller_account_id=$1 AND p.publication_id=$2
		UNION
		SELECT DISTINCT c.id::text, c.signature_id::text FROM action_case_assessments a
			JOIN action_case_proposals p ON p.id=a.previous_proposal_id
			JOIN action_cases c ON c.id=p.case_id
			JOIN action_signatures s ON s.id=c.signature_id
			WHERE s.seller_account_id=$1 AND a.analysis_run_id=$3
		UNION
		SELECT DISTINCT c.id::text, c.signature_id::text FROM action_system_events e
			JOIN action_cases c ON c.id=e.case_id
			JOIN action_signatures s ON s.id=c.signature_id
			WHERE s.seller_account_id=$1 AND e.publication_id=$2
		UNION
		SELECT DISTINCT st.case_id::text, c.signature_id::text FROM action_case_state st
			JOIN action_case_proposals p ON p.id=st.current_proposal_id
			JOIN action_cases c ON c.id=st.case_id
			JOIN action_signatures s ON s.id=c.signature_id
			WHERE s.seller_account_id=$1 AND p.publication_id=$2`,
		f.seller, winnerPub, winnerRun)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var caseID, sigID string
		if err := rows.Scan(&caseID, &sigID); err != nil {
			panic(err)
		}
		if caseID != "" {
			cases[caseID] = true
		}
		if sigID != "" {
			sigs[sigID] = true
		}
	}
	return cases, sigs
}

func prompt134BAssertLoserZero(t *testing.T, ctx context.Context, f prompt81Fixture, loserRun, winnerRun, winnerPub int64, before, after prompt134BSellerSnap, beforeCaseOverlay, beforeSigOverlay map[string]string) {
	t.Helper()
	assertZero := func(name string, n int) {
		t.Helper()
		if n != 0 {
			t.Fatalf("loser %s=%d want 0", name, n)
		}
	}
	var n int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("publications", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("assessments", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_support_assessments sa JOIN action_case_assessments a ON a.id=sa.assessment_id WHERE a.analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("support_assessments", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_proposals p JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("proposals", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_cases c JOIN analysis_publications pub ON pub.id=c.opened_by_publication_id WHERE pub.analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("cases", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups g JOIN analysis_publications p ON p.id=g.publication_id WHERE p.analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("groups", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id JOIN analysis_publications p ON p.id=g.publication_id WHERE p.analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("group_members", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_release_events e JOIN analysis_publications p ON p.id=e.publication_id WHERE p.analysis_run_id=$1`, loserRun).Scan(&n)
	assertZero("release_events", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE payload_json->>'analysis_run_id'=$1`, strconv.FormatInt(loserRun, 10)).Scan(&n)
	assertZero("outbox_events", n)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_system_events WHERE publication_id IN (SELECT id FROM analysis_publications WHERE analysis_run_id=$1)`, loserRun).Scan(&n)
	assertZero("system_events", n)
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_signatures s
		WHERE s.seller_account_id=$1 AND s.id::text <> ALL(string_to_array($2, ','))
		  AND NOT EXISTS (
			SELECT 1 FROM action_cases c WHERE c.signature_id=s.id AND c.opened_by_publication_id=$3
		  )`, f.seller, before.SigIDs, winnerPub).Scan(&n)
	assertZero("orphan_new_signatures", n)
	newCases := prompt134BNewCSV(before.CaseIDs, after.CaseIDs)
	if newCases != "" {
		_ = f.pool.QueryRow(ctx, `
			SELECT count(*) FROM action_cases WHERE id::text = ANY(string_to_array($1, ',')) AND opened_by_publication_id<>$2`,
			newCases, winnerPub).Scan(&n)
		assertZero("new_cases_not_winner", n)
	}
	newProps := prompt134BNewCSV(before.ProposalIDs, after.ProposalIDs)
	if newProps != "" {
		_ = f.pool.QueryRow(ctx, `
			SELECT count(*) FROM action_case_proposals WHERE id::text = ANY(string_to_array($1, ',')) AND publication_id<>$2`,
			newProps, winnerPub).Scan(&n)
		assertZero("new_proposals_not_winner", n)
	}
	newAssess := prompt134BNewCSV(before.AssessmentIDs, after.AssessmentIDs)
	if newAssess != "" {
		_ = f.pool.QueryRow(ctx, `
			SELECT count(*) FROM action_case_assessments WHERE id::text = ANY(string_to_array($1, ',')) AND (new_publication_id<>$2 OR analysis_run_id=$3)`,
			newAssess, winnerPub, loserRun).Scan(&n)
		assertZero("new_assessments_not_winner", n)
	}
	newSys := prompt134BNewCSV(before.SystemEventIDs, after.SystemEventIDs)
	if newSys != "" {
		_ = f.pool.QueryRow(ctx, `
			SELECT count(*) FROM action_system_events
			WHERE id::text = ANY(string_to_array($1, ','))
			  AND publication_id IS NOT NULL AND publication_id<>$2`,
			newSys, winnerPub).Scan(&n)
		assertZero("new_system_events_not_winner", n)
	}
	newUsers := prompt134BNewCSV(before.UserEventIDs, after.UserEventIDs)
	if newUsers != "" {
		t.Fatalf("loser/user events appeared: %s", newUsers)
	}
	newOutbox := prompt134BNewCSV(before.OutboxIDs, after.OutboxIDs)
	if newOutbox != "" {
		_ = f.pool.QueryRow(ctx, `
			SELECT count(*) FROM analysis_outbox_events
			WHERE id::text = ANY(string_to_array($1, ','))
			  AND coalesce(payload_json->>'publication_id','') <> $2`,
			newOutbox, strconv.FormatInt(winnerPub, 10)).Scan(&n)
		assertZero("new_outbox_not_winner", n)
	}
	if after.CurrentPubID != winnerPub {
		t.Fatalf("current pointer=%d want winner=%d (loser mutation)", after.CurrentPubID, winnerPub)
	}
	if after.Publications != before.Publications+1 {
		t.Fatalf("publications delta=%d want +1 winner only", after.Publications-before.Publications)
	}
	winnerCases, winnerSigs := prompt134BWinnerAffectedOverlay(ctx, f, winnerRun, winnerPub)
	afterCaseOverlay := prompt134BSnapshotCaseStateOverlay(ctx, f)
	afterSigOverlay := prompt134BSnapshotSigStateOverlay(ctx, f)
	for caseID, beforeRow := range beforeCaseOverlay {
		afterRow, ok := afterCaseOverlay[caseID]
		if !ok {
			t.Fatalf("pre-existing case %s missing after concurrent publish", caseID)
		}
		if afterRow != beforeRow && !winnerCases[caseID] {
			t.Fatalf("loser case_state mutation case=%s before=%s after=%s winner_affected=false", caseID, beforeRow, afterRow)
		}
	}
	for sigID, beforeRow := range beforeSigOverlay {
		afterRow, ok := afterSigOverlay[sigID]
		if !ok {
			t.Fatalf("pre-existing signature %s missing after concurrent publish", sigID)
		}
		if afterRow != beforeRow && !winnerSigs[sigID] {
			t.Fatalf("loser signature_state mutation sig=%s before=%s after=%s winner_affected=false", sigID, beforeRow, afterRow)
		}
	}
}

func prompt134BNewCSV(before, after string) string {
	set := map[string]bool{}
	for _, id := range strings.Split(before, ",") {
		if id != "" {
			set[id] = true
		}
	}
	var extra []string
	for _, id := range strings.Split(after, ",") {
		if id != "" && !set[id] {
			extra = append(extra, id)
		}
	}
	return strings.Join(extra, ",")
}

type prompt134BGrounding struct {
	EvidenceID string
	Handle     string
	FactRefs   []string
	Units      int
}

func prompt134BStockGrounding(t *testing.T, f prompt81Fixture, store artifacts.ArtifactStore, runID int64, wantUnits int) prompt134BGrounding {
	t.Helper()
	pack := prompt91LoadStageBody(t, f, store, runID, string(stages.BuildDeepPack))
	var wire deep.PackWireV1
	if err := json.Unmarshal(pack, &wire); err != nil {
		t.Fatal(err)
	}
	regRaw := prompt91LoadArtifactKind(t, f, store, runID, string(artifacts.AnalysisFactHandleRegistry))
	var reg evidence.UnifiedHandleRegistryV1
	_ = json.Unmarshal(regRaw, &reg)
	byHandle := map[string]evidence.UnifiedHandle{}
	for _, e := range reg.Entries {
		byHandle[e.Handle] = e
	}
	fill := func(g prompt134BGrounding) prompt134BGrounding {
		for _, e := range reg.Entries {
			if g.EvidenceID != "" && e.EvidenceID == g.EvidenceID {
				g.FactRefs = e.FactRefs
				if g.Handle == "" {
					g.Handle = e.Handle
				}
				return g
			}
		}
		if e, ok := byHandle[g.Handle]; ok {
			if len(g.FactRefs) == 0 {
				g.FactRefs = e.FactRefs
			}
			if g.EvidenceID == "" {
				g.EvidenceID = e.EvidenceID
			}
		}
		return g
	}
	for _, link := range wire.Payload.ContinuitySupportEvidenceLinks {
		for _, it := range wire.Payload.EvidenceRegistry {
			if it.EvidenceID != link.EvidenceID || !strings.Contains(it.EvidenceType, "STOCK") {
				continue
			}
			raw := string(it.Data)
			if stockObsLatestUnits(raw) != wantUnits && !stockObsHasUnits(raw, wantUnits) {
				continue
			}
			h := firstEvidenceHandle(wire.Payload, it.EvidenceID)
			if h == "" {
				continue
			}
			g := fill(prompt134BGrounding{EvidenceID: it.EvidenceID, Handle: h, Units: wantUnits})
			return g
		}
	}
	for _, it := range wire.Payload.EvidenceRegistry {
		if !strings.Contains(it.EvidenceType, "STOCK") {
			continue
		}
		raw := string(it.Data)
		h := firstEvidenceHandle(wire.Payload, it.EvidenceID)
		if h == "" {
			h = stockHandleFromPack(wire)
		}
		if h == "" {
			continue
		}
		units := stockObsLatestUnits(raw)
		if units < 0 && stockObsHasUnits(raw, wantUnits) {
			units = wantUnits
		}
		if wantUnits > 0 && units != wantUnits && !stockObsHasUnits(raw, wantUnits) {
			continue
		}
		if units < 0 {
			units = wantUnits
		}
		return fill(prompt134BGrounding{EvidenceID: it.EvidenceID, Handle: h, Units: units})
	}
	h := stockHandleFromPack(wire)
	if h == "" {
		for _, f := range wire.Payload.FactRegistry {
			if f.Handle == "" {
				continue
			}
			if !strings.Contains(strings.ToLower(f.SemanticPath), "stock") && f.SourceKind != "EVIDENCE" {
				continue
			}
			g := fill(prompt134BGrounding{Handle: f.Handle, EvidenceID: f.SourceRef, Units: wantUnits})
			if g.EvidenceID == "" {
				g.EvidenceID = f.Handle
			}
			return g
		}
		for _, e := range reg.Entries {
			blob := strings.ToUpper(strings.Join(e.FactRefs, " "))
			if e.Handle == "" || (!strings.Contains(strings.ToLower(e.SemanticPath), "stock") && !strings.Contains(blob, "SKU_STOCK")) {
				continue
			}
			eid := e.EvidenceID
			if eid == "" {
				eid = e.Handle
			}
			return fill(prompt134BGrounding{Handle: e.Handle, EvidenceID: eid, Units: wantUnits, FactRefs: e.FactRefs})
		}
	}
	if h == "" {
		var hid, eid string
		_ = f.pool.QueryRow(context.Background(), `
			SELECT COALESCE(ps.fact_refs_json->>0,''), COALESCE(ps.revalidation_json->>'evidence_type','')
			FROM action_proposal_supports ps
			JOIN action_case_proposals p ON p.id=ps.proposal_id
			JOIN analysis_publications pub ON pub.id=p.publication_id
			WHERE pub.analysis_run_id=$1
			ORDER BY ps.id LIMIT 1`, runID).Scan(&hid, &eid)
		if hid != "" {
			g := fill(prompt134BGrounding{Handle: hid, EvidenceID: eid, Units: wantUnits})
			if g.EvidenceID == "" {
				g.EvidenceID = hid
			}
			return g
		}
		t.Logf("pack facts=%d evidence=%d continuity_links=%d", len(wire.Payload.FactRegistry), len(wire.Payload.EvidenceRegistry), len(wire.Payload.ContinuitySupportEvidenceLinks))
		for _, it := range wire.Payload.EvidenceRegistry {
			t.Logf("evidence type=%s id=%s", it.EvidenceType, it.EvidenceID)
		}
		return prompt134BGrounding{}
	}
	g := fill(prompt134BGrounding{Handle: h, Units: wantUnits, EvidenceID: h})
	return g
}

func prompt134BImmutableContentHash(ctx context.Context, f prompt81Fixture, pubID int64) string {
	queries := []string{
		`SELECT seller_account_id::text, publication_generation::text, analysis_run_id::text, analysis_date::text,
			analysis_cutoff_at::text, factual_universe_id::text, factual_universe_hash, contract_bundle_hash, publication_content_hash, published_at::text
			FROM analysis_publications WHERE id=$1`,
		`SELECT component_type, schema_version, coalesce(artifact_id::text,''), sha256 FROM publication_components WHERE publication_id=$1 ORDER BY component_type`,
		`SELECT finding_ref, ordinal::text, primary_kind, primary_id, statement, display_order::text, grounding_refs_json::text
			FROM publication_findings WHERE publication_id=$1 ORDER BY finding_ref`,
		`SELECT recommendation_ref, ordinal::text, actionability, summary, finding_refs_json::text, display_order::text
			FROM publication_recommendations WHERE publication_id=$1 ORDER BY recommendation_ref`,
		`SELECT projection_key, schema_version, payload_json::text, payload_hash FROM publication_projections WHERE publication_id=$1 ORDER BY projection_key`,
		`SELECT sku_id, row_json::text FROM publication_stock_sku_rows WHERE publication_id=$1 ORDER BY sku_id`,
		`SELECT campaign_id, row_json::text FROM publication_ad_campaign_rows WHERE publication_id=$1 ORDER BY campaign_id`,
		`SELECT sku_id, row_json::text FROM publication_sku_focus_rows WHERE publication_id=$1 ORDER BY sku_id`,
		`SELECT id::text, recommendation_id, target_kind, family, action_type, display_order::text, display_title, display_summary, presentation_template_key
			FROM publication_action_groups WHERE publication_id=$1 ORDER BY id::text`,
		`SELECT m.group_id::text, m.case_id::text, m.proposal_id::text, m.member_order::text
			FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id
			WHERE g.publication_id=$1 ORDER BY m.group_id::text, m.proposal_id::text`,
		`SELECT p.case_id::text, p.recommendation_id, p.action_type, p.parameters_json::text, p.display_order::text
			FROM action_case_proposals p WHERE p.publication_id=$1 ORDER BY p.id::text`,
		`SELECT ps.support_key, ps.support_role, ps.claim, ps.fact_refs_json::text, ps.finding_refs_json::text, ps.revalidation_json::text
			FROM action_proposal_supports ps JOIN action_case_proposals p ON p.id=ps.proposal_id
			WHERE p.publication_id=$1 ORDER BY p.id::text, ps.support_key`,
	}
	var parts []string
	for _, q := range queries {
		parts = append(parts, prompt134BHashQuery(ctx, f, q, pubID))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n--\n")))
	return hex.EncodeToString(sum[:])
}

func prompt134BHashQuery(ctx context.Context, f prompt81Fixture, sql string, pubID int64) string {
	rows, err := f.pool.Query(ctx, sql, pubID)
	if err != nil {
		return "ERR:" + err.Error()
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return "ERR:" + err.Error()
		}
		cells := make([]string, 0, len(vals))
		for _, v := range vals {
			switch x := v.(type) {
			case []byte:
				cells = append(cells, string(x))
			case nil:
				cells = append(cells, "")
			default:
				cells = append(cells, fmt.Sprint(x))
			}
		}
		lines = append(lines, strings.Join(cells, "\x1f"))
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func prompt134BSetCampaignBudget(ctx context.Context, f prompt81Fixture, amount string) {
	_, err := f.pool.Exec(ctx, `
		UPDATE advertising_campaign_versions SET budget=$2::numeric
		WHERE seller_account_id=$1`, f.seller, amount)
	if err != nil {
		panic(err)
	}
}

func prompt134BAssertFamilySentinel(t *testing.T, ctx context.Context, f prompt81Fixture, currentID, p1ID, p2ID int64, table, idCol, p1Sentinel, p2Sentinel string) {
	t.Helper()
	var nP2 int
	_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), p2ID).Scan(&nP2)
	if nP2 < 1 {
		t.Fatalf("%s P2 rows=%d", table, nP2)
	}
	rows, err := f.pool.Query(ctx, fmt.Sprintf(`SELECT publication_id, %s, row_json::text FROM %s WHERE publication_id=$1`, idCol, table), currentID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	mix, nCur, sawP2, sawP1 := 0, 0, 0, 0
	for rows.Next() {
		var pubID int64
		var id, raw string
		if err := rows.Scan(&pubID, &id, &raw); err != nil {
			t.Fatal(err)
		}
		nCur++
		if pubID != p2ID {
			mix++
		}
		if pubID == p1ID {
			mix++
		}
		if prompt134BRowHasSentinel(raw, p2Sentinel) {
			sawP2++
		}
		if prompt134BRowHasSentinel(raw, p1Sentinel) {
			sawP1++
		}
	}
	if nCur != nP2 {
		t.Fatalf("%s current rows=%d expected P2=%d", table, nCur, nP2)
	}
	if mix != 0 {
		t.Fatalf("%s mix_count=%d want 0", table, mix)
	}
	if sawP2 < 1 {
		t.Fatalf("%s P2 sentinel %q missing in current result", table, p2Sentinel)
	}
	if sawP1 != 0 {
		t.Fatalf("%s P1 sentinel %q present in current result count=%d", table, p1Sentinel, sawP1)
	}
	t.Logf("%s P1_sentinel=%s P2_sentinel=%s current=%d mix=0 row_count=%d", table, p1Sentinel, p2Sentinel, currentID, nCur)
}

func prompt134BRowHasSentinel(raw, sentinel string) bool {
	if sentinel == "" {
		return false
	}
	if strings.Contains(raw, sentinel) {
		return true
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(raw, " ", ""), "\n", "")
	want := strings.ReplaceAll(strings.ReplaceAll(sentinel, " ", ""), "\n", "")
	if strings.Contains(compact, want) {
		return true
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return false
	}
	blob, _ := json.Marshal(m)
	return strings.Contains(string(blob), want) || strings.Contains(fmt.Sprint(m), strings.Trim(sentinel, `"`))
}
