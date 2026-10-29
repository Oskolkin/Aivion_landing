package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "collect-catalog":
		os.Exit(runCollectCatalog(os.Args[2:]))
	case "collect-sales":
		os.Exit(runCollectSales(os.Args[2:]))
	case "collect-lifecycle":
		os.Exit(runCollectLifecycle(os.Args[2:]))
	case "collect-canonical-vertical":
		os.Exit(runCollectCanonicalVertical(os.Args[2:]))
	case "evaluate-readiness":
		os.Exit(runEvaluateReadiness(os.Args[2:]))
	case "factual-smoke":
		os.Exit(runFactualSmoke(os.Args[2:]))
	case "seal-factual-universe":
		os.Exit(runSealFactualUniverse(os.Args[2:]))
	case "analysis-runtime-smoke":
		os.Exit(runAnalysisRuntimeSmoke(os.Args[2:]))
	case "landscape-materiality-smoke":
		os.Exit(runLandscapeMaterialitySmoke(os.Args[2:]))
	case "explain-material-selection":
		os.Exit(runExplainMaterialSelection(os.Args[2:]))
	case "analysis-recover":
		os.Exit(runAnalysisRecover(os.Args[2:]))
	case "inspect-analysis-run":
		os.Exit(runInspectAnalysisRun(os.Args[2:]))
	case "collection-foundation-scenario":
		os.Exit(runCollectionFoundationScenario(os.Args[2:]))
	case "seed-ad-efficiency":
		os.Exit(runSeedAdEfficiency(os.Args[2:]))
	case "seed-actions":
		os.Exit(runSeedActions(os.Args[2:]))
	case "rerun-actions":
		os.Exit(runRerunActions(os.Args[2:]))
	case "run-ai-pipeline":
		os.Exit(runAIPipeline(os.Args[2:]))
	case "inspect-ai-pipeline":
		os.Exit(runInspectAIPipeline(os.Args[2:]))
	case "export-ai-run-inspector":
		os.Exit(runExportAIRunInspector(os.Args[2:]))
	case "inspect-actions-quality":
		os.Exit(runInspectActionsQuality(os.Args[2:]))
	case "check-ai-connectivity":
		os.Exit(runCheckAIConnectivity(os.Args[2:]))
	case "check-ai-run-readiness":
		os.Exit(runCheckAIRunReadiness(os.Args[2:]))
	case "check-local-env":
		os.Exit(runCheckLocalEnv(os.Args[2:]))
	case "check-ai-pipeline-running":
		os.Exit(runCheckAIPipelineRunning(os.Args[2:]))
	case "mark-stale-ai-pipeline-run":
		os.Exit(runMarkStaleAIPipelineRun(os.Args[2:]))
	case "sync-ozon-enrichment":
		os.Exit(runSyncOzonEnrichment(os.Args[2:]))
	case "ozon-enrichment-status":
		os.Exit(runOzonEnrichmentStatus(os.Args[2:]))
	case "seed-ozon-enrichment":
		os.Exit(runSeedOzonEnrichment(os.Args[2:]))
	case "dump-ai-data-pack":
		os.Exit(runDumpAIDataPack(os.Args[2:]))
	case "dump-ai-data-pack-chunks":
		os.Exit(runDumpAIDataPackChunks(os.Args[2:]))
	case "dump-dashboard-narrative-context":
		os.Exit(runDumpDashboardNarrativeContext(os.Args[2:]))
	case "reset-dashboard-ai-state":
		os.Exit(runResetDashboardAIState(os.Args[2:]))
	case "reset-demo-seller-data":
		os.Exit(runResetDemoSellerData(os.Args[2:]))
	case "inspect-dashboard-ai-only-run":
		os.Exit(runInspectDashboardAIOnlyRun(os.Args[2:]))
	case "validate-dashboard-ui-contract":
		os.Exit(runValidateDashboardUIContract(os.Args[2:]))
	case "audit-dashboard-ai-first":
		os.Exit(runAuditDashboardAIFirst(os.Args[2:]))
	case "cleanup-duplicate-action-chats":
		os.Exit(runCleanupDuplicateActionChats(os.Args[2:]))
	case "cleanup-chat-attachments":
		os.Exit(runCleanupChatAttachments(os.Args[2:]))
	case "repair-chat-titles":
		os.Exit(runRepairChatTitles(os.Args[2:]))
	case "migration-preflight":
		os.Exit(runMigrationPreflight(os.Args[2:]))
	case "build-unified-analysis-pack":
		os.Exit(runBuildUnifiedAnalysisPack(os.Args[2:]))
	case "run-unified-provider-smoke":
		os.Exit(runUnifiedProviderSmoke(os.Args[2:]))
	case "run-unified-analysis":
		os.Exit(runUnifiedAnalysis(os.Args[2:]))
	case "run-unified-analysis-evals":
		os.Exit(runUnifiedAnalysisEvals(os.Args[2:]))
	case "inspect-unified-provider-run":
		os.Exit(runInspectUnifiedProviderRun(os.Args[2:]))
	case "inspect-unified-analysis-output":
		os.Exit(runInspectUnifiedAnalysisOutput(os.Args[2:]))
	case "validate-unified-analysis-output":
		os.Exit(runValidateUnifiedAnalysisOutput(os.Args[2:]))
	case "cancel-unified-provider-run":
		os.Exit(runCancelUnifiedProviderRun(os.Args[2:]))
	case "check-unified-provider-readiness":
		os.Exit(runCheckUnifiedProviderReadiness(os.Args[2:]))
	case "check-unified-analysis-readiness":
		os.Exit(runCheckUnifiedAnalysisReadiness(os.Args[2:]))
	case "publish-unified-analysis":
		os.Exit(runPublishUnifiedAnalysis(os.Args[2:]))
	case "repair-stock-cohorts-and-publish":
		os.Exit(runRepairStockCohortsAndPublish(os.Args[2:]))
	case "inspect-unified-publication":
		os.Exit(runInspectUnifiedPublication(os.Args[2:]))
	case "inspect-unified-bundle":
		os.Exit(runInspectUnifiedBundle(os.Args[2:]))
	case "list-unified-publications":
		os.Exit(runListUnifiedPublications(os.Args[2:]))
	case "rollback-unified-publication":
		os.Exit(runRollbackUnifiedPublication(os.Args[2:]))
	case "promote-shadow-publication":
		os.Exit(runPromoteShadowPublication(os.Args[2:]))
	case "reconcile-unified-publication":
		os.Exit(runReconcileUnifiedPublication(os.Args[2:]))
	case "rebuild-dashboard-publication":
		os.Exit(runRebuildDashboardPublication(os.Args[2:]))
	case "set-unified-analysis-mode":
		os.Exit(runSetUnifiedAnalysisMode(os.Args[2:]))
	case "prepare-unified-browser-scenario":
		os.Exit(runPrepareUnifiedBrowserScenario(os.Args[2:]))
	case "switch-analysis-current-publication":
		os.Exit(runSwitchAnalysisCurrentPublication(os.Args[2:]))
	case "export-unified-publication":
		os.Exit(runExportUnifiedPublication(os.Args[2:]))
	case "import-unified-publication":
		os.Exit(runImportUnifiedPublication(os.Args[2:]))
	case "check-unified-publication-readiness":
		os.Exit(runCheckUnifiedPublicationReadiness(os.Args[2:]))
	case "compare-analysis-sources":
		os.Exit(runCompareAnalysisSources(os.Args[2:]))
	case "validate-seller-analysis-data":
		os.Exit(runValidateSellerAnalysisData(os.Args[2:]))
	case "audit-unified-publication":
		os.Exit(runAuditUnifiedPublication(os.Args[2:]))
	case "prepare-demo-seller-cabinet":
		os.Exit(runPrepareDemoSellerCabinet(os.Args[2:]))
	case "run-eval-suite":
		os.Exit(runEvalSuite(os.Args[2:]))
	case "compare-eval-runs":
		os.Exit(runCompareEvalRuns(os.Args[2:]))
	case "promote-eval-baseline":
		os.Exit(runPromoteEvalBaseline(os.Args[2:]))
	case "export-eval-report":
		os.Exit(runExportEvalReport(os.Args[2:]))
	case "record-human-review":
		os.Exit(runRecordHumanReview(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-catalog --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-sales --seller-id=1 --initial-90d")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-sales --seller-id=1 --date-from=2026-01-01 --date-to=2026-01-31")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-sales --seller-id=1 --resume-domain-run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-lifecycle --seller-id=1 --initial-90d --streams=all")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=stock")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=price")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=advertising --date-from=2026-08-01 --date-to=2026-08-24")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=funnel --date-from=2026-08-01 --date-to=2026-08-24")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=search --date-to=2026-08-24")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=promotions")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=remaining --date-from=2026-08-01 --date-to=2026-08-24")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=all --date-from=2026-08-01 --date-to=2026-08-24")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-canonical-vertical --seller-id=1 --domain=price --resume-domain-run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collect-lifecycle --seller-id=1 --streams=postings --resume-domain-run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools evaluate-readiness --seller-id=1")
	fmt.Fprintln(os.Stderr, "  FACTUAL_TOKEN_SIGNING_KEY=... go run ./cmd/devtools factual-smoke --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools seal-factual-universe --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools analysis-runtime-smoke --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools landscape-materiality-smoke --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools explain-material-selection --run-id=1 --kind=SKU --key=sku:1001")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools analysis-recover")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools inspect-analysis-run --run-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools collection-foundation-scenario --seller-id=1")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_SEED=1 go run ./cmd/devtools seed-ad-efficiency --seller-id=1 --scenario=available")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_SEED=1 go run ./cmd/devtools seed-actions --seller-id=1 --scenario=available")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_ACTIONS_RERUN=1 go run ./cmd/devtools rerun-actions --seller-id=1 --metric-date=2026-06-24")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_AI_PIPELINE=1 SELLER_AI_PIPELINE_ENABLED=1 ACTIONS_AI_FIRST=1 go run ./cmd/devtools run-ai-pipeline --seller-id=1 --metric-date=2026-06-24 --dashboard=true --actions=true --ai-enabled=false --debug-artifacts=true")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_AI_PIPELINE=1 go run ./cmd/devtools inspect-ai-pipeline --pipeline-run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools export-ai-run-inspector --run-id=73 --output=./debug/run-73")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_AI_PIPELINE=1 go run ./cmd/devtools inspect-actions-quality --seller-id=1")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_AI_PIPELINE=1 go run ./cmd/devtools inspect-actions-quality --run-id=44")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools check-ai-connectivity --provider=openai --proxy-diagnostics=true")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools check-local-env")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools cleanup-duplicate-action-chats --dry-run")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools cleanup-chat-attachments --apply")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools repair-chat-titles --dry-run")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools migration-preflight --database-url=$DATABASE_URL")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools repair-chat-titles --apply")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_SEED=1 go run ./cmd/devtools seed-ozon-enrichment --seller-id=1 --metric-date=2026-06-25 --scenario=rich")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools sync-ozon-enrichment --seller-id=1 --metric-date=2026-06-25 --scope=p0 --force")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools ozon-enrichment-status --seller-id=1 --metric-date=2026-06-25")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools dump-ai-data-pack --seller-id=1 --metric-date=2026-06-25 --output=ai-data-pack.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools dump-ai-data-pack-chunks --seller-id=1 --metric-date=2026-06-25 --output-dir=chunks")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools build-unified-analysis-pack --seller-id=1 --metric-date=2026-07-08 --output=./debug/unified-analysis-pack --upload=false --validate=true")
	fmt.Fprintln(os.Stderr, "  ALLOW_DEV_SEED=1 go run ./cmd/devtools prepare-demo-seller-cabinet --seller-id=1 --metric-date=2026-07-08 --run-ai=true --publish=true")
	fmt.Fprintln(os.Stderr, "  UNIFIED_ANALYSIS_PROVIDER_ENABLED=1 go run ./cmd/devtools run-unified-provider-smoke --seller-id=1 --metric-date=2026-07-08 --execution=queue --wait=true")
	fmt.Fprintln(os.Stderr, "  UNIFIED_ANALYSIS_PROMPT_PROFILE=analysis UNIFIED_ANALYSIS_PROVIDER_ENABLED=1 go run ./cmd/devtools run-unified-analysis --seller-id=1 --date-mode=current --execution=queue --wait=true --force=true")
	fmt.Fprintln(os.Stderr, "  UNIFIED_ANALYSIS_PROMPT_PROFILE=analysis UNIFIED_ANALYSIS_PROVIDER_ENABLED=1 go run ./cmd/devtools run-unified-analysis --seller-id=1 --date-mode=fixed --metric-date=2026-07-08 --publish-as-current=true --execution=queue --wait=true --force=true")
	fmt.Fprintln(os.Stderr, "  UNIFIED_ANALYSIS_PROMPT_PROFILE=analysis UNIFIED_ANALYSIS_PROVIDER_ENABLED=1 go run ./cmd/devtools run-unified-analysis-evals --scenario=healthy_no_actions --execution=queue --wait=true --timeout=45m --cleanup=true --force=true")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools inspect-unified-analysis-output --run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools validate-unified-analysis-output --run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools check-unified-analysis-readiness --live=false")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools inspect-unified-provider-run --run-id=123")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools cancel-unified-provider-run --run-id=123 --execution=direct")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools check-unified-provider-readiness --live=false")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools publish-unified-analysis --run-id=123 --execution=queue --wait=true")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools rebuild-dashboard-publication --seller-id=1 --metric-date=2026-07-08")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools inspect-unified-publication --publication-id=45")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools export-unified-publication --publication-id=172 --output=./tmp/export-pub")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools import-unified-publication --input=./tmp/export-pub --seller-id=1 --set-current=true")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools inspect-unified-bundle --publication-id=45")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools list-unified-publications --seller-id=1 --limit=20")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools rollback-unified-publication --seller-id=1 --target-publication-id=2 --reason=\"restore previous\"")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools promote-shadow-publication --publication-id=45 --reason=\"pilot approved\"")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools reconcile-unified-publication --seller-id=1 --dry-run=true")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools set-unified-analysis-mode --seller-id=1 --mode=shadow --auto-run=true --auto-publish=false")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools check-unified-publication-readiness --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools compare-analysis-sources --seller-id=1")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools validate-seller-analysis-data --seller-id=1 --metric-date=2026-07-08 --history-days=30 --output=./debug/validate-seller-analysis-data")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools audit-unified-publication --seller-id=1 --publication-id=45 --output=./debug/seller-1-full-reseed/final-audit")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools analyze-dashboard-chunks --seller-id=1 --metric-date=2026-06-25 --output=chunk-analyses.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools dump-dashboard-narrative-context --seller-id=1 --metric-date=2026-06-25 --source=data_pack --output=dashboard-narrative-context.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools reset-dashboard-ai-state --seller-id=1 --metric-date=2026-06-25 --confirm")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools inspect-dashboard-ai-only-run --seller-id=1 --metric-date=2026-06-30 --dashboard-json=dashboard-home-ai-only-current.json --debug-dir=dashboard-narrative-debug-current --data-pack=ai-data-pack-current.json --output=dashboard-content-forensics.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools validate-dashboard-ui-contract --input=dashboard-home-ai-only-current.json --output=dashboard-ui-contract-validation.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools audit-dashboard-ai-first --input=dashboard-home-ai-only-current.json --output=dashboard-ai-first-audit.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools reset-demo-seller-data --seller-id=1 --metric-date=2026-06-25 --confirm")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools run-eval-suite --suite FACTUAL_GOLDEN --split HOLDOUT --provider fake")
	fmt.Fprintln(os.Stderr, "  OPENAI_LIVE_EVAL=1 go run ./cmd/devtools run-eval-suite --suite CHAT_GOLDEN --split HOLDOUT --provider openai")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools export-eval-report --out eval-report.json")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/devtools record-human-review --case-run-id=1 --reviewer=name --status=PASS")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Scenarios:")
	fmt.Fprintln(os.Stderr, "  available, available_ai_pending, partial, not_enough_data, reset")
}

func runSeedActions(args []string) int {
	fs := flag.NewFlagSet("seed-actions", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		scenarioRaw   string
		dryRun        bool
		force         bool
		aiEnabledRaw  string
		setAsCurrent  bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "optional anchor date YYYY-MM-DD")
	fs.StringVar(&scenarioRaw, "scenario", string(devseed.ActionsSeedScenarioAvailable), "available|checked|empty|mixed")
	fs.BoolVar(&dryRun, "dry-run", false, "validate input without writes")
	fs.BoolVar(&force, "force", false, "reserved for compatibility")
	fs.StringVar(&aiEnabledRaw, "ai-enabled", "", "optional true/false override")
	fs.BoolVar(&setAsCurrent, "set-current", false, "promote seeded actions as seller current legacy AI snapshot")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	scenario := devseed.ActionsSeedScenario(strings.TrimSpace(scenarioRaw))
	var metricDate *time.Time
	if strings.TrimSpace(metricDateRaw) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
		if err != nil {
			fmt.Fprintf(os.Stderr, "metric-date: expected YYYY-MM-DD: %v\n", err)
			return 2
		}
		t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		metricDate = &t
	}
	var aiEnabled *bool
	if strings.TrimSpace(aiEnabledRaw) != "" {
		switch strings.ToLower(strings.TrimSpace(aiEnabledRaw)) {
		case "1", "true", "yes":
			v := true
			aiEnabled = &v
		case "0", "false", "no":
			v := false
			aiEnabled = &v
		default:
			fmt.Fprintln(os.Stderr, "ai-enabled: use true or false")
			return 2
		}
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()
	result, err := devseed.SeedActions(ctx, postgres.Pool, devseed.SeedActionsOptions{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
		Scenario:        scenario,
		DryRun:          dryRun,
		Force:           force,
		AIEnabled:       aiEnabled,
		SetAsCurrent:    setAsCurrent,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed actions: %v\n", err)
		return 1
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	return 0
}

func runRerunActions(args []string) int {
	fs := flag.NewFlagSet("rerun-actions", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		dryRun        bool
		force         bool
		aiEnabledRaw  string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "optional anchor date YYYY-MM-DD")
	fs.BoolVar(&dryRun, "dry-run", false, "run generation without persisting actions")
	fs.BoolVar(&force, "force", false, "reserved for compatibility")
	fs.StringVar(&aiEnabledRaw, "ai-enabled", "", "optional true/false override")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	var metricDate *time.Time
	if strings.TrimSpace(metricDateRaw) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
		if err != nil {
			fmt.Fprintf(os.Stderr, "metric-date: expected YYYY-MM-DD: %v\n", err)
			return 2
		}
		t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		metricDate = &t
	}
	var aiEnabled *bool
	if strings.TrimSpace(aiEnabledRaw) != "" {
		switch strings.ToLower(strings.TrimSpace(aiEnabledRaw)) {
		case "1", "true", "yes":
			v := true
			aiEnabled = &v
		case "0", "false", "no":
			v := false
			aiEnabled = &v
		default:
			fmt.Fprintln(os.Stderr, "ai-enabled: use true or false")
			return 2
		}
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()
	result, err := devseed.RerunActions(ctx, postgres.Pool, nil, devseed.RerunActionsOptions{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
		AIEnabled:       aiEnabled,
		Force:           force,
		DryRun:          dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "rerun actions: %v\n", err)
		return 1
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	return 0
}

func runSeedAdEfficiency(args []string) int {
	fs := flag.NewFlagSet("seed-ad-efficiency", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		scenarioRaw   string
		resetFirst    bool
		withAIRaw     string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.Int64Var(&sellerID, "seller-account-id", 0, "alias for --seller-id")
	fs.StringVar(&metricDateRaw, "metric-date", "", "optional anchor date YYYY-MM-DD")
	fs.StringVar(&scenarioRaw, "scenario", string(devseed.AdEfficiencyScenarioAvailable), "available|available_ai_pending|partial|not_enough_data|reset")
	fs.BoolVar(&resetFirst, "reset-first", true, "delete dev fixture ad data before insert")
	fs.StringVar(&withAIRaw, "with-ai-narrative", "", "optional override true/false")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}

	scenario := devseed.AdEfficiencySeedScenario(strings.TrimSpace(scenarioRaw))
	var metricDate *time.Time
	if strings.TrimSpace(metricDateRaw) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
		if err != nil {
			fmt.Fprintf(os.Stderr, "metric-date: expected YYYY-MM-DD: %v\n", err)
			return 2
		}
		t := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
		metricDate = &t
	}

	var withAI *bool
	if strings.TrimSpace(withAIRaw) != "" {
		switch strings.ToLower(strings.TrimSpace(withAIRaw)) {
		case "1", "true", "yes":
			v := true
			withAI = &v
		case "0", "false", "no":
			v := false
			withAI = &v
		default:
			fmt.Fprintln(os.Stderr, "with-ai-narrative: use true or false")
			return 2
		}
	}

	fmt.Println("dev-only: seeding ad efficiency fixture data")
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	result, err := devseed.SeedAdEfficiency(ctx, postgres.Pool, cfg, devseed.AdEfficiencySeedOptions{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
		Scenario:        scenario,
		ResetFirst:      resetFirst,
		WithAINarrative: withAI,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed ad efficiency: %v\n", err)
		return 1
	}
	fmt.Println()
	devseed.PrintAdEfficiencySeedReport(result)
	return 0
}
