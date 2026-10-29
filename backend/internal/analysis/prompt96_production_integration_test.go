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

func TestPrompt96ProductionContentfulE2EV4(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("set TEST_DATABASE_URL")
	}
	if strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_HMAC_SECRET")) == "" {
		t.Setenv("FACTUAL_TOKEN_HMAC_SECRET", strings.Repeat("t", 32))
	}
	f := prompt93Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt96-exec", nil)
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
	for _, block := range []struct {
		name string
		ok   bool
	}{
		{"sales", acct.Sales != nil},
		{"assortment", acct.Assortment != nil},
		{"stock", acct.Stock != nil},
		{"price", acct.Price != nil},
		{"advertising", acct.Advertising != nil},
		{"concentration", acct.Concentration != nil},
	} {
		if !block.ok {
			t.Errorf("account.%s missing in V4 core", block.name)
		}
	}
	if acct.Advertising != nil && acct.Advertising.Traffic == nil && acct.Advertising.Attribution == nil {
		t.Error("account advertising missing traffic/attribution split")
	}

	var trafficWin, attrWin *landscape.DateRange
	for i := range core.DomainWindows {
		w := core.DomainWindows[i]
		if w.Domain == "advertising" && w.FactFamily == "ADVERTISING.TRAFFIC" {
			trafficWin = w.Current
		}
		if w.Domain == "advertising" && w.FactFamily == "ADVERTISING.ATTRIBUTION" {
			attrWin = w.Current
		}
	}
	if trafficWin == nil || attrWin == nil {
		t.Fatalf("advertising domain windows missing traffic=%v attr=%v", trafficWin, attrWin)
	}

	for _, c := range core.Campaigns {
		if c.Performance == nil {
			continue
		}
		if c.Performance.Traffic != nil && c.Performance.Traffic.CurrentPeriod != nil && trafficWin != nil {
			if c.Performance.Traffic.CurrentPeriod.From != trafficWin.From || c.Performance.Traffic.CurrentPeriod.To != trafficWin.To {
				t.Errorf("campaign %d traffic period mixed with domain window", c.Identity.CampaignID)
			}
		}
		if c.Performance.Attribution != nil && c.Performance.Attribution.CurrentPeriod != nil && attrWin != nil {
			if c.Performance.Attribution.CurrentPeriod.From != attrWin.From || c.Performance.Attribution.CurrentPeriod.To != attrWin.To {
				t.Errorf("campaign %d attribution period mixed with domain window", c.Identity.CampaignID)
			}
		}
	}

	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Context.LandscapeSchemaVersion != landscape.LandscapeSchemaV5 {
		t.Fatalf("detail schema=%s", detail.Context.LandscapeSchemaVersion)
	}
	if detail.Context.LandscapeProfileVersion != landscape.DetailProfileVersionSealV5() {
		t.Fatalf("detail profile seal mismatch")
	}

	for _, s := range detail.SKUs {
		if s.QueryContext != nil && (s.QueryContext.State == landscape.BlockReady || s.QueryContext.State == landscape.BlockPartial) {
			if s.QueryContext.Period == nil || s.QueryContext.AsOf != nil {
				t.Errorf("sku %d query_context provenance", s.Identity.CanonicalSKUID)
			}
		}
		if s.CampaignContext != nil && (s.CampaignContext.State == landscape.BlockReady || s.CampaignContext.State == landscape.BlockPartial) {
			if s.CampaignContext.Period == nil {
				t.Errorf("sku %d campaign_context period missing", s.Identity.CanonicalSKUID)
			}
		}
		if s.PromotionContext != nil && (s.PromotionContext.State == landscape.BlockReady || s.PromotionContext.State == landscape.BlockPartial) {
			if s.PromotionContext.AsOf == nil || s.PromotionContext.Period != nil {
				t.Errorf("sku %d promotion_context provenance", s.Identity.CanonicalSKUID)
			}
		}
		if s.Funnel != nil && s.Funnel.State == landscape.BlockReady {
			if s.Funnel.SearchSurface == nil && s.Funnel.PDPSurface == nil {
				t.Errorf("sku %d funnel READY without surfaces", s.Identity.CanonicalSKUID)
			}
		}
	}
	for _, c := range detail.Campaigns {
		if c.SKUContext != nil && (c.SKUContext.State == landscape.BlockReady || c.SKUContext.State == landscape.BlockPartial) {
			if c.SKUContext.WindowRef != "CAMPAIGN_TARGETS_AS_OF" || c.SKUContext.AsOf == nil {
				t.Errorf("campaign %d sku_context provenance", c.Identity.CampaignID)
			}
		}
	}
	for _, c := range detail.Categories {
		if c.Funnel != nil && c.Funnel.State == landscape.BlockReady && c.Funnel.PDPSurface == nil && c.Funnel.SearchSurface == nil {
			t.Errorf("category %d funnel READY without facts", c.Identity.CategoryID)
		}
	}

	rankBody := prompt91LoadArtifactKind(t, f, store, run.ID, string(artifacts.MaterialityRanking))
	var ranking materiality.RankingArtifact
	if err = json.Unmarshal(rankBody, &ranking); err != nil {
		t.Fatal(err)
	}
	if ranking.PolicyVersion != materiality.MaterialityPolicyV1_4 {
		t.Fatalf("ranking policy=%s", ranking.PolicyVersion)
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

	active := materiality.ActiveLaneManifest()
	if len(active) == 0 {
		t.Fatal("active lane manifest empty")
	}
	for _, lane := range active {
		if strings.Contains(lane.ProfileField, "performance.") {
			t.Errorf("V3 performance path still in lane %s: %s", lane.LaneKey, lane.ProfileField)
		}
	}

	list, err := f.repo.ListStages(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.Key == string(stages.PlanActionReconciliation) && s.Status != "SUCCEEDED" {
			t.Fatalf("stage 11 must succeed; status=%s", s.Status)
		}
	}
}
