package unifiedanalysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
	"github.com/jackc/pgx/v5/pgtype"
)

// PollRun retrieves provider progress and completes successful smoke responses.
func (s *Service) PollRun(ctx context.Context, runID int64) (*PollResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("unified analysis service is not configured")
	}
	if s.provider == nil {
		return nil, errorf(CodeProviderDisabled, "provider is not configured")
	}

	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	result := &PollResult{Run: run}

	switch run.Status {
	case StatusProviderCompleted, StatusFailed, StatusCancelled, StatusCleanupCompleted,
		StatusPublicationReady, StatusPublicationBlocked, StatusOutputValidating:
		result.Completed = run.Status == StatusProviderCompleted || run.Status == StatusCleanupCompleted ||
			run.Status == StatusPublicationReady || run.Status == StatusOutputValidating
		result.Failed = run.Status == StatusFailed || run.Status == StatusPublicationBlocked
		result.Cancelled = run.Status == StatusCancelled
		return result, nil
	case StatusProviderRunning:
		// continue
	default:
		result.InProgress = true
		return result, nil
	}

	if strings.TrimSpace(run.ProviderResponseID) == "" {
		return nil, errorf(CodeInconsistentRunState, "provider response id is empty")
	}

	// Honor cancel requested before retrieve when cancellation was already recorded.
	if run.CancelRequestedAt != nil {
		_, _ = s.provider.CancelResponse(ctx, run.ProviderResponseID)
		if _, err := s.repo.MarkCancelled(ctx, run.ID, dbgen.MarkUnifiedAnalysisCancelledParams{
			ProviderStatus: nullableText(string(provider.ProviderResponseStatusCancelled)),
			CleanupStatus:  nullableText(CleanupStatusPending),
		}); err != nil {
			return result, err
		}
		result.Run, _ = s.repo.GetRun(ctx, runID)
		result.Cancelled = true
		return result, nil
	}

	resp, err := s.provider.RetrieveResponse(ctx, run.ProviderResponseID)
	if err != nil {
		return nil, wrapError(CodeProviderFailed, "retrieve provider response", err)
	}

	switch resp.Status {
	case provider.ProviderResponseStatusQueued, provider.ProviderResponseStatusInProgress:
		if run.ProviderStartedAt != nil {
			elapsed := s.now().Sub(*run.ProviderStartedAt)
			maxDur := time.Duration(s.cfg.MaxPollDurationSeconds) * time.Second
			if maxDur > 0 && elapsed > maxDur {
				_, _ = s.provider.CancelResponse(ctx, run.ProviderResponseID)
				if markErr := s.markFailed(ctx, run.ID, CodeProviderPollTimeout, fmt.Errorf("poll exceeded %s", maxDur)); markErr != nil {
					return result, markErr
				}
				result.Run, _ = s.repo.GetRun(ctx, runID)
				result.Failed = true
				return result, nil
			}
		}
		_, _ = s.repo.UpdateProviderStatus(ctx, run.ID, string(resp.Status), StatusProviderRunning)
		result.Run, _ = s.repo.GetRun(ctx, runID)
		result.InProgress = true
		result.NextPollAfter = pollBackoff(s.cfg.PollIntervalSeconds, 0)
		return result, nil
	case provider.ProviderResponseStatusCancelled:
		s.persistTerminalAttempt(ctx, run, resp, latencyMs(run, resp, s.now()))
		_, _ = s.repo.MarkCancelled(ctx, run.ID, dbgen.MarkUnifiedAnalysisCancelledParams{
			ProviderStatus: nullableText(string(resp.Status)),
			CleanupStatus:  nullableText(CleanupStatusPending),
		})
		result.Run, _ = s.repo.GetRun(ctx, runID)
		result.Cancelled = true
		return result, nil
	case provider.ProviderResponseStatusFailed:
		return s.handleTerminalFailed(ctx, run, resp, result)
	case provider.ProviderResponseStatusIncomplete:
		return s.handleTerminalIncomplete(ctx, run, resp, result)
	case provider.ProviderResponseStatusCompleted:
		// Completed provider result wins over a concurrent cancel request.
		return s.handleTerminalCompleted(ctx, run, resp, result)
	default:
		return result, errorf(CodeInconsistentRunState, "unknown provider status %q", resp.Status)
	}
}

func (s *Service) handleTerminalIncomplete(ctx context.Context, run *Run, resp *provider.ProviderResponse, result *PollResult) (*PollResult, error) {
	latency := latencyMs(run, resp, s.now())
	s.persistTerminalAttempt(ctx, run, resp, latency)

	reason := incompleteReason(resp)
	if s.shouldRetryMaxOutputIncomplete(run, resp) {
		retried, err := s.retryProviderAttempt(ctx, run, resp, result)
		if err != nil {
			return result, err
		}
		if retried {
			return result, nil
		}
	}

	msg := "provider response incomplete"
	if reason != "" {
		msg = fmt.Sprintf("provider response incomplete: %s", reason)
	}
	if err := s.saveProviderTerminalFailure(ctx, run, resp, latency, CodeProviderResponseIncomplete, msg); err != nil {
		return result, err
	}
	result.Run, _ = s.repo.GetRun(ctx, run.ID)
	result.Failed = true
	return result, nil
}

func (s *Service) handleTerminalFailed(ctx context.Context, run *Run, resp *provider.ProviderResponse, result *PollResult) (*PollResult, error) {
	latency := latencyMs(run, resp, s.now())
	s.persistTerminalAttempt(ctx, run, resp, latency)

	errCode := ""
	errMsg := fmt.Sprintf("provider response status %s", resp.Status)
	if resp.Error != nil {
		errCode = strings.TrimSpace(resp.Error.Code)
		if strings.TrimSpace(resp.Error.Message) != "" {
			errMsg = strings.TrimSpace(resp.Error.Message)
		}
	}
	if isTransientProviderErrorCode(errCode) && s.canRetryAttempt(run) {
		retried, err := s.retryProviderAttempt(ctx, run, resp, result)
		if err != nil {
			return result, err
		}
		if retried {
			return result, nil
		}
	}
	if err := s.saveProviderTerminalFailure(ctx, run, resp, latency, CodeProviderFailed, errMsg); err != nil {
		return result, err
	}
	result.Run, _ = s.repo.GetRun(ctx, run.ID)
	result.Failed = true
	return result, nil
}

func (s *Service) handleTerminalCompleted(ctx context.Context, run *Run, resp *provider.ProviderResponse, result *PollResult) (*PollResult, error) {
	latency := latencyMs(run, resp, s.now())
	s.persistTerminalAttempt(ctx, run, resp, latency)

	if err := validateProviderCompletion(resp, run); err != nil {
		code := CodeSmokeValidationFailed
		if ue, ok := asError(err); ok {
			code = ue.Code
		}
		if code == CodeStructuredOutputMissing && s.canRetryAttempt(run) && isAnalysisPromptVersion(run.PromptVersion) {
			retried, retryErr := s.retryProviderAttempt(ctx, run, resp, result)
			if retryErr != nil {
				return result, retryErr
			}
			if retried {
				return result, nil
			}
		}
		if saveErr := s.saveProviderTerminalFailure(ctx, run, resp, latency, code, err.Error()); saveErr != nil {
			return result, saveErr
		}
		result.Run, _ = s.repo.GetRun(ctx, run.ID)
		result.Failed = true
		return result, nil
	}
	return s.completeSuccessfulPoll(ctx, run, resp, result, false /* already persisted */)
}

func (s *Service) completeSuccessfulPoll(ctx context.Context, run *Run, resp *provider.ProviderResponse, result *PollResult, persistArtifacts bool) (*PollResult, error) {
	now := s.now()
	latency := latencyMs(run, resp, now)
	ciCalls := int32(len(resp.CodeInterpreterCalls))

	if persistArtifacts {
		s.persistTerminalAttempt(ctx, run, resp, latency)
	}

	completedAt := now
	if resp.CompletedAt != nil {
		completedAt = resp.CompletedAt.UTC()
	}

	nextStatus := StatusProviderCompleted
	if isAnalysisPromptVersion(run.PromptVersion) && s.cfg.OutputValidationEnabled {
		nextStatus = StatusOutputValidating
	}

	updated, err := s.repo.SaveResponseCompleted(ctx, run.ID, dbgen.SaveUnifiedAnalysisResponseCompletedParams{
		Status:               nextStatus,
		ProviderStatus:       nullableText(string(resp.Status)),
		ProviderCompletedAt:  timestamptzNow(completedAt),
		ProviderLatencyMs:    pgtype.Int8{Int64: latency, Valid: true},
		InputTokens:          pgtype.Int8{Int64: resp.Usage.InputTokens, Valid: true},
		OutputTokens:         pgtype.Int8{Int64: resp.Usage.OutputTokens, Valid: true},
		TotalTokens:          pgtype.Int8{Int64: resp.Usage.TotalTokens, Valid: true},
		CodeInterpreterCalls: pgtype.Int4{Int32: ciCalls, Valid: true},
		CleanupStatus:        nullableText(CleanupStatusPending),
		CompletedAt:          timestamptzNow(completedAt),
	})
	if err != nil {
		if errors.Is(err, ErrCASConflict) {
			result.Run, _ = s.repo.GetRun(ctx, run.ID)
			if result.Run != nil && (result.Run.Status == StatusProviderCompleted || result.Run.Status == StatusOutputValidating ||
				result.Run.Status == StatusPublicationReady || result.Run.Status == StatusPublicationBlocked) {
				result.Completed = true
				return result, nil
			}
		}
		return result, err
	}

	result.Run = updated
	result.Completed = true
	return result, nil
}

func (s *Service) persistTerminalAttempt(ctx context.Context, run *Run, resp *provider.ProviderResponse, latency int64) {
	if s.artifacts == nil || resp == nil || run == nil {
		return
	}
	attempt := buildTerminalAttemptArtifact(run, resp, latency)
	_ = s.artifacts.SaveProviderTerminalAttempt(ctx, run.ID, attempt, resp.RawJSON)
}

func buildTerminalAttemptArtifact(run *Run, resp *provider.ProviderResponse, latency int64) providerTerminalAttemptArtifact {
	attemptN := int(run.ProviderAttemptCount)
	if attemptN <= 0 {
		attemptN = 1
	}
	art := providerTerminalAttemptArtifact{
		Attempt:               attemptN,
		ResponseID:            resp.ID,
		Status:                string(resp.Status),
		RequestID:             firstNonEmpty(run.ProviderRequestID, resp.RequestID),
		RetrieveRequestID:     resp.RequestID,
		Model:                 resp.Model,
		IncompleteReason:      incompleteReason(resp),
		RefusalPresent:        strings.TrimSpace(resp.Refusal) != "",
		AssistantMessageCount: resp.AssistantMessageCount,
		OutputTextBytes:       len(resp.OutputText),
		OutputItemTypes:       append([]string(nil), resp.OutputItemTypes...),
		CodeInterpreterCalls:  len(resp.CodeInterpreterCalls),
		InputTokens:           resp.Usage.InputTokens,
		OutputTokens:          resp.Usage.OutputTokens,
		TotalTokens:           resp.Usage.TotalTokens,
		RawBytes:              len(resp.RawJSON),
		LatencyMs:             latency,
	}
	if resp.Error != nil {
		art.ProviderErrorCode = resp.Error.Code
		art.ProviderErrorMessage = resp.Error.Message
	}
	return art
}

func (s *Service) saveProviderTerminalFailure(ctx context.Context, run *Run, resp *provider.ProviderResponse, latency int64, code, message string) error {
	ciCalls := int32(0)
	providerStatus := ""
	inputTok, outputTok, totalTok := int64(0), int64(0), int64(0)
	completedAt := s.now()
	if resp != nil {
		providerStatus = string(resp.Status)
		ciCalls = int32(len(resp.CodeInterpreterCalls))
		inputTok = resp.Usage.InputTokens
		outputTok = resp.Usage.OutputTokens
		totalTok = resp.Usage.TotalTokens
		if resp.CompletedAt != nil {
			completedAt = resp.CompletedAt.UTC()
		}
	}
	_, err := s.repo.SaveProviderTerminalFailure(ctx, run.ID, dbgen.SaveUnifiedAnalysisProviderTerminalFailureParams{
		ProviderStatus:       nullableText(providerStatus),
		ProviderCompletedAt:  timestamptzNow(completedAt),
		ProviderLatencyMs:    pgtype.Int8{Int64: latency, Valid: true},
		InputTokens:          pgtype.Int8{Int64: inputTok, Valid: true},
		OutputTokens:         pgtype.Int8{Int64: outputTok, Valid: true},
		TotalTokens:          pgtype.Int8{Int64: totalTok, Valid: true},
		CodeInterpreterCalls: pgtype.Int4{Int32: ciCalls, Valid: true},
		ErrorCode:            nullableText(code),
		ErrorMessage:         nullableText(message),
		CleanupStatus:        nullableText(CleanupStatusPending),
	})
	return err
}

func incompleteReason(resp *provider.ProviderResponse) string {
	if resp == nil || resp.IncompleteDetails == nil {
		return ""
	}
	return strings.TrimSpace(resp.IncompleteDetails.Reason)
}

func latencyMs(run *Run, resp *provider.ProviderResponse, now time.Time) int64 {
	if run == nil || run.ProviderStartedAt == nil {
		return 0
	}
	if resp != nil && resp.CompletedAt != nil {
		return resp.CompletedAt.Sub(*run.ProviderStartedAt).Milliseconds()
	}
	return now.Sub(*run.ProviderStartedAt).Milliseconds()
}

// pollBackoff returns a deterministic poll delay. Base interval is used for early
// attempts; after a few polls it steps up to 10s then 15s (capped).
func pollBackoff(baseSeconds, attempt int) time.Duration {
	base := time.Duration(baseSeconds) * time.Second
	if base <= 0 {
		base = 5 * time.Second
	}
	switch {
	case attempt <= 2:
		return base
	case attempt <= 4:
		if base < 10*time.Second {
			return 10 * time.Second
		}
		return base
	default:
		if base < 15*time.Second {
			return 15 * time.Second
		}
		return base
	}
}
