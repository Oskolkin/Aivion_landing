//go:build worker_restart || integration

package unifiedanalysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/testutil"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Stage 5 acceptance: worker restart recovery (real OS process) ---
//
// These tests exercise cmd/worker's actual startup recovery path -- PlanRecovery ->
// EnqueuePublish and ReconcileAllCurrentPublicationPointers, both invoked once from
// main() before the asynq server starts serving -- by building the worker binary and
// running it as a genuine child process against TEST_DATABASE_URL/TEST_REDIS_ADDR,
// rather than calling unifiedanalysis.Service methods in-process. This is deliberately
// heavier/slower than the asynq_integration acceptance tests in
// acceptance_post_sync_test.go: it is the only way to prove recovery actually runs from
// a cold process start (nothing here is reachable via an already-warm embedded worker).
//
// Local helpers below duplicate small pieces of queue_integration_test.go /
// acceptance_post_sync_test.go under distinct names: this file's build tag
// ("worker_restart || integration") differs from theirs ("asynq_integration ||
// integration"), so under `-tags=worker_restart` (without integration) those files are
// not compiled and their helpers would not exist; under `-tags=integration` all three
// files compile together, so identical helper names would collide.

func restartDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run worker restart acceptance tests")
	}
	testutil.RequireSafeTestDatabaseURL(t, dsn)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("db ping: %v", err)
	}
	return pool
}

func restartRedisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	return addr
}

func restartCreateSeller(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var userID int64
	email := fmt.Sprintf("ua-worker-restart-%d@test.local", time.Now().UnixNano())
	if err := pool.QueryRow(ctx, `
INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var sellerID int64
	if err := pool.QueryRow(ctx, `
INSERT INTO seller_accounts (user_id, name, status) VALUES ($1, $2, 'active') RETURNING id`,
		userID, "ua-worker-restart").Scan(&sellerID); err != nil {
		t.Fatalf("insert seller: %v", err)
	}
	return sellerID
}

func restartUpsertSellerSettings(t *testing.T, q *dbgen.Queries, sellerID int64, mode string, autoRun, autoPublish, legacyFallback bool) {
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

func freeTCPPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}

var (
	workerRestartBinaryOnce sync.Once
	workerRestartBinaryPath string
	workerRestartBinaryErr  error
)

// buildWorkerRestartBinary compiles cmd/worker once per `go test` process (sync.Once)
// and reuses the resulting executable across every TestWorkerRestart* test, so the
// ~10s `go build` cost is paid once rather than per scenario.
func buildWorkerRestartBinary(t *testing.T) string {
	t.Helper()
	workerRestartBinaryOnce.Do(func() {
		dir, err := os.MkdirTemp("", "unified-worker-restart")
		if err != nil {
			workerRestartBinaryErr = fmt.Errorf("mkdir temp dir: %w", err)
			return
		}
		name := "worker"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		workerRestartBinaryPath = filepath.Join(dir, name)
		backendDir, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			workerRestartBinaryErr = fmt.Errorf("resolve backend dir: %w", err)
			return
		}
		cmd := exec.Command("go", "build", "-o", workerRestartBinaryPath, "./cmd/worker")
		cmd.Dir = backendDir
		if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
			workerRestartBinaryErr = fmt.Errorf("go build ./cmd/worker: %w\n%s", buildErr, out)
		}
	})
	if workerRestartBinaryErr != nil {
		t.Fatalf("build worker binary: %v", workerRestartBinaryErr)
	}
	return workerRestartBinaryPath
}

// workerRestartEnv builds the child process environment for cmd/worker: it starts from
// the current process environment (so PATH etc. survive) and overrides every variable
// the worker actually reads at startup (see config.Load/validate and cmd/worker/main.go)
// with fixed, deterministic test values. AI_PROVIDER_MODE=fake keeps the legacy AI
// pipeline (SELLER_AI_PIPELINE_ENABLED defaults true; disabling it trips
// aipipeline.ValidateRuntimeModeConfig's ErrPipelineDisabled at startup) off the real
// OpenAI API without requiring a real key; UNIFIED_ANALYSIS_PROVIDER_ENABLED=true is
// what actually gates the Stage 3-5 wiring these tests exercise. REDIS_DB=15 isolates
// this file's asynq queues from queue_integration_test.go/acceptance_post_sync_test.go,
// which use DB 14.
func workerRestartEnv(dbURL, redisAddr, metricsPort string) []string {
	overrides := map[string]string{
		"APP_ENV":                               "test",
		"BACKEND_PORT":                          "0",
		"WORKER_METRICS_PORT":                   metricsPort,
		"DATABASE_URL":                          dbURL,
		"MIGRATIONS_PATH":                       "./migrations",
		"REDIS_ADDR":                            redisAddr,
		"REDIS_PASSWORD":                        "",
		"REDIS_DB":                              "15",
		"S3_ENDPOINT":                           "127.0.0.1:19000",
		"S3_ACCESS_KEY":                         "minio",
		"S3_SECRET_KEY":                         "minio123",
		"S3_BUCKET_RAW":                         "raw-payloads",
		"S3_BUCKET_EXPORTS":                     "exports",
		"S3_BUCKET_ARTIFACTS":                   "artifacts",
		"S3_USE_SSL":                            "false",
		"JWT_SECRET":                            "replace-me-local",
		"ENCRYPTION_KEY":                        "0123456789abcdef0123456789abcdef",
		"AI_PROVIDER_MODE":                      "fake",
		"OPENAI_API_KEY":                        "test-worker-restart-key",
		"OPENAI_MODEL":                          "gpt-4.1",
		"SELLER_AI_PIPELINE_ENABLED":            "1",
		"ACTIONS_AI_FIRST":                      "1",
		"LEGACY_AI_POST_SYNC_ENABLED":           "0",
		"WORKER_SCHEDULER_ENABLED":              "false",
		"AI_PIPELINE_DISPATCH_RECOVERY_ENABLED": "false",
		"DISCONNECTED_ACCOUNT_CLEANUP_ENABLED":  "false",
		"CLEANUP_ENABLED":                       "false",
		"CHAT_OPENAI_FILE_TTL_HOURS":            "24",

		"UNIFIED_ANALYSIS_PROVIDER_ENABLED":          "true",
		"UNIFIED_ANALYSIS_PROVIDER":                  "openai",
		"UNIFIED_ANALYSIS_MODEL":                     "gpt-4.1",
		"UNIFIED_ANALYSIS_CODE_INTERPRETER_MEMORY":   "4g",
		"UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS":         "16000",
		"UNIFIED_ANALYSIS_PROMPT_PROFILE":            "analysis",
		"UNIFIED_ANALYSIS_OUTPUT_VALIDATION_ENABLED": "true",
		"UNIFIED_ANALYSIS_POLL_INTERVAL_SECONDS":     "1",
		"UNIFIED_ANALYSIS_MAX_POLL_DURATION_SECONDS": "60",
		"UNIFIED_ANALYSIS_HTTP_TIMEOUT_SECONDS":      "10",
		"UNIFIED_ANALYSIS_TRANSPORT_RETRIES":         "0",
		"UNIFIED_ANALYSIS_CLEANUP_ENABLED":           "true",
		"UNIFIED_ANALYSIS_DEBUG_ARTIFACTS":           "true",
		// All rollout flags on: recovery.go's publication_ready/publishing ->
		// RecoveryActionPublish does not itself gate on EffectiveAutoPublish, but
		// publication.Service.PublishRun still requires the seller not be in legacy
		// mode, so these globals plus the test-seeded seller settings (mode=unified)
		// keep the full worker-startup path exercised end to end.
		"UNIFIED_ANALYSIS_ROLLOUT_ENABLED":         "true",
		"UNIFIED_ANALYSIS_AUTO_RUN_ENABLED":        "true",
		"UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED":    "true",
		"UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED": "true",
	}
	base := os.Environ()
	env := make([]string, 0, len(base)+len(overrides))
	for _, kv := range base {
		key := kv
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			key = kv[:idx]
		}
		if _, overridden := overrides[key]; overridden {
			continue
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

type restartWorkerHandle struct {
	cmd    *exec.Cmd
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// startRestartWorker launches the built worker binary as a real child OS process and
// registers a t.Cleanup that force-kills it (workers otherwise run forever serving
// asynq). On test failure it dumps the process's captured stdout/stderr for debugging.
func startRestartWorker(t *testing.T, binPath string, env []string) *restartWorkerHandle {
	t.Helper()
	cmd := exec.Command(binPath)
	cmd.Env = env
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start worker process: %v", err)
	}
	h := &restartWorkerHandle{cmd: cmd, stdout: stdout, stderr: stderr}
	t.Cleanup(func() {
		stopRestartWorker(t, h)
		if t.Failed() {
			t.Logf("worker process stdout:\n%s", stdout.String())
			t.Logf("worker process stderr:\n%s", stderr.String())
		}
	})
	return h
}

func stopRestartWorker(t *testing.T, h *restartWorkerHandle) {
	t.Helper()
	if h == nil || h.cmd == nil || h.cmd.Process == nil {
		return
	}
	_ = h.cmd.Process.Kill()
	_, _ = h.cmd.Process.Wait()
}

func waitForRestartCondition(t *testing.T, timeout time.Duration, cond func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := cond()
		if err != nil {
			lastErr = err
		} else if ok {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("condition not met within %s: %v", timeout, lastErr)
	}
	t.Fatalf("condition not met within %s", timeout)
}

type restartSeededRun struct {
	RunID      int64
	PackID     string
	PackHash   string
	OutputHash string
}

// seedValidatedPublicationReadyRun seeds a run in exactly the state Stage 4
// (ValidateOutput) leaves it in on success: status=publication_ready with a validated
// output artifact and validation-identity artifact already persisted. Mirrors
// queue_integration_test.go's TestAsynqPublishE2E seeding, which is exercised via an
// embedded in-process worker rather than a real OS process.
func seedValidatedPublicationReadyRun(t *testing.T, pool *pgxpool.Pool, q *dbgen.Queries, sellerID int64, metricDate time.Time, trigger string) restartSeededRun {
	t.Helper()
	ctx := context.Background()
	policy := publication.DefaultPublicationPolicy()
	fixturePath := filepath.Join("contract", "testdata", "unified_output_v1.no_actions.json")
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixturePath, err)
	}
	var out contract.UnifiedAnalysisOutputV1
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	packID := fmt.Sprintf("pack:%d:%s:worker-restart", sellerID, metricDate.Format("2006-01-02"))
	packHash := fmt.Sprintf("restart_hash_%d_0123456789abcdef0123456789abcdef", sellerID)
	out.AnalysisMeta.PackID = packID
	out.AnalysisMeta.PackHash = packHash
	out.AnalysisMeta.MetricDate = metricDate.Format("2006-01-02")
	canonical, err := outputvalidator.CanonicalJSON(&out)
	if err != nil {
		t.Fatalf("canonical json: %v", err)
	}
	outputHash := outputvalidator.OutputHash(canonical)

	runRow, err := q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       sellerID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:               trigger,
		Status:                unifiedanalysis.StatusPublicationReady,
		DataPackSchemaVersion: pgtype.Text{String: contract.AnalysisPackSchemaVersionV1, Valid: true},
		DataPackHash:          packHash,
		PipelineVersion:       pgtype.Text{String: "test", Valid: true},
		PromptVersion:         pgtype.Text{String: policy.AcceptedPromptVersion, Valid: true},
		Provider:              pgtype.Text{String: "openai", Valid: true},
		ProviderModel:         pgtype.Text{String: "gpt-test", Valid: true},
		ProviderAttemptCount:  1,
		StartedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateUnifiedAnalysisRun: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_runs SET pack_id=$2, pack_hash=$3, code_interpreter_calls=1, cleanup_status='completed' WHERE id=$1`,
		runRow.ID, packID, packHash); err != nil {
		t.Fatalf("stamp pack: %v", err)
	}
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runRow.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidated,
		Hash:          pgtype.Text{String: outputHash, Valid: true},
		Storage:       "db",
		PayloadJson:   canonical,
	}); err != nil {
		t.Fatalf("validated artifact: %v", err)
	}
	valPayload, _ := json.Marshal(map[string]any{
		"prompt_version": policy.AcceptedPromptVersion,
		"prompt_hash":    policy.AcceptedPromptHash,
		"schema_version": policy.AcceptedSchemaVersion,
		"schema_hash":    policy.AcceptedSchemaHash,
		"output_hash":    outputHash,
		"valid":          true,
	})
	if _, err := q.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
		PipelineRunID: runRow.ID,
		Kind:          unifiedanalysis.ArtifactUnifiedAnalysisOutputValidation,
		Hash:          pgtype.Text{String: "validation", Valid: true},
		Storage:       "db",
		PayloadJson:   valPayload,
	}); err != nil {
		t.Fatalf("validation artifact: %v", err)
	}
	artifacts := unifiedanalysis.NewArtifactStore(unifiedanalysis.NewRepository(q))
	facts := unifiedanalysis.BuildFixtureDashboardFacts(sellerID, metricDate, packHash, &out)
	if err := artifacts.SaveDashboardFacts(ctx, runRow.ID, facts); err != nil {
		t.Fatalf("SaveDashboardFacts: %v", err)
	}
	return restartSeededRun{RunID: runRow.ID, PackID: packID, PackHash: packHash, OutputHash: outputHash}
}

func runStatus(t *testing.T, pool *pgxpool.Pool, runID int64) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM ai_pipeline_runs WHERE id=$1`, runID).Scan(&status); err != nil {
		t.Fatalf("load run status: %v", err)
	}
	return status
}

// TestWorkerRestartPublicationReady seeds a publication_ready run (Stage 4's terminal
// success state) directly in Postgres -- no asynq task is ever enqueued for it -- then
// starts a real `cmd/worker` OS process and asserts its startup recovery scan
// (PlanRecovery -> RecoveryActionPublish -> EnqueuePublish) discovers and publishes it
// without any test-side enqueue call.
func TestWorkerRestartPublicationReady(t *testing.T) {
	pool := restartDBPool(t)
	redisAddr := restartRedisAddr(t)
	ctx := context.Background()
	binPath := buildWorkerRestartBinary(t)

	q := dbgen.New(pool)
	sellerID := restartCreateSeller(t, pool)
	restartUpsertSellerSettings(t, q, sellerID, string(publication.AnalysisModeUnified), true, true, true)

	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	seeded := seedValidatedPublicationReadyRun(t, pool, q, sellerID, metricDate, "worker_restart_publication_ready")

	env := workerRestartEnv(os.Getenv("TEST_DATABASE_URL"), redisAddr, freeTCPPort(t))
	startRestartWorker(t, binPath, env)

	waitForRestartCondition(t, 60*time.Second, func() (bool, error) {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_runs WHERE id=$1`, seeded.RunID).Scan(&status); err != nil {
			return false, err
		}
		return status == unifiedanalysis.StatusPublished, nil
	})

	var pubCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM unified_analysis_publications WHERE run_id=$1`, seeded.RunID).Scan(&pubCount); err != nil {
		t.Fatalf("count publications: %v", err)
	}
	if pubCount != 1 {
		t.Fatalf("publications for run=%d want 1, got %d", seeded.RunID, pubCount)
	}

	var pubStatus, visibility string
	if err := pool.QueryRow(ctx, `SELECT status, visibility FROM unified_analysis_publications WHERE run_id=$1`, seeded.RunID).Scan(&pubStatus, &visibility); err != nil {
		t.Fatalf("load publication: %v", err)
	}
	if pubStatus != string(publication.StatusPublished) || visibility != string(publication.VisibilityVisible) {
		t.Fatalf("publication status=%s visibility=%s, want published/visible", pubStatus, visibility)
	}

	var compatRun int64
	if err := pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&compatRun); err != nil {
		t.Fatalf("load compat pointer: %v", err)
	}
	if compatRun != seeded.RunID {
		t.Fatalf("compat pointer run_id=%d want %d", compatRun, seeded.RunID)
	}
	t.Logf("worker restart publication_ready recovery run_id=%d final_status=%s", seeded.RunID, runStatus(t, pool, seeded.RunID))
}

// TestWorkerRestartCommittedRunMismatch reproduces a crash between
// MarkUnifiedAnalysisPublicationPublished (the publication row is committed
// status=published) and the run's own SetAIPipelinePublishedAt transition, by letting a
// first worker process publish normally, killing it, then manually resetting the run's
// status back to "publishing" (as if that final write never landed) before starting a
// second worker process. classifyRecoveryActions still schedules RecoveryActionPublish
// for status=publishing, so PublishRun's hasExisting/StatusPublished branch must detect
// the run/publication mismatch and finalize the run to published without creating (or
// re-projecting into) a second unified_analysis_publications row.
func TestWorkerRestartCommittedRunMismatch(t *testing.T) {
	pool := restartDBPool(t)
	redisAddr := restartRedisAddr(t)
	ctx := context.Background()
	binPath := buildWorkerRestartBinary(t)

	q := dbgen.New(pool)
	sellerID := restartCreateSeller(t, pool)
	restartUpsertSellerSettings(t, q, sellerID, string(publication.AnalysisModeUnified), true, true, true)

	metricDate := time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC)
	seeded := seedValidatedPublicationReadyRun(t, pool, q, sellerID, metricDate, "worker_restart_commit_mismatch")

	firstEnv := workerRestartEnv(os.Getenv("TEST_DATABASE_URL"), redisAddr, freeTCPPort(t))
	first := startRestartWorker(t, binPath, firstEnv)
	waitForRestartCondition(t, 60*time.Second, func() (bool, error) {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_runs WHERE id=$1`, seeded.RunID).Scan(&status); err != nil {
			return false, err
		}
		return status == unifiedanalysis.StatusPublished, nil
	})
	stopRestartWorker(t, first)

	var pubIDBefore int64
	if err := pool.QueryRow(ctx, `SELECT id FROM unified_analysis_publications WHERE run_id=$1`, seeded.RunID).Scan(&pubIDBefore); err != nil {
		t.Fatalf("load publication before mismatch: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE ai_pipeline_runs SET status='publishing' WHERE id=$1`, seeded.RunID); err != nil {
		t.Fatalf("force run back to publishing: %v", err)
	}
	if got := runStatus(t, pool, seeded.RunID); got != unifiedanalysis.StatusPublishing {
		t.Fatalf("run status after fault injection=%q want publishing", got)
	}

	secondEnv := workerRestartEnv(os.Getenv("TEST_DATABASE_URL"), redisAddr, freeTCPPort(t))
	startRestartWorker(t, binPath, secondEnv)

	waitForRestartCondition(t, 60*time.Second, func() (bool, error) {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_runs WHERE id=$1`, seeded.RunID).Scan(&status); err != nil {
			return false, err
		}
		return status == unifiedanalysis.StatusPublished, nil
	})

	var pubCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM unified_analysis_publications WHERE run_id=$1`, seeded.RunID).Scan(&pubCount); err != nil {
		t.Fatalf("count publications after re-recovery: %v", err)
	}
	if pubCount != 1 {
		t.Fatalf("publications for run=%d after re-recovery want 1, got %d", seeded.RunID, pubCount)
	}

	var pubIDAfter int64
	if err := pool.QueryRow(ctx, `SELECT id FROM unified_analysis_publications WHERE run_id=$1`, seeded.RunID).Scan(&pubIDAfter); err != nil {
		t.Fatalf("load publication after mismatch: %v", err)
	}
	if pubIDAfter != pubIDBefore {
		t.Fatalf("publication id changed across restart recovery: before=%d after=%d, want re-recovery to reuse the same committed row", pubIDBefore, pubIDAfter)
	}
}

// TestWorkerRestartPointerReconcile corrupts the legacy/compat
// seller_current_ai_snapshots pointer (pointing it at an unrelated run for the same
// seller) after a normal publish, then asserts a fresh worker process's startup
// ReconcileAllCurrentPublicationPointers call repairs it back to agree with the
// authoritative seller_current_unified_publications pointer -- without ever touching
// the unified pointer itself (see reconcile.go doc).
func TestWorkerRestartPointerReconcile(t *testing.T) {
	pool := restartDBPool(t)
	redisAddr := restartRedisAddr(t)
	ctx := context.Background()
	binPath := buildWorkerRestartBinary(t)

	q := dbgen.New(pool)
	sellerID := restartCreateSeller(t, pool)
	restartUpsertSellerSettings(t, q, sellerID, string(publication.AnalysisModeUnified), true, true, true)

	metricDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	seeded := seedValidatedPublicationReadyRun(t, pool, q, sellerID, metricDate, "worker_restart_pointer_reconcile")

	firstEnv := workerRestartEnv(os.Getenv("TEST_DATABASE_URL"), redisAddr, freeTCPPort(t))
	first := startRestartWorker(t, binPath, firstEnv)
	waitForRestartCondition(t, 60*time.Second, func() (bool, error) {
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM ai_pipeline_runs WHERE id=$1`, seeded.RunID).Scan(&status); err != nil {
			return false, err
		}
		return status == unifiedanalysis.StatusPublished, nil
	})
	stopRestartWorker(t, first)

	var compatRunBefore int64
	if err := pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&compatRunBefore); err != nil {
		t.Fatalf("load compat pointer before corruption: %v", err)
	}
	if compatRunBefore != seeded.RunID {
		t.Fatalf("compat pointer before corruption=%d want %d", compatRunBefore, seeded.RunID)
	}

	// A second, otherwise-unrelated run row for the same seller gives the corrupted
	// compat pointer a value that still satisfies seller_current_ai_snapshots'
	// pipeline_run_id -> ai_pipeline_runs(id) FK, mimicking drift from a manual/legacy
	// write (see reconcile.go doc) rather than a value the DB would reject outright.
	bogusRun, err := q.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:      sellerID,
		MetricDate:           pgtype.Date{Time: metricDate, Valid: true},
		Mode:                 unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:              "worker_restart_pointer_reconcile_bogus",
		Status:               unifiedanalysis.StatusCreated,
		DataPackHash:         "bogus-hash",
		ProviderAttemptCount: 0,
		StartedAt:            pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create bogus run: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE seller_current_ai_snapshots SET pipeline_run_id=$2 WHERE seller_account_id=$1`, sellerID, bogusRun.ID); err != nil {
		t.Fatalf("corrupt compat pointer: %v", err)
	}
	if got := mustCompatPointer(t, pool, sellerID); got != bogusRun.ID {
		t.Fatalf("compat pointer after corruption=%d want %d", got, bogusRun.ID)
	}

	secondEnv := workerRestartEnv(os.Getenv("TEST_DATABASE_URL"), redisAddr, freeTCPPort(t))
	startRestartWorker(t, binPath, secondEnv)

	waitForRestartCondition(t, 30*time.Second, func() (bool, error) {
		var compatRun int64
		if err := pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&compatRun); err != nil {
			return false, err
		}
		return compatRun == seeded.RunID, nil
	})

	var currentPubRunID int64
	if err := pool.QueryRow(ctx, `
SELECT p.run_id FROM seller_current_unified_publications s
JOIN unified_analysis_publications p ON p.id = s.publication_id
WHERE s.seller_account_id=$1`, sellerID).Scan(&currentPubRunID); err != nil {
		t.Fatalf("load unified pointer run: %v", err)
	}
	if currentPubRunID != seeded.RunID {
		t.Fatalf("reconcile must never touch the authoritative unified pointer: run=%d want %d", currentPubRunID, seeded.RunID)
	}
}

func mustCompatPointer(t *testing.T, pool *pgxpool.Pool, sellerID int64) int64 {
	t.Helper()
	var runID int64
	if err := pool.QueryRow(context.Background(), `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=$1`, sellerID).Scan(&runID); err != nil {
		t.Fatalf("load compat pointer: %v", err)
	}
	return runID
}
