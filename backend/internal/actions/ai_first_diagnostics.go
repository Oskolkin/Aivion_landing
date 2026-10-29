package actions

import (
	"fmt"
	"log/slog"
	"strings"
)

// AIFirstActionOutcomeDiagnostic records per-action pipeline outcome for debug artifacts.
type AIFirstActionOutcomeDiagnostic struct {
	AIIndex         int      `json:"ai_index"`
	ActionType      string   `json:"action_type,omitempty"`
	ActionKey       string   `json:"action_key,omitempty"`
	Outcome         string   `json:"outcome"`
	DropReasonCodes []string `json:"drop_reason_codes,omitempty"`
	WarningCodes    []string `json:"warning_codes,omitempty"`
}

const (
	actionOutcomePersisted            = "persisted"
	actionOutcomeDroppedNormalization = "dropped_normalization"
	actionOutcomeDroppedValidation    = "dropped_validation"
	actionOutcomeDroppedLimit         = "dropped_limit"
	actionOutcomeDroppedPersistGuard  = "dropped_persist_guard"
)

// AIFirstPipelineDiagnostics aggregates action loss and quality diagnostics.
type AIFirstPipelineDiagnostics struct {
	ReturnedCount                   int `json:"actions_returned_count"`
	NormalizationActionDroppedCount int `json:"actions_normalization_action_dropped_count"`
	AfterNormalizationCount         int `json:"actions_after_normalization_count"`
	ValidationActionDroppedCount    int `json:"actions_validation_action_dropped_count"`
	AfterValidationCount            int `json:"actions_after_validation_count"`
	LimitDroppedCount               int `json:"actions_limit_dropped_count"`
	AfterLimitCount                 int `json:"actions_after_limit_count"`
	PersistGuardActionDroppedCount  int `json:"actions_persist_guard_action_dropped_count"`
	AfterPersistGuardCount          int `json:"actions_after_persist_guard_count"`
	FinalCount                      int `json:"actions_final_count"`

	DropIssueCounts    map[string]int `json:"actions_drop_issue_counts,omitempty"`
	WarningIssueCounts map[string]int `json:"actions_warning_issue_counts,omitempty"`

	CompletionConditionsReturnedCount      int `json:"actions_completion_conditions_returned_count"`
	CompletionConditionsValidCount         int `json:"actions_completion_conditions_valid_count"`
	CompletionConditionsDroppedCount       int `json:"actions_completion_conditions_dropped_count"`
	CompletionConditionsFactIDRemovedCount int `json:"actions_completion_conditions_fact_id_removed_count"`
	CompletionConditionsEmptyActionsCount  int `json:"actions_completion_conditions_empty_actions_count"`

	CandidateCoverage map[string]any                   `json:"actions_candidate_coverage,omitempty"`
	ActionOutcomes    []AIFirstActionOutcomeDiagnostic `json:"action_outcomes,omitempty"`

	InputEstimatedTokens         int     `json:"actions_input_estimated_tokens,omitempty"`
	InputActualTokens            int     `json:"actions_input_actual_tokens,omitempty"`
	InputHeadroomEstimatedTokens int     `json:"actions_input_headroom_estimated_tokens,omitempty"`
	InputHeadroomActualTokens    int     `json:"actions_input_headroom_actual_tokens,omitempty"`
	InputEstimationErrorTokens   int     `json:"actions_input_estimation_error_tokens,omitempty"`
	InputEstimationErrorPct      float64 `json:"actions_input_estimation_error_pct,omitempty"`

	ActionsWithCompletionConditionsCount    int     `json:"actions_with_completion_conditions_count,omitempty"`
	ActionsWithoutCompletionConditionsCount int     `json:"actions_without_completion_conditions_count,omitempty"`
	ActionsCompletionConditionsCoveragePct  float64 `json:"actions_completion_conditions_coverage_pct,omitempty"`
	ActionsCompletionDisplayMismatchCount   int     `json:"actions_completion_display_mismatch_count,omitempty"`
	ActionsVagueCompletionCriteriaCount     int     `json:"actions_vague_completion_criteria_count,omitempty"`

	ActionsEntityScopedCount  int `json:"actions_entity_scoped_count,omitempty"`
	ActionsAccountScopedCount int `json:"actions_account_scoped_count,omitempty"`
	ActionsMixedScopedCount   int `json:"actions_mixed_scoped_count,omitempty"`

	ActionsObjectActionsFullCount          int     `json:"actions_object_actions_full_count,omitempty"`
	ActionsObjectActionsPartialCount       int     `json:"actions_object_actions_partial_count,omitempty"`
	ActionsObjectActionsMissingCount       int     `json:"actions_object_actions_missing_count,omitempty"`
	ActionsAffectedEntitiesTotalCount      int     `json:"actions_affected_entities_total_count,omitempty"`
	ActionsObjectActionsReturnedTotalCount int     `json:"actions_object_actions_returned_total_count,omitempty"`
	ActionsObjectActionsCoveragePct        float64 `json:"actions_object_actions_coverage_pct,omitempty"`

	ActionsRequestedMaxOutputTokens           int  `json:"actions_requested_max_output_tokens,omitempty"`
	ActionsProviderOutputTokens               int  `json:"actions_provider_output_tokens,omitempty"`
	ActionsOutputTokensExceededRequestedLimit bool `json:"actions_output_tokens_exceeded_requested_limit,omitempty"`

	ActionsComposeCalls                    int    `json:"actions_compose_calls,omitempty"`
	ActionsComposeInputTokens              int    `json:"actions_compose_input_tokens,omitempty"`
	ActionsComposeOutputTokens             int    `json:"actions_compose_output_tokens,omitempty"`
	ActionsComposeRequestedMaxOutputTokens int    `json:"actions_compose_requested_max_output_tokens,omitempty"`
	ActionsComposeEffectiveMaxOutputTokens int    `json:"actions_compose_effective_max_output_tokens,omitempty"`
	ActionsComposeOutputLimitWiring        string `json:"actions_compose_output_limit_wiring,omitempty"`
	ActionsComposeProviderOutputTokens     int    `json:"actions_compose_provider_output_tokens,omitempty"`
	ActionsComposeOutputTruncated          bool   `json:"actions_compose_output_truncated,omitempty"`
	ActionsComposeReturnedCount            int    `json:"actions_compose_returned_count,omitempty"`
	ActionsComposeValidCount               int    `json:"actions_compose_valid_count,omitempty"`
	ActionsComposeDroppedCount             int    `json:"actions_compose_dropped_count,omitempty"`

	ActionsObjectDetailCalls                    int `json:"actions_object_detail_calls,omitempty"`
	ActionsObjectDetailInputTokens              int `json:"actions_object_detail_input_tokens,omitempty"`
	ActionsObjectDetailOutputTokens             int `json:"actions_object_detail_output_tokens,omitempty"`
	ActionsObjectDetailRequestedMaxOutputTokens int `json:"actions_object_detail_requested_max_output_tokens,omitempty"`
	ActionsTotalAICalls                         int `json:"actions_total_ai_calls,omitempty"`

	ActionsObjectDetailBatchesTotal     int `json:"actions_object_detail_batches_total,omitempty"`
	ActionsObjectDetailBatchesCompleted int `json:"actions_object_detail_batches_completed,omitempty"`
	ActionsObjectDetailBatchesPartial   int `json:"actions_object_detail_batches_partial,omitempty"`
	ActionsObjectDetailBatchesFailed    int `json:"actions_object_detail_batches_failed,omitempty"`
	ActionsObjectDetailBatchesTruncated int `json:"actions_object_detail_batches_truncated,omitempty"`

	ActionsObjectDetailsEntitiesTotal     int     `json:"actions_object_details_entities_total,omitempty"`
	ActionsObjectDetailsEntitiesCompleted int     `json:"actions_object_details_entities_completed,omitempty"`
	ActionsObjectDetailsEntitiesMissing   int     `json:"actions_object_details_entities_missing,omitempty"`
	ActionsObjectDetailsCoveragePct       float64 `json:"actions_object_details_coverage_pct,omitempty"`

	Execution *ActionsTwoPhaseExecutionDiagnostics `json:"actions_execution,omitempty"`
}

type aiFirstPersistGuardResult struct {
	Items           []ActionItem
	DroppedCount    int
	DropIssueCounts map[string]int
	Outcomes        []AIFirstActionOutcomeDiagnostic
}

func persistGuardErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "missing affected entities"):
		return "empty_authoritative_scope"
	case strings.Contains(msg, "missing evidence_fact_ids"), strings.Contains(msg, "no valid evidence"):
		return pilotIssueNoValidEvidenceAfterDerivation
	case strings.Contains(msg, "missing evidence"):
		return "missing_evidence"
	case strings.Contains(msg, "missing completion_criteria"):
		return "missing_completion_criteria"
	case strings.Contains(msg, "missing funnel transition"):
		return "missing_funnel_transition"
	case strings.Contains(msg, "not allowed for ai-first"):
		return "persist_guard_not_allowed"
	default:
		return "persist_guard_rejected"
	}
}

func mapAIFirstDraftsWithDiagnostics(
	metricDateStr string,
	input ActionsAIFirstInput,
	drafts []AIFirstDraftAction,
	warningsByIndex map[int][]string,
) aiFirstPersistGuardResult {
	result := aiFirstPersistGuardResult{
		Items:           make([]ActionItem, 0, len(drafts)),
		DropIssueCounts: map[string]int{},
		Outcomes:        make([]AIFirstActionOutcomeDiagnostic, 0, len(drafts)),
	}
	for i, draft := range drafts {
		item := MapAIFirstActionToItem(metricDateStr, input, draft)
		outcome := AIFirstActionOutcomeDiagnostic{
			AIIndex:      i,
			ActionType:   string(item.Type),
			ActionKey:    item.ActionKey,
			WarningCodes: append([]string(nil), warningsByIndex[i]...),
		}
		if guardErr := ValidateAIFirstPersistableActionItem(item); guardErr != nil {
			slog.Warn("actions ai-first: persist guard dropped action", "type", item.Type, "err", guardErr)
			code := persistGuardErrorCode(guardErr)
			outcome.Outcome = actionOutcomeDroppedPersistGuard
			outcome.DropReasonCodes = []string{code}
			result.DropIssueCounts[code]++
			result.DroppedCount++
			result.Outcomes = append(result.Outcomes, outcome)
			continue
		}
		outcome.Outcome = actionOutcomePersisted
		result.Items = append(result.Items, item)
		result.Outcomes = append(result.Outcomes, outcome)
	}
	return result
}

func buildWarningIssueCounts(issues []ValidationIssue) map[string]int {
	counts := map[string]int{}
	for _, issue := range issues {
		if issue.Severity != IssueSeverityWarning {
			continue
		}
		counts[issue.Code]++
	}
	return counts
}

func warningsByActionIndex(issues []ValidationIssue) map[int][]string {
	out := map[int][]string{}
	for _, issue := range issues {
		if issue.Severity != IssueSeverityWarning || issue.ActionIndex == nil {
			continue
		}
		idx := *issue.ActionIndex
		out[idx] = append(out[idx], issue.Code)
	}
	return out
}

func mergeIssueCounts(into map[string]int, from map[string]int) {
	for k, v := range from {
		into[k] += v
	}
}

func buildAIFirstPipelineDiagnostics(
	returnedCount int,
	normalizationDropped int,
	afterNormalization int,
	pilot PilotActionsValidationResult,
	persist aiFirstPersistGuardResult,
	allIssues []ValidationIssue,
	candidateCoverage map[string]any,
) AIFirstPipelineDiagnostics {
	dropCounts := map[string]int{}
	for k, v := range pilot.DropIssueCounts {
		dropCounts[k] = v
	}
	mergeIssueCounts(dropCounts, persist.DropIssueCounts)

	diag := AIFirstPipelineDiagnostics{
		ReturnedCount:                   returnedCount,
		NormalizationActionDroppedCount: normalizationDropped,
		AfterNormalizationCount:         afterNormalization,
		ValidationActionDroppedCount:    pilot.DroppedCount,
		AfterValidationCount:            pilot.ValidBeforeLimit,
		LimitDroppedCount:               pilot.TruncatedCount,
		AfterLimitCount:                 pilot.AcceptedCount,
		PersistGuardActionDroppedCount:  persist.DroppedCount,
		AfterPersistGuardCount:          len(persist.Items),
		FinalCount:                      len(persist.Items),
		DropIssueCounts:                 dropCounts,
		WarningIssueCounts:              buildWarningIssueCounts(allIssues),
		CandidateCoverage:               candidateCoverage,
		ActionOutcomes:                  persist.Outcomes,
	}
	enrichAIFirstExtendedDiagnostics(&diag, persist.Items, allIssues)
	return diag
}

func enrichAIFirstExtendedDiagnostics(diag *AIFirstPipelineDiagnostics, items []ActionItem, issues []ValidationIssue) {
	if diag == nil {
		return
	}
	for _, item := range items {
		if len(item.CompletionConditions) > 0 {
			diag.ActionsWithCompletionConditionsCount++
		} else {
			diag.ActionsWithoutCompletionConditionsCount++
		}
		scope := resolvePersistScopeType(item)
		switch scope {
		case AIFirstActionScopeAccount:
			diag.ActionsAccountScopedCount++
		case AIFirstActionScopeEntity:
			diag.ActionsEntityScopedCount++
		case AIFirstActionScopeMixed:
			diag.ActionsMixedScopedCount++
		}

		if scope == AIFirstActionScopeEntity || scope == AIFirstActionScopeMixed {
			total := item.AffectedObjectsTotal
			if total == 0 {
				total = countAffectedEntityObjects(item.AffectedEntities)
			}
			displayed := item.AffectedObjectsDisplayed
			if displayed == 0 && item.OperationalDetail != nil {
				displayed = len(item.OperationalDetail.ObjectActions)
			}
			diag.ActionsAffectedEntitiesTotalCount += total
			diag.ActionsObjectActionsReturnedTotalCount += displayed
			switch {
			case total > 0 && displayed >= total:
				diag.ActionsObjectActionsFullCount++
			case total > 0 && displayed > 0 && displayed < total:
				diag.ActionsObjectActionsPartialCount++
			case total > 0 && displayed == 0:
				diag.ActionsObjectActionsMissingCount++
			}
		}
	}
	persisted := diag.ActionsWithCompletionConditionsCount + diag.ActionsWithoutCompletionConditionsCount
	if persisted > 0 {
		diag.ActionsCompletionConditionsCoveragePct = float64(diag.ActionsWithCompletionConditionsCount) / float64(persisted) * 100
	}
	if diag.ActionsAffectedEntitiesTotalCount > 0 {
		diag.ActionsObjectActionsCoveragePct = float64(diag.ActionsObjectActionsReturnedTotalCount) / float64(diag.ActionsAffectedEntitiesTotalCount) * 100
	}
	for _, issue := range issues {
		if issue.Severity != IssueSeverityWarning {
			continue
		}
		switch issue.Code {
		case "completion_display_conditions_mismatch":
			diag.ActionsCompletionDisplayMismatchCount++
		case "vague_completion_criteria_phrase":
			diag.ActionsVagueCompletionCriteriaCount++
		}
	}
}

func countAffectedEntityObjects(entities ActionAffectedEntities) int {
	n := len(entities.SKUs) + len(entities.Campaigns)
	if entities.Funnel != nil {
		n++
	}
	return n
}

func applyActionsOutputTokenDiagnostics(diag *AIFirstPipelineDiagnostics, providerOutputTokens int) {
	if diag == nil {
		return
	}
	requested := ActionsAIFirstRequestedMaxOutputTokens()
	diag.ActionsRequestedMaxOutputTokens = requested
	diag.ActionsProviderOutputTokens = providerOutputTokens
	if providerOutputTokens > requested {
		diag.ActionsOutputTokensExceededRequestedLimit = true
		if diag.WarningIssueCounts == nil {
			diag.WarningIssueCounts = map[string]int{}
		}
		diag.WarningIssueCounts["provider_output_tokens_exceed_requested_limit"]++
	}
}

func (d AIFirstPipelineDiagnostics) invariantOK() bool {
	left := d.NormalizationActionDroppedCount + d.ValidationActionDroppedCount + d.LimitDroppedCount + d.PersistGuardActionDroppedCount + d.FinalCount
	return left == d.ReturnedCount
}

func (d AIFirstPipelineDiagnostics) invariantError() error {
	if d.invariantOK() {
		return nil
	}
	left := d.NormalizationActionDroppedCount + d.ValidationActionDroppedCount + d.LimitDroppedCount + d.PersistGuardActionDroppedCount + d.FinalCount
	return fmt.Errorf("actions diagnostics invariant failed: returned=%d accounted=%d", d.ReturnedCount, left)
}

// ApplyActionsTokenDiagnostics records estimated vs provider-reported input token usage.
func ApplyActionsTokenDiagnostics(diag *AIFirstPipelineDiagnostics, estimated, actual, maxInput int) {
	if diag == nil {
		return
	}
	diag.InputEstimatedTokens = estimated
	if estimated > 0 && maxInput > estimated {
		diag.InputHeadroomEstimatedTokens = maxInput - estimated
	}
	if actual <= 0 {
		return
	}
	diag.InputActualTokens = actual
	if maxInput > actual {
		diag.InputHeadroomActualTokens = maxInput - actual
	}
	if estimated > 0 {
		diag.InputEstimationErrorTokens = actual - estimated
		diag.InputEstimationErrorPct = float64(diag.InputEstimationErrorTokens) / float64(estimated) * 100
	}
}
