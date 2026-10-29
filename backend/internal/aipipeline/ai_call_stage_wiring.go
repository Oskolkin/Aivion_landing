package aipipeline

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

func saveGlobalContextCallArtifacts(
	ctx context.Context,
	artifacts *ArtifactRecorder,
	runID int64,
	meta map[string]any,
	provider string,
	raw *GlobalContextRawResponse,
	parsed *aicontext.GlobalSemanticContext,
	timeoutSeconds int,
	debugArtifacts bool,
) {
	if artifacts == nil || raw == nil {
		return
	}
	env := BuildAICallDiagnosticEnvelopeFromGlobalContextRaw(provider, raw, timeoutSeconds)
	env.PipelineRunID = runID
	_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindGlobalContextCallDiagnostics, meta, env)
	if debugArtifacts {
		_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextRawResponse, meta,
			BuildAICallRawResponseArtifact(env, raw.Content))
	}
	if parsed != nil {
		_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindGlobalContextParsedOutput, meta,
			BuildGlobalContextParsedOutputArtifact(parsed))
	}
}

func saveDashboardCallArtifacts(
	ctx context.Context,
	artifacts *ArtifactRecorder,
	runID int64,
	meta map[string]any,
	provider string,
	raw *actions.AIRawResponse,
	parsedPayload any,
	parseStatus, parseError string,
	timeoutSeconds int,
	debugArtifacts bool,
) {
	if artifacts == nil || raw == nil {
		return
	}
	env := BuildAICallDiagnosticEnvelopeFromRaw(
		"dashboard", AIUsageComponentDashboardSynthesis, provider,
		raw, raw.RequestedMaxOutputTokens, timeoutSeconds,
		time.Time{}, time.Time{}, parseStatus, parseError,
	)
	env.PipelineRunID = runID
	_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindDashboardCallDiagnostics, meta, env)
	if debugArtifacts {
		body := raw.RawText
		_ = artifacts.SaveDocument(ctx, ArtifactKindDashboardRawResponse, meta,
			BuildAICallRawResponseArtifact(env, body))
	}
	if parsedPayload != nil && parseStatus == ParseStatusParsed {
		_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindDashboardParsedOutput, meta,
			BuildDashboardParsedOutputArtifact(parsedPayload))
	}
}

func dashboardSynthesisRawFromOutcome(outcome *DashboardRunOutcome) *actions.AIRawResponse {
	if outcome == nil || outcome.Result == nil {
		return nil
	}
	res := outcome.Result
	raw := &actions.AIRawResponse{
		Model:                    res.DashboardSynthesisModel,
		RequestID:                res.DashboardSynthesisRequestID,
		InputTokens:              res.DashboardSynthesisInputTokens,
		OutputTokens:             res.DashboardSynthesisOutputTokens,
		LatencyMs:                res.DashboardSynthesisLatencyMs,
		RawText:                  res.DashboardSynthesisRawResponse,
		ProviderStatus:           res.DashboardSynthesisProviderStatus,
		FinishReason:             res.DashboardSynthesisFinishReason,
		IncompleteReason:         res.DashboardSynthesisIncompleteReason,
		OutputTruncated:          res.DashboardSynthesisOutputTruncated,
		RequestedMaxOutputTokens: res.DashboardSynthesisRequestedMaxOutputTokens,
	}
	return raw
}

func dashboardSynthesisParseStatus(outcome *DashboardRunOutcome) string {
	if outcome == nil || outcome.Result == nil {
		return ParseStatusNotCaptured
	}
	if outcome.Result.SynthesisParseError != "" {
		return ParseStatusInvalidJSON
	}
	if outcome.Result.DashboardSynthesisRawResponse != "" && outcome.Result.IsSuccess() {
		return ParseStatusParsed
	}
	if outcome.Result.DashboardSynthesisRawResponse != "" {
		return ParseStatusUnknown
	}
	return ParseStatusNotCaptured
}

func persistDashboardCallDiagnostics(
	ctx context.Context,
	artifacts *ArtifactRecorder,
	runID int64,
	req RunSellerAIPipelineRequest,
	providers PipelineProviders,
	dataPackHash, gcHash string,
	dashOutcome *DashboardRunOutcome,
) {
	if artifacts == nil || dashOutcome == nil || dashOutcome.Result == nil {
		return
	}
	if dashOutcome.Result.Skipped {
		return
	}
	if dashOutcome.Result.DashboardSynthesisAICalls == 0 && dashOutcome.Result.DashboardSynthesisRawResponse == "" {
		return
	}
	meta := aiArtifactMeta(runID, providers.Dashboard, dashboardArtifactModel(dashOutcome), 0, 0, map[string]any{
		"data_pack_hash":      dataPackHash,
		"global_context_hash": gcHash,
		"mode":                dashboardArtifactMode(dashOutcome),
	})
	raw := dashboardSynthesisRawFromOutcome(dashOutcome)
	parseStatus := dashboardSynthesisParseStatus(dashOutcome)
	parseErr := dashOutcome.Result.SynthesisParseError
	var parsed any
	if dashOutcome.DebugOutput != nil {
		parsed = dashOutcome.DebugOutput
	}
	saveDashboardCallArtifacts(ctx, artifacts, runID, meta, providers.Dashboard, raw, parsed, parseStatus, parseErr, 0, req.DebugArtifacts)
}
