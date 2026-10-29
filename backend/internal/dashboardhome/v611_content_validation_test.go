package dashboardhome

import "testing"

func TestReplaceInternalMetricNamesInText(t *testing.T) {
	got := replaceInternalMetricNamesInText("ad_efficiency_drr вырос у кампании, daily_account_orders_count стабилен.")
	if containsInternalMetricName(got) {
		t.Fatalf("expected sanitized text, got %q", got)
	}
	if got != "ДРР кампании вырос у кампании, количество заказов стабилен." {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestInferSellerPulseQuickActionKindSlowTurnover(t *testing.T) {
	kind := inferSellerPulseQuickActionKind(
		"Разобрать SKU без заказов с крупным остатком",
		"monitoring",
		nil,
	)
	if kind != "slow_turnover" {
		t.Fatalf("kind=%q", kind)
	}
}

func TestImprovedGroupStrengthsLabel(t *testing.T) {
	group := dashboardNarrativeWhatChangedGroup{
		Label: "Улучшилось",
		Items: []dashboardNarrativeWhatChangedItem{{
			Label: "Эффективные кампании",
			Text:  "Часть кампаний удерживает низкий ДРР и может стать базой для бюджета.",
		}},
	}
	if !improvedGroupIsStrengthsNotProvenImprovement(group) {
		t.Fatal("expected strengths heuristic")
	}
	if got := resolveImprovedGroupLabel(group); got != "Точки опоры" {
		t.Fatalf("label=%q", got)
	}
}

func TestValidateCategoryCatalogCoverage(t *testing.T) {
	payload := minimalAIOnlyPayload()
	input := DashboardSynthesisInput{
		CategorySelectionPolicy: CategorySummariesSelectionPolicyAllCatalog,
		CategoryCatalogCount:    2,
		CategoryCatalog: []DashboardCategoryCatalogEntry{
			{CategoryID: "1", CategoryName: "A"},
			{CategoryID: "2", CategoryName: "B"},
		},
	}
	issues := validateAIOnlyCategoryCatalogCoverage(payload, input)
	if len(issues) == 0 {
		t.Fatal("expected catalog mismatch")
	}
}
