package aipipeline

import (
	"context"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func saveSharedChunkDebugArtifacts(
	ctx context.Context,
	artifacts *ArtifactRecorder,
	runID int64,
	chunkSet *aidatapack.ChunkSet,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
	dataPackHash string,
) {
	if artifacts == nil || chunkSet == nil || chunkRun == nil {
		return
	}
	meta := pipelineArtifactMeta(runID, map[string]any{
		"schema_version": ChunkSetArtifactSchemaVersion,
		"data_pack_hash": dataPackHash,
		"chunk_count":    len(chunkSet.Chunks),
	})
	chunkSetPayload := buildChunkSetDebugArtifact(chunkSet)
	_ = artifacts.SaveDocument(ctx, ArtifactKindChunkSet, meta, chunkSetPayload)

	rawMeta := pipelineArtifactMeta(runID, map[string]any{
		"schema_version": ChunkRawResponseArtifactSchemaVersion,
		"data_pack_hash": dataPackHash,
	})
	_ = artifacts.SaveDocument(ctx, ArtifactKindChunkRawResponse, rawMeta, buildChunkRawResponseArtifact(chunkRun))

	valMeta := pipelineArtifactMeta(runID, map[string]any{
		"schema_version": ChunkValidationArtifactSchemaVersion,
		"data_pack_hash": dataPackHash,
	})
	_ = artifacts.SaveDocument(ctx, ArtifactKindChunkValidation, valMeta, buildChunkValidationArtifact(chunkRun))
}

func buildChunkSetDebugArtifact(chunkSet *aidatapack.ChunkSet) map[string]any {
	chunks := make([]map[string]any, 0, len(chunkSet.Chunks))
	for _, ch := range chunkSet.Chunks {
		chunks = append(chunks, map[string]any{
			"chunk_id":           ch.ChunkID,
			"chunk_type":         ch.ChunkType,
			"chunk_hash":         ch.ChunkHash,
			"approx_tokens":      ch.ApproxTokens,
			"local_fact_count":   len(ch.LocalFactCatalog),
			"local_entity_count": len(ch.LocalEntityCatalog),
			"source_table":       ch.SourceTable,
		})
	}
	return map[string]any{
		"schema_version":    ChunkSetArtifactSchemaVersion,
		"seller_account_id": chunkSet.SellerAccountID,
		"metric_date":       chunkSet.MetricDate,
		"chunk_count":       len(chunkSet.Chunks),
		"chunks":            chunks,
	}
}

func buildChunkRawResponseArtifact(chunkRun *dashboardhome.ChunkAnalysisRunResult) map[string]any {
	calls := make([]map[string]any, 0, len(chunkRun.ChunkCalls))
	for _, call := range chunkRun.ChunkCalls {
		entry := map[string]any{
			"chunk_id":                    call.ChunkID,
			"chunk_type":                  call.ChunkType,
			"provider":                    call.Provider,
			"model":                       call.Model,
			"provider_request_id":         call.RequestID,
			"usage_request_id":            BuildChunkUsageRequestID(call.ChunkID, call.RequestID),
			"input_tokens":                call.InputTokens,
			"output_tokens":               call.OutputTokens,
			"latency_ms":                  call.LatencyMs,
			"status":                      call.Status,
			"usage_source":                call.UsageSource,
			"error":                       call.Error,
			"provider_status":             call.ProviderStatus,
			"finish_reason":               call.FinishReason,
			"incomplete_reason":           call.IncompleteReason,
			"requested_max_output_tokens": call.RequestedMaxOutputTokens,
			"output_truncated":            call.OutputTruncated,
			"parse_status":                call.ParseStatus,
			"parse_error":                 call.ParseError,
			"raw_body_bytes":              len(call.RawBody),
		}
		if call.RawBody != "" {
			entry["body"] = call.RawBody
			entry["body_hash"] = hashBody(call.RawBody)
		}
		calls = append(calls, entry)
	}
	return map[string]any{
		"schema_version": ChunkRawResponseArtifactSchemaVersion,
		"calls":          calls,
	}
}

func buildChunkValidationArtifact(chunkRun *dashboardhome.ChunkAnalysisRunResult) []map[string]any {
	out := make([]map[string]any, 0, len(chunkRun.Analyses))
	for _, analysis := range chunkRun.Analyses {
		entry := map[string]any{
			"chunk_id":                  analysis.ChunkID,
			"chunk_type":                analysis.ChunkType,
			"analysis_status":           analysis.AnalysisStatus,
			"accepted_signals_count":    len(analysis.Signals),
			"accepted_candidates_count": len(analysis.ActionCandidates),
		}
		for _, call := range chunkRun.ChunkCalls {
			if call.ChunkID == analysis.ChunkID {
				entry["validation_status"] = call.Status
				break
			}
		}
		out = append(out, entry)
	}
	return out
}

func buildGlobalContextRawOutputArtifact(raw *GlobalContextRawResponse, model, status string) map[string]any {
	payload := map[string]any{
		"schema_version": RawOutputArtifactSchemaVersion,
		"component":      AIUsageComponentGlobalContext,
		"model":          model,
		"status":         status,
	}
	if raw != nil {
		payload["request_id"] = raw.RequestID
		payload["input_tokens"] = raw.InputTokens
		payload["output_tokens"] = raw.OutputTokens
		payload["latency_ms"] = raw.LatencyMs
		payload["content"] = raw.Content
	}
	return payload
}

func buildDashboardRawOutputArtifact(model, status string, inputTokens, outputTokens int, latencyMs int64, requestID string) map[string]any {
	return map[string]any{
		"schema_version": RawOutputArtifactSchemaVersion,
		"component":      AIUsageComponentDashboardSynthesis,
		"model":          model,
		"status":         status,
		"request_id":     requestID,
		"input_tokens":   inputTokens,
		"output_tokens":  outputTokens,
		"latency_ms":     latencyMs,
	}
}

func buildActionsRawOutputArtifact(actRes *actions.GenerateActionsResult, model, status string) map[string]any {
	payload := map[string]any{
		"schema_version": RawOutputArtifactSchemaVersion,
		"component":      AIUsageComponentActionsSynthesis,
		"model":          model,
		"status":         status,
		"content_type":   ArtifactContentMetadataOnly,
	}
	if actRes != nil {
		payload["request_id"] = actRes.RequestID
		payload["input_tokens"] = actRes.InputTokens
		payload["output_tokens"] = actRes.OutputTokens
		payload["latency_ms"] = actRes.LatencyMs
		if actRes.ComposeRawResponse != nil {
			raw := actRes.ComposeRawResponse
			if model == "" {
				payload["model"] = raw.Model
			}
			payload["provider_status"] = raw.ProviderStatus
			payload["finish_reason"] = raw.FinishReason
			payload["incomplete_reason"] = raw.IncompleteReason
			payload["requested_max_output_tokens"] = raw.RequestedMaxOutputTokens
			if body := raw.RawText; strings.TrimSpace(body) != "" {
				payload["raw_text"] = body
				payload["content_type"] = ArtifactContentFullBody
			}
		}
	}
	return payload
}
