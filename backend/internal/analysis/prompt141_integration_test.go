//go:build integration

package analysis_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboard"
	"github.com/google/uuid"
)

func hex64(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func prompt141InsertCase(t *testing.T, f prompt81Fixture, pubID int64, target string, order int) (caseID, propID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	caseID = uuid.New()
	propID = uuid.New()
	sig := hex64(fmt.Sprintf("p141-sig-%d-%s-%s", f.seller, target, caseID.String()))
	nctx := hex64("p141-ctx")
	var sigID int64
	if err := f.pool.QueryRow(ctx, `
		INSERT INTO action_signatures(seller_account_id,signature_hash,action_type,target_kind,target_id,normalized_context_hash,normalized_context_json,intent_key)
		VALUES($1,$2,'SKU_INCREASE_REPLENISHMENT','SKU',$3,$4,'{}'::jsonb,'INCREASE') RETURNING id`,
		f.seller, sig, target, nctx).Scan(&sigID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO action_signature_state(signature_id,last_occurrence_no,updated_at) VALUES($1,1,NOW())`, sigID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO action_cases(id,signature_id,occurrence_no,opened_by_publication_id) VALUES($1,$2,1,$3)`, caseID, sigID, pubID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO action_case_proposals(id,case_id,publication_id,recommendation_id,action_type,parameters_json,display_order)
		VALUES($1,$2,$3,'R-P141','SKU_INCREASE_REPLENISHMENT','{}'::jsonb,$4)`, propID, caseID, pubID, order); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO action_case_state(case_id,system_state,user_state,current_proposal_id,updated_at) VALUES($1,'ACTIVE','OPEN',$2,NOW())`, caseID, propID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE action_signature_state SET active_case_id=$2 WHERE signature_id=$1`, sigID, caseID); err != nil {
		t.Fatal(err)
	}
	return caseID, propID
}

func prompt141SeedGroup(t *testing.T, f prompt81Fixture, pubID int64, rec string, members int, displayOrder int) (uuid.UUID, []string) {
	t.Helper()
	ctx := context.Background()
	gid := uuid.New()
	ids := make([]string, 0, members)
	type pair struct{ c, p uuid.UUID }
	pairs := make([]pair, 0, members)
	for i := 0; i < members; i++ {
		c, p := prompt141InsertCase(t, f, pubID, fmt.Sprintf("%s-sku-%d", rec, i), i+1)
		ids = append(ids, c.String())
		pairs = append(pairs, pair{c, p})
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO publication_action_groups(id,publication_id,recommendation_id,action_type,target_kind,family,display_order,display_title,display_summary,presentation_template_key)
		VALUES($1,$2,$3,'SKU_INCREASE_REPLENISHMENT','SKU','INVENTORY',$4,$5,'Пополнить запас','SKU_INCREASE_REPLENISHMENT')`,
		gid, pubID, rec, displayOrder, fmt.Sprintf("Пополнить %d товара", members)); err != nil {
		t.Fatal(err)
	}
	for i, pr := range pairs {
		if _, err := f.pool.Exec(ctx, `INSERT INTO publication_action_group_members(group_id,case_id,proposal_id,member_order) VALUES($1,$2,$3,$4)`,
			gid, pr.c, pr.p, i+1); err != nil {
			t.Fatal(err)
		}
	}
	return gid, ids
}

func TestPrompt141FourMemberGroupLifecycle(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	ctx := context.Background()
	pubID := prompt14PubID(t, f, runID)
	var hashBefore string
	if err := f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, pubID).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	gid, cases := prompt141SeedGroup(t, f, pubID, "R-P141-FOUR", 4, 1)
	av := actionview.NewService(f.repo)
	list, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var g *actionview.GroupView
	wantID := actionview.EncodeGroup(gid.String())
	nFour := 0
	for i := range list.Groups {
		if list.Groups[i].ActionViewID == wantID {
			g = &list.Groups[i]
		}
		if list.Groups[i].ActiveMemberCount == 4 && list.Groups[i].ActionType == "SKU_INCREASE_REPLENISHMENT" && len(list.Groups[i].Members) == 4 {
			nFour++
		}
	}
	if g == nil {
		t.Fatal("four-member group missing")
	}
	if g.ActiveMemberCount != 4 || len(g.Members) != 4 {
		t.Fatalf("count=%d members=%d", g.ActiveMemberCount, len(g.Members))
	}
	if nFour != 1 {
		t.Fatalf("top-level four-member groups=%d want 1", nFour)
	}
	counts := []int{4}
	for i, id := range cases {
		if err := av.Mutate(ctx, f.seller, nil, id, "DONE"); err != nil {
			t.Fatal(err)
		}
		again, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		active := 0
		for _, gg := range again.Groups {
			if gg.ActionViewID == wantID {
				found = true
				active = gg.ActiveMemberCount
			}
		}
		want := 3 - i
		if want < 0 {
			want = 0
		}
		if want == 0 {
			if found {
				t.Fatal("group must leave Active at 0")
			}
			counts = append(counts, 0)
			continue
		}
		if !found || active != want {
			t.Fatalf("after DONE #%d found=%v active=%d want=%d", i+1, found, active, want)
		}
		counts = append(counts, active)
	}
	var hashAfter string
	if err := f.pool.QueryRow(ctx, `SELECT publication_content_hash FROM analysis_publications WHERE id=$1`, pubID).Scan(&hashAfter); err != nil {
		t.Fatal(err)
	}
	if hashBefore != hashAfter {
		t.Fatal("publication hash changed")
	}
	t.Logf("lifecycle counts=%v group=%s pub=%d hash_before=%s hash_after=%s equal=YES", counts, gid, pubID, hashBefore, hashAfter)
	hist, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "done", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	done := 0
	for _, h := range hist.History {
		for _, id := range cases {
			if h.CaseID == id {
				done++
			}
		}
	}
	if done != 4 {
		t.Fatalf("history done=%d", done)
	}
}

func TestPrompt141DismissMemberKeepsGroup(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	ctx := context.Background()
	pubID := prompt14PubID(t, f, runID)
	gid, cases := prompt141SeedGroup(t, f, pubID, "R-P141-DISMISS", 4, 2)
	av := actionview.NewService(f.repo)
	if err := av.Mutate(ctx, f.seller, nil, cases[0], "DISMISSED"); err != nil {
		t.Fatal(err)
	}
	list, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	wantID := actionview.EncodeGroup(gid.String())
	for _, g := range list.Groups {
		if g.ActionViewID == wantID {
			if g.ActiveMemberCount != 3 {
				t.Fatalf("active=%d", g.ActiveMemberCount)
			}
			return
		}
	}
	t.Fatal("group missing after dismiss")
}

func TestPrompt141ActionCursorThreePages(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	ctx := context.Background()
	pubID := prompt14PubID(t, f, runID)
	for i := 0; i < 8; i++ {
		prompt141SeedGroup(t, f, pubID, fmt.Sprintf("R-P141-PAGE-%d", i), 1, i+10)
	}
	av := actionview.NewService(f.repo)
	ftr := actionview.ListFilter{Status: "active", Limit: 3}
	var seen []string
	pages := 0
	cursor := ""
	for {
		ftr.Cursor = cursor
		resp, err := av.List(ctx, f.seller, ftr)
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, g := range resp.Groups {
			seen = append(seen, g.ActionViewID)
		}
		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
		if cursor == "" {
			t.Fatal("has_more without cursor")
		}
		if pages > 20 {
			t.Fatal("too many pages")
		}
	}
	uniq := map[string]struct{}{}
	for _, id := range seen {
		if _, ok := uniq[id]; ok {
			t.Fatalf("duplicate %s", id)
		}
		uniq[id] = struct{}{}
	}
	t.Logf("pagination pages=%d views=%d unique=%d", pages, len(seen), len(uniq))
	if pages < 3 {
		t.Fatalf("pages=%d views=%d", pages, len(seen))
	}
	other := prompt81Seller(t, f.pool)
	bad := actionview.BindCursor(other, actionview.ListFilter{Status: "active"}, 1, seen[0])
	_, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 3, Cursor: bad})
	if err == nil {
		t.Fatal("cross-seller cursor must reject")
	}
	resp, err := av.List(ctx, other, actionview.ListFilter{Status: "active", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Groups) != 0 && resp.State != "PRE_ANALYSIS" {
		t.Fatalf("other seller leaked groups state=%s n=%d", resp.State, len(resp.Groups))
	}
}

func TestPrompt141DashboardAndRowQueryShape(t *testing.T) {
	f, runID := prompt14FirstPublication(t)
	ctx := context.Background()
	pubID := prompt14PubID(t, f, runID)
	pool, counter := newCountedPool(t)
	repo := analysispg.New(pool)
	dash := dashboard.NewService(repo)
	av := actionview.NewService(repo)
	for i := 0; i < 500; i++ {
		sku := fmt.Sprintf("scale-500-%d", i)
		raw := fmt.Sprintf(`{"sku_id":%q,"sellable_units_state":"KNOWN","sellable_units":1}`, sku)
		if _, err := f.pool.Exec(ctx, `INSERT INTO publication_stock_sku_rows(publication_id,sku_id,row_json) VALUES($1,$2,$3::jsonb) ON CONFLICT DO NOTHING`, pubID, sku, raw); err != nil {
			t.Fatal(err)
		}
	}
	counter.Reset()
	start := time.Now()
	st500, err := dash.StockSKUs(ctx, f.seller, pubID, "", "", nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	st500Dur := time.Since(start)
	st500SQL := counter.Count()
	sb500, _ := json.Marshal(st500)
	t.Logf("STOCK500 page SQL=%d rows=%d dur=%s bytes=%d", st500SQL, len(st500.Items), st500Dur, len(sb500))
	if st500SQL > 8 {
		t.Fatalf("stock 500 N+1 SQL=%d", st500SQL)
	}
	for i := 0; i < 2000; i++ {
		sku := fmt.Sprintf("scale-2000-%d", i)
		raw := fmt.Sprintf(`{"sku_id":%q,"sellable_units_state":"KNOWN","sellable_units":1}`, sku)
		if _, err := f.pool.Exec(ctx, `INSERT INTO publication_stock_sku_rows(publication_id,sku_id,row_json) VALUES($1,$2,$3::jsonb) ON CONFLICT DO NOTHING`, pubID, sku, raw); err != nil {
			t.Fatal(err)
		}
	}
	counter.Reset()
	start = time.Now()
	st2000, err := dash.StockSKUs(ctx, f.seller, pubID, "", "", nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	st2000Dur := time.Since(start)
	st2000SQL := counter.Count()
	sb2000, _ := json.Marshal(st2000)
	t.Logf("STOCK2000 page SQL=%d rows=%d dur=%s bytes=%d", st2000SQL, len(st2000.Items), st2000Dur, len(sb2000))
	if st2000SQL > 8 {
		t.Fatalf("stock 2000 N+1 SQL=%d", st2000SQL)
	}
	for i := 0; i < 40; i++ {
		cid := fmt.Sprintf("camp-%d", i)
		raw := fmt.Sprintf(`{"campaign_id":%q,"budget_state":"KNOWN","budget_amount":"0"}`, cid)
		if _, err := f.pool.Exec(ctx, `INSERT INTO publication_ad_campaign_rows(publication_id,campaign_id,row_json) VALUES($1,$2,$3::jsonb) ON CONFLICT DO NOTHING`, pubID, cid, raw); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 100; i++ {
		prompt141SeedGroup(t, f, pubID, fmt.Sprintf("R-P141-PERF-%d", i), 5, 100+i)
	}
	counter.Reset()
	start = time.Now()
	home, err := dash.Home(ctx, f.seller)
	if err != nil {
		t.Fatal(err)
	}
	homeDur := time.Since(start)
	homeSQL := counter.Count()
	hb, _ := json.Marshal(home)
	if homeSQL > 20 {
		t.Fatalf("dashboard home SQL=%d scales too high", homeSQL)
	}
	t.Logf("DASHBOARD home SQL=%d dur=%s bytes=%d", homeSQL, homeDur, len(hb))

	counter.Reset()
	start = time.Now()
	ads, err := dash.AdCampaigns(ctx, f.seller, pubID, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	adDur := time.Since(start)
	adSQL := counter.Count()
	ab, _ := json.Marshal(ads)
	t.Logf("ADS page SQL=%d rows=%d dur=%s bytes=%d", adSQL, len(ads.Items), adDur, len(ab))
	if adSQL > 8 {
		t.Fatalf("ads page N+1 SQL=%d", adSQL)
	}

	counter.Reset()
	start = time.Now()
	acts, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	actDur := time.Since(start)
	actSQL := counter.Count()
	actb, _ := json.Marshal(acts)
	t.Logf("ACTIONS list SQL=%d groups_page=%d seeded_groups=100 seeded_cases=500 dur=%s bytes=%d", actSQL, len(acts.Groups), actDur, len(actb))
	if actSQL > 15 {
		t.Fatalf("actions list N+1 SQL=%d", actSQL)
	}
}
