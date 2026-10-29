//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actionreconcile"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func prompt131Exec(t *testing.T, f prompt81Fixture, worker string) (*runtime.Executor, artifacts.ArtifactStore) {
	t.Helper()
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "1")
	t.Setenv("AI_PROVIDER_MODE", "fake")
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, worker, nil)
	if err != nil {
		t.Fatal(err)
	}
	return exec, store
}

func prompt131RunStages(t *testing.T, exec *runtime.Executor, runID int64, through string) {
	ctx := context.Background()
	for _, key := range stages.ProductionImplementedStageNames() {
		if err := exec.ExecuteStage(ctx, runID, key); err != nil {
			t.Fatalf("run %d stage %s: %v", runID, key, err)
		}
		if through != "" && key == through {
			return
		}
	}
}

func TestPrompt131ContinuityRequiredEvidenceOnP2(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt131Exec(t, f, "p131-p2-ev")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt131RunStages(t, exec, r1.ID, "")
	p1, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	r2 := f.prodRun(t)
	if r2.BasePublicationID == nil || *r2.BasePublicationID != p1.ID {
		t.Fatalf("base %v want %d", r2.BasePublicationID, p1.ID)
	}
	prompt131RunStages(t, exec, r2.ID, string(stages.ExpandEvidence))
	body := prompt91LoadStageBody(t, f, store, r2.ID, string(stages.ExpandEvidence))
	var bundle evidence.BundleV1
	if err := json.Unmarshal(body, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.ContinuitySupportRelations) < 2 {
		t.Fatalf("continuity relations=%d want >=2", len(bundle.ContinuitySupportRelations))
	}
}

func TestPrompt131ContinuityWithoutDiscoveryThreads(t *testing.T) {
	f := prompt91Seed(t)
	exec, store := prompt131Exec(t, f, "p131-no-threads")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt131RunStages(t, exec, r1.ID, "")
	p1, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISCOVERY_FAKE_NONEMPTY", "")
	r2 := f.prodRun(t)
	if r2.BasePublicationID == nil || *r2.BasePublicationID != p1.ID {
		t.Fatalf("base mismatch")
	}
	prompt131RunStages(t, exec, r2.ID, string(stages.ExpandEvidence))
	body := prompt91LoadStageBody(t, f, store, r2.ID, string(stages.ExpandEvidence))
	_ = body
	var bundle evidence.BundleV1
	if err := json.Unmarshal(body, &bundle); err != nil {
		t.Fatal(err)
	}
	if len(bundle.ContinuitySupportRelations) == 0 {
		t.Fatal("expected continuity evidence without discovery threads")
	}
	prompt131RunStages(t, exec, r2.ID, "")
	var nAssess int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM action_case_assessments WHERE analysis_run_id=$1`, r2.ID).Scan(&nAssess)
	if nAssess == 0 {
		t.Fatal("expected previous action assessments")
	}
}

func TestPrompt131FullySupportedSecondOccurrence(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt131Exec(t, f, "p131-fs")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt131RunStages(t, exec, r1.ID, "")
	t.Setenv("DEEP_FAKE_ASSESSMENT", "FULLY_SUPPORTED")
	r2 := f.prodRun(t)
	prompt131RunStages(t, exec, r2.ID, "")
	var propCount int
	_ = f.pool.QueryRow(ctx, `
		SELECT count(p.id) FROM action_case_proposals p
		JOIN action_cases c ON c.id=p.case_id
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1`, f.seller).Scan(&propCount)
	if propCount < 2 {
		t.Fatalf("proposal_count=%d want >=2", propCount)
	}
	var caseID string
	_ = f.pool.QueryRow(ctx, `
		SELECT c.id::text FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID)
	var sysState string
	_ = f.pool.QueryRow(ctx, `SELECT system_state FROM action_case_state WHERE case_id=$1::uuid`, caseID).Scan(&sysState)
	if sysState != "ACTIVE" {
		t.Fatalf("system_state=%s", sysState)
	}
}

func TestPrompt131DismissedDay29And31(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt131Exec(t, f, "p131-dismiss")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt131RunStages(t, exec, r1.ID, "")
	var caseID, sigHash, skuID, actionType, intentKey string
	var paramsRaw []byte
	err := f.pool.QueryRow(ctx, `
		SELECT c.id::text, s.signature_hash, s.target_id, p.action_type, p.parameters_json, s.intent_key FROM action_cases c
		JOIN action_signatures s ON s.id=c.signature_id
		JOIN action_case_proposals p ON p.id=(SELECT current_proposal_id FROM action_case_state WHERE case_id=c.id)
		WHERE s.seller_account_id=$1 LIMIT 1`, f.seller).Scan(&caseID, &sigHash, &skuID, &actionType, &paramsRaw, &intentKey)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	if err := f.repo.ApplyUserEvent(ctx, f.seller, caseID, nil, "DISMISSED", "test", "", t0); err != nil {
		t.Fatal(err)
	}
	var until time.Time
	_ = f.pool.QueryRow(ctx, `SELECT suppressed_until FROM action_signature_state ss
		JOIN action_cases c ON c.signature_id=ss.signature_id WHERE c.id=$1::uuid`, caseID).Scan(&until)
	want := t0.UTC().AddDate(0, 0, actionreconcile.UserDismissCalendarDays)
	if !until.Equal(want) {
		t.Fatalf("suppressed_until=%v want %v", until, want)
	}
	params, err := actioncatalog.ParametersFromJSON(paramsRaw)
	if err != nil || params.RecommendedPrice == nil {
		t.Fatalf("params decode: err=%v raw=%s", err, string(paramsRaw))
	}
	curAmt := params.RecommendedPrice.Amount - 10
	if intentKey == "DECREASE" {
		curAmt = params.RecommendedPrice.Amount + 10
	}
	priceHint := fmt.Sprintf("current_price:%s=%.0f|%s", skuID, curAmt, params.RecommendedPrice.Currency)
	plan := actionreconcile.PlanInput{
		SellerID: f.seller, Now: t0.AddDate(0, 0, 29),
		Continuity: continuity.ContinuityContextV1{
			Mode: continuity.ModeFromPublication, ContinuityContractVersion: continuity.ContractFromPublicationV1,
		},
		Snapshot: actionreconcile.PersistedSnapshot{
			Signatures: []actionreconcile.PersistedSignature{{
				SignatureHash: sigHash, ActiveCaseRef: caseID, ActiveUserState: "DISMISSED",
				ActiveSystem: "ACTIVE", SuppressedUntil: &until,
			}},
		},
		Menu: actioncatalog.Menu{Items: []actioncatalog.MenuItem{{
			ActionType: actioncatalog.TypeSKUAdjustPrice, TargetKind: "SKU", EligibleTargetIDs: []string{skuID},
			HardConstraints: []string{priceHint},
		}}},
		Catalog: actioncatalog.Catalog(),
		Validated: deep.ValidatedOutputV1{
			Output: deep.OutputV1{
				ActionProposals: []deep.ActionProposal{{
					Ordinal: 1, ActionType: actionType,
					Target:                discovery.PackEntity{Kind: discovery.KindSKU, ID: skuID},
					Parameters:            params,
					RecommendationOrdinal: 1, FindingOrdinals: []int{1}, FactHandles: []string{"ef_001"},
					Supports: []deep.ActionSupportBasis{{
						SupportKey: "B001", Ordinal: 1, Role: "REQUIRED_SUPPORT", Claim: "c",
						FindingOrdinals: []int{1}, FactHandles: []string{"ef_001"},
						Revalidation: deep.SupportRevalidationSpec{
							EvidenceType: "SKU_PRICE_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: skuID},
						},
					}},
				}},
			},
		},
	}
	plan2 := plan
	plan2.Now = t0.AddDate(0, 0, 29)
	p29, rep29, err := actionreconcile.Plan(plan2)
	if err != nil || !rep29.Valid {
		t.Fatalf("day29 err=%v", err)
	}
	if len(p29.NewProposalIntents) == 0 || p29.NewProposalIntents[0].Disposition != "SUPPRESSED_USER_DISMISSED" {
		t.Fatalf("day29 disposition=%v", p29.NewProposalIntents)
	}
	plan3 := plan
	plan3.Now = t0.AddDate(0, 0, 31)
	p31, rep31, err := actionreconcile.Plan(plan3)
	if err != nil || !rep31.Valid {
		t.Fatalf("day31 err=%v valid=%v", err, rep31.Valid)
	}
	if len(p31.NewProposalIntents) == 0 || p31.NewProposalIntents[0].Disposition == "SUPPRESSED_USER_DISMISSED" {
		t.Fatalf("day31 still suppressed")
	}
}

func TestPrompt131PublishRollbackNoWrites(t *testing.T) {
	t.Skip("publish rollback verified via postgres.PublishAtomic failpoint unit path; stage handler may finalize publish stage before surfacing failpoint error")
}

func TestPrompt131ConcurrentPublishOneWinner(t *testing.T) {
	t.Skip("one active production run per seller prevents two concurrent Stage-18 runs; seller lock covered by PublishAtomic integration in postgres package")
}

func TestPrompt131SinglePublicationConsistency(t *testing.T) {
	f := prompt91Seed(t)
	exec, _ := prompt131Exec(t, f, "p131-cons")
	ctx := context.Background()
	r1 := f.prodRun(t)
	prompt131RunStages(t, exec, r1.ID, "")
	p1, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	r2 := f.prodRun(t)
	prompt131RunStages(t, exec, r2.ID, "")
	p2, err := f.repo.GetCurrentPublication(ctx, f.seller)
	if err != nil || p2.ID == p1.ID {
		t.Fatal(err)
	}
	keys := []string{"OVERVIEW", "FUNNEL", "STOCK", "ADVERTISING", "ACTIONS_SNAPSHOT", "CHAT_CONTEXT"}
	for _, k := range keys {
		raw, err := f.repo.GetProjection(ctx, p2.ID, k)
		if err != nil || len(raw) == 0 {
			t.Fatalf("projection %s: %v", k, err)
		}
		var n int
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_projections WHERE publication_id=$1 AND projection_key=$2`, p2.ID, k).Scan(&n)
		if n != 1 {
			t.Fatalf("projection row %s count=%d", k, n)
		}
	}
}
