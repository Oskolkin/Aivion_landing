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

func TestPrompt93AllActiveLanesAndPreDiscovery(t *testing.T) {
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		t.Fatal("set TEST_DATABASE_URL")
	}
	f := prompt93Seed(t)
	ctx := context.Background()
	run := f.evalRun(t)
	store := artifacts.NewMemoryStore()
	exec, err := runtime.NewProductionExecutor(f.pool, store, "prompt93-exec", nil)
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
	detailBody := prompt91LoadStageBody(t, f, store, run.ID, string(stages.BuildLandscapeDetail))
	var detail landscape.LandscapeSet
	if err = json.Unmarshal(detailBody, &detail); err != nil {
		t.Fatal(err)
	}

	manifest := materiality.ActiveLaneManifest()
	for _, lv := range manifest {
		found := false
		switch lv.EntityKind {
		case materiality.EntitySKU:
			for _, e := range ranking.SKUs {
				for _, m := range e.LaneMemberships {
					if m.LaneKey == lv.LaneKey {
						found = true
						break
					}
				}
			}
		case materiality.EntityCategory:
			for _, e := range ranking.Categories {
				for _, m := range e.LaneMemberships {
					if m.LaneKey == lv.LaneKey {
						found = true
						break
					}
				}
			}
		case materiality.EntityCampaign:
			for _, e := range ranking.Campaigns {
				for _, m := range e.LaneMemberships {
					if m.LaneKey == lv.LaneKey {
						found = true
						break
					}
				}
			}
		}
		if !found {
			t.Errorf("active lane not exercised in ranking: %s (%s)", lv.LaneKey, lv.EntityKind)
		}
	}

	coreStage, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeCore))
	if err != nil {
		t.Fatal(err)
	}
	detailStage, err := f.repo.GetStage(ctx, run.ID, string(stages.BuildLandscapeDetail))
	if err != nil {
		t.Fatal(err)
	}
	if err := analysis.ValidatePreDiscoveryFoundation(analysis.PreDiscoveryInput{
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
		t.Fatal(err)
	}
}

func prompt91StageOutputHash(t *testing.T, f prompt81Fixture, runID int64, stage string) string {
	t.Helper()
	st, err := f.repo.GetStage(context.Background(), runID, stage)
	if err != nil {
		t.Fatal(err)
	}
	return st.OutputHash
}
