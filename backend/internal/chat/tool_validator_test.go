package chat

import "testing"

func strPtr(v string) *string { return &v }

func TestToolPlanValidatorRejectsUnknownTool(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentUnknown,
		ToolCalls: []ToolCall{
			{Name: "unknown_tool", Args: map[string]any{}},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestToolPlanValidatorRejectsSellerAccountArg(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{"seller_account_id": int64(1)}},
		},
	})
	if err == nil {
		t.Fatal("expected seller_account_id rejection")
	}
}

func TestToolPlanValidatorRejectsTooManyTools(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	v.MaxTools = 1
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{}},
			{Name: ToolGetOpenRecommendations, Args: map[string]any{}},
		},
	})
	if err == nil {
		t.Fatal("expected too many tools error")
	}
}

func TestToolPlanValidatorAcceptsValidPlan(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent:      ChatIntentAlerts,
		Assumptions: []string{"a"},
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{"limit": 10, "severities": []string{"high"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out.ToolCalls))
	}
}

func TestToolPlanValidatorRejectsUnknownArg(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{"foo": "bar"}},
		},
	})
	if err == nil {
		t.Fatal("expected unknown arg error")
	}
}

func TestToolPlanValidatorRejectsInvalidEnum(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentABCAnalysis,
		ToolCalls: []ToolCall{
			{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "profit"}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid enum error")
	}
}

func TestToolPlanValidatorClampsLimitAboveMax(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent: ChatIntentABCAnalysis,
		ToolCalls: []ToolCall{
			{Name: ToolRunABCAnalysis, Args: map[string]any{"limit": 999, "metric": "revenue"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	limit := out.ToolCalls[0].Args["limit"].(int)
	if limit != MaxABCAnalysisLimit {
		t.Fatalf("expected limit clamp to %d, got %d", MaxABCAnalysisLimit, limit)
	}
	if len(out.Warnings) == 0 {
		t.Fatal("expected clamp warning")
	}
}

func TestToolPlanValidatorRejectsMissingCampaignID(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAdvertising,
		ToolCalls: []ToolCall{
			{Name: ToolGetCampaignContext, Args: map[string]any{}},
		},
	})
	if err == nil {
		t.Fatal("expected missing required campaign_id error")
	}
}

func TestToolPlanValidatorAcceptsValidABCPlan(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent: ChatIntentABCAnalysis,
		ToolCalls: []ToolCall{
			{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "orders", "limit": 100}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(out.ToolCalls))
	}
}

func TestToolPlanValidatorRejectsInvalidConfidence(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent:     ChatIntentAlerts,
		Confidence: 1.1,
		ToolCalls:  []ToolCall{{Name: ToolGetOpenAlerts, Args: map[string]any{}}},
	})
	if err == nil {
		t.Fatal("expected confidence validation error")
	}
}

func TestToolPlanValidatorCoercesUnknownIntent(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent:    ChatIntent("some_new_intent"),
		Language:  "ru",
		ToolCalls: []ToolCall{},
	})
	if err != nil {
		t.Fatalf("expected unknown intent coercion, got %v", err)
	}
	if out.Intent != ChatIntentGeneralOverview {
		t.Fatalf("expected general_overview, got %s", out.Intent)
	}
}

func TestToolPlanValidatorUnsupportedIntentRequiresEmptyCalls(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent:            ChatIntentUnsupported,
		UnsupportedReason: strPtr("not supported"),
		ToolCalls:         []ToolCall{{Name: ToolGetOpenAlerts, Args: map[string]any{}}},
	})
	if err == nil {
		t.Fatal("expected unsupported intent empty calls error")
	}
}

func TestToolPlanValidatorUnsupportedIntentRequiresReason(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent:    ChatIntentUnsupported,
		ToolCalls: []ToolCall{},
	})
	if err == nil {
		t.Fatal("expected unsupported reason error")
	}
}

func TestToolPlanValidatorAllowsEmptyToolsForConversationalIntents(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent:    ChatIntentGeneralOverview,
		Language:  "ru",
		ToolCalls: []ToolCall{},
	})
	if err != nil {
		t.Fatalf("expected conversational empty tools to pass: %v", err)
	}
	if len(out.ToolCalls) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(out.ToolCalls))
	}
}

func TestToolPlanValidatorNonUnsupportedRequiresTools(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent:    ChatIntentAlerts,
		ToolCalls: []ToolCall{},
	})
	if err == nil {
		t.Fatal("expected at least one tool call error")
	}
}

func TestToolPlanValidatorRejectsDuplicateToolCalls(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{}},
			{Name: ToolGetOpenAlerts, Args: map[string]any{"groups": []string{"stock"}}},
			{Name: ToolGetOpenAlerts, Args: map[string]any{"limit": 5}},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate tool call error when exceeding MaxSameToolCalls")
	}
}

func TestToolPlanValidatorAllowsTwoSameToolCalls(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetDashboardSummary, Args: map[string]any{"as_of_date": "2026-07-29"}},
			{Name: ToolGetDashboardSummary, Args: map[string]any{"as_of_date": "2026-07-22"}},
		},
	})
	if err != nil {
		t.Fatalf("expected two dashboard summary calls for week compare: %v", err)
	}
	if len(out.ToolCalls) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(out.ToolCalls))
	}
}

func TestToolPlanValidatorRejectsForbiddenArgs(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	cases := []string{"api_key", "sql", "raw_payload"}
	for _, arg := range cases {
		_, err := v.Validate(&ToolPlan{
			Intent: ChatIntentAlerts,
			ToolCalls: []ToolCall{
				{Name: ToolGetOpenAlerts, Args: map[string]any{arg: "x"}},
			},
		})
		if err == nil {
			t.Fatalf("expected forbidden arg error for %s", arg)
		}
	}
}

func TestToolPlanValidatorRejectsSQLLikeStringArgValue(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentPricing,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUMetrics, Args: map[string]any{"offer_id": "select * from users"}},
		},
	})
	if err == nil {
		t.Fatal("expected sql-like value rejection")
	}
}

func TestToolPlanValidatorRejectsInvalidDateFormat(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUMetrics, Args: map[string]any{"date_from": "2026/01/01"}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid date format error")
	}
}

func TestToolPlanValidatorRejectsDateFromAfterDateTo(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUMetrics, Args: map[string]any{"date_from": "2026-05-01", "date_to": "2026-04-01"}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid date order error")
	}
}

func TestToolPlanValidatorRejectsDateRangeTooLarge(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUMetrics, Args: map[string]any{"date_from": "2026-01-01", "date_to": "2026-05-01"}},
		},
	})
	if err == nil {
		t.Fatal("expected date range max error")
	}
}

func TestToolPlanValidatorAcceptsValidDateRange(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUMetrics, Args: map[string]any{"date_from": "2026-01-01", "date_to": "2026-02-15"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolPlanValidatorRejectsInvalidSeverityEnum(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{"severities": []string{"severe"}}},
		},
	})
	if err == nil {
		t.Fatal("expected severity enum error")
	}
}

func TestToolPlanValidatorRejectsInvalidGroupEnum(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetAlertsByGroup, Args: map[string]any{"group": "finance"}},
		},
	})
	if err == nil {
		t.Fatal("expected group enum error")
	}
}

func TestToolPlanValidatorRejectsInvalidHorizonEnum(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentRecommendations,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenRecommendations, Args: map[string]any{"horizon": "today"}},
		},
	})
	if err == nil {
		t.Fatal("expected horizon enum error")
	}
}

func TestToolPlanValidatorRejectsInvalidMetricEnum(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentABCAnalysis,
		ToolCalls: []ToolCall{
			{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "gmv"}},
		},
	})
	if err == nil {
		t.Fatal("expected metric enum error")
	}
}

func TestToolPlanValidatorRejectsInvalidSortByEnum(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUMetrics, Args: map[string]any{"sort_by": "ctr"}},
		},
	})
	if err == nil {
		t.Fatal("expected sort_by enum error")
	}
}

func TestToolPlanValidatorAppliesDefaultLimit(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	out, err := v.Validate(&ToolPlan{
		Intent: ChatIntentAlerts,
		ToolCalls: []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	limit := out.ToolCalls[0].Args["limit"].(int)
	if limit != DefaultToolLimit {
		t.Fatalf("expected default limit %d, got %d", DefaultToolLimit, limit)
	}
}

func TestToolPlanValidatorRejectsSKUContextWithoutKeys(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUContext, Args: map[string]any{}},
		},
	})
	if err == nil {
		t.Fatal("expected sku_context cross-field error")
	}
}

func TestToolPlanValidatorAcceptsSKUContextWithOfferID(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentSales,
		ToolCalls: []ToolCall{
			{Name: ToolGetSKUContext, Args: map[string]any{"offer_id": "A-1"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolPlanValidatorRejectsToolNotSupportedForIntent(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentABCAnalysis,
		ToolCalls: []ToolCall{
			{Name: ToolGetCampaignContext, Args: map[string]any{"campaign_id": 123}},
		},
	})
	if err == nil {
		t.Fatal("expected tool-intent mismatch error")
	}
}

func TestToolPlanValidatorAcceptsValidPrioritiesPlan(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentPriorities,
		ToolCalls: []ToolCall{
			{Name: ToolGetDashboardSummary, Args: map[string]any{}},
			{Name: ToolGetOpenRecommendations, Args: map[string]any{"priority_levels": []string{"high"}}},
			{Name: ToolGetOpenAlerts, Args: map[string]any{"groups": []string{"stock"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolPlanValidatorAcceptsValidUnsafeAdsPlan(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentUnsafeAds,
		ToolCalls: []ToolCall{
			{Name: ToolGetAdvertisingAnalytics, Args: map[string]any{}},
			{Name: ToolGetStockRisks, Args: map[string]any{"limit": 5}},
			{Name: ToolGetOpenAlerts, Args: map[string]any{"groups": []string{"advertising"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolPlanValidatorAcceptsValidABCAnalysisPlan(t *testing.T) {
	v := NewToolPlanValidator(NewDefaultToolRegistry())
	_, err := v.Validate(&ToolPlan{
		Intent: ChatIntentABCAnalysis,
		ToolCalls: []ToolCall{
			{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "revenue", "limit": 150}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
