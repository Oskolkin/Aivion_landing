//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func prompt134Setup(t *testing.T) {
	t.Helper()
	t.Setenv("OPENAI_DEEP_MAX_INPUT_TOKENS", "131072")
}

type prompt134RunResidue struct {
	Publications, Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows                                     int
	Proposals, Assessments, SupportAssessments                       int
	Groups, GroupMembers, ReleaseEvents                              int
}

func prompt134SnapshotRunResidue(ctx context.Context, f prompt81Fixture, runID int64) prompt134RunResidue {
	var r prompt134RunResidue
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
	return r
}

func prompt134RunResidueEqual(t *testing.T, before, after prompt134RunResidue) {
	t.Helper()
	if before != after {
		t.Fatalf("run residue changed before=%+v after=%+v", before, after)
	}
}

type prompt134Residue struct {
	Publications, Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows                                     int
	Cases, Proposals, Assessments, SupportAssessments                int
	Groups, GroupMembers, ReleaseEvents, Outbox                      int
	UserEvents, SystemEvents                                         int
	CurrentPubID                                                     int64
	CaseStateDigest, SigStateDigest                                  string
}

func prompt134SnapshotResidue(ctx context.Context, f prompt81Fixture, runID int64) prompt134Residue {
	var r prompt134Residue
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, runID).Scan(&r.Publications)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_components c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Components)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_findings c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Findings)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_recommendations c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Recommendations)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_projections c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Projections)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.StockRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_ad_campaign_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.AdRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_sku_focus_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.FocusRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_cases c JOIN action_signatures s ON s.id=c.signature_id JOIN action_case_proposals p ON p.case_id=c.id JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.analysis_run_id=$1`, runID).Scan(&r.Cases)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_proposals c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Proposals)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, runID).Scan(&r.Assessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_support_assessments a JOIN action_case_assessments c ON c.id=a.assessment_id WHERE c.analysis_run_id=$1`, runID).Scan(&r.SupportAssessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.Groups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id JOIN analysis_publications p ON p.id=g.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.GroupMembers)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_release_events c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&r.ReleaseEvents)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_outbox_events WHERE payload_json->>'seller_account_id'=$1`, strconv.FormatInt(f.seller, 10)).Scan(&r.Outbox)
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_user_events e
		JOIN action_cases c ON c.id=e.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1`, f.seller).Scan(&r.UserEvents)
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_system_events e
		JOIN action_cases c ON c.id=e.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1`, f.seller).Scan(&r.SystemEvents)
	var digest string
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(md5(string_agg(st.system_state||':'||st.user_state||':'||c.id::text, ',' ORDER BY c.id::text)), '')
		FROM action_case_state st JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id WHERE s.seller_account_id=$1`, f.seller).Scan(&digest)
	r.CaseStateDigest = digest
	_ = f.pool.QueryRow(ctx, `
		SELECT coalesce(md5(string_agg(ss.suppression_source||':'||coalesce(ss.suppressed_until::text,'')||':'||s.signature_hash, ',' ORDER BY s.signature_hash)), '')
		FROM action_signature_state ss JOIN action_signatures s ON s.id=ss.signature_id
		WHERE s.seller_account_id=$1`, f.seller).Scan(&r.SigStateDigest)
	cur, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err == nil {
		r.CurrentPubID = cur.ID
	}
	return r
}

func prompt134ResidueEqual(t *testing.T, before, after prompt134Residue) {
	t.Helper()
	if before != after {
		t.Fatalf("residue changed before=%+v after=%+v", before, after)
	}
}

type prompt134DuplicateSnapshot struct {
	Publications, Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows                                     int
	Cases, Proposals, Assessments, SupportAssessments                int
	Groups, GroupMembers, ReleaseEvents                              int
	OutboxCurrentChanged                                             int
}

func prompt134DuplicateSnapshotForRun(ctx context.Context, f prompt81Fixture, runID int64) prompt134DuplicateSnapshot {
	var s prompt134DuplicateSnapshot
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_publications WHERE analysis_run_id=$1`, runID).Scan(&s.Publications)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_components c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.Components)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_findings c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.Findings)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_recommendations c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.Recommendations)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_projections c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.Projections)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.StockRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_ad_campaign_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.AdRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_sku_focus_rows c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.FocusRows)
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_cases c
		JOIN action_case_proposals p ON p.case_id=c.id
		JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.analysis_run_id=$1`, runID).Scan(&s.Cases)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_proposals c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.Proposals)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, runID).Scan(&s.Assessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_support_assessments a JOIN action_case_assessments c ON c.id=a.assessment_id WHERE c.analysis_run_id=$1`, runID).Scan(&s.SupportAssessments)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.Groups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id JOIN analysis_publications p ON p.id=g.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.GroupMembers)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_release_events c JOIN analysis_publications p ON p.id=c.publication_id WHERE p.analysis_run_id=$1`, runID).Scan(&s.ReleaseEvents)
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM analysis_outbox_events
		WHERE event_type='publication.current_changed' AND payload_json->>'seller_account_id'=$1`,
		strconv.FormatInt(f.seller, 10)).Scan(&s.OutboxCurrentChanged)
	return s
}

type prompt134P1ImmutableSnapshot struct {
	ContentHash, FindingStatement                      string
	Components, Findings, Recommendations, Projections int
	StockRows, AdRows, FocusRows, Groups, GroupMembers int
	ProposalParams                                     []byte
	ProposalSupportCount                               int
}

func prompt134CaptureP1(ctx context.Context, f prompt81Fixture, pubID int64, runID int64) prompt134P1ImmutableSnapshot {
	var s prompt134P1ImmutableSnapshot
	_ = f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, pubID).Scan(&s.ContentHash)
	_ = f.pool.QueryRow(ctx, `SELECT statement FROM publication_findings WHERE publication_id=$1 ORDER BY ordinal LIMIT 1`, pubID).Scan(&s.FindingStatement)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_components WHERE publication_id=$1`, pubID).Scan(&s.Components)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_findings WHERE publication_id=$1`, pubID).Scan(&s.Findings)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_recommendations WHERE publication_id=$1`, pubID).Scan(&s.Recommendations)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_projections WHERE publication_id=$1`, pubID).Scan(&s.Projections)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_stock_sku_rows WHERE publication_id=$1`, pubID).Scan(&s.StockRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_ad_campaign_rows WHERE publication_id=$1`, pubID).Scan(&s.AdRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_sku_focus_rows WHERE publication_id=$1`, pubID).Scan(&s.FocusRows)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups WHERE publication_id=$1`, pubID).Scan(&s.Groups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`, pubID).Scan(&s.GroupMembers)
	_ = f.pool.QueryRow(ctx, `
		SELECT parameters_json FROM action_case_proposals p
		JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.analysis_run_id=$1 LIMIT 1`, runID).Scan(&s.ProposalParams)
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_proposal_supports ps
		JOIN action_case_proposals p ON p.id=ps.proposal_id
		JOIN analysis_publications pub ON pub.id=p.publication_id WHERE pub.analysis_run_id=$1`, runID).Scan(&s.ProposalSupportCount)
	return s
}

func prompt134AssertP1Immutable(t *testing.T, before, after prompt134P1ImmutableSnapshot) {
	t.Helper()
	if before.ContentHash != after.ContentHash || before.FindingStatement != after.FindingStatement ||
		before.Components != after.Components || before.Findings != after.Findings ||
		before.Recommendations != after.Recommendations || before.Projections != after.Projections ||
		before.StockRows != after.StockRows || before.AdRows != after.AdRows || before.FocusRows != after.FocusRows ||
		before.Groups != after.Groups || before.GroupMembers != after.GroupMembers ||
		before.ProposalSupportCount != after.ProposalSupportCount ||
		string(before.ProposalParams) != string(after.ProposalParams) {
		t.Fatalf("P1 immutable snapshot changed before=%+v after=%+v", before, after)
	}
}

func prompt134SetStockUnits(ctx context.Context, f prompt81Fixture, units int) {
	_, err := f.pool.Exec(ctx, `
		UPDATE stock_snapshot_lines SET sellable_units=$2, provider_valid_stock_count=$2
		WHERE seller_account_id=$1 AND canonical_sku_id IS NOT NULL`, f.seller, units)
	if err != nil {
		panic(err)
	}
}

func prompt134RequirePriceActionP1(t *testing.T, f prompt81Fixture) (actionType, intent, sigHash, caseID, propID string) {
	t.Helper()
	ctx := context.Background()
	err := f.pool.QueryRow(ctx, `
		SELECT s.action_type, s.intent_key, s.signature_hash, c.id::text, p.id::text
		FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_proposals p ON p.id=(SELECT current_proposal_id FROM action_case_state WHERE case_id=c.id)
		WHERE s.seller_account_id=$1 AND s.action_type=$2
		ORDER BY c.occurrence_no LIMIT 1`, f.seller, actioncatalog.TypeSKUAdjustPrice).Scan(&actionType, &intent, &sigHash, &caseID, &propID)
	if err != nil {
		t.Fatalf("P1 must have SKU_ADJUST_PRICE: %v", err)
	}
	return actionType, intent, sigHash, caseID, propID
}

func TestPrompt134SupersededOppositePriceIntentPostgres(t *testing.T) {
	prompt134Setup(t)
	t.Setenv("DEEP_FAKE_PRICE_INTENT", "DECREASE")
	t.Setenv("DEEP_FAKE_PRICE_MENU", "1")
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134-super-price")
	ctx := context.Background()
	prompt134SetStockUnits(ctx, f, 100)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	oldType, oldIntent, oldSig, oldCase, oldProp := prompt134RequirePriceActionP1(t, f)
	if oldIntent != "DECREASE" {
		t.Fatalf("old intent=%s want DECREASE", oldIntent)
	}
	p1ID, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	p1Before := prompt134CaptureP1(ctx, f, p1ID.ID, r1.ID)
	t.Setenv("DEEP_FAKE_ASSESSMENT", "SUPERSEDED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	p1After := prompt134CaptureP1(ctx, f, p1ID.ID, r1.ID)
	prompt134AssertP1Immutable(t, p1Before, p1After)
	var nPriceSig int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(*) FROM action_signatures
		WHERE seller_account_id=$1 AND action_type=$2`, f.seller, actioncatalog.TypeSKUAdjustPrice).Scan(&nPriceSig)
	if nPriceSig != 2 {
		t.Fatalf("price signatures=%d want 2", nPriceSig)
	}
	var newIntent, newSig string
	err := f.pool.QueryRow(ctx, `
		SELECT intent_key, signature_hash FROM action_signatures
		WHERE seller_account_id=$1 AND action_type=$2 AND signature_hash<>$3 LIMIT 1`,
		f.seller, actioncatalog.TypeSKUAdjustPrice, oldSig).Scan(&newIntent, &newSig)
	if err != nil {
		t.Fatal(err)
	}
	if newIntent != "INCREASE" {
		t.Fatalf("new intent=%s want INCREASE", newIntent)
	}
	if newSig == oldSig {
		t.Fatal("signatures must differ")
	}
	var oldSys string
	_ = f.pool.QueryRow(ctx, `
		SELECT system_state FROM action_case_state WHERE case_id=$1::uuid`, oldCase).Scan(&oldSys)
	if oldSys != "SUPERSEDED" {
		t.Fatalf("old case sys=%s", oldSys)
	}
	var newCase, newSys, newUser string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text, st.system_state, st.user_state FROM action_cases c
		JOIN action_case_state st ON st.case_id=c.id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.signature_hash=$1 LIMIT 1`, newSig).Scan(&newCase, &newSys, &newUser)
	if newSys != "ACTIVE" || newUser != "OPEN" {
		t.Fatalf("new case state=%s/%s", newSys, newUser)
	}
	var newPropPub int64
	_ = f.pool.QueryRow(ctx, `SELECT publication_id FROM action_case_proposals WHERE id=(SELECT current_proposal_id FROM action_case_state WHERE case_id=$1::uuid)`, newCase).Scan(&newPropPub)
	var p2Pub int64
	_ = f.pool.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, r2.ID).Scan(&p2Pub)
	if newPropPub != p2Pub {
		t.Fatalf("new proposal pub=%d p2=%d", newPropPub, p2Pub)
	}
	var oldParams []byte
	_ = f.pool.QueryRow(ctx, `SELECT parameters_json FROM action_case_proposals WHERE id=$1::uuid`, oldProp).Scan(&oldParams)
	var oldParams2 []byte
	_ = f.pool.QueryRow(ctx, `SELECT parameters_json FROM action_case_proposals WHERE id=$1::uuid`, oldProp).Scan(&oldParams2)
	if string(oldParams) != string(oldParams2) {
		t.Fatal("old proposal parameters mutated")
	}
	_ = oldType
}

func TestPrompt134DismissedOppositePriceIntentPostgres(t *testing.T) {
	prompt134Setup(t)
	t.Setenv("DEEP_FAKE_PRICE_INTENT", "DECREASE")
	t.Setenv("DEEP_FAKE_PRICE_MENU", "1")
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134-dismiss-price")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	_, oldIntent, oldSig, caseID, _ := prompt134RequirePriceActionP1(t, f)
	if oldIntent != "DECREASE" {
		t.Fatalf("old intent=%s", oldIntent)
	}
	t0 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DISMISSED", "test", "", t0); err != nil {
		t.Fatal(err)
	}
	var oldSuppressed *time.Time
	var oldSupSource *string
	_ = f.pool.QueryRow(ctx, `
		SELECT suppressed_until, suppression_source FROM action_signature_state
		WHERE signature_id=(SELECT signature_id FROM action_cases WHERE id=$1::uuid)`, caseID).Scan(&oldSuppressed, &oldSupSource)
	if oldSuppressed == nil || oldSupSource == nil || *oldSupSource != "USER_DISMISSED" {
		t.Fatalf("old suppression=%v source=%v", oldSuppressed, oldSupSource)
	}
	t.Setenv("DEEP_FAKE_ASSESSMENT", "SUPERSEDED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	var newIntent, newSig string
	err := f.pool.QueryRow(ctx, `
		SELECT intent_key, signature_hash FROM action_signatures
		WHERE seller_account_id=$1 AND action_type=$2 AND signature_hash<>$3 LIMIT 1`,
		f.seller, actioncatalog.TypeSKUAdjustPrice, oldSig).Scan(&newIntent, &newSig)
	if err != nil {
		t.Fatal(err)
	}
	if newIntent != "INCREASE" {
		t.Fatalf("new intent=%s want INCREASE", newIntent)
	}
	var newSys, newUser string
	_ = f.pool.QueryRow(ctx, `
		SELECT st.system_state, st.user_state FROM action_case_state st
		JOIN action_cases c ON c.id=st.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.signature_hash=$1 LIMIT 1`, newSig).Scan(&newSys, &newUser)
	if newSys != "ACTIVE" || newUser != "OPEN" {
		t.Fatalf("new case=%s/%s", newSys, newUser)
	}
	var newSupSource *string
	_ = f.pool.QueryRow(ctx, `
		SELECT suppression_source FROM action_signature_state
		WHERE signature_id=(SELECT id FROM action_signatures WHERE signature_hash=$1)`, newSig).Scan(&newSupSource)
	if newSupSource != nil && *newSupSource == "USER_DISMISSED" {
		t.Fatal("new signature incorrectly user-dismissed suppressed")
	}
	var oldUser string
	_ = f.pool.QueryRow(ctx, `SELECT user_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&oldUser)
	if oldUser != "DISMISSED" {
		t.Fatalf("old user=%s", oldUser)
	}
}

func TestPrompt134ContinuityCurrentStockValues(t *testing.T) {
	prompt134Setup(t)
	t.Setenv("DEEP_FAKE_STOCK_SUPPORT", "1")
	t.Setenv("DEEP_FAKE_PRICE_MENU", "1")
	t.Setenv("DEEP_FAKE_PRICE_INTENT", "DECREASE")
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134-cont-stock")
	ctx := context.Background()
	prompt134SetStockUnits(ctx, f, 2)
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	prompt134SetStockUnits(ctx, f, 40)
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, string(stages.BuildDeepPack))
	pack2 := prompt91LoadStageBody(t, f, store, r2.ID, string(stages.BuildDeepPack))
	var wire2 deep.PackWireV1
	if err := json.Unmarshal(pack2, &wire2); err != nil {
		t.Fatal(err)
	}
	found40, found2Current := false, false
	p2Handle := ""
	for _, link := range wire2.Payload.ContinuitySupportEvidenceLinks {
		for _, it := range wire2.Payload.EvidenceRegistry {
			if it.EvidenceID != link.EvidenceID || !strings.Contains(it.EvidenceType, "STOCK") {
				continue
			}
			raw := string(it.Data)
			if stockObsHasUnits(raw, 40) {
				found40 = true
				p2Handle = firstEvidenceHandle(wire2.Payload, it.EvidenceID)
			}
			if stockObsLatestUnits(raw) == 2 {
				found2Current = true
			}
		}
	}
	if !found40 {
		t.Fatal("P2 continuity stock evidence missing sellable_units=40")
	}
	if found2Current {
		t.Fatal("stale P1 stock value 2 is current continuity evidence")
	}
	if p2Handle == "" {
		t.Fatal("missing P2 stock fact handle for units=40")
	}
}

func stockObsLatestUnits(raw string) int {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return -1
	}
	obs, ok := top["observations"]
	if !ok {
		return -1
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(obs, &rows); err != nil || len(rows) == 0 {
		return -1
	}
	row := rows[0]
	if v, ok := row["sellable_units"]; ok {
		var n int64
		if err := json.Unmarshal(v, &n); err == nil {
			return int(n)
		}
	}
	return -1
}

func stockObsHasUnits(raw string, want int) bool {
	wantStr := strconv.Itoa(want)
	if strings.Contains(raw, "\"sellable_units\":"+wantStr) ||
		strings.Contains(raw, "\"sellable_units\":\""+wantStr+"\"") {
		return true
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return false
	}
	obs, ok := top["observations"]
	if !ok {
		return false
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(obs, &rows); err != nil {
		return false
	}
	for _, row := range rows {
		if v, ok := row["sellable_units"]; ok {
			var n int64
			if err := json.Unmarshal(v, &n); err == nil && int(n) == want {
				return true
			}
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				if i, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil && int(i) == want {
					return true
				}
			}
		}
	}
	return false
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func firstEvidenceHandle(p deep.PayloadV1, evidenceID string) string {
	for _, f := range p.FactRegistry {
		if f.SourceRef == evidenceID && f.SourceKind == "EVIDENCE" {
			return f.Handle
		}
	}
	return ""
}

func TestPrompt134OverBudgetProviderCallsZero(t *testing.T) {
	deep.TestProviderCallCount = 0
	f := prompt91Seed(t)
	t.Setenv("OPENAI_DEEP_MAX_INPUT_TOKENS", "2048")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	exec, _ := prompt132Exec(t, f, "p134-budget-calls")
	ctx := context.Background()
	r1 := f.prodRun(t)
	var execErr error
	for _, key := range stages.ProductionImplementedStageNames() {
		execErr = exec.ExecuteStage(ctx, r1.ID, key)
		if execErr != nil {
			break
		}
		if key == string(stages.BuildDeepPack) {
			break
		}
	}
	if deep.TestProviderCallCount != 0 {
		t.Fatalf("provider_calls=%d want 0", deep.TestProviderCallCount)
	}
	st, _ := exec.Repo.GetStage(ctx, r1.ID, string(stages.BuildDeepPack))
	if st.ErrorCode != deep.CodePackTooLarge {
		t.Fatalf("ErrorCode=%s want %s msg=%s execErr=%v", st.ErrorCode, deep.CodePackTooLarge, st.ErrorMessage, execErr)
	}
	if st.Status == "SUCCEEDED" || st.AcceptedAttemptID != nil {
		t.Fatal("BUILD_DEEP_PACK must not succeed")
	}
	stAI, _ := exec.Repo.GetStage(ctx, r1.ID, string(stages.DeepAI))
	if stAI.Status == "SUCCEEDED" || stAI.AcceptedAttemptID != nil {
		t.Fatal("DEEP_AI must not succeed")
	}
	var deepOut int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM analysis_artifacts WHERE run_id=$1 AND kind=ANY($2)`,
		r1.ID, []string{string(artifacts.DeepOutput), string(artifacts.DeepValidatedOutput), string(artifacts.DeepOutputAttempt)}).Scan(&deepOut)
	if deepOut != 0 {
		t.Fatalf("deep result artifacts=%d want 0", deepOut)
	}
}

func TestPrompt134ConcurrentPublishFullLoserResidue(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134-conc-full")
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
		t.Fatalf("wins=%d stale=%d", wins, stale)
	}
	loser := prompt134SnapshotRunResidue(ctx, f, loserRun)
	base := beforeA
	if loserRun == rB.ID {
		base = beforeB
	}
	prompt134RunResidueEqual(t, base, loser)
}

func TestPrompt134UnknownCommitFullDuplicateSnapshot(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134-unk-full")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	in := prompt132PublishInput(t, f, store, r1.ID, "unk-full")
	analysispg.TestPublishUnknownCommitAfterSuccess = true
	res1, err := f.repo.PublishAtomic(ctx, in)
	analysispg.TestPublishUnknownCommitAfterSuccess = false
	if !errors.Is(err, analysispg.ErrPublishUnknownCommit) {
		t.Fatalf("err=%v", err)
	}
	snap1 := prompt134DuplicateSnapshotForRun(ctx, f, r1.ID)
	res2, err := f.repo.PublishAtomic(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Idempotent || res2.PublicationID != res1.PublicationID {
		t.Fatalf("retry idempotent=%v id1=%d id2=%d", res2.Idempotent, res1.PublicationID, res2.PublicationID)
	}
	snap2 := prompt134DuplicateSnapshotForRun(ctx, f, r1.ID)
	if snap1 != snap2 {
		t.Fatalf("duplicate snapshot changed before=%+v after=%+v", snap1, snap2)
	}
}

func TestPrompt134OutboxDuplicateBusinessEffectOnce(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134-obx-dup")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.Publish))
	analysispg.ResetOutboxTestCounters()
	analysispg.TestOutboxSellerFilter = f.seller
	defer func() { analysispg.TestOutboxSellerFilter = 0 }()
	n1, err := f.repo.ProcessOutbox(ctx, 10)
	if err != nil || n1 < 1 {
		t.Fatalf("first delivery n=%d err=%v", n1, err)
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
		t.Fatalf("replay n=%d err=%v", n2, err)
	}
	if analysispg.TestOutboxDeliveryAttemptCount != 2 {
		t.Fatalf("delivery_attempt_count=%d want 2", analysispg.TestOutboxDeliveryAttemptCount)
	}
	if analysispg.TestOutboxSideEffectCount != 1 {
		t.Fatalf("business_effect_count=%d want 1", analysispg.TestOutboxSideEffectCount)
	}
}

func TestPrompt134SixProjectionMandatoryMetadata(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "FULLY_SUPPORTED")
	ctx := context.Background()
	p2, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var date string
	var cutoff time.Time
	var uni string
	_ = f.pool.QueryRow(ctx, `SELECT analysis_date::text, analysis_cutoff_at, factual_universe_hash FROM analysis_publications WHERE id=$1`, p2.ID).Scan(&date, &cutoff, &uni)
	cutStr := cutoff.UTC().Format("2006-01-02T15:04:05Z07:00")
	type check struct {
		key    string
		fields map[string]string
	}
	checks := []check{
		{projection.KeyOverview, map[string]string{
			"seller_account_id":     strconv.FormatInt(f.seller, 10),
			"analysis_date":         date,
			"analysis_cutoff_at":    cutStr,
			"factual_universe_hash": uni,
		}},
		{projection.KeyChatContext, map[string]string{"analysis_date": date}},
	}
	for _, c := range checks {
		raw, err := f.repo.GetProjection(ctx, p2.ID, c.key)
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s missing: %v", c.key, err)
		}
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatal(err)
		}
		for field, want := range c.fields {
			v, ok := meta[field]
			if !ok {
				t.Fatalf("%s missing mandatory field %s", c.key, field)
			}
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				var n int64
				if err2 := json.Unmarshal(v, &n); err2 != nil {
					t.Fatalf("%s field %s unmarshal: %v", c.key, field, err)
				}
				s = strconv.FormatInt(n, 10)
			}
			if s != want {
				t.Fatalf("%s %s=%s want %s", c.key, field, s, want)
			}
		}
	}
	for _, k := range []string{projection.KeyFunnel, projection.KeyStock, projection.KeyAdvertising, projection.KeyActionsSnapshot} {
		raw, err := f.repo.GetProjection(ctx, p2.ID, k)
		if err != nil || len(raw) == 0 {
			t.Fatalf("%s missing", k)
		}
		var meta map[string]json.RawMessage
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatal(err)
		}
		if _, ok := meta["schema_version"]; !ok {
			t.Fatalf("%s missing schema_version", k)
		}
	}
	_ = r2
}

func TestPrompt134LargeRowP1P2AllFamilies(t *testing.T) {
	f, r1, r2 := prompt132RunP1P2(t, "FULLY_SUPPORTED")
	ctx := context.Background()
	var p1, p2 int64
	_ = f.pool.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, r1).Scan(&p1)
	_ = f.pool.QueryRow(ctx, `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, r2).Scan(&p2)
	assertP2Only := func(table string) {
		var nP1, nP2, curRows int
		_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), p1).Scan(&nP1)
		_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), p2).Scan(&nP2)
		if nP1 < 1 || nP2 < 1 {
			t.Fatalf("%s p1=%d p2=%d want both populated", table, nP1, nP2)
		}
		cur, _ := f.repo.GetCurrentPublication(ctx, f.seller)
		_ = f.pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE publication_id=$1`, table), cur.ID).Scan(&curRows)
		if curRows != nP2 {
			t.Fatalf("%s current rows=%d p2=%d", table, curRows, nP2)
		}
	}
	assertP2Only("publication_stock_sku_rows")
	assertP2Only("publication_ad_campaign_rows")
	assertP2Only("publication_sku_focus_rows")
	cur, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	if cur.ID != p2 {
		t.Fatal("current not p2")
	}
}

func TestPrompt134PartiallySupportedP1BeforeAfter(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134-partial-p1")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	p1Before := prompt134CaptureP1(ctx, f, p1.ID, r1.ID)
	t.Setenv("DEEP_FAKE_ASSESSMENT", "PARTIALLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")
	p1After := prompt134CaptureP1(ctx, f, p1.ID, r1.ID)
	prompt134AssertP1Immutable(t, p1Before, p1After)
	var outcome string
	_ = f.pool.QueryRow(ctx, `SELECT outcome FROM action_case_assessments WHERE analysis_run_id=$1`, r2.ID).Scan(&outcome)
	if outcome != "PARTIALLY_SUPPORTED" {
		t.Fatalf("outcome=%s", outcome)
	}
}

func prompt134RollbackFull(t *testing.T, fp analysispg.PublishFailpoint) {
	t.Helper()
	f := prompt91Seed(t)
	exec, store := prompt132Exec(t, f, "p134-roll-"+string(fp))
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, string(stages.ValidatePublication))
	before := prompt134SnapshotResidue(ctx, f, r1.ID)
	analysispg.TestPublishFailpoint = fp
	defer func() { analysispg.TestPublishFailpoint = "" }()
	_, err := f.repo.PublishAtomic(ctx, prompt132PublishInput(t, f, store, r1.ID, "roll"))
	if err == nil {
		t.Fatal("expected failpoint")
	}
	after := prompt134SnapshotResidue(ctx, f, r1.ID)
	prompt134ResidueEqual(t, before, after)
}

func TestPrompt134RollbackPublicationInsertActionState(t *testing.T) {
	prompt134RollbackFull(t, analysispg.FailpointAfterPublicationInsert)
}
func TestPrompt134RollbackActionMutationsActionState(t *testing.T) {
	prompt134RollbackFull(t, analysispg.FailpointAfterActionMutations)
}
func TestPrompt134RollbackProjectionsActionState(t *testing.T) {
	prompt134RollbackFull(t, analysispg.FailpointAfterProjections)
}
func TestPrompt134RollbackBeforePointerActionState(t *testing.T) {
	prompt134RollbackFull(t, analysispg.FailpointBeforeCurrentPointer)
}

func TestPrompt134ActionGroupImmutabilityPopulated(t *testing.T) {
	prompt134Setup(t)
	t.Setenv("DEEP_FAKE_OPPOSITE_PRICE_PAIR", "1")
	t.Setenv("DEEP_FAKE_PRICE_MENU", "1")
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p134-grp-immut")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	p1, _ := f.repo.GetCurrentPublication(ctx, f.seller)
	var nGroups, nMembers int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups WHERE publication_id=$1`, p1.ID).Scan(&nGroups)
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`, p1.ID).Scan(&nMembers)
	if nGroups < 1 || nMembers < 1 {
		t.Fatalf("groups=%d members=%d must be populated", nGroups, nMembers)
	}
	attacks := []string{
		`UPDATE publication_action_groups SET display_title='x' WHERE publication_id=$1`,
		`DELETE FROM publication_action_groups WHERE publication_id=$1`,
		`UPDATE publication_action_group_members SET member_order=member_order+1 WHERE group_id IN (SELECT id FROM publication_action_groups WHERE publication_id=$1)`,
		`DELETE FROM publication_action_group_members WHERE group_id IN (SELECT id FROM publication_action_groups WHERE publication_id=$1)`,
	}
	for _, sql := range attacks {
		_, err := f.pool.Exec(ctx, sql, p1.ID)
		if err == nil {
			t.Fatalf("expected reject: %s", sql)
		}
	}
}
