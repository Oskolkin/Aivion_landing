package dashboardhome

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// DashboardContentForensicIssue documents a single content consistency problem across pipeline layers.
type DashboardContentForensicIssue struct {
	ID                  string         `json:"id"`
	Severity            string         `json:"severity"`
	Symptom             string         `json:"symptom"`
	DataPackValue       map[string]any `json:"data_pack_value,omitempty"`
	SynthesisInputValue map[string]any `json:"synthesis_input_value,omitempty"`
	RawAIValue          map[string]any `json:"raw_ai_value,omitempty"`
	ParsedPayloadValue  map[string]any `json:"parsed_payload_value,omitempty"`
	DashboardHomeValue  map[string]any `json:"dashboard_home_value,omitempty"`
	RootCauseLayer      string         `json:"root_cause_layer"`
	ResponsibleFiles    []string       `json:"responsible_files"`
	FixRequired         bool           `json:"fix_required"`
	FixPlan             string         `json:"fix_plan"`
}

// DashboardContentForensicsReport is the output of inspect-dashboard-ai-only-run.
type DashboardContentForensicsReport struct {
	MetricDate        string                             `json:"metric_date"`
	RunID             string                             `json:"run_id"`
	Summary           DashboardContentForensicsSummary   `json:"summary"`
	PipelineArtifacts DashboardContentForensicsArtifacts `json:"pipeline_artifacts"`
	Issues            []DashboardContentForensicIssue    `json:"issues"`
}

type DashboardContentForensicsSummary struct {
	OverallStatus  string `json:"overall_status"`
	IssuesTotal    int    `json:"issues_total"`
	CriticalIssues int    `json:"critical_issues"`
	WarningIssues  int    `json:"warning_issues"`
}

type DashboardContentForensicsArtifacts struct {
	DataPackLoaded        bool `json:"data_pack_loaded"`
	SynthesisInputLoaded  bool `json:"synthesis_input_loaded"`
	RawAILoaded           bool `json:"raw_ai_loaded"`
	ParsedAILoaded        bool `json:"parsed_ai_loaded"`
	StoredNarrativeLoaded bool `json:"stored_narrative_loaded"`
	DashboardHomeLoaded   bool `json:"dashboard_home_loaded"`
}

// DashboardContentForensicsInput bundles artifacts for forensic analysis.
type DashboardContentForensicsInput struct {
	MetricDate     string
	RunID          string
	DataPack       *aidatapack.SellerAIDataPack
	SynthesisInput *DashboardSynthesisInput
	RawAIResponse  string
	ParsedPayload  *dashboardNarrativePayload
	StoredPayload  *dashboardNarrativePayload
	DashboardHome  *Response
}

var categoryIDNamePattern = regexp.MustCompile(`(?i)^категория\s+\d+$`)

func RunDashboardContentForensics(in DashboardContentForensicsInput) DashboardContentForensicsReport {
	report := DashboardContentForensicsReport{
		MetricDate: in.MetricDate,
		RunID:      in.RunID,
		PipelineArtifacts: DashboardContentForensicsArtifacts{
			DataPackLoaded:        in.DataPack != nil,
			SynthesisInputLoaded:  in.SynthesisInput != nil,
			RawAILoaded:           strings.TrimSpace(in.RawAIResponse) != "",
			ParsedAILoaded:        in.ParsedPayload != nil,
			StoredNarrativeLoaded: in.StoredPayload != nil,
			DashboardHomeLoaded:   in.DashboardHome != nil,
		},
	}
	issues := make([]DashboardContentForensicIssue, 0, 16)
	issues = append(issues, forensicOrdersSemantics(in)...)
	issues = append(issues, forensicSellerPulseRisksKPI(in)...)
	issues = append(issues, forensicTopActionStockCategory(in)...)
	issues = append(issues, forensicWhatChangedSubstitution(in)...)
	issues = append(issues, forensicFunnelAIIncomplete(in)...)
	issues = append(issues, forensicCategorySummaryRawIDs(in)...)
	issues = append(issues, forensicRiskOverviewEmptyGroup(in)...)
	issues = append(issues, forensicDRRSemantics(in)...)
	issues = append(issues, forensicSurplusVsSlowTurnover(in)...)
	issues = append(issues, forensicRevenueAtRisk(in)...)
	issues = append(issues, forensicSynthesisInputMetricDate(in)...)
	issues = append(issues, forensicDebugAppliedBlocks(in)...)
	issues = append(issues, forensicDashboardUIContract(in)...)
	issues = append(issues, forensicHealthBreakdownDriverMismatch(in)...)
	issues = append(issues, forensicExcessiveSellerFacingFactIDs(in)...)
	issues = append(issues, forensicTopActionsGrounding(in)...)
	issues = append(issues, forensicCategoryCatalogCoverage(in)...)

	contentIssues := ValidateDashboardAIOnlyContent(in.ParsedPayload, synthesisInputOrEmpty(in.SynthesisInput), in.DashboardHome)
	for _, ci := range contentIssues {
		id := ci.Code
		if _, seen := indexForensicByID(issues, id); seen {
			continue
		}
		issues = append(issues, DashboardContentForensicIssue{
			ID:             id,
			Severity:       mapConsistencySeverity(ci.Severity),
			Symptom:        ci.Message,
			RootCauseLayer: rootCauseLayerForCode(ci.Code),
			FixRequired:    ci.Severity == "failed",
			FixPlan:        ci.Message,
		})
	}

	report.Issues = dedupeForensicIssues(issues)
	report.Summary = summarizeForensics(report.Issues)
	return report
}

func synthesisInputOrEmpty(in *DashboardSynthesisInput) DashboardSynthesisInput {
	if in != nil {
		return *in
	}
	return DashboardSynthesisInput{}
}

func mapConsistencySeverity(s string) string {
	if s == "failed" {
		return "critical"
	}
	return s
}

func rootCauseLayerForCode(code string) string {
	switch code {
	case "orders_semantics_mismatch", "cancellation_rate_denominator_mismatch":
		return "dashboard_apply_mapping"
	case "seller_pulse_risks_kpi_mismatch":
		return "deterministic_widget_builder"
	case "top_action_stock_classified_as_sales":
		return "raw_ai_response"
	case "what_changed_deterministic_substitution":
		return "dashboard_apply_mapping"
	case "funnel_ai_analysis_structurally_empty":
		return "synthesis_prompt"
	case "category_summary_raw_id":
		return "synthesis_input_builder"
	case "risk_overview_empty_group_severity":
		return "raw_ai_response"
	case "drr_semantics_unlabeled":
		return "deterministic_widget_builder"
	case "surplus_slow_turnover_confusion":
		return "dashboard_apply_mapping"
	case "revenue_at_risk_method_unclear":
		return "deterministic_widget_builder"
	case "synthesis_input_metric_date_empty":
		return "synthesis_input_builder"
	case "debug_applied_blocks_misreport":
		return "debug_misreporting"
	default:
		return "response_consistency_validator"
	}
}

func summarizeForensics(issues []DashboardContentForensicIssue) DashboardContentForensicsSummary {
	summary := DashboardContentForensicsSummary{
		OverallStatus: "passed_content_consistency",
		IssuesTotal:   len(issues),
	}
	for _, issue := range issues {
		switch issue.Severity {
		case "critical":
			summary.CriticalIssues++
		case "warning":
			summary.WarningIssues++
		}
	}
	if summary.CriticalIssues > 0 {
		summary.OverallStatus = "failed_content_consistency"
	} else if summary.WarningIssues > 0 {
		summary.OverallStatus = "warning_content_consistency"
	}
	return summary
}

func dedupeForensicIssues(issues []DashboardContentForensicIssue) []DashboardContentForensicIssue {
	seen := map[string]struct{}{}
	out := make([]DashboardContentForensicIssue, 0, len(issues))
	for _, issue := range issues {
		if _, ok := seen[issue.ID]; ok {
			continue
		}
		seen[issue.ID] = struct{}{}
		out = append(out, issue)
	}
	return out
}

func indexForensicByID(issues []DashboardContentForensicIssue, id string) (DashboardContentForensicIssue, bool) {
	for _, issue := range issues {
		if issue.ID == id {
			return issue, true
		}
	}
	return DashboardContentForensicIssue{}, false
}

func extractOrdersMetrics(in DashboardContentForensicsInput) map[string]any {
	metrics := map[string]any{}
	if in.DataPack != nil && in.DataPack.Tables.AccountRollup.DataStatus == "available" {
		if v, ok := mapNumber(in.DataPack.Tables.AccountRollup.CurrentDay["orders_count"]); ok {
			metrics["daily_account_orders"] = int64(v)
		}
	}
	if in.DashboardHome != nil {
		for _, kpi := range in.DashboardHome.SellerPulse.KPIs {
			if kpi.Key == "orders" {
				metrics["seller_pulse_orders"] = kpi.Value
			}
		}
		for _, driver := range in.DashboardHome.AIDiagnostics.AccountSummary.RiskDrivers {
			if driver.ID == "driver-sales-revenue" || driver.ID == driverAccountSales {
				if v, ok := evidenceInt(driver.Evidence, "orders_today"); ok {
					metrics["sales_driver_orders_today"] = v
				}
			}
			if driver.ID == "driver-cancellations" {
				if v, ok := evidenceInt(driver.Evidence, "orders_today"); ok {
					metrics["cancellations_denominator"] = v
				}
			}
		}
		if in.DashboardHome.AIDiagnostics.SalesFunnel.Stages != nil {
			for _, st := range in.DashboardHome.AIDiagnostics.SalesFunnel.Stages {
				if st.Key == "orders" && st.Value != nil {
					metrics["funnel_ordered_units_or_orders"] = *st.Value
				}
				if st.Key == "delivered_units" && st.Value != nil {
					metrics["funnel_completed_orders"] = *st.Value
				}
			}
		}
		var statusSum int64
		for _, driver := range in.DashboardHome.AIDiagnostics.AccountSummary.RiskDrivers {
			if driver.ID != "driver-cancellations" {
				continue
			}
			if raw, ok := driver.Evidence["orders_by_status"]; ok {
				if rows, ok := raw.([]any); ok {
					for _, row := range rows {
						if m, ok := row.(map[string]any); ok {
							if c, ok := mapNumber(m["orders_count"]); ok {
								statusSum += int64(c)
							}
						}
					}
				}
			}
		}
		if statusSum > 0 {
			metrics["orders_by_status_sum"] = statusSum
		}
	}
	if in.SynthesisInput != nil {
		if sem, ok := in.SynthesisInput.OrderSemantics["daily_account_orders_count"]; ok {
			metrics["synthesis_daily_account_orders"] = sem
		}
	}
	return metrics
}

func forensicOrdersSemantics(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	metrics := extractOrdersMetrics(in)
	daily, hasDaily := intFromAny(metrics["daily_account_orders"])
	pulseOrders := parseIntLoose(stringFromAny(metrics["seller_pulse_orders"]))
	salesOrders, hasSales := intFromAny(metrics["sales_driver_orders_today"])
	cancelDenom, hasCancel := intFromAny(metrics["cancellations_denominator"])

	conflict := false
	if hasDaily && pulseOrders > 0 && int64(pulseOrders) != daily {
		conflict = true
	}
	if hasDaily && hasSales && int64(salesOrders) != daily {
		conflict = true
	}
	if hasDaily && hasCancel && int64(cancelDenom) != daily {
		conflict = true
	}
	if !conflict {
		return nil
	}
	return []DashboardContentForensicIssue{{
		ID:       "orders_semantics_mismatch",
		Severity: "critical",
		Symptom: fmt.Sprintf(
			"conflicting order counts: daily_account=%v seller_pulse=%v sales_driver=%v cancellations=%v",
			metrics["daily_account_orders"], metrics["seller_pulse_orders"], metrics["sales_driver_orders_today"], metrics["cancellations_denominator"],
		),
		DataPackValue:       pickKeys(metrics, "daily_account_orders"),
		SynthesisInputValue: pickKeys(metrics, "synthesis_daily_account_orders"),
		DashboardHomeValue:  metrics,
		RootCauseLayer:      "dashboard_apply_mapping",
		ResponsibleFiles: []string{
			"backend/internal/dashboardhome/risk_drivers.go",
			"backend/internal/dashboardhome/service.go",
			"backend/internal/dashboardhome/narrative_datapack_synthesis_input.go",
		},
		FixRequired: true,
		FixPlan:     "Use Data Pack daily_account_orders_count as single seller-facing orders metric; label funnel/status metrics separately.",
	}}
}

func forensicSellerPulseRisksKPI(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	var risksKPI int64
	for _, kpi := range in.DashboardHome.SellerPulse.KPIs {
		if kpi.Key == "risks" {
			risksKPI = parseIntLoose(kpi.Value)
		}
	}
	critHigh := countCriticalHighRiskDrivers(in.DashboardHome.AIDiagnostics.AccountSummary.RiskDrivers)
	if critHigh == 0 || risksKPI == critHigh {
		return nil
	}
	return []DashboardContentForensicIssue{{
		ID:       "seller_pulse_risks_kpi_mismatch",
		Severity: "critical",
		Symptom:  fmt.Sprintf("seller_pulse risks KPI=%d but account_summary has %d critical/high AI risk drivers", risksKPI, critHigh),
		DashboardHomeValue: map[string]any{
			"seller_pulse_risks_kpi":        risksKPI,
			"ai_risk_drivers_critical_high": critHigh,
		},
		RootCauseLayer:   "deterministic_widget_builder",
		ResponsibleFiles: []string{"backend/internal/dashboardhome/service.go", "backend/internal/dashboardhome/narrative_apply_helpers.go"},
		FixRequired:      true,
		FixPlan:          "In AI-only mode count risks KPI from account_summary.risk_drivers critical/high, not alert summary.",
	}}
}

func countCriticalHighRiskDrivers(drivers []RiskDriver) int64 {
	var n int64
	for _, d := range drivers {
		switch strings.ToLower(d.Severity) {
		case "critical", "high":
			n++
		}
	}
	return n
}

func forensicTopActionStockCategory(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	check := func(payload *dashboardNarrativePayload, layer string) []DashboardContentForensicIssue {
		if payload == nil || len(payload.TopActions) == 0 {
			return nil
		}
		action := payload.TopActions[0]
		if !topActionLooksStock(action.Title, action.Reason, action.NextStep) {
			return nil
		}
		if action.Category == "stock" && action.DriverID == driverStock {
			return nil
		}
		return []DashboardContentForensicIssue{{
			ID:       "top_action_stock_classified_as_sales",
			Severity: "critical",
			Symptom:  fmt.Sprintf("stock-like top action has category=%s driver_id=%s title=%q", action.Category, action.DriverID, action.Title),
			ParsedPayloadValue: map[string]any{
				"driver_id": action.DriverID, "category": action.Category, "title": action.Title,
			},
			RootCauseLayer: layer,
			ResponsibleFiles: []string{
				"backend/internal/dashboardhome/narrative_datapack_synthesis_input.go",
				"backend/internal/dashboardhome/narrative_datapack_synthesis_prompt.go",
				"backend/internal/dashboardhome/ai_only_repair.go",
			},
			FixRequired: true,
			FixPlan:     "Provide stock driver/candidate hints from sku_rollups; repair stock-like actions to driver-stock.",
		}}
	}
	if issues := check(in.ParsedPayload, "raw_ai_response"); len(issues) > 0 {
		return issues
	}
	if in.DashboardHome != nil && len(in.DashboardHome.AIDiagnostics.TopActions) > 0 {
		action := in.DashboardHome.AIDiagnostics.TopActions[0]
		if topActionLooksStock(action.Title, action.Reason, action.NextStep) && action.Category != "stock" {
			return []DashboardContentForensicIssue{{
				ID:       "top_action_stock_classified_as_sales",
				Severity: "critical",
				Symptom:  fmt.Sprintf("dashboard top action category=%s driver_id=%s", action.Category, action.DriverID),
				DashboardHomeValue: map[string]any{
					"category": action.Category, "driver_id": action.DriverID, "title": action.Title,
				},
				RootCauseLayer:   "dashboard_apply_mapping",
				ResponsibleFiles: []string{"backend/internal/dashboardhome/ai_only_repair.go", "backend/internal/dashboardhome/narrative_apply.go"},
				FixRequired:      true,
				FixPlan:          "Reclassify stock-like top actions during AI-only repair/apply.",
			}}
		}
	}
	return nil
}

func topActionLooksStock(parts ...string) bool {
	text := strings.ToLower(strings.Join(parts, " "))
	keywords := []string{"налич", "остат", "пополн", "out-of-stock", "покрыти", "закончатся", "дефицит"}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func forensicWhatChangedSubstitution(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.ParsedPayload == nil || in.ParsedPayload.WhatChanged == nil || in.DashboardHome == nil {
		return nil
	}
	parsedSummary := strings.TrimSpace(in.ParsedPayload.WhatChanged.Summary)
	homeSummary := strings.TrimSpace(in.DashboardHome.AIDiagnostics.WhatChanged.Summary)
	if parsedSummary == "" || homeSummary == "" || parsedSummary == homeSummary {
		return nil
	}
	// Fallback what_changed often cites exact previous revenue numbers.
	if strings.Contains(homeSummary, "против") && strings.Contains(homeSummary, "прошлой сводке") &&
		!strings.Contains(parsedSummary, "против") {
		source := blockApplySource(in.DashboardHome, "what_changed")
		if source == "" {
			source = "unknown"
		}
		return []DashboardContentForensicIssue{{
			ID:                 "what_changed_deterministic_substitution",
			Severity:           "critical",
			Symptom:            "dashboard what_changed summary differs from parsed AI payload (likely deterministic fallback)",
			ParsedPayloadValue: map[string]any{"summary": parsedSummary},
			DashboardHomeValue: map[string]any{
				"summary": homeSummary, "applied_source": source,
			},
			RootCauseLayer:   "dashboard_apply_mapping",
			ResponsibleFiles: []string{"backend/internal/dashboardhome/what_changed_apply.go", "backend/internal/dashboardhome/response_consistency.go"},
			FixRequired:      true,
			FixPlan:          "Do not substitute deterministic what_changed in AI-only; track real block source in debug.",
		}}
	}
	return nil
}

func forensicFunnelAIIncomplete(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	check := func(ai FunnelAIAnalysisWidget, layer string, value map[string]any) []DashboardContentForensicIssue {
		if ai.Status != WidgetAvailable {
			return nil
		}
		empty := strings.TrimSpace(ai.WeakestStage.Key) == "" &&
			len(ai.Comparisons.StageMetrics) == 0 &&
			len(ai.Comparisons.ConversionMetrics) == 0 &&
			len(ai.ActionItems) == 0
		if !empty {
			return nil
		}
		return []DashboardContentForensicIssue{{
			ID:                 "funnel_ai_analysis_structurally_empty",
			Severity:           "critical",
			Symptom:            "funnel_ai_analysis.status=available but weakest_stage/comparisons/action_items are empty",
			ParsedPayloadValue: value,
			RootCauseLayer:     layer,
			ResponsibleFiles: []string{
				"backend/internal/dashboardhome/narrative_datapack_synthesis_prompt.go",
				"backend/internal/dashboardhome/narrative_acceptance.go",
			},
			FixRequired: true,
			FixPlan:     "Strengthen synthesis schema and reject structurally empty funnel_ai_analysis in AI-only.",
		}}
	}
	if in.ParsedPayload != nil && in.ParsedPayload.FunnelAIAnalysis != nil {
		if issues := check(*in.ParsedPayload.FunnelAIAnalysis, "raw_ai_response", map[string]any{
			"short_summary": in.ParsedPayload.FunnelAIAnalysis.ShortSummary,
		}); len(issues) > 0 {
			return issues
		}
	}
	if in.DashboardHome != nil {
		return check(in.DashboardHome.AIDiagnostics.FunnelAnalysis.AIAnalysis, "dashboard_apply_mapping", nil)
	}
	return nil
}

func forensicCategorySummaryRawIDs(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	names := map[string]string{}
	if in.DataPack != nil {
		for _, row := range in.DataPack.Tables.CategoryRollups {
			if row.Name != "" {
				names[row.CategoryID] = row.Name
			}
		}
	}
	checkCards := func(cards []CategorySummaryCard, layer string) []DashboardContentForensicIssue {
		for _, card := range cards {
			if categoryIDNamePattern.MatchString(strings.TrimSpace(card.CategoryName)) {
				id := strings.TrimPrefix(strings.TrimSpace(card.CategoryName), "Категория ")
				id = strings.TrimPrefix(id, "категория ")
				if _, ok := names[id]; ok {
					return []DashboardContentForensicIssue{{
						ID:       "category_summary_raw_id",
						Severity: "critical",
						Symptom:  fmt.Sprintf("category_summaries use raw id label %q while catalog has name %q", card.CategoryName, names[id]),
						DataPackValue: map[string]any{
							"category_id": id, "category_name": names[id],
						},
						ParsedPayloadValue: map[string]any{"category_name": card.CategoryName},
						RootCauseLayer:     layer,
						ResponsibleFiles: []string{
							"backend/internal/dashboardhome/narrative_datapack_synthesis_input.go",
							"backend/internal/dashboardhome/narrative_datapack_synthesis_prompt.go",
						},
						FixRequired: true,
						FixPlan:     "Include category_catalog in synthesis input and require category_name in schema.",
					}}
				}
			}
		}
		return nil
	}
	if in.ParsedPayload != nil {
		if issues := checkCards(in.ParsedPayload.CategorySummaries, "synthesis_prompt"); len(issues) > 0 {
			return issues
		}
	}
	if in.DashboardHome != nil {
		return checkCards(in.DashboardHome.AIDiagnostics.CategorySummaries, "dashboard_apply_mapping")
	}
	return nil
}

func forensicRiskOverviewEmptyGroup(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	check := func(overview *RiskDistributionWidget, layer string) []DashboardContentForensicIssue {
		if overview == nil || overview.Status != WidgetAvailable {
			return nil
		}
		for _, g := range overview.Groups {
			if strings.TrimSpace(g.Group) == "" || strings.TrimSpace(g.Severity) == "" {
				return []DashboardContentForensicIssue{{
					ID:       "risk_overview_empty_group_severity",
					Severity: "critical",
					Symptom:  fmt.Sprintf("risk_overview group %q has empty group/severity", g.Label),
					ParsedPayloadValue: map[string]any{
						"group": g.Group, "label": g.Label, "severity": g.Severity, "count": g.Count,
					},
					RootCauseLayer: layer,
					ResponsibleFiles: []string{
						"backend/internal/dashboardhome/narrative_datapack_synthesis_prompt.go",
						"backend/internal/dashboardhome/ai_only_repair.go",
					},
					FixRequired: true,
					FixPlan:     "Require group/severity in schema; repair from label when missing.",
				}}
			}
		}
		return nil
	}
	if in.ParsedPayload != nil && in.ParsedPayload.RiskOverview != nil {
		if issues := check(in.ParsedPayload.RiskOverview, "raw_ai_response"); len(issues) > 0 {
			return issues
		}
	}
	if in.DashboardHome != nil {
		return check(&in.DashboardHome.AIDiagnostics.RiskDistribution, "dashboard_apply_mapping")
	}
	return nil
}

func forensicDRRSemantics(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	var accountDRR, adDRR string
	for _, kpi := range in.DashboardHome.SellerPulse.KPIs {
		if kpi.Key == "drr" {
			accountDRR = kpi.Value
		}
	}
	if in.DashboardHome.AIDiagnostics.AdEfficiency.Summary != nil && in.DashboardHome.AIDiagnostics.AdEfficiency.Summary.CurrentDRRPct != nil {
		adDRR = fmt.Sprintf("%.2f%%", *in.DashboardHome.AIDiagnostics.AdEfficiency.Summary.CurrentDRRPct)
	}
	if accountDRR == "" || adDRR == "" || accountDRR == adDRR {
		return nil
	}
	// Both exist and differ — OK if labeled; flag if hints are generic.
	genericHints := false
	for _, kpi := range in.DashboardHome.SellerPulse.KPIs {
		if kpi.Key == "drr" && strings.Contains(strings.ToLower(kpi.Hint), "доля рекламных расходов в выручке") &&
			!strings.Contains(strings.ToLower(kpi.Label), "кабинет") {
			genericHints = true
		}
	}
	if !genericHints {
		return nil
	}
	return []DashboardContentForensicIssue{{
		ID:       "drr_semantics_unlabeled",
		Severity: "warning",
		Symptom:  fmt.Sprintf("multiple DRR values without distinct labels: seller_pulse=%s ad_efficiency=%s", accountDRR, adDRR),
		DashboardHomeValue: map[string]any{
			"account_drr": accountDRR, "ad_efficiency_drr": adDRR,
		},
		RootCauseLayer:   "deterministic_widget_builder",
		ResponsibleFiles: []string{"backend/internal/dashboardhome/service.go", "backend/internal/dashboardhome/ad_efficiency_widget.go"},
		FixRequired:      true,
		FixPlan:          "Add method-specific DRR labels for account vs ad contour vs campaign rows.",
	}}
}

func forensicSurplusVsSlowTurnover(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	slow := len(in.DashboardHome.SKUFocus.SlowTurnover)
	if slow == 0 {
		slow = len(in.DashboardHome.SKUFocus.Surplus)
	}
	var ozonSurplus int
	for _, seg := range in.DashboardHome.AIDiagnostics.StockCoverage.Segments {
		if seg.Key == "surplus" {
			ozonSurplus = seg.SKUCount
		}
	}
	if slow == 0 || ozonSurplus == slow {
		return nil
	}
	if ozonSurplus == 0 && slow > 0 {
		// Expected distinct semantics — only warn if labels don't explain it.
		msg := strings.ToLower(in.DashboardHome.AIDiagnostics.StockCoverage.Message)
		if strings.Contains(msg, "медленная оборачиваемость") || strings.Contains(msg, "ozon") {
			return nil
		}
	}
	if slow > 0 && ozonSurplus == 0 {
		return []DashboardContentForensicIssue{{
			ID:       "surplus_slow_turnover_confusion",
			Severity: "warning",
			Symptom:  fmt.Sprintf("stock_coverage.surplus=%d but sku_focus.surplus(slow turnover)=%d without clear labels", ozonSurplus, slow),
			DashboardHomeValue: map[string]any{
				"ozon_surplus_segment": ozonSurplus, "slow_turnover_sku_focus": slow,
			},
			RootCauseLayer:   "dashboard_apply_mapping",
			ResponsibleFiles: []string{"backend/internal/dashboardhome/narrative_apply_helpers.go", "backend/internal/dashboardhome/risk_drivers.go"},
			FixRequired:      true,
			FixPlan:          "Distinguish Ozon SURPLUS grade from slow-turnover/storage risk in labels.",
		}}
	}
	return nil
}

func forensicRevenueAtRisk(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil || in.DashboardHome.AIDiagnostics.StockCoverage.Narrative == nil {
		return nil
	}
	rev := in.DashboardHome.AIDiagnostics.StockCoverage.Narrative.RevenueAtRisk
	if rev.Status != "available" || strings.TrimSpace(rev.Value) == "" {
		return nil
	}
	dailyRevenue := 0.0
	for _, kpi := range in.DashboardHome.SellerPulse.KPIs {
		if kpi.Key == "revenue" {
			dailyRevenue = parseMoneyLoose(kpi.Value)
		}
	}
	atRisk := parseMoneyLoose(rev.Value)
	if dailyRevenue <= 0 || atRisk <= dailyRevenue*1.2 {
		return nil
	}
	note := strings.ToLower(rev.MethodNote)
	if strings.Contains(note, "может отличаться") || strings.Contains(note, "сумма по sku") {
		return nil
	}
	return []DashboardContentForensicIssue{{
		ID:       "revenue_at_risk_method_unclear",
		Severity: "warning",
		Symptom:  fmt.Sprintf("revenue_at_risk %s exceeds daily revenue %.0f without explanatory method_note", rev.Value, dailyRevenue),
		DashboardHomeValue: map[string]any{
			"revenue_at_risk": rev.Value, "daily_revenue": dailyRevenue, "method_note": rev.MethodNote,
		},
		RootCauseLayer:   "deterministic_widget_builder",
		ResponsibleFiles: []string{"backend/internal/dashboardhome/stock_coverage_narrative.go", "backend/internal/dashboardhome/stock_coverage_seller_polish.go"},
		FixRequired:      true,
		FixPlan:          "Clarify revenue_at_risk is weighted SKU 7d estimate, not daily account revenue.",
	}}
}

func forensicSynthesisInputMetricDate(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.SynthesisInput == nil {
		return nil
	}
	if strings.TrimSpace(in.SynthesisInput.MetricDate) != "" {
		return nil
	}
	want := in.MetricDate
	if in.SynthesisInput.ManifestSummary != nil {
		if md, ok := in.SynthesisInput.ManifestSummary["metric_date"].(string); ok && md != "" {
			want = md
		}
	}
	return []DashboardContentForensicIssue{{
		ID:       "synthesis_input_metric_date_empty",
		Severity: "critical",
		Symptom:  "synthesis_input.metric_date is empty",
		SynthesisInputValue: map[string]any{
			"metric_date": in.SynthesisInput.MetricDate, "expected": want,
		},
		RootCauseLayer:   "synthesis_input_builder",
		ResponsibleFiles: []string{"backend/internal/dashboardhome/narrative_datapack_context.go", "backend/internal/dashboardhome/narrative_datapack_synthesis_input.go"},
		FixRequired:      true,
		FixPlan:          "Populate build.Context.MetricDate from Data Pack seller_context.metric_date.",
	}}
}

func forensicDebugAppliedBlocks(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil || in.DashboardHome.DashboardNarrativeDebug == nil {
		return nil
	}
	debug := in.DashboardHome.DashboardNarrativeDebug
	source := blockApplySource(in.DashboardHome, "what_changed")
	if source == "" {
		source = debug.AppliedBlocks["what_changed"]
	}
	if source == "ai" {
		if issues := forensicWhatChangedSubstitution(in); len(issues) > 0 {
			return []DashboardContentForensicIssue{{
				ID:       "debug_applied_blocks_misreport",
				Severity: "critical",
				Symptom:  "applied_blocks.what_changed=ai but content matches deterministic fallback",
				DashboardHomeValue: map[string]any{
					"applied_blocks.what_changed": debug.AppliedBlocks["what_changed"],
					"tracked_source":              source,
				},
				RootCauseLayer:   "debug_misreporting",
				ResponsibleFiles: []string{"backend/internal/dashboardhome/response_consistency.go", "backend/internal/dashboardhome/what_changed_apply.go"},
				FixRequired:      true,
				FixPlan:          "Set applied_blocks from actual apply-layer source metadata.",
			}}
		}
	}
	for block, src := range debug.AppliedBlocks {
		if src == "ai" {
			continue
		}
		if src == "substituted_by_deterministic" || src == "deterministic_substitution" || src == "substitution" {
			return []DashboardContentForensicIssue{{
				ID:                 "debug_applied_blocks_misreport",
				Severity:           "critical",
				Symptom:            fmt.Sprintf("AI-only debug reports non-ai source for %s: %s", block, src),
				DashboardHomeValue: map[string]any{"block": block, "source": src},
				RootCauseLayer:     "debug_misreporting",
				ResponsibleFiles:   []string{"backend/internal/dashboardhome/response_consistency.go"},
				FixRequired:        true,
				FixPlan:            "In AI-only any deterministic substitution must fail consistency, not report ai.",
			}}
		}
	}
	return nil
}

func pickKeys(m map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	return out
}

func intFromAny(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func parseIntLoose(s string) int64 {
	s = strings.TrimSpace(strings.ReplaceAll(s, " ", ""))
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func parseMoneyLoose(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "₽/день")
	s = strings.TrimSuffix(s, "₽")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")
	v, _ := strconv.ParseFloat(s, 64)
	if math.IsNaN(v) {
		return 0
	}
	return v
}

func mapNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func LoadDashboardHomeResponseFromFile(path string) (*Response, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func LoadDataPackFromFile(path string) (*aidatapack.SellerAIDataPack, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pack aidatapack.SellerAIDataPack
	if err := json.Unmarshal(raw, &pack); err != nil {
		return nil, err
	}
	return &pack, nil
}

func LoadSynthesisInputFromFile(path string) (*DashboardSynthesisInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var input DashboardSynthesisInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, err
	}
	return &input, nil
}

// ParseDashboardNarrativePayloadFromBytes parses stored or debug narrative JSON.
func ParseDashboardNarrativePayloadFromBytes(raw []byte) (*dashboardNarrativePayload, error) {
	return parseDashboardNarrativePayload(string(raw))
}

func forensicTopActionsGrounding(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	payload := in.StoredPayload
	if payload == nil {
		payload = in.ParsedPayload
	}
	if payload == nil || len(payload.TopActions) == 0 {
		return nil
	}
	reasonFacts := map[string][]string{}
	for _, reason := range payload.HealthReasons {
		if len(reason.FactIDs) > 0 {
			reasonFacts[reason.DriverID] = reason.FactIDs
		}
	}
	issues := make([]DashboardContentForensicIssue, 0)
	for i, action := range payload.TopActions {
		if len(action.FactIDs) > 0 {
			continue
		}
		if facts, ok := reasonFacts[action.DriverID]; ok && len(facts) > 0 {
			continue
		}
		issues = append(issues, DashboardContentForensicIssue{
			ID:       "top_action_missing_fact_ids",
			Severity: "critical",
			Symptom:  fmt.Sprintf("stored top_actions[%d] missing fact_ids grounding", i),
			ParsedPayloadValue: map[string]any{
				"driver_id": action.DriverID,
				"title":     action.Title,
			},
			RootCauseLayer:   "synthesis_prompt",
			ResponsibleFiles: []string{"backend/internal/dashboardhome/narrative_datapack_synthesis_prompt.go"},
			FixRequired:      true,
			FixPlan:          "top_actions must include fact_ids[] grounded in validated signals",
		})
	}
	return issues
}

func forensicCategoryCatalogCoverage(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.SynthesisInput == nil {
		return nil
	}
	for _, issue := range validateAIOnlyCategoryCatalogCoverage(in.ParsedPayload, *in.SynthesisInput) {
		return []DashboardContentForensicIssue{{
			ID:       issue.Code,
			Severity: mapConsistencySeverity(issue.Severity),
			Symptom:  issue.Message,
			SynthesisInputValue: map[string]any{
				"category_catalog_count":    in.SynthesisInput.CategoryCatalogCount,
				"category_selection_policy": in.SynthesisInput.CategorySelectionPolicy,
			},
			ParsedPayloadValue: map[string]any{
				"category_summaries_count": categorySummariesCount(in.ParsedPayload),
			},
			RootCauseLayer:   "synthesis_prompt",
			ResponsibleFiles: []string{"backend/internal/dashboardhome/narrative_datapack_synthesis_prompt.go"},
			FixRequired:      issue.Severity == "failed",
			FixPlan:          issue.Message,
		}}
	}
	return nil
}

func categorySummariesCount(payload *dashboardNarrativePayload) int {
	if payload == nil {
		return 0
	}
	return len(payload.CategorySummaries)
}
