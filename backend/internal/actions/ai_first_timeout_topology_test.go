package actions_test

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

func TestObjectDetailBatches_StageElapsedExceedsSingleCallTimeoutBudget(t *testing.T) {
	t.Setenv("OPENAI_ACTION_OBJECT_DETAIL_TIMEOUT_SECONDS", "1")
	t.Setenv("ACTIONS_OBJECT_DETAIL_BATCH_SIZE", "1")
	t.Setenv("ACTIONS_OBJECT_DETAIL_MAX_CONCURRENCY", "1")

	batchDelay := 150 * time.Millisecond
	client := &actions.FakeActionAIClient{
		ComposeObjectDetailFn: func(ctx context.Context, batch actions.ObjectDetailBatchInput) (*actions.ObjectDetailBatchOutput, *actions.AIRawResponse, error) {
			time.Sleep(batchDelay)
			out := &actions.ObjectDetailBatchOutput{ActionKey: batch.Action.ActionKey, BatchIndex: batch.Batch.BatchIndex}
			for _, entity := range batch.Entities {
				out.ObjectActions = append(out.ObjectActions, actions.ObjectDetailObjectAction{
					EntityRef: entity.EntityRef, Recommendation: "Проверить остаток", Reason: "Тест",
				})
			}
			return out, &actions.AIRawResponse{Model: "fake", InputTokens: 5, OutputTokens: 3}, nil
		},
	}

	base := []actions.AIFirstBaseAction{{
		AIFirstComposeAction: actions.AIFirstComposeAction{
			ActionKey: "test_action", ScopeType: actions.AIFirstActionScopeEntity,
		},
		AuthoritativeEntities: []actions.AIFirstAffectedEntityRef{
			{EntityID: "sku:1"}, {EntityID: "sku:2"}, {EntityID: "sku:3"},
		},
	}}
	stageStart := time.Now()
	_, calls, _, _, _ := actions.RunObjectDetailBatches(context.Background(), client, actions.ActionsAIFirstInput{}, base)
	elapsed := time.Since(stageStart)
	if calls != 3 {
		t.Fatalf("calls=%d want 3", calls)
	}
	if elapsed <= batchDelay {
		t.Fatalf("elapsed=%v should exceed single-call timeout budget %v", elapsed, batchDelay)
	}
}
