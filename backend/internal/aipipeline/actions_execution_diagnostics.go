package aipipeline

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

func buildActionsExecutionDiagnostics(
	actRes *actions.GenerateActionsResult,
	diag *PipelineDiagnostics,
) map[string]any {
	out := map[string]any{
		"schema_version": ActionsExecutionDiagnosticsSchemaVersion,
	}
	if diag != nil {
		stage := map[string]any{
			"parent_context_has_deadline": false,
		}
		out["compose"] = map[string]any{
			"timeout_seconds": diag.Actions.ComposeTimeoutSeconds,
		}
		out["object_details"] = map[string]any{
			"batch_size":               actions.ActionsObjectDetailBatchSize(),
			"max_concurrency":          actions.ActionsObjectDetailMaxConcurrency(),
			"per_call_timeout_seconds": diag.Actions.ObjectDetailTimeoutSeconds,
		}
		out["stage"] = stage
	}
	if actRes == nil {
		return out
	}
	if actRes.PipelineDiagnostics != nil {
		pd := actRes.PipelineDiagnostics
		if exec := pd.Execution; exec != nil {
			out["compose"] = map[string]any{
				"timeout_seconds":  exec.ComposeTimeoutSeconds,
				"started_at":       exec.ComposeStartedAt,
				"finished_at":      exec.ComposeFinishedAt,
				"elapsed_ms":       exec.ComposeElapsedMs,
				"provider_status":  exec.ComposeProviderStatus,
				"output_truncated": pd.ActionsComposeOutputTruncated,
				"input_tokens":     pd.ActionsComposeInputTokens,
				"output_tokens":    pd.ActionsComposeOutputTokens,
			}
			batches := make([]map[string]any, 0, len(exec.DetailBatches))
			for _, b := range exec.DetailBatches {
				batches = append(batches, map[string]any{
					"action_key":      b.ActionKey,
					"batch_index":     b.BatchIndex,
					"entity_count":    b.EntityCount,
					"started_at":      b.StartedAt,
					"finished_at":     b.FinishedAt,
					"elapsed_ms":      b.ElapsedMs,
					"timeout_seconds": b.TimeoutSeconds,
					"status":          b.Status,
					"provider_error":  b.ProviderError,
					"truncated":       b.Truncated,
					"input_tokens":    b.InputTokens,
					"output_tokens":   b.OutputTokens,
				})
			}
			out["object_details"] = map[string]any{
				"batch_size":               actions.ActionsObjectDetailBatchSize(),
				"max_concurrency":          actions.ActionsObjectDetailMaxConcurrency(),
				"planned_batches":          exec.PlannedBatches,
				"started_batches":          exec.StartedBatches,
				"completed_batches":        exec.CompletedBatches,
				"partial_batches":          exec.PartialBatches,
				"failed_batches":           exec.FailedBatches,
				"truncated_batches":        exec.TruncatedBatches,
				"per_call_timeout_seconds": exec.DetailTimeoutSeconds,
				"batches":                  batches,
			}
			out["stage"] = map[string]any{
				"started_at":                  exec.StageStartedAt,
				"finished_at":                 exec.StageFinishedAt,
				"elapsed_ms":                  exec.StageElapsedMs,
				"parent_context_has_deadline": exec.ParentContextHasDeadline,
				"parent_deadline_at":          exec.ParentDeadlineAt,
				"cancel_reason":               exec.CancelReason,
			}
			return out
		}
	}
	if actRes.ComposeRawResponse != nil {
		out["compose"] = map[string]any{
			"elapsed_ms":       actRes.ComposeRawResponse.LatencyMs,
			"input_tokens":     actRes.ComposeRawResponse.InputTokens,
			"output_tokens":    actRes.ComposeRawResponse.OutputTokens,
			"output_truncated": actRes.PipelineDiagnostics != nil && actRes.PipelineDiagnostics.ActionsComposeOutputTruncated,
		}
	}
	if diag != nil {
		out["object_details"] = map[string]any{
			"batch_size":        actions.ActionsObjectDetailBatchSize(),
			"max_concurrency":   actions.ActionsObjectDetailMaxConcurrency(),
			"planned_batches":   diag.Actions.ObjectDetailCalls,
			"completed_batches": diag.Actions.ObjectDetailCalls,
		}
	}
	return out
}

func hasBlockingGlobalContextPlaceholder(validation GlobalContextValidationResult) bool {
	for _, issue := range validation.Issues {
		if issue.Code != GlobalContextUnresolvedNumericPlaceholderCode {
			continue
		}
		if issue.Field == "overall_story" || issue.Field == "primary_risks" || issue.Field == "secondary_risks" {
			return true
		}
	}
	return false
}

func populateGlobalContextValidationDiagnostics(diag *PipelineDiagnostics, beforeStatus, afterStatus, beforeConfidence, afterConfidence string, validation GlobalContextValidationResult) {
	if diag == nil {
		return
	}
	diag.GlobalContext.ValidationStatusBefore = beforeStatus
	diag.GlobalContext.ValidationStatus = afterStatus
	diag.GlobalContext.ConfidenceBefore = beforeConfidence
	diag.GlobalContext.Confidence = afterConfidence
	blocking, warnings := 0, 0
	for _, issue := range validation.Issues {
		if issue.Severity == "fatal" || issue.Severity == "error" {
			blocking++
		} else {
			warnings++
		}
	}
	diag.GlobalContext.ValidationBlockingIssueCount = blocking
	diag.GlobalContext.ValidationWarningCount = warnings
}

func globalContextValidationArtifactFromResult(
	beforeStatus, afterStatus, beforeConfidence, afterConfidence string,
	validation GlobalContextValidationResult,
) map[string]any {
	return buildGlobalContextValidationArtifact(beforeStatus, afterStatus, beforeConfidence, afterConfidence, validation)
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
