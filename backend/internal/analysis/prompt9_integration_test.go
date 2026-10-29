//go:build integration

package analysis_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestPrompt9ContinuityNoBase(t *testing.T) {
	body, hash, err := continuity.Prepare(nil)
	if err != nil || len(hash) != 64 || len(body) == 0 {
		t.Fatalf("no-base prepare failed: err=%v hash=%q", err, hash)
	}
	id := int64(42)
	if _, _, err = continuity.Prepare(&id); err == nil || !strings.Contains(err.Error(), continuity.ErrReaderNotImplemented) {
		t.Fatalf("expected CONTINUITY_READER_NOT_IMPLEMENTED, got %v", err)
	}
}

func TestPrompt9LandscapeMaterialityMemPath(t *testing.T) {
	sealed, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 99,
		AnalysisDate:    time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
		CutoffAt:        time.Date(2026, 8, 20, 2, 40, 0, 0, time.UTC),
		Policies:        factualapi.CurrentPolicies(),
	}, prompt9Seal{id: "u1", hash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	catID := int64(11)
	feed := landscape.CoreFeed{
		Account: landscape.AccountLandscape{
			Profile:  landscape.ProfileAccountDiscoveryV1,
			Identity: landscape.AccountIdentity{SellerAccountID: 99},
			Sales: &landscape.AccountSalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current:     &landscape.MoneyAmount{Amount: "5000", CurrencyCode: "RUB"},
					Previous:    &landscape.MoneyAmount{Amount: "4000", CurrencyCode: "RUB"},
					ChangeState: landscape.ChangeIncreased,
				},
			},
		},
		GlobalCapabilities: []landscape.CapabilityFlag{
			{Key: "SALES.SKU_DEMAND", FinalState: materiality.ReadinessReady},
			{Key: "SALES.CATEGORY_DEMAND", FinalState: materiality.ReadinessReady},
			{Key: "SALES.ACCOUNT_DEMAND", FinalState: materiality.ReadinessReady},
			{Key: "STOCK.SKU_CURRENT", FinalState: materiality.ReadinessReady},
			{Key: "STOCK.COVERAGE_7D", FinalState: materiality.ReadinessReady},
			{Key: "PRICE.CUSTOMER_PRICE", FinalState: materiality.ReadinessReady},
			{Key: "SEARCH.SELLER_SUMMARY", FinalState: materiality.ReadinessReady},
			{Key: "SUPPLY.STATE", FinalState: materiality.ReadinessReady},
		},
		Categories: []landscape.CategoryLandscape{{
			Profile:  landscape.ProfileCategoryCoreV1,
			Identity: landscape.CategoryIdentity{CategoryID: 11, IsLeaf: true, Name: "Leaf"},
			Sales: &landscape.CategorySalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current: &landscape.MoneyAmount{Amount: "3000", CurrencyCode: "RUB"}, ChangeState: landscape.ChangeNoBase,
				},
			},
		}},
		SKUs: []landscape.SKULandscape{{
			Profile:  landscape.ProfileSKUCoreV1,
			Identity: landscape.SKUIdentity{CanonicalSKUID: 1001, CategoryID: &catID},
			Sales: &landscape.SKUSalesBlock{
				State: landscape.BlockReady,
				OrderedAmount: landscape.MoneyComparison{
					Current: &landscape.MoneyAmount{Amount: "2000", CurrencyCode: "RUB"}, ChangeState: landscape.ChangeNoBase,
				},
			},
			Stock: &landscape.SKUStockBlock{
				State: landscape.BlockReadyEmpty,
				Limitations: []landscape.Limitation{{
					Code: "STOCK_COVERAGE_MISSING", Message: "mem fixture: coverage not seeded",
				}},
			},
			Price: &landscape.SKUPriceBlock{
				State: landscape.BlockReadyEmpty,
			},
			Search: &landscape.SKUSearchBlock{State: landscape.BlockReadyEmpty},
		}},
	}
	core, _, _, err := landscape.CoreBuilder{Source: landscape.MemSource{Feed: feed}}.BuildCore(
		context.Background(), sealed, "u1", strings.Repeat("a", 64), strings.Repeat("b", 64),
	)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := materiality.CoreFactsFromLandscape(core)
	if err != nil {
		t.Fatal(err)
	}
	ranking := materiality.BuildMaterialityRanking(facts, materiality.CurrentPolicy(), "")
	sel := materiality.SelectMaterialEntities(ranking, materiality.DiscoveryStandardBudget(), facts.Account)
	if ranking.RankingHash == "" || sel.SelectionHash == "" || ranking.RankingHash == sel.SelectionHash {
		t.Fatalf("expected distinct ranking/selection hashes: r=%s s=%s", ranking.RankingHash, sel.SelectionHash)
	}
	detail, _, _, err := landscape.DetailBuilder{}.BuildDetail(context.Background(), sealed, landscape.DetailBuildInput{
		Core: core, Selected: materiality.SelectionEntityRefs(sel), SelectionHash: sel.SelectionHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.Account.Identity.SellerAccountID != 99 || len(detail.SKUs) == 0 {
		t.Fatalf("detail projection unexpected: skus=%d", len(detail.SKUs))
	}
	raw, _, err := landscape.MarshalAndHash(detail)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"fusion_utility", "lane_rank", "materiality_score", "selection_reason", "diversity_penalty"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("Detail leaked materiality audit field %q", banned)
		}
	}
}

func TestPrompt9Stages1to6Frontier(t *testing.T) {
	f := prompt91Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt9-exec", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
		st, err := f.repo.GetStage(ctx, run.ID, key)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != "SUCCEEDED" && st.Status != "SUCCEEDED_WITH_LIMITATIONS" {
			t.Fatalf("stage %s status=%s err=%s", key, st.Status, st.ErrorMessage)
		}
	}
	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status == "READY" {
			t.Fatal("stage 11 BUILD_DEEP_PACK must not become READY")
		}
	}
	var frontier bool
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM analysis_run_events WHERE run_id=$1 AND event_type='IMPLEMENTED_FRONTIER_REACHED')`, run.ID).Scan(&frontier)
	if err != nil || !frontier {
		t.Fatalf("expected IMPLEMENTED_FRONTIER_REACHED event: err=%v found=%v", err, frontier)
	}
	err = f.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM analysis_run_events WHERE run_id=$1 AND event_type='IMPLEMENTED_FRONTIER_REACHED' 		AND payload_json->>'last_stage'='POST_PUBLISH')`, run.ID).Scan(&frontier)
	if err != nil || !frontier {
		t.Fatal("frontier last_stage must be POST_PUBLISH")
	}
}

type prompt9Seal struct{ id, hash string }

func (s prompt9Seal) SealedBindingHash() string { return s.hash }
func (s prompt9Seal) SealedUniverseID() string  { return s.id }
