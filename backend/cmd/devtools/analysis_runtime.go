package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/contracts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/universe"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openAnalysisArtifactStore(ctx context.Context) artifacts.ArtifactStore {
	store := artifacts.ArtifactStore(artifacts.NewMemoryStore())
	_ = config.LoadEnvFiles()
	if cfg, err := config.Load(); err == nil {
		if client, err := storage.New(ctx, storage.S3Config{
			Endpoint: cfg.S3.Endpoint, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey,
			UseSSL: cfg.S3.UseSSL, BucketRaw: cfg.S3.BucketRaw, BucketExports: cfg.S3.BucketExports, BucketArtifacts: cfg.S3.BucketArtifacts,
		}); err == nil {
			store = artifacts.NewMinIOStore(client)
		}
	}
	return store
}

func runSealFactualUniverse(args []string) int {
	return runAnalysisRuntimeSmoke(args)
}

func runAnalysisRuntimeSmoke(args []string) int {
	pg, seller, err := openReadiness(args, "analysis-runtime-smoke")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	ctx := context.Background()
	store := openAnalysisArtifactStore(ctx)
	eval, err := readiness.NewService(pg.Pool, readiness.RealClock{}, readiness.ProductionSemanticRegistry{}).Evaluate(ctx, seller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if eval.Context == nil {
		fmt.Fprintln(os.Stderr, "no LATEST_STABLE factual context from readiness")
		return 1
	}
	repo := analysispg.New(pg.Pool)
	bundle, err := contracts.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	body, _ := bundle.MarshalCanonical()
	if err = repo.PutContractBundle(ctx, bundle.Hash(), "", body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	run, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: eval.Context.AnalysisDate(), CutoffAt: eval.Context.CutoffAt(),
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err = repo.EnsureStages(ctx, run.ID, stages.ProductionImplementedStageNames()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	exec, err := runtime.NewProductionExecutor(pg.Pool, store, "devtools-smoke", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err = exec.ExecuteStage(ctx, run.ID, string(stages.ResolveContext)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err = exec.ExecuteStage(ctx, run.ID, string(stages.SealUniverse)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	run, err = repo.GetRun(ctx, run.ID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if run.UniverseID == "" {
		fmt.Fprintln(os.Stderr, "runtime smoke: run has no factual_universe_id after SEAL_UNIVERSE")
		return 1
	}
	sealer := universe.Sealer{Repo: repo, Store: store}
	binding, err := sealer.LoadValidated(ctx, run.UniverseID, seller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	sealed, err := universe.NewSealedContext(binding.Record, binding)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resolveStage, _ := repo.GetStage(ctx, run.ID, string(stages.ResolveContext))
	sealStage, _ := repo.GetStage(ctx, run.ID, string(stages.SealUniverse))
	printSafeJSON(map[string]any{
		"path":                 "production_executor",
		"seller_account_id":    seller,
		"run_id":               run.ID,
		"analysis_date":        sealed.AnalysisDate().Format("2006-01-02"),
		"analysis_cutoff_at":   sealed.CutoffAt(),
		"universe_id":          binding.ID,
		"manifest_hash":        binding.Hash,
		"contract_bundle_hash": bundle.Hash(),
		"sealed_context_hash":  sealed.ContextHash(),
		"resolve_context": map[string]any{
			"status":      resolveStage.Status,
			"output_hash": resolveStage.OutputHash,
		},
		"seal_universe": map[string]any{
			"status":      sealStage.Status,
			"output_hash": sealStage.OutputHash,
		},
		"sources":             binding.Manifest.Sources,
		"capabilities":        binding.Manifest.Capabilities,
		"dq":                  binding.Manifest.DQ,
		"entity_sets":         binding.Manifest.EntitySets,
		"manifest_object_key": binding.Record.ObjectKey,
	})
	return 0
}

func runAnalysisRecover(args []string) int {
	_ = flag.CommandLine.Parse(args)
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	store := openAnalysisArtifactStore(ctx)
	exec, err := runtime.NewProductionExecutor(pg.Pool, store, "devtools-recover", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	disp := runtime.NewDispatcherFromEnv(exec)
	disp.Enabled = true
	items, err := disp.Scan(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printSafeJSON(map[string]any{"candidates": items, "note": "diagnostics only; no fake stage success"})
	return 0
}

func runInspectAnalysisRun(args []string) int {
	fs := flag.NewFlagSet("inspect-analysis-run", flag.ContinueOnError)
	runID := fs.Int64("run-id", 0, "analysis run id")
	if err := fs.Parse(args); err != nil || *runID <= 0 {
		fmt.Fprintln(os.Stderr, "usage: inspect-analysis-run --run-id N")
		return 2
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	repo := analysispg.New(pg.Pool)
	run, err := repo.GetRun(ctx, *runID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	stagesList, _ := repo.ListStages(ctx, *runID)
	b, _ := json.MarshalIndent(map[string]any{"run": run, "stages": stagesList}, "", "  ")
	fmt.Println(string(b))
	return 0
}

func runLandscapeMaterialitySmoke(args []string) int {
	pg, seller, err := openReadiness(args, "landscape-materiality-smoke")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	ctx := context.Background()
	store := openAnalysisArtifactStore(ctx)
	eval, err := readiness.NewService(pg.Pool, readiness.RealClock{}, readiness.ProductionSemanticRegistry{}).Evaluate(ctx, seller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if eval.Context == nil {
		fmt.Fprintln(os.Stderr, "no LATEST_STABLE factual context from readiness")
		return 1
	}
	repo := analysispg.New(pg.Pool)
	bundle, err := contracts.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	body, _ := bundle.MarshalCanonical()
	if err = repo.PutContractBundle(ctx, bundle.Hash(), "", body); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	run, err := repo.CreateEvaluationRun(ctx, analysispg.Run{
		SellerID: seller, AnalysisDate: eval.Context.AnalysisDate(), CutoffAt: eval.Context.CutoffAt(),
		ContractHash: bundle.Hash(), IntentHash: runtime.StandardIntentHash(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err = repo.EnsureStages(ctx, run.ID, stages.ProductionImplementedStageNames()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	exec, err := runtime.NewProductionExecutor(pg.Pool, store, "devtools-landscape-mat", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	stageOut := map[string]any{}
	for _, key := range stages.ProductionImplementedStageNames() {
		if err = exec.ExecuteStage(ctx, run.ID, key); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		st, err := repo.GetStage(ctx, run.ID, key)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		stageOut[key] = map[string]any{"status": st.Status, "output_hash": st.OutputHash, "input_hash": st.InputHash}
	}
	list, _ := repo.ListStages(ctx, run.ID)
	discoveryStatus := ""
	for _, s := range list {
		if s.Key == string(stages.BuildDiscoveryPack) {
			discoveryStatus = s.Status
		}
	}
	printSafeJSON(map[string]any{
		"path":                  "production_executor_stages_1_to_6",
		"seller_account_id":     seller,
		"run_id":                run.ID,
		"stages":                stageOut,
		"discovery_pack_status": discoveryStatus,
		"policy_version":        "MATERIALITY_POLICY_V1",
		"budget_version":        "DISCOVERY_STANDARD_V1",
		"contract_bundle_hash":  bundle.Hash(),
		"frontier":              "BUILD_LANDSCAPE_DETAIL",
	})
	return 0
}

func runExplainMaterialSelection(args []string) int {
	fs := flag.NewFlagSet("explain-material-selection", flag.ContinueOnError)
	runID := fs.Int64("run-id", 0, "analysis run id")
	kind := fs.String("kind", "", "ACCOUNT|CATEGORY|SKU|CAMPAIGN")
	key := fs.String("key", "", "semantic entity key")
	if err := fs.Parse(args); err != nil || *runID <= 0 || strings.TrimSpace(*kind) == "" || strings.TrimSpace(*key) == "" {
		fmt.Fprintln(os.Stderr, "usage: explain-material-selection --run-id N --kind SKU --key <semantic_key>")
		return 2
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	store := openAnalysisArtifactStore(ctx)
	rankArt, err := loadArtifactByKind(ctx, pg.Pool, store, *runID, string(artifacts.MaterialityRanking))
	if err != nil {
		fmt.Fprintln(os.Stderr, "ranking artifact:", err)
		return 1
	}
	selArt, err := loadArtifactByKind(ctx, pg.Pool, store, *runID, string(artifacts.MaterialitySelection))
	if err != nil {
		fmt.Fprintln(os.Stderr, "selection artifact:", err)
		return 1
	}
	var art materiality.RankingArtifact
	if err = json.Unmarshal(rankArt, &art); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ranking := materiality.Ranking{
		PolicyVersion: art.PolicyVersion, PolicyHash: art.PolicyHash, RankingHash: art.RankingHash,
		Categories: rankedFromArt(art.Categories), SKUs: rankedFromArt(art.SKUs), Campaigns: rankedFromArt(art.Campaigns),
	}
	var sel materiality.SelectionArtifact
	if err = json.Unmarshal(selArt, &sel); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	selection := materiality.Selection{
		Account: sel.Account, Categories: sel.Categories, SKUs: sel.SKUs, Campaigns: sel.Campaigns,
		RankingHash: sel.RankingHash, BudgetVersion: sel.BudgetVersion, BudgetHash: sel.BudgetHash,
		SelectionHash: sel.SelectionHash, PolicyVersion: sel.PolicyVersion, PolicyHash: sel.PolicyHash,
	}
	ex := materiality.ExplainSelection(ranking, selection, materiality.EntityKind(strings.ToUpper(*kind)), *key)
	printSafeJSON(ex)
	return 0
}

func rankedFromArt(in []materiality.RankedEntityArtifact) []materiality.RankedEntity {
	out := make([]materiality.RankedEntity, 0, len(in))
	for _, e := range in {
		lanes := make([]materiality.LaneMembership, 0, len(e.LaneMemberships))
		for _, m := range e.LaneMemberships {
			lanes = append(lanes, materiality.LaneMembership{LaneKey: m.LaneKey, Rank: m.Rank})
		}
		out = append(out, materiality.RankedEntity{
			SemanticKey: e.SemanticKey, CanonicalRefID: e.CanonicalRefID, EntityKind: e.EntityKind,
			CategoryKey: e.CategoryKey, LaneMemberships: lanes, LaneFingerprint: e.LaneFingerprint,
			FusionRank: e.FusionRank, DiversityRank: e.DiversityRank, IsAnchor: e.IsAnchor, AnchorOrder: e.AnchorOrder,
		})
	}
	return out
}

func loadArtifactByKind(ctx context.Context, pool *pgxpool.Pool, store artifacts.ArtifactStore, runID int64, kind string) ([]byte, error) {
	var key string
	err := pool.QueryRow(ctx, `SELECT storage_key FROM analysis_artifacts WHERE run_id=$1 AND kind=$2 ORDER BY id DESC LIMIT 1`, runID, kind).Scan(&key)
	if err != nil {
		return nil, err
	}
	return store.Get(ctx, key)
}
