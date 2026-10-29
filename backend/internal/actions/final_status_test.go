package actions

import "testing"

func TestComputeFinalStatus_CompletedRepaired(t *testing.T) {
	results := []DomainPipelineResult{
		{Status: CategoryRunStatusRepair, Repaired: true},
		{Status: CategoryRunStatusCompleted},
	}
	got := computeFinalStatus(results, 3, false, 1, 0)
	if got != GenerationFinalStatusCompletedRepaired {
		t.Fatalf("final_status=%q want completed_repaired", got)
	}
}

func TestComputeFinalStatus_CompletedPartialOnDropped(t *testing.T) {
	results := []DomainPipelineResult{
		{Status: CategoryRunStatusCompleted},
	}
	got := computeFinalStatus(results, 2, false, 0, 1)
	if got != GenerationFinalStatusCompletedPartial {
		t.Fatalf("final_status=%q want completed_partial", got)
	}
}

func TestComputeFinalStatus_FailedWhenNoActions(t *testing.T) {
	results := []DomainPipelineResult{
		{Status: CategoryRunStatusCompleted},
		{Status: CategoryRunStatusFailed},
	}
	got := computeFinalStatus(results, 0, false, 0, 0)
	if got != GenerationFinalStatusFailed {
		t.Fatalf("final_status=%q want failed", got)
	}
}

func TestComputeFinalStatus_EmptyWhenAllDomainsFailed(t *testing.T) {
	results := []DomainPipelineResult{{Status: CategoryRunStatusFailed}}
	got := computeFinalStatus(results, 0, false, 0, 0)
	if got != GenerationFinalStatusEmpty {
		t.Fatalf("final_status=%q want empty", got)
	}
}
