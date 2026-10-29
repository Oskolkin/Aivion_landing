package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

func TestDetectUnresolvedPlaceholders_Run72Story(t *testing.T) {
	text := "За последние 7 дней выручка снизилась на %, а заказы уменьшились на %."
	issues := DetectUnresolvedPlaceholders(text)
	if len(issues) < 2 {
		t.Fatalf("issues=%d want >=2", len(issues))
	}
}

func TestDetectUnresolvedPlaceholders_ValidPercent(t *testing.T) {
	issues := DetectUnresolvedPlaceholders("Выручка снизилась на 7,79%.")
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestGlobalContextPlaceholderDowngradesConfidence(t *testing.T) {
	ctx := &aicontext.GlobalSemanticContext{Confidence: "high", OverallStory: "снизилась на %"}
	result := GlobalContextValidationResult{
		Issues: []GlobalContextValidationIssue{{
			Code: GlobalContextUnresolvedNumericPlaceholderCode, Severity: aicontext.IssueSeverityWarning,
		}},
	}
	NormalizeGlobalContextConfidenceAfterValidation(ctx, result)
	if ctx.Confidence == "high" {
		t.Fatal("confidence should not remain high")
	}
}

func TestLegacyActionsAdapter_Available(t *testing.T) {
	adapter := LegacyActionsInspectorAdapter{
		ActionsOutput:     []byte(`{"accepted_count":0,"dropped_count":1}`),
		ActionsValidation: []byte(`{"issues":[{"code":"entity_outside_candidate_scope_removed","severity":"warning"}]}`),
	}
	if !adapter.Available() {
		t.Fatal("expected available")
	}
	view := adapter.BuildView()
	if view == nil {
		t.Fatal("expected view")
	}
}
