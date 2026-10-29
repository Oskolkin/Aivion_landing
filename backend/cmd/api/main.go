package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/account"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actionview"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/admin"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysissource"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/buildinfo"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatexport"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	dashboardsvc "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboard"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/health"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/handlers"
	appmw "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/middleware"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	appLogger "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/logger"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability/canonical"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ozonenrichmentsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/pricingconstraints"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productcatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/readiness"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	appRedis "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/redis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sentryx"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/syncstate"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type adminIngestionAdapter struct {
	svc *ingestion.OrchestrationService
}

func (a adminIngestionAdapter) StartInitialSync(ctx context.Context, sellerAccountID int64) (int64, string, error) {
	job, err := a.svc.StartInitialSync(ctx, sellerAccountID)
	if err != nil {
		return 0, "", err
	}
	return job.ID, job.Status, nil
}

type adminCursorAdapter struct {
	svc *syncstate.SyncCursorService
}

func (a adminCursorAdapter) ResetCursor(ctx context.Context, sellerAccountID int64, domain, cursorType string, cursorValue *string) error {
	_, err := a.svc.ResetCursor(ctx, sellerAccountID, domain, cursorType, cursorValue)
	return err
}

type adminAlertsAdapter struct {
	svc *alerts.Service
}

func (a adminAlertsAdapter) Rerun(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (map[string]any, error) {
	summary, err := a.svc.RunForAccountWithType(ctx, sellerAccountID, asOfDate, alerts.RunTypeManual)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"alert_run_id":           summary.RunID,
		"status":                 summary.Status,
		"total_generated_alerts": summary.TotalGeneratedAlerts,
		"total_upserted_alerts":  summary.TotalUpsertedAlerts,
		"total_skipped_rules":    summary.TotalSkippedRules,
	}, nil
}

type adminRecommendationsAdapter struct {
	svc *recommendations.Service
}

func (a adminRecommendationsAdapter) Rerun(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (map[string]any, error) {
	summary, err := a.svc.GenerateForAccountWithType(ctx, sellerAccountID, asOfDate, "manual")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"recommendation_run_id": summary.RunID,
		"generated_total":       summary.GeneratedTotal,
		"valid_total":           summary.ValidTotal,
		"rejected_total":        summary.RejectedTotal,
		"upserted_total":        summary.UpsertedTotal,
		"input_tokens":          summary.InputTokens,
		"output_tokens":         summary.OutputTokens,
	}, nil
}

type chatActionContextProvider struct {
	svc *actions.Service
}

func (p chatActionContextProvider) GetActionForChatContext(ctx context.Context, sellerAccountID int64, actionKey string) (*chat.ActionPromptContext, error) {
	if p.svc == nil {
		return nil, nil
	}
	item, err := p.svc.GetActionForChatContext(ctx, sellerAccountID, actionKey)
	if err != nil || item == nil {
		return nil, err
	}
	return &chat.ActionPromptContext{
		ActionKey:          item.ActionKey,
		Title:              item.Title,
		Type:               item.Type,
		Category:           item.Category,
		Priority:           item.Priority,
		Status:             item.Status,
		MetricDate:         item.MetricDate,
		Summary:            item.Summary,
		Reason:             item.Reason,
		NextStep:           item.NextStep,
		ExpectedEffect:     item.ExpectedEffect,
		Evidence:           item.Evidence,
		EvidenceFactIDs:    item.EvidenceFactIDs,
		ObjectActions:      item.ObjectActions,
		CompletionCriteria: item.CompletionCriteria,
		Limitations:        item.Limitations,
		PipelineRunID:      item.PipelineRunID,
		ActionsRunID:       item.ActionsRunID,
		DataPackHash:       item.DataPackHash,
		GlobalContextHash:  item.GlobalContextHash,
	}, nil
}

func main() {
	_ = config.LoadEnvFiles()

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	if err := aipipeline.ValidateRuntimeModeConfig(); err != nil {
		panic(err)
	}
	if err := aipipeline.ValidateAIProviderConfig(cfg.App.Env, cfg.OpenAI.APIKey); err != nil {
		panic(err)
	}

	if err := sentryx.Init(sentryx.Config{
		DSN:         cfg.Sentry.DSN,
		Environment: cfg.App.Env,
		Release:     cfg.Sentry.Release,
	}); err != nil {
		panic(err)
	}
	defer sentryx.Flush()

	log, err := appLogger.New(cfg.App.Env, "backend-api")
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	registry := prometheus.NewRegistry()
	m := metrics.New(registry)
	canonical.Register(registry)
	m.AppInfo.WithLabelValues("backend-api", cfg.App.Env, buildinfo.BuildVersion).Set(1)

	shutdownTracer, err := observability.InitTracer(context.Background(), observability.TracerConfig{ServiceName: "backend-api", Environment: cfg.App.Env})
	if err != nil {
		log.Fatal("failed to init tracer", zap.Error(err))
	}
	defer func() { _ = shutdownTracer(context.Background()) }()

	log.Info("config loaded",
		zap.String("port", cfg.Server.Port),
		zap.String("migrations_path", cfg.DB.MigrationsPath),
		zap.String("redis_addr", cfg.Redis.Addr),
		zap.String("s3_endpoint", cfg.S3.Endpoint),
		zap.String("auth_cookie_name", cfg.Auth.CookieName),
	)
	uaFlags := publication.GlobalFlagsFromEnv()
	log.Info("unified analysis runtime config",
		zap.Bool("unified_rollout_enabled", uaFlags.RolloutEnabled),
		zap.Bool("unified_auto_run_enabled", uaFlags.AutoRunEnabled),
		zap.Bool("unified_auto_publish_enabled", uaFlags.AutoPublishEnabled),
		zap.Bool("legacy_fallback_enabled", uaFlags.LegacyFallbackEnabled),
		zap.String("dashboard_read_mode", "published_snapshot_pointer"),
	)
	buildInfoProvider := buildinfo.NewProvider("marketplace-ai-mvp-api", cfg.App.Env, ":"+cfg.Server.Port)
	buildDetails := buildInfoProvider.Info()
	log.Info("api build info",
		zap.String("version", buildDetails.Build.Version),
		zap.String("commit", buildDetails.Build.Commit),
		zap.String("built_at", buildDetails.Build.BuiltAt),
		zap.String("started_at", buildDetails.StartedAt),
		zap.String("go_version", buildDetails.Runtime.GoVersion),
		zap.String("environment", buildDetails.Runtime.Environment),
		zap.String("listen_addr", buildDetails.Runtime.ListenAddr),
	)

	ctx := context.Background()

	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		m.DBUp.Set(0)
		sentry.CaptureException(err)
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer postgres.Close()

	m.DBUp.Set(1)
	log.Info("db connected")

	if err := db.RunMigrations(postgres.SQLDB, cfg.DB.MigrationsPath); err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to run migrations", zap.Error(err))
	}
	log.Info("migrations ok")

	redisClient, err := appRedis.New(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		m.RedisUp.Set(0)
		sentry.CaptureException(err)
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	m.RedisUp.Set(1)
	log.Info("redis connected")

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
		m.S3Up.Set(0)
		sentry.CaptureException(err)
		log.Fatal("failed to connect to s3 storage", zap.Error(err))
	}

	m.S3Up.Set(1)
	log.Info("s3 connected")

	if err := storage.RunSmokeTest(ctx, s3Client); err != nil {
		sentry.CaptureException(err)
		log.Fatal("s3 smoke test failed", zap.Error(err))
	}
	log.Info("s3 smoke test ok")

	asynqClient := jobs.NewAsynqClient(jobs.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer asynqClient.Close()

	task, err := jobs.NewSystemPingTask("hello from api startup")
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to create demo task", zap.Error(err))
	}

	info, err := asynqClient.Enqueue(task)
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to enqueue demo task", zap.Error(err))
	}

	log.Info("demo task enqueued",
		zap.String("task_id", info.ID),
		zap.String("queue", info.Queue),
		zap.String("type", task.Type()),
	)

	authService := auth.NewService(
		postgres.Pool,
		time.Duration(cfg.Auth.SessionTTLHours)*time.Hour,
		cfg.Admin.Emails,
	)
	authRateLimiter := appmw.NewAuthRateLimiter(
		redisClient.Raw,
		cfg.Server.AuthRateLimit.LoginIPLimit,
		cfg.Server.AuthRateLimit.LoginEmailLimit,
		cfg.Server.AuthRateLimit.RegisterIPLimit,
		cfg.Server.AuthRateLimit.Window,
		cfg.Server.AuthRateLimit.RedisFailOpen,
		log,
	)
	authHandler := handlers.NewAuthHandler(authService, cfg.Auth.CookieName, cfg.Admin.Emails, cfg.Auth.SessionCookieSecure).
		WithPublicRegistration(cfg.Auth.PublicRegistration).
		WithLoginRateLimiter(authRateLimiter).
		WithDemoSellerChecker(cfg).
		WithPublicDemoMode(cfg.App.PublicDemoMode)
	liveReviewAuth := &auth.LiveReviewConfig{
		Enabled:                cfg.LiveReview.Enabled,
		AllowedHostSuffix:      cfg.LiveReview.AllowedHostSuffix,
		AllowedSellerAccountID: cfg.LiveReview.AllowedSellerAccountID,
		IsAllowedSeller:        cfg.IsStagingDemoSeller,
		OnEvent: func(event auth.LiveReviewEvent) {
			fields := []zap.Field{
				zap.String("event", event.Name),
				zap.String("hostname", event.Hostname),
				zap.String("method", event.Method),
				zap.String("path", event.Path),
				zap.Int("status", event.Status),
			}
			if event.InviteID > 0 {
				fields = append(fields, zap.Int64("invite_id", event.InviteID))
			}
			if event.SellerAccountID > 0 {
				fields = append(fields, zap.Int64("seller_account_id", event.SellerAccountID))
			}
			if event.Reason != "" {
				fields = append(fields, zap.String("reason", event.Reason))
			}
			log.Info(event.Name, fields...)
			switch event.Name {
			case "live_review_auth_success":
				m.LiveReviewRequestsTotal.WithLabelValues("live_review", "ok").Inc()
			case "live_review_auth_failed", "live_review_expired", "live_review_revoked":
				reason := event.Reason
				if reason == "" {
					reason = "unknown"
				}
				m.LiveReviewAuthFailuresTotal.WithLabelValues(reason).Inc()
				m.LiveReviewRequestsTotal.WithLabelValues("live_review", "auth_failed").Inc()
			case "live_review_write_blocked":
				m.LiveReviewWriteBlockedTotal.Inc()
				m.LiveReviewRequestsTotal.WithLabelValues("live_review", "write_blocked").Inc()
			}
		},
	}
	authMiddleware := auth.Middleware(authService, cfg.Auth.CookieName, liveReviewAuth)
	sellerMiddleware := auth.RequireSellerAccount()
	adminMiddleware := auth.AdminMiddleware(cfg.Admin.Emails)

	accountService := account.NewService(postgres.Pool)
	accountHandler := handlers.NewAccountHandler(accountService)
	dashboardService := analytics.NewDashboardService(postgres.Pool)
	stocksViewService := analytics.NewStocksViewService(postgres.Pool)
	criticalSKUService := analytics.NewCriticalSKUService(postgres.Pool)
	replenishmentService := analytics.NewReplenishmentService(postgres.Pool)
	advertisingService := analytics.NewAdvertisingService(postgres.Pool)
	pricingConstraintsService := pricingconstraints.NewService(dbgen.New(postgres.Pool))
	analyticsDashboardHandler := handlers.NewAnalyticsDashboardHandler(
		dashboardService,
		stocksViewService,
		criticalSKUService,
		replenishmentService,
		advertisingService,
	)
	pricingConstraintsHandler := handlers.NewPricingConstraintsHandler(pricingConstraintsService)
	productsCatalogService := productcatalog.NewService(postgres.Pool, dashboardService)
	productsHandler := handlers.NewProductsHandler(productsCatalogService)
	alertsService := alerts.NewService(alerts.NewSQLCRepository(dbgen.New(postgres.Pool)))
	alertsHandler := handlers.NewAlertsHandler(alertsService)
	queries := dbgen.New(postgres.Pool)
	sourceResolver := analysissource.NewResolver(queries, analysissource.GlobalFlagsFromEnv())
	actionsService := actions.NewServiceWithSourceResolver(actions.NewSQLCRepository(queries), sourceResolver)
	analysisRepo := analysispg.New(postgres.Pool)
	actionViewService := actionview.NewService(analysisRepo)
	dashboardHomeHandler := handlers.NewDashboardHomeHandler(dashboardsvc.NewService(analysisRepo))
	actionViewHandler := handlers.NewActionViewHandler(actionViewService)
	aiDailyUsage := aicost.NewDailyUsageGuard(queries, aicost.DailyLimitsConfig{
		ChatMaxQuestionsPerDay:      cfg.AI.ChatMaxQuestionsPerDay,
		RecommendationMaxRunsPerDay: cfg.AI.RecommendationMaxRunsPerDay,
		SummaryMaxRunsPerDay:        cfg.AI.SummaryMaxRunsPerDay,
		MaxInputTokensPerRequest:    cfg.AI.MaxInputTokensPerRequest,
		MaxOutputTokensPerRequest:   cfg.AI.MaxOutputTokensPerRequest,
	})
	recommendationsRepo := recommendations.NewSQLCRepository(queries)
	recOpenAIClient, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:               cfg.OpenAI.APIKey,
		Model:                cfg.OpenAI.Model,
		TimeoutSeconds:       cfg.OpenAI.TimeoutSeconds,
		MaxRetries:           cfg.OpenAI.MaxRetries,
		ProxyURL:             cfg.OpenAI.AIHTTPSProxyURL(),
		MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
		MaxOutputTokens:      cfg.AI.OpenAIMaxOutputTokens(),
	})
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to initialize recommendations openai client", zap.Error(err))
	}
	composeOpenAI, detailOpenAI, _, _, err := aipipeline.SelectActionsOpenAIClientsFromBase(recOpenAIClient, cfg)
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to initialize actions openai client", zap.Error(err))
	}
	actionsAIClient := actions.NewOpenAIActionClientWithDetail(composeOpenAI, detailOpenAI, cfg.OpenAI.Model)
	actionsRerunService := actions.NewRerunService(actions.NewGenerationServiceFromPool(postgres.Pool, actionsAIClient))
	actionsHandler := handlers.NewActionsHandler(actionsService, actionsRerunService)
	recommendationsService := recommendations.NewService(
		recommendationsRepo,
		recommendations.NewContextBuilderWithLimits(recommendationsRepo, recommendations.ContextBuildLimits{
			MaxItemsPerList: cfg.AI.RecommendationMaxContextItems,
			MaxContextBytes: cfg.AI.RecommendationMaxContextBytes,
		}),
		recOpenAIClient,
		recommendations.NewOutputValidator(),
		recommendations.ServiceConfig{
			RunType:       "manual",
			Source:        "chatgpt",
			Model:         cfg.OpenAI.Model,
			PromptVersion: "stage8.prompt.v1",
		},
		aiDailyUsage,
	)
	recommendationsHandler := handlers.NewRecommendationsHandler(recommendationsService)

	chatQueries := queries
	chatRepo := chat.NewSQLCRepository(chatQueries)
	chatToolRegistry := chat.NewDefaultToolRegistry()
	chatToolValidator := chat.NewToolPlanValidator(chatToolRegistry)
	chatToolDataRepo := chat.NewSQLCToolDataRepository(chatQueries)
	chatToolSet := chat.NewToolSet(chatToolRegistry, chatToolDataRepo)
	var chatAIClient chat.AIClient
	if aipipeline.UseFakeAIProviders() {
		chatAIClient = chat.NewDeterministicFakeAIClient()
		log.Info("chat AI client: deterministic fake (AI_PROVIDER_MODE=fake)")
	} else {
		openAIChatClient, err := chat.NewOpenAIClient(chat.OpenAIClientConfig{
			APIKey:               cfg.OpenAI.APIKey,
			Model:                cfg.OpenAI.Model,
			TimeoutSeconds:       cfg.OpenAI.TimeoutSeconds,
			MaxRetries:           cfg.OpenAI.MaxRetries,
			ProxyURL:             cfg.OpenAI.AIHTTPSProxyURL(),
			MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
			MaxOutputTokens:      cfg.AI.OpenAIMaxOutputTokens(),
		})
		if err != nil {
			sentry.CaptureException(err)
			log.Fatal("failed to initialize chat openai client", zap.Error(err))
		}
		chatAIClient = openAIChatClient
	}
	chatExportData := chatexport.NewDataService(
		chatQueries,
		criticalSKUService,
		advertisingService,
	)
	chatExportStorage := chatexport.NewObjectStorage(s3Client, cfg.S3.BucketExports)
	chatExporter := chatexport.NewExporter(chatExportData, chatExportStorage)
	chatAttachmentRepo := chat.NewSQLCAttachmentRepository(chatQueries)
	chatInputStorage := chat.NewS3InputAttachmentStorage(s3Client)
	chatPrechecker := chat.NewPrecheckerWithAsOf(
		chat.NewSQLCEntityResolver(chatQueries),
		chat.NewChatexportAsOfResolver(chatExportData),
	)
	chatService, err := chat.NewServiceWithDeps(chat.ServiceDeps{
		Repo:              chatRepo,
		AIClient:          chatAIClient,
		ToolRegistry:      chatToolRegistry,
		ToolPlanValidator: chatToolValidator,
		ToolExecutor:      chatToolSet,
		ContextAssembler:  chat.NewContextAssemblerWithLimits(cfg.AI.ChatMaxContextBytes, cfg.AI.ChatMaxContextItems),
		AnswerValidator:   chat.NewAnswerValidator(),
		Prechecker:        chatPrechecker,
		Exporter:          chatExporter,
		Attachments:       chatAttachmentRepo,
		ExportStorage:     chat.NewS3ExportStorage(s3Client, cfg.S3.BucketExports),
		InputStorage:      chatInputStorage,
		InputBucket:       cfg.S3.BucketArtifacts,
		ActionContextProvider: chatActionContextProvider{
			svc: actionsService,
		},
		Config: chat.ServiceConfig{
			PlannerModel:         cfg.OpenAI.Model,
			AnswerModel:          cfg.OpenAI.Model,
			PlannerPromptVersion: chat.PlannerPromptVersion,
			AnswerPromptVersion:  chat.AnswerPromptVersion,
		},
		DailyUsage: aiDailyUsage,
	})
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to initialize chat service", zap.Error(err))
	}
	var chatV2Provider chatv2.ChatAgentProvider
	if aipipeline.UseFakeAIProviders() {
		fp := chatv2.NewFakeProvider()
		if ms, err := strconv.Atoi(strings.TrimSpace(os.Getenv("FAKE_PROVIDER_DELAY_MS"))); err == nil && ms > 0 {
			fp.Delay = time.Duration(ms) * time.Millisecond
		}
		chatV2Provider = fp
		log.Info("chat v2 provider: deterministic fake")
	} else {
		timeout := time.Duration(cfg.OpenAI.TimeoutSeconds) * time.Second
		chatV2Provider = chatv2.NewOpenAIProvider(cfg.OpenAI.APIKey, cfg.OpenAI.Model, "", timeout)
	}
	secret := strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_HMAC_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("FACTUAL_TOKEN_SIGNING_KEY"))
	}
	readinessSvc := readiness.NewService(postgres.Pool, readiness.RealClock{}, readiness.ProductionSemanticRegistry{})
	chatV2, err := chatv2.NewFromPool(postgres.Pool, secret, analysisRepo, actionViewService, chatv2.ReadinessLatest{Svc: readinessSvc}, chatV2Provider, chatv2.NewS3ArtifactStore(s3Client, cfg.S3.BucketExports))
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to initialize chat v2", zap.Error(err))
	}
	chatHandler := handlers.NewChatHandler(chatService, actionsService).WithV2(chatV2)

	orchestrationService := ingestion.NewOrchestrationService(postgres.Pool, asynqClient)

	ozonService, err := ozon.NewService(
		postgres.Pool,
		cfg.Auth.EncryptionKey,
		cfg.Ozon.PerformanceAuthBaseURL,
		cfg.Ozon.DisconnectRetentionDays,
	)
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to initialize ozon service", zap.Error(err))
	}
	ozonHandler := handlers.NewOzonHandler(ozonService, orchestrationService).WithDemoSellerChecker(cfg)
	connectionRequiredMiddleware := appmw.RequireOzonConnection(ozonService, cfg)
	ozonIngestionSyncHandler := handlers.NewOzonIngestionSyncHandler(orchestrationService)
	syncCursorService := syncstate.NewSyncCursorService(postgres.Pool)

	statusService := ingestion.NewStatusService(postgres.Pool)
	ozonIngestionStatusHandler := handlers.NewOzonIngestionStatusHandler(statusService)
	ozonEnrichmentService := ozonenrichmentsync.NewService(postgres.Pool, ozonService)
	ozonEnrichmentHandler := handlers.NewOzonEnrichmentHandler(ozonEnrichmentService)
	ozonOAuthCallback := handlers.NewOzonOAuthCallback(ozonService)
	aiDataPackHandler := handlers.NewAIDataPackHandler(aidatapack.NewService(postgres.Pool))

	readinessChecker := health.NewCompositeChecker(
		health.NewPostgresChecker(postgres.Pool),
		health.NewRedisChecker(redisClient.Raw),
	)
	healthHandler := health.NewHandler(readinessChecker)

	adminRepo := admin.NewSQLCRepository(dbgen.New(postgres.Pool))
	accountMetricsRebuild := analytics.NewAccountMetricsService(postgres.Pool)
	skuMetricsRebuild := analytics.NewSKUMetricsService(postgres.Pool)
	adminService, err := admin.NewService(admin.ServiceDeps{
		Repo:                   adminRepo,
		IngestionService:       adminIngestionAdapter{svc: orchestrationService},
		CursorService:          adminCursorAdapter{svc: syncCursorService},
		AlertsService:          adminAlertsAdapter{svc: alertsService},
		RecommendationsService: adminRecommendationsAdapter{svc: recommendationsService},
		MetricsService:         admin.NewAnalyticsMetricsRerunner(accountMetricsRebuild, skuMetricsRebuild),
	})
	if err != nil {
		sentry.CaptureException(err)
		log.Fatal("failed to initialize admin service", zap.Error(err))
	}
	adminHandler := handlers.NewAdminHandler(adminService)
	aiPipelineAdminHandler := handlers.NewAIPipelineAdminHandler(aipipeline.NewStore(queries), queries, adminService)

	aiSummariesHandler := handlers.NewAISummariesHandler(postgres.Pool)
	systemHandler := handlers.NewSystemHandler(buildInfoProvider)

	server := httpserver.New(
		cfg.Server.Port,
		cfg.Server.HTTP,
		cfg.Server.CORSAllowedOrigins,
		authRateLimiter,
		healthHandler,
		authHandler,
		adminHandler,
		accountHandler,
		chatHandler,
		analyticsDashboardHandler,
		dashboardHomeHandler,
		systemHandler,
		pricingConstraintsHandler,
		productsHandler,
		alertsHandler,
		actionsHandler,
		actionViewHandler,
		recommendationsHandler,
		aiSummariesHandler,
		ozonHandler,
		ozonIngestionSyncHandler,
		ozonIngestionStatusHandler,
		ozonEnrichmentHandler,
		ozonOAuthCallback,
		aiDataPackHandler,
		aiPipelineAdminHandler,
		authMiddleware,
		sellerMiddleware,
		connectionRequiredMiddleware,
		adminMiddleware,
		log,
		m,
		registry,
	)

	log.Info("starting backend",
		zap.String("port", cfg.Server.Port),
	)

	runCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-runCtx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			sentry.CaptureException(err)
			log.Fatal("server failed to start", zap.Error(err))
		}
		return
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		sentry.CaptureException(err)
		log.Error("server shutdown error", zap.Error(err))
	}
	log.Info("server stopped")
}
