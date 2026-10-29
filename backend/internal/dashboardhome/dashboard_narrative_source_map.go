package dashboardhome

import "fmt"

// buildDashboardNarrativeSourceMap maps dashboard fields to their content source for AI-first audit.
func buildDashboardNarrativeSourceMap(resp *Response) map[string]string {
	if resp == nil {
		return nil
	}
	m := map[string]string{
		"seller_pulse.kpis":                       "backend_factual_metrics",
		"seller_pulse.narrative.summary":          sourceWithApplyRepairs("ai_block:seller_pulse_narrative", resp, "seller_pulse."),
		"seller_pulse.narrative.quick_actions":    sourceWithApplyRepairs("ai_block:seller_pulse_narrative", resp, "seller_pulse.quick_actions"),
		"seller_pulse.narrative.important_notes":  sourceWithApplyRepairs("ai_block:seller_pulse_narrative", resp, "seller_pulse."),
		"account_summary.risk_drivers":            sourceWithApplyRepairs("ai_block:account_health", resp, "account_health."),
		"account_summary.health_score":            "ai_block:account_health",
		"account_summary.health_score_breakdown":  "debug_only",
		"account_summary.headline":                sourceWithApplyRepairs("ai_block:account_health", resp, "account_health.", "headline", "executive_summary"),
		"account_summary.executive_summary":       sourceWithApplyRepairs("ai_block:account_health", resp, "account_health.", "executive_summary"),
		"what_changed":                            sourceWithApplyRepairs("ai_block:what_changed", resp, "what_changed."),
		"top_actions":                             "ai_block:top_actions",
		"recommended_actions":                     "deprecated_dashboard_block",
		"funnel_analysis.stages":                  "backend_factual_metrics",
		"funnel_analysis.primary_drop_message":    "deprecated_backend_factual_comparison",
		"funnel_analysis.ai_analysis":             sourceWithApplyRepairs("ai_block:funnel_ai_analysis", resp, "funnel_"),
		"stock_coverage.segments":                 "backend_factual_metrics",
		"stock_coverage.stock_coverage_narrative": "ai_block:stock_coverage_narrative",
		"ad_efficiency.summary":                   "backend_factual_metrics",
		"ad_efficiency.ad_efficiency_narrative":   sourceWithApplyRepairs("ai_block:ad_efficiency_narrative", resp, "ad_efficiency"),
		"category_summaries.score":                "ai_block:category_summaries",
		"category_summaries.text":                 sourceWithApplyRepairs("ai_block:category_summaries", resp, "category_summary_"),
		"category_summaries.selection_policy":     CategorySummariesSelectionPolicyAllCatalog,
		"risk_distribution":                       riskDistributionSource(resp),
		"sku_focus.lists":                         "backend_factual_list",
	}
	if n := resp.SellerPulse.Narrative; n != nil && n.DisplayHeadline {
		m["seller_pulse.narrative.headline"] = "backend_ui_layout"
	} else {
		m["seller_pulse.narrative.headline"] = "debug_only"
	}
	if len(narrativeApplyRepairs(resp)) > 0 {
		m["apply_repairs"] = fmt.Sprintf("%d", len(narrativeApplyRepairs(resp)))
	}
	return m
}

func riskDistributionSource(resp *Response) string {
	if resp.AIDiagnostics.RiskDistribution.Status == WidgetAvailable && resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai" {
		return "ai"
	}
	return "missing"
}
