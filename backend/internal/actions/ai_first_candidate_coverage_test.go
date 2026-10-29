package actions

import (
	"fmt"
	"math"
	"testing"
)

func TestBuildActionsCandidateCoverageDiagnostics_Run71Fixture(t *testing.T) {
	input := ActionsAIFirstInput{
		SchemaVersion:    ActionsAIFirstSchemaVersionV2,
		ActionCandidates: make([]AIFirstActionCandidate, 35),
	}
	for i := range input.ActionCandidates {
		input.ActionCandidates[i].CandidateID = candidateIDForIndex(i)
	}

	returned := make([]AIFirstDraftAction, 6)
	persistedKeys := map[string]bool{}
	for i := 0; i < 18; i++ {
		id := candidateIDForIndex(i)
		persistedKeys[id] = true
		returned[0].SourceCandidateIDs = append(returned[0].SourceCandidateIDs, id)
	}
	// Two candidates only in dropped (persist-guard) action.
	for i := 18; i < 20; i++ {
		returned[5].SourceCandidateIDs = append(returned[5].SourceCandidateIDs, candidateIDForIndex(i))
	}

	accepted := []AIFirstDraftAction{{SourceCandidateIDs: returned[0].SourceCandidateIDs}}
	coverage := BuildActionsCandidateCoverageDiagnostics(input, returned, accepted, nil, persistedKeys)

	assertCoverageInt(t, coverage, "actions_candidate_total_count", 35)
	assertCoverageInt(t, coverage, "actions_candidate_referenced_persisted_count", 18)
	assertCoverageInt(t, coverage, "actions_candidate_referenced_dropped_only_count", 2)
	assertCoverageInt(t, coverage, "actions_candidate_not_referenced_by_ai_count", 15)
	assertCoverageInt(t, coverage, "actions_candidate_referenced_returned_count", 20)

	persistedPct, _ := coverage["actions_candidate_persisted_coverage_pct"].(float64)
	if math.Abs(persistedPct-51.428571) > 0.001 {
		t.Fatalf("persisted coverage=%v want ~51.428571", persistedPct)
	}
	returnedPct, _ := coverage["actions_candidate_returned_coverage_pct"].(float64)
	if math.Abs(returnedPct-57.142857) > 0.001 {
		t.Fatalf("returned coverage=%v want ~57.142857", returnedPct)
	}
	if coverage["candidate_coverage_invariant_mismatch"] == true {
		t.Fatal("invariant should hold")
	}
}

func TestBuildActionsCandidateCoverageDiagnostics_PersistedWinsOverDropped(t *testing.T) {
	input := ActionsAIFirstInput{
		SchemaVersion: ActionsAIFirstSchemaVersionV2,
		ActionCandidates: []AIFirstActionCandidate{
			{CandidateID: "candidate:A"},
		},
	}
	returned := []AIFirstDraftAction{
		{SourceCandidateIDs: []string{"candidate:A"}},
		{SourceCandidateIDs: []string{"candidate:A"}},
	}
	accepted := []AIFirstDraftAction{returned[0]}
	persistedKeys := map[string]bool{"candidate:A": true}

	coverage := BuildActionsCandidateCoverageDiagnostics(input, returned, accepted, nil, persistedKeys)
	assertCoverageInt(t, coverage, "actions_candidate_referenced_persisted_count", 1)
	assertCoverageInt(t, coverage, "actions_candidate_referenced_dropped_only_count", 0)

	candidates, _ := coverage["candidates"].([]map[string]any)
	if len(candidates) != 1 || candidates[0]["disposition"] != "referenced_in_persisted_action" {
		t.Fatalf("disposition=%v", candidates)
	}
}

func candidateIDForIndex(i int) string {
	return fmt.Sprintf("candidate:%02d", i+1)
}

func assertCoverageInt(t *testing.T, coverage map[string]any, key string, want int) {
	t.Helper()
	got, _ := coverage[key].(int)
	if got != want {
		t.Fatalf("%s=%d want %d", key, got, want)
	}
}
