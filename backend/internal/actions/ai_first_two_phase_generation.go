package actions

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

func (s *GenerationService) generateActionsAIFirstV2TwoPhase(
	ctx context.Context,
	input ActionsAIFirstInput,
	opts GenerateActionsOptions,
	aiClient ActionsAIFirstClient,
	runID, sellerAccountID int64,
	metricDate time.Time,
	metricDateStr string,
) (*GenerateActionsResult, error) {
	wireInput, wireErr := marshalActionsAIFirstWirePayload(input)
	if wireErr != nil {
		return nil, wireErr
	}
	stageStarted := time.Now().UTC()
	composeTimeout := ActionsComposeTimeoutSeconds()
	composeCtx, composeCancel := perCallTimeoutContext(ctx, composeTimeout)
	composeStarted := time.Now().UTC()
	composeOutput, composeRaw, composeErr := aiClient.ComposeAIFirstCompose(composeCtx, input)
	composeCancel()
	composeFinished := time.Now().UTC()
	if composeErr != nil {
		slog.Warn("actions ai-first v2: compose failed", "err", composeErr)
		code := ActionsErrorProviderFailed
		if strings.Contains(composeErr.Error(), ActionsErrorInvalidJSON) {
			code = ActionsErrorInvalidJSON
		}
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID: runID, SellerID: sellerAccountID, MetricDate: metricDate, MetricDateStr: metricDateStr,
			Input: input, RawResponse: composeRaw, FinalStatus: GenerationFinalStatusFailed,
			FailureSubreason: code, ComposeCalls: 1, Notes: []string{code + ": " + composeErr.Error()},
			ValidatorMode: PilotValidatorMode, Opts: opts, DryRun: opts.DryRun,
		})
	}

	composeMaxTokens := ActionsComposeRequestedMaxOutputTokens()
	composeTruncated := IsResponseOutputTruncated(composeRaw, composeMaxTokens)
	if composeTruncated {
		return s.finishAIFirstRun(ctx, aiFirstRunState{
			RunID: runID, SellerID: sellerAccountID, MetricDate: metricDate, MetricDateStr: metricDateStr,
			Input: input, RawResponse: composeRaw, FinalStatus: GenerationFinalStatusFailed,
			FailureSubreason: ActionsErrorComposeOutputTruncated, ComposeCalls: 1,
			Notes:         []string{ActionsErrorComposeOutputTruncated},
			ValidatorMode: PilotValidatorMode, Opts: opts, DryRun: opts.DryRun,
			PipelineDiagnostics: &AIFirstPipelineDiagnostics{
				ActionsComposeRequestedMaxOutputTokens: composeMaxTokens,
				ActionsComposeOutputTruncated:          true,
			},
		})
	}

	rawReturnedActions := composeActionsToRawDrafts(composeOutput)
	baseActions, composeNorm, normEvents := NormalizeComposeActions(input, *composeOutput)

	drafts := make([]AIFirstDraftAction, 0, len(baseActions))
	detailMerges, detailCalls, detailInTokens, detailOutTokens, detailEvents := RunObjectDetailBatches(ctx, aiClient, input, baseActions)
	normEvents = append(normEvents, detailEvents...)

	detailPartial := false
	for i, base := range baseActions {
		draft := baseActionToDraft(base)
		if i < len(detailMerges) {
			draft = finalizeDraftAfterObjectDetail(draft, detailMerges[i])
			if detailMerges[i].DetailCoveragePartial || detailMerges[i].Truncated {
				detailPartial = true
			}
		}
		objectCount := len(base.AuthoritativeEntities)
		computedMap, computeIssues := executeComputedValues(input, draft.ComputedValues, base.AIIndex)
		composeNorm.Issues = append(composeNorm.Issues, computeIssues...)
		draft = renderActionTemplates(draft, objectCount, computedMap)
		drafts = append(drafts, draft)
	}

	legacyOutput := ActionsAIFirstOutput{
		SchemaVersion: composeOutput.SchemaVersion,
		Focus:         composeOutput.Focus,
		Actions:       drafts,
		Notes:         composeOutput.Notes,
	}
	normOutput, postNorm := finalizeTwoPhaseDrafts(input, legacyOutput)
	allNormIssues := append(composeNorm.Issues, postNorm.Issues...)
	output := &normOutput

	pilot := ValidateActionsAIFirstOutputPilot(input, *output)
	pilot.Issues = append(pilot.Issues, allNormIssues...)
	allIssues := pilot.Issues
	warningsByIndex := warningsByActionIndex(allIssues)
	persistResult := mapAIFirstDraftsWithDiagnostics(metricDateStr, input, pilot.Accepted, warningsByIndex)

	persistedCandidateKeys := map[string]bool{}
	for _, item := range persistResult.Items {
		for _, id := range item.SourceCandidateIDs {
			persistedCandidateKeys[strings.TrimSpace(id)] = true
		}
	}
	candidateCoverage := BuildActionsCandidateCoverageDiagnostics(
		input, rawReturnedActions, pilot.Accepted, pilot.Dropped, persistedCandidateKeys,
	)

	lifecycleSummary, actionLifecycles := BuildActionLifecycleDiagnostics(
		composeOutput, baseActions, pilot, persistResult,
	)

	totalComposeDropped := composeNorm.DroppedActionCount
	pipelineDiag := buildAIFirstPipelineDiagnostics(
		len(composeOutput.Actions),
		totalComposeDropped,
		len(output.Actions),
		pilot,
		persistResult,
		allIssues,
		candidateCoverage,
	)
	enrichTwoPhaseDiagnostics(&pipelineDiag, composeRaw, composeMaxTokens, detailCalls, detailInTokens, detailOutTokens, detailMerges)
	pipelineDiag.Execution = buildTwoPhaseExecutionDiagnostics(
		ctx, stageStarted, composeStarted, composeFinished, "completed", false, detailMerges,
	)
	pipelineDiag.CompletionConditionsReturnedCount = composeNorm.CompletionConditionsReturnedCount + postNorm.CompletionConditionsReturnedCount
	pipelineDiag.CompletionConditionsValidCount = composeNorm.CompletionConditionsValidCount + postNorm.CompletionConditionsValidCount
	pipelineDiag.CompletionConditionsDroppedCount = composeNorm.CompletionConditionsDroppedCount + postNorm.CompletionConditionsDroppedCount
	pipelineDiag.CompletionConditionsFactIDRemovedCount = composeNorm.CompletionConditionsFactIDRemovedCount + postNorm.CompletionConditionsFactIDRemovedCount
	pipelineDiag.CompletionConditionsEmptyActionsCount = composeNorm.CompletionConditionsEmptyActionsCount + postNorm.CompletionConditionsEmptyActionsCount

	validation := pilotValidationToLegacyResult(pilot)
	items := persistResult.Items
	validationDropped := totalComposeDropped + pilot.DroppedCount + persistResult.DroppedCount
	totalDropped := validationDropped + pilot.TruncatedCount

	finalStatus := GenerationFinalStatusCompleted
	failureSubreason := ""
	validCandidateCount := len(input.ActionCandidates)
	switch {
	case validCandidateCount > 0 && len(composeOutput.Actions) == 0:
		finalStatus = GenerationFinalStatusFailed
		failureSubreason = ActionsErrorEmptyOutputFromNonEmptyCandidates
	case len(items) == 0 && len(composeOutput.Actions) == 0:
		finalStatus = GenerationFinalStatusFailed
		failureSubreason = ActionsErrorNoValidActions
	case len(items) == 0 && len(composeOutput.Actions) > 0:
		finalStatus = GenerationFinalStatusFailed
		failureSubreason = ActionsErrorAllDropped
	case validationDropped > 0 || persistResult.DroppedCount > 0 || detailPartial:
		finalStatus = GenerationFinalStatusCompletedPartial
	}

	focusValid := output.Focus != nil
	aiFocus := BuildAIFocusFromAIFirst(output.Focus, focusValid, items)
	notes := []string(nil)
	if failureSubreason != "" {
		notes = []string{failureSubreason}
	}

	totalIn := composeRaw.InputTokens + detailInTokens
	totalOut := composeRaw.OutputTokens + detailOutTokens

	return s.finishAIFirstRun(ctx, aiFirstRunState{
		RunID: runID, SellerID: sellerAccountID, MetricDate: metricDate, MetricDateStr: metricDateStr,
		Input: input, Items: items, TopKeys: firstTopActionKeys(items, 3), AIFocus: aiFocus,
		FinalStatus: finalStatus, DroppedCount: totalDropped,
		ReturnedCount: len(composeOutput.Actions), ValidBeforeLimitCount: pilot.ValidBeforeLimit,
		TruncatedCount: pilot.TruncatedCount, ComposeCalls: 1, ObjectDetailCalls: detailCalls,
		ValidatorMode: PilotValidatorMode, FailureSubreason: failureSubreason,
		DropIssueCounts: pipelineDiag.DropIssueCounts, WarningIssueCounts: pipelineDiag.WarningIssueCounts,
		PipelineDiagnostics: &pipelineDiag, RawResponse: composeRaw,
		ComposeInputTokens: composeRaw.InputTokens, ComposeOutputTokens: composeRaw.OutputTokens,
		ObjectDetailInputTokens: detailInTokens, ObjectDetailOutputTokens: detailOutTokens,
		InputTokens: totalIn, OutputTokens: totalOut,
		NormalizationEvents: normEvents, DetailMerges: detailMerges,
		ComposeOutput: composeOutput, BaseActions: baseActions, ComposeWireInput: wireInput,
		Validation: &validation, FinalValidation: &validation, Notes: notes,
		LifecycleSummary: lifecycleSummary, ActionLifecycles: actionLifecycles,
		Opts: opts, DryRun: opts.DryRun,
	})
}

func composeActionsToRawDrafts(compose *ActionsAIFirstComposeOutput) []AIFirstDraftAction {
	if compose == nil {
		return nil
	}
	out := make([]AIFirstDraftAction, 0, len(compose.Actions))
	for _, a := range compose.Actions {
		out = append(out, AIFirstDraftAction{SourceCandidateIDs: append([]string(nil), a.SourceCandidateIDs...)})
	}
	return out
}

func finalizeTwoPhaseDrafts(input ActionsAIFirstInput, output ActionsAIFirstOutput) (ActionsAIFirstOutput, AIFirstNormalizationResult) {
	result := AIFirstNormalizationResult{Actions: make([]AIFirstDraftAction, 0, len(output.Actions))}
	normalized := make([]AIFirstDraftAction, 0, len(output.Actions))
	for i, action := range output.Actions {
		result.CompletionConditionsReturnedCount += len(action.CompletionConditions)
		action.ObjectActions = hydrateObjectActionMetrics(input, action.ObjectActions, i, &result.Issues)
		result.CompletionConditionsValidCount += len(action.CompletionConditions)
		normalized = append(normalized, action)
	}
	output.Actions = normalized
	result.Actions = normalized
	result.ReturnedActionCount = len(normalized)
	return output, result
}

func enrichTwoPhaseDiagnostics(
	diag *AIFirstPipelineDiagnostics,
	composeRaw *AIRawResponse,
	composeMaxTokens int,
	detailCalls, detailIn, detailOut int,
	merges []ObjectDetailMergeResult,
) {
	if diag == nil {
		return
	}
	diag.ActionsComposeCalls = 1
	diag.ActionsObjectDetailCalls = detailCalls
	diag.ActionsTotalAICalls = 1 + detailCalls
	diag.ActionsComposeRequestedMaxOutputTokens = composeMaxTokens
	diag.ActionsComposeEffectiveMaxOutputTokens = composeMaxTokens
	diag.ActionsComposeOutputLimitWiring = ActionsComposeOutputLimitWiring()
	diag.ActionsComposeInputTokens = composeRaw.InputTokens
	diag.ActionsComposeOutputTokens = composeRaw.OutputTokens
	diag.ActionsObjectDetailInputTokens = detailIn
	diag.ActionsObjectDetailOutputTokens = detailOut
	if composeRaw != nil {
		diag.ActionsComposeProviderOutputTokens = composeRaw.OutputTokens
		diag.ActionsComposeOutputTruncated = IsResponseOutputTruncated(composeRaw, composeMaxTokens)
	}
	diag.ActionsObjectDetailRequestedMaxOutputTokens = ActionsObjectDetailRequestedMaxOutputTokens()

	var batchesTotal, completed, partial, failed, truncated int
	var entitiesTotal, entitiesCompleted, entitiesMissing int
	for _, m := range merges {
		if len(m.BatchResults) == 0 {
			continue
		}
		batchesTotal += len(m.BatchResults)
		entitiesTotal += m.AuthoritativeTotal
		entitiesCompleted += m.DisplayedCount
		entitiesMissing += m.AuthoritativeTotal - m.DisplayedCount
		for _, br := range m.BatchResults {
			switch br.Status {
			case "completed":
				completed++
			case "partial":
				partial++
			case "truncated":
				truncated++
				failed++
			case "failed":
				failed++
			}
		}
	}
	diag.ActionsObjectDetailBatchesTotal = batchesTotal
	diag.ActionsObjectDetailBatchesCompleted = completed
	diag.ActionsObjectDetailBatchesPartial = partial
	diag.ActionsObjectDetailBatchesFailed = failed
	diag.ActionsObjectDetailBatchesTruncated = truncated
	diag.ActionsObjectDetailsEntitiesTotal = entitiesTotal
	diag.ActionsObjectDetailsEntitiesCompleted = entitiesCompleted
	diag.ActionsObjectDetailsEntitiesMissing = entitiesMissing
	if entitiesTotal > 0 {
		diag.ActionsObjectDetailsCoveragePct = float64(entitiesCompleted) / float64(entitiesTotal) * 100
	}
}

const ActionsErrorComposeOutputTruncated = "actions_compose_output_truncated"
