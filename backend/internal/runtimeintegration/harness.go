//go:build integration

package runtimeintegration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ingestion"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ordersync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ozonenrichmentsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productanalyticssync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stocksync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testozon"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	testEncryptionKey = "01234567890123456789012345678901"
	testRedisDB       = 15
)

// Harness wires worker handlers for unified runtime integration tests.
type Harness struct {
	t           *testing.T
	Pool        *pgxpool.Pool
	Queries     *dbgen.Queries
	Cfg         *config.Config
	Orchestrate *ingestion.OrchestrationService
	AsynqClient *asynq.Client
	Server      *asynq.Server
	MetricDate  time.Time
	cancel      context.CancelFunc
	serverDone  chan error
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "migrations"))
}

func redisAddr() string {
	if v := os.Getenv("TEST_REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6379"
}

func setupUnifiedRuntimeEnv(t *testing.T, sellerURL, performanceURL string) {
	t.Helper()
	t.Setenv("AI_PROVIDER_MODE", "fake")
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "true")
	t.Setenv("LEGACY_AI_POST_SYNC_ENABLED", "false")
	t.Setenv("ACTIONS_AI_FIRST", "true")
	t.Setenv("APP_ENV", "test")
	t.Setenv("BACKEND_PORT", "8081")
	t.Setenv("WORKER_METRICS_PORT", "9091")
	t.Setenv("MIGRATIONS_PATH", migrationsDir(t))
	t.Setenv("REDIS_ADDR", redisAddr())
	t.Setenv("REDIS_DB", fmt.Sprintf("%d", testRedisDB))
	t.Setenv("S3_ENDPOINT", getenv("TEST_S3_ENDPOINT", "localhost:19000"))
	t.Setenv("S3_ACCESS_KEY", getenv("TEST_S3_ACCESS_KEY", "minio"))
	t.Setenv("S3_SECRET_KEY", getenv("TEST_S3_SECRET_KEY", "minio123"))
	t.Setenv("S3_BUCKET_RAW", getenv("TEST_S3_BUCKET_RAW", "raw-payloads"))
	t.Setenv("S3_BUCKET_EXPORTS", getenv("TEST_S3_BUCKET_EXPORTS", "exports"))
	t.Setenv("S3_BUCKET_ARTIFACTS", getenv("TEST_S3_BUCKET_ARTIFACTS", "artifacts"))
	t.Setenv("AUTH_COOKIE_NAME", "session_token")
	t.Setenv("AUTH_SESSION_TTL_HOURS", "168")
	t.Setenv("JWT_SECRET", "test-jwt-secret-32-bytes-minimum!!")
	t.Setenv("ENCRYPTION_KEY", testEncryptionKey)
	t.Setenv("OPENAI_MODEL", "gpt-4.1-mini")
	t.Setenv("OPENAI_TIMEOUT_SECONDS", "30")
	t.Setenv("OPENAI_MAX_RETRIES", "2")
	t.Setenv("OPENAI_API_KEY", "fake")
	t.Setenv("WORKER_SCHEDULER_ENABLED", "false")
	t.Setenv("OZON_SELLER_API_BASE_URL", sellerURL)
	t.Setenv("OZON_PERFORMANCE_API_BASE_URL", performanceURL)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func flushRedisDB(t *testing.T, addr string, db int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	defer client.Close()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis unavailable at %s db=%d: %v", addr, db, err)
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis db %d: %v", db, err)
	}
}

func openMigratedPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.RunMigrations(sqlDB, migrationsDir(t)); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return pool
}

// NewHarness builds a worker-like runtime wired to fake Ozon APIs and in-process Asynq.
func NewHarness(t *testing.T, dsn string, metricDate time.Time) *Harness {
	t.Helper()
	if metricDate.IsZero() {
		metricDate = time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	}

	fake := testozon.Start(testozon.Options{MetricDate: metricDate})
	t.Cleanup(fake.Seller.Close)
	t.Cleanup(fake.Performance.Close)

	setupUnifiedRuntimeEnv(t, fake.Seller.URL, fake.Performance.URL)
	t.Setenv("DATABASE_URL", dsn)
	flushRedisDB(t, redisAddr(), testRedisDB)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	ctx := context.Background()
	postgres, err := db.New(ctx, dsn)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(postgres.Close)

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
		t.Fatalf("storage.New: %v", err)
	}

	logger := zap.NewNop()
	registry := prometheus.NewRegistry()
	m := metrics.New(registry)

	ozonService, err := ozon.NewService(postgres.Pool, cfg.Auth.EncryptionKey, cfg.Ozon.PerformanceAuthBaseURL, cfg.Ozon.DisconnectRetentionDays)
	if err != nil {
		t.Fatalf("ozon.NewService: %v", err)
	}

	rawPayloadService := rawpayloads.NewService(postgres.Pool, s3Client, cfg.S3.BucketRaw)
	productsImporter := productsync.NewService(postgres.Pool, ozonService, rawPayloadService)
	ordersImporter := ordersync.NewService(postgres.Pool, ozonService, rawPayloadService)
	stocksImporter := stocksync.NewService(postgres.Pool, ozonService, rawPayloadService)
	adsImporter := adsync.NewService(postgres.Pool, ozonService, rawPayloadService)

	redisCfg := jobs.RedisConfig{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	server := jobs.NewAsynqServer(redisCfg)
	asynqClient := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = asynqClient.Close() })

	coordinator := jobs.NewOzonSyncCoordinatorHandler(postgres.Pool, asynqClient, logger)
	workerQueries := dbgen.New(postgres.Pool)

	resolvePostSyncMetricDate := func(ctx context.Context, sellerAccountID int64) (string, error) {
		latest, err := workerQueries.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
		if err != nil {
			return "", err
		}
		if !latest.Valid || latest.Time.Year() < 1970 {
			return "", nil
		}
		return latest.Time.UTC().Format("2006-01-02"), nil
	}

	enqueuePostSyncRecalc := func(ctx context.Context, sellerAccountID, syncJobID int64) error {
		task, err := jobs.NewRecalculateAfterSyncTask(sellerAccountID, syncJobID)
		if err != nil {
			return err
		}
		_, err = asynqClient.Enqueue(task, asynq.TaskID(fmt.Sprintf("post-sync:%d:recalc", syncJobID)), asynq.Unique(48*time.Hour))
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
		_, err = asynqClient.Enqueue(task, asynq.TaskID(fmt.Sprintf("seller-ai-pipeline:%d:%d", sellerAccountID, syncJobID)), asynq.Unique(48*time.Hour))
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
			metricKey = metricDate.UTC().Format("2006-01-02")
		}
		task, err := jobs.NewOzonEnrichmentP0Task(sellerAccountID, syncJobID, metricKey, "p0")
		if err != nil {
			return err
		}
		_, err = asynqClient.Enqueue(task, asynq.TaskID(fmt.Sprintf("ozon-enrichment:%d", syncJobID)), asynq.Unique(48*time.Hour))
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

	handler := jobs.NewHandler(logger)
	mux := jobs.NewServeMux(handler, logger, m)
	mux.HandleFunc(jobs.TaskTypeOzonSyncCoordinator, func(ctx context.Context, t *asynq.Task) error {
		return coordinator.Handle(ctx, t.Payload())
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

	var enqueueAfterRecalc func(context.Context, int64, int64) error
	if aipipeline.SellerAIPipelineEnabled() {
		enqueueAfterRecalc = enqueueSellerAIPipeline
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

	if err := aipipeline.ValidatePipelineConfig(); err != nil {
		t.Fatalf("ValidatePipelineConfig: %v", err)
	}
	pipelineRuntime, err := aipipeline.BuildRuntimeService(cfg, postgres.Pool)
	if err != nil {
		t.Fatalf("BuildRuntimeService: %v", err)
	}
	pipelineHandler := jobs.NewRunSellerAIPipelineHandler(pipelineRuntime, workerQueries, postgres.Pool, logger, m)
	mux.HandleFunc(jobs.TaskTypeRunSellerAIPipeline, func(ctx context.Context, t *asynq.Task) error {
		return pipelineHandler.Handle(ctx, t.Payload())
	})
	recoveryHandler := jobs.NewAIPipelineDispatchRecoveryHandler(postgres.Pool, workerQueries, 100, enqueueSellerAIPipeline, logger)
	mux.HandleFunc(jobs.TaskTypeAIPipelineDispatchRecovery, func(ctx context.Context, t *asynq.Task) error {
		return recoveryHandler.Handle(ctx, t.Payload())
	})

	runCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		if err := jobs.RunAsynqUntilContext(runCtx, server, mux); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
		close(errCh)
	}()

	h := &Harness{
		t:           t,
		Pool:        postgres.Pool,
		Queries:     dbgen.New(postgres.Pool),
		Cfg:         cfg,
		Orchestrate: ingestion.NewOrchestrationService(postgres.Pool, asynqClient),
		AsynqClient: asynqClient,
		Server:      server,
		MetricDate:  metricDate.UTC(),
		cancel:      cancel,
		serverDone:  errCh,
	}
	t.Cleanup(func() {
		cancel()
		if err := <-h.serverDone; err != nil {
			t.Errorf("asynq server: %v", err)
		}
	})
	return h
}

// SeedSeller creates user, seller, ozon connection, and dashboard anchor metric.
func (h *Harness) SeedSeller(t *testing.T) int64 {
	t.Helper()
	ctx := context.Background()
	email := fmt.Sprintf("runtime-%d@test.invalid", time.Now().UnixNano())
	u, err := h.Queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:        email,
		PasswordHash: "x",
		Status:       "active",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	sa, err := h.Queries.CreateSellerAccount(ctx, dbgen.CreateSellerAccountParams{
		UserID: u.ID,
		Name:   "runtime-it",
		Status: "active",
	})
	if err != nil {
		t.Fatalf("CreateSellerAccount: %v", err)
	}
	if err := devseed.SeedOzonConnectionMock(ctx, h.Queries, sa.ID, time.Now().UTC(), testEncryptionKey); err != nil {
		t.Fatalf("SeedOzonConnectionMock: %v", err)
	}
	if err := seedDashboardAnchor(ctx, h.Pool, sa.ID, h.MetricDate); err != nil {
		t.Fatalf("seedDashboardAnchor: %v", err)
	}
	return sa.ID
}

func seedDashboardAnchor(ctx context.Context, pool *pgxpool.Pool, sellerID int64, metricDate time.Time) error {
	q := dbgen.New(pool)
	var revenue pgtype.Numeric
	if err := revenue.Scan("2500.00"); err != nil {
		return err
	}
	_, err := q.UpsertDailyAccountMetric(ctx, dbgen.UpsertDailyAccountMetricParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate.UTC(), Valid: true},
		Revenue:         revenue,
		OrdersCount:     12,
		ReturnsCount:    1,
		CancelCount:     0,
		UnitsSold:       pgtype.Int4{Int32: 14, Valid: true},
	})
	return err
}

// WaitForPipelinePublished blocks until sync/import/enrichment/pipeline complete or times out.
func (h *Harness) WaitForPipelinePublished(t *testing.T, sellerID, syncJobID int64, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.dumpState(t, sellerID, syncJobID)
			t.Fatalf("timeout waiting for unified runtime flow")
		case <-ticker.C:
			if h.flowComplete(ctx, sellerID, syncJobID) {
				return
			}
		}
	}
}

func (h *Harness) flowComplete(ctx context.Context, sellerID, syncJobID int64) bool {
	syncJob, err := h.Queries.GetSyncJobByID(ctx, syncJobID)
	if err != nil || syncJob.Status != "completed" {
		return false
	}
	imports, err := h.Queries.ListImportJobsBySyncJobID(ctx, syncJobID)
	if err != nil || len(imports) != 4 {
		return false
	}
	for _, ij := range imports {
		if ij.Status != "completed" {
			return false
		}
	}
	run, err := h.Queries.GetLatestOzonEnrichmentSyncRunBySellerAndDate(ctx, dbgen.GetLatestOzonEnrichmentSyncRunBySellerAndDateParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: h.MetricDate.UTC(), Valid: true},
	})
	if err != nil {
		return false
	}
	switch run.Status {
	case "completed", "completed_partial":
	default:
		return false
	}
	pipeline, err := h.Queries.GetLatestPublishedAIPipelineRunBySeller(ctx, sellerID)
	if err != nil || !pipeline.PublishedAt.Valid {
		return false
	}
	return true
}

func (h *Harness) dumpState(t *testing.T, sellerID, syncJobID int64) {
	t.Helper()
	ctx := context.Background()
	syncJob, _ := h.Queries.GetSyncJobByID(ctx, syncJobID)
	imports, _ := h.Queries.ListImportJobsBySyncJobID(ctx, syncJobID)
	t.Logf("sync_job status=%s", syncJob.Status)
	for _, ij := range imports {
		t.Logf("import domain=%s status=%s err=%v", ij.Domain, ij.Status, ij.ErrorMessage)
	}
	run, err := h.Queries.GetLatestOzonEnrichmentSyncRunBySellerAndDate(ctx, dbgen.GetLatestOzonEnrichmentSyncRunBySellerAndDateParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: h.MetricDate.UTC(), Valid: true},
	})
	if err != nil {
		t.Logf("enrichment run: %v", err)
	} else {
		t.Logf("enrichment status=%s run_id=%d", run.Status, run.ID)
	}
	pipeline, err := h.Queries.GetLatestPublishedAIPipelineRunBySeller(ctx, sellerID)
	if err != nil {
		t.Logf("pipeline published: %v", err)
	} else {
		t.Logf("pipeline status=%s published=%v", pipeline.Status, pipeline.PublishedAt.Valid)
	}
	if dispatch, derr := h.Queries.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
		SyncJobID: syncJobID,
		Stage:     "ai_pipeline",
	}); derr != nil {
		t.Logf("dispatch: %v", derr)
	} else {
		t.Logf("dispatch status=%s pipeline_run_id=%v pipeline_attempts=%d lease_expires=%v",
			dispatch.Status, dispatch.PipelineRunID, dispatch.PipelineAttempts, dispatch.LeaseExpiresAt)
	}
	inspector := asynq.NewInspector(asynq.RedisClientOpt{Addr: redisAddr(), DB: testRedisDB})
	defer inspector.Close()
	if pending, err := inspector.ListPendingTasks("default"); err == nil {
		for _, task := range pending {
			t.Logf("pending task type=%s id=%s", task.Type, task.ID)
		}
	}
	if active, err := inspector.ListActiveTasks("default"); err == nil {
		for _, task := range active {
			t.Logf("active task type=%s id=%s", task.Type, task.ID)
		}
	}
	if retry, err := inspector.ListRetryTasks("default"); err == nil {
		for _, task := range retry {
			t.Logf("retry task type=%s id=%s err=%s", task.Type, task.ID, task.LastErr)
		}
	}
}

// RecordingEnqueuer records enqueued task types for contract tests.
type RecordingEnqueuer struct {
	mu    sync.Mutex
	Tasks []*asynq.Task
	Inner ingestion.TaskEnqueuer
}

func NewRecordingEnqueuer(inner ingestion.TaskEnqueuer) *RecordingEnqueuer {
	return &RecordingEnqueuer{Inner: inner}
}

func (r *RecordingEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	r.mu.Lock()
	r.Tasks = append(r.Tasks, task)
	r.mu.Unlock()
	if r.Inner == nil {
		return &asynq.TaskInfo{ID: "recorded"}, nil
	}
	return r.Inner.Enqueue(task, opts...)
}

func (r *RecordingEnqueuer) Types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.Tasks))
	for _, task := range r.Tasks {
		out = append(out, task.Type())
	}
	return out
}

var unifiedAllowedTaskTypes = map[string]struct{}{
	jobs.TaskTypeOzonSyncCoordinator:   {},
	jobs.TaskTypeOzonImportProducts:    {},
	jobs.TaskTypeOzonImportOrders:      {},
	jobs.TaskTypeOzonImportStocks:      {},
	jobs.TaskTypeOzonImportAds:         {},
	jobs.TaskTypeOzonEnrichmentP0:      {},
	jobs.TaskTypePostSyncRecalculation: {},
	jobs.TaskTypeRunSellerAIPipeline:   {},
}

func assertUnifiedEnqueueContract(t *testing.T, types []string) {
	t.Helper()
	legacy := jobs.TaskTypeGenerateAIDailySummaries
	for _, typ := range types {
		if typ == legacy {
			t.Fatalf("legacy task %s enqueued in unified mode", typ)
		}
		if _, ok := unifiedAllowedTaskTypes[typ]; !ok {
			t.Fatalf("unexpected task type %s in unified mode", typ)
		}
	}
}
