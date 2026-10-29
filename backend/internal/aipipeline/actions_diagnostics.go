package aipipeline

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"

func populateActionsDiagnosticsFromInput(diag *ActionsDiagnostics, input actions.ActionsAIFirstInput, source string) {
	if diag == nil {
		return
	}
	diag.InputSchema = input.SchemaVersion
	diag.InputSource = source
	diag.CandidateCount = len(input.ActionCandidates)
	diag.FactCatalogCount = len(input.FactCatalog)
	diag.EntityCatalogCount = len(input.EntityCatalog)
	diag.FailedChunkCount = len(input.FailedChunkIDs)
	diag.LimitationsCount = len(input.DataLimitations)
	diag.InputDataPackSerialized = input.SchemaVersion == actions.ActionsAIFirstSchemaVersionV1
}

func populateActionsDiagnosticsFromResult(diag *ActionsDiagnostics, actRes *actions.GenerateActionsResult) {
	if diag == nil || actRes == nil {
		return
	}
	diag.ValidatorMode = actRes.ValidatorMode
	diag.ComposeCalls = actRes.ComposeCalls
	diag.ObjectDetailCalls = actRes.ObjectDetailCalls
	diag.TotalAICalls = actRes.ComposeCalls + actRes.ObjectDetailCalls
	diag.ReturnedCount = actRes.ReturnedCount
	diag.ValidBeforeLimitCount = actRes.ValidBeforeLimitCount
	diag.TruncatedCount = actRes.TruncatedCount
	diag.FinalStatus = actRes.FinalStatus
	diag.FinalCount = actRes.AcceptedCount
	diag.AcceptedCount = actRes.AcceptedCount
	diag.DroppedCount = actRes.DroppedCount
	diag.RetryCalls = 0
	diag.RetryCount = 0
	diag.AIOrderPreserved = true
	diag.TopActionKeysCount = len(actRes.TopActionKeys)
	diag.FailureSubreason = actRes.FailureSubreason
	diag.InputTokens = actRes.InputTokens
	diag.OutputTokens = actRes.OutputTokens
	diag.LatencyMs = actRes.LatencyMs
	diag.RequestID = actRes.RequestID
	if diag.ComposeCalls > 0 && (actRes.InputTokens > 0 || actRes.OutputTokens > 0) {
		diag.UsageSource = UsageSourceProvider
	}
	if len(actRes.DropIssueCounts) > 0 {
		diag.DropIssueCounts = make(map[string]int, len(actRes.DropIssueCounts))
		for k, v := range actRes.DropIssueCounts {
			diag.DropIssueCounts[k] = v
		}
	}
	if actRes.PipelineDiagnostics != nil {
		pd := actRes.PipelineDiagnostics
		diag.NormalizationActionDroppedCount = pd.NormalizationActionDroppedCount
		diag.AfterNormalizationCount = pd.AfterNormalizationCount
		diag.ValidationActionDroppedCount = pd.ValidationActionDroppedCount
		diag.AfterValidationCount = pd.AfterValidationCount
		diag.LimitDroppedCount = pd.LimitDroppedCount
		diag.AfterLimitCount = pd.AfterLimitCount
		diag.PersistGuardActionDroppedCount = pd.PersistGuardActionDroppedCount
		diag.AfterPersistGuardCount = pd.AfterPersistGuardCount
		diag.CompletionConditionsReturnedCount = pd.CompletionConditionsReturnedCount
		diag.CompletionConditionsValidCount = pd.CompletionConditionsValidCount
		diag.CompletionConditionsDroppedCount = pd.CompletionConditionsDroppedCount
		diag.CompletionConditionsFactIDRemovedCount = pd.CompletionConditionsFactIDRemovedCount
		diag.CompletionConditionsEmptyActionsCount = pd.CompletionConditionsEmptyActionsCount
		diag.CandidateCoverage = pd.CandidateCoverage
		if len(pd.WarningIssueCounts) > 0 {
			diag.WarningIssueCounts = make(map[string]int, len(pd.WarningIssueCounts))
			for k, v := range pd.WarningIssueCounts {
				diag.WarningIssueCounts[k] = v
			}
		}
		if pd.InputEstimatedTokens > 0 {
			diag.InputEstimatedTokens = pd.InputEstimatedTokens
		}
		if pd.InputActualTokens > 0 {
			diag.InputActualTokens = pd.InputActualTokens
		}
		if pd.InputHeadroomEstimatedTokens > 0 {
			diag.InputHeadroomTokens = pd.InputHeadroomEstimatedTokens
			diag.InputHeadroomEstimatedTokens = pd.InputHeadroomEstimatedTokens
		}
		if pd.InputHeadroomActualTokens > 0 {
			diag.InputHeadroomActualTokens = pd.InputHeadroomActualTokens
		}
		diag.InputEstimationErrorTokens = pd.InputEstimationErrorTokens
		diag.InputEstimationErrorPct = pd.InputEstimationErrorPct
		diag.ActionsWithCompletionConditionsCount = pd.ActionsWithCompletionConditionsCount
		diag.ActionsWithoutCompletionConditionsCount = pd.ActionsWithoutCompletionConditionsCount
		diag.ActionsCompletionConditionsCoveragePct = pd.ActionsCompletionConditionsCoveragePct
		diag.ActionsCompletionDisplayMismatchCount = pd.ActionsCompletionDisplayMismatchCount
		diag.ActionsVagueCompletionCriteriaCount = pd.ActionsVagueCompletionCriteriaCount
		diag.ActionsEntityScopedCount = pd.ActionsEntityScopedCount
		diag.ActionsAccountScopedCount = pd.ActionsAccountScopedCount
		diag.ActionsMixedScopedCount = pd.ActionsMixedScopedCount
		diag.ActionsObjectActionsFullCount = pd.ActionsObjectActionsFullCount
		diag.ActionsObjectActionsPartialCount = pd.ActionsObjectActionsPartialCount
		diag.ActionsObjectActionsMissingCount = pd.ActionsObjectActionsMissingCount
		diag.ActionsAffectedEntitiesTotalCount = pd.ActionsAffectedEntitiesTotalCount
		diag.ActionsObjectActionsReturnedTotalCount = pd.ActionsObjectActionsReturnedTotalCount
		diag.ActionsObjectActionsCoveragePct = pd.ActionsObjectActionsCoveragePct
		if pd.ActionsRequestedMaxOutputTokens > 0 {
			diag.RequestedMaxOutputTokens = pd.ActionsRequestedMaxOutputTokens
			diag.MaxOutputTokens = pd.ActionsRequestedMaxOutputTokens
		}
		if pd.ActionsProviderOutputTokens > 0 {
			diag.ProviderOutputTokens = pd.ActionsProviderOutputTokens
		}
		diag.OutputTokensExceededRequestedLimit = pd.ActionsOutputTokensExceededRequestedLimit
	}
}
