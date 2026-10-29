package chat

import (
	"fmt"
	"strings"
)

// resolveValidatedToolPlan validates a planner output, injects safe default tools when
// the planner returns a domain intent with an empty plan, and applies action-context fallbacks.
func resolveValidatedToolPlan(
	validator *ToolPlanValidator,
	plan ToolPlan,
	actionContext *ActionPromptContext,
) (*ValidatedToolPlan, error) {
	// Inject safe defaults only when there is no action context to fall back to.
	// With action context, empty domain plans still recover via action-only fallback.
	if actionContext == nil {
		plan = injectDefaultToolsForEmptyDomainPlan(plan)
	}
	validated, err := validator.Validate(&plan)
	if err == nil {
		return validated, nil
	}

	// Intent/tool mismatch: keep tools, coerce intent to general_overview and retry.
	if actionContext == nil && strings.Contains(err.Error(), "is not supported for intent") && len(plan.ToolCalls) > 0 {
		retryPlan := plan
		retryPlan.Intent = ChatIntentGeneralOverview
		if retry, retryErr := validator.Validate(&retryPlan); retryErr == nil {
			retry.Warnings = append(retry.Warnings, "intent coerced to general_overview after tool/intent mismatch")
			return retry, nil
		}
	}

	// Without action context, retry once with defaults if validation still wants tools.
	if actionContext == nil && strings.Contains(err.Error(), "requires at least one tool call") {
		withDefaults := injectDefaultToolsForEmptyDomainPlan(ToolPlan{
			Intent:      plan.Intent,
			Confidence:  plan.Confidence,
			Language:    plan.Language,
			Assumptions: append([]string(nil), plan.Assumptions...),
			ToolCalls:   nil,
		})
		if len(withDefaults.ToolCalls) > 0 {
			if retry, retryErr := validator.Validate(&withDefaults); retryErr == nil {
				retry.Warnings = append(retry.Warnings, "default cabinet tools injected after empty planner plan")
				return retry, nil
			}
		}
	}

	// Drop invalid tool calls missing required args, keep the rest.
	if actionContext == nil && (strings.Contains(err.Error(), "requires sku") || strings.Contains(err.Error(), "required")) && len(plan.ToolCalls) > 0 {
		filtered := plan
		filtered.ToolCalls = filterValidToolCalls(validator, plan)
		if len(filtered.ToolCalls) == 0 {
			filtered = injectDefaultToolsForEmptyDomainPlan(ToolPlan{
				Intent: plan.Intent, Confidence: plan.Confidence, Language: plan.Language, Assumptions: plan.Assumptions,
			})
		}
		if len(filtered.ToolCalls) > 0 {
			if retry, retryErr := validator.Validate(&filtered); retryErr == nil {
				retry.Warnings = append(retry.Warnings, "invalid tool calls removed; continued with valid/default tools")
				return retry, nil
			}
		}
	}

	// Deduplicate excess same-tool calls (keep first MaxSameToolCalls per name).
	if actionContext == nil && strings.Contains(err.Error(), "duplicate tool call") {
		trimmed := plan
		trimmed.ToolCalls = trimDuplicateToolCalls(plan.ToolCalls, validator.MaxSameToolCalls)
		if len(trimmed.ToolCalls) > 0 && len(trimmed.ToolCalls) < len(plan.ToolCalls) {
			if retry, retryErr := validator.Validate(&trimmed); retryErr == nil {
				retry.Warnings = append(retry.Warnings, "duplicate tool calls trimmed to validator limit")
				return retry, nil
			}
		}
	}

	if actionContext == nil || plan.Intent == ChatIntentUnsupported {
		return nil, err
	}
	if len(plan.ToolCalls) > validator.MaxTools {
		truncated := plan
		truncated.ToolCalls = append([]ToolCall(nil), plan.ToolCalls[:validator.MaxTools]...)
		if validated, truncErr := validator.Validate(&truncated); truncErr == nil {
			validated.Warnings = append(validated.Warnings,
				fmt.Sprintf("tool plan truncated from %d to %d calls", len(plan.ToolCalls), validator.MaxTools))
			return validated, nil
		}
	}
	if canFallbackToActionContextOnly(err) {
		return &ValidatedToolPlan{
			Intent:            plan.Intent,
			ToolCalls:         []ToolCall{},
			Assumptions:       append([]string(nil), plan.Assumptions...),
			Warnings:          []string{"using action context only"},
			UnsupportedReason: nil,
		}, nil
	}

	// Last resort without action context: never surface invalid_tool_plan for recoverable cabinet asks.
	if actionContext == nil && plan.Intent != ChatIntentUnsupported {
		fallback := injectDefaultToolsForEmptyDomainPlan(ToolPlan{
			Intent:      plan.Intent,
			Confidence:  plan.Confidence,
			Language:    plan.Language,
			Assumptions: append([]string(nil), plan.Assumptions...),
		})
		if len(fallback.ToolCalls) == 0 {
			fallback.Intent = ChatIntentGeneralOverview
			fallback.ToolCalls = []ToolCall{{Name: ToolGetDashboardSummary, Args: map[string]any{}}}
		}
		if retry, retryErr := validator.Validate(&fallback); retryErr == nil {
			retry.Warnings = append(retry.Warnings, "last-resort default tools after plan validation failure: "+err.Error())
			return retry, nil
		}
		// Absolute last resort: conversational answer without tools.
		return &ValidatedToolPlan{
			Intent:      ChatIntentGeneralOverview,
			ToolCalls:   []ToolCall{},
			Assumptions: append([]string(nil), plan.Assumptions...),
			Warnings:    []string{"plan validation failed; answering without tools: " + err.Error()},
		}, nil
	}
	return nil, err
}

func injectDefaultToolsForEmptyDomainPlan(plan ToolPlan) ToolPlan {
	if len(plan.ToolCalls) > 0 {
		return plan
	}
	switch plan.Intent {
	case ChatIntentUnsupported, ChatIntentGeneralOverview, ChatIntentUnknown, "":
		return plan
	}
	defaults := defaultToolsForIntent(plan.Intent)
	if len(defaults) == 0 {
		return plan
	}
	plan.ToolCalls = defaults
	plan.Assumptions = append(append([]string(nil), plan.Assumptions...), "default cabinet tools injected for empty planner plan")
	return plan
}

func defaultToolsForIntent(intent ChatIntent) []ToolCall {
	switch intent {
	case ChatIntentSales:
		return []ToolCall{
			{Name: ToolGetDashboardSummary, Args: map[string]any{}},
			{Name: ToolGetSKUMetrics, Args: map[string]any{"limit": 20, "sort_by": "revenue"}},
		}
	case ChatIntentStock:
		return []ToolCall{
			{Name: ToolGetStockRisks, Args: map[string]any{"limit": 10}},
			{Name: ToolGetCriticalSKUs, Args: map[string]any{"limit": 10}},
		}
	case ChatIntentAdvertising, ChatIntentAdLoss, ChatIntentUnsafeAds:
		return []ToolCall{
			{Name: ToolGetAdvertisingAnalytics, Args: map[string]any{"limit": 10}},
		}
	case ChatIntentAlerts:
		return []ToolCall{
			{Name: ToolGetOpenAlerts, Args: map[string]any{"limit": 10}},
		}
	case ChatIntentPriorities, ChatIntentRecommendations, ChatIntentExplainRecommendation:
		return []ToolCall{
			{Name: ToolGetOpenRecommendations, Args: map[string]any{"limit": 5}},
			{Name: ToolGetDashboardSummary, Args: map[string]any{}},
		}
	case ChatIntentPricing:
		return []ToolCall{
			{Name: ToolGetPriceEconomicsRisks, Args: map[string]any{"limit": 10}},
		}
	case ChatIntentABCAnalysis:
		return []ToolCall{
			{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "revenue", "limit": 20}},
		}
	default:
		return []ToolCall{
			{Name: ToolGetDashboardSummary, Args: map[string]any{}},
		}
	}
}

func trimDuplicateToolCalls(calls []ToolCall, maxSame int) []ToolCall {
	if maxSame < 1 {
		maxSame = 1
	}
	seen := map[string]int{}
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		if seen[call.Name] >= maxSame {
			continue
		}
		seen[call.Name]++
		out = append(out, call)
	}
	return out
}

func filterValidToolCalls(validator *ToolPlanValidator, plan ToolPlan) []ToolCall {
	if validator == nil {
		return nil
	}
	out := make([]ToolCall, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		probe := plan
		probe.ToolCalls = []ToolCall{call}
		if _, err := validator.Validate(&probe); err == nil {
			out = append(out, call)
		}
	}
	return out
}

func canFallbackToActionContextOnly(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"too many tools requested",
		"duplicate tool call",
		"non-unsupported intent requires at least one tool call",
		"tool plan exceeds context size limit",
		"unknown tool",
		"is not supported for intent",
		"required",
		"forbidden tool arg",
	}
	for _, needle := range needles {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
