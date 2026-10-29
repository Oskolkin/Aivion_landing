package aisummaries

import "testing"

func TestReacceptSummaryPayload_oneBadCategoryDoesNotAffectOtherPayloads(t *testing.T) {
	good := &SummaryPayload{
		Headline:          "Cat A",
		ExecutiveSummary:  "Summary A",
		HealthScore:       50,
		Severity:          "low",
		DataRichness:      "medium",
		SummaryDepth:      "normal",
		KeyRisk:           "risk",
		RecommendedAction: "act",
	}
	catID := int64(12)
	good.CategoryID = &catID
	name := "Books"
	good.CategoryName = &name

	goodOutcome := ReacceptSummaryPayload(good, SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "12",
		CategoryName: "Books",
	})
	if !goodOutcome.Accepted {
		t.Fatalf("good category should accept: %+v", goodOutcome.Repair)
	}

	missingHeadline := &SummaryPayload{
		ExecutiveSummary: "Summary without headline is repairable.",
		HealthScore:      50,
		Severity:         "low",
	}
	missingOutcome := ReacceptSummaryPayload(missingHeadline, SummaryRepairContext{
		SummaryType:  "category_daily_summary",
		ScopeID:      "12",
		CategoryName: "Books",
	})
	if !missingOutcome.Accepted {
		t.Fatalf("missing headline category should repair-accept: %+v", missingOutcome.Repair)
	}
}
