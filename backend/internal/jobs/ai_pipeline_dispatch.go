package jobs

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const postSyncDispatchStageAIPipeline = "ai_pipeline"

// Dispatch status constants — single source of truth for ai_pipeline_dispatches lifecycle.
const (
	DispatchStatusPending                = "pending"
	DispatchStatusRecalculationRunning   = "recalculation_running"
	DispatchStatusRecalculationCompleted = "recalculation_completed"
	DispatchStatusEnqueuePending         = "enqueue_pending"
	DispatchStatusEnqueued               = "enqueued"
	DispatchStatusPipelineStarted        = "pipeline_started"
	DispatchStatusRetryPending           = "retry_pending"
	DispatchStatusCompleted              = "completed"
	DispatchStatusFailed                 = "failed"
)

// DispatchConfig holds bounded retry/recovery settings (env-driven).
type DispatchConfig struct {
	MaxAttempts      int32
	BaseBackoff      time.Duration
	MaxBackoff       time.Duration
	EnqueuedTimeout  time.Duration
	StartedTimeout   time.Duration
	LeaseDuration    time.Duration
	EnqueueStuckWait time.Duration
	RecoveryBatch    int32
}

func LoadDispatchConfig() DispatchConfig {
	started := envDuration("AI_PIPELINE_DISPATCH_STARTED_TIMEOUT", 60*time.Minute)
	return DispatchConfig{
		MaxAttempts:      int32(envInt("AI_PIPELINE_DISPATCH_MAX_ATTEMPTS", 10)),
		BaseBackoff:      envDuration("AI_PIPELINE_DISPATCH_BASE_BACKOFF", time.Minute),
		MaxBackoff:       envDuration("AI_PIPELINE_DISPATCH_MAX_BACKOFF", time.Hour),
		EnqueuedTimeout:  envDuration("AI_PIPELINE_DISPATCH_ENQUEUED_TIMEOUT", 15*time.Minute),
		StartedTimeout:   started,
		LeaseDuration:    envDuration("AI_PIPELINE_DISPATCH_LEASE_DURATION", started),
		EnqueueStuckWait: envDuration("AI_PIPELINE_DISPATCH_ENQUEUE_STUCK_WAIT", 2*time.Minute),
		RecoveryBatch:    int32(envInt("AI_PIPELINE_DISPATCH_RECOVERY_BATCH_SIZE", 100)),
	}
}

// DispatchBackoff returns bounded exponential backoff for attempt number (1-based).
func DispatchBackoff(attempt int32, cfg DispatchConfig) time.Duration {
	if attempt <= 0 {
		return cfg.BaseBackoff
	}
	exp := float64(attempt - 1)
	if exp > 30 {
		exp = 30
	}
	mult := math.Pow(2, exp)
	d := time.Duration(float64(cfg.BaseBackoff) * mult)
	if d > cfg.MaxBackoff {
		return cfg.MaxBackoff
	}
	return d
}

func SellerAIPipelineTaskID(sellerAccountID, syncJobID int64) string {
	return fmt.Sprintf("seller-ai-pipeline:%d:%d", sellerAccountID, syncJobID)
}

func dispatchRecalculationDone(status string) bool {
	switch status {
	case DispatchStatusRecalculationCompleted,
		DispatchStatusEnqueuePending,
		DispatchStatusEnqueued,
		DispatchStatusPipelineStarted,
		DispatchStatusRetryPending,
		DispatchStatusCompleted:
		return true
	default:
		return false
	}
}

func dispatchTerminal(status string) bool {
	return status == DispatchStatusCompleted || status == DispatchStatusFailed
}

// AIPipelineDispatchService coordinates durable dispatch lifecycle transitions.
type AIPipelineDispatchService struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
	cfg     DispatchConfig
	log     *zap.Logger
}

func NewAIPipelineDispatchService(q *dbgen.Queries, cfg DispatchConfig, log *zap.Logger) *AIPipelineDispatchService {
	if log == nil {
		log = zap.NewNop()
	}
	return &AIPipelineDispatchService{queries: q, cfg: cfg, log: log}
}

func (s *AIPipelineDispatchService) EnsureDispatch(ctx context.Context, sellerAccountID, syncJobID int64) (dbgen.AiPipelineDispatch, error) {
	if s == nil || s.queries == nil {
		return dbgen.AiPipelineDispatch{}, fmt.Errorf("dispatch service not configured")
	}
	return s.queries.UpsertAIPipelineDispatch(ctx, dbgen.UpsertAIPipelineDispatchParams{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
		MetricDate:      pgtype.Date{},
		Stage:           postSyncDispatchStageAIPipeline,
		Status:          DispatchStatusPending,
		TaskID:          pgtype.Text{},
		LastError:       pgtype.Text{},
	})
}

// BeginHeavyRecalculation atomically claims recalculation work. Returns skip=true when another worker owns it or it is already done.
func (s *AIPipelineDispatchService) BeginHeavyRecalculation(ctx context.Context, dispatch dbgen.AiPipelineDispatch) (dbgen.AiPipelineDispatch, bool, error) {
	if dispatchRecalculationDone(dispatch.Status) || dispatchTerminal(dispatch.Status) {
		return dispatch, true, nil
	}
	if dispatch.Status == DispatchStatusFailed {
		return dispatch, true, nil
	}
	if dispatch.Status == DispatchStatusRecalculationRunning {
		return dispatch, true, nil
	}
	updated, err := s.queries.BeginAIPipelineDispatchRecalculation(ctx, dispatch.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			refreshed, rerr := s.queries.GetAIPipelineDispatchByID(ctx, dispatch.ID)
			if rerr != nil {
				return dispatch, true, rerr
			}
			if dispatchRecalculationDone(refreshed.Status) || refreshed.Status == DispatchStatusRecalculationRunning {
				return refreshed, true, nil
			}
			return refreshed, false, fmt.Errorf("dispatch not ready for recalculation: status=%s", refreshed.Status)
		}
		return dispatch, false, err
	}
	return updated, false, nil
}

func (s *AIPipelineDispatchService) CompleteHeavyRecalculation(ctx context.Context, dispatchID int64) (dbgen.AiPipelineDispatch, error) {
	row, err := s.queries.MarkAIPipelineDispatchRecalculationCompleted(ctx, dispatchID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.queries.GetAIPipelineDispatchByID(ctx, dispatchID)
		}
		return dbgen.AiPipelineDispatch{}, err
	}
	return row, nil
}

// EnqueuePipelineTask performs guarded enqueue with deterministic task ID and duplicate-as-success semantics.
func (s *AIPipelineDispatchService) EnqueuePipelineTask(
	ctx context.Context,
	dispatch dbgen.AiPipelineDispatch,
	enqueuer func(ctx context.Context, sellerAccountID, syncJobID int64) error,
) (dbgen.AiPipelineDispatch, error) {
	if dispatchTerminal(dispatch.Status) {
		return dispatch, nil
	}
	if dispatch.Status == DispatchStatusFailed {
		return dispatch, nil
	}
	if dispatch.Status == DispatchStatusEnqueued || dispatch.Status == DispatchStatusPipelineStarted {
		if dispatch.Status == DispatchStatusEnqueued {
			enqueueErr := enqueuer(ctx, dispatch.SellerAccountID, dispatch.SyncJobID)
			if enqueueErr == nil || isDuplicateTask(enqueueErr) {
				return s.markEnqueued(ctx, dispatch.ID, dispatch.SellerAccountID, dispatch.SyncJobID)
			}
			if isTerminalEnqueueError(enqueueErr) {
				return s.markFailed(ctx, dispatch.ID, enqueueErr.Error(), "enqueue_terminal")
			}
			retried, rerr := s.queries.MarkAIPipelineDispatchPipelineRetry(ctx, dbgen.MarkAIPipelineDispatchPipelineRetryParams{
				ID:            dispatch.ID,
				LastError:     pgtype.Text{String: enqueueErr.Error(), Valid: true},
				LastErrorCode: pgtype.Text{String: "stale_enqueued", Valid: true},
				NextAttemptAt: pgTimestamptz(time.Now().UTC()),
			})
			if rerr != nil {
				return dispatch, enqueueErr
			}
			return s.EnqueuePipelineTask(ctx, retried, enqueuer)
		}
		return dispatch, nil
	}

	var prepared dbgen.AiPipelineDispatch
	var err error
	switch dispatch.Status {
	case DispatchStatusEnqueuePending:
		prepared = dispatch
	case DispatchStatusRecalculationCompleted, DispatchStatusRetryPending:
		prepared, err = s.queries.BeginAIPipelineDispatchEnqueue(ctx, dispatch.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				refreshed, rerr := s.reloadDispatch(ctx, dispatch)
				if rerr != nil {
					return dispatch, rerr
				}
				return s.EnqueuePipelineTask(ctx, refreshed, enqueuer)
			}
			return dispatch, err
		}
	default:
		refreshed, rerr := s.reloadDispatch(ctx, dispatch)
		if rerr != nil {
			return dispatch, rerr
		}
		return s.EnqueuePipelineTask(ctx, refreshed, enqueuer)
	}

	if prepared.Attempts > s.cfg.MaxAttempts {
		return s.markFailed(ctx, prepared.ID, "dispatch max attempts exceeded", "max_attempts_exceeded")
	}

	enqueueErr := enqueuer(ctx, prepared.SellerAccountID, prepared.SyncJobID)
	if enqueueErr != nil {
		if isDuplicateTask(enqueueErr) {
			return s.markEnqueued(ctx, prepared.ID, prepared.SellerAccountID, prepared.SyncJobID)
		}
		if isTerminalEnqueueError(enqueueErr) {
			return s.markFailed(ctx, prepared.ID, enqueueErr.Error(), "enqueue_terminal")
		}
		return s.markEnqueueRetry(ctx, prepared, enqueueErr)
	}
	return s.markEnqueued(ctx, prepared.ID, prepared.SellerAccountID, prepared.SyncJobID)
}

func (s *AIPipelineDispatchService) markEnqueued(ctx context.Context, dispatchID, sellerAccountID, syncJobID int64) (dbgen.AiPipelineDispatch, error) {
	row, err := s.queries.MarkAIPipelineDispatchEnqueued(ctx, dbgen.MarkAIPipelineDispatchEnqueuedParams{
		ID:     dispatchID,
		TaskID: pgtype.Text{String: SellerAIPipelineTaskID(sellerAccountID, syncJobID), Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.queries.GetAIPipelineDispatchByID(ctx, dispatchID)
		}
		return dbgen.AiPipelineDispatch{}, err
	}
	return row, nil
}

func (s *AIPipelineDispatchService) markEnqueueRetry(ctx context.Context, dispatch dbgen.AiPipelineDispatch, cause error) (dbgen.AiPipelineDispatch, error) {
	if dispatch.Attempts >= s.cfg.MaxAttempts {
		return s.markFailed(ctx, dispatch.ID, cause.Error(), "max_attempts_exceeded")
	}
	next := time.Now().UTC().Add(DispatchBackoff(dispatch.Attempts, s.cfg))
	row, err := s.queries.MarkAIPipelineDispatchEnqueueRetry(ctx, dbgen.MarkAIPipelineDispatchEnqueueRetryParams{
		ID:            dispatch.ID,
		LastError:     pgtype.Text{String: cause.Error(), Valid: true},
		LastErrorCode: pgtype.Text{String: "enqueue_retryable", Valid: true},
		NextAttemptAt: pgTimestamptz(next),
	})
	if err != nil {
		return dispatch, cause
	}
	return row, cause
}

func (s *AIPipelineDispatchService) markFailed(ctx context.Context, dispatchID int64, message, code string) (dbgen.AiPipelineDispatch, error) {
	return s.queries.MarkAIPipelineDispatchFailed(ctx, dbgen.MarkAIPipelineDispatchFailedParams{
		ID:            dispatchID,
		LastError:     pgtype.Text{String: message, Valid: true},
		LastErrorCode: pgtype.Text{String: code, Valid: true},
	})
}

func (s *AIPipelineDispatchService) reloadDispatch(ctx context.Context, dispatch dbgen.AiPipelineDispatch) (dbgen.AiPipelineDispatch, error) {
	if dispatch.ID > 0 {
		return s.queries.GetAIPipelineDispatchByID(ctx, dispatch.ID)
	}
	return s.queries.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
		SyncJobID: dispatch.SyncJobID,
		Stage:     dispatch.Stage,
	})
}

// BeginPipelineExecution marks dispatch pipeline_started before expensive AI work.
func (s *AIPipelineDispatchService) BeginPipelineExecution(ctx context.Context, syncJobID int64, pipelineRunID int64) (dbgen.AiPipelineDispatch, error) {
	var runID pgtype.Int8
	if pipelineRunID > 0 {
		runID = pgtype.Int8{Int64: pipelineRunID, Valid: true}
	}
	row, err := s.queries.BeginAIPipelineDispatchPipelineStarted(ctx, dbgen.BeginAIPipelineDispatchPipelineStartedParams{
		SyncJobID:     syncJobID,
		Stage:         postSyncDispatchStageAIPipeline,
		PipelineRunID: runID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.queries.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
				SyncJobID: syncJobID,
				Stage:     postSyncDispatchStageAIPipeline,
			})
		}
		return dbgen.AiPipelineDispatch{}, err
	}
	return row, nil
}

func (s *AIPipelineDispatchService) LinkPipelineRun(ctx context.Context, dispatchID, pipelineRunID int64) error {
	if dispatchID <= 0 || pipelineRunID <= 0 {
		return nil
	}
	_, err := s.queries.UpdateAIPipelineDispatchPipelineRunID(ctx, dbgen.UpdateAIPipelineDispatchPipelineRunIDParams{
		ID:            dispatchID,
		PipelineRunID: pgtype.Int8{Int64: pipelineRunID, Valid: true},
	})
	return err
}

func (s *AIPipelineDispatchService) CompleteDispatch(ctx context.Context, syncJobID, pipelineRunID int64) (dbgen.AiPipelineDispatch, error) {
	var runID pgtype.Int8
	if pipelineRunID > 0 {
		runID = pgtype.Int8{Int64: pipelineRunID, Valid: true}
	}
	row, err := s.queries.MarkAIPipelineDispatchCompleted(ctx, dbgen.MarkAIPipelineDispatchCompletedParams{
		SyncJobID:     syncJobID,
		PipelineRunID: runID,
		Stage:         postSyncDispatchStageAIPipeline,
	})
	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		return s.queries.GetAIPipelineDispatchBySyncAndStage(ctx, dbgen.GetAIPipelineDispatchBySyncAndStageParams{
			SyncJobID: syncJobID,
			Stage:     postSyncDispatchStageAIPipeline,
		})
	}
	return row, err
}

func (s *AIPipelineDispatchService) MarkPipelineRetry(ctx context.Context, dispatch dbgen.AiPipelineDispatch, cause error, code string, leaseToken string) (dbgen.AiPipelineDispatch, error) {
	if dispatch.PipelineAttempts >= s.cfg.MaxAttempts {
		return s.markFailed(ctx, dispatch.ID, cause.Error(), "max_attempts_exceeded")
	}
	next := time.Now().UTC().Add(DispatchBackoff(dispatch.PipelineAttempts, s.cfg))
	var lease pgtype.Text
	if leaseToken != "" {
		lease = pgtype.Text{String: leaseToken, Valid: true}
	}
	return s.queries.MarkAIPipelineDispatchPipelineRetry(ctx, dbgen.MarkAIPipelineDispatchPipelineRetryParams{
		ID:                  dispatch.ID,
		LastError:           pgtype.Text{String: cause.Error(), Valid: true},
		LastErrorCode:       pgtype.Text{String: code, Valid: true},
		NextAttemptAt:       pgTimestamptz(next),
		ExecutionLeaseToken: lease,
	})
}

// ReconcileStartedDispatch heals published or fresh-running dispatches without re-running AI.
func (s *AIPipelineDispatchService) ReconcileStartedDispatch(ctx context.Context, dispatch dbgen.AiPipelineDispatch) (dbgen.AiPipelineDispatch, bool, error) {
	if dispatch.Status != DispatchStatusPipelineStarted && dispatch.Status != DispatchStatusEnqueued {
		return dispatch, false, nil
	}
	now := time.Now().UTC()
	if dispatch.PipelineRunID.Valid && dispatch.PipelineRunID.Int64 > 0 {
		run, err := s.queries.GetAIPipelineRun(ctx, dispatch.PipelineRunID.Int64)
		if err == nil {
			if run.PublishedAt.Valid {
				completed, cerr := s.CompleteDispatch(ctx, dispatch.SyncJobID, run.ID)
				return completed, true, cerr
			}
			if dispatch.Status == DispatchStatusPipelineStarted && hasFreshLease(dispatch, now) {
				return dispatch, true, nil
			}
			if run.Status == aipipeline.PipelineStatusFailed || run.Status == "failed" {
				if dispatch.PipelineAttempts >= s.cfg.MaxAttempts {
					failed, ferr := s.markFailed(ctx, dispatch.ID, "pipeline run failed", "pipeline_terminal")
					return failed, true, ferr
				}
				// Terminal run failure is handled by recovery/release, not duplicate handlers.
				return dispatch, true, nil
			}
		}
	}
	if dispatch.Status == DispatchStatusEnqueued {
		return dispatch, false, nil
	}
	if hasFreshLease(dispatch, now) {
		return dispatch, true, nil
	}
	return dispatch, false, nil
}

func (s *AIPipelineDispatchService) AdminRetry(ctx context.Context, dispatchID int64) (dbgen.AiPipelineDispatch, error) {
	row, err := s.queries.AdminRetryAIPipelineDispatch(ctx, dispatchID)
	if err != nil {
		return dbgen.AiPipelineDispatch{}, err
	}
	return row, nil
}

func isTerminalEnqueueError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "invalid seller") ||
		strings.Contains(msg, "invalid payload") ||
		strings.Contains(msg, "not configured")
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func envInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// AdminDispatchDiagnostics is the admin-facing dispatch lifecycle view.
type AdminDispatchDiagnostics struct {
	DispatchID               int64  `json:"dispatch_id"`
	SellerAccountID          int64  `json:"seller_account_id"`
	SyncJobID                int64  `json:"sync_job_id"`
	Stage                    string `json:"stage"`
	Status                   string `json:"status"`
	RecalculationCompletedAt string `json:"recalculation_completed_at,omitempty"`
	EnqueueRequestedAt       string `json:"enqueue_requested_at,omitempty"`
	EnqueuedAt               string `json:"enqueued_at,omitempty"`
	PipelineStartedAt        string `json:"pipeline_started_at,omitempty"`
	PipelineRunID            int64  `json:"pipeline_run_id,omitempty"`
	Attempts                 int32  `json:"attempts"`
	LastAttemptAt            string `json:"last_attempt_at,omitempty"`
	NextAttemptAt            string `json:"next_attempt_at,omitempty"`
	LastError                string `json:"last_error,omitempty"`
	LastErrorCode            string `json:"last_error_code,omitempty"`
	CompletedAt              string `json:"completed_at,omitempty"`
	FailedAt                 string `json:"failed_at,omitempty"`
	TaskID                   string `json:"task_id,omitempty"`
}

func BuildAdminDispatchDiagnostics(row dbgen.AiPipelineDispatch) AdminDispatchDiagnostics {
	out := AdminDispatchDiagnostics{
		DispatchID:      row.ID,
		SellerAccountID: row.SellerAccountID,
		SyncJobID:       row.SyncJobID,
		Stage:           row.Stage,
		Status:          row.Status,
		Attempts:        row.Attempts,
	}
	if row.PipelineRunID.Valid {
		out.PipelineRunID = row.PipelineRunID.Int64
	}
	if row.TaskID.Valid {
		out.TaskID = row.TaskID.String
	}
	if row.RecalculationCompletedAt.Valid {
		out.RecalculationCompletedAt = row.RecalculationCompletedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.EnqueueRequestedAt.Valid {
		out.EnqueueRequestedAt = row.EnqueueRequestedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.EnqueuedAt.Valid {
		out.EnqueuedAt = row.EnqueuedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.PipelineStartedAt.Valid {
		out.PipelineStartedAt = row.PipelineStartedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.LastAttemptAt.Valid {
		out.LastAttemptAt = row.LastAttemptAt.Time.UTC().Format(time.RFC3339)
	}
	if row.NextAttemptAt.Valid {
		out.NextAttemptAt = row.NextAttemptAt.Time.UTC().Format(time.RFC3339)
	}
	if row.LastError.Valid {
		out.LastError = row.LastError.String
	}
	if row.LastErrorCode.Valid {
		out.LastErrorCode = row.LastErrorCode.String
	}
	if row.CompletedAt.Valid {
		out.CompletedAt = row.CompletedAt.Time.UTC().Format(time.RFC3339)
	}
	if row.FailedAt.Valid {
		out.FailedAt = row.FailedAt.Time.UTC().Format(time.RFC3339)
	}
	return out
}
