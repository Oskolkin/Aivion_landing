package chat

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPlannerUserPromptWithExplicitAsOfDate(t *testing.T) {
	asOf := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	prompt := BuildPlannerUserPrompt("Покажи продажи", &asOf, nil)
	if !strings.Contains(prompt, "2026-05-10") {
		t.Fatalf("expected explicit as-of date in prompt: %s", prompt)
	}
}

func TestBuildPlannerUserPrompt_WithActionContext(t *testing.T) {
	prompt := BuildPlannerUserPrompt("Что сделать?", nil, &ActionPromptContext{
		Title:              "Пополнить SKU",
		Reason:             "Низкое покрытие",
		NextStep:           "Сделать поставку",
		Evidence:           []string{"DOC: 2.1"},
		EvidenceFactIDs:    []string{"sku.1.last_7d.orders"},
		ObjectActions:      []string{"SKU-1: Добавить в поставку"},
		CompletionCriteria: "SKU покрыт запасом на 14 дней",
		Limitations:        []string{"Нет прогноза поставок"},
	})
	for _, marker := range []string{
		"Action context from Actions Engine",
		"Title: Пополнить SKU",
		"Why: Низкое покрытие",
		"Next step: Сделать поставку",
		"Evidence:",
		"Evidence fact ids:",
		"Object actions:",
		"Completion criteria:",
		"Limitations:",
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("expected marker %q in prompt: %s", marker, prompt)
		}
	}
	if strings.Contains(prompt, "entity_group_json") || strings.Contains(prompt, "source_candidate_ids") {
		t.Fatalf("planner prompt leaked technical payload: %s", prompt)
	}
}

func TestBuildAnswerUserPrompt_IncludesActionContext(t *testing.T) {
	prompt := BuildAnswerUserPrompt(FactContext{
		Question: "Что сделать?",
		Intent:   ChatIntentPriorities,
		Action: &ActionPromptContext{
			Title:       "Пополнить SKU",
			Reason:      "Низкое покрытие",
			NextStep:    "Сделать поставку",
			Evidence:    []string{"DOC: 2.1"},
			Limitations: []string{"Нет прогноза поставок"},
		},
	})
	for _, marker := range []string{
		"action_context",
		"Пополнить SKU",
		"Низкое покрытие",
		"Сделать поставку",
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("expected marker %q in answer prompt: %s", marker, prompt)
		}
	}
}
