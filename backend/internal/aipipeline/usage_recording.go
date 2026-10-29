package aipipeline

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type aiUsageRecorder interface {
	Record(ctx context.Context, in aicost.UsageEventInput) error
}

func (s *SellerAIPipelineService) SetUsageRecorder(rec aiUsageRecorder) {
	if s != nil {
		s.usage = rec
	}
}

func (s *SellerAIPipelineService) recordComponentUsage(
	ctx context.Context,
	sellerID int64,
	pipelineRunID int64,
	component string,
	providerLabel string,
	model string,
	requestID string,
	inputTokens int,
	outputTokens int,
	latencyMs int64,
	status string,
) {
	if s == nil || s.usage == nil || sellerID <= 0 || component == "" {
		return
	}
	pid := pipelineRunID
	_ = s.usage.Record(ctx, aicost.UsageEventInput{
		SellerAccountID: sellerID,
		PipelineRunID:   &pid,
		Component:       component,
		Attempt:         pipelineUsageAttempt,
		Provider:        usageProviderName(providerLabel),
		Model:           usageBillingModel(providerLabel, model),
		RequestID:       requestID,
		InputTokens:     normalizeTokenCount(inputTokens),
		OutputTokens:    normalizeTokenCount(outputTokens),
		LatencyMs:       latencyMs,
		Status:          status,
	})
}

func (s *SellerAIPipelineService) recordGlobalContextUsage(
	ctx context.Context,
	sellerID int64,
	pipelineRunID int64,
	providerLabel string,
	model string,
	raw *GlobalContextRawResponse,
	err error,
) {
	u := usageFromGlobalContextRaw(providerLabel, model, raw, err)
	status := "completed"
	if err != nil {
		status = "failed"
	}
	s.recordComponentUsage(
		ctx,
		sellerID,
		pipelineRunID,
		AIUsageComponentGlobalContext,
		providerLabel,
		u.Model,
		u.RequestID,
		u.InputTokens,
		u.OutputTokens,
		u.LatencyMs,
		status,
	)
}

func (s *SellerAIPipelineService) recordDashboardSynthesisUsage(
	ctx context.Context,
	sellerID int64,
	pipelineRunID int64,
	providerLabel string,
	model string,
	raw *actions.AIRawResponse,
	status string,
) {
	u := usageFromAIRaw(providerLabel, model, raw)
	if status == "" {
		status = "completed"
	}
	s.recordComponentUsage(
		ctx,
		sellerID,
		pipelineRunID,
		AIUsageComponentDashboardSynthesis,
		providerLabel,
		u.Model,
		u.RequestID,
		u.InputTokens,
		u.OutputTokens,
		u.LatencyMs,
		status,
	)
}

func (s *SellerAIPipelineService) recordActionsSynthesisUsage(
	ctx context.Context,
	sellerID int64,
	pipelineRunID int64,
	providerLabel string,
	model string,
	raw *actions.AIRawResponse,
	status string,
) {
	u := usageFromAIRaw(providerLabel, model, raw)
	if status == "" {
		status = "completed"
	}
	s.recordComponentUsage(
		ctx,
		sellerID,
		pipelineRunID,
		AIUsageComponentActionsSynthesis,
		providerLabel,
		u.Model,
		u.RequestID,
		u.InputTokens,
		u.OutputTokens,
		u.LatencyMs,
		status,
	)
}

func (s *SellerAIPipelineService) recordChunkAnalysisUsage(
	ctx context.Context,
	sellerID int64,
	pipelineRunID int64,
	providerLabel string,
	call dashboardhome.ChunkCallUsage,
) {
	if call.Status == dashboardhome.ChunkCallStatusSkipped {
		return
	}
	model := call.Model
	if model == "" {
		model = providerLabel
	}
	s.recordComponentUsage(
		ctx,
		sellerID,
		pipelineRunID,
		AIUsageComponentChunkAnalysis,
		providerLabel,
		model,
		BuildChunkUsageRequestID(call.ChunkID, call.RequestID),
		call.InputTokens,
		call.OutputTokens,
		call.LatencyMs,
		call.Status,
	)
}

func (s *SellerAIPipelineService) recordChunkAnalysisRunUsage(
	ctx context.Context,
	sellerID int64,
	pipelineRunID int64,
	providerLabel string,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
) {
	if chunkRun == nil {
		return
	}
	for _, call := range chunkRun.ChunkCalls {
		s.recordChunkAnalysisUsage(ctx, sellerID, pipelineRunID, providerLabel, call)
	}
}
