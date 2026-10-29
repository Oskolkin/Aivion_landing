package aisummaries

import (
	"encoding/json"
	"fmt"
)

type accountContextTrimResult struct {
	Context             map[string]any
	Trimmed             bool
	TrimTier            int
	OriginalTokens      int
	FinalTokens         int
	DroppedSections     []string
	SKUCandidatesBefore int
	SKUCandidatesAfter  int
}

func estimateApproxInputTokens(systemPrompt, userPrompt string, ctx map[string]any) int {
	raw, _ := json.Marshal(ctx)
	totalChars := len(systemPrompt) + len(userPrompt) + len(raw) + 512
	return (totalChars + 3) / 4
}

func trimAccountContextForTokenBudget(systemPrompt, userPrompt string, input map[string]any, limit int) accountContextTrimResult {
	if limit <= 0 {
		limit = 20000
	}
	ctx := deepCopyMap(input)
	orig := estimateApproxInputTokens(systemPrompt, userPrompt, ctx)
	beforeCandidates := nestedSliceLen(ctx, "sku_candidates")
	res := accountContextTrimResult{
		Context:             ctx,
		OriginalTokens:      orig,
		FinalTokens:         orig,
		SKUCandidatesBefore: beforeCandidates,
		SKUCandidatesAfter:  beforeCandidates,
	}
	if orig <= limit {
		return res
	}
	res.Trimmed = true
	apply := func(tier int, fn func(m map[string]any) []string) bool {
		dropped := fn(ctx)
		if len(dropped) > 0 {
			res.TrimTier = tier
			res.DroppedSections = append(res.DroppedSections, dropped...)
		}
		res.FinalTokens = estimateApproxInputTokens(systemPrompt, userPrompt, ctx)
		res.SKUCandidatesAfter = nestedSliceLen(ctx, "sku_candidates")
		return res.FinalTokens <= limit
	}
	if apply(2, trimTier2) {
		return res
	}
	if apply(3, trimTier3) {
		return res
	}
	if apply(4, trimTier4) {
		return res
	}
	apply(5, trimTier5)
	return res
}

func trimTier2(ctx map[string]any) []string {
	dropped := []string{}
	if clampNestedSlice(ctx, "sku_candidates", 70) {
		dropped = append(dropped, "sku_candidates_reduced_70")
	}
	if clampNestedSlice(ctx, "open_alerts_sample", 12) {
		dropped = append(dropped, "open_alerts_sample_reduced_12")
	}
	if clampNestedSlice(ctx, "open_recommendations_sample", 12) {
		dropped = append(dropped, "open_recommendations_sample_reduced_12")
	}
	return dropped
}

func trimTier3(ctx map[string]any) []string {
	dropped := []string{}
	if clampNestedSlice(ctx, "sku_candidates", 45) {
		dropped = append(dropped, "sku_candidates_reduced_45")
	}
	if clampNestedSlice(ctx, "category_rollup_top", 10) {
		dropped = append(dropped, "category_rollup_top_reduced_10")
	}
	if clampNestedSlice(ctx, "ad_campaigns_today", 6) {
		dropped = append(dropped, "ad_campaigns_today_reduced_6")
	}
	if clampNestedSlice(ctx, "ad_metrics_today", 8) {
		dropped = append(dropped, "ad_metrics_today_reduced_8")
	}
	return dropped
}

func trimTier4(ctx map[string]any) []string {
	dropped := []string{}
	if _, ok := ctx["dashboard"]; ok {
		delete(ctx, "dashboard")
		dropped = append(dropped, "dashboard_dropped")
	}
	if _, ok := ctx["account_metrics_last_7d"]; ok {
		delete(ctx, "account_metrics_last_7d")
		dropped = append(dropped, "account_metrics_last_7d_dropped")
	}
	if _, ok := ctx["ad_campaigns_today"]; ok {
		delete(ctx, "ad_campaigns_today")
		dropped = append(dropped, "ad_campaigns_today_dropped")
	}
	if _, ok := ctx["ad_metrics_today"]; ok {
		delete(ctx, "ad_metrics_today")
		dropped = append(dropped, "ad_metrics_today_dropped")
	}
	if clampNestedSlice(ctx, "sku_candidates", 25) {
		dropped = append(dropped, "sku_candidates_reduced_25")
	}
	return dropped
}

func trimTier5(ctx map[string]any) []string {
	minimal := map[string]any{
		"metric_date":              ctx["metric_date"],
		"data_quality_notes":       []string{"token_budget_minimal_context_applied"},
		"sku_coverage":             ctx["sku_coverage"],
		"account_aggregates":       ctx["account_aggregates"],
		"category_rollup_top":      firstNSliceAny(ctx["category_rollup_top"], 5),
		"other_categories_summary": ctx["other_categories_summary"],
		"funnel_context": map[string]any{
			"current_funnel":        nestedMapField(ctx, "funnel_context", "current_funnel"),
			"comparison_buckets":    nestedMapField(ctx, "funnel_context", "comparison_buckets"),
			"related_risks_context": nestedMapField(ctx, "funnel_context", "related_risks_context"),
		},
		"sku_candidates": firstNSliceAny(ctx["sku_candidates"], 12),
	}
	for k := range ctx {
		delete(ctx, k)
	}
	for k, v := range minimal {
		ctx[k] = v
	}
	return []string{"minimal_context_rebuild"}
}

func nestedMapField(root map[string]any, k1, k2 string) any {
	m, ok := root[k1].(map[string]any)
	if !ok {
		return nil
	}
	return m[k2]
}

func nestedSliceLen(root map[string]any, key string) int {
	if arr, ok := root[key].([]any); ok {
		return len(arr)
	}
	if arr, ok := root[key].([]map[string]any); ok {
		return len(arr)
	}
	return 0
}

func firstNSliceAny(v any, n int) any {
	switch arr := v.(type) {
	case []any:
		if len(arr) <= n {
			return arr
		}
		return arr[:n]
	case []map[string]any:
		if len(arr) <= n {
			return arr
		}
		return arr[:n]
	default:
		return v
	}
}

func clampNestedSlice(root map[string]any, key string, n int) bool {
	switch arr := root[key].(type) {
	case []any:
		if len(arr) <= n {
			return false
		}
		root[key] = arr[:n]
		return true
	case []map[string]any:
		if len(arr) <= n {
			return false
		}
		root[key] = arr[:n]
		return true
	default:
		return false
	}
}

func deepCopyMap(in map[string]any) map[string]any {
	raw, _ := json.Marshal(in)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{"copy_error": fmt.Sprintf("%v", err)}
	}
	return out
}
