package outputvalidator

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

func TestSyncActionEntityCounts_RewritesTitleAfterPrune(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			{
				AnalysisActionID: "a1",
				Type:             "stock_replenishment",
				Category:         "stock",
				Priority:         "critical",
				ExecutionTier:    "required",
				Title:            "Пополнить 4 ключевых SKU с покрытием меньше 7 дней",
				Summary:          "Эти 4 SKU формируют 18% выручки.",
				Problem:          "4 SKU имеют низкое покрытие.",
				WhyNow:           "Срочно.",
				OperationalGoal:  "Довести покрытие.",
				ExpectedEffect:   "Рост эффективности",
				Scope: contract.UnifiedActionScopeV1{
					ScopeType: "sku",
					EntityIDs: []string{"100001"},
				},
				Steps: []contract.UnifiedActionStepV1{
					{Order: 1, Title: "Следующий шаг", Description: "Добавьте SKU в поставку."},
				},
				CompletionCriteria: "покрытие >= 7",
				EvidenceSummary:    "pack",
				EvidenceFactIDs:    []string{"sku:100001:stock_coverage_days_7d"},
				Confidence:         "high",
			},
		},
	}

	res := SyncActionEntityCounts(out)
	if res.TitlesSynced != 1 {
		t.Fatalf("TitlesSynced=%d want 1", res.TitlesSynced)
	}
	a := out.Actions[0]
	if ExtractMentionedEntityCount(a.Title) != 1 {
		t.Fatalf("title=%q want count 1", a.Title)
	}
	if ExtractMentionedEntityCount(a.Summary) != 1 {
		t.Fatalf("summary=%q want count 1", a.Summary)
	}
	if IsGenericExpectedEffect(a.ExpectedEffect) {
		t.Fatalf("expected_effect still generic: %q", a.ExpectedEffect)
	}
	if len(a.Steps) < 3 {
		t.Fatalf("steps=%d want >=3", len(a.Steps))
	}
	for _, step := range a.Steps {
		if step.Title == "Следующий шаг" {
			t.Fatalf("vague step remained: %+v", step)
		}
	}
}

func TestSyncActionEntityCounts_KeepsMatchingCount(t *testing.T) {
	out := &contract.UnifiedAnalysisOutputV1{
		Actions: []contract.UnifiedActionV1{
			{
				Title:          "Пополнить 2 SKU без остатка",
				Summary:        "2 SKU без остатка.",
				Problem:        "Нет остатка.",
				WhyNow:         "Каждый день без остатка — потеря заказов по подтверждённому спросу и риск снижения позиций.",
				ExpectedEffect: "После пополнения товары снова станут доступны для заказа, а риск потери продаж снизится существенно.",
				Category:       "stock",
				Scope: contract.UnifiedActionScopeV1{
					ScopeType: "sku",
					EntityIDs: []string{"1", "2"},
				},
				Steps: []contract.UnifiedActionStepV1{
					{Order: 1, Title: "Проверить остатки и входящие поставки", Description: "Сверить остаток и поставки."},
					{Order: 2, Title: "Рассчитать потребность", Description: "Рассчитать объём."},
					{Order: 3, Title: "Создать поставку", Description: "Оформить заявку."},
				},
			},
		},
	}
	res := SyncActionEntityCounts(out)
	if res.TitlesSynced != 0 {
		t.Fatalf("TitlesSynced=%d want 0", res.TitlesSynced)
	}
	if ExtractMentionedEntityCount(out.Actions[0].Title) != 2 {
		t.Fatalf("title changed unexpectedly: %q", out.Actions[0].Title)
	}
}
