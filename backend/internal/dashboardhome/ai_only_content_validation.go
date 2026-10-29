package dashboardhome

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidateDashboardAIOnlyContent checks seller-facing content consistency for AI-only dashboard.
func validateAIOnlyContentBlocks(payload *dashboardNarrativePayload) []string {
	issues := ValidateDashboardAIOnlyContent(payload, DashboardSynthesisInput{}, nil)
	errs := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue.Severity == "failed" {
			errs = append(errs, issue.Code+": "+issue.Message)
		}
	}
	return errs
}

func ValidateDashboardAIOnlyContent(
	payload *dashboardNarrativePayload,
	input DashboardSynthesisInput,
	resp *Response,
) []DashboardHomeConsistencyIssue {
	issues := make([]DashboardHomeConsistencyIssue, 0, 12)
	issues = append(issues, validateAIOnlyFunnelStructure(payload)...)
	issues = append(issues, validateAIOnlyCategorySummaries(payload, input)...)
	issues = append(issues, validateAIOnlyRiskOverviewGroups(payload)...)
	issues = append(issues, validateAIOnlyTopActionCategories(payload)...)
	issues = append(issues, validateAIOnlyWhatChangedSource(resp)...)
	if resp != nil {
		issues = append(issues, validateAIOnlyOrdersSemantics(resp, input)...)
		issues = append(issues, validateAIOnlySellerPulseRisks(resp)...)
		issues = append(issues, validateAIOnlyDRRLabels(resp)...)
		issues = append(issues, validateAIOnlySurplusLabels(resp)...)
		issues = append(issues, validateAIOnlyRevenueAtRiskNote(resp)...)
	}
	issues = append(issues, validateAIOnlyV611Content(payload, input)...)
	return issues
}

func validateAIOnlyFunnelStructure(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload == nil || payload.FunnelAIAnalysis == nil {
		return nil
	}
	ai := payload.FunnelAIAnalysis
	if ai.Status != WidgetAvailable {
		return nil
	}
	empty := strings.TrimSpace(ai.WeakestStage.Key) == "" &&
		strings.TrimSpace(ai.MainConclusion) == "" &&
		len(ai.ActionItems) == 0 &&
		len(ai.Comparisons.StageMetrics) == 0 &&
		len(ai.Comparisons.ConversionMetrics) == 0
	if !empty {
		return nil
	}
	return []DashboardHomeConsistencyIssue{{
		Code:     "funnel_ai_analysis_structurally_empty",
		Severity: "failed",
		Message:  "funnel_ai_analysis is available but structurally empty",
	}}
}

func validateAIOnlyCategorySummaries(payload *dashboardNarrativePayload, input DashboardSynthesisInput) []DashboardHomeConsistencyIssue {
	if payload == nil || len(payload.CategorySummaries) == 0 {
		return nil
	}
	catalog := map[string]string{}
	for _, row := range input.CategoryCatalog {
		if row.CategoryName != "" {
			catalog[row.CategoryID] = row.CategoryName
			catalog[row.EntityID] = row.CategoryName
		}
	}
	issues := make([]DashboardHomeConsistencyIssue, 0)
	for i, card := range payload.CategorySummaries {
		if categoryIDNamePattern.MatchString(strings.TrimSpace(card.CategoryName)) {
			id := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(card.CategoryName, "Категория "), "категория "))
			if _, ok := catalog[id]; ok {
				issues = append(issues, DashboardHomeConsistencyIssue{
					Code:     "category_summary_raw_id",
					Severity: "failed",
					Message:  fmt.Sprintf("category_summaries[%d] uses raw id label while catalog has name", i),
				})
			}
		}
		if strings.TrimSpace(card.KeyRisk) == "" || strings.TrimSpace(card.RecommendedAction) == "" {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "category_summary_incomplete",
				Severity: "failed",
				Message:  fmt.Sprintf("category_summaries[%d] missing key_risk or recommended_action", i),
			})
		}
	}
	return issues
}

func validateAIOnlyRiskOverviewGroups(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload == nil || payload.RiskOverview == nil || payload.RiskOverview.Status != WidgetAvailable {
		return nil
	}
	for i, g := range payload.RiskOverview.Groups {
		if strings.TrimSpace(g.Group) == "" || strings.TrimSpace(g.Severity) == "" {
			return []DashboardHomeConsistencyIssue{{
				Code:     "risk_overview_empty_group_severity",
				Severity: "failed",
				Message:  fmt.Sprintf("risk_overview.groups[%d] missing group or severity", i),
			}}
		}
	}
	return nil
}

func validateAIOnlyTopActionCategories(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload == nil {
		return nil
	}
	for i, action := range payload.TopActions {
		if topActionLooksStock(action.Title, action.Reason, action.NextStep) &&
			(action.Category == "sales" || action.DriverID == driverAccountSales) {
			return []DashboardHomeConsistencyIssue{{
				Code:     "top_action_stock_classified_as_sales",
				Severity: "failed",
				Message:  fmt.Sprintf("top_actions[%d] stock-like action classified as sales", i),
			}}
		}
	}
	return nil
}

func validateAIOnlyWhatChangedSource(resp *Response) []DashboardHomeConsistencyIssue {
	if resp == nil {
		return nil
	}
	src := blockApplySource(resp, "what_changed")
	if src == "substituted_by_deterministic" || src == "deterministic_substitution" {
		return []DashboardHomeConsistencyIssue{{
			Code: "what_changed_deterministic_substitution", Severity: "failed",
			Message: "AI-only what_changed was substituted by deterministic fallback",
		}}
	}
	if src == "missing" || resp.AIDiagnostics.WhatChanged.Status == WidgetSourceNotAvailable {
		return []DashboardHomeConsistencyIssue{{
			Code: "what_changed_not_applied", Severity: "failed",
			Message: "AI-only what_changed was not applied after validation",
		}}
	}
	return nil
}

func validateAIOnlyOrdersSemantics(resp *Response, input DashboardSynthesisInput) []DashboardHomeConsistencyIssue {
	daily, hasDaily := orderSemanticsInt(input.OrderSemantics, "daily_account_orders_count")
	if !hasDaily {
		return nil
	}
	var pulseOrders int64
	for _, kpi := range resp.SellerPulse.KPIs {
		if kpi.Key == "orders" {
			pulseOrders = parseIntLoose(kpi.Value)
		}
	}
	if pulseOrders > 0 && int64(pulseOrders) != daily {
		return []DashboardHomeConsistencyIssue{{
			Code: "orders_semantics_mismatch", Severity: "failed",
			Message: fmt.Sprintf("seller_pulse orders=%d differs from daily_account_orders=%d", pulseOrders, daily),
		}}
	}
	for _, driver := range resp.AIDiagnostics.AccountSummary.RiskDrivers {
		if driver.ID == "driver-cancellations" || driver.ID == "driver-sales-revenue" || driver.ID == driverAccountSales {
			if v, ok := evidenceInt(driver.Evidence, "orders_today"); ok && int64(v) != daily {
				return []DashboardHomeConsistencyIssue{{
					Code: "orders_semantics_mismatch", Severity: "failed",
					Message: fmt.Sprintf("%s orders_today=%d differs from daily_account_orders=%d", driver.ID, v, daily),
				}}
			}
		}
	}
	return nil
}

func validateAIOnlySellerPulseRisks(resp *Response) []DashboardHomeConsistencyIssue {
	var risksKPI int64
	for _, kpi := range resp.SellerPulse.KPIs {
		if kpi.Key == "risks" {
			risksKPI = parseIntLoose(kpi.Value)
		}
	}
	critHigh := countCriticalHighRiskDrivers(resp.AIDiagnostics.AccountSummary.RiskDrivers)
	if critHigh > 0 && risksKPI != critHigh {
		return []DashboardHomeConsistencyIssue{{
			Code: "seller_pulse_risks_kpi_mismatch", Severity: "failed",
			Message: fmt.Sprintf("seller_pulse risks=%d want %d from AI risk_drivers", risksKPI, critHigh),
		}}
	}
	return nil
}

var accountDRRLabelPattern = regexp.MustCompile(`(?i)кабинет|account|общей выручк`)

func validateAIOnlyDRRLabels(resp *Response) []DashboardHomeConsistencyIssue {
	hasAccount := false
	hasAd := resp.AIDiagnostics.AdEfficiency.Summary != nil && resp.AIDiagnostics.AdEfficiency.Summary.CurrentDRRPct != nil
	for _, kpi := range resp.SellerPulse.KPIs {
		if kpi.Key == "drr" {
			hasAccount = true
			if !accountDRRLabelPattern.MatchString(kpi.Label) {
				return []DashboardHomeConsistencyIssue{{
					Code: "drr_semantics_unlabeled", Severity: "warning",
					Message: "seller_pulse DRR KPI should be labeled as account/cabinet DRR",
				}}
			}
		}
	}
	if hasAccount && hasAd {
		return nil
	}
	return nil
}

func validateAIOnlySurplusLabels(resp *Response) []DashboardHomeConsistencyIssue {
	slow := len(resp.SKUFocus.Surplus)
	var ozon int
	for _, seg := range resp.AIDiagnostics.StockCoverage.Segments {
		if seg.Key == "surplus" {
			ozon = seg.SKUCount
		}
	}
	if slow > 0 && ozon == 0 {
		msg := strings.ToLower(resp.AIDiagnostics.StockCoverage.Message)
		if !strings.Contains(msg, "медленн") && !strings.Contains(msg, "ozon") {
			return []DashboardHomeConsistencyIssue{{
				Code: "surplus_slow_turnover_confusion", Severity: "warning",
				Message: "slow turnover count differs from Ozon SURPLUS segment without explanatory label",
			}}
		}
	}
	return nil
}

func validateAIOnlyRevenueAtRiskNote(resp *Response) []DashboardHomeConsistencyIssue {
	n := resp.AIDiagnostics.StockCoverage.Narrative
	if n == nil || n.RevenueAtRisk.Status != "available" {
		return nil
	}
	if strings.TrimSpace(n.RevenueAtRisk.MethodNote) == "" {
		return []DashboardHomeConsistencyIssue{{
			Code: "revenue_at_risk_method_unclear", Severity: "warning",
			Message: "stock_coverage revenue_at_risk missing method_note",
		}}
	}
	return nil
}

func orderSemanticsInt(sem map[string]any, key string) (int64, bool) {
	if sem == nil {
		return 0, false
	}
	return intFromAny(sem[key])
}

func enforceAIOnlyContentConsistency(resp *Response, payload *dashboardNarrativePayload, input DashboardSynthesisInput) {
	if !dashboardAIOnlyEnabled() || resp == nil {
		return
	}
	issues := ValidateDashboardAIOnlyContent(payload, input, resp)
	issues = append(issues, ValidateDashboardHomeResponseConsistency(resp)...)
	if resp.DashboardNarrativeDebug != nil {
		resp.DashboardNarrativeDebug.ConsistencyIssues = issues
		resp.DashboardNarrativeDebug.ConsistencyStatus = consistencyStatusFromIssues(issues)
	}
	for _, issue := range issues {
		if issue.Severity == "failed" {
			resp.Status = StatusAIGenerationRequired
			resp.AIDiagnostics.AccountSummary.ValidationStatus = "failed_content_consistency"
			return
		}
	}
}
