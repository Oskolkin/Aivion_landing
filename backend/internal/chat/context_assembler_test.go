package chat

import (
	"strings"
	"testing"
	"time"
)

func TestContextAssemblerAssembleBasicDashboard(t *testing.T) {
	a := NewContextAssembler()
	asOf := time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC)
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Что с выручкой?",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentSales, Assumptions: []string{"A1"}},
		ToolResults: []ToolResult{{
			Name: ToolGetDashboardSummary,
			Data: map[string]any{"as_of_date": "2026-05-05", "data_freshness": "fresh", "kpi": map[string]any{"revenue": 1234}},
		}},
		AsOfDate: &asOf,
		Language: "ru",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.ContextVersion != FactContextVersionV1 {
		t.Fatalf("unexpected context version: %s", ctx.ContextVersion)
	}
	if ctx.Facts.Dashboard["as_of_date"] != "2026-05-05" {
		t.Fatalf("dashboard routing failed")
	}
	if _, ok := ctx.Freshness["dashboard"]; !ok {
		t.Fatalf("dashboard freshness expected")
	}
	if !ctx.GeneratedAt.UTC().Equal(ctx.GeneratedAt) {
		t.Fatalf("generated_at must be UTC")
	}
}

func TestContextAssemblerRoutesRecommendationsAndDedupesRelated(t *testing.T) {
	a := NewContextAssembler()
	results := []ToolResult{
		{
			Name: ToolGetOpenRecommendations,
			Data: map[string]any{"items": []any{
				map[string]any{"id": 1, "recommendation_type": "pricing", "priority_level": "high", "title": "R1", "recommended_action": "A1"},
				map[string]any{"id": 1, "recommendation_type": "pricing", "priority_level": "high", "title": "R1", "recommended_action": "A1"},
			}},
		},
		{
			Name: ToolGetRecommendationDetail,
			Data: map[string]any{
				"recommendation": map[string]any{"id": 2, "recommendation_type": "stock", "priority_level": "medium", "title": "R2", "recommended_action": "A2"},
				"related_alerts": []any{map[string]any{"id": 21, "alert_type": "stockout", "group": "stock", "title": "A"}},
			},
		},
	}
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Что приоритетно?",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentPriorities},
		ToolResults:     results,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Facts.Recommendations) != 2 {
		t.Fatalf("recommendations should be routed")
	}
	if len(ctx.RelatedRecommendations) != 2 {
		t.Fatalf("expected deduped related recommendations, got %d", len(ctx.RelatedRecommendations))
	}
	if len(ctx.RelatedAlerts) != 1 {
		t.Fatalf("related alerts should include recommendation detail alerts")
	}
}

func TestContextAssemblerRoutesAlertsAndPriceEconomicsSeparately(t *testing.T) {
	a := NewContextAssembler()
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Какие риски?",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentAlerts},
		ToolResults: []ToolResult{
			{Name: ToolGetOpenAlerts, Data: map[string]any{"items": []any{map[string]any{"id": 1, "alert_type": "ad_loss"}}}},
			{Name: ToolGetPriceEconomicsRisks, Data: map[string]any{"items": []any{map[string]any{"id": 2, "alert_type": "price_economics"}}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ctx.Facts.Alerts) != 1 {
		t.Fatalf("alerts section mismatch")
	}
	if len(ctx.Facts.PriceEconomicsRisks) != 1 {
		t.Fatalf("price economics section mismatch")
	}
}

func TestContextAssemblerSanitizesForbiddenAndTruncatesString(t *testing.T) {
	a := NewContextAssembler()
	a.MaxTextLength = 10
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Q",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentUnknown},
		ToolResults: []ToolResult{{
			Name: ToolGetSKUContext,
			Data: map[string]any{
				"product": map[string]any{
					"title":           "very long text that must be cut",
					"raw_ai_response": "secret",
					"token":           "secret2",
					"sql":             "SELECT *",
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	product := ctx.Facts.SKUContexts[0]["product"].(map[string]any)
	if _, ok := product["raw_ai_response"]; ok {
		t.Fatalf("raw_ai_response must be stripped")
	}
	if _, ok := product["token"]; ok {
		t.Fatalf("token must be stripped")
	}
	if _, ok := product["sql"]; ok {
		t.Fatalf("sql must be stripped")
	}
	if !strings.Contains(product["title"].(string), "[truncated]") {
		t.Fatalf("long strings must be truncated")
	}
}

func TestContextAssemblerAggregatesLimitationsAndErrors(t *testing.T) {
	a := NewContextAssembler()
	errMsg := "boom"
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Q",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentUnknown},
		ToolResults: []ToolResult{
			{Name: ToolGetOpenAlerts, Data: map[string]any{"items": []any{}}, Limitations: []string{"L1"}},
			{Name: ToolGetAdvertisingAnalytics, Data: map[string]any{}, Error: &errMsg},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.ContextStats.FailedToolsCount != 1 {
		t.Fatalf("failed tools count mismatch")
	}
	joined := strings.Join(ctx.Limitations, " | ")
	if !strings.Contains(joined, "L1") || !strings.Contains(joined, "Tool get_advertising_analytics failed: boom") {
		t.Fatalf("limitations should include tool limitations and failures")
	}
}

func TestContextAssemblerSizeLimitTriggersTruncation(t *testing.T) {
	a := NewContextAssembler()
	a.MaxContextBytes = 1000
	items := make([]any, 0, 40)
	for i := 0; i < 40; i++ {
		items = append(items, map[string]any{"id": i + 1, "title": strings.Repeat("x", 200)})
	}
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Q",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentRecommendations},
		ToolResults: []ToolResult{{
			Name: ToolGetOpenRecommendations,
			Data: map[string]any{"items": items},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.ContextStats.Truncated {
		t.Fatalf("context must be truncated when over size limit")
	}
	if !contains(ctx.Limitations, "Context was truncated to fit size limits.") {
		t.Fatalf("truncation limitation missing")
	}
}

func TestContextAssemblerHandlesEmptyToolResultsGracefully(t *testing.T) {
	a := NewContextAssembler()
	ctx, err := a.Assemble(AssembleContextInput{
		Question:        "Q",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentUnsupported},
		ToolResults:     nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.ContextStats.ToolResultsCount != 0 {
		t.Fatalf("tool result count mismatch")
	}
	if !contains(ctx.Limitations, "No factual data was available for this question.") {
		t.Fatalf("empty facts limitation expected")
	}
}

func TestContextAssemblerReturnsErrorForEmptyQuestion(t *testing.T) {
	a := NewContextAssembler()
	_, err := a.Assemble(AssembleContextInput{
		Question:        "  ",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentUnknown},
	})
	if err == nil {
		t.Fatalf("expected error for empty question")
	}
}

func TestContextAssemblerDoesNotMutateSourceToolResult(t *testing.T) {
	a := NewContextAssembler()
	src := ToolResult{
		Name: ToolGetSKUContext,
		Data: map[string]any{
			"product": map[string]any{"token": "secret", "title": "safe"},
		},
	}
	_, err := a.Assemble(AssembleContextInput{
		Question:        "Q",
		SellerAccountID: 11,
		Plan:            ValidatedToolPlan{Intent: ChatIntentUnknown},
		ToolResults:     []ToolResult{src},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := src.Data.(map[string]any)["product"].(map[string]any)["token"]; !exists {
		t.Fatalf("source tool result must stay unchanged")
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
