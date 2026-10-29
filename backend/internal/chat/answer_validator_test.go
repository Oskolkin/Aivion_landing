package chat

import (
	"strings"
	"testing"
)

func TestAnswerValidatorRejectsNilAnswer(t *testing.T) {
	v := NewAnswerValidator()
	_, err := v.Validate(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil answer")
	}
}

func TestAnswerValidatorRejectsRequiredFields(t *testing.T) {
	v := NewAnswerValidator()
	result, err := v.Validate(&ChatAnswer{
		Answer:          " ",
		Summary:         " ",
		ConfidenceLevel: ConfidenceLevel("bad"),
		SupportingFacts: []SupportingFact{},
	}, buildFactContext())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Fatal("expected invalid result")
	}
}

func TestAnswerValidatorAcceptsLimitationOnlyForNoFactsLowConfidence(t *testing.T) {
	v := NewAnswerValidator()
	result, err := v.Validate(&ChatAnswer{
		Answer:          "По доступным данным точных фактов нет.",
		Summary:         "Недостаточно данных.",
		ConfidenceLevel: ConfidenceLevelLow,
		SupportingFacts: []SupportingFact{{Source: "limitation", Fact: "No factual data was available for this question."}},
		Limitations:     []string{"No factual data was available for this question."},
	}, &FactContext{
		Limitations: []string{"No factual data was available for this question."},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid limitation-only answer, errors=%v", result.Errors)
	}
	if result.FinalConfidenceLevel != ConfidenceLevelLow {
		t.Fatalf("expected low confidence")
	}
}

func TestAnswerValidatorRejectsUnknownRelatedIDs(t *testing.T) {
	v := NewAnswerValidator()
	ctx := buildFactContext()
	result, err := v.Validate(&ChatAnswer{
		Answer:                   "A",
		Summary:                  "S",
		ConfidenceLevel:          ConfidenceLevelMedium,
		SupportingFacts:          []SupportingFact{{Source: "dashboard", Fact: "f"}},
		RelatedAlertIDs:          []int64{999},
		RelatedRecommendationIDs: []int64{888},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Fatal("expected invalid for unknown related ids")
	}
}

func TestAnswerValidatorSupportingFactsReferenceValidation(t *testing.T) {
	v := NewAnswerValidator()
	ctx := buildFactContext()
	badID := int64(999)
	result, err := v.Validate(&ChatAnswer{
		Answer:          "A",
		Summary:         "S",
		ConfidenceLevel: ConfidenceLevelMedium,
		SupportingFacts: []SupportingFact{
			{Source: "alert", ID: &badID, Fact: "f1"},
			{Source: "recommendation", ID: &badID, Fact: "f2"},
		},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Fatal("expected invalid for bad supporting fact ids")
	}
}

func TestAnswerValidatorWarnsOnUnknownSupportingSource(t *testing.T) {
	v := NewAnswerValidator()
	result, err := v.Validate(&ChatAnswer{
		Answer:          "A",
		Summary:         "S",
		ConfidenceLevel: ConfidenceLevelMedium,
		SupportingFacts: []SupportingFact{{Source: "weird_source", Fact: "f"}},
	}, buildFactContext())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for unknown source")
	}
}

func TestAnswerValidatorRejectsForbiddenClaims(t *testing.T) {
	v := NewAnswerValidator()
	cases := []string{
		"я изменил цену для sku",
		"I stopped the campaign yesterday",
		"я выполнил SQL select *",
		"я проверил ozon напрямую",
		"my key is sk-abc123",
	}
	for _, text := range cases {
		result, err := v.Validate(&ChatAnswer{
			Answer:          text,
			Summary:         "sum",
			ConfidenceLevel: ConfidenceLevelMedium,
			SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "f"}},
		}, buildFactContext())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsValid {
			t.Fatalf("expected invalid for forbidden claim: %s", text)
		}
	}
}

func TestAnswerValidatorDoesNotRejectSafeRecommendationWording(t *testing.T) {
	v := NewAnswerValidator()
	result, err := v.Validate(&ChatAnswer{
		Answer:          "Рекомендую вручную проверить цену и при необходимости скорректировать.",
		Summary:         "Нужна ручная проверка.",
		ConfidenceLevel: ConfidenceLevelMedium,
		SupportingFacts: []SupportingFact{{Source: "recommendation", Fact: "Есть риск маржи."}},
	}, buildFactContext())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid safe wording, errors=%v", result.Errors)
	}
}

func TestAnswerValidatorRejectsTextHallucinatedIDs(t *testing.T) {
	v := NewAnswerValidator()
	result, err := v.Validate(&ChatAnswer{
		Answer:          "alert 999 требует внимания, recommendation 999 тоже",
		Summary:         "S",
		ConfidenceLevel: ConfidenceLevelMedium,
		SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "f"}},
	}, buildFactContext())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Fatal("expected invalid hallucinated ids")
	}
}

func TestAnswerValidatorLimitationsDowngradeConfidence(t *testing.T) {
	v := NewAnswerValidator()
	ctx := buildFactContext()
	ctx.Limitations = []string{"Category filtering may be approximate."}
	result, err := v.Validate(&ChatAnswer{
		Answer:          "Вот вывод по данным.",
		Summary:         "S",
		ConfidenceLevel: ConfidenceLevelHigh,
		SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "f"}},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalConfidenceLevel != ConfidenceLevelMedium {
		t.Fatalf("expected downgrade high->medium, got %s", result.FinalConfidenceLevel)
	}
	if !strings.Contains(strings.Join(result.Warnings, " "), "limitations") {
		t.Fatal("expected limitations warning")
	}
}

func TestAnswerValidatorCriticalNoDataSetsLowConfidence(t *testing.T) {
	v := NewAnswerValidator()
	result, err := v.Validate(&ChatAnswer{
		Answer:          "Ограничение: данных недостаточно.",
		Summary:         "S",
		ConfidenceLevel: ConfidenceLevelHigh,
		SupportingFacts: []SupportingFact{{Source: "limitation", Fact: "No factual data was available for this question."}},
		Limitations:     []string{"No factual data was available for this question."},
	}, &FactContext{
		Limitations: []string{"No factual data was available for this question."},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalConfidenceLevel != ConfidenceLevelLow {
		t.Fatalf("expected final low confidence, got %s", result.FinalConfidenceLevel)
	}
}

func TestAnswerValidatorTruncatedAndFailedToolsDowngrade(t *testing.T) {
	v := NewAnswerValidator()
	ctx := buildFactContext()
	ctx.ContextStats.Truncated = true
	ctx.ContextStats.FailedToolsCount = 1
	result, err := v.Validate(&ChatAnswer{
		Answer:          "Ограничения учтены.",
		Summary:         "S",
		ConfidenceLevel: ConfidenceLevelHigh,
		SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "f"}},
		Limitations:     []string{"часть данных недоступна"},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinalConfidenceLevel != ConfidenceLevelLow {
		t.Fatalf("expected double downgrade to low, got %s", result.FinalConfidenceLevel)
	}
}

func TestAnswerValidatorValidNormalAnswerWithReferences(t *testing.T) {
	v := NewAnswerValidator()
	ctx := buildFactContext()
	alertID := int64(101)
	recID := int64(201)
	result, err := v.Validate(&ChatAnswer{
		Answer:                   "Есть риск по alert 101 и recommendation 201.",
		Summary:                  "Нужны ручные действия.",
		ConfidenceLevel:          ConfidenceLevelMedium,
		RelatedAlertIDs:          []int64{alertID},
		RelatedRecommendationIDs: []int64{recID},
		SupportingFacts: []SupportingFact{
			{Source: "alert", ID: &alertID, Fact: "Высокая срочность."},
			{Source: "recommendation", ID: &recID, Fact: "Рекомендуется корректировка."},
		},
		Limitations: []string{"Категорийный фильтр приблизительный."},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid normal answer, errors=%v", result.Errors)
	}
}

func TestAnswerValidatorValidDashboardFactWithoutRelatedIDs(t *testing.T) {
	v := NewAnswerValidator()
	ctx := buildFactContext()
	result, err := v.Validate(&ChatAnswer{
		Answer:          "Выручка выросла по dashboard-метрикам.",
		Summary:         "Рост выручки.",
		ConfidenceLevel: ConfidenceLevelMedium,
		SupportingFacts: []SupportingFact{{Source: "dashboard", Fact: "Revenue +10% d/d"}},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid dashboard answer, errors=%v", result.Errors)
	}
}

func TestAnswerValidatorAcceptsUnsupportedSafeRefusal(t *testing.T) {
	v := NewAnswerValidator()
	ctx := &FactContext{
		Intent:      ChatIntentUnsupported,
		Limitations: []string{"Запрос требует auto-action, который запрещен в AI Chat MVP."},
	}
	result, err := v.Validate(&ChatAnswer{
		Answer:          "Я не могу выполнить это действие автоматически. Могу помочь проанализировать данные и подсказать ручной шаг.",
		Summary:         "Запрос требует действия, которое AI-чат не выполняет.",
		Intent:          ChatIntentUnsupported,
		ConfidenceLevel: ConfidenceLevelHigh,
		SupportingFacts: []SupportingFact{{Source: "limitation", Fact: "AI-чат не выполняет auto-actions."}},
		Limitations:     []string{"Запрос требует auto-action, который запрещен в AI Chat MVP."},
	}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Fatalf("expected valid unsupported safe refusal, errors=%v", result.Errors)
	}
}

func buildFactContext() *FactContext {
	return &FactContext{
		Facts: FactContextFacts{
			Dashboard: map[string]any{"kpi": map[string]any{"revenue": 100}},
		},
		RelatedAlerts: []FactAlertReference{
			{ID: 101, AlertType: "stock"},
		},
		RelatedRecommendations: []FactRecommendationReference{
			{ID: 201, RecommendationType: "pricing"},
		},
	}
}
