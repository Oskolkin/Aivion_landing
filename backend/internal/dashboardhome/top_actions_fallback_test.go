package dashboardhome

import (
	"strings"
	"testing"
)

func TestFallbackExpectedEffect_CoversKnownCategories(t *testing.T) {
	for _, category := range []string{"advertising", "stock", "funnel", "sales", "cancellations", "surplus", "products"} {
		if strings.TrimSpace(fallbackExpectedEffect(category)) == "" {
			t.Fatalf("expected non-empty effect for %q", category)
		}
	}
	if fallbackExpectedEffect("unknown-category") != "Сфокусироваться на действии с наибольшим влиянием." {
		t.Fatalf("default effect=%q", fallbackExpectedEffect("unknown-category"))
	}
}

func TestBuildFallbackTopActions_OnlyRiskDriversGetBackendSafeExpectedEffect(t *testing.T) {
	drivers := []RiskDriver{
		makeRiskDriver("driver-stock", "stock", "high", 60),
	}
	actions := buildFallbackTopActionsForAIDiagnostics(nil, drivers, nil, topActionsLimit)
	if len(actions) != 1 {
		t.Fatalf("actions=%#v", actions)
	}
	action := actions[0]
	if action.Source != "risk_driver" {
		t.Fatalf("source=%q", action.Source)
	}
	if action.DriverID != "driver-stock" {
		t.Fatalf("driver_id=%q", action.DriverID)
	}
	if action.ExpectedEffect != fallbackExpectedEffect("stock") {
		t.Fatalf("expected_effect=%q", action.ExpectedEffect)
	}
}

func TestBuildFallbackTopActions_PreservesAISummaryExpectedEffect(t *testing.T) {
	aiSummaryActions := buildAISummaryFallbackTopActions([]ActionItem{
		{
			Priority:          "critical",
			Action:            "Остановите кампанию без заказов",
			Reason:            "Кампания тратит бюджет без результата.",
			ExpectedEffect:    "Сэкономить рекламный бюджет.",
			RelatedEntityType: "campaign",
			Source:            "ai_summary",
		},
		{
			Priority:       "high",
			Action:         "Без ожидаемого эффекта",
			Reason:         "Нет эффекта.",
			ExpectedEffect: "",
			Source:         "ai_summary",
		},
	})
	actions := buildFallbackTopActionsForAIDiagnostics(aiSummaryActions, nil, nil, topActionsLimit)
	if len(actions) != 1 {
		t.Fatalf("only AI-summary actions with expected_effect should qualify: %#v", actions)
	}
	if actions[0].ExpectedEffect != "Сэкономить рекламный бюджет." {
		t.Fatalf("expected_effect=%q", actions[0].ExpectedEffect)
	}
}

func TestBuildFallbackTopActions_DedupesByCategoryWhenAlternativesExist(t *testing.T) {
	pool := []TopActionItem{
		makeAdAction("ad-1", "critical", "Остановите неэффективную рекламную кампанию"),
		makeAdAction("ad-2", "high", "Сократите бюджет кампании с низкой отдачей"),
	}
	drivers := []RiskDriver{
		makeRiskDriver("driver-stock", "stock", "high", 60),
		makeRiskDriver("driver-funnel", "funnel", "medium", 40),
	}
	actions := buildFallbackTopActionsForAIDiagnostics(nil, drivers, pool, topActionsLimit)
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %#v", actions)
	}
	categories := map[string]int{}
	for _, action := range actions {
		categories[action.Category]++
	}
	if categories["advertising"] > 1 {
		t.Fatalf("expected at most one advertising card when alternatives exist, got %#v", actions)
	}
	if len(categories) < 3 {
		t.Fatalf("expected spread across categories, got %#v", actions)
	}
}

func TestBuildFallbackTopActions_AllowsMultipleSameCategoryWhenOnlyOption(t *testing.T) {
	pool := []TopActionItem{
		makeAdAction("ad-1", "critical", "Остановите неэффективную рекламную кампанию"),
		makeAdAction("ad-2", "high", "Сократите бюджет кампании с низкой отдачей"),
		makeAdAction("ad-3", "medium", "Поставьте рекламную кампанию на контроль"),
	}
	actions := buildFallbackTopActionsForAIDiagnostics(nil, nil, pool, topActionsLimit)
	if len(actions) != 3 {
		t.Fatalf("expected 3 advertising actions when only advertising exists, got %#v", actions)
	}
	for _, action := range actions {
		if action.Category != "advertising" {
			t.Fatalf("unexpected category in advertising-only fallback: %#v", action)
		}
	}
}

func TestBuildFallbackTopActions_EmptyWhenNoCandidates(t *testing.T) {
	actions := buildFallbackTopActionsForAIDiagnostics(nil, nil, nil, topActionsLimit)
	if len(actions) != 0 {
		t.Fatalf("expected empty fallback, got %#v", actions)
	}
}
