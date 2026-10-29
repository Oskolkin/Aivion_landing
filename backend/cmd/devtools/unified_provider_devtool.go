package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	uapack "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	uaprovider "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

type unifiedDevtoolEnv struct {
	Cfg      *config.Config
	Postgres *db.Postgres
	S3       *storage.Client
	Queries  *dbgen.Queries
	Asynq    *asynq.Client
	Service  *unifiedanalysis.Service
	UAConfig unifiedanalysis.Config
	Provider uaprovider.Provider
}

func loadUnifiedDevtoolEnv(ctx context.Context, overrides unifiedDevtoolOverrides) (*unifiedDevtoolEnv, error) {
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	uaCfg := cfg.UnifiedAnalysis
	if overrides.DebugArtifacts != nil {
		uaCfg.DebugArtifacts = *overrides.DebugArtifacts
	}
	if overrides.CleanupEnabled != nil {
		uaCfg.CleanupEnabled = *overrides.CleanupEnabled
	}
	if profile := strings.TrimSpace(overrides.PromptProfile); profile != "" {
		uaCfg.PromptProfile = profile
	}
	if model := strings.TrimSpace(overrides.ModelOverride); model != "" {
		uaCfg.Model = model
	}
	if overrides.ForceEnabled && !uaCfg.Enabled {
		if strings.TrimSpace(cfg.OpenAI.APIKey) == "" && !aipipeline.UseFakeAIProviders() {
			return nil, fmt.Errorf("UNIFIED_ANALYSIS_PROVIDER_ENABLED is false and OPENAI_API_KEY is missing; set UNIFIED_ANALYSIS_PROVIDER_ENABLED=true for worker queue processing")
		}
		uaCfg.Enabled = true
	}
	if uaCfg.Enabled {
		if err := uaCfg.Validate(cfg.OpenAI.APIKey); err != nil && !aipipeline.UseFakeAIProviders() {
			return nil, err
		}
	}

	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	s3Client, err := storage.New(ctx, storage.S3Config{
		Endpoint:        cfg.S3.Endpoint,
		AccessKey:       cfg.S3.AccessKey,
		SecretKey:       cfg.S3.SecretKey,
		UseSSL:          cfg.S3.UseSSL,
		BucketRaw:       cfg.S3.BucketRaw,
		BucketExports:   cfg.S3.BucketExports,
		BucketArtifacts: cfg.S3.BucketArtifacts,
	})
	if err != nil {
		postgres.Close()
		return nil, fmt.Errorf("connect minio: %w", err)
	}

	queries := dbgen.New(postgres.Pool)
	svc, prov, err := buildUnifiedAnalysisService(cfg, queries, postgres.Pool, s3Client, uaCfg)
	if err != nil {
		postgres.Close()
		return nil, err
	}

	env := &unifiedDevtoolEnv{
		Cfg:      cfg,
		Postgres: postgres,
		S3:       s3Client,
		Queries:  queries,
		UAConfig: uaCfg,
		Service:  svc,
		Provider: prov,
	}
	if overrides.WithAsynq {
		env.Asynq = jobs.NewAsynqClient(jobs.RedisConfig{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
	}
	return env, nil
}

type unifiedDevtoolOverrides struct {
	ForceEnabled   bool
	DebugArtifacts *bool
	CleanupEnabled *bool
	WithAsynq      bool
	PromptProfile  string
	ModelOverride  string
}

func (e *unifiedDevtoolEnv) Close() {
	if e == nil {
		return
	}
	if e.Asynq != nil {
		e.Asynq.Close()
	}
	if e.Postgres != nil {
		e.Postgres.Close()
	}
}

func buildUnifiedAnalysisService(
	cfg *config.Config,
	queries *dbgen.Queries,
	pool *pgxpool.Pool,
	s3Client *storage.Client,
	uaCfg unifiedanalysis.Config,
) (*unifiedanalysis.Service, uaprovider.Provider, error) {
	uaRepo := unifiedanalysis.NewRepository(queries)
	uaArtifacts := unifiedanalysis.NewArtifactStore(uaRepo)
	uaPackLoader := analysisdata.NewSQLLoader(pool)
	uaPackSvc := uapack.NewService(
		uaPackLoader,
		uapack.NewPackBuilder(uapack.DefaultBuildOptions()),
		uapack.NewValidator(uapack.DefaultBuildOptions()),
		uapack.NewS3PackStorage(s3Client),
	)

	prov, err := resolveUnifiedProvider(cfg, uaCfg)
	if err != nil {
		return nil, nil, err
	}

	svc := unifiedanalysis.NewService(
		uaRepo,
		uaPackSvc,
		prov,
		uaArtifacts,
		uapack.NewS3PackObjectStore(s3Client),
		uaCfg,
	)
	return svc, prov, nil
}

func resolveUnifiedProvider(cfg *config.Config, uaCfg unifiedanalysis.Config) (uaprovider.Provider, error) {
	if aipipeline.UseFakeAIProviders() {
		return uaprovider.NewFakeProvider(uaprovider.FakeScenarioQueuedThenCompleted), nil
	}
	return uaprovider.NewOpenAIClient(uaprovider.ClientConfig{
		APIKey:         cfg.OpenAI.APIKey,
		ProxyURL:       cfg.OpenAI.ProxyURL,
		TimeoutSeconds: uaCfg.HTTPTimeoutSeconds,
		MaxRetries:     uaCfg.TransportRetries,
	})
}

func unifiedRunSummary(run *unifiedanalysis.Run) map[string]any {
	if run == nil {
		return map[string]any{"error": "run is nil"}
	}
	out := map[string]any{
		"run_id":                       run.ID,
		"seller_account_id":            run.SellerAccountID,
		"metric_date":                  run.MetricDate.Format("2006-01-02"),
		"mode":                         run.Mode,
		"trigger":                      run.Trigger,
		"status":                       run.Status,
		"pipeline_version":             run.PipelineVersion,
		"prompt_version":               run.PromptVersion,
		"data_pack_schema_version":     run.DataPackSchemaVersion,
		"data_pack_hash":               run.DataPackHash,
		"pack_id":                      run.PackID,
		"pack_hash":                    run.PackHash,
		"pack_storage_key":             run.PackStorageKey,
		"pack_archive_sha256":          run.PackArchiveSHA256,
		"pack_compressed_bytes":        run.PackCompressedBytes,
		"pack_uncompressed_bytes":      run.PackUncompressedBytes,
		"provider":                     run.Provider,
		"provider_model":               run.ProviderModel,
		"provider_container_id":        run.ProviderContainerID,
		"provider_container_file_id":   run.ProviderContainerFileID,
		"provider_container_file_path": run.ProviderContainerFilePath,
		"provider_response_id":         run.ProviderResponseID,
		"provider_status":              run.ProviderStatus,
		"provider_attempt_count":       run.ProviderAttemptCount,
		"input_tokens":                 run.InputTokens,
		"output_tokens":                run.OutputTokens,
		"total_tokens":                 run.TotalTokens,
		"code_interpreter_calls":       run.CodeInterpreterCalls,
		"provider_latency_ms":          run.ProviderLatencyMs,
		"cleanup_status":               run.CleanupStatus,
		"cleanup_error":                run.CleanupError,
		"error_code":                   run.ErrorCode,
		"error_message":                run.ErrorMessage,
	}
	if run.ProviderStartedAt != nil {
		out["provider_started_at"] = run.ProviderStartedAt.UTC().Format(time.RFC3339)
	}
	if run.ProviderCompletedAt != nil {
		out["provider_completed_at"] = run.ProviderCompletedAt.UTC().Format(time.RFC3339)
	}
	if run.CleanupStartedAt != nil {
		out["cleanup_started_at"] = run.CleanupStartedAt.UTC().Format(time.RFC3339)
	}
	if run.CleanupCompletedAt != nil {
		out["cleanup_completed_at"] = run.CleanupCompletedAt.UTC().Format(time.RFC3339)
	}
	if run.StartedAt != nil {
		out["started_at"] = run.StartedAt.UTC().Format(time.RFC3339)
	}
	if run.CompletedAt != nil {
		out["completed_at"] = run.CompletedAt.UTC().Format(time.RFC3339)
	}
	if run.CancelRequestedAt != nil {
		out["cancel_requested_at"] = run.CancelRequestedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func printUnifiedRunSummary(run *unifiedanalysis.Run) {
	raw, _ := json.MarshalIndent(unifiedRunSummary(run), "", "  ")
	fmt.Println(string(raw))
}

func isUnifiedTerminalStatus(status string) bool {
	return unifiedanalysis.IsTerminalRunStatus(status)
}

func waitUnifiedRunDB(ctx context.Context, repo *unifiedanalysis.Repository, runID int64, timeout time.Duration) (*unifiedanalysis.Run, error) {
	return waitUnifiedRunDBProfile(ctx, repo, runID, timeout, false)
}

func waitUnifiedRunDBProfile(ctx context.Context, repo *unifiedanalysis.Repository, runID int64, timeout time.Duration, analysisProfile bool) (*unifiedanalysis.Run, error) {
	deadline := time.Now().Add(timeout)
	interval := 2 * time.Second
	for {
		run, err := repo.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		waitDone := unifiedanalysis.IsWaitTerminal(run.Status, analysisProfile)
		cleanupDone := run.CleanupStatus == unifiedanalysis.CleanupStatusCompleted || run.ProviderContainerID == ""
		if waitDone && cleanupDone {
			return run, nil
		}
		if time.Now().After(deadline) {
			return run, fmt.Errorf("timeout after %s waiting for terminal+cleanup status (status=%q cleanup=%q)", timeout, run.Status, run.CleanupStatus)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return run, ctx.Err()
		case <-timer.C:
		}
	}
}

func directPollUnifiedRun(ctx context.Context, svc *unifiedanalysis.Service, uaCfg unifiedanalysis.Config, runID int64, timeout time.Duration) (*unifiedanalysis.Run, error) {
	deadline := time.Now().Add(timeout)
	for {
		result, err := svc.PollRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		if result == nil || result.Run == nil {
			return nil, fmt.Errorf("poll returned empty result")
		}
		if result.Completed || result.Failed || result.Cancelled {
			return result.Run, nil
		}
		if time.Now().After(deadline) {
			return result.Run, fmt.Errorf("timeout after %s waiting for provider completion (status=%q)", timeout, result.Run.Status)
		}
		delay := result.NextPollAfter
		if delay <= 0 {
			delay = time.Duration(uaCfg.PollIntervalSeconds) * time.Second
		}
		if delay <= 0 {
			delay = 5 * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result.Run, ctx.Err()
		case <-timer.C:
		}
	}
}

func enqueueUnifiedAnalysisStart(client *asynq.Client, runID int64) error {
	enqueue := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	return enqueue.EnqueueStart(context.Background(), runID)
}

func enqueueUnifiedAnalysisCleanup(client *asynq.Client, runID int64) error {
	enqueue := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	return enqueue.EnqueueCleanup(context.Background(), runID)
}

func checkUnifiedAnalysisMigration(ctx context.Context, pool *pgxpool.Pool) (bool, string) {
	var exists bool
	err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM information_schema.columns
  WHERE table_schema = 'public'
    AND table_name = 'ai_pipeline_runs'
    AND column_name = 'provider_container_id'
)`).Scan(&exists)
	if err != nil {
		return false, err.Error()
	}
	if !exists {
		return false, "migration 000054_unified_analysis_provider not applied"
	}
	return true, "provider columns present"
}

func warnIfProviderDisabledInEnv() {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("UNIFIED_ANALYSIS_PROVIDER_ENABLED")), "true") {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("UNIFIED_ANALYSIS_ENABLED")), "true") {
		return
	}
	fmt.Fprintln(os.Stderr, "note: UNIFIED_ANALYSIS_PROVIDER_ENABLED is false; worker will not process queue tasks unless enabled in worker env")
}
