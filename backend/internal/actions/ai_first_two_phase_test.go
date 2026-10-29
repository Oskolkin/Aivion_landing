package actions

import (
	"context"
	"os"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestBuildObjectDetailBatches_Partition27(t *testing.T) {
	entities := make([]AIFirstAffectedEntityRef, 27)
	for i := range entities {
		entities[i] = AIFirstAffectedEntityRef{EntityType: "sku", EntityID: candidateIDForIndex(i)}
	}
	batches := BuildObjectDetailBatches(entities, 10)
	if len(batches) != 3 {
		t.Fatalf("batch count=%d want 3", len(batches))
	}
	if len(batches[0]) != 10 || len(batches[1]) != 10 || len(batches[2]) != 7 {
		t.Fatalf("sizes=%d,%d,%d want 10,10,7", len(batches[0]), len(batches[1]), len(batches[2]))
	}
}

func TestNormalizeComposeActions_FullScope27(t *testing.T) {
	input := ActionsAIFirstInput{
		SchemaVersion: ActionsAIFirstSchemaVersionV2,
		FactCatalog: map[string]aidatapack.CompactFact{
			"fact_1": {FactID: "fact_1"},
		},
	}
	for i := 0; i < 3; i++ {
		ents := make([]AIFirstAffectedEntityRef, 0)
		for j := 0; j < 10; j++ {
			if i == 2 && j >= 7 {
				break
			}
			idx := i*10 + j
			ents = append(ents, AIFirstAffectedEntityRef{EntityType: "sku", EntityID: candidateIDForIndex(idx)})
		}
		input.ActionCandidates = append(input.ActionCandidates, AIFirstActionCandidate{
			CandidateID: candidateIDForIndex(i), ChunkID: "stock_positions_001",
			Type:             ActionTypeStockReplenishment,
			AffectedEntities: ents,
			EvidenceFactIDs:  []string{"fact_1"},
		})
	}
	compose := ActionsAIFirstComposeOutput{
		Actions: []AIFirstComposeAction{{
			ActionKey: "replenish", Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
			Priority: ActionPriorityCritical, ScopeType: AIFirstActionScopeEntity,
			OperationalGoal: "replenish_stock",
			SourceCandidateIDs: []string{
				candidateIDForIndex(0), candidateIDForIndex(1), candidateIDForIndex(2),
			},
			TitleTemplate: "Пополнить {object_count} SKU", Reason: fakeLongReason([]string{"test"}, nil),
			CompletionCriteria: "done",
		}},
	}
	base, _, _ := NormalizeComposeActions(input, compose)
	if len(base) != 1 {
		t.Fatalf("base actions=%d", len(base))
	}
	if len(base[0].AuthoritativeEntities) != 27 {
		t.Fatalf("scope=%d want 27", len(base[0].AuthoritativeEntities))
	}
}

func TestRunObjectDetailBatches_AccountActionZeroCalls(t *testing.T) {
	input := ActionsAIFirstInput{SchemaVersion: ActionsAIFirstSchemaVersionV2}
	base := []AIFirstBaseAction{{
		AIFirstComposeAction: AIFirstComposeAction{
			ActionKey: "account_review", ScopeType: AIFirstActionScopeAccount,
		},
	}}
	client := NewDeterministicAIFirstFakeClient()
	merges, calls, _, _, _ := RunObjectDetailBatches(context.Background(), client, input, base)
	if calls != 0 {
		t.Fatalf("detail calls=%d want 0", calls)
	}
	if len(merges) != 1 || merges[0].AuthoritativeTotal != 0 {
		t.Fatalf("merges=%+v", merges)
	}
}

func TestDeduplicateBaseActions_SingleActionNotSelfCompared(t *testing.T) {
	actions := []AIFirstBaseAction{{
		AIFirstComposeAction:  AIFirstComposeAction{OperationalGoal: "g", ScopeType: AIFirstActionScopeEntity},
		AuthoritativeEntities: []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku:1"}},
	}}
	out, issues, _ := deduplicateBaseActionsByOperationalGoal(actions)
	if out[0].Dropped {
		t.Fatal("single action should not drop itself")
	}
	if len(issues) != 0 {
		t.Fatalf("issues=%v", issues)
	}
}

func TestBuildActionsCandidateCoverageDiagnostics_Run72Fixture(t *testing.T) {
	input := ActionsAIFirstInput{
		SchemaVersion:    ActionsAIFirstSchemaVersionV2,
		ActionCandidates: make([]AIFirstActionCandidate, 35),
	}
	for i := range input.ActionCandidates {
		input.ActionCandidates[i].CandidateID = candidateIDForIndex(i)
	}
	returned := []AIFirstDraftAction{{
		SourceCandidateIDs: func() []string {
			out := make([]string, 8)
			for i := 0; i < 8; i++ {
				out[i] = candidateIDForIndex(i)
			}
			return out
		}(),
	}}
	coverage := BuildActionsCandidateCoverageDiagnostics(input, returned, nil, nil, map[string]bool{})
	assertCoverageInt(t, coverage, "actions_candidate_referenced_persisted_count", 0)
	assertCoverageInt(t, coverage, "actions_candidate_referenced_dropped_only_count", 8)
	assertCoverageInt(t, coverage, "actions_candidate_not_referenced_by_ai_count", 27)
}

func TestActionsComposeOutputLimitWiring_PrefersOpenAIActionsEnv(t *testing.T) {
	t.Setenv("OPENAI_ACTIONS_MAX_OUTPUT_TOKENS", "16384")
	t.Setenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST", "8192")
	if got := ActionsComposeRequestedMaxOutputTokens(); got != 16384 {
		t.Fatalf("compose requested=%d want 16384", got)
	}
	if got := ActionsComposeOutputLimitWiring(); got != "OPENAI_ACTIONS_MAX_OUTPUT_TOKENS" {
		t.Fatalf("wiring=%q", got)
	}
}

func TestActionsComposeRequestedMaxOutputTokens_Default16384(t *testing.T) {
	os.Unsetenv("OPENAI_ACTIONS_MAX_OUTPUT_TOKENS")
	os.Unsetenv("AI_MAX_OUTPUT_TOKENS_PER_REQUEST")
	os.Unsetenv("AI_MAX_OUTPUT_TOKENS")
	if got := ActionsComposeRequestedMaxOutputTokens(); got != 16384 {
		t.Fatalf("compose tokens=%d want 16384", got)
	}
}

func TestMergeObjectDetailResults_DeterministicOrder(t *testing.T) {
	base := AIFirstBaseAction{
		AIFirstComposeAction: AIFirstComposeAction{ActionKey: "a"},
		AuthoritativeEntities: []AIFirstAffectedEntityRef{
			{EntityType: "sku", EntityID: "e1"}, {EntityType: "sku", EntityID: "e2"},
		},
	}
	batchResults := []ObjectDetailBatchResult{
		{ValidObjectActions: []AIFirstObjectAction{{EntityType: "sku", EntityID: "e2", RecommendedAction: "b"}}},
		{ValidObjectActions: []AIFirstObjectAction{{EntityType: "sku", EntityID: "e1", RecommendedAction: "a"}}},
	}
	merge := MergeObjectDetailResults(base, batchResults)
	if len(merge.ObjectActions) != 2 {
		t.Fatalf("count=%d", len(merge.ObjectActions))
	}
	if merge.ObjectActions[0].EntityID != "e1" || merge.ObjectActions[1].EntityID != "e2" {
		t.Fatalf("order=%v,%v", merge.ObjectActions[0].EntityID, merge.ObjectActions[1].EntityID)
	}
}
