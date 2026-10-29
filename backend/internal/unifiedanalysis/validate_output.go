package unifiedanalysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputextract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/prompt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// ValidateOutputResult summarizes Stage 4 technical validation for a run.
type ValidateOutputResult struct {
	Run            *Run
	Valid          bool
	Blocked        bool
	Failed         bool
	Report         *outputvalidator.Report
	OutputHash     string
	Outcome        string
	ActionCount    int
	TopActionCount int
}

// ValidateOutput extracts structured output, runs the technical contract validator,
// persists artifacts, and transitions to publication_ready or publication_blocked.
// Smoke-profile runs are no-ops (Stage 3 ends at provider_completed).
func (s *Service) ValidateOutput(ctx context.Context, runID int64) (*ValidateOutputResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("unified analysis service is not configured")
	}

	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	result := &ValidateOutputResult{Run: run}

	if !isAnalysisPromptVersion(run.PromptVersion) {
		// Smoke profile never enters Stage 4 validation.
		return result, nil
	}
	if !s.cfg.OutputValidationEnabled {
		return nil, errorf(CodeInconsistentRunState, "output validation is disabled; cannot reach publication_ready")
	}

	switch run.Status {
	case StatusPublicationReady:
		result.Valid = true
		if summary, ok := s.loadOutputSummary(ctx, runID); ok {
			result.OutputHash = summary.OutputHash
			result.Outcome = summary.Outcome
			result.ActionCount = summary.ActionCount
			result.TopActionCount = summary.TopActionCount
		}
		return result, nil
	case StatusPublicationBlocked:
		result.Blocked = true
		return result, nil
	case StatusFailed, StatusCancelled:
		result.Failed = true
		return result, nil
	case StatusProviderCompleted:
		if _, err := s.repo.TransitionStatus(ctx, runID, StatusProviderCompleted, StatusOutputValidating); err != nil {
			if !errors.Is(err, ErrCASConflict) {
				return result, err
			}
			run, err = s.repo.GetRun(ctx, runID)
			if err != nil {
				return result, err
			}
			if run.Status == StatusPublicationReady {
				result.Valid = true
				result.Run = run
				return result, nil
			}
			if run.Status == StatusPublicationBlocked {
				result.Blocked = true
				result.Run = run
				return result, nil
			}
			if run.Status != StatusOutputValidating {
				return result, errorf(CodeInconsistentRunState, "unexpected status %q during validate", run.Status)
			}
		} else {
			run, _ = s.repo.GetRun(ctx, runID)
			result.Run = run
		}
	case StatusOutputValidating:
		// continue
	default:
		return result, errorf(CodeInconsistentRunState, "validate_output requires provider_completed or output_validating, got %q", run.Status)
	}

	started := s.now()

	// Idempotent re-entry: existing validated artifact with matching hash.
	if existing, ok := s.loadValidatedOutputArtifact(ctx, runID); ok && existing.OutputHash != "" {
		if _, err := s.repo.TransitionStatus(ctx, runID, StatusOutputValidating, StatusPublicationReady); err != nil && !errors.Is(err, ErrCASConflict) {
			return result, err
		}
		run, _ = s.repo.GetRun(ctx, runID)
		result.Run = run
		result.Valid = true
		result.OutputHash = existing.OutputHash
		result.Outcome = existing.Outcome
		result.ActionCount = existing.ActionCount
		result.TopActionCount = existing.TopActionCount
		return result, nil
	}

	resp, err := s.loadProviderResponseForValidation(ctx, run)
	if err != nil {
		return result, err
	}

	// Provider-level failures (refusal / incomplete) → failed, not publication_blocked.
	if strings.TrimSpace(resp.Refusal) != "" {
		if markErr := s.markFailed(ctx, runID, CodeProviderRefusal, fmt.Errorf("provider refusal")); markErr != nil {
			return result, markErr
		}
		result.Run, _ = s.repo.GetRun(ctx, runID)
		result.Failed = true
		return result, nil
	}
	if resp.Status == provider.ProviderResponseStatusIncomplete {
		if markErr := s.markFailed(ctx, runID, CodeProviderResponseIncomplete, fmt.Errorf("provider response incomplete")); markErr != nil {
			return result, markErr
		}
		result.Run, _ = s.repo.GetRun(ctx, runID)
		result.Failed = true
		return result, nil
	}
	if !provider.HasCodeInterpreterCall(resp) {
		if markErr := s.markFailed(ctx, runID, CodeSmokeValidationFailed, fmt.Errorf("missing code interpreter call")); markErr != nil {
			return result, markErr
		}
		result.Run, _ = s.repo.GetRun(ctx, runID)
		result.Failed = true
		return result, nil
	}

	extracted, err := outputextract.ExtractStructuredOutput(resp)
	if err != nil {
		code := CodeStructuredOutputMissing
		if c, ok := outputextract.CodeOf(err); ok {
			code = c
		}
		// Extraction/refusal-class codes are provider failures.
		switch code {
		case CodeProviderRefusal, CodeProviderResponseIncomplete, CodeProviderOutputTruncated,
			CodeStructuredOutputMissing, CodeStructuredOutputAmbiguous,
			outputextract.CodeProviderFailed:
			if markErr := s.markFailed(ctx, runID, code, err); markErr != nil {
				return result, markErr
			}
			result.Run, _ = s.repo.GetRun(ctx, runID)
			result.Failed = true
			return result, nil
		default:
			return s.blockPublication(ctx, run, result, nil, nil, code, err.Error(), started)
		}
	}

	if s.artifacts != nil {
		_ = s.artifacts.SaveOutputRaw(ctx, runID, extracted.JSONBytes)
	}

	out, err := outputextract.ParseOutput(extracted.JSONBytes)
	if err != nil {
		code := CodeStructuredOutputInvalidJSON
		if c, ok := outputextract.CodeOf(err); ok {
			code = c
		}
		report := &outputvalidator.Report{
			Valid:  false,
			Errors: []outputvalidator.Issue{{Code: code, Path: "$", Message: err.Error()}},
		}
		return s.blockPublication(ctx, run, result, report, extracted.JSONBytes, code, err.Error(), started)
	}

	packBytes, err := s.loadPackZip(ctx, run)
	if err != nil {
		return result, wrapError(CodeOutputArtifactSaveFailed, "load pack for validation", err)
	}
	idx, err := outputvalidator.BuildPackReferenceIndex(packBytes)
	if err != nil {
		return result, wrapError(CodePackValidationFailed, "build pack reference index: "+err.Error(), err)
	}

	meta := outputvalidator.ExpectedRunMeta{
		PackID:     run.PackID,
		PackHash:   run.PackHash,
		MetricDate: run.MetricDate.UTC().Format("2006-01-02"),
	}
	// Deterministic grounding repairs for mechanical model failures (wrong
	// pack identity, invented/duplicate SKUs, already-met targets, signal_id
	// collisions). Applied before the hard publication contract.
	_ = outputvalidator.SanitizeGrounding(out, meta, idx)
	_ = outputvalidator.SyncActionEntityCounts(out)
	_ = outputvalidator.RepairUnsupportedContentSteps(out, idx)
	// Drop mixed AI stock umbrellas (e.g. "11 дефицитных SKU" that mixes
	// healthy + deficit) so band backfill can publish mutually exclusive
	// OOS / critical / short / surplus Actions without overlap.
	_ = outputvalidator.ReconcileStockActionCohorts(out, idx)
	// Deterministic material-domain backfill: append well-formed Actions for
	// Pack-grounded problems (stock OOS/critical, high-DRR ads, spend without
	// orders, material sales decline, no-sales-with-stock, surplus) the model
	// left uncovered, so multi-problem cabinets surface 8–12 Actions instead
	// of just the 1-2 the model happened to write about. Never invents SKU/
	// campaign ids, never reuses a SKU already owned by an existing Action,
	// and never exceeds contract.MaxActionsV1. Must run before
	// FinalizeTopActionIDs so backfilled required Actions are eligible for
	// dashboard.top_action_ids.
	_ = outputvalidator.EnsureMaterialDomainActions(out, idx)
	_ = outputvalidator.EnsureCategorySummaries(out, idx)
	_ = outputvalidator.ScrubInternalFieldNames(out)
	// Re-sync after domain backfill so appended Actions also get authoritative
	// entity counts, non-generic expected_effect, and actionable plans.
	_ = outputvalidator.SyncActionEntityCounts(out)
	// Deterministic Top 3 finalization: drop non-required / unknown /
	// duplicate top_action_ids and backfill remaining slots from required
	// Actions only (never promote recommended or invent Actions).
	_ = outputvalidator.FinalizeTopActionIDs(out)
	// Deterministic risk_drivers finalization: rewrite composite AI ids
	// (high_drr_<campaign>) onto canonical healthpolicy registry codes and
	// keep entity references in entity_ids.
	_ = outputvalidator.FinalizeDashboardRiskDrivers(out, idx)
	// Section-aware fact grounding: drop campaign/SKU facts from Seller Pulse
	// (never rewrite them into account metrics).
	_ = outputvalidator.FinalizeDashboardSectionFactReferences(out)
	report := outputvalidator.Validate(out, meta, idx)
	result.Report = &report
	result.Outcome = out.AnalysisOutcome
	result.ActionCount = len(out.Actions)
	result.TopActionCount = len(out.Dashboard.TopActionIDs)

	schemaHash, _ := contract.SchemaHash()
	promptHash := ""
	if rendered, renderErr := prompt.Render(prompt.PromptProfileAnalysis); renderErr == nil {
		promptHash = rendered.Hash
	}

	if !report.Valid {
		return s.blockPublication(ctx, run, result, &report, extracted.JSONBytes, CodeOutputContractInvalid, firstValidationMessage(report), started)
	}

	canonical, err := outputvalidator.CanonicalJSON(out)
	if err != nil {
		return result, wrapError(CodeOutputArtifactSaveFailed, "canonical json", err)
	}
	outputHash := outputvalidator.OutputHash(canonical)
	result.OutputHash = outputHash

	if s.artifacts != nil {
		if err := s.artifacts.SaveOutputValidated(ctx, runID, canonical, outputHash); err != nil {
			return result, wrapError(CodeOutputArtifactSaveFailed, "save validated output", err)
		}
		_ = s.artifacts.SaveOutputValidation(ctx, runID, outputValidationArtifact{
			Valid:          true,
			Errors:         mapValidationIssues(report.Errors),
			Warnings:       mapValidationIssues(report.Warnings),
			SchemaVersion:  contract.UnifiedAnalysisOutputSchemaVersionCurrent,
			SchemaHash:     schemaHash,
			PromptVersion:  firstNonEmpty(run.PromptVersion, PromptVersionAnalyst),
			PromptHash:     promptHash,
			PackID:         run.PackID,
			PackHash:       run.PackHash,
			OutputHash:     outputHash,
			Outcome:        out.AnalysisOutcome,
			ActionCount:    len(out.Actions),
			TopActionCount: len(out.Dashboard.TopActionIDs),
			DurationMs:     s.now().Sub(started).Milliseconds(),
		})
		_ = s.artifacts.SaveOutputSummary(ctx, runID, outputSummaryArtifact{
			Outcome:        out.AnalysisOutcome,
			ActionCount:    len(out.Actions),
			TopActionCount: len(out.Dashboard.TopActionIDs),
			OutputHash:     outputHash,
			Bytes:          len(canonical),
			Valid:          true,
		})
	}

	updated, err := s.repo.TransitionStatus(ctx, runID, StatusOutputValidating, StatusPublicationReady)
	if err != nil {
		if errors.Is(err, ErrCASConflict) {
			run, _ = s.repo.GetRun(ctx, runID)
			result.Run = run
			if run != nil && run.Status == StatusPublicationReady {
				result.Valid = true
				return result, nil
			}
		}
		return result, err
	}
	result.Run = updated
	result.Valid = true
	return result, nil
}

func (s *Service) blockPublication(
	ctx context.Context,
	run *Run,
	result *ValidateOutputResult,
	report *outputvalidator.Report,
	raw []byte,
	code, message string,
	started time.Time,
) (*ValidateOutputResult, error) {
	if report == nil {
		report = &outputvalidator.Report{
			Valid:  false,
			Errors: []outputvalidator.Issue{{Code: code, Path: "$", Message: message}},
		}
	}
	result.Report = report
	schemaHash, _ := contract.SchemaHash()
	promptHash := ""
	if rendered, renderErr := prompt.Render(prompt.PromptProfileAnalysis); renderErr == nil {
		promptHash = rendered.Hash
	}
	if s.artifacts != nil {
		if len(raw) > 0 {
			_ = s.artifacts.SaveOutputRaw(ctx, run.ID, raw)
		}
		_ = s.artifacts.SaveOutputValidation(ctx, run.ID, outputValidationArtifact{
			Valid:          false,
			Errors:         mapValidationIssues(report.Errors),
			Warnings:       mapValidationIssues(report.Warnings),
			SchemaVersion:  contract.UnifiedAnalysisOutputSchemaVersionCurrent,
			SchemaHash:     schemaHash,
			PromptVersion:  firstNonEmpty(run.PromptVersion, PromptVersionAnalyst),
			PromptHash:     promptHash,
			PackID:         run.PackID,
			PackHash:       run.PackHash,
			Outcome:        result.Outcome,
			ActionCount:    result.ActionCount,
			TopActionCount: result.TopActionCount,
			DurationMs:     s.now().Sub(started).Milliseconds(),
			ErrorCode:      code,
		})
		_ = s.artifacts.SaveOutputSummary(ctx, run.ID, outputSummaryArtifact{
			Outcome:        result.Outcome,
			ActionCount:    result.ActionCount,
			TopActionCount: result.TopActionCount,
			Valid:          false,
			ErrorCode:      code,
		})
	}

	// Persist error_code on the run via MarkFailed-like fields without status=failed.
	if blocker, ok := s.repo.(outputBlockStore); ok {
		updated, err := blocker.SavePublicationBlocked(ctx, run.ID, code, message)
		if err != nil {
			// Fall back to transition only.
			if _, terr := s.repo.TransitionStatus(ctx, run.ID, StatusOutputValidating, StatusPublicationBlocked); terr != nil && !errors.Is(terr, ErrCASConflict) {
				return result, terr
			}
		} else {
			result.Run = updated
			result.Blocked = true
			return result, nil
		}
	}
	updated, err := s.repo.TransitionStatus(ctx, run.ID, StatusOutputValidating, StatusPublicationBlocked)
	if err != nil && !errors.Is(err, ErrCASConflict) {
		return result, err
	}
	if updated != nil {
		result.Run = updated
	} else {
		result.Run, _ = s.repo.GetRun(ctx, run.ID)
	}
	result.Blocked = true
	return result, nil
}

type outputBlockStore interface {
	SavePublicationBlocked(ctx context.Context, runID int64, code, message string) (*Run, error)
}

func (s *Service) loadProviderResponseForValidation(ctx context.Context, run *Run) (*provider.ProviderResponse, error) {
	if s.artifacts != nil {
		if raw, ok := s.artifacts.LoadProviderResponseRaw(ctx, run.ID); ok && len(raw) > 0 {
			resp, err := provider.ParseResponseJSON(raw)
			if err == nil {
				return resp, nil
			}
		}
	}
	if s.provider != nil && strings.TrimSpace(run.ProviderResponseID) != "" {
		return s.provider.RetrieveResponse(ctx, run.ProviderResponseID)
	}
	return nil, errorf(CodeStructuredOutputMissing, "provider response artifact unavailable")
}

type storedValidatedMeta struct {
	OutputHash     string
	Outcome        string
	ActionCount    int
	TopActionCount int
}

func (s *Service) loadValidatedOutputArtifact(ctx context.Context, runID int64) (storedValidatedMeta, bool) {
	if s.artifacts == nil {
		return storedValidatedMeta{}, false
	}
	return s.artifacts.LoadValidatedMeta(ctx, runID)
}

func (s *Service) loadOutputSummary(ctx context.Context, runID int64) (outputSummaryArtifact, bool) {
	if s.artifacts == nil {
		return outputSummaryArtifact{}, false
	}
	return s.artifacts.LoadOutputSummary(ctx, runID)
}

func isAnalysisPromptVersion(version string) bool {
	return prompt.ProfileForVersion(strings.TrimSpace(version)) == prompt.PromptProfileAnalysis
}

func firstValidationMessage(report outputvalidator.Report) string {
	if len(report.Errors) == 0 {
		return "output contract invalid"
	}
	return report.Errors[0].Code + ": " + report.Errors[0].Message
}

func mapValidationIssues(in []outputvalidator.Issue) []outputValidationIssue {
	if len(in) == 0 {
		return nil
	}
	out := make([]outputValidationIssue, len(in))
	for i, issue := range in {
		out[i] = outputValidationIssue{Code: issue.Code, Path: issue.Path, Message: issue.Message}
	}
	return out
}

func truncateErr(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}
