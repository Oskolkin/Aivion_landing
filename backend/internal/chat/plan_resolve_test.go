package chat

import (
	"fmt"
	"strings"
	"testing"
)

func TestResolveValidatedToolPlan_TruncatesTooManyDistinctTools(t *testing.T) {
	registry := NewDefaultToolRegistry()
	validator := NewToolPlanValidator(registry)
	plan := ToolPlan{
		Intent:     ChatIntentStock,
		Confidence: 0.9,
		Language:   "ru",
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{}},
			{Name: ToolGetAlertsByGroup, Args: map[string]any{"group": "stock"}},
			{Name: ToolGetCriticalSKUs, Args: map[string]any{}},
			{Name: ToolGetStockRisks, Args: map[string]any{}},
			{Name: ToolGetOpenRecommendations, Args: map[string]any{}},
			{Name: ToolGetSKUMetrics, Args: map[string]any{"sku": "SKU-1"}},
		},
	}
	action := &ActionPromptContext{Title: "Пополнить SKU", Reason: "Низкий DOC"}

	validated, err := resolveValidatedToolPlan(validator, plan, action)
	if err != nil {
		t.Fatalf("expected truncated plan to validate: %v", err)
	}
	if len(validated.ToolCalls) != validator.MaxTools {
		t.Fatalf("expected %d tool calls, got %d", validator.MaxTools, len(validated.ToolCalls))
	}
	if !stringsContainsAny(strings.Join(validated.Warnings, " "), []string{"truncated"}) {
		t.Fatalf("expected truncation warning, got %#v", validated.Warnings)
	}
}

func TestResolveValidatedToolPlan_DuplicateSKUContextFallsBackToActionContext(t *testing.T) {
	registry := NewDefaultToolRegistry()
	validator := NewToolPlanValidator(registry)
	calls := make([]ToolCall, 0, 8)
	for i := 0; i < 8; i++ {
		calls = append(calls, ToolCall{
			Name: ToolGetSKUContext,
			Args: map[string]any{"sku": fmt.Sprintf("SKU-%d", i+1)},
		})
	}
	plan := ToolPlan{
		Intent:     ChatIntentStock,
		Confidence: 0.9,
		Language:   "ru",
		ToolCalls:  calls,
	}
	action := &ActionPromptContext{Title: "Пополнить SKU", Reason: "Низкий DOC"}

	validated, err := resolveValidatedToolPlan(validator, plan, action)
	if err != nil {
		t.Fatalf("expected action-context fallback: %v", err)
	}
	if len(validated.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls after duplicate-tool fallback, got %d", len(validated.ToolCalls))
	}
	if !stringsContainsAny(strings.Join(validated.Warnings, " "), []string{"action context only"}) {
		t.Fatalf("expected action-context-only warning, got %#v", validated.Warnings)
	}
}

func TestResolveValidatedToolPlan_ActionContextOnlyFallback(t *testing.T) {
	registry := NewDefaultToolRegistry()
	validator := NewToolPlanValidator(registry)
	plan := ToolPlan{
		Intent:     ChatIntentStock,
		Confidence: 0.9,
		Language:   "ru",
		ToolCalls:  []ToolCall{},
	}
	action := &ActionPromptContext{Title: "Пополнить SKU", Reason: "Низкий DOC"}

	validated, err := resolveValidatedToolPlan(validator, plan, action)
	if err != nil {
		t.Fatalf("expected action-context fallback: %v", err)
	}
	if len(validated.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(validated.ToolCalls))
	}
	if !stringsContainsAny(strings.Join(validated.Warnings, " "), []string{"action context only"}) {
		t.Fatalf("expected action-context-only warning, got %#v", validated.Warnings)
	}
}

func TestResolveValidatedToolPlan_InjectsDefaultsWithoutActionContext(t *testing.T) {
	registry := NewDefaultToolRegistry()
	validator := NewToolPlanValidator(registry)
	plan := ToolPlan{
		Intent:     ChatIntentStock,
		Confidence: 0.9,
		Language:   "ru",
		ToolCalls:  []ToolCall{},
	}
	validated, err := resolveValidatedToolPlan(validator, plan, nil)
	if err != nil {
		t.Fatalf("expected default tools injection: %v", err)
	}
	if len(validated.ToolCalls) == 0 {
		t.Fatal("expected injected tool calls")
	}
	if validated.ToolCalls[0].Name != ToolGetStockRisks {
		t.Fatalf("expected %s, got %s", ToolGetStockRisks, validated.ToolCalls[0].Name)
	}
}

func TestBuildAnswerUserPrompt_ActionScopedGuidance(t *testing.T) {
	prompt := BuildAnswerUserPrompt(FactContext{
		Question: "Помоги",
		Intent:   ChatIntentStock,
		Action:   &ActionPromptContext{Title: "Пополнить SKU"},
	})
	for _, marker := range []string{
		"action_context",
		"Action-scoped question",
		`supporting_facts.source="action_context"`,
	} {
		if !strings.Contains(prompt, marker) {
			t.Fatalf("expected marker %q in prompt: %s", marker, prompt)
		}
	}
}
