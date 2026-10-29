package actions

// TwoPhaseActionsReplayResult is the offline replay outcome for compose + normalization + validation + persist guard.
type TwoPhaseActionsReplayResult struct {
	ComposeReturnedCount int
	AfterNormalization   int
	AcceptedCount        int
	PersistableCount     int
	DropIssueCounts      map[string]int
	Pilot                PilotActionsValidationResult
	Persist              aiFirstPersistGuardResult
	ComposeNorm          AIFirstNormalizationResult
	BaseActions          []AIFirstBaseAction
	Output               ActionsAIFirstOutput
}

// ReplayTwoPhaseActionsFromCompose replays Actions normalization/validation without AI calls.
func ReplayTwoPhaseActionsFromCompose(
	input ActionsAIFirstInput,
	compose ActionsAIFirstComposeOutput,
	detailMerges []ObjectDetailMergeResult,
) TwoPhaseActionsReplayResult {
	baseActions, composeNorm, _ := NormalizeComposeActions(input, compose)

	drafts := make([]AIFirstDraftAction, 0, len(baseActions))
	for i, base := range baseActions {
		draft := baseActionToDraft(base)
		if i < len(detailMerges) {
			draft = finalizeDraftAfterObjectDetail(draft, detailMerges[i])
		}
		objectCount := len(base.AuthoritativeEntities)
		computedMap, computeIssues := executeComputedValues(input, draft.ComputedValues, base.AIIndex)
		composeNorm.Issues = append(composeNorm.Issues, computeIssues...)
		draft = renderActionTemplates(draft, objectCount, computedMap)
		drafts = append(drafts, draft)
	}

	legacyOutput := ActionsAIFirstOutput{
		SchemaVersion: compose.SchemaVersion,
		Focus:         compose.Focus,
		Actions:       drafts,
		Notes:         compose.Notes,
	}
	normOutput, postNorm := finalizeTwoPhaseDrafts(input, legacyOutput)
	allNormIssues := append(composeNorm.Issues, postNorm.Issues...)
	output := normOutput

	pilot := ValidateActionsAIFirstOutputPilot(input, output)
	pilot.Issues = append(pilot.Issues, allNormIssues...)
	persist := mapAIFirstDraftsWithDiagnostics("2026-07-08", input, pilot.Accepted, warningsByActionIndex(pilot.Issues))

	dropCounts := map[string]int{}
	for k, v := range pilot.DropIssueCounts {
		dropCounts[k] = v
	}
	mergeIssueCounts(dropCounts, persist.DropIssueCounts)

	return TwoPhaseActionsReplayResult{
		ComposeReturnedCount: len(compose.Actions),
		AfterNormalization:   len(output.Actions),
		AcceptedCount:        pilot.AcceptedCount,
		PersistableCount:     len(persist.Items),
		DropIssueCounts:      dropCounts,
		Pilot:                pilot,
		Persist:              persist,
		ComposeNorm:          composeNorm,
		BaseActions:          baseActions,
		Output:               output,
	}
}
