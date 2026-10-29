package dashboardhome

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// DashboardAIFirstAuditViolation documents a forbidden or missing source in AI-only ready dashboard.
type DashboardAIFirstAuditViolation struct {
	Field  string `json:"field"`
	Source string `json:"source,omitempty"`
	Reason string `json:"reason"`
	Level  string `json:"level"` // critical | warning
}

// DashboardAIFirstAuditReport is output of audit-dashboard-ai-first.
type DashboardAIFirstAuditReport struct {
	OverallStatus string                            `json:"overall_status"`
	Blocks        map[string]map[string]interface{} `json:"blocks,omitempty"`
	SourceMap     map[string]string                 `json:"source_map,omitempty"`
	Violations    []DashboardAIFirstAuditViolation  `json:"violations"`
}

var forbiddenAIFirstSources = map[string]string{
	"legacy":                       "legacy seller-facing content in ready AI-only dashboard",
	"fallback":                     "hidden fallback content",
	"stale":                        "stale narrative substitution",
	"risk_driver_fallback":         "risk driver fallback recommendations",
	"deterministic_summary":        "deterministic analytical summary",
	"deterministic_recommendation": "deterministic recommendations",
	"ai_summary_legacy":            "legacy ai_summary path",
	"completed_partial_as_success": "partial AI treated as success",
	"hidden_substitution":          "hidden content substitution",
	"generic_placeholder":          "generic placeholder text",
	"backend_business_conclusion":  "backend-generated business conclusion",
	"substituted_by_deterministic": "deterministic substitution",
	"deterministic_substitution":   "deterministic substitution",
	"substitution":                 "block substitution",
}

// RunDashboardAIFirstAudit validates field-level sources for AI-first dashboard contract.
func RunDashboardAIFirstAudit(resp *Response) DashboardAIFirstAuditReport {
	report := DashboardAIFirstAuditReport{
		OverallStatus: "passed",
		Blocks:        map[string]map[string]interface{}{},
		Violations:    []DashboardAIFirstAuditViolation{},
	}
	if resp == nil {
		report.OverallStatus = "failed"
		report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
			Field: "response", Level: "critical", Reason: "nil dashboard response",
		})
		return report
	}
	report.SourceMap = buildDashboardNarrativeSourceMap(resp)
	if len(report.SourceMap) == 0 {
		report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
			Field: "source_map", Level: "critical", Reason: "source_map missing",
		})
	}
	report.Blocks["seller_pulse"] = map[string]interface{}{
		"kpis":              map[string]string{"source": "backend_factual_metrics", "allowed": "true"},
		"narrative.summary": map[string]string{"source": report.SourceMap["seller_pulse.narrative.summary"], "allowed": allowedAnalyticalSource(report.SourceMap["seller_pulse.narrative.summary"])},
		"important_notes":   map[string]string{"source": "ai_risk_drivers", "allowed": "true"},
		"quick_actions":     map[string]string{"source": "ai_top_actions", "allowed": "true"},
	}
	readyAI := resp.Status == StatusReady && resp.AIDiagnostics.AccountSummary.NarrativeSource == "ai"
	for field, src := range report.SourceMap {
		if reason, forbidden := forbiddenAIFirstSources[src]; forbidden && readyAI {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: field, Source: src, Level: "critical", Reason: reason,
			})
		}
	}
	appendSemanticAuditViolations(resp, readyAI, &report)
	if len(report.Violations) > 0 {
		for _, v := range report.Violations {
			if v.Level == "critical" {
				report.OverallStatus = "failed"
				break
			}
		}
	}
	return report
}

func allowedAnalyticalSource(src string) string {
	switch src {
	case "ai", "ai_repaired_technical", "ai_repaired_from_grounded_facts", "ai_top_actions", "ai_risk_drivers":
		return "true"
	case "backend_factual_metrics", "backend_factual_formatting", "backend_factual_score", "backend_factual_list", "backend_factual_comparison", "backend_score_from_ai_drivers", "backend_ui_layout", "debug_only", "missing":
		return "true"
	default:
		if _, forbidden := forbiddenAIFirstSources[src]; forbidden {
			return "false"
		}
		return "true"
	}
}

func appendSemanticAuditViolations(resp *Response, readyAI bool, report *DashboardAIFirstAuditReport) {
	for i, action := range resp.AIDiagnostics.TopActions {
		if !actionCategoryConsistentWithText(action) {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: fmt.Sprintf("top_actions[%d]", i), Level: "critical",
				Reason: fmt.Sprintf("action category %q inconsistent with title/reason semantics", action.Category),
			})
		}
		if readyAI && (action.Source == "fallback" || action.Source == "legacy" || action.Source == "risk_driver") {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: fmt.Sprintf("top_actions[%d].source", i), Source: action.Source, Level: "critical",
				Reason: "forbidden top_action source in ready AI-only dashboard",
			})
		}
	}
	if n := resp.SellerPulse.Narrative; n != nil {
		topByLabel := map[string]TopActionItem{}
		for _, top := range resp.AIDiagnostics.TopActions {
			topByLabel[strings.ToLower(strings.TrimSpace(top.Title))] = top
		}
		for i, qa := range n.QuickActions {
			top, ok := topByLabel[strings.ToLower(strings.TrimSpace(qa.Label))]
			if !ok {
				continue
			}
			if !quickActionKindMatchesCategory(qa.Kind, top.Category) {
				report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
					Field: fmt.Sprintf("seller_pulse.quick_actions[%d]", i), Level: "critical",
					Reason: fmt.Sprintf("quick_action kind %q does not match top_action category %q", qa.Kind, top.Category),
				})
			}
		}
	}
	wc := resp.AIDiagnostics.WhatChanged
	if wc.Status == WidgetAvailable {
		for gi, group := range wc.Groups {
			labels := map[string]int{}
			for ii, item := range group.Items {
				lk := strings.ToLower(strings.TrimSpace(item.Label))
				if lk != "" {
					labels[lk]++
					if labels[lk] > 1 {
						report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
							Field: fmt.Sprintf("what_changed.groups[%d].items[%d]", gi, ii), Level: "warning",
							Reason: "duplicate label in what_changed group: " + item.Label,
						})
					}
				}
				if actionMalformedWhatChangedStart.MatchString(item.Text) || actionGenericSignalPattern.MatchString(item.Text) {
					report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
						Field: fmt.Sprintf("what_changed.groups[%d].items[%d]", gi, ii), Level: "critical",
						Reason: "malformed or generic what_changed item text",
					})
				}
			}
		}
		if isWeakWhatChangedMainConclusion(firstNonEmpty(wc.MainConclusion, wc.Summary)) {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: "what_changed.main_conclusion", Level: "warning",
				Reason: "main_conclusion is generic or weak",
			})
		}
	}
	if funnelHasContradictoryComparison(resp.AIDiagnostics.FunnelAnalysis) {
		report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
			Field: "funnel_analysis", Level: "critical",
			Reason: "primary_drop_message contradicts funnel ai_analysis comparison availability",
		})
	}
	if resp.SKUFocus.NoSalesWithStockTotalCount > 0 {
		n := resp.AIDiagnostics.StockCoverage.Narrative
		if n == nil || n.NoSalesWithStock.Status != "available" {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: "stock_coverage.no_sales_with_stock", Level: "critical",
				Reason: "no_sales_with_stock unavailable while sku_focus has data",
			})
		}
	}
	for _, blob := range collectSellerFacingTextBlobs(resp) {
		if containsInternalFieldReference(blob) {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: "seller_facing_text", Level: "critical",
				Reason: "user-facing text contains internal API field reference",
			})
			break
		}
	}
	for _, driver := range resp.AIDiagnostics.AccountSummary.RiskDrivers {
		if driver.ID == driverSlowTurnover && driver.CategoryLabel == "Кабинет" {
			report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
				Field: "account_summary.risk_drivers", Level: "critical",
				Reason: "driver-slow-turnover has invalid category_label",
			})
		}
	}
	if len(resp.SKUFocus.Surplus) > 0 && len(resp.SKUFocus.SlowTurnover) > 0 {
		report.Violations = append(report.Violations, DashboardAIFirstAuditViolation{
			Field: "sku_focus.surplus", Level: "warning",
			Reason: "surplus and slow_turnover both populated in seller-facing response",
		})
	}
}

func funnelHasContradictoryComparison(analysis FunnelAnalysisWidget) bool {
	dropMsg := strings.TrimSpace(analysis.PrimaryDropMessage)
	if dropMsg == "" || analysis.AIAnalysis.Status != WidgetAvailable {
		return false
	}
	hasFactualDrop := funnelHasDeltaPattern.MatchString(dropMsg)
	aiSaysNoData := funnelNoComparisonDataPattern.MatchString(analysis.AIAnalysis.ShortSummary) ||
		funnelNoComparisonDataPattern.MatchString(analysis.AIAnalysis.Summary)
	return hasFactualDrop && aiSaysNoData
}

func collectSellerFacingTextBlobs(resp *Response) []string {
	out := []string{resp.AIDiagnostics.StockCoverage.Message}
	if n := resp.AIDiagnostics.StockCoverage.Narrative; n != nil {
		out = append(out, n.Headline, n.Summary, n.NextStep, n.NoSalesWithStock.Summary)
	}
	for _, a := range resp.AIDiagnostics.TopActions {
		out = append(out, a.Title, a.Reason, a.NextStep)
	}
	return out
}

func WriteDashboardAIFirstAuditReport(path string, report DashboardAIFirstAuditReport) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
