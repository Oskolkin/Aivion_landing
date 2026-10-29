//go:build asynq_integration || integration

package unifiedanalysis_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// postSyncFallbackMetricDate mirrors the '1970-01-01'::date COALESCE fallback in
// GetLatestAvailableDashboardMetricDateBySellerAccountID: a fresh test seller with no
// daily_account_metrics/daily_sku_metrics rows always resolves to this date, so the
// unified run created by the auto-run trigger is anchored here.
const postSyncFallbackMetricDate = "1970-01-01"

// loadNoActionsAnalysisOutputForMetricDate loads the shared no_actions fixture (see
// loadNoActionsAnalysisOutput in queue_integration_test.go) and additionally overrides
// analysis_meta.metric_date so it matches the metric date the run was actually created
// with -- ValidateOutput rejects (publication_blocked) any mismatch between the two.
func loadNoActionsAnalysisOutputForMetricDate(t *testing.T, packID, packHash, metricDate string) []byte {
	t.Helper()
	raw := loadNoActionsAnalysisOutput(t, packID, packHash)
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	out.AnalysisMeta.MetricDate = metricDate
	outBytes, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return outBytes
}

// --- Stage 5 acceptance: post-sync trigger fakes (mirrors jobs.recalculate_after_sync_test.go
// fakes, duplicated here because the jobs package's postSync* interfaces are unexported —
// Go interface satisfaction is structural, so these local fakes are still accepted by
// jobs.NewRecalculateAfterSyncHandler). ---

type postSyncFakeAccount struct{}

func (f *postSyncFakeAccount) RebuildDailyAccountMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	return 1, nil
}

type postSyncFakeSKU struct{}

func (f *postSyncFakeSKU) RebuildDailySKUMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error) {
	return 1, nil
}

type postSyncFakeAlerts struct{}

func (f *postSyncFakeAlerts) RunForAccountWithType(ctx context.Context, sellerAccountID int64, asOfDate time.Time, runType alerts.RunType) (alerts.RunForAccountSummary, error) {
	return alerts.RunForAccountSummary{RunID: 1, Status: alerts.RunStatusCompleted}, nil
}

// postSyncLatestMetricDate mirrors cmd/worker/main.go's resolvePostSyncMetricDate: the
// COALESCE in the underlying query always resolves to a date (falling back to
// 1970-01-01 for a seller with no metrics yet), so this never blocks a fresh test seller.
func postSyncLatestMetricDate(ctx context.Context, q *dbgen.Queries, sellerAccountID int64) (string, error) {
	latest, err := q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return "", err
	}
	if !latest.Valid || latest.Time.Year() < 1970 {
		return "", nil
	}
	return latest.Time.UTC().Format("2006-01-02"), nil
}

// buildUnifiedAutoRunTrigger mirrors the Stage 5 pilot wiring installed on
// RecalculateAfterSyncHandler by cmd/worker/main.go: EffectiveSettings -> CreateRun ->
// EnqueueStart. It is additive to the legacy pipeline and a no-op whenever the seller's
// resolved EffectiveAutoRun is false (unconfigured sellers, legacy mode, or rollout
// disabled globally/per-seller).
func buildUnifiedAutoRunTrigger(
	q *dbgen.Queries,
	uaSvc *unifiedanalysis.Service,
	enqueue jobs.UnifiedAnalysisEnqueue,
	global publication.GlobalFlags,
) func(ctx context.Context, sellerAccountID int64) error {
	return func(ctx context.Context, sellerAccountID int64) error {
		var settingsPtr *dbgen.SellerAnalysisSetting
		row, err := q.GetSellerAnalysisSettings(ctx, sellerAccountID)
		if err == nil {
			settingsPtr = &row
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("load seller analysis settings: %w", err)
		}
		settings := publication.EffectiveSettings(settingsPtr, global)
		if !settings.EffectiveAutoRun {
			return nil
		}
		metricKey, err := postSyncLatestMetricDate(ctx, q, sellerAccountID)
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
		run, err := uaSvc.CreateRunWithOptions(ctx, sellerAccountID, metricDate, unifiedanalysis.CreateRunOptions{
			Force:   false,
			Trigger: unifiedanalysis.TriggerPostSync,
		})
		if err != nil {
			if errors.Is(err, unifiedanalysis.ErrActiveRunExists) {
				return nil
			}
			return fmt.Errorf("create unified analysis run: %w", err)
		}
		return enqueue.EnqueueStart(ctx, run.ID)
	}
}

func upsertPostSyncSellerSettings(t *testing.T, q *dbgen.Queries, sellerID int64, mode string, autoRun, autoPublish, legacyFallback bool) {
	t.Helper()
	if _, err := q.UpsertSellerAnalysisSettings(context.Background(), dbgen.UpsertSellerAnalysisSettingsParams{
		SellerAccountID:       sellerID,
		AnalysisMode:          mode,
		AutoRunEnabled:        autoRun,
		AutoPublishEnabled:    autoPublish,
		LegacyFallbackEnabled: legacyFallback,
		UpdatedBy:             "test",
	}); err != nil {
		t.Fatalf("upsert seller analysis settings: %v", err)
	}
}

func insertPostSyncSyncJob(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID int64, status string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, created_at)
VALUES ($1, 'incremental_sync', $2, now())
RETURNING id`, sellerID, status).Scan(&id); err != nil {
		t.Fatalf("insert sync job: %v", err)
	}
	return id
}

func latestUnifiedRunID(pool *pgxpool.Pool, sellerID int64) (int64, bool) {
	var id int64
	err := pool.QueryRow(context.Background(), `
SELECT id FROM ai_pipeline_runs WHERE seller_account_id=$1 AND mode=$2 ORDER BY id DESC LIMIT 1`,
		sellerID, unifiedanalysis.ModeUnifiedAnalysis).Scan(&id)
	if err != nil {
		return 0, false
	}
	return id, true
}

func countUnifiedRuns(t *testing.T, pool *pgxpool.Pool, sellerID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM ai_pipeline_runs WHERE seller_account_id=$1 AND mode=$2`,
		sellerID, unifiedanalysis.ModeUnifiedAnalysis).Scan(&n); err != nil {
		t.Fatalf("count unified runs: %v", err)
	}
	return n
}

func countUnifiedPublications(t *testing.T, pool *pgxpool.Pool, sellerID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM unified_analysis_publications WHERE seller_account_id=$1`, sellerID).Scan(&n); err != nil {
		t.Fatalf("count unified publications: %v", err)
	}
	return n
}

// postSyncHarness wires a RecalculateAfterSyncHandler (queries=nil, so the parent sync
// guard/dispatch stages are skipped and Handle runs straight to the unified auto-run
// trigger) together with a full Stage 3/4/5 asynq pipeline: Start/Poll/Validate/Publish
// handlers backed by a fake provider that completes with a validated no_actions output.
type postSyncHarness struct {
	sellerID int64
	q        *dbgen.Queries
	repo     *unifiedanalysis.Repository
	uaSvc    *unifiedanalysis.Service
	fake     *provider.FakeProvider
	enqueue  *jobs.AsynqUnifiedAnalysisEnqueue
	handler  *jobs.RecalculateAfterSyncHandler
}

func setupPostSyncHarness(t *testing.T, pool *pgxpool.Pool, redisCfg jobs.RedisConfig, global publication.GlobalFlags) *postSyncHarness {
	t.Helper()
	clearAsynqQueues(t, redisCfg)

	client := jobs.NewAsynqClient(redisCfg)
	t.Cleanup(func() { _ = client.Close() })

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	repo := unifiedanalysis.NewRepository(q)
	artifacts := unifiedanalysis.NewArtifactStore(repo)

	packID := fmt.Sprintf("pack:%d:post-sync-acceptance", sellerID)
	zip, packHash := unifiedanalysis.MustMinimalValidPackZip(packID, sellerID, "2026-07-08")
	stubKey := fmt.Sprintf("ai-analysis-packs/seller-%d/post-sync/analysis_pack.zip", sellerID)
	stub := &stubPackBuilder{
		result: &pack.BuildResult{
			SchemaVersion: contract.AnalysisPackSchemaVersionV1,
			PackID:        packID,
			PackHash:      packHash,
			ZipBytes:      zip,
			Validation:    pack.ValidationReport{Valid: true, CompressedBytes: int64(len(zip)), UncompressedBytes: int64(len(zip))},
		},
		key: stubKey,
	}
	store := &memoryPackStore{}
	store.Put(stubKey, zip)

	fake := provider.NewFakeProvider(provider.FakeScenarioQueuedThenCompleted)
	fake.SetSmokePackID(packID)
	fake.SetAnalysisOutputJSON(loadNoActionsAnalysisOutputForMetricDate(t, packID, packHash, postSyncFallbackMetricDate))

	uaSvc := unifiedanalysis.NewServiceWithStore(repo, stub, fake, artifacts, store, analysisQueueConfig())

	enqueue := &jobs.AsynqUnifiedAnalysisEnqueue{Client: client}
	uaHandler := jobs.NewUnifiedAnalysisHandler(uaSvc, enqueue, zap.NewNop())
	uaHandler.EnableAutoPublish(q, global)

	pubSvc := publication.NewService(pool, q, artifacts, publication.DefaultPublicationPolicy(), global)
	pubHandler := jobs.NewUnifiedAnalysisPublishHandler(pubSvc, zap.NewNop())

	mux := asynq.NewServeMux()
	jobs.RegisterUnifiedAnalysisHandlers(mux, uaHandler)
	jobs.RegisterUnifiedAnalysisPublishHandler(mux, pubHandler)
	_, _ = startEmbeddedWorker(t, redisCfg, mux)

	handler := jobs.NewRecalculateAfterSyncHandler(
		&postSyncFakeAccount{}, &postSyncFakeSKU{}, nil, &postSyncFakeAlerts{},
		nil, nil, zap.NewNop(), nil,
	)
	handler.SetUnifiedAutoRunTrigger(buildUnifiedAutoRunTrigger(q, uaSvc, enqueue, global))

	return &postSyncHarness{
		sellerID: sellerID,
		q:        q,
		repo:     repo,
		uaSvc:    uaSvc,
		fake:     fake,
		enqueue:  enqueue,
		handler:  handler,
	}
}

func TestPostSyncUnifiedModeE2E(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	global := publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true}
	h := setupPostSyncHarness(t, pool, redisCfg, global)
	upsertPostSyncSellerSettings(t, h.q, h.sellerID, string(publication.AnalysisModeUnified), true, true, true)

	task, err := jobs.NewRecalculateAfterSyncTask(h.sellerID, 1)
	if err != nil {
		t.Fatalf("build post-sync task: %v", err)
	}
	if err := h.handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	runID, ok := latestUnifiedRunID(pool, h.sellerID)
	if !ok {
		t.Fatal("expected unified analysis run to be created for unified-mode seller")
	}

	final := waitRun(t, h.repo, runID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusPublished
	})
	if final.Status != unifiedanalysis.StatusPublished {
		t.Fatalf("run status=%q want published", final.Status)
	}

	if got := countUnifiedRuns(t, pool, h.sellerID); got != 1 {
		t.Fatalf("unified runs=%d want 1", got)
	}
	if got := countUnifiedPublications(t, pool, h.sellerID); got != 1 {
		t.Fatalf("unified publications=%d want 1", got)
	}

	var currentPub, compatRun int64
	if err := pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_unified_publications WHERE seller_account_id=$1`, h.sellerID).Scan(&currentPub); err != nil {
		t.Fatalf("unified pointer: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, h.sellerID).Scan(&compatRun); err != nil {
		t.Fatalf("compat pointer: %v", err)
	}
	if compatRun != runID {
		t.Fatalf("compat pointer run_id=%d want %d", compatRun, runID)
	}
	if currentPub <= 0 {
		t.Fatal("expected a unified publication pointer")
	}

	pubSvc := publication.NewService(pool, h.q, unifiedanalysis.NewArtifactStore(h.repo), publication.DefaultPublicationPolicy(), global)
	src, err := pubSvc.ResolveCurrentAnalysisSource(ctx, h.sellerID)
	if err != nil {
		t.Fatalf("ResolveCurrentAnalysisSource: %v", err)
	}
	if src.Source != publication.SourceUnified {
		t.Fatalf("source=%q want unified", src.Source)
	}
	if src.PublicationID != currentPub {
		t.Fatalf("source publication_id=%d want %d", src.PublicationID, currentPub)
	}
}

func TestPostSyncLegacyModeNoRun(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	global := publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true}
	h := setupPostSyncHarness(t, pool, redisCfg, global)
	upsertPostSyncSellerSettings(t, h.q, h.sellerID, string(publication.AnalysisModeLegacy), true, true, true)

	task, err := jobs.NewRecalculateAfterSyncTask(h.sellerID, 1)
	if err != nil {
		t.Fatalf("build post-sync task: %v", err)
	}
	if err := h.handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got := countUnifiedRuns(t, pool, h.sellerID); got != 0 {
		t.Fatalf("unified runs=%d want 0 for legacy-mode seller", got)
	}
}

func TestPostSyncShadowModeE2E(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	global := publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true}
	h := setupPostSyncHarness(t, pool, redisCfg, global)
	upsertPostSyncSellerSettings(t, h.q, h.sellerID, string(publication.AnalysisModeShadow), true, true, true)

	task, err := jobs.NewRecalculateAfterSyncTask(h.sellerID, 1)
	if err != nil {
		t.Fatalf("build post-sync task: %v", err)
	}
	if err := h.handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	runID, ok := latestUnifiedRunID(pool, h.sellerID)
	if !ok {
		t.Fatal("expected unified analysis run to be created for shadow-mode seller")
	}
	final := waitRun(t, h.repo, runID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
		return r.Status == unifiedanalysis.StatusPublished
	})
	if final.Status != unifiedanalysis.StatusPublished {
		t.Fatalf("run status=%q want published", final.Status)
	}

	var visibility string
	if err := pool.QueryRow(ctx, `SELECT visibility FROM unified_analysis_publications WHERE run_id=$1`, runID).Scan(&visibility); err != nil {
		t.Fatalf("load publication visibility: %v", err)
	}
	if visibility != string(publication.VisibilityShadow) {
		t.Fatalf("visibility=%q want shadow", visibility)
	}

	// Shadow publish materializes the projection for comparison/QA only: it must never
	// flip the seller's current (visible) unified pointer.
	var pointerCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM seller_current_unified_publications WHERE seller_account_id=$1`, h.sellerID).Scan(&pointerCount); err != nil {
		t.Fatalf("count unified pointer: %v", err)
	}
	if pointerCount != 0 {
		t.Fatalf("shadow publish must not set the seller's current unified pointer, count=%d", pointerCount)
	}

	pubSvc := publication.NewService(pool, h.q, unifiedanalysis.NewArtifactStore(h.repo), publication.DefaultPublicationPolicy(), global)
	src, err := pubSvc.ResolveCurrentAnalysisSource(ctx, h.sellerID)
	if err != nil {
		t.Fatalf("ResolveCurrentAnalysisSource: %v", err)
	}
	if src.Source == publication.SourceUnified {
		t.Fatalf("source=%q; shadow-mode seller must never resolve to unified", src.Source)
	}
}

func TestPostSyncKillSwitches(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	type killSwitchCase struct {
		name            string
		global          publication.GlobalFlags
		sellerAutoRun   bool
		sellerAutoPub   bool
		wantRun         bool
		wantPublication bool
	}

	cases := []killSwitchCase{
		{
			name:          "global_rollout_disabled",
			global:        publication.GlobalFlags{RolloutEnabled: false, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true},
			sellerAutoRun: true, sellerAutoPub: true,
			wantRun: false, wantPublication: false,
		},
		{
			name:          "global_auto_run_disabled",
			global:        publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: false, AutoPublishEnabled: true, LegacyFallbackEnabled: true},
			sellerAutoRun: true, sellerAutoPub: true,
			wantRun: false, wantPublication: false,
		},
		{
			name:          "global_auto_publish_disabled",
			global:        publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: false, LegacyFallbackEnabled: true},
			sellerAutoRun: true, sellerAutoPub: true,
			wantRun: true, wantPublication: false,
		},
		{
			name:          "seller_auto_run_disabled",
			global:        publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true},
			sellerAutoRun: false, sellerAutoPub: true,
			wantRun: false, wantPublication: false,
		},
		{
			name:          "seller_auto_publish_disabled",
			global:        publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true},
			sellerAutoRun: true, sellerAutoPub: false,
			wantRun: true, wantPublication: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := setupPostSyncHarness(t, pool, redisCfg, tc.global)
			upsertPostSyncSellerSettings(t, h.q, h.sellerID, string(publication.AnalysisModeUnified), tc.sellerAutoRun, tc.sellerAutoPub, true)

			task, err := jobs.NewRecalculateAfterSyncTask(h.sellerID, 1)
			if err != nil {
				t.Fatalf("build post-sync task: %v", err)
			}
			if err := h.handler.Handle(ctx, task.Payload()); err != nil {
				t.Fatalf("Handle: %v", err)
			}

			if !tc.wantRun {
				if got := countUnifiedRuns(t, pool, h.sellerID); got != 0 {
					t.Fatalf("unified runs=%d want 0", got)
				}
				return
			}

			runID, ok := latestUnifiedRunID(pool, h.sellerID)
			if !ok {
				t.Fatal("expected unified analysis run to be created")
			}
			final := waitRun(t, h.repo, runID, 60*time.Second, func(r *unifiedanalysis.Run) bool {
				return r.Status == unifiedanalysis.StatusPublicationReady || r.Status == unifiedanalysis.StatusPublished
			})
			if tc.wantPublication {
				if final.Status != unifiedanalysis.StatusPublished {
					t.Fatalf("status=%q want published", final.Status)
				}
			} else if final.Status != unifiedanalysis.StatusPublicationReady {
				t.Fatalf("status=%q want publication_ready (auto-publish disabled)", final.Status)
			}

			pubCount := countUnifiedPublications(t, pool, h.sellerID)
			if tc.wantPublication && pubCount != 1 {
				t.Fatalf("unified publications=%d want 1", pubCount)
			}
			if !tc.wantPublication && pubCount != 0 {
				t.Fatalf("unified publications=%d want 0 (auto-publish disabled)", pubCount)
			}
		})
	}
}

func TestPostSyncFailedParentSyncNoRun(t *testing.T) {
	pool := asynqDBPool(t)
	ctx := context.Background()

	sellerID := createQueueSeller(t, pool)
	q := dbgen.New(pool)
	syncJobID := insertPostSyncSyncJob(t, ctx, pool, sellerID, "failed")

	upsertPostSyncSellerSettings(t, q, sellerID, string(publication.AnalysisModeUnified), true, true, true)

	// queries is non-nil here (unlike setupPostSyncHarness), so
	// NewRecalculateAfterSyncHandler wires a real SQLCParentSyncGuard that rejects this
	// failed parent sync job before any downstream work -- including the unified
	// auto-run trigger -- runs.
	handler := jobs.NewRecalculateAfterSyncHandler(
		&postSyncFakeAccount{}, &postSyncFakeSKU{}, nil, &postSyncFakeAlerts{},
		q, nil, zap.NewNop(), nil,
	)
	triggerCalled := false
	handler.SetUnifiedAutoRunTrigger(func(ctx context.Context, sellerAccountID int64) error {
		triggerCalled = true
		return nil
	})

	task, err := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	if err != nil {
		t.Fatalf("build post-sync task: %v", err)
	}
	handleErr := handler.Handle(ctx, task.Payload())
	if handleErr == nil {
		t.Fatal("expected parent sync guard error for a failed parent sync")
	}
	if !errors.Is(handleErr, asynq.SkipRetry) {
		t.Fatalf("expected asynq.SkipRetry (permanent guard failure), got %v", handleErr)
	}
	if triggerCalled {
		t.Fatal("unified auto-run trigger must not run when the parent sync failed")
	}
	if got := countUnifiedRuns(t, pool, sellerID); got != 0 {
		t.Fatalf("unified runs=%d want 0", got)
	}
}

func TestPostSyncPartialOrCancelledSyncNoRun(t *testing.T) {
	pool := asynqDBPool(t)
	ctx := context.Background()

	// sync_jobs_status_check allows: pending|running|completed|failed
	for _, status := range []string{"pending", "running"} {
		status := status
		t.Run(status, func(t *testing.T) {
			sellerID := createQueueSeller(t, pool)
			q := dbgen.New(pool)
			syncJobID := insertPostSyncSyncJob(t, ctx, pool, sellerID, status)
			upsertPostSyncSellerSettings(t, q, sellerID, string(publication.AnalysisModeUnified), true, true, true)

			handler := jobs.NewRecalculateAfterSyncHandler(
				&postSyncFakeAccount{}, &postSyncFakeSKU{}, nil, &postSyncFakeAlerts{},
				q, nil, zap.NewNop(), nil,
			)
			triggerCalled := false
			handler.SetUnifiedAutoRunTrigger(func(ctx context.Context, sellerAccountID int64) error {
				triggerCalled = true
				return nil
			})

			task, err := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
			if err != nil {
				t.Fatalf("build post-sync task: %v", err)
			}
			handleErr := handler.Handle(ctx, task.Payload())
			if handleErr == nil {
				t.Fatalf("expected parent sync guard error for status=%s", status)
			}
			if triggerCalled {
				t.Fatalf("unified auto-run must not run for parent status=%s", status)
			}
			if got := countUnifiedRuns(t, pool, sellerID); got != 0 {
				t.Fatalf("unified runs=%d want 0 for status=%s", got, status)
			}
		})
	}
}

func TestPostSyncDuplicateCompletionIdempotent(t *testing.T) {
	pool := asynqDBPool(t)
	redisCfg := asynqRedisConfig(t)
	ctx := context.Background()

	global := publication.GlobalFlags{RolloutEnabled: true, AutoRunEnabled: true, AutoPublishEnabled: true, LegacyFallbackEnabled: true}
	h := setupPostSyncHarness(t, pool, redisCfg, global)
	upsertPostSyncSellerSettings(t, h.q, h.sellerID, string(publication.AnalysisModeUnified), true, true, true)

	task, err := jobs.NewRecalculateAfterSyncTask(h.sellerID, 1)
	if err != nil {
		t.Fatalf("build post-sync task: %v", err)
	}
	if err := h.handler.Handle(ctx, task.Payload()); err != nil {
		t.Fatalf("Handle (first): %v", err)
	}
	if got := countUnifiedRuns(t, pool, h.sellerID); got != 1 {
		t.Fatalf("unified runs after first Handle=%d want 1", got)
	}

	// A second post-sync completion for the same seller (e.g. a retried/duplicated
	// asynq task) must not create a second unified analysis run: CreateRun(force=false)
	// finds the still-active run and the trigger swallows ErrActiveRunExists as a no-op.
	task2, err := jobs.NewRecalculateAfterSyncTask(h.sellerID, 2)
	if err != nil {
		t.Fatalf("build post-sync task 2: %v", err)
	}
	if err := h.handler.Handle(ctx, task2.Payload()); err != nil {
		t.Fatalf("Handle (second): %v", err)
	}

	if got := countUnifiedRuns(t, pool, h.sellerID); got != 1 {
		t.Fatalf("unified runs after duplicate completion=%d want 1", got)
	}

	runID, ok := latestUnifiedRunID(pool, h.sellerID)
	if !ok {
		t.Fatal("expected the single unified analysis run to still exist")
	}
	// Full publish path is covered by TestPostSyncUnifiedModeE2E; this test's
	// contract is CreateRun idempotency under duplicate post-sync completion.
	run, err := h.repo.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run == nil || run.ID != runID {
		t.Fatalf("unexpected run %+v", run)
	}
	if got := countUnifiedRuns(t, pool, h.sellerID); got != 1 {
		t.Fatalf("unified runs still=%d want 1", got)
	}
}
