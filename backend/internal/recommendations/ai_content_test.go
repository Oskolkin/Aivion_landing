package recommendations

import "testing"

func TestNormalizeAIJSONContent_StripsMarkdownFence(t *testing.T) {
	in := "```json\n{\"recommendations\":[{\"title\":\"x\"}]}\n```"
	got := normalizeAIJSONContent(in)
	if got == "" || got[0] != '{' {
		t.Fatalf("unexpected normalized content: %q", got)
	}
}

func TestCountOpenAlertsInContext_UsesTopOpenWhenOpenTotalZero(t *testing.T) {
	ctx := &AIRecommendationContext{
		Alerts: AlertsContext{
			OpenTotal: 0,
			TopOpen:   []AlertSignal{{ID: 1}, {ID: 2}},
		},
	}
	if countOpenAlertsInContext(ctx) != 2 {
		t.Fatalf("expected 2 open alerts from top_open")
	}
}

func TestEvaluateAfterSave_FailsWhenNothingSaved(t *testing.T) {
	out := evaluateAfterSave(35, 0, &ValidationResult{TotalRecommendations: 0}, runOutcome{})
	if !out.FailRun {
		t.Fatal("expected fail when open alerts but nothing saved")
	}
}
