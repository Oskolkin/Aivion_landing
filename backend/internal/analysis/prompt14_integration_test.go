//go:build integration

package analysis_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/projection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboard"
)

func prompt14PubID(t *testing.T, f prompt81Fixture, runID int64) int64 {
	t.Helper()
	var id int64
	err := f.pool.QueryRow(context.Background(), `SELECT id FROM analysis_publications WHERE analysis_run_id=$1`, runID).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func prompt14FirstPublication(t *testing.T) (prompt81Fixture, int64) {
	t.Helper()
	t.Setenv("DEEP_FAKE_ASSESSMENT", "FULLY_SUPPORTED")
	f := prompt91Seed(t)
	exec, _ := prompt132Exec(t, f, "p14-first")
	run := f.prodRun(t)
	prompt132RunThrough(t, exec, run.ID, "")
	return f, run.ID
}

func TestPrompt14DashboardServesCurrentPublicationOnly(t *testing.T) {
	f, r1, r2 := prompt132RunP1P2(t, "PARTIALLY_SUPPORTED")
	ctx := context.Background()
	p1 := prompt14PubID(t, f, r1)
	p2 := prompt14PubID(t, f, r2)
	svc := dashboard.NewService(f.repo)
	home, err := svc.Home(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if home.State != "READY" {
		t.Fatalf("state=%s error=%v", home.State, home.Error)
	}
	if home.Publication == nil || home.Publication.ID != strconv.FormatInt(p2, 10) {
		t.Fatalf("publication=%v want P2=%d", home.Publication, p2)
	}
	if home.Overview == nil || home.Funnel == nil || home.Stock == nil || home.Advertising == nil {
		t.Fatal("all domain projections required")
	}
	if home.Overview.SchemaVersion != projection.OverviewV2 {
		t.Fatalf("overview schema=%s", home.Overview.SchemaVersion)
	}
	if home.Funnel.SchemaVersion != projection.FunnelV2 {
		t.Fatalf("funnel schema=%s", home.Funnel.SchemaVersion)
	}
	if home.Stock.SchemaVersion != projection.StockV2 || home.Advertising.SchemaVersion != projection.AdvertisingV2 {
		t.Fatalf("stock=%s ads=%s", home.Stock.SchemaVersion, home.Advertising.SchemaVersion)
	}
	if home.Overview.AnalysisCutoffAt == "" || home.Overview.SellerAccountID != f.seller {
		t.Fatalf("overview not bound to seller/cutoff")
	}
	_ = p1
}

func TestPrompt14LazyRowsStayPinnedToRequestedPublication(t *testing.T) {
	f, r1, r2 := prompt132RunP1P2(t, "PARTIALLY_SUPPORTED")
	ctx := context.Background()
	p1 := prompt14PubID(t, f, r1)
	p2 := prompt14PubID(t, f, r2)
	svc := dashboard.NewService(f.repo)
	home, err := svc.Home(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	if home.Publication.ID != strconv.FormatInt(p2, 10) {
		t.Fatalf("current=%s", home.Publication.ID)
	}
	stock, err := svc.StockSKUs(ctx, f.seller, p1, "", "", nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stock.PublicationID != strconv.FormatInt(p1, 10) {
		t.Fatalf("stock pin=%s", stock.PublicationID)
	}
	ads, err := svc.AdCampaigns(ctx, f.seller, p1, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if ads.PublicationID != strconv.FormatInt(p1, 10) {
		t.Fatalf("ads pin=%s", ads.PublicationID)
	}
}

func TestPrompt14CrossSellerPublicationDenied(t *testing.T) {
	f, _, r2 := prompt132RunP1P2(t, "PARTIALLY_SUPPORTED")
	other := prompt81Seller(t, f.pool)
	p2 := prompt14PubID(t, f, r2)
	svc := dashboard.NewService(f.repo)
	home, err := svc.Home(context.Background(), other)
	if err != nil {
		t.Fatal(err)
	}
	if home.State != "PRE_ANALYSIS" {
		t.Fatalf("other seller state=%s", home.State)
	}
	_, err = svc.StockSKUs(context.Background(), other, p2, "", "", nil, 10)
	if err == nil {
		t.Fatal("cross-seller stock rows must fail")
	}
	av := actionview.NewService(f.repo)
	list, err := av.List(context.Background(), other, actionview.ListFilter{Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if list.State != "PRE_ANALYSIS" {
		t.Fatalf("actions state=%s", list.State)
	}
}

func TestPrompt14PreAnalysisHasNoFakePublication(t *testing.T) {
	pool := prompt81Pool(t)
	seller := prompt81Seller(t, pool)
	svc := dashboard.NewService(analysispg.New(pool))
	home, err := svc.Home(context.Background(), seller)
	if err != nil {
		t.Fatal(err)
	}
	if home.State != "PRE_ANALYSIS" || home.Publication != nil {
		t.Fatalf("%+v", home)
	}
	if home.PreAnalysis == nil || home.PreAnalysis.ReasonCode != "NO_CURRENT_PUBLICATION" {
		t.Fatalf("pre_analysis=%+v", home.PreAnalysis)
	}
}

func TestPrompt14ActionOverlayDoneDoesNotChangePublicationHash(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	ctx := context.Background()
	p2 := prompt14PubID(t, f, runID)
	var hashBefore string
	if err := f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, p2).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	av := actionview.NewService(f.repo)
	list, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Groups) == 0 {
		var groups, members, openActive int
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_groups WHERE publication_id=$1`, p2).Scan(&groups)
		_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM publication_action_group_members m JOIN publication_action_groups g ON g.id=m.group_id WHERE g.publication_id=$1`, p2).Scan(&members)
		_ = f.pool.QueryRow(ctx, `
			SELECT count(*) FROM publication_action_group_members m
			JOIN publication_action_groups g ON g.id=m.group_id
			JOIN action_case_state st ON st.case_id=m.case_id
			WHERE g.publication_id=$1 AND st.user_state='OPEN' AND st.system_state='ACTIVE'`, p2).Scan(&openActive)
		t.Fatalf("expected active groups publication=%d groups=%d members=%d open_active=%d state=%s pub=%s", p2, groups, members, openActive, list.State, list.PublicationID)
	}
	g := list.Groups[0]
	before := g.ActiveMemberCount
	if len(g.Members) == 0 {
		t.Fatal("expected members")
	}
	caseID := g.Members[0].CaseID
	if err := av.Mutate(ctx, f.seller, nil, caseID, "DONE"); err != nil {
		t.Fatal(err)
	}
	if err := av.Mutate(ctx, f.seller, nil, caseID, "DONE"); err != nil {
		t.Fatal(err)
	}
	var hashAfter string
	if err := f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, p2).Scan(&hashAfter); err != nil {
		t.Fatal(err)
	}
	if hashBefore != hashAfter {
		t.Fatal("publication hash must be unchanged after DONE")
	}
	again, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var afterCount int
	found := false
	for _, gg := range again.Groups {
		if gg.ActionViewID == g.ActionViewID {
			found = true
			afterCount = gg.ActiveMemberCount
		}
	}
	if before == 1 {
		if found {
			t.Fatal("group with no active members must leave Active list")
		}
	} else if !found || afterCount != before-1 {
		t.Fatalf("active count %d -> found=%v count=%d", before, found, afterCount)
	}
	hist, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "done", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, h := range hist.History {
		if h.CaseID == caseID && h.UserFacingMode == "DONE" {
			ok = true
		}
	}
	if !ok {
		t.Fatal("DONE case missing from history")
	}
}

func TestPrompt14DismissReopenBulk(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	ctx := context.Background()
	av := actionview.NewService(f.repo)
	list, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Groups) == 0 || len(list.Groups[0].Members) == 0 {
		t.Fatal("need members")
	}
	ids := make([]string, 0, len(list.Groups[0].Members))
	for _, m := range list.Groups[0].Members {
		ids = append(ids, m.CaseID)
	}
	target := ids[0]
	if err := av.Mutate(ctx, f.seller, nil, target, "DISMISSED"); err != nil {
		t.Fatal(err)
	}
	irr, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "irrelevant", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, h := range irr.History {
		if h.CaseID == target {
			seen = true
		}
	}
	if !seen {
		t.Fatal("dismissed case not in irrelevant")
	}
	if err := av.Mutate(ctx, f.seller, nil, target, "REOPENED"); err != nil {
		t.Fatal(err)
	}
	if len(ids) > 1 {
		if err := av.Bulk(ctx, f.seller, nil, ids[1:], "DONE"); err != nil {
			t.Fatal(err)
		}
	}
}
