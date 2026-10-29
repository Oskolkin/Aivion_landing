//go:build integration

package analysis_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt94AccountV3DerivedBlocksAndPreDiscovery(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	f := prompt93Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt94-exec", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			t.Fatalf("stage %s: %v", key, err)
		}
	}

	coreBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeCore))
	var core landscape.LandscapeSet
	if err = json.Unmarshal(coreBody, &core); err != nil {
		t.Fatal(err)
	}
	acct := core.Account
	if acct.Sales == nil || acct.Sales.State == "" {
		t.Fatal("account.sales missing")
	}
	if acct.Sales.AnalysisDayAmount == nil || acct.Sales.AnalysisDayUnits == nil {
		t.Error("account.sales analysis_day comparisons expected")
	}
	if acct.Advertising == nil {
		t.Fatal("account.advertising missing")
	}
	if acct.Advertising.Traffic == nil && acct.Advertising.Attribution == nil {
		t.Error("account.advertising should aggregate campaign traffic/attribution facts")
	}
	if acct.Advertising.Traffic != nil && acct.Advertising.Traffic.Impressions.Current == nil && acct.Advertising.Traffic.PaidSpend.Current == nil {
		t.Error("account.advertising.traffic should aggregate campaign facts")
	}
	if acct.Stock == nil || acct.Stock.TotalSellableUnits == nil {
		t.Error("account.stock derived totals expected")
	}
	if acct.Concentration == nil {
		t.Fatal("account.concentration missing")
	}
	if acct.Concentration.State == landscape.BlockNotApplicable {
		t.Error("account.concentration must be derived, not NOT_APPLICABLE")
	}

	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Context.MaterialitySelectionHash == "" {
		t.Error("detail context missing materiality selection hash")
	}

	rankBody := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.MaterialityRanking))
	var ranking materiality.RankingArtifact
	if err = json.Unmarshal(rankBody, &ranking); err != nil {
		t.Fatal(err)
	}
	selBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.RunMateriality))
	var selection materiality.SelectionArtifact
	if err = json.Unmarshal(selBody, &selection); err != nil {
		t.Fatal(err)
	}

	coreStage, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeCore))
	if err != nil {
		t.Fatal(err)
	}
	detailStage, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeDetail))
	if err != nil {
		t.Fatal(err)
	}
	if err = analysis.ValidatePreDiscoveryFoundation(analysis.PreDiscoveryInput{
		SellerAccountID:           f.seller,
		UniverseID:                core.Context.FactualUniverseID,
		UniverseHash:              core.Context.FactualUniverseHash,
		AnalysisDate:              core.Context.AnalysisDate,
		CutoffAt:                  core.Context.AnalysisCutoffAt,
		ContractBundleHash:        core.Context.ContractBundleHash,
		ExpectedCoreSchema:        landscape.LandscapeSchemaV5,
		ExpectedCoreProfileSeal:   landscape.CoreProfileVersionSealV5(),
		ExpectedDetailSchema:      landscape.LandscapeSchemaV5,
		ExpectedDetailProfileSeal: landscape.DetailProfileVersionSealV5(),
		Core:                      core,
		CoreHash:                  coreStage.OutputHash,
		Ranking:                   ranking,
		Selection:                 selection,
		Detail:                    detail,
		DetailHash:                detailStage.OutputHash,
	}); err != nil {
		t.Fatalf("pre-discovery V3: %v", err)
	}
}
