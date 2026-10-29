package actions

import "testing"

func TestDeduplicateBaseActions_SelfComparisonEvent(t *testing.T) {
	actions := []AIFirstBaseAction{
		{
			AIFirstComposeAction: AIFirstComposeAction{
				ActionKey: "a", OperationalGoal: "prevent_stockout", ScopeType: AIFirstActionScopeEntity,
			},
			AIIndex:               0,
			AuthoritativeEntities: []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku:1"}},
		},
		{
			AIFirstComposeAction: AIFirstComposeAction{
				ActionKey: "b", OperationalGoal: "prevent_stockout", ScopeType: AIFirstActionScopeEntity,
			},
			AIIndex:               1,
			AuthoritativeEntities: []AIFirstAffectedEntityRef{{EntityType: "sku", EntityID: "sku:1"}},
		},
	}
	out, _, events := deduplicateBaseActionsByOperationalGoal(actions)
	if !out[1].Dropped {
		t.Fatal("expected second action dropped by dedup")
	}
	foundComparison := false
	foundSelf := false
	for _, ev := range events {
		if ev.EventCode == "cross_action_scope_dedup_comparison" {
			foundComparison = true
		}
		if ev.EventCode == "dedup_self_comparison_detected" {
			foundSelf = true
		}
	}
	if !foundComparison {
		t.Fatalf("events=%+v", events)
	}
	if foundSelf {
		t.Fatal("self-comparison should not trigger when comparing different action indexes")
	}
}
