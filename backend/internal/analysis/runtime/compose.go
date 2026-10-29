package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewProductionExecutor wires Prompt №12 stages 1–13 for worker/devtools.
func NewProductionExecutor(pool *pgxpool.Pool, store artifacts.ArtifactStore, workerID string, enqueue EnqueueFunc) (*Executor, error) {
	repo := analysispg.New(pool)
	if _, err := contracts.Current(); err != nil {
		return nil, err
	}
	resolver := resolve.Resolver{Pool: pool}
	sealer := universe.Sealer{Repo: repo, Store: store}
	secret, err := factualTokenHMACSecret()
	if err != nil {
		return nil, err
	}
	signer, err := factualapi.NewTokenSigner(secret)
	if err != nil {
		return nil, err
	}
	factualAPI := factual.NewService(pool, signer, 0, 0)
	coreSrc := landscape.FactualAPISource{API: factualAPI}
	coreBuilder := landscape.CoreBuilder{Source: coreSrc, Profiles: landscape.DefaultCoreProfilesV5()}
	detailBuilder := landscape.DetailBuilder{Source: landscape.FactualAPIDetailSource{API: factualAPI}}

	loadStageBody := func(ctx context.Context, runID int64, stageKey string) ([]byte, string, error) {
		stage, err := repo.GetStage(ctx, runID, stageKey)
		if err != nil {
			return nil, "", err
		}
		if stage.Status != "SUCCEEDED" && stage.Status != "SUCCEEDED_WITH_LIMITATIONS" {
			return nil, "", fmt.Errorf("runtime: %s requires accepted prior stage %s", stageKey, stageKey)
		}
		if stage.OutputHash == "" {
			return nil, "", fmt.Errorf("runtime: %s output missing", stageKey)
		}
		key, err := repo.ArtifactStorageKey(ctx, runID, stage.OutputHash)
		if err != nil {
			return nil, "", err
		}
		body, err := store.Get(ctx, key)
		return body, stage.OutputHash, err
	}

	loadSealed := func(ctx context.Context, run analysispg.Run) (factualapi.FactualReadContext, universe.Binding, error) {
		if run.UniverseID == "" {
			return factualapi.FactualReadContext{}, universe.Binding{}, fmt.Errorf("runtime: factual universe not bound")
		}
		binding, err := sealer.LoadValidated(ctx, run.UniverseID, run.SellerID)
		if err != nil {
			return factualapi.FactualReadContext{}, universe.Binding{}, err
		}
		sealed, err := universe.NewSealedContext(binding.Record, binding)
		return sealed, binding, err
	}

	resolveH := ResolveContextHandler{Resolve: func(ctx context.Context, w Work) ([]byte, error) {
		run, err := repo.GetRun(ctx, w.RunID)
		if err != nil {
			return nil, err
		}
		_, body, _, err := resolver.Resolve(ctx, resolve.Input{
			SellerID: run.SellerID, AnalysisDate: run.AnalysisDate, CutoffAt: run.CutoffAt,
			RunKind: run.Kind, IntentHash: run.IntentHash, ContractBundleHash: run.ContractHash,
			Policies: factualapi.CurrentPolicies(),
		})
		return body, err
	}}
	sealH := SealUniverseHandler{Seal: func(ctx context.Context, w Work) ([]byte, error) {
		run, err := repo.GetRun(ctx, w.RunID)
		if err != nil {
			return nil, err
		}
		stage, err := repo.GetStage(ctx, w.RunID, string(stages.ResolveContext))
		if err != nil {
			return nil, err
		}
		if stage.Status != "SUCCEEDED" && stage.Status != "SUCCEEDED_WITH_LIMITATIONS" {
			return nil, fmt.Errorf("runtime: SEAL_UNIVERSE requires accepted RESOLVE_CONTEXT")
		}
		if stage.OutputHash == "" {
			return nil, fmt.Errorf("runtime: RESOLVE_CONTEXT output missing")
		}
		key, err := repo.ArtifactStorageKey(ctx, w.RunID, stage.OutputHash)
		if err != nil {
			return nil, err
		}
		body, err := store.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		var resolved resolve.Manifest
		if err = json.Unmarshal(body, &resolved); err != nil {
			return nil, err
		}
		readCtx, err := universe.ReadContextFromResolved(resolved)
		if err != nil {
			return nil, err
		}
		entitySets, err := universe.BuildEntitySets(ctx, factualAPI, readCtx)
		if err != nil {
			return nil, err
		}
		stageID := w.StageID
		binding, err := sealer.Seal(ctx, universe.Input{
			SellerID: run.SellerID, RunID: run.ID, StageID: &stageID,
			ResolvedContextHash: stage.OutputHash, Resolved: &resolved, EntitySets: entitySets,
		})
		if err != nil {
			return nil, err
		}
		if err = repo.BindUniverse(ctx, run.ID, binding.ID, binding.Hash); err != nil {
			return nil, err
		}
		return binding.Record.ManifestJSON, nil
	}}
	continuityH := ContinuityHandler{Prepare: func(ctx context.Context, w Work) ([]byte, error) {
		run, err := repo.GetRun(ctx, w.RunID)
		if err != nil {
			return nil, err
		}
		body, _, err := continuity.PrepareWithReader(run.SellerID, run.BasePublicationID, repo)
		return body, err
	}}
	landscapeCoreH := LandscapeCoreHandler{Build: func(ctx context.Context, w Work) ([]byte, error) {
		run, err := repo.GetRun(ctx, w.RunID)
		if err != nil {
			return nil, err
		}
		sealed, binding, err := loadSealed(ctx, run)
		if err != nil {
			return nil, err
		}
		_, body, _, err := coreBuilder.BuildCore(ctx, sealed, binding.ID, binding.Hash, run.ContractHash)
		return body, err
	}}
	materialityH := MaterialityHandler{Run: func(ctx context.Context, w Work) ([]byte, error) {
		run, err := repo.GetRun(ctx, w.RunID)
		if err != nil {
			return nil, err
		}
		coreBody, coreHash, err := loadStageBody(ctx, w.RunID, string(stages.BuildLandscapeCore))
		if err != nil {
			return nil, err
		}
		var coreSet landscape.LandscapeSet
		if err = json.Unmarshal(coreBody, &coreSet); err != nil {
			return nil, err
		}
		policyV := materiality.CurrentPolicy()
		budget := materiality.DiscoveryStandardBudget()
		if pv, ph, pj, e := materiality.PolicySeed(); e == nil {
			if bv, bh, bj, e2 := materiality.BudgetSeed(); e2 == nil {
				_ = repo.UpsertMaterialityPolicySeeds(ctx, pv, ph, pj, bv, bh, bj)
			}
		}
		facts, err := materiality.CoreFactsFromLandscape(coreSet)
		if err != nil {
			return nil, err
		}
		ranking := materiality.BuildMaterialityRanking(facts, policyV, coreHash)
		selection := materiality.SelectMaterialEntities(ranking, budget, facts.Account)

		rankArt, err := materiality.NewRankingArtifact(ranking)
		if err != nil {
			return nil, err
		}
		rankBody, err := canonicaljson.Marshal(rankArt)
		if err != nil {
			return nil, err
		}
		var rankingArtifactID *int64
		stageID := w.StageID
		if a, putErr := artifacts.PutBeforeState(ctx, store, artifacts.Location{
			SellerID: run.SellerID, RunID: run.ID, Stage: string(stages.RunMateriality), Kind: artifacts.MaterialityRanking,
		}, rankBody, func(a artifacts.Artifact) error {
			id, saveErr := repo.AddArtifact(ctx, run.ID, &stageID, string(artifacts.MaterialityRanking), a.Key, a.Hash, a.Size)
			if saveErr == nil {
				rankingArtifactID = &id
			}
			return saveErr
		}); putErr != nil {
			return nil, putErr
		} else {
			_ = a
		}

		selArt := materiality.NewSelectionArtifact(selection)
		selBody, err := canonicaljson.Marshal(selArt)
		if err != nil {
			return nil, err
		}
		var selectionArtifactID *int64
		if a, putErr := artifacts.PutBeforeState(ctx, store, artifacts.Location{
			SellerID: run.SellerID, RunID: run.ID, Stage: string(stages.RunMateriality), Kind: artifacts.MaterialitySelection,
		}, selBody, func(a artifacts.Artifact) error {
			id, saveErr := repo.AddArtifact(ctx, run.ID, &stageID, string(artifacts.MaterialitySelection), a.Key, a.Hash, a.Size)
			if saveErr == nil {
				selectionArtifactID = &id
			}
			return saveErr
		}); putErr != nil {
			return nil, putErr
		} else {
			_ = a
		}

		sealStage, err := repo.GetStage(ctx, w.RunID, string(stages.SealUniverse))
		if err != nil {
			return nil, err
		}
		matRunID, err := repo.PutMaterialityRun(ctx, analysispg.MaterialityRunInput{
			AnalysisRunID: run.ID, SellerID: run.SellerID, UniverseID: run.UniverseID,
			UniverseManifestHash: sealStage.OutputHash, LandscapeCoreProfile: coreSet.Context.LandscapeProfileVersion,
			InputLandscapeCoreHash: coreHash, PolicyVersion: policyV.Version, PolicyHash: policyV.Hash(),
			BudgetVersion: budget.Version, BudgetHash: budget.Hash(),
			RankingHash: ranking.RankingHash, SelectionHash: selection.SelectionHash,
			RankingArtifactID: rankingArtifactID, SelectionArtifactID: selectionArtifactID,
		})
		if err != nil {
			return nil, err
		}
		rows := materialitySelectionRows(ranking, selection)
		if err = repo.PutMaterialitySelections(ctx, matRunID, rows); err != nil {
			return nil, err
		}
		return selBody, nil
	}}
	landscapeDetailH := LandscapeDetailHandler{Build: func(ctx context.Context, w Work) ([]byte, error) {
		run, err := repo.GetRun(ctx, w.RunID)
		if err != nil {
			return nil, err
		}
		sealed, _, err := loadSealed(ctx, run)
		if err != nil {
			return nil, err
		}
		coreBody, _, err := loadStageBody(ctx, w.RunID, string(stages.BuildLandscapeCore))
		if err != nil {
			return nil, err
		}
		var coreSet landscape.LandscapeSet
		if err = json.Unmarshal(coreBody, &coreSet); err != nil {
			return nil, err
		}
		selBody, _, err := loadStageBody(ctx, w.RunID, string(stages.RunMateriality))
		if err != nil {
			return nil, err
		}
		var selArt materiality.SelectionArtifact
		if err = json.Unmarshal(selBody, &selArt); err != nil {
			return nil, err
		}
		sel := materiality.Selection{
			Account: selArt.Account, Categories: selArt.Categories, SKUs: selArt.SKUs, Campaigns: selArt.Campaigns,
			RankingHash: selArt.RankingHash, BudgetVersion: selArt.BudgetVersion, BudgetHash: selArt.BudgetHash,
			SelectionHash: selArt.SelectionHash, PolicyVersion: selArt.PolicyVersion, PolicyHash: selArt.PolicyHash,
		}
		_, body, _, err := detailBuilder.BuildDetail(ctx, sealed, landscape.DetailBuildInput{
			Core: coreSet, Selected: materiality.SelectionEntityRefs(sel), SelectionHash: sel.SelectionHash,
		})
		if err != nil {
			return nil, err
		}
		return body, nil
	}}

	provider, err := NewDiscoveryProviderFromEnv()
	if err != nil {
		return nil, err
	}
	deepProvider, err := NewDeepProviderFromEnv()
	if err != nil {
		return nil, err
	}
	discDeps := DiscoveryDeps{
		Repo: repo, Store: store, API: factualAPI, Provider: provider, DeepProvider: deepProvider,
		LoadBody: loadStageBody,
		Sealed: func(ctx context.Context, run analysispg.Run) (factualapi.FactualReadContext, error) {
			sealed, _, err := loadSealed(ctx, run)
			return sealed, err
		},
	}
	pol := policy.Current(nil, nil)
	if v := strings.TrimSpace(os.Getenv("OPENAI_DISCOVERY_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if pol.Timeouts == nil {
				pol.Timeouts = map[string]time.Duration{}
			}
			pol.Timeouts["ai_default"] = time.Duration(n) * time.Second
		}
	}
	if v := strings.TrimSpace(os.Getenv("OPENAI_DEEP_TIMEOUT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if pol.Timeouts == nil {
				pol.Timeouts = map[string]time.Duration{}
			}
			pol.Timeouts["ai_deep"] = time.Duration(n) * time.Second
		}
	}

	return &Executor{
		Repo: repo, Store: store, Policy: pol, Registry: stages.ProductionRegistry,
		Handlers: ProductionHandlers(map[stages.Name]Handler{
			stages.ResolveContext:           resolveH,
			stages.SealUniverse:             sealH,
			stages.PrepareContinuity:        continuityH,
			stages.BuildLandscapeCore:       landscapeCoreH,
			stages.RunMateriality:           materialityH,
			stages.BuildLandscapeDetail:     landscapeDetailH,
			stages.BuildDiscoveryPack:       DiscoveryPackHandler(discDeps),
			stages.DiscoveryAI:              DiscoveryAIHandler(discDeps),
			stages.ValidateDiscovery:        ValidateDiscoveryHandler(discDeps),
			stages.ExpandEvidence:           ExpandEvidenceHandler(discDeps),
			stages.BuildDeepPack:            DeepPackHandler(discDeps),
			stages.DeepAI:                   DeepAIHandler(discDeps),
			stages.ValidateDeep:             ValidateDeepHandler(discDeps),
			stages.PlanActionReconciliation: PlanActionReconciliationHandler(discDeps),
			stages.BuildPublication:         BuildPublicationHandler(discDeps),
			stages.BuildProjections:         BuildProjectionsHandler(discDeps),
			stages.ValidatePublication:      ValidatePublicationHandler(discDeps),
			stages.Publish:                  PublishHandler(discDeps),
			stages.PostPublish:              PostPublishHandler(discDeps),
		}),
		WorkerID: workerID, Enqueue: enqueue,
		NextStages: stages.ProductionImplementedStageNames(),
	}, nil
}

// factualTokenHMACSecret loads the production Factual page-token / FactRef HMAC key.
// Prefer FACTUAL_TOKEN_HMAC_SECRET; fall back to FACTUAL_TOKEN_SIGNING_KEY (devtools).
// No hard-coded production default — missing/short secret fails configuration.
func factualTokenHMACSecret() ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_HMAC_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_SIGNING_KEY"))
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("runtime: FACTUAL_TOKEN_HMAC_SECRET or FACTUAL_TOKEN_SIGNING_KEY must be set (len>=32)")
	}
	return []byte(secret), nil
}

func materialitySelectionRows(ranking materiality.Ranking, sel materiality.Selection) []analysispg.MaterialitySelectionRow {
	add := func(out []analysispg.MaterialitySelectionRow, e materiality.SelectedEntity) []analysispg.MaterialitySelectionRow {
		ex := materiality.ExplainSelection(ranking, sel, e.EntityKind, e.SemanticKey)
		summary, _ := json.Marshal(ex)
		return append(out, analysispg.MaterialitySelectionRow{
			EntityKind: string(e.EntityKind), SemanticKey: e.SemanticKey, CanonicalRefID: e.CanonicalRefID,
			SelectionOrder: e.SelectionOrder, SelectedAsAnchor: e.SelectedAsAnchor,
			AuditReasonCodes: ex.ReasonCodes, AuditSummaryJSON: summary,
		})
	}
	out := make([]analysispg.MaterialitySelectionRow, 0, 1+len(sel.Categories)+len(sel.SKUs)+len(sel.Campaigns))
	out = add(out, sel.Account)
	for _, e := range sel.Categories {
		out = add(out, e)
	}
	for _, e := range sel.SKUs {
		out = add(out, e)
	}
	for _, e := range sel.Campaigns {
		out = add(out, e)
	}
	return out
}
