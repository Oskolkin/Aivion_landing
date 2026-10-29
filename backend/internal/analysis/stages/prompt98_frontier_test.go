package stages

import "testing"

func TestPrompt13FrontierStages1to19(t *testing.T) {
	for _, name := range []Name{PlanActionReconciliation, BuildPublication, BuildProjections, ValidatePublication, Publish, PostPublish} {
		d, ok := ProductionRegistry.Lookup(name)
		if !ok || !d.ImplementedInCurrentBuild {
			t.Fatalf("%s must be implemented after Prompt 13", name)
		}
	}
	got := ProductionImplementedStageNames()
	if len(got) != 19 || got[18] != string(PostPublish) {
		t.Fatalf("production frontier drifted: %#v", got)
	}
}
