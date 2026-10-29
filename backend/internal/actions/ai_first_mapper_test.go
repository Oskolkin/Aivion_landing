package actions

import "testing"

func TestMapAIFirstActionToItem_PreservesAIFirstSemantics(t *testing.T) {
	input := testAIFirstInput()
	action := validAIFirstAction()
	action.UrgencyMarker = AIFirstUrgencyUrgent
	action.VisibleLimitations = []string{"Лимитация теста"}

	item := MapAIFirstActionToItem("2026-06-24", input, action)
	if item.Status != ActionStatusOpen {
		t.Fatalf("status=%q want open", item.Status)
	}
	if len(item.SourceCandidateIDs) != 0 {
		t.Fatalf("ai-first mapped item must not require source candidates, got %v", item.SourceCandidateIDs)
	}
	if item.CompletionCriteria == "" {
		t.Fatal("completion_criteria should be mapped")
	}
	if len(item.VisibleLimitations) == 0 {
		t.Fatal("visible_limitations should be mapped")
	}
	if len(item.CTAs) != 3 {
		t.Fatalf("ai-first deterministic ctas=%d want 3", len(item.CTAs))
	}
	for _, cta := range item.CTAs {
		if cta.Type == "export" || cta.Type == "open_sku_list" || cta.Type == "open_campaign_list" {
			t.Fatalf("disallowed CTA on AI-first item: %q", cta.Type)
		}
	}
}
