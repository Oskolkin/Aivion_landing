package actions

import "testing"

func TestPublishableFinalStatusContract(t *testing.T) {
	for _, status := range []string{
		GenerationFinalStatusCompleted,
		GenerationFinalStatusCompletedRepaired,
		GenerationFinalStatusCompletedPartial,
		GenerationFinalStatusCompletedAfterRetry,
		GenerationFinalStatusEmpty,
	} {
		if !IsPublishableActionFinalStatus(status) {
			t.Fatalf("expected publishable: %s", status)
		}
	}
	if IsPublishableActionFinalStatus(GenerationFinalStatusFailed) {
		t.Fatal("failed should not be publishable")
	}
}
