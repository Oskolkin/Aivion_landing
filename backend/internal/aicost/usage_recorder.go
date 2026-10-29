package aicost

import (
	"context"
	"fmt"
	"math/big"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// UsageEventInput is a single AI usage accounting row.
type UsageEventInput struct {
	SellerAccountID int64
	PipelineRunID   *int64
	Component       string
	Attempt         int
	Provider        string
	Model           string
	RequestID       string
	InputTokens     int
	OutputTokens    int
	TotalTokens     int
	EstimatedCost   *float64
	Status          string
	LatencyMs       int64 // observability only; not persisted until DB schema adds the column
}

// UsageRecorder persists ai_usage_events rows.
type UsageRecorder struct {
	q *dbgen.Queries
}

func NewUsageRecorder(q *dbgen.Queries) *UsageRecorder {
	return &UsageRecorder{q: q}
}

func (r *UsageRecorder) Record(ctx context.Context, in UsageEventInput) error {
	if r == nil || r.q == nil {
		return nil
	}
	if in.SellerAccountID <= 0 || in.Component == "" {
		return fmt.Errorf("invalid usage event")
	}
	status := in.Status
	if status == "" {
		status = "completed"
	}
	total := in.TotalTokens
	if total <= 0 {
		total = in.InputTokens + in.OutputTokens
	}
	var cost pgtype.Numeric
	if in.EstimatedCost != nil {
		_ = cost.Scan(fmt.Sprintf("%f", *in.EstimatedCost))
	} else if in.Model != "" && (in.InputTokens > 0 || in.OutputTokens > 0) {
		est := EstimateUSD(in.Model, in.InputTokens, in.OutputTokens)
		if est.Known {
			_ = cost.Scan(big.NewFloat(est.CostUSD).Text('f', 6))
		}
	}
	var pipelineID pgtype.Int8
	if in.PipelineRunID != nil && *in.PipelineRunID > 0 {
		pipelineID = pgtype.Int8{Int64: *in.PipelineRunID, Valid: true}
	}
	_, err := r.q.CreateAIUsageEvent(ctx, dbgen.CreateAIUsageEventParams{
		SellerAccountID: in.SellerAccountID,
		PipelineRunID:   pipelineID,
		Component:       in.Component,
		Attempt:         int32(in.Attempt),
		Model:           pgtype.Text{String: in.Model, Valid: in.Model != ""},
		RequestID:       pgtype.Text{String: in.RequestID, Valid: in.RequestID != ""},
		InputTokens:     int32(in.InputTokens),
		OutputTokens:    int32(in.OutputTokens),
		TotalTokens:     int32(total),
		EstimatedCost:   cost,
		Status:          status,
	})
	if err != nil {
		return fmt.Errorf("create ai usage event: %w", err)
	}
	return nil
}
