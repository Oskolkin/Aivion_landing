//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type prompt15E2EManifest struct {
	SellerAccountID        int64    `json:"seller_account_id"`
	Email                  string   `json:"email"`
	Password               string   `json:"password"`
	P1PublicationID        int64    `json:"p1_publication_id"`
	P2PublicationID        int64    `json:"p2_publication_id"`
	P1UniverseID           string   `json:"p1_universe_id"`
	P2UniverseID           string   `json:"p2_universe_id"`
	CurrentPublicationID   int64    `json:"current_publication_id"`
	SKU417CanonicalID      int64    `json:"sku417_canonical_id"`
	SKU417OzonSKU          int64    `json:"sku417_ozon_sku"`
	GroupID                string   `json:"group_id"`
	GroupActionViewID      string   `json:"group_action_view_id"`
	GroupTitle             string   `json:"group_title"`
	GroupCaseIDs           []string `json:"group_case_ids"`
	SingletonCaseID        string   `json:"singleton_case_id"`
	SingletonActionViewID  string   `json:"singleton_action_view_id"`
	SingletonTitle         string   `json:"singleton_title"`
	HistoricalCaseID       string   `json:"historical_case_id"`
	HistoricalActionViewID string   `json:"historical_action_view_id"`
	HistoricalTitle        string   `json:"historical_title"`
	HistoricalOriginPubID  int64    `json:"historical_origin_publication_id"`
	HistoricalCurrentPubID int64    `json:"historical_current_publication_id"`
	ActiveActionCount      int      `json:"active_action_count"`
	CompletedActionCount   int      `json:"completed_action_count"`
}

// TestE2EPreparePrompt15BackendScenario seeds a canonical Prompt14/15 browser fixture for one seller.
func TestE2EPreparePrompt15BackendScenario(t *testing.T) {
	if os.Getenv("PROMPT15_E2E_PREPARE") != "1" {
		t.Skip("set PROMPT15_E2E_PREPARE=1")
	}
	ctx := context.Background()
	email := os.Getenv("E2E_PREPARE_EMAIL")
	password := os.Getenv("E2E_PREPARE_PASSWORD")
	if email == "" {
		stamp := time.Now().UnixNano()
		email = fmt.Sprintf("e2e-p15-%d@example.test", stamp)
		password = fmt.Sprintf("e2e-pass-%d", stamp)
	}
	if password == "" {
		t.Fatal("E2E_PREPARE_PASSWORD required when E2E_PREPARE_EMAIL is set")
	}

	pool := prompt81Pool(t)
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,status) VALUES($1,$2,'active') RETURNING id`, email, hash).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `INSERT INTO seller_accounts(user_id,name,status) VALUES($1,$2,'active') RETURNING id`, userID, "E2E Prompt15").Scan(&sellerID); err != nil {
		t.Fatal(err)
	}
	encKey := os.Getenv("ENCRYPTION_KEY")
	if encKey == "" {
		encKey = "01234567890123456789012345678901"
	}
	if err := devseed.SeedOzonConnectionMock(ctx, dbgen.New(pool), sellerID, time.Now().UTC(), encKey); err != nil {
		t.Fatal(err)
	}
	if envSeller := os.Getenv("E2E_SELLER_ID"); envSeller != "" {
		t.Fatalf("E2E_SELLER_ID must not be set for prepare; got %s", envSeller)
	}

	t.Setenv("DEEP_FAKE_ASSESSMENT", "PARTIALLY_SUPPORTED")
	f := prompt91SeedForSeller(t, sellerID)
	prompt15Relabel(t, f)
	exec, _ := prompt132Exec(t, f, "p15-e2e")
	r1 := f.prodRun(t)
	prompt132RunThrough(t, exec, r1.ID, "")
	prompt15PromoteStock300(t, f)
	r2 := f.prodRun(t)
	prompt132RunThrough(t, exec, r2.ID, "")

	p1 := prompt14PubID(t, f, r1.ID)
	p2 := prompt14PubID(t, f, r2.ID)
	skuID := prompt15PolarisID(t, f)
	var p1Universe, p2Universe string
	if err := f.pool.QueryRow(ctx, `SELECT factual_universe_id FROM analysis_publications WHERE id=$1`, p1).Scan(&p1Universe); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT factual_universe_id FROM analysis_publications WHERE id=$1`, p2).Scan(&p2Universe); err != nil {
		t.Fatal(err)
	}

	if got := prompt15DirectSellable(t, f, p1, skuID); got != 0 {
		t.Fatalf("P1 sellable=%v want 0", got)
	}
	if got := prompt15DirectSellable(t, f, p2, skuID); got != 300 {
		t.Fatalf("P2 sellable=%v want 300", got)
	}
	secret := "tttttttttttttttttttttttttttttttt"
	signer, err := factualapi.NewTokenSigner([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	fapi := factual.NewService(f.pool, signer, 0, 0)
	for _, step := range []struct {
		label string
		pub   int64
		stock float64
	}{{"P1", p1, 0}, {"P2", p2, 300}} {
		found, serr := fapi.SearchEntities(ctx, prompt15SealedFactual(t, f, step.pub), factualapi.EntitySearchRequest{
			Query: "Какой остаток у Polaris 417?",
			Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU},
			Limit: 8,
		})
		if serr != nil {
			t.Fatal(serr)
		}
		ok := false
		for _, it := range found.Items {
			if it.CanonicalID == skuID {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("%s search missed Polaris sku=%d items=%+v", step.label, skuID, found.Items)
		}
		t.Logf("E2E PREFLIGHT %s publication=%d search_sku=%d stock=%v", step.label, step.pub, skuID, step.stock)
	}

	histCaseID, histViewID := prompt152InsertPriceCase(t, f, p1, "SKU-P15-E2E-HIST")
	execHist, _ := prompt132Exec(t, f, "p15-e2e-hist")
	for i := 0; i < 4; i++ {
		r := f.prodRun(t)
		prompt132RunThrough(t, execHist, r.ID, "")
	}
	var currentPub int64
	if err := f.pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_publications WHERE seller_account_id=$1`, f.seller).Scan(&currentPub); err != nil {
		t.Fatal(err)
	}
	if currentPub == p1 {
		t.Fatal("expected newer current publication after historical advance runs")
	}

	gid, groupCases := prompt141SeedGroup(t, f, currentPub, "R-P15-E2E-FOUR", 4, 1)
	groupViewID := actionview.EncodeGroup(gid.String())

	singleCaseID, _ := prompt141InsertCase(t, f, currentPub, "p15-e2e-single", 2)
	singleViewID := actionview.EncodeCase(singleCaseID.String())

	av := actionview.NewService(f.repo)
	if err := av.Mutate(ctx, f.seller, nil, histCaseID.String(), "DONE"); err != nil {
		t.Fatal(err)
	}

	active, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "active", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := av.List(ctx, f.seller, actionview.ListFilter{Status: "done", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}

	var groupTitle, singleTitle string
	foundGroup := false
	foundSingle := false
	for i := range active.Groups {
		g := active.Groups[i]
		if g.ActionViewID == groupViewID {
			foundGroup = true
			groupTitle = g.DisplayTitle
			if g.ActiveMemberCount != 4 || len(g.Members) != 4 {
				t.Fatalf("group members=%d active=%d", len(g.Members), g.ActiveMemberCount)
			}
		}
		if g.ActionViewID == singleViewID {
			foundSingle = true
			singleTitle = g.DisplayTitle
			if g.ActiveMemberCount != 1 {
				t.Fatalf("singleton active=%d", g.ActiveMemberCount)
			}
		}
	}
	if !foundGroup {
		t.Fatal("active four-member group missing from actionview list")
	}
	if !foundSingle {
		t.Fatal("active singleton missing from actionview list")
	}

	foundHist := false
	var histTitle string
	for i := range completed.History {
		if completed.History[i].ActionViewID == histViewID {
			foundHist = true
			histTitle = completed.History[i].DisplayTitle
			break
		}
	}
	if !foundHist {
		t.Fatal("completed historical action missing from actionview history")
	}
	if len(active.Groups) < 2 {
		t.Fatalf("expected at least group+singleton active, got %d groups", len(active.Groups))
	}

	manifest := prompt15E2EManifest{
		SellerAccountID:        f.seller,
		Email:                  email,
		Password:               password,
		P1PublicationID:        p1,
		P2PublicationID:        p2,
		P1UniverseID:           p1Universe,
		P2UniverseID:           p2Universe,
		CurrentPublicationID:   currentPub,
		SKU417CanonicalID:      skuID,
		SKU417OzonSKU:          9105,
		GroupID:                gid.String(),
		GroupActionViewID:      groupViewID,
		GroupTitle:             groupTitle,
		GroupCaseIDs:           groupCases,
		SingletonCaseID:        singleCaseID.String(),
		SingletonActionViewID:  singleViewID,
		SingletonTitle:         singleTitle,
		HistoricalCaseID:       histCaseID.String(),
		HistoricalActionViewID: histViewID,
		HistoricalTitle:        histTitle,
		HistoricalOriginPubID:  p1,
		HistoricalCurrentPubID: currentPub,
		ActiveActionCount:      len(active.Groups),
		CompletedActionCount:   len(completed.History),
	}

	outDir := os.Getenv("E2E_PREPARE_OUTPUT_DIR")
	if outDir == "" {
		outDir = filepath.Join("..", "frontend", "e2e")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(outDir, ".prompt15-e2e-manifest.json")
	credPath := filepath.Join(outDir, ".e2e-credentials.json")
	sellerPath := filepath.Join(outDir, ".seller-id")

	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	creds, _ := json.MarshalIndent(map[string]any{
		"email": email, "password": password, "sellerId": f.seller,
	}, "", "  ")
	if err := os.WriteFile(credPath, creds, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sellerPath, []byte(fmt.Sprintf("%d", f.seller)), 0o644); err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(b))
	if manifest.SellerAccountID <= 0 || manifest.GroupActionViewID == "" || manifest.SingletonActionViewID == "" || manifest.HistoricalActionViewID == "" {
		t.Fatal("manifest incomplete")
	}
}
