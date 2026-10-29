package actions

import "slices"

// publishableFinalStatuses is the single source of truth for seller-facing action runs.
var publishableFinalStatuses = []string{
	GenerationFinalStatusCompleted,
	GenerationFinalStatusCompletedRepaired,
	GenerationFinalStatusCompletedPartial,
	GenerationFinalStatusCompletedAfterRetry,
	GenerationFinalStatusEmpty,
}

// PublishableFinalStatusSQLFragment is embedded in SQL queries; keep in sync via TestPublishableFinalStatusContract.
const PublishableFinalStatusSQLFragment = `'completed', 'completed_repaired', 'completed_partial', 'completed_after_retry', 'empty'`

func IsPublishableActionFinalStatus(status string) bool {
	return slices.Contains(publishableFinalStatuses, status)
}

func IsSuccessfulActionFinalStatus(status string) bool {
	return IsPublishableActionFinalStatus(status) && status != GenerationFinalStatusEmpty
}

func IsEmptyActionFinalStatus(status string) bool {
	return status == GenerationFinalStatusEmpty
}
