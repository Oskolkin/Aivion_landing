package actions

import (
	"context"
	"time"
)

func perCallTimeoutContext(parent context.Context, timeoutSeconds int) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeoutSeconds <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, time.Duration(timeoutSeconds)*time.Second)
}

func buildTwoPhaseExecutionDiagnostics(
	ctx context.Context,
	stageStarted time.Time,
	composeStarted, composeFinished time.Time,
	composeStatus string,
	composeTruncated bool,
	detailMerges []ObjectDetailMergeResult,
) *ActionsTwoPhaseExecutionDiagnostics {
	exec := &ActionsTwoPhaseExecutionDiagnostics{
		ComposeTimeoutSeconds: ActionsComposeTimeoutSeconds(),
		DetailTimeoutSeconds:  ActionsObjectDetailTimeoutSeconds(),
		ComposeStartedAt:      composeStarted.UTC().Format(time.RFC3339),
		ComposeFinishedAt:     composeFinished.UTC().Format(time.RFC3339),
		ComposeElapsedMs:      composeFinished.Sub(composeStarted).Milliseconds(),
		ComposeProviderStatus: composeStatus,
		StageStartedAt:        stageStarted.UTC().Format(time.RFC3339),
	}
	if deadline, ok := ctx.Deadline(); ok {
		exec.ParentContextHasDeadline = true
		exec.ParentDeadlineAt = deadline.UTC().Format(time.RFC3339)
	}
	if ctx.Err() != nil {
		exec.CancelReason = ctx.Err().Error()
	}
	stageFinished := time.Now().UTC()
	exec.StageFinishedAt = stageFinished.Format(time.RFC3339)
	exec.StageElapsedMs = stageFinished.Sub(stageStarted).Milliseconds()
	if composeTruncated {
		exec.ComposeProviderStatus = "truncated"
	}
	for _, merge := range detailMerges {
		for _, batch := range merge.BatchResults {
			exec.PlannedBatches++
			exec.StartedBatches++
			switch batch.Status {
			case "completed":
				exec.CompletedBatches++
			case "partial":
				exec.PartialBatches++
			case "truncated":
				exec.TruncatedBatches++
			default:
				exec.FailedBatches++
			}
			exec.DetailBatches = append(exec.DetailBatches, ActionsDetailBatchExecution{
				ActionKey:      batch.ActionKey,
				BatchIndex:     batch.BatchIndex,
				EntityCount:    batch.EntityCount,
				StartedAt:      batch.StartedAt,
				FinishedAt:     batch.FinishedAt,
				ElapsedMs:      batch.ElapsedMs,
				TimeoutSeconds: batch.TimeoutSeconds,
				Status:         batch.Status,
				ProviderError:  batch.ProviderError,
				Truncated:      batch.OutputTruncated,
			})
			if batch.RawResponse != nil {
				last := len(exec.DetailBatches) - 1
				exec.DetailBatches[last].InputTokens = batch.RawResponse.InputTokens
				exec.DetailBatches[last].OutputTokens = batch.RawResponse.OutputTokens
			}
		}
	}
	return exec
}
