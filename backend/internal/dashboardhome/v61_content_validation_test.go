package dashboardhome

import "testing"

func TestIsCategoryScoreSeverityConsistent(t *testing.T) {
	cases := []struct {
		score    int32
		severity string
		want     bool
	}{
		{2, "high", true},
		{2, "critical", true},
		{2, "medium", false},
		{5, "medium", true},
		{8, "low", true},
		{8, "medium", false},
		{7, "high", false},
		{9, "stable", true},
	}
	for _, c := range cases {
		if got := isCategoryScoreSeverityConsistent(c.score, c.severity); got != c.want {
			t.Fatalf("score=%d severity=%q want=%v got=%v", c.score, c.severity, c.want, got)
		}
	}
}

func TestRepairAISeesSellerFacingPhrase(t *testing.T) {
	got := repairAISeesSellerFacingPhrase("AI видит сочетание рисков в остатках и рекламе.")
	want := "В кабинете сочетаются рисков в остатках и рекламе."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHasDuplicateWhatChangedLabelPrefix(t *testing.T) {
	if !hasDuplicateWhatChangedLabelPrefix("Выручка", "Выручка: Выручка снизилась на 5%") {
		t.Fatal("expected duplicate prefix")
	}
	if hasDuplicateWhatChangedLabelPrefix("Выручка", "Выручка снизилась на 5%") {
		t.Fatal("expected no duplicate when label only prefixes once")
	}
}

func TestRepairDuplicateWhatChangedLabelPrefix(t *testing.T) {
	got := repairDuplicateWhatChangedLabelPrefix("Выручка", "Выручка: Выручка снизилась на 5%")
	if got == "Выручка: Выручка снизилась на 5%" {
		t.Fatalf("expected repair, got %q", got)
	}
}

func TestAllowedImprovedGroupLabel(t *testing.T) {
	if !allowedImprovedGroupLabel("Точки опоры", false) {
		t.Fatal("expected strengths label when empty")
	}
	if allowedImprovedGroupLabel("Улучшилось", false) {
		t.Fatal("improved label not allowed without items")
	}
}

func TestValidateAIOnlyV61RejectsTitleEqualsTextChecklist(t *testing.T) {
	payload := minimalAIOnlyPayload()
	payload.AdEfficiencyNarrative.Checklist = []AdEfficiencyCheckItem{{
		Title: "Остановить кампанию",
		Text:  "Остановить кампанию",
	}}
	issues := validateAIOnlyV61Content(payload, DashboardSynthesisInput{})
	found := false
	for _, issue := range issues {
		if issue.Code == "ad_efficiency_checklist_title_equals_text" {
			found = true
		}
	}
	if !found {
		t.Fatalf("issues=%+v", issues)
	}
}

func TestFunnelStageDayOverDayFromStages(t *testing.T) {
	v := int64(100)
	p := int64(80)
	chg := -5.0
	stages := []FunnelStage{{
		Key: "impressions", Label: "Показы", Value: &v, PreviousDayValue: &p, ChangeVsPreviousDayPct: &chg,
	}}
	comparisons := buildFunnelStageDayOverDayFromStages(stages)
	if len(comparisons) != 1 || comparisons[0]["stage_key"] != "impressions" {
		t.Fatalf("comparisons=%+v", comparisons)
	}
}
