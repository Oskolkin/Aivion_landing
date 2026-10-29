package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type RunSellerAIPipelineHandler struct {
	pipeline    aipipeline.RuntimeRunner
	queries     *dbgen.Queries
	pool        *pgxpool.Pool
	parentGuard ParentSyncGuard
	dispatch    *AIPipelineDispatchService
	log         *zap.Logger
	metrics     *metrics.Metrics
}

func NewRunSellerAIPipelineHandler(pipeline aipipeline.RuntimeRunner, queries *dbgen.Queries, pool *pgxpool.Pool, log *zap.Logger, jobMetrics *metrics.Metrics) *RunSellerAIPipelineHandler {
	var parentGuard ParentSyncGuard
	if queries != nil {
		parentGuard = NewSQLCParentSyncGuard(queries, log, jobMetrics, "ai_pipeline")
	}
	return &RunSellerAIPipelineHandler{
		pipeline:    pipeline,
		queries:     queries,
		pool:        pool,
		parentGuard: parentGuard,
		dispatch:    NewAIPipelineDispatchService(queries, LoadDispatchConfig(), log).WithPool(pool),
		log:         log,
		metrics:     jobMetrics,
	}
}

func (h *RunSellerAIPipelineHandler) Handle(ctx context.Context, taskPayload []byte) error {
	var payload RunSellerAIPipelinePayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal seller ai pipeline payload: %w", err)
	}
	if payload.SellerAccountID <= 0 {
		return fmt.Errorf("invalid seller_account_id")
	}
	if payload.SyncJobID <= 0 {
		return fmt.Errorf("invalid sync_job_id")
	}

	requestedMetricDate, err := parseOptionalMetricDate(payload.MetricDate)
	if err != nil {
		return err
	}

	var validated ValidatedParentSync
	if h.parentGuard != nil {
		validated, err = h.parentGuard.RequireCompletedParentSync(ctx, payload.SyncJobID, payload.SellerAccountID, requestedMetricDate)
		if err != nil {
			return MapParentSyncGuardErrorToAsynq(err)
		}
	}
	metricDate := validated.MetricDate
	if metricDate.IsZero() && requestedMetricDate != nil {
		metricDate = *requestedMetricDate
	}

	if h.dispatch == nil || payload.SyncJobID <= 0 {
		return fmt.Errorf("dispatch service is not configured")
	}

	dispatch, err := h.dispatch.EnsureDispatch(ctx, payload.SellerAccountID, payload.SyncJobID)
	if err != nil {
		return fmt.Errorf("load dispatch: %w", err)
	}
	if dispatchTerminal(dispatch.Status) {
		return nil
	}

	reconciled, done, rerr := h.dispatch.ReconcileStartedDispatch(ctx, dispatch)
	if rerr != nil {
		return fmt.Errorf("reconcile dispatch: %w", rerr)
	}
	if done {
		return nil
	}
	dispatch = reconciled

	leaseToken := uuid.NewString()
	claim, err := h.dispatch.ClaimPipelineExecution(ctx, dispatch.ID, leaseToken, metricDate, ClaimSourceHandler)
	if err != nil {
		return fmt.Errorf("claim pipeline execution: %w", err)
	}
	h.recordClaimMetric(claim)
	if claim.RetryNotDue || claim.TerminalFailed {
		return nil
	}
	if claim.IdempotentSuccess() {
		return nil
	}
	if !claim.Claimed {
		return fmt.Errorf("dispatch claim rejected without idempotent result")
	}

	trigger := payload.Trigger
	if trigger == "" {
		trigger = "post_sync"
	}

	leaseGuard := dispatchLeaseGuard{
		svc:        h.dispatch,
		dispatchID: claim.DispatchID,
		leaseToken: claim.LeaseToken,
	}

	result, err := h.pipeline.RunExistingPipeline(ctx, aipipeline.ExistingPipelineInput{
		SellerAccountID: payload.SellerAccountID,
		SyncJobID:       payload.SyncJobID,
		MetricDate:      metricDate,
		Trigger:         trigger,
		Force:           payload.Force,
		DispatchID:      claim.DispatchID,
		PipelineRunID:   claim.PipelineRunID,
		LeaseGuard:      leaseGuard,
	})
	if err != nil {
		h.log.Error("seller ai pipeline failed",
			zap.Int64("dispatch_id", claim.DispatchID),
			zap.Int64("pipeline_run_id", claim.PipelineRunID),
			zap.Int64("seller_account_id", payload.SellerAccountID),
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.String("lease_token_hash", LeaseHash(claim.LeaseToken)),
			zap.Error(err),
		)
		refreshed, rerr := h.queries.GetAIPipelineDispatchByID(ctx, claim.DispatchID)
		if rerr != nil {
			return fmt.Errorf("reload dispatch for retry: %w", rerr)
		}
		if _, merr := h.dispatch.MarkPipelineRetry(ctx, refreshed, err, "pipeline_retryable", claim.LeaseToken); merr != nil {
			return fmt.Errorf("mark pipeline retry: %w", merr)
		}
		return nil
	}

	if result.Published {
		if _, cerr := h.dispatch.CompleteDispatch(ctx, payload.SyncJobID, result.PipelineRunID); cerr != nil {
			return fmt.Errorf("complete dispatch: %w", cerr)
		}
	} else if result.Status == aipipeline.PipelineStatusFailed {
		refreshed, rerr := h.queries.GetAIPipelineDispatchByID(ctx, claim.DispatchID)
		if rerr != nil {
			return fmt.Errorf("reload dispatch for failed retry: %w", rerr)
		}
		if _, merr := h.dispatch.MarkPipelineRetry(ctx, refreshed, fmt.Errorf("pipeline status=%s", result.Status), "pipeline_failed", claim.LeaseToken); merr != nil {
			return fmt.Errorf("mark pipeline failed retry: %w", merr)
		}
		return nil
	}

	h.log.Info("seller ai pipeline completed",
		zap.Int64("dispatch_id", claim.DispatchID),
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int64("pipeline_run_id", result.PipelineRunID),
		zap.String("status", result.Status),
		zap.Bool("reused", result.Reused),
		zap.Bool("published", result.Published),
	)
	return nil
}

func (h *RunSellerAIPipelineHandler) recordClaimMetric(claim PipelineExecutionClaim) {
	if h.metrics == nil {
		return
	}
	result := "claimed"
	switch {
	case claim.Claimed:
		result = "claimed"
	case claim.AlreadyRunning:
		result = "already_running"
	case claim.AlreadyCompleted:
		result = "already_completed"
	case claim.AlreadyPublished:
		result = "already_published"
	case claim.AwaitingRecovery:
		result = "awaiting_recovery"
	case claim.RetryNotDue:
		result = "retry_not_due"
	case claim.TerminalFailed:
		result = "terminal_failed"
	default:
		result = "unknown"
	}
	h.metrics.AIPipelineExecutionClaimTotal.WithLabelValues(result).Inc()
}
