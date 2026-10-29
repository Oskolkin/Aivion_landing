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
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt95ProductionContentfulE2E(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	f := prompt93Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt95-exec", nil)
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
	if core.Context.LandscapeSchemaVersion != landscape.LandscapeSchemaV5 {
		t.Fatalf("core schema=%s want %s", core.Context.LandscapeSchemaVersion, landscape.LandscapeSchemaV5)
	}
	if core.Context.LandscapeProfileVersion != landscape.CoreProfileVersionSealV5() {
		t.Fatalf("core profile seal mismatch")
	}

	acct := core.Account
	required := []struct {
		name string
		ok   bool
	}{
		{"sales", acct.Sales != nil},
		{"assortment", acct.Assortment != nil},
		{"stock", acct.Stock != nil},
		{"price", acct.Price != nil},
		{"advertising", acct.Advertising != nil},
		{"concentration", acct.Concentration != nil},
	}
	for _, block := range required {
		if !block.ok {
			t.Errorf("account.%s missing in V3 core", block.name)
		}
	}
	capReady := func(key string) bool {
		for _, c := range core.GlobalCapabilities {
			if c.Key == key && (c.FinalState == "READY" || c.FinalState == "PARTIAL") {
				return true
			}
		}
		return false
	}
	if capReady("FUNNEL.ACCOUNT") && acct.Funnel == nil {
		t.Error("account.funnel missing while FUNNEL.ACCOUNT READY")
	}
	if capReady("FINANCE.ACCOUNT_NET") && acct.Finance == nil {
		t.Error("account.finance missing while FINANCE.ACCOUNT_NET READY")
	}
	if capReady("PROMOTIONS.DEFINITIONS") && acct.Promotions == nil {
		t.Error("account.promotions missing while PROMOTIONS.DEFINITIONS READY")
	}
	if capReady("HEALTH.CURRENT_RATINGS") && acct.Health == nil {
		t.Error("account.health missing while HEALTH.CURRENT_RATINGS READY")
	}
	if capReady("SUPPLY.STATE") && acct.Supply == nil {
		t.Error("account.supply missing while SUPPLY.STATE READY")
	}
	if capReady("SEARCH.SELLER_SUMMARY") && acct.Search == nil {
		t.Error("account.search missing while SEARCH.SELLER_SUMMARY READY")
	}

	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Context.LandscapeProfileVersion != landscape.DetailProfileVersionSealV5() {
		t.Fatalf("detail profile seal mismatch")
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

	bundle, err := contracts.Current()
	if err != nil {
		t.Fatal(err)
	}
	if err = analysis.ValidatePreDiscoveryFoundation(analysis.PreDiscoveryInput{
		SellerAccountID:           f.seller,
		UniverseID:                core.Context.FactualUniverseID,
		UniverseHash:              core.Context.FactualUniverseHash,
		AnalysisDate:              core.Context.AnalysisDate,
		CutoffAt:                  core.Context.AnalysisCutoffAt,
		ContractBundleHash:        bundle.Hash(),
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
		t.Fatalf("pre-discovery: %v", err)
	}

	for _, c := range detail.Campaigns {
		if c.Performance == nil {
			continue
		}
		if c.Performance.Traffic == nil && c.Performance.Attribution == nil {
			t.Errorf("campaign %d performance missing traffic/attribution subblocks", c.Identity.CampaignID)
		}
		if c.Exposure != nil && c.Exposure.State == landscape.BlockReadyEmpty && c.Performance.Traffic != nil {
			if c.Performance.Traffic.PaidSpend.Current != nil {
				t.Errorf("campaign %d exposure READY_EMPTY with known traffic spend", c.Identity.CampaignID)
			}
		}
	}
}
