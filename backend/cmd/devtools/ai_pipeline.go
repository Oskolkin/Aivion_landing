package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
)

func runAIPipeline(args []string) int {
	fs := flag.NewFlagSet("run-ai-pipeline", flag.ContinueOnError)
	var (
		sellerID              int64
		metricDateRaw         string
		dashboardRaw          string
		actionsRaw            string
		aiEnabledRaw          string
		debugArtifactsRaw     string
		resultJSONPath        string
		globalContextProvider string
		dashboardProvider     string
		actionsProvider       string
		force                 bool
		resumePipelineRunID   int64
		resumeRunID           int64
		restartFromStage      string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required for new runs)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required for new runs)")
	fs.StringVar(&dashboardRaw, "dashboard", "true", "generate dashboard true/false")
	fs.StringVar(&actionsRaw, "actions", "true", "generate actions true/false")
	fs.StringVar(&aiEnabledRaw, "ai-enabled", "", "use real OpenAI when configured (default false for new runs, true for resume)")
	fs.StringVar(&debugArtifactsRaw, "debug-artifacts", "false", "persist debug artifacts")
	fs.StringVar(&resultJSONPath, "result-json", "", "optional path to write final machine-readable result JSON")
	fs.StringVar(&globalContextProvider, "global-context-provider", "", "global context provider override: real|fake")
	fs.StringVar(&dashboardProvider, "dashboard-provider", "", "dashboard provider override: real|fake")
	fs.StringVar(&actionsProvider, "actions-provider", "", "actions provider override: real|fake")
	fs.BoolVar(&force, "force", false, "force regenerate")
	fs.Int64Var(&resumePipelineRunID, "resume-pipeline-run-id", 0, "resume an existing ai_pipeline_runs.id (reuses saved stage artifacts when compatible)")
	fs.Int64Var(&resumeRunID, "resume-run-id", 0, "alias for --resume-pipeline-run-id")
	fs.StringVar(&restartFromStage, "restart-from-stage", "", "restart a derived run from a source run (supported: actions, global_context, shared_chunk_validation)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if resumeRunID > 0 {
		resumePipelineRunID = resumeRunID
	}
	restartFromStage = strings.TrimSpace(strings.ToLower(restartFromStage))
	if err := validateRunAIPipelineFlags(resumePipelineRunID, force, restartFromStage, parseBoolFlag(dashboardRaw), parseBoolFlag(actionsRaw)); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	if err := devseed.AssertDevAIPipelineAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	defer postgres.Close()

	if resumePipelineRunID > 0 {
		queries := dbgen.New(postgres.Pool)
		run, loadErr := queries.GetAIPipelineRun(ctx, resumePipelineRunID)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "resume pipeline run %d: %v\n", resumePipelineRunID, loadErr)
			return 1
		}
		sellerID = run.SellerAccountID
		if run.MetricDate.Valid {
			metricDateRaw = run.MetricDate.Time.Format("2006-01-02")
		}
		fmt.Fprintf(os.Stderr, "resume_requested=true resume_source_pipeline_run_id=%d seller_id=%d metric_date=%s\n",
			resumePipelineRunID, sellerID, metricDateRaw)
	}

	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "metric-date is required")
		return 2
	}
	metricDate, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: expected YYYY-MM-DD: %v\n", err)
		return 2
	}
	metricDate = time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)

	narrativeSvc, err := dashboardhome.BuildNarrativeService(cfg, postgres.Pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}

	aiEnabled := resolveRunAIPipelineAIEnabled(resumePipelineRunID > 0 || restartFromStage != "", aiEnabledRaw)
	hasOpenAI := narrativeSvc.OpenAIClient() != nil
	if aiEnabled && !hasOpenAI {
		fmt.Fprintln(os.Stderr, "ai-enabled=true but OpenAI client is not configured (check OPENAI_API_KEY)")
		return 1
	}

	overrides := aipipeline.PipelineProviderOverrides{
		GlobalContext: globalContextProvider,
		Dashboard:     dashboardProvider,
		Actions:       actionsProvider,
	}

	pipelineReq := aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   sellerID,
		MetricDate:        metricDate,
		GenerateDashboard: parseBoolFlag(dashboardRaw),
		GenerateActions:   parseBoolFlag(actionsRaw),
		ForceRegenerate:   force,
		DebugArtifacts:    parseBoolFlag(debugArtifactsRaw),
		AIEnabled:         aiEnabled,
		ProviderOverrides: overrides,
	}
	if restartFromStage != "" {
		pipelineReq.SourcePipelineRunID = resumePipelineRunID
		pipelineReq.RestartFromStage = restartFromStage
		pipelineReq.StageRestartRequested = true
		pipelineReq.Trigger = "devtool_stage_restart"
	} else if resumePipelineRunID > 0 {
		pipelineReq.ExistingPipelineRunID = resumePipelineRunID
		pipelineReq.ResumeRequested = true
		pipelineReq.Trigger = "devtool"
	} else {
		pipelineReq.Trigger = "devtool"
	}

	providers, err := aipipeline.ResolvePipelineProviders(pipelineReq, hasOpenAI)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	pipelineReq.ResolvedProviders = providers

	proxySummary := openaix.ProxySummaryFromEnv()
	fmt.Fprintf(os.Stderr, "openai_timeout_seconds=%d openai_global_context_timeout_seconds=%d openai_actions_timeout_seconds=%d openai_max_retries=%d dashboard_validation_mode=%s proxy_configured=%t\n",
		cfg.OpenAI.TimeoutSeconds,
		cfg.OpenAI.GlobalContextRequestTimeoutSeconds(),
		cfg.OpenAI.ActionsRequestTimeoutSeconds(),
		cfg.OpenAI.MaxRetries,
		dashboardhome.CurrentDashboardValidationMode(),
		proxySummary.AIProxyConfigured)

	var warnings []string
	if aiEnabled && !aipipeline.UseRealProvider(providers.GlobalContext) {
		warnings = append(warnings, "global_context is not using OpenAI provider")
	}

	aiClient := selectActionsClient(narrativeSvc, cfg, providers.Actions)
	genSvc := actions.NewGenerationServiceFromPool(postgres.Pool, aiClient)
	pipelineStore := aipipeline.NewStore(dbgen.New(postgres.Pool))

	dashboardRunner := selectDashboardRunner(narrativeSvc, pipelineStore, providers.Dashboard)

	gcClient, gcErr := aipipeline.SelectGlobalContextClient(narrativeSvc, cfg, providers.GlobalContext)
	if gcErr != nil {
		fmt.Fprintln(os.Stderr, gcErr.Error())
		return 1
	}

	pipelineSvc := aipipeline.NewSellerAIPipelineService(
		aidatapack.NewService(postgres.Pool),
		dashboardRunner,
		&aipipeline.ActionsGenerationAdapter{GenerationService: genSvc},
		pipelineStore,
		gcClient,
	)
	wireDevtoolsPipelineChunkRunner(pipelineSvc, narrativeSvc, providers)
	pipelineSvc.SetSnapshotPublisher(aipipeline.NewSQLCSnapshotPublisher(postgres.Pool))
	pipelineSvc.SetActionsTimeoutSeconds(cfg.OpenAI.ActionsRequestTimeoutSeconds())

	result, err := pipelineSvc.Run(ctx, pipelineReq)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	result.Warnings = warnings
	proxySummary = openaix.ProxySummaryFromEnv()
	result.AIProxy = &proxySummary
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	if strings.TrimSpace(resultJSONPath) != "" {
		if writeErr := os.WriteFile(strings.TrimSpace(resultJSONPath), raw, 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "write result-json: %v\n", writeErr)
			return 1
		}
	}
	if result.Status == aipipeline.PipelineStatusFailed || result.Status == aipipeline.PipelineStatusContextFailed {
		return 1
	}
	return 0
}

func validateRunAIPipelineFlags(resumePipelineRunID int64, force bool, restartFromStage string, dashboardEnabled, actionsEnabled bool) error {
	if restartFromStage != "" {
		if resumePipelineRunID <= 0 {
			return fmt.Errorf("restart-from-stage requires --resume-run-id or --resume-pipeline-run-id")
		}
		if force {
			return fmt.Errorf("restart-from-stage cannot be used with --force=true")
		}
		if !aipipeline.IsSupportedStageRestart(restartFromStage) {
			return fmt.Errorf("unsupported restart-from-stage %q; supported: actions, global_context, shared_chunk_validation", restartFromStage)
		}
		if restartFromStage == aipipeline.ResumeStageActions {
			if !actionsEnabled {
				return fmt.Errorf("restart-from-stage=actions requires --actions=true")
			}
			if !dashboardEnabled {
				return fmt.Errorf("restart-from-stage=actions requires --dashboard=true because the new pipeline snapshot must contain Dashboard and Actions")
			}
		}
		if restartFromStage == aipipeline.ResumeStageGlobalContext || restartFromStage == aipipeline.ResumeStageSharedChunkValidation || restartFromStage == aipipeline.ResumeStageSharedChunkAnalysis {
			if !actionsEnabled || !dashboardEnabled {
				return fmt.Errorf("restart-from-stage=%s requires --dashboard=true and --actions=true", restartFromStage)
			}
		}
		return nil
	}
	if resumePipelineRunID > 0 && force {
		return fmt.Errorf("resume-pipeline-run-id cannot be used with --force=true")
	}
	return nil
}

func resolveRunAIPipelineAIEnabled(resume bool, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return resume
	}
	return parseBoolFlag(raw)
}

func wireDevtoolsPipelineChunkRunner(
	svc *aipipeline.SellerAIPipelineService,
	narrativeSvc *dashboardhome.NarrativeService,
	providers aipipeline.PipelineProviders,
) {
	if svc == nil || !aipipeline.SharedChunkAnalysisEnabled() {
		return
	}
	svc.SetChunkAnalysisRunner(aipipeline.SelectChunkAnalysisRunner(narrativeSvc, providers.Dashboard))
}

func selectDashboardRunner(narrativeSvc *dashboardhome.NarrativeService, store *aipipeline.Store, provider string) aipipeline.DashboardRunner {
	if aipipeline.UseRealProvider(provider) {
		return &aipipeline.DashboardNarrativeAdapter{NarrativeService: narrativeSvc}
	}
	return aipipeline.NewFakeDashboardRunner(store)
}

func selectActionsClient(narrativeSvc *dashboardhome.NarrativeService, cfg *config.Config, provider string) actions.ActionAIClient {
	if aipipeline.UseRealProvider(provider) && narrativeSvc != nil && narrativeSvc.OpenAIClient() != nil {
		composeAI, detailAI, _, _, err := aipipeline.SelectActionsOpenAIClientsFromBase(narrativeSvc.OpenAIClient(), cfg)
		if err != nil {
			return actions.NewDeterministicAIFirstFakeClient()
		}
		return actions.NewOpenAIActionClientWithDetail(composeAI, detailAI, cfg.OpenAI.Model)
	}
	return actions.NewDeterministicAIFirstFakeClient()
}

func parseBoolFlag(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
