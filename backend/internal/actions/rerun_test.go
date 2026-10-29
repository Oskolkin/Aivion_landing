package actions

import (
	"context"
	"testing"
)

func TestRerunService_AIDisabledUsesFallback(t *testing.T) {
	t.Setenv("ACTIONS_AI_FIRST", "false")
	builder := buildTestCandidateBuilder(t)
	repo := newFakeGenerationRepo()
	fakeAI := &FakeActionAIClient{}
	svc := NewRerunService(NewGenerationService(builder, fakeAI, repo))
	aiDisabled := false
	result, err := svc.RerunActions(context.Background(), RerunActionsRequest{
		SellerAccountID: 1,
		AIEnabled:       &aiDisabled,
	})
	if err != nil {
		t.Fatalf("RerunActions: %v", err)
	}
	if result.RunID <= 0 {
		t.Fatalf("expected run_id")
	}
	if len(result.ActionKeys) == 0 {
		t.Fatalf("expected generated action keys")
	}
	if len(fakeAI.ComposeCalls) != 0 {
		t.Fatal("AI should not be called when disabled")
	}
}
