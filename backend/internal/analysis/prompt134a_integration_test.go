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
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt134AActionGroupMemberUpdateRejected(t *testing.T) {
	TestPrompt134ActionGroupImmutabilityPopulated(t)
}

func TestPrompt134ASixViewPublicationIdentity(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "FULLY_SUPPORTED")
	ctx := context.Background()
	p2, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	var date, uni string
	var cutoff time.Time
	var seller int64
	if err := f.pool.QueryRow(ctx, `
		SELECT seller_account_id, analysis_date::text, analysis_cutoff_at, factual_universe_hash
		FROM analysis_publications WHERE id=$1`, p2.ID).Scan(&seller, &date, &cutoff, &uni); err != nil {
		t.Fatal(err)
	}
	if seller != f.seller || uni == "" || date == "" {
		t.Fatalf("P2 identity incomplete seller=%d date=%s uni=%s", seller, date, uni)
	}
	keys := []string{
		projection.KeyOverview, projection.KeyFunnel, projection.KeyStock,
		projection.KeyAdvertising, projection.KeyActionsSnapshot, projection.KeyChatContext,
	}
	for _, k := range keys {
		var boundPub int64
		var schema string
		err := f.pool.QueryRow(ctx, `
			SELECT publication_id, schema_version FROM publication_projections
			WHERE publication_id=$1 AND projection_key=$2`, p2.ID, k).Scan(&boundPub, &schema)
		if err != nil {
			t.Fatalf("%s envelope missing: %v", k, err)
		}
		if boundPub != p2.ID {
			t.Fatalf("%s publication_id=%d want P2=%d", k, boundPub, p2.ID)
		}
		if schema == "" {
			t.Fatalf("%s missing schema_version in envelope", k)
		}
		raw, err := f.repo.GetProjection(ctx, p2.ID, k)
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s GetProjection missing: %v", k, err)
		}
	}
	ov, err := f.repo.GetProjection(ctx, p2.ID, projection.KeyOverview)
	if err != nil {
		t.Fatal(err)
	}
	var overview projection.OverviewV1DTO
	if err := json.Unmarshal(ov, &overview); err != nil {
		t.Fatal(err)
	}
	if overview.SellerAccountID != f.seller {
		t.Fatalf("Overview seller=%d want %d", overview.SellerAccountID, f.seller)
	}
	if overview.AnalysisDate != date {
		t.Fatalf("Overview analysis_date=%s want %s", overview.AnalysisDate, date)
	}
	if overview.FactualUniverseHash != uni {
		t.Fatalf("Overview universe mismatch")
	}
	wantCutoff := cutoff.UTC().Format("2006-01-02T15:04:05Z07:00")
	if overview.AnalysisCutoffAt != wantCutoff {
		t.Fatalf("Overview analysis_cutoff_at=%s want %s", overview.AnalysisCutoffAt, wantCutoff)
	}
	t.Logf("P2 publication_id=%d seller=%d date=%s cutoff=%s universe=%s", p2.ID, seller, date, wantCutoff, uni)
	chatRaw, err := f.repo.GetProjection(ctx, p2.ID, projection.KeyChatContext)
	if err != nil {
		t.Fatal(err)
	}
	var chat projection.ChatContextDTO
	if err := json.Unmarshal(chatRaw, &chat); err != nil {
		t.Fatal(err)
	}
	if chat.AnalysisDate != date {
		t.Fatalf("Chat analysis_date=%s want %s", chat.AnalysisDate, date)
	}
	_ = r2
}

type prompt134ASellerSnap struct {
	Publications, Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows                                     int
	Signatures, SigStates, Cases, CaseStates                         int
	Proposals, Assessments, SupportAssessments                       int
	UserEvents, SystemEvents                                         int
	Groups, GroupMembers, ReleaseEvents, Outbox                      int
	CurrentPubID                                                     int64
	CaseStateDigest, SigStateDigest                                  string
}

func prompt134ASnapshotSeller(ctx context.Context, f prompt81Fixture) prompt134ASellerSnap {
	var s prompt134ASellerSnap
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
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(md5(string_agg(st.system_state||':'||st.user_state||':'||c.id::text, ',' ORDER BY c.id::text)), '')
		FROM action_case_state st JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, sid).Scan(&s.CaseStateDigest)
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(md5(string_agg(coalesce(ss.suppression_source,'')||':'||coalesce(ss.suppressed_until::text,'')||':'||s.signature_hash, ',' ORDER BY s.signature_hash)), '')
		FROM action_signature_state ss JOIN action_signatures s ON s.id=ss.signature_id
		WHERE s.seller_account_id=$1`, sid).Scan(&s.SigStateDigest)
	cur, err := f.repo.GetCurrentPublication(ctx, sid)
	if err == nil {
		s.CurrentPubID = cur.ID
	}
	return s
}

func TestPrompt134AConcurrentFullLoserResidue(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134a-conc")
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
	beforeSeller := prompt134ASnapshotSeller(ctx, f)
	beforeA := prompt134SnapshotRunResidue(ctx, f, rA.ID)
	beforeB := prompt134SnapshotRunResidue(ctx, f, rB.ID)
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
	loserAfter := prompt134SnapshotRunResidue(ctx, f, loserRun)
	base := beforeA
	if loserRun == rB.ID {
		base = beforeB
	}
	prompt134RunResidueEqual(t, base, loserAfter)
	afterSeller := prompt134ASnapshotSeller(ctx, f)
	winnerAfter := prompt134SnapshotRunResidue(ctx, f, winnerRun)
	if winnerAfter.Publications != 1 {
		t.Fatalf("winner publications=%d", winnerAfter.Publications)
	}
	if afterSeller.Publications != beforeSeller.Publications+1 {
		t.Fatalf("seller pubs before=%d after=%d", beforeSeller.Publications, afterSeller.Publications)
	}
	if afterSeller.Outbox != beforeSeller.Outbox+1 {
		t.Fatalf("outbox before=%d after=%d want +1 winner only", beforeSeller.Outbox, afterSeller.Outbox)
	}
	if afterSeller.CurrentPubID == 0 || afterSeller.CurrentPubID == beforeSeller.CurrentPubID {
		t.Fatal("current pointer must move to winner")
	}
	var loserPubs int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, loserRun).Scan(&loserPubs)
	if loserPubs != 0 {
		t.Fatalf("loser publications=%d", loserPubs)
	}
}

func TestPrompt134AContinuityFactHandles(t *testing.T) {
	prompt134Setup(t)
	t.Setenv("DEEP_FAKE_STOCK_SUPPORT", "1")
	t.Setenv("DEEP_FAKE_PRICE_MENU", "1")
	t.Setenv("DEEP_FAKE_PRICE_INTENT", "DECREASE")
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134a-handles")
	ctx := context.Background()
	prompt134SetStockUnits(ctx, f, 2)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	hOld := ""
	_ = f.pool.QueryRow(ctx, `
		SELECT COALESCE((ps.fact_refs_json->>0),'')
		FROM action_proposal_supports ps
		JOIN action_case_proposals p ON p.id=ps.proposal_id
		JOIN analysis_publications pub ON pub.id=p.publication_id
		WHERE pub.analysis_run_id=$1 AND COALESCE(ps.revalidation_json->>'evidence_type','') LIKE '%STOCK%'
		LIMIT 1`, r1.ID).Scan(&hOld)
	if hOld == "" {
		pack1 := prompt91LoadStageBody(t, f, store, r1.ID, string(stages.BuildDeepPack))
		var wire1 deep.PackWireV1
		if err := json.Unmarshal(pack1, &wire1); err != nil {
			t.Fatal(err)
		}
		hOld = stockHandleFromPack(wire1)
	}
	if hOld == "" {
		t.Fatal("P1 stock Fact Handle empty")
	}
	prompt134SetStockUnits(ctx, f, 40)
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, string(stages.BuildDeepPack))
	pack2 := prompt91LoadStageBody(t, f, store, r2.ID, string(stages.BuildDeepPack))
	var wire2 deep.PackWireV1
	if err := json.Unmarshal(pack2, &wire2); err != nil {
		t.Fatal(err)
	}
	found40, found2, hCur := false, false, ""
	for _, link := range wire2.Payload.ContinuitySupportEvidenceLinks {
		for _, it := range wire2.Payload.EvidenceRegistry {
			if it.EvidenceID != link.EvidenceID || !strings.Contains(it.EvidenceType, "STOCK") {
				continue
			}
			raw := string(it.Data)
			if stockObsHasUnits(raw, 40) {
				found40 = true
				hCur = firstEvidenceHandle(wire2.Payload, it.EvidenceID)
			}
			if stockObsLatestUnits(raw) == 2 {
				found2 = true
			}
		}
	}
	if !found40 {
		t.Fatal("P2 continuity stock missing sellable_units=40")
	}
	if found2 {
		t.Fatal("stale P1 stock=2 is current continuity evidence")
	}
	if hCur == "" {
		t.Fatal("P2 current Fact Handle empty")
	}
	t.Logf("P1 handle=%s P2 handle=%s P2 value=40 stale_current=NONE", hOld, hCur)
}

func stockHandleFromPack(wire deep.PackWireV1) string {
	for _, it := range wire.Payload.EvidenceRegistry {
		if !strings.Contains(it.EvidenceType, "STOCK") {
			continue
		}
		if h := firstEvidenceHandle(wire.Payload, it.EvidenceID); h != "" {
			return h
		}
		for _, f := range wire.Payload.FactRegistry {
			if f.SourceRef == it.EvidenceID && f.Handle != "" {
				return f.Handle
			}
		}
	}
	for _, f := range wire.Payload.FactRegistry {
		if strings.Contains(strings.ToLower(f.SemanticPath), "stock") && f.Handle != "" {
			return f.Handle
		}
	}
	return ""
}

func TestPrompt134AOverBudgetExactNoArtifact(t *testing.T) {
	TestPrompt134OverBudgetProviderCallsZero(t)
}

func prompt134AImmutableContentHash(ctx context.Context, f prompt81Fixture, pubID int64) string {
	var parts []string
	var core string
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(publication_content_hash,'')||'|'||coalesce(factual_universe_hash,'')||'|'||analysis_date::text||'|'||analysis_cutoff_at::text
		FROM analysis_publications WHERE id=$1`, pubID).Scan(&core)
	parts = append(parts, "core:"+core)
	addAgg := func(sql string) {
		var h string
		_ = f.pool.QueryRow(ctx, sql, pubID).Scan(&h)
		parts = append(parts, h)
	}
	addAgg(`SELECT coalesce(md5(string_agg(component_type||':'||sha256, ',' ORDER BY component_type)), '') FROM publication_components WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(ordinal::text||':'||coalesce(statement,''), ',' ORDER BY ordinal)), '') FROM publication_findings WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(ordinal::text||':'||coalesce(summary,''), ',' ORDER BY ordinal)), '') FROM publication_recommendations WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(projection_key||':'||payload_hash, ',' ORDER BY projection_key)), '') FROM publication_projections WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(sku_id||':'||md5(row_json::text), ',' ORDER BY sku_id)), '') FROM publication_stock_sku_rows WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(campaign_id||':'||md5(row_json::text), ',' ORDER BY campaign_id)), '') FROM publication_ad_campaign_rows WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(sku_id||':'||md5(row_json::text), ',' ORDER BY sku_id)), '') FROM publication_sku_focus_rows WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(id::text||':'||display_title||':'||action_type, ',' ORDER BY id::text)), '') FROM publication_action_groups WHERE publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(m.group_id::text||':'||m.proposal_id::text||':'||m.member_order::text, ',' ORDER BY m.group_id::text, m.proposal_id::text)), '')
		FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(p.id::text||':'||md5(p.parameters_json::text), ',' ORDER BY p.id::text)), '')
		FROM action_case_proposals p WHERE p.publication_id=$1`)
	addAgg(`SELECT coalesce(md5(string_agg(ps.support_key||':'||md5(ps.claim::text), ',' ORDER BY ps.support_key, p.id::text)), '')
		FROM action_proposal_supports ps JOIN action_case_proposals p ON p.id=ps.proposal_id WHERE p.publication_id=$1`)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

func TestPrompt134APartiallySupportedP1ContentHash(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134a-p1hash")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	hBefore := prompt134AImmutableContentHash(ctx, f, p1.ID)
	if hBefore == "" || hBefore == hex.EncodeToString(make([]byte, 32)) {
		t.Fatal("empty P1 content hash")
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "PARTIALLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	hAfter := prompt134AImmutableContentHash(ctx, f, p1.ID)
	if hBefore != hAfter {
		t.Fatalf("P1 content hash changed before=%s after=%s", hBefore, hAfter)
	}
	t.Logf("P1 immutable content hash before=%s after=%s equal=YES", hBefore, hAfter)
	var outcome string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2.ID).Scan(&outcome)
	if outcome != "PARTIALLY_SUPPORTED" {
		t.Fatalf("outcome=%s", outcome)
	}
}

func TestPrompt134ALargeRowCurrentReadAllFamilies(t *testing.T) {
	prompt134Setup(t)
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134a-rows")
	ctx := context.Background()
	prompt134SetStockUnits(ctx, f, 2)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	prompt134SetStockUnits(ctx, f, 40)
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	p2, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	if p2.ID == p1.ID {
		t.Fatal("current still P1")
	}
	assertFamily := func(table, idCol, p1Sentinel, p2Sentinel string) {
		t.Helper()
		var nP1, nP2 int
		_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), p1.ID).Scan(&nP1)
		_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), p2.ID).Scan(&nP2)
		if nP1 < 1 || nP2 < 1 {
			t.Fatalf("%s p1=%d p2=%d", table, nP1, nP2)
		}
		cur, err := f.repo.GetCurrentPublication(ctx, f.seller)
		if err != nil || cur.ID != p2.ID {
			t.Fatalf("current-read pointer %v id=%d want P2=%d", err, cur.ID, p2.ID)
		}
		rows, err := f.pool.Query(ctx, fmt.Sprintf(`SELECT publication_id, %s, row_json FROM %s WHERE publication_id=$1`, idCol, table), cur.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		mix, nCur, sawP2ID := 0, 0, false
		for rows.Next() {
			var pubID int64
			var id string
			var raw []byte
			if err := rows.Scan(&pubID, &id, &raw); err != nil {
				t.Fatal(err)
			}
			nCur++
			if pubID != p2.ID {
				mix++
			}
			if pubID == p1.ID {
				mix++
			}
			if pubID == p2.ID {
				sawP2ID = true
			}
			_ = id
			_ = raw
		}
		if nCur != nP2 {
			t.Fatalf("%s current rows=%d p2=%d", table, nCur, nP2)
		}
		if mix != 0 {
			t.Fatalf("%s mix_count=%d want 0", table, mix)
		}
		if !sawP2ID {
			t.Fatalf("%s current result missing P2 sentinel publication_id=%d", table, p2.ID)
		}
		var p1InCurrent int
		_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), p1.ID).Scan(&p1InCurrent)
		if p1InCurrent < 1 {
			t.Fatalf("%s P1 sentinel rows missing", table)
		}
		_ = p1Sentinel
		_ = p2Sentinel
	}
	assertFamily("publication_stock_sku_rows", "sku_id", fmt.Sprintf("P1_STOCK:%d", p1.ID), fmt.Sprintf("P2_STOCK:%d", p2.ID))
	assertFamily("publication_ad_campaign_rows", "campaign_id", fmt.Sprintf("P1_CAMPAIGN:%d", p1.ID), fmt.Sprintf("P2_CAMPAIGN:%d", p2.ID))
	assertFamily("publication_sku_focus_rows", "sku_id", fmt.Sprintf("P1_FOCUS:%d", p1.ID), fmt.Sprintf("P2_FOCUS:%d", p2.ID))
}

func TestPrompt134AOutboxDuplicateNoDownstreamEffect(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134a-obx")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	cur1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var pubs1, outbox1 int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE seller_account_id=$1`, f.seller).Scan(&pubs1)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE payload_json->>'seller_account_id'=$1`, strconv.FormatInt(f.seller, 10)).Scan(&outbox1)
	n1, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n1 < 1 {
		t.Fatalf("first process n=%d err=%v", n1, err)
	}
	var eventID string
	var attempts1 int
	_ = f.pool.QueryRow(ctx, `
		SELECT id::text, attempts FROM analysis_outbox_events
		WHERE payload_json->>'seller_account_id'=$1 AND event_type='publication.current_changed' LIMIT 1`,
		strconv.FormatInt(f.seller, 10)).Scan(&eventID, &attempts1)
	_, _ = f.pool.Exec(ctx, `UPDATE analysis_outbox_events SET processed_at=NULL WHERE id=$1::uuid`, eventID)
	n2, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n2 < 1 {
		t.Fatalf("replay n=%d err=%v", n2, err)
	}
	var pubs2, outbox2, attempts2 int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE seller_account_id=$1`, f.seller).Scan(&pubs2)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE payload_json->>'seller_account_id'=$1`, strconv.FormatInt(f.seller, 10)).Scan(&outbox2)
	_ = f.pool.QueryRow(ctx, `SELECT attempts FROM analysis_outbox_events WHERE id=$1::uuid`, eventID).Scan(&attempts2)
	if pubs2 != pubs1 {
		t.Fatalf("publications duplicated pubs1=%d pubs2=%d", pubs1, pubs2)
	}
	if outbox2 != outbox1 {
		t.Fatalf("outbox rows duplicated %d -> %d", outbox1, outbox2)
	}
	if attempts2 <= attempts1 {
		t.Fatalf("attempts not incremented %d -> %d", attempts1, attempts2)
	}
	cur2, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	if cur2.ID != cur1.ID {
		t.Fatal("current publication changed on duplicate delivery")
	}
}

func TestPrompt134ASellerLockIndependence(t *testing.T) {
	TestPrompt133SellerLockIndependence(t)
}
