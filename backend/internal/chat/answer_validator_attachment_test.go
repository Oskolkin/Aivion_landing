package chat

import "testing"

func TestFactContextHasFactsWithAttachmentOther(t *testing.T) {
	ctx := &FactContext{
		Facts: FactContextFacts{
			Other: []map[string]any{{"source": "attachment", "file_name": "a.txt", "extracted_text": "TXT_CONTROL_16735"}},
		},
	}
	if !factContextHasFacts(ctx) {
		t.Fatal("attachment facts in Other must count as available facts")
	}
	answer := &ChatAnswer{
		Answer:          "Контрольный код TXT_CONTROL_16735",
		Summary:         "Код из файла",
		Intent:          ChatIntentGeneralOverview,
		ConfidenceLevel: ConfidenceLevelHigh,
		SupportingFacts: []SupportingFact{{Source: "attachment", Fact: "TXT_CONTROL_16735 from a.txt"}},
	}
	res, err := NewAnswerValidator().Validate(answer, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Fatalf("expected valid attachment answer, errors=%v", res.Errors)
	}
}
