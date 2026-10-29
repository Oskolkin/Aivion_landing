package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"go.uber.org/zap"
)

// postSyncMetricsLookbackDays is the inclusive window (UTC calendar days) for rebuilding
// daily_account_metrics and daily_sku_metrics after ingestion. Keeps work bounded vs full history.
const postSyncMetricsLookbackDays = 30

// postSyncAccountMetrics rebuilds account-level aggregates for a date range.
type postSyncAccountMetrics interface {
	RebuildDailyAccountMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error)
}

// postSyncSKUMetrics rebuilds SKU-level aggregates for a date range.
type postSyncSKUMetrics interface {
	RebuildDailySKUMetricsForDateRange(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (int, error)
}

// postSyncAlerts runs the alerts engine for an account (fingerprint / idempotency live inside alerts.Service).
type postSyncAlerts interface {
	RunForAccountWithType(ctx context.Context, sellerAccountID int64, asOfDate time.Time, runType alerts.RunType) (alerts.RunForAccountSummary, error)
}

// postSyncFunnelAnalytics syncs Ozon /v1/analytics/data into product_analytics_daily.
// Implementations must not fail post-sync on 401/403/402 (access/premium restrictions).
type postSyncFunnelAnalytics interface {
	SyncFunnelForSellerRange(ctx context.Context, sellerAccountID int64, fromDate, toDate time.Time) error
}

// RecalculateAfterSyncHandler runs bounded downstream work after a sync_job completes:
// account metrics -> SKU metrics -> funnel analytics -> alerts -> (optional) enqueue AI daily summaries.
//
// AI daily summaries are generated asynchronously and are idempotent per seller + metric_date.
type RecalculateAfterSyncHandler struct {
	account            postSyncAccountMetrics
	sku                postSyncSKUMetrics
	funnel             postSyncFunnelAnalytics
	alerts             postSyncAlerts
	queries            *dbgen.Queries
	parentGuard        ParentSyncGuard
	dispatch           *AIPipelineDispatchService
	enqueueAISummaries func(ctx context.Context, sellerAccountID, syncJobID int64) error
	// unifiedAutoRun is optional (nil-safe): see SetUnifiedAutoRunTrigger. It is
	// additive to the legacy dispatch above and never blocks/fails post-sync
	// recalculation — errors are logged and swallowed.
	unifiedAutoRun func(ctx context.Context, sellerAccountID int64) error
	log            *zap.Logger
}

// SetUnifiedAutoRunTrigger wires a Stage 5 unified-analysis auto-run hook, called
// best-effort at the end of Handle for every post-sync recalculation. trigger is
// expected to resolve the seller's effective rollout settings itself (see
// publication.EffectiveSettings) and no-op when EffectiveAutoRun is false, so it is
// safe to call unconditionally here regardless of legacy pipeline mode:
//   - legacy mode: only the legacy pipeline runs (trigger no-ops, EffectiveAutoRun=false).
//   - shadow mode: legacy pipeline (if SELLER_AI_PIPELINE_ENABLED) + unified auto-run.
//   - unified mode: unified auto-run; legacy pipeline still runs as today when
//     SELLER_AI_PIPELINE_ENABLED (kept for Stage 5 pilot fallback), unified is additive.
//
// Not calling this (unifiedAutoRun stays nil) fully preserves pre-Stage-5 behavior.
func (h *RecalculateAfterSyncHandler) SetUnifiedAutoRunTrigger(trigger func(ctx context.Context, sellerAccountID int64) error) {
	if h == nil {
		return
	}
	h.unifiedAutoRun = trigger
}

// NewRecalculateAfterSyncHandler wires downstream services used after ingestion completes.
// enqueueAISummaries may be nil (no-op). funnel may be nil (skipped). jobMetrics may be nil.
func NewRecalculateAfterSyncHandler(
	account postSyncAccountMetrics,
	sku postSyncSKUMetrics,
	funnel postSyncFunnelAnalytics,
	alertsSvc postSyncAlerts,
	queries *dbgen.Queries,
	enqueueAISummaries func(ctx context.Context, sellerAccountID, syncJobID int64) error,
	log *zap.Logger,
	jobMetrics *metrics.Metrics,
) *RecalculateAfterSyncHandler {
	var dispatch *AIPipelineDispatchService
	var parentGuard ParentSyncGuard
	if queries != nil {
		dispatch = NewAIPipelineDispatchService(queries, LoadDispatchConfig(), log)
		parentGuard = NewSQLCParentSyncGuard(queries, log, jobMetrics, "recalculation")
	}
	return &RecalculateAfterSyncHandler{
		account:            account,
		sku:                sku,
		funnel:             funnel,
		alerts:             alertsSvc,
		queries:            queries,
		parentGuard:        parentGuard,
		dispatch:           dispatch,
		enqueueAISummaries: enqueueAISummaries,
		log:                log,
	}
}

func postSyncCalendarDayUTC(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// Handle rebuilds bounded metrics and runs alerts. Returns error on any failed stage so the task can retry.
func (h *RecalculateAfterSyncHandler) Handle(ctx context.Context, taskPayload []byte) error {
	var payload RecalculateAfterSyncPayload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal post-sync recalculation payload: %w", err)
	}
	if payload.SellerAccountID <= 0 || payload.SyncJobID <= 0 {
		return fmt.Errorf("invalid post-sync payload: seller_account_id=%d sync_job_id=%d", payload.SellerAccountID, payload.SyncJobID)
	}
	if h.parentGuard != nil {
		if _, err := h.parentGuard.RequireCompletedParentSync(ctx, payload.SyncJobID, payload.SellerAccountID, nil); err != nil {
			return MapParentSyncGuardErrorToAsynq(err)
		}
	}

	var dispatch dbgen.AiPipelineDispatch
	if h.dispatch != nil {
		var err error
		dispatch, err = h.dispatch.EnsureDispatch(ctx, payload.SellerAccountID, payload.SyncJobID)
		if err != nil {
			return fmt.Errorf("ensure ai pipeline dispatch: %w", err)
		}
		if dispatchTerminal(dispatch.Status) {
			return nil
		}
		if dispatch.Status == DispatchStatusFailed {
			return nil
		}
	}

	to := postSyncCalendarDayUTC(time.Now())
	from := to.AddDate(0, 0, -(postSyncMetricsLookbackDays - 1))

	h.log.Info("post-sync recalculation started",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.String("date_from", from.Format("2006-01-02")),
		zap.String("date_to", to.Format("2006-01-02")),
	)

	nAcc := 0
	nSku := 0
	runHeavy := h.dispatch == nil || !dispatchRecalculationDone(dispatch.Status)
	if runHeavy && h.dispatch != nil {
		var skip bool
		var err error
		dispatch, skip, err = h.dispatch.BeginHeavyRecalculation(ctx, dispatch)
		if err != nil {
			return err
		}
		if skip {
			refreshed, rerr := h.dispatch.reloadDispatch(ctx, dispatch)
			if rerr != nil {
				return rerr
			}
			dispatch = refreshed
			if dispatch.Status == DispatchStatusRecalculationRunning {
				return fmt.Errorf("recalculation in progress")
			}
			runHeavy = !dispatchRecalculationDone(dispatch.Status)
		}
	}
	if runHeavy {
		var err error
		nAcc, err = h.account.RebuildDailyAccountMetricsForDateRange(ctx, payload.SellerAccountID, from, to)
		if err != nil {
			h.log.Error("post-sync account metrics rebuild failed",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
				zap.Error(err),
			)
			return fmt.Errorf("rebuild account metrics: %w", err)
		}

		nSku, err = h.sku.RebuildDailySKUMetricsForDateRange(ctx, payload.SellerAccountID, from, to)
		if err != nil {
			h.log.Error("post-sync sku metrics rebuild failed",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
				zap.Error(err),
			)
			return fmt.Errorf("rebuild sku metrics: %w", err)
		}

		if h.funnel != nil {
			funnelFrom := to.AddDate(0, 0, -1)
			if err := h.funnel.SyncFunnelForSellerRange(ctx, payload.SellerAccountID, funnelFrom, to); err != nil {
				h.log.Warn("post-sync product funnel sync failed",
					zap.Int64("seller_account_id", payload.SellerAccountID),
					zap.Int64("sync_job_id", payload.SyncJobID),
					zap.String("funnel_date_from", funnelFrom.Format("2006-01-02")),
					zap.String("funnel_date_to", to.Format("2006-01-02")),
					zap.Error(err),
				)
			}
		}

		summary, err := h.alerts.RunForAccountWithType(ctx, payload.SellerAccountID, to, alerts.RunTypePostSync)
		if err != nil {
			h.log.Error("post-sync alerts run failed",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
				zap.Error(err),
			)
			return fmt.Errorf("run alerts post_sync: %w", err)
		}
		_ = summary
		if h.dispatch != nil {
			dispatch, err = h.dispatch.CompleteHeavyRecalculation(ctx, dispatch.ID)
			if err != nil {
				return fmt.Errorf("mark dispatch recalculation completed: %w", err)
			}
		}
	}

	h.log.Info("post-sync recalculation completed",
		zap.Int64("seller_account_id", payload.SellerAccountID),
		zap.Int64("sync_job_id", payload.SyncJobID),
		zap.Int("account_daily_metrics_rows", nAcc),
		zap.Int("sku_daily_metrics_rows", nSku),
	)

	if h.enqueueAISummaries != nil && h.dispatch != nil {
		updated, err := h.dispatch.EnqueuePipelineTask(ctx, dispatch, h.enqueueAISummaries)
		if err != nil {
			h.log.Error("enqueue ai pipeline failed",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
				zap.Error(err),
			)
			return fmt.Errorf("enqueue continuation after recalc: %w", err)
		}
		_ = updated
	}

	if h.unifiedAutoRun != nil {
		if err := h.unifiedAutoRun(ctx, payload.SellerAccountID); err != nil {
			h.log.Warn("unified analysis post-sync auto-run failed",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
				zap.Error(err),
			)
		}
	}
	return nil
}
