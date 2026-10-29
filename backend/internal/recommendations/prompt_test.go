package recommendations

import (
	"strings"
	"testing"
)

func TestDefaultPrompts_RequireRecommendationsWhenAlertsExist(t *testing.T) {
	sys := DefaultSystemPrompt()
	user := DefaultUserPrompt()
	for _, snippet := range []string{
		"5-8",
		"supporting_alert_ids",
		"Do not return empty recommendations",
		`"recommendations"`,
		"alerts.top_open",
	} {
		if !strings.Contains(strings.ToLower(sys+user), strings.ToLower(snippet)) {
			t.Fatalf("expected prompt to mention %q", snippet)
		}
	}
}

func TestDefaultPrompts_ContainsAllCanonicalRecommendationTypes(t *testing.T) {
	sys := DefaultSystemPrompt()
	for _, canon := range CanonicalRecommendationTypes {
		if !strings.Contains(sys, canon) {
			t.Fatalf("system prompt missing canonical recommendation_type %q", canon)
		}
	}
	// Aliases must not appear as allowed values in the prompt enum block.
	if strings.Contains(sys, "stock_replenishment") {
		t.Fatal("system prompt must not list alias stock_replenishment as allowed value")
	}
	for _, disabled := range LegacyRecommendationTypes {
		if strings.Contains(sys, disabled) {
			t.Fatalf("system prompt must not list disabled type %q", disabled)
		}
	}
}

func TestCanonicalRecommendationTypesJSONSchemaEnum_MatchesCanonicalList(t *testing.T) {
	enum := CanonicalRecommendationTypesJSONSchemaEnum()
	if len(enum) != len(CanonicalRecommendationTypes) {
		t.Fatalf("enum length %d != canonical %d", len(enum), len(CanonicalRecommendationTypes))
	}
	for i, v := range enum {
		if v != CanonicalRecommendationTypes[i] {
			t.Fatalf("enum[%d]=%q want %q", i, v, CanonicalRecommendationTypes[i])
		}
	}
}

func TestNewService_AppliesDefaultPrompts(t *testing.T) {
	svc := NewService(&mockServiceRepo{}, mockBuilder{}, mockAIClient{}, mockValidator{}, ServiceConfig{}, nil)
	if strings.TrimSpace(svc.cfg.SystemPrompt) == "" || strings.TrimSpace(svc.cfg.UserPrompt) == "" {
		t.Fatalf("expected default prompts to be set")
	}
}
