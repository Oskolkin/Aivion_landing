package chat

import "testing"

func TestSynthesizeStreamAnswerForcesLowConfidenceWithoutFacts(t *testing.T) {
	answer := synthesizeStreamAnswer("Контроль остатков важен для продаж.", &FactContext{
		Intent:      ChatIntentGeneralOverview,
		Limitations: []string{"no tools; answering without cabinet tool reads"},
	})
	if answer.ConfidenceLevel != ConfidenceLevelLow {
		t.Fatalf("expected low confidence, got %s", answer.ConfidenceLevel)
	}
	hasLimitation := false
	for _, sf := range answer.SupportingFacts {
		if sf.Source == "limitation" {
			hasLimitation = true
			break
		}
	}
	if !hasLimitation {
		t.Fatal("expected limitation supporting fact")
	}
	res, err := NewAnswerValidator().Validate(&answer, &FactContext{
		Intent:      ChatIntentGeneralOverview,
		Limitations: []string{"no tools; answering without cabinet tool reads"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Fatalf("expected valid conversational stream answer, errors=%v", res.Errors)
	}
}
