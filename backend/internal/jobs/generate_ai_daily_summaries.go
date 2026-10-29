package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

const TaskTypeGenerateAIDailySummaries = "ai.generate_daily_summaries"

type GenerateAIDailySummariesPayload struct {
	SellerAccountID int64 `json:"seller_account_id"`
	SyncJobID       int64 `json:"sync_job_id"`
}

func NewGenerateAIDailySummariesTask(sellerAccountID, syncJobID int64) (*asynq.Task, error) {
	payload, err := json.Marshal(GenerateAIDailySummariesPayload{
		SellerAccountID: sellerAccountID,
		SyncJobID:       syncJobID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal ai daily summaries payload: %w", err)
	}
	return asynq.NewTask(TaskTypeGenerateAIDailySummaries, payload), nil
}

// GenerateAIDailySummariesHandler runs AI daily summary generation (async, idempotent).
type GenerateAIDailySummariesHandler struct {
	svc        AISummariesGenerator
	narratives DashboardNarrativeGenerator
	log        *zap.Logger
}

// AISummariesGenerator is implemented by aisummaries.Service.
type AISummariesGenerator interface {
	RunForSellerAfterSync(ctx context.Context, sellerAccountID int64) error
}

type DashboardNarrativeGenerator interface {
	RunForSellerAfterSync(ctx context.Context, sellerAccountID int64) error
}

func NewGenerateAIDailySummariesHandler(
	svc AISummariesGenerator,
	narratives DashboardNarrativeGenerator,
	log *zap.Logger,
) *GenerateAIDailySummariesHandler {
	return &GenerateAIDailySummariesHandler{svc: svc, narratives: narratives, log: log}
}

func (h *GenerateAIDailySummariesHandler) Handle(ctx context.Context, payload []byte) error {
	var p GenerateAIDailySummariesPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("unmarshal ai daily summaries payload: %w", err)
	}
	if p.SellerAccountID <= 0 {
		return fmt.Errorf("invalid ai daily summaries payload: seller_account_id=%d", p.SellerAccountID)
	}
	if err := h.svc.RunForSellerAfterSync(ctx, p.SellerAccountID); err != nil {
		h.log.Warn("ai daily summaries run failed",
			zap.Int64("seller_account_id", p.SellerAccountID),
			zap.Int64("sync_job_id", p.SyncJobID),
			zap.Error(err),
		)
		return err
	}
	if h.narratives != nil {
		if err := h.narratives.RunForSellerAfterSync(ctx, p.SellerAccountID); err != nil {
			h.log.Warn("dashboard narrative run failed",
				zap.Int64("seller_account_id", p.SellerAccountID),
				zap.Int64("sync_job_id", p.SyncJobID),
				zap.Error(err),
			)
			return err
		}
	}
	h.log.Info("ai daily summaries run finished",
		zap.Int64("seller_account_id", p.SellerAccountID),
		zap.Int64("sync_job_id", p.SyncJobID),
	)
	return nil
}
