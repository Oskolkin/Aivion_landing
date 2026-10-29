package unifiedanalysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// transientProviderErrorCodes are the only failed-status codes eligible for
// automatic in-run retry. Unknown codes are never treated as transient.
var transientProviderErrorCodes = map[string]struct{}{
	"server_error":        {},
	"rate_limit_exceeded": {},
	"timeout":             {},
	"service_unavailable": {},
}

func isTransientProviderErrorCode(code string) bool {
	_, ok := transientProviderErrorCodes[strings.ToLower(strings.TrimSpace(code))]
	return ok
}

func (s *Service) providerMaxAttempts() int {
	if s == nil || s.cfg.ProviderMaxAttempts < 1 {
		return defaultProviderMaxAttempts
	}
	return s.cfg.ProviderMaxAttempts
}

func (s *Service) canRetryAttempt(run *Run) bool {
	if run == nil {
		return false
	}
	return int(run.ProviderAttemptCount) < s.providerMaxAttempts()
}

func (s *Service) shouldRetryMaxOutputIncomplete(run *Run, resp *provider.ProviderResponse) bool {
	if run == nil || resp == nil {
		return false
	}
	if !isAnalysisPromptVersion(run.PromptVersion) {
		return false
	}
	if incompleteReason(resp) != "max_output_tokens" {
		return false
	}
	// Bounded by ProviderMaxAttempts: a second incomplete at the model cap
	// exhausts the budget and fails with full attempt artifacts (no third try).
	return s.canRetryAttempt(run)
}

// retryProviderAttempt starts a new OpenAI Response for the same logical run.
//
// Order (concurrency-safe):
//  1. Caller already persisted the terminal attempt artifact for the old response.
//  2. Create a new provider Response (OpenAI POST) first.
//  3. CAS StartNextProviderAttempt(expectedOldResponseID → newResponseID).
//  4. On CAS loss: CancelResponse(new) to avoid orphans; re-read run and continue.
//  5. On CAS win: save request summary for the new attempt; return InProgress
//     (cleanup is NOT enqueued — HandlePoll only cleans up Completed/Failed/Cancelled).
func (s *Service) retryProviderAttempt(ctx context.Context, run *Run, oldResp *provider.ProviderResponse, result *PollResult) (bool, error) {
	if s == nil || s.provider == nil || run == nil {
		return false, nil
	}
	expectedOldID := strings.TrimSpace(run.ProviderResponseID)
	if expectedOldID == "" {
		return false, nil
	}

	start, err := s.buildAndStartProviderResponse(ctx, run)
	if err != nil {
		return false, err
	}
	newResp := start.Response
	if newResp == nil || strings.TrimSpace(newResp.ID) == "" {
		return false, errorf(CodeProviderFailed, "retry start returned empty response id")
	}

	now := s.now()
	updated, casErr := s.repo.StartNextProviderAttempt(ctx, run.ID, dbgen.StartNextUnifiedAnalysisProviderAttemptParams{
		ProviderResponseID:   nullableText(expectedOldID),
		ProviderResponseID_2: nullableText(newResp.ID),
		ProviderStatus:       nullableText(string(newResp.Status)),
		ProviderRequestID:    nullableText(newResp.RequestID),
		ProviderStartedAt:    timestamptzNow(now),
		ProviderAttemptCount: int32(s.providerMaxAttempts()),
	})
	if casErr != nil {
		// Orphan cleanup: we created a Response before losing the CAS.
		_, _ = s.provider.CancelResponse(ctx, newResp.ID)
		if casErr == ErrCASConflict {
			current, getErr := s.repo.GetRun(ctx, run.ID)
			if getErr != nil {
				return false, getErr
			}
			result.Run = current
			if current != nil && current.Status == StatusProviderRunning {
				result.InProgress = true
				result.NextPollAfter = pollBackoff(s.cfg.PollIntervalSeconds, 0)
				return true, nil
			}
			// Another worker already finished the logical run.
			result.Completed = current != nil && (current.Status == StatusProviderCompleted || current.Status == StatusOutputValidating)
			result.Failed = current != nil && current.Status == StatusFailed
			result.Cancelled = current != nil && current.Status == StatusCancelled
			return true, nil
		}
		return false, casErr
	}

	if s.artifacts != nil {
		summary := start.Summary
		summary.Attempt = int(updated.ProviderAttemptCount)
		summary.RequestID = newResp.RequestID
		_ = s.artifacts.SaveProviderRequestSummary(ctx, run.ID, summary)
	}

	result.Run = updated
	result.InProgress = true
	result.Failed = false
	result.Completed = false
	result.Cancelled = false
	result.NextPollAfter = time.Second
	_ = oldResp // already persisted by caller
	return true, nil
}

type startedProviderResponse struct {
	Response *provider.ProviderResponse
	Summary  providerRequestSummary
}

// buildAndStartProviderResponse renders the prompt/schema and starts a background
// Response using the run's existing container + file (shared by StartRun + retry).
func (s *Service) buildAndStartProviderResponse(ctx context.Context, run *Run) (*startedProviderResponse, error) {
	rendered, textFormat, err := s.renderProviderPrompt(ctx, run)
	if err != nil {
		return nil, err
	}
	toolChoice := "required"
	if rendered.Profile == prompt.PromptProfileAnalysis {
		toolChoice = "auto"
	}
	maxOut := s.cfg.EffectiveMaxOutputTokens(string(rendered.Profile))
	metadata := map[string]string{
		"aivion_run_id":  fmt.Sprintf("%d", run.ID),
		"pipeline_mode":  ModeUnifiedAnalysis,
		"prompt_version": rendered.Version,
		"prompt_hash":    rendered.Hash,
		"prompt_profile": string(rendered.Profile),
	}
	resp, err := s.provider.StartBackgroundResponse(ctx, provider.StartBackgroundResponseRequest{
		Model:           firstNonEmpty(run.ProviderModel, s.cfg.Model),
		ContainerID:     run.ProviderContainerID,
		Instructions:    rendered.Instructions,
		Input:           rendered.Input,
		TextFormat:      textFormat,
		ToolChoice:      toolChoice,
		MaxOutputTokens: maxOut,
		Metadata:        metadata,
	})
	if err != nil {
		return nil, err
	}

	summary := providerRequestSummary{
		Attempt:         int(run.ProviderAttemptCount) + 1,
		Model:           firstNonEmpty(run.ProviderModel, s.cfg.Model),
		Background:      true,
		ContainerID:     run.ProviderContainerID,
		ContainerFileID: run.ProviderContainerFileID,
		ToolChoice:      toolChoice,
		MaxOutputTokens: maxOut,
		PromptProfile:   string(rendered.Profile),
		PromptVersion:   rendered.Version,
		PromptHash:      rendered.Hash,
		PipelineMode:    ModeUnifiedAnalysis,
		PackID:          run.PackID,
		PackHash:        run.PackHash,
		RequestID:       resp.RequestID,
		Metadata:        metadata,
	}
	if textFormat != nil {
		summary.SchemaName = textFormat.Name
		summary.SchemaVersion = contract.UnifiedAnalysisOutputSchemaVersionCurrent
		if h, hashErr := contract.SchemaHash(); hashErr == nil {
			summary.SchemaHash = h
		}
		strict := textFormat.Strict
		summary.Strict = &strict
	}
	return &startedProviderResponse{Response: resp, Summary: summary}, nil
}
