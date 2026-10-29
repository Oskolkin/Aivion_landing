package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	analysisruntime "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/runtime"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	appLogger "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/logger"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability/canonical"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ordersync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ozonenrichmentsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productanalyticssync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sentryx"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stocksync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncdomain"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	uapack "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	uaprovider "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/getsentry/sentry-go"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func main() {
	_ = config.LoadEnvFiles()

	cfg, err := config.Load()
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}
	if err := aipipeline.ValidateRuntimeModeConfig(); err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}
	if err := aipipeline.ValidateAIProviderConfig(cfg.App.Env, cfg.OpenAI.APIKey); err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}

	if err := sentryx.Init(sentryx.Config{
		DSN:         cfg.Sentry.DSN,
		Environment: cfg.App.Env,
		Release:     cfg.Sentry.Release,
	}); err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}
	defer sentryx.Flush()

	logger, err := appLogger.New(cfg.App.Env, "backend-worker")
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal(err)
	}
	defer logger.Sync()

	registry := prometheus.NewRegistry()
	m := metrics.New(registry)
	canonical.Register(registry)
	m.AppInfo.WithLabelValues("backend-worker", cfg.App.Env, "dev").Set(1)

	shutdownTracer, err := observability.InitTracer(context.Background(), observability.TracerConfig{ServiceName: "backend-worker", Environment: cfg.App.Env})
	if err != nil {
		logger.Fatal("failed to init tracer", zap.Error(err))
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	metrics.StartMetricsServer(cfg.Server.WorkerMetricsPort, registry, logger, "backend-worker")

	logger.Info("worker config loaded",
		zap.String("redis_addr", cfg.Redis.Addr),
		zap.String("worker_metrics_port", cfg.Server.WorkerMetricsPort),
		zap.String("s3_endpoint", cfg.S3.Endpoint),
	)

	ctx := context.Background()

	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		sentry.CaptureException(err)
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer postgres.Close()

	logger.Info("worker postgres connected")

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
		sentry.CaptureException(err)
		logger.Fatal("failed to connect to s3 storage", zap.Error(err))
	}

	logger.Info("worker s3 connected")

	ozonService, err := ozon.NewService(
		postgres.Pool,
		cfg.Auth.EncryptionKey,
		cfg.Ozon.PerformanceAuthBaseURL,
		cfg.Ozon.DisconnectRetentionDays,
	)
	if err != nil {
		sentry.CaptureException(err)
		logger.Fatal("failed to initialize ozon service", zap.Error(err))
	}

	rawPayloadService := rawpayloads.NewService(postgres.Pool, s3Client, cfg.S3.BucketRaw)
	productsImporter := productsync.NewService(postgres.Pool, ozonService, rawPayloadService)
	ordersImporter := ordersync.NewService(postgres.Pool, ozonService, rawPayloadService)
	stocksImporter := stocksync.NewService(postgres.Pool, ozonService, rawPayloadService)
	adsImporter := adsync.NewService(postgres.Pool, ozonService, rawPayloadService)

	redisCfg := jobs.RedisConfig{
		Addr:        cfg.Redis.Addr,
		Password:    cfg.Redis.Password,
		DB:          cfg.Redis.DB,
		Concurrency: cfg.Sync.WorkerConcurrency,
	}

	server := jobs.NewAsynqServer(redisCfg)
	handler := jobs.NewHandler(logger)
	asynqClient := jobs.NewAsynqClient(redisCfg)
	defer asynqClient.Close()

	ozonSyncCoordinatorHandler := jobs.NewOzonSyncCoordinatorHandler(postgres.Pool, asynqClient, logger)

	workerQueries := dbgen.New(postgres.Pool)

	resolvePostSyncMetricDate := func(ctx context.Context, sellerAccountID int64) (string, error) {
		coverage := analysisdata.NewSQLCoverageSource(postgres.Pool)
		resolution, err := analysisdata.ResolveAnalysisMetricDate(ctx, coverage, analysisdata.ResolveAnalysisMetricDateInput{
			SellerAccountID: sellerAccountID,
			DateMode:        analysisdata.DateModeCurrent,
			Timezone:        analysisdata.DefaultTimezone,
		})
		if err != nil {
			return "", err
		}
		if !resolution.Allowed || resolution.EffectiveMetricDate == "" {
			logger.Info("post-sync unified analysis skipped: no complete data date",
				zap.Int64("seller_account_id", sellerAccountID),
				zap.String("reason", resolution.ResolutionReason),
			)
			return "", nil
		}
		logger.Info("post-sync metric date resolved",
			zap.Int64("seller_account_id", sellerAccountID),
			zap.String("requested_date", resolution.RequestedDate),
			zap.String("effective_metric_date", resolution.EffectiveMetricDate),
			zap.String("latest_complete_data_date", resolution.LatestCompleteDataDate),
			zap.Int("freshness_lag_days", resolution.FreshnessLagDays),
			zap.Bool("is_stale", resolution.IsStale),
			zap.String("resolution_reason", resolution.ResolutionReason),
		)
		return resolution.EffectiveMetricDate, nil
	}

	enqueuePostSyncRecalc := func(ctx context.Context, sellerAccountID, syncJobID int64) error {
		task, err := jobs.NewRecalculateAfterSyncTask(sellerAccountID, syncJobID)
		if err != nil {
			return err
		}
		_, err = asynqClient.Enqueue(task,
			asynq.TaskID("post-sync:"+strconv.FormatInt(syncJobID, 10)+":recalc"),
			asynq.Unique(48*time.Hour),
		)
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		return nil
	}

	enqueueSellerAIPipeline := func(ctx context.Context, sellerAccountID, syncJobID int64) error {
		if !aipipeline.SellerAIPipelineEnabled() {
			return nil
		}
		metricKey, err := resolvePostSyncMetricDate(ctx, sellerAccountID)
		if err != nil {
			return err
		}
		if metricKey == "" {
			return nil
		}
		task, err := jobs.NewRunSellerAIPipelineTask(sellerAccountID, syncJobID, metricKey, "post_sync", false)
		if err != nil {
			return err
		}
		_, err = asynqClient.Enqueue(task,
			asynq.TaskID(fmt.Sprintf("seller-ai-pipeline:%d:%d", sellerAccountID, syncJobID)),
			asynq.Unique(48*time.Hour),
		)
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		return nil
	}

	enqueuePostSyncEnrichment := func(ctx context.Context, sellerAccountID, syncJobID int64) error {
		metricKey, err := resolvePostSyncMetricDate(ctx, sellerAccountID)
		if err != nil {
			return err
		}
		if metricKey == "" {
			metricKey = time.Now().UTC().Format("2006-01-02")
		}
		task, err := jobs.NewOzonEnrichmentP0Task(sellerAccountID, syncJobID, metricKey, "p0")
		if err != nil {
			return err
		}
		_, err = asynqClient.Enqueue(task,
			asynq.TaskID("ozon-enrichment:"+strconv.FormatInt(syncJobID, 10)),
			asynq.Unique(48*time.Hour),
		)
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		return nil
	}

	syncJobCompletedHook := jobs.WithSyncJobCompletedHook(enqueuePostSyncEnrichment)

	productsImportHandler := jobs.NewOzonImportHandler(postgres.Pool, logger, "products", productsImporter, ordersImporter, stocksImporter, adsImporter, syncJobCompletedHook)
	ordersImportHandler := jobs.NewOzonImportHandler(postgres.Pool, logger, "orders", productsImporter, ordersImporter, stocksImporter, adsImporter, syncJobCompletedHook)
	stocksImportHandler := jobs.NewOzonImportHandler(postgres.Pool, logger, "stocks", productsImporter, ordersImporter, stocksImporter, adsImporter, syncJobCompletedHook)
	adsImportHandler := jobs.NewOzonImportHandler(postgres.Pool, logger, "ads", productsImporter, ordersImporter, stocksImporter, adsImporter, syncJobCompletedHook)

	mux := jobs.NewServeMux(handler, logger, m)

	ozonInitialSyncHandler := jobs.NewOzonInitialSyncHandler(postgres.Pool, logger)

	mux.HandleFunc(jobs.TaskTypeOzonInitialSync, func(ctx context.Context, t *asynq.Task) error {
		return ozonInitialSyncHandler.Handle(ctx, t.Payload())
	})
	mux.HandleFunc(jobs.TaskTypeOzonSyncCoordinator, func(ctx context.Context, t *asynq.Task) error {
		return ozonSyncCoordinatorHandler.Handle(ctx, t.Payload())
	})
	mux.HandleFunc(jobs.TaskTypeOzonImportProducts, func(ctx context.Context, t *asynq.Task) error {
		return productsImportHandler.Handle(ctx, t.Payload())
	})
	mux.HandleFunc(jobs.TaskTypeOzonImportOrders, func(ctx context.Context, t *asynq.Task) error {
		return ordersImportHandler.Handle(ctx, t.Payload())
	})
	mux.HandleFunc(jobs.TaskTypeOzonImportStocks, func(ctx context.Context, t *asynq.Task) error {
		return stocksImportHandler.Handle(ctx, t.Payload())
	})
	mux.HandleFunc(jobs.TaskTypeOzonImportAds, func(ctx context.Context, t *asynq.Task) error {
		return adsImportHandler.Handle(ctx, t.Payload())
	})

	accountMetrics := analytics.NewAccountMetricsService(postgres.Pool)
	skuMetrics := analytics.NewSKUMetricsService(postgres.Pool)
	alertsService := alerts.NewService(alerts.NewSQLCRepository(dbgen.New(postgres.Pool)))

	var aiDailySummariesService *aisummaries.Service
	if aipipeline.LegacyAIPostSyncEnabled() {
		var err error
		aiDailySummariesService, err = aisummaries.BuildService(cfg, postgres.Pool)
		if err != nil {
			logger.Fatal("failed to initialize ai daily summaries service", zap.Error(err))
		}
	}
	dashboardNarrativeService, err := dashboardhome.BuildNarrativeService(cfg, postgres.Pool)
	if err != nil {
		logger.Fatal("failed to initialize dashboard narrative service", zap.Error(err))
	}

	enqueueAIDailySummaries := func(ctx context.Context, sellerAccountID, syncJobID int64) error {
		latest, err := workerQueries.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
		if err != nil {
			return fmt.Errorf("ai summaries enqueue latest metric date: %w", err)
		}
		if !latest.Valid || latest.Time.Year() < 1970 {
			return nil
		}

		runRow, err := workerQueries.GetAIDailySummaryRunBySellerAndMetricDate(ctx, dbgen.GetAIDailySummaryRunBySellerAndMetricDateParams{
			SellerAccountID: sellerAccountID,
			MetricDate:      latest,
		})
		if err == nil {
			switch runRow.Status {
			case "completed", "running":
				return nil
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("ai summaries enqueue get run: %w", err)
		}

		task, err := jobs.NewGenerateAIDailySummariesTask(sellerAccountID, syncJobID)
		if err != nil {
			return err
		}
		metricKey := latest.Time.UTC().Format("2006-01-02")
		_, err = asynqClient.Enqueue(task,
			asynq.TaskID(fmt.Sprintf("ai-daily-summaries:%d:%s", sellerAccountID, metricKey)),
			asynq.Unique(48*time.Hour),
		)
		if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
			return err
		}
		return nil
	}

	var enqueueAfterRecalc func(context.Context, int64, int64) error
	if aipipeline.SellerAIPipelineEnabled() {
		enqueueAfterRecalc = enqueueSellerAIPipeline
	} else if aipipeline.LegacyAIPostSyncEnabled() {
		enqueueAfterRecalc = enqueueAIDailySummaries
	}

	postSyncRecalcHandler := jobs.NewRecalculateAfterSyncHandler(
		accountMetrics,
		skuMetrics,
		productanalyticssync.NewServiceWithRawPayloads(postgres.Pool, ozonService, rawPayloadService),
		alertsService,
		workerQueries,
		enqueueAfterRecalc,
		logger,
		m,
	)
	mux.HandleFunc(jobs.TaskTypePostSyncRecalculation, func(ctx context.Context, t *asynq.Task) error {
		return postSyncRecalcHandler.Handle(ctx, t.Payload())
	})

	enrichmentSvc := ozonenrichmentsync.NewService(postgres.Pool, ozonService)
	enrichmentHandler := jobs.NewOzonEnrichmentP0Handler(enrichmentSvc, workerQueries, enqueuePostSyncRecalc, logger, m)
	mux.HandleFunc(jobs.TaskTypeOzonEnrichmentP0, func(ctx context.Context, t *asynq.Task) error {
		return enrichmentHandler.Handle(ctx, t.Payload())
	})

	if aipipeline.SellerAIPipelineEnabled() {
		if err := aipipeline.ValidatePipelineConfig(); err != nil {
			logger.Fatal("seller ai pipeline config invalid", zap.Error(err))
		}
		pipelineRuntime, err := aipipeline.BuildRuntimeService(cfg, postgres.Pool)
		if err != nil {
			logger.Fatal("failed to initialize seller ai pipeline runtime", zap.Error(err))
		}
		recoverCtx, recoverCancel := context.WithTimeout(ctx, 30*time.Second)
		if err := pipelineRuntime.RecoverStaleRunningPipelines(recoverCtx); err != nil {
			logger.Warn("stale ai pipeline recovery failed", zap.Error(err))
		}
		recoverCancel()
		pipelineHandler := jobs.NewRunSellerAIPipelineHandler(pipelineRuntime, workerQueries, postgres.Pool, logger, m)
		mux.HandleFunc(jobs.TaskTypeRunSellerAIPipeline, func(ctx context.Context, t *asynq.Task) error {
			return pipelineHandler.Handle(ctx, t.Payload())
		})
		recoveryBatch := int32(envInt("AI_PIPELINE_DISPATCH_RECOVERY_BATCH_SIZE", 100))
		recoveryHandler := jobs.NewAIPipelineDispatchRecoveryHandler(postgres.Pool, workerQueries, recoveryBatch, enqueueSellerAIPipeline, logger)
		mux.HandleFunc(jobs.TaskTypeAIPipelineDispatchRecovery, func(ctx context.Context, t *asynq.Task) error {
			return recoveryHandler.Handle(ctx, t.Payload())
		})
	}

	// Unified Analysis provider (Stage 3) — NOT hooked to post-sync.
	if cfg.UnifiedAnalysis.Enabled {
		if err := cfg.UnifiedAnalysis.Validate(cfg.OpenAI.APIKey); err != nil {
			logger.Fatal("unified analysis config invalid", zap.Error(err))
		}
		uaRepo := unifiedanalysis.NewRepository(workerQueries)
		uaArtifacts := unifiedanalysis.NewArtifactStore(uaRepo)
		uaPackLoader := analysisdata.NewSQLLoader(postgres.Pool)
		uaPackSvc := uapack.NewService(
			uaPackLoader,
			uapack.NewPackBuilder(uapack.DefaultBuildOptions()),
			uapack.NewValidator(uapack.DefaultBuildOptions()),
			uapack.NewS3PackStorage(s3Client),
		)
		uaProvider, err := uaprovider.NewOpenAIClient(uaprovider.ClientConfig{
			APIKey:         cfg.OpenAI.APIKey,
			ProxyURL:       cfg.OpenAI.ProxyURL,
			TimeoutSeconds: cfg.UnifiedAnalysis.HTTPTimeoutSeconds,
			MaxRetries:     cfg.UnifiedAnalysis.TransportRetries,
		})
		if err != nil {
			logger.Fatal("unified analysis openai client failed", zap.Error(err))
		}
		uaSvc := unifiedanalysis.NewService(
			uaRepo,
			uaPackSvc,
			uaProvider,
			uaArtifacts,
			uapack.NewS3PackObjectStore(s3Client),
			cfg.UnifiedAnalysis,
		)
		uaGlobalFlags := publication.GlobalFlagsFromEnv()
		uaPubSvc := publication.NewService(postgres.Pool, workerQueries, uaArtifacts, publication.DefaultPublicationPolicy(), uaGlobalFlags)
		uaEnqueue := &jobs.AsynqUnifiedAnalysisEnqueue{Client: asynqClient}
		uaHandler := jobs.NewUnifiedAnalysisHandler(uaSvc, uaEnqueue, logger)
		uaHandler.EnableAutoPublish(workerQueries, uaGlobalFlags)
		jobs.RegisterUnifiedAnalysisHandlers(mux, uaHandler)
		uaPublishHandler := jobs.NewUnifiedAnalysisPublishHandler(uaPubSvc, logger)
		jobs.RegisterUnifiedAnalysisPublishHandler(mux, uaPublishHandler)

		// Stage 5 pilot: post-sync auto-run is additive to the legacy pipeline dispatch
		// above (shadow mode: legacy + unified; unified mode: unified auto-run, legacy
		// dispatch stays gated by SELLER_AI_PIPELINE_ENABLED as a fallback). It is a
		// no-op whenever the seller's resolved EffectiveAutoRun is false (unconfigured
		// sellers, legacy mode, or rollout disabled globally/per-seller).
		postSyncRecalcHandler.SetUnifiedAutoRunTrigger(func(ctx context.Context, sellerAccountID int64) error {
			var settingsPtr *dbgen.SellerAnalysisSetting
			row, settingsErr := workerQueries.GetSellerAnalysisSettings(ctx, sellerAccountID)
			if settingsErr == nil {
				settingsPtr = &row
			} else if !errors.Is(settingsErr, pgx.ErrNoRows) {
				return fmt.Errorf("load seller analysis settings: %w", settingsErr)
			}
			settings := publication.EffectiveSettings(settingsPtr, uaGlobalFlags)
			if !settings.EffectiveAutoRun {
				return nil
			}
			metricKey, err := resolvePostSyncMetricDate(ctx, sellerAccountID)
			if err != nil {
				return fmt.Errorf("resolve post-sync metric date: %w", err)
			}
			if metricKey == "" {
				return nil
			}
			metricDate, err := time.Parse("2006-01-02", metricKey)
			if err != nil {
				return fmt.Errorf("parse post-sync metric date: %w", err)
			}
			coverage := analysisdata.NewSQLCoverageSource(postgres.Pool)
			resolution, resolveErr := analysisdata.ResolveAnalysisMetricDate(ctx, coverage, analysisdata.ResolveAnalysisMetricDateInput{
				SellerAccountID: sellerAccountID,
				DateMode:        analysisdata.DateModeCurrent,
				Timezone:        analysisdata.DefaultTimezone,
			})
			if resolveErr != nil {
				return fmt.Errorf("resolve analysis metric date: %w", resolveErr)
			}
			run, err := uaSvc.CreateRunWithOptions(ctx, sellerAccountID, metricDate, unifiedanalysis.CreateRunOptions{
				Force:          false,
				DateResolution: resolution,
				Trigger:        unifiedanalysis.TriggerPostSync,
			})
			if err != nil {
				if errors.Is(err, unifiedanalysis.ErrActiveRunExists) {
					return nil
				}
				return fmt.Errorf("create unified analysis run: %w", err)
			}
			return uaEnqueue.EnqueueStart(ctx, run.ID)
		})
		recoverUACtx, recoverUACancel := context.WithTimeout(ctx, 20*time.Second)
		plans, err := uaSvc.PlanRecovery(recoverUACtx, 50)
		recoverUACancel()
		if err != nil {
			logger.Warn("unified analysis recovery scan failed", zap.Error(err))
		} else {
			for _, plan := range plans {
				var enqErr error
				switch plan.Action {
				case unifiedanalysis.RecoveryActionStart:
					enqErr = uaEnqueue.EnqueueStart(context.Background(), plan.RunID)
				case unifiedanalysis.RecoveryActionPoll:
					enqErr = uaEnqueue.EnqueuePoll(context.Background(), plan.RunID, 1, 0)
				case unifiedanalysis.RecoveryActionCleanup:
					enqErr = uaEnqueue.EnqueueCleanup(context.Background(), plan.RunID)
				case unifiedanalysis.RecoveryActionValidate:
					enqErr = uaEnqueue.EnqueueValidateOutput(context.Background(), plan.RunID)
				case unifiedanalysis.RecoveryActionPublish:
					enqErr = uaEnqueue.EnqueuePublish(context.Background(), plan.RunID)
				}
				if enqErr != nil {
					logger.Warn("unified analysis recovery enqueue failed",
						zap.Int64("run_id", plan.RunID),
						zap.String("action", string(plan.Action)),
						zap.String("status", plan.Status),
						zap.Error(enqErr),
					)
					continue
				}
				logger.Info("unified analysis recovery scheduled",
					zap.Int64("run_id", plan.RunID),
					zap.String("action", string(plan.Action)),
					zap.String("status", plan.Status),
				)
			}
			logger.Info("unified analysis recovery enqueued", zap.Int("count", len(plans)))
		}
		reconcileCtx, reconcileCancel := context.WithTimeout(ctx, 20*time.Second)
		reconcileResults, reconcileErr := uaPubSvc.ReconcileAllCurrentPublicationPointers(reconcileCtx, 100, false)
		reconcileCancel()
		if reconcileErr != nil {
			logger.Warn("unified publication pointer reconcile failed", zap.Error(reconcileErr))
		}
		repaired := 0
		mismatched := 0
		for _, r := range reconcileResults {
			if r.Mismatched {
				mismatched++
			}
			if r.Repaired {
				repaired++
			}
		}
		if mismatched > 0 || repaired > 0 {
			logger.Info("unified publication pointer reconcile completed",
				zap.Int("checked", len(reconcileResults)),
				zap.Int("mismatched", mismatched),
				zap.Int("repaired", repaired),
			)
		}
		logger.Info("unified analysis provider handlers registered",
			zap.Bool("rollout_enabled", uaGlobalFlags.RolloutEnabled),
			zap.Bool("auto_run_enabled", uaGlobalFlags.AutoRunEnabled),
			zap.Bool("auto_publish_enabled", uaGlobalFlags.AutoPublishEnabled),
		)
	}

	if aiDailySummariesService != nil {
		aiDailySummariesHandler := jobs.NewGenerateAIDailySummariesHandler(aiDailySummariesService, dashboardNarrativeService, logger)
		mux.HandleFunc(jobs.TaskTypeGenerateAIDailySummaries, func(ctx context.Context, t *asynq.Task) error {
			return aiDailySummariesHandler.Handle(ctx, t.Payload())
		})
	}

	analysisArtifactStore := artifacts.NewMinIOStore(s3Client)
	analysisExecutor, err := analysisruntime.NewProductionExecutor(
		postgres.Pool,
		analysisArtifactStore,
		"backend-worker",
		analysisruntime.AsynqEnqueueFunc(asynqClient),
	)
	if err != nil {
		logger.Fatal("failed to initialize analysis runtime executor", zap.Error(err))
	}
	analysisruntime.RegisterStageExecuteHandler(mux, analysisExecutor)
	logger.Info("analysis stage execute handler registered")

	analysisRecovery := analysisruntime.NewDispatcherFromEnv(analysisExecutor)
	if analysisRecovery.Enabled {
		recoveryInterval := envDuration("ANALYSIS_RECOVERY_INTERVAL", time.Minute)
		go func() {
			ticker := time.NewTicker(recoveryInterval)
			defer ticker.Stop()
			for range ticker.C {
				if _, err := analysisRecovery.DispatchRecovery(context.Background()); err != nil {
					logger.Warn("analysis recovery dispatch failed", zap.Error(err))
				}
			}
		}()
		logger.Info("analysis recovery scheduler started", zap.Duration("interval", recoveryInterval))
	} else {
		logger.Info("analysis auto-scheduling disabled; stage handler remains registered",
			zap.String("env", "ANALYSIS_ENABLED"),
		)
	}

	cleanupHandler := jobs.NewCleanupMaintenanceHandler(postgres.Pool, logger)
	mux.HandleFunc(jobs.TaskTypeCleanupMaintenance, func(ctx context.Context, t *asynq.Task) error {
		return cleanupHandler.Handle(ctx, t.Payload())
	})

	disconnectedCleanupHandler := jobs.NewDisconnectedAccountCleanupHandler(postgres.Pool, logger)
	mux.HandleFunc(jobs.TaskTypeDisconnectedAccountCleanup, func(ctx context.Context, t *asynq.Task) error {
		return disconnectedCleanupHandler.Handle(ctx, t.Payload())
	})

	chatAttachmentsCleanupHandler, err := jobs.NewChatAttachmentsCleanupHandler(postgres.Pool, cfg, s3Client, logger)
	if err != nil {
		logger.Fatal("failed to initialize chat attachments cleanup handler", zap.Error(err))
	}
	mux.HandleFunc(jobs.TaskTypeChatAttachmentsCleanup, func(ctx context.Context, t *asynq.Task) error {
		return chatAttachmentsCleanupHandler.Handle(ctx, t.Payload())
	})

	orchestrationService := ingestion.NewOrchestrationService(postgres.Pool, asynqClient)
	queries := dbgen.New(postgres.Pool)

	// Periodic sync: each worker instance can run a ticker; disable on all but one replica via
	// WORKER_SCHEDULER_ENABLED=false. Enqueued coordinator tasks also use asynq.Unique; StartSync
	// skips sellers that already have an active sync job.
	if cfg.Sync.SchedulerEnabled && cfg.Sync.Interval > 0 {
		go func() {
			ticker := time.NewTicker(cfg.Sync.Interval)
			defer ticker.Stop()
			for range ticker.C {
				ids, err := queries.ListOzonConnectionsForPeriodicSync(context.Background())
				if err != nil {
					logger.Warn("list ozon connections for periodic sync failed", zap.Error(err))
					continue
				}
				for _, sellerAccountID := range ids {
					if _, err := orchestrationService.StartSync(context.Background(), sellerAccountID, syncdomain.SyncTypeIncremental.String()); err != nil {
						logger.Debug("periodic sync enqueue skipped", zap.Int64("seller_account_id", sellerAccountID), zap.Error(err))
					}
				}
			}
		}()
		logger.Info("ozon periodic sync scheduler started", zap.Duration("interval", cfg.Sync.Interval))
	}

	if cfg.Ozon.DisconnectedAccountCleanupEnabled {
		go func() {
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				if err := disconnectedCleanupHandler.EnqueueDueAccounts(context.Background(), asynqClient); err != nil {
					logger.Warn("disconnected account cleanup enqueue failed", zap.Error(err))
				}
			}
		}()
	}

	if cfg.Cleanup.Enabled && cfg.Cleanup.Schedule > 0 {
		go func() {
			ticker := time.NewTicker(cfg.Cleanup.Schedule)
			defer ticker.Stop()
			for range ticker.C {
				task, err := jobs.NewCleanupMaintenanceTask(cfg.Cleanup.RetentionDays)
				if err != nil {
					logger.Error("cleanup task build failed", zap.Error(err))
					continue
				}
				if _, err := asynqClient.Enqueue(task); err != nil {
					logger.Warn("cleanup enqueue failed", zap.Error(err))
				}
			}
		}()
	}
	if aipipeline.SellerAIPipelineEnabled() && envBool("AI_PIPELINE_DISPATCH_RECOVERY_ENABLED", true) {
		interval := envDuration("AI_PIPELINE_DISPATCH_RECOVERY_INTERVAL", time.Minute)
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				if err := jobs.EnqueueAIPipelineDispatchRecovery(context.Background(), asynqClient); err != nil {
					logger.Warn("ai pipeline dispatch recovery enqueue failed", zap.Error(err))
				}
			}
		}()
	}

	if chat.ChatAttachmentCleanupEnabled() {
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				task, err := jobs.NewChatAttachmentsCleanupTask()
				if err != nil {
					logger.Error("chat attachments cleanup task build failed", zap.Error(err))
					continue
				}
				if _, err := asynqClient.Enqueue(task, asynq.Unique(time.Hour)); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
					logger.Warn("chat attachments cleanup enqueue failed", zap.Error(err))
				}
			}
		}()
		logger.Info("chat attachments cleanup scheduler started", zap.Duration("interval", time.Hour))
	}

	logger.Info("starting worker")

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := jobs.RunAsynqUntilContext(runCtx, server, mux); err != nil && !errors.Is(err, context.Canceled) {
		sentry.CaptureException(err)
		logger.Fatal("worker failed", zap.Error(err))
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()
	_ = shutdownCtx
	logger.Info("worker stopped gracefully")
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "TRUE", "yes", "on":
		return true
	case "0", "false", "FALSE", "no", "off":
		return false
	default:
		return fallback
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
