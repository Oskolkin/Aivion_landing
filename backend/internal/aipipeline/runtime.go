package aipipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RuntimeRunner executes the unified seller AI pipeline with guards for post-sync triggers.
type RuntimeRunner interface {
	RunPostSync(ctx context.Context, in PostSyncPipelineInput) (*RunSellerAIPipelineResult, error)
	RunExistingPipeline(ctx context.Context, in ExistingPipelineInput) (*RunSellerAIPipelineResult, error)
}

type PostSyncPipelineInput struct {
	SellerAccountID int64
	SyncJobID       int64
	MetricDate      time.Time
	Trigger         string
	Force           bool
}

type Runtime struct {
	svc     *SellerAIPipelineService
	queries *dbgen.Queries
	cfg     RuntimeConfig
}

type RuntimeConfig struct {
	MaxRunsPerDay   int
	MinInterval     time.Duration
	RunningTimeout  time.Duration
	PipelineEnabled bool
	LegacyPostSync  bool
}

func LoadRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		MaxRunsPerDay:   envInt("AI_PIPELINE_MAX_RUNS_PER_DAY", 3),
		MinInterval:     envDuration("AI_PIPELINE_MIN_INTERVAL", 2*time.Hour),
		RunningTimeout:  envDuration("AI_PIPELINE_RUNNING_TIMEOUT", 60*time.Minute),
		PipelineEnabled: SellerAIPipelineEnabled(),
		LegacyPostSync:  envBool("LEGACY_AI_POST_SYNC_ENABLED", false),
	}
}

// StaleRecoveryBatchSize returns bounded batch size for startup stale pipeline recovery.
func StaleRecoveryBatchSize() int {
	n := envInt("AI_PIPELINE_STALE_RECOVERY_BATCH_SIZE", 100)
	if n <= 0 {
		return 100
	}
	return n
}

// BuildRuntimeService wires the unified pipeline for worker and devtools.
func BuildRuntimeService(cfg *config.Config, pool *pgxpool.Pool) (*Runtime, error) {
	if err := ValidateRuntimeModeConfig(); err != nil {
		return nil, err
	}
	if err := ValidateAIProviderConfig(cfg.App.Env, cfg.OpenAI.APIKey); err != nil {
		return nil, err
	}
	narrativeSvc, err := dashboardhome.BuildNarrativeService(cfg, pool)
	if err != nil {
		return nil, err
	}
	hasOpenAI := narrativeSvc.OpenAIClient() != nil && !UseFakeAIProviders()
	if !UseFakeAIProviders() && cfg.App.Env != "test" && cfg.App.Env != "development" && cfg.App.Env != "local" && !hasOpenAI {
		return nil, fmt.Errorf("seller ai pipeline enabled but OpenAI client is not configured")
	}

	providers, err := ResolvePipelineProviders(RunSellerAIPipelineRequest{AIEnabled: hasOpenAI}, hasOpenAI)
	if err != nil {
		return nil, err
	}

	aiClient := selectActionsClient(narrativeSvc, cfg, providers.Actions)
	genSvc := actions.NewGenerationServiceFromPool(pool, aiClient)
	store := NewStore(dbgen.New(pool))
	usage := aicost.NewUsageRecorder(dbgen.New(pool))

	gcClient, err := SelectGlobalContextClient(narrativeSvc, cfg, providers.GlobalContext)
	if err != nil {
		return nil, err
	}

	pipelineSvc := NewSellerAIPipelineService(
		aidatapack.NewService(pool),
		selectDashboardRunner(narrativeSvc, store, providers.Dashboard),
		&ActionsGenerationAdapter{GenerationService: genSvc},
		store,
		gcClient,
	)
	pipelineSvc.SetSnapshotPublisher(NewSQLCSnapshotPublisher(pool))
	pipelineSvc.SetUsageRecorder(usage)
	pipelineSvc.SetChunkAnalysisRunner(SelectChunkAnalysisRunner(narrativeSvc, providers.Dashboard))
	pipelineSvc.SetActionsTimeoutSeconds(cfg.OpenAI.ActionsRequestTimeoutSeconds())

	return &Runtime{
		svc:     pipelineSvc,
		queries: dbgen.New(pool),
		cfg:     LoadRuntimeConfig(),
	}, nil
}

// RecoverStaleRunningPipelines marks timed-out running pipelines failed (bounded batch).
func (r *Runtime) RecoverStaleRunningPipelines(ctx context.Context) error {
	if r == nil || r.queries == nil {
		return nil
	}
	cutoff := time.Now().UTC().Add(-r.cfg.RunningTimeout)
	return r.queries.MarkStaleAIPipelineRunsFailed(ctx, pgTimestamptz(cutoff))
}

func (r *Runtime) RunPostSync(ctx context.Context, in PostSyncPipelineInput) (*RunSellerAIPipelineResult, error) {
	if r == nil || r.svc == nil {
		return nil, fmt.Errorf("pipeline runtime is not configured")
	}
	metricDate := time.Date(in.MetricDate.Year(), in.MetricDate.Month(), in.MetricDate.Day(), 0, 0, 0, 0, time.UTC)
	// Parent sync validation is enforced by worker handlers via ParentSyncGuard before RunPostSync is called.

	_ = r.queries.MarkStaleAIPipelineRunsFailed(ctx, pgTimestamptz(time.Now().UTC().Add(-r.cfg.RunningTimeout)))

	if !in.Force {
		if blocked, reason := r.guardPostSyncRun(ctx, in.SellerAccountID, metricDate); blocked {
			return &RunSellerAIPipelineResult{
				SellerAccountID: in.SellerAccountID,
				MetricDate:      metricDate.Format("2006-01-02"),
				Status:          PipelineStatusSkipped,
				ErrorMessage:    reason,
			}, nil
		}
	}

	req := RunSellerAIPipelineRequest{
		SellerAccountID:   in.SellerAccountID,
		MetricDate:        metricDate,
		GenerateDashboard: true,
		GenerateActions:   true,
		Trigger:           in.Trigger,
		ForceRegenerate:   in.Force,
		AIEnabled:         true,
	}
	return r.svc.Run(ctx, req)
}

func (r *Runtime) RunExistingPipeline(ctx context.Context, in ExistingPipelineInput) (*RunSellerAIPipelineResult, error) {
	if r == nil || r.svc == nil {
		return nil, fmt.Errorf("pipeline runtime is not configured")
	}
	if in.PipelineRunID <= 0 {
		return nil, fmt.Errorf("pipeline_run_id is required")
	}
	if in.DispatchID <= 0 {
		return nil, fmt.Errorf("dispatch_id is required")
	}
	if in.LeaseGuard == nil {
		return nil, fmt.Errorf("execution lease guard is required")
	}
	metricDate := time.Date(in.MetricDate.Year(), in.MetricDate.Month(), in.MetricDate.Day(), 0, 0, 0, 0, time.UTC)
	trigger := in.Trigger
	if trigger == "" {
		trigger = "post_sync"
	}
	req := RunSellerAIPipelineRequest{
		SellerAccountID:       in.SellerAccountID,
		MetricDate:            metricDate,
		GenerateDashboard:     true,
		GenerateActions:       true,
		Trigger:               trigger,
		ForceRegenerate:       in.Force,
		AIEnabled:             true,
		ExistingPipelineRunID: in.PipelineRunID,
		DispatchID:            in.DispatchID,
		LeaseGuard:            in.LeaseGuard,
	}
	return r.svc.Run(ctx, req)
}

func (r *Runtime) guardPostSyncRun(ctx context.Context, sellerAccountID int64, metricDate time.Time) (bool, string) {
	if _, err := r.queries.GetRunningAIPipelineRunBySellerAndDate(ctx, dbgen.GetRunningAIPipelineRunBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(metricDate),
	}); err == nil {
		return true, "pipeline already running"
	}

	dayStart := time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)
	count, err := r.queries.CountAIPipelineRunsForSellerOnDate(ctx, dbgen.CountAIPipelineRunsForSellerOnDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(metricDate),
		DayStart:        pgTimestamptz(dayStart),
	})
	if err == nil && count >= int64(r.cfg.MaxRunsPerDay) {
		return true, "daily pipeline run limit reached"
	}

	if r.cfg.MinInterval > 0 {
		if latest, err := r.queries.GetLatestPublishedAIPipelineRunBySeller(ctx, sellerAccountID); err == nil {
			if latest.PublishedAt.Valid && time.Since(latest.PublishedAt.Time) < r.cfg.MinInterval {
				return true, "pipeline min interval not elapsed"
			}
		}
	}
	return false, ""
}

func pgDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func selectDashboardRunner(narrativeSvc *dashboardhome.NarrativeService, store *Store, provider string) DashboardRunner {
	if UseRealProvider(provider) {
		return &DashboardNarrativeAdapter{NarrativeService: narrativeSvc}
	}
	return NewFakeDashboardRunner(store)
}

// SelectChunkAnalysisRunner wires shared chunk analysis using the dashboard provider mode.
// BuildRuntimeService and cmd/devtools run-ai-pipeline must both use this factory.
func SelectChunkAnalysisRunner(narrativeSvc *dashboardhome.NarrativeService, provider string) ChunkAnalysisRunner {
	if UseRealProvider(provider) {
		return &DashboardNarrativeAdapter{NarrativeService: narrativeSvc}
	}
	return NewDeterministicChunkAnalysisRunner()
}

func selectActionsClient(narrativeSvc *dashboardhome.NarrativeService, cfg *config.Config, provider string) actions.ActionAIClient {
	if UseRealProvider(provider) && narrativeSvc != nil && narrativeSvc.OpenAIClient() != nil {
		composeAI, detailAI, _, _, err := SelectActionsOpenAIClientsFromBase(narrativeSvc.OpenAIClient(), cfg)
		if err != nil {
			return actions.NewDeterministicAIFirstFakeClient()
		}
		return actions.NewOpenAIActionClientWithDetail(composeAI, detailAI, cfg.OpenAI.Model)
	}
	return actions.NewDeterministicAIFirstFakeClient()
}
