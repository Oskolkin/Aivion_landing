package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ozonenrichmentsync"
	"go.uber.org/zap"
)

type OzonEnrichmentP0Handler struct {
	svc                   *ozonenrichmentsync.Service
	queries               *dbgen.Queries
	parentGuard           ParentSyncGuard
	enqueuePostSyncRecalc func(ctx context.Context, sellerAccountID, syncJobID int64) error
	log                   *zap.Logger
}

func NewOzonEnrichmentP0Handler(
	svc *ozonenrichmentsync.Service,
	queries *dbgen.Queries,
	enqueuePostSyncRecalc func(ctx context.Context, sellerAccountID, syncJobID int64) error,
	log *zap.Logger,
	jobMetrics *metrics.Metrics,
) *OzonEnrichmentP0Handler {
	var parentGuard ParentSyncGuard
	if queries != nil {
		parentGuard = NewSQLCParentSyncGuard(queries, log, jobMetrics, "enrichment")
	}
	return &OzonEnrichmentP0Handler{
		svc:                   svc,
		queries:               queries,
		parentGuard:           parentGuard,
		enqueuePostSyncRecalc: enqueuePostSyncRecalc,
		log:                   log,
	}
}

func (h *OzonEnrichmentP0Handler) Handle(ctx context.Context, taskPayload []byte) error {
	var payload OzonEnrichmentP0Payload
	if err := json.Unmarshal(taskPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal enrichment payload: %w", err)
	}
	if payload.SellerAccountID <= 0 || payload.SyncJobID <= 0 {
		return fmt.Errorf("invalid enrichment payload")
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
	scope := payload.Scope
	if scope == "" {
		scope = "p0"
	}

	result, err := h.svc.SyncOzonEnrichmentP0(ctx, ozonenrichmentsync.SyncInput{
		SellerAccountID: payload.SellerAccountID,
		MetricDate:      metricDate,
		Scope:           scope,
	})
	if err != nil {
		h.log.Warn("ozon enrichment failed",
			zap.Int64("seller_account_id", payload.SellerAccountID),
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.Error(err),
		)
		if isLastAsynqRetry(ctx) {
			h.log.Warn("ozon enrichment exhausted retries; continuing post-sync on base data",
				zap.Int64("seller_account_id", payload.SellerAccountID),
				zap.Int64("sync_job_id", payload.SyncJobID),
			)
			return h.enqueueRecalcIfConfigured(ctx, payload)
		}
		return err
	}

	switch result.Status {
	case "completed", "completed_partial":
		h.log.Info("ozon enrichment finished",
			zap.Int64("seller_account_id", payload.SellerAccountID),
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.String("status", string(result.Status)),
			zap.Int64("run_id", result.RunID),
		)
	case "failed":
		h.log.Warn("ozon enrichment terminal failed; continuing post-sync on base data",
			zap.Int64("seller_account_id", payload.SellerAccountID),
			zap.Int64("sync_job_id", payload.SyncJobID),
			zap.Int64("run_id", result.RunID),
		)
	default:
		return fmt.Errorf("ozon enrichment terminal status: %s", result.Status)
	}

	return h.enqueueRecalcIfConfigured(ctx, payload)
}

func (h *OzonEnrichmentP0Handler) enqueueRecalcIfConfigured(ctx context.Context, payload OzonEnrichmentP0Payload) error {
	if h.enqueuePostSyncRecalc != nil {
		if err := h.enqueuePostSyncRecalc(ctx, payload.SellerAccountID, payload.SyncJobID); err != nil {
			return err
		}
	}
	return nil
}
