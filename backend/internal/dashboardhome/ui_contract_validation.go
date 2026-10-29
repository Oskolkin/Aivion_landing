package dashboardhome

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// DashboardUIContractIssue documents a UI contract mismatch in /dashboard/home response.
type DashboardUIContractIssue struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// DashboardUIContractValidationReport is output of validate-dashboard-ui-contract.
type DashboardUIContractValidationReport struct {
	Status  string                     `json:"status"`
	Issues  []DashboardUIContractIssue `json:"issues"`
	Summary struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
	} `json:"summary"`
}

func RunDashboardUIContractValidation(resp *Response) DashboardUIContractValidationReport {
	report := DashboardUIContractValidationReport{Status: "passed"}
	if resp == nil {
		report.Status = "failed"
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "missing_response", Severity: "critical", Message: "dashboard response is nil",
		})
		return finalizeUIContractReport(report)
	}

	if n := resp.SellerPulse.Narrative; n != nil && (n.Status == "available" || n.Status == "ready") {
		if len(n.ImportantNotes) == 0 {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "seller_pulse_missing_important_notes", Severity: "critical",
				Message: "seller_pulse.narrative is available but important_notes is empty",
			})
		}
		if len(n.QuickActions) == 0 && len(resp.AIDiagnostics.TopActions) >= 3 {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "seller_pulse_missing_quick_actions", Severity: "critical",
				Message: "top_actions present but seller_pulse quick_actions missing",
			})
		}
	}

	hasAOV := false
	hasAdShare := false
	for _, kpi := range resp.SellerPulse.KPIs {
		switch kpi.Key {
		case "average_order_value", "aov":
			hasAOV = true
			if len(kpi.Sparkline7D) == 0 {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: "kpi_aov_missing_sparkline", Severity: "warning",
					Message: "seller_pulse.kpis[average_order_value] missing sparkline_7d",
				})
			}
		case "ad_spend_share_total_revenue", "drr":
			hasAdShare = true
			if len(kpi.Sparkline7D) == 0 {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: fmt.Sprintf("kpi_%s_missing_sparkline", kpi.Key), Severity: "warning",
					Message: fmt.Sprintf("seller_pulse.kpis[%s] missing sparkline_7d", kpi.Key),
				})
			}
			if countSparklineDataPoints(kpi.Sparkline7D) >= 2 {
				label := ""
				if kpi.Trend != nil {
					label = strings.TrimSpace(kpi.Trend.Label)
				}
				if strings.EqualFold(label, "Динамика недоступна") {
					report.Issues = append(report.Issues, DashboardUIContractIssue{
						ID: fmt.Sprintf("kpi_%s_trend_label_unavailable_with_sparkline", kpi.Key), Severity: "critical",
						Message: fmt.Sprintf("kpi[%s] has sparkline but trend label says unavailable", kpi.Key),
					})
				}
				if kpi.Delta == nil && label != "" && !strings.Contains(strings.ToLower(label), "7 дн") {
					report.Issues = append(report.Issues, DashboardUIContractIssue{
						ID: fmt.Sprintf("kpi_%s_missing_sparkline_trend_label", kpi.Key), Severity: "warning",
						Message: fmt.Sprintf("kpi[%s] has sparkline without delta but trend label is not 7-day", kpi.Key),
					})
				}
			}
		case "ctr":
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "seller_pulse_ctr_in_primary_set", Severity: "warning",
				Message: "seller_pulse primary KPI set should use average_order_value instead of ctr",
			})
		case "revenue":
			if kpi.Delta != nil && kpi.Delta.Pct != nil && *kpi.Delta.Pct > -5 && *kpi.Delta.Pct < 0 {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: "revenue_delta_pct_looks_like_ratio", Severity: "critical",
					Message: fmt.Sprintf("revenue.delta.pct=%v looks like ratio not percent", *kpi.Delta.Pct),
				})
			}
		}
	}
	if !hasAOV {
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "seller_pulse_missing_aov_kpi", Severity: "critical",
			Message: "seller_pulse primary KPI set must include average_order_value",
		})
	}
	_ = hasAdShare

	wc := resp.AIDiagnostics.WhatChanged
	if wc.Status == WidgetAvailable {
		conclusion := strings.TrimSpace(firstNonEmpty(wc.MainConclusion, wc.Summary))
		if isGenericWhatChangedText(conclusion) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_generic_main_conclusion", Severity: "critical",
				Message: "what_changed main conclusion is generic",
			})
		}
		if isAISeesWhatChangedText(conclusion) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_ai_sees_main_conclusion", Severity: "critical",
				Message: "what_changed main conclusion starts with AI/system phrasing",
			})
		}
		if conclusion == "" {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_empty_main_conclusion", Severity: "critical",
				Message: "what_changed main conclusion is empty",
			})
		}
		if strings.HasSuffix(conclusion, "...") || strings.HasSuffix(conclusion, "…") {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_truncated_main_conclusion", Severity: "critical",
				Message: "what_changed main conclusion looks truncated",
			})
		}
		if utf8.RuneCountInString(conclusion) > whatChangedMainConclusionMaxRunes {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_main_conclusion_too_long", Severity: "warning",
				Message: "what_changed main conclusion exceeds recommended length",
			})
		}
		for _, group := range wc.Groups {
			if len(group.Items) > whatChangedGroupMaxItems {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: "what_changed_group_" + group.Key + "_too_many_items", Severity: "warning",
					Message: fmt.Sprintf("what_changed group %s has more than %d items", group.Key, whatChangedGroupMaxItems),
				})
			}
			if len(group.Items) == 1 {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: "what_changed_group_" + group.Key + "_sparse_items", Severity: "warning",
					Message: fmt.Sprintf("what_changed group %s has only one item", group.Key),
				})
			}
			labels := map[string]int{}
			for _, item := range group.Items {
				lk := strings.ToLower(strings.TrimSpace(item.Label))
				if lk != "" {
					labels[lk]++
					if labels[lk] > 1 {
						report.Issues = append(report.Issues, DashboardUIContractIssue{
							ID: "what_changed_duplicate_label_" + group.Key, Severity: "warning",
							Message: fmt.Sprintf("what_changed group %s has duplicate label %q", group.Key, item.Label),
						})
					}
				}
				if actionMalformedWhatChangedStart.MatchString(item.Text) {
					report.Issues = append(report.Issues, DashboardUIContractIssue{
						ID: "what_changed_malformed_item_text", Severity: "critical",
						Message: "what_changed item text starts with technical number pattern",
					})
				}
				if actionGenericSignalPattern.MatchString(item.Text) {
					report.Issues = append(report.Issues, DashboardUIContractIssue{
						ID: "what_changed_generic_signal_text", Severity: "critical",
						Message: "what_changed item contains generic signal filler without fact",
					})
				}
			}
		}
		if isWeakWhatChangedMainConclusion(conclusion) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_weak_main_conclusion", Severity: "warning",
				Message: "what_changed main_conclusion is weak or template-like",
			})
		}
	}

	if n := resp.AIDiagnostics.StockCoverage.Narrative; n != nil && n.Status == "available" {
		headline := strings.TrimSpace(n.Headline)
		summary := strings.TrimSpace(n.Summary)
		nextStep := strings.TrimSpace(n.NextStep)
		if headline == "" || summary == "" || nextStep == "" {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "stock_coverage_narrative_missing_seller_fields", Severity: "critical",
				Message: "stock_coverage_narrative available but headline/summary/next_step missing",
			})
		}
		if utf8.RuneCountInString(headline) > stockCoverageHeadlineMaxRunes {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "stock_coverage_headline_too_long", Severity: "critical",
				Message: "stock_coverage headline exceeds 80 characters",
			})
		}
		if strings.Contains(headline, "...") || strings.Contains(headline, "…") || strings.Contains(headline, "\uFFFD") {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "stock_coverage_headline_truncated_or_corrupt", Severity: "critical",
				Message: "stock_coverage headline looks truncated or corrupt",
			})
		}
		if summary != "" && (strings.EqualFold(normalizeComparableText(summary), normalizeComparableText(headline)) ||
			strings.HasPrefix(normalizeComparableText(summary), normalizeComparableText(headline))) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "stock_coverage_summary_duplicates_headline", Severity: "warning",
				Message: "stock_coverage summary duplicates headline",
			})
		}
		if utf8.RuneCountInString(summary) < stockCoverageSummaryMinRunes {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "stock_coverage_summary_too_short", Severity: "warning",
				Message: "stock_coverage summary is shorter than recommended minimum",
			})
		}
		if resp.SKUFocus.NoSalesWithStockTotalCount > 0 && n.NoSalesWithStock.Status != "available" {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "stock_coverage_no_sales_unavailable_with_sku_focus", Severity: "critical",
				Message: "no_sales_with_stock unavailable while sku_focus has data",
			})
		}
	}
	if containsInternalFieldReference(resp.AIDiagnostics.StockCoverage.Message) {
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "stock_coverage_message_internal_field_ref", Severity: "critical",
			Message: "stock_coverage.message contains internal API field reference",
		})
	}

	for i, action := range resp.AIDiagnostics.TopActions {
		if !actionCategoryConsistentWithText(action) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("top_action_%d_category_semantic_mismatch", i), Severity: "critical",
				Message: fmt.Sprintf("top_action[%d] category %q inconsistent with advertising/stock/funnel semantics", i, action.Category),
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
			if ok && !quickActionKindMatchesCategory(qa.Kind, top.Category) {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: fmt.Sprintf("quick_action_%d_kind_mismatch", i), Severity: "critical",
					Message: fmt.Sprintf("quick_action kind %q does not match top_action category %q", qa.Kind, top.Category),
				})
			}
		}
		if n.DisplayHeadline && strings.EqualFold(strings.TrimSpace(n.Headline), "Пульс продавца") {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "seller_pulse_headline_display_enabled", Severity: "warning",
				Message: "seller_pulse narrative headline should not be displayed",
			})
		}
	}
	if funnelHasContradictoryComparison(resp.AIDiagnostics.FunnelAnalysis) {
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "funnel_comparison_contradiction", Severity: "critical",
			Message: "funnel primary_drop_message contradicts ai_analysis comparison availability",
		})
	}
	if len(resp.SKUFocus.Surplus) > 0 && len(resp.SKUFocus.SlowTurnover) > 0 {
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "sku_focus_surplus_slow_turnover_duplicate", Severity: "warning",
			Message: "sku_focus.surplus and slow_turnover both populated",
		})
	}

	for i, driver := range resp.AIDiagnostics.AccountSummary.RiskDrivers {
		title := strings.TrimSpace(driver.Title)
		reason := strings.TrimSpace(driver.Reason)
		if title == "" {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("risk_driver_%d_missing_title", i), Severity: "critical",
				Message: fmt.Sprintf("risk_driver[%d] missing title", i),
			})
		}
		if title == reason {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("risk_driver_%d_title_equals_reason", i), Severity: "warning",
				Message: fmt.Sprintf("risk_driver[%d] title equals reason", i),
			})
		}
		if utf8.RuneCountInString(title) > stockCoverageHeadlineMaxRunes {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("risk_driver_%d_title_too_long", i), Severity: "warning",
				Message: fmt.Sprintf("risk_driver[%d] title exceeds 80 characters", i),
			})
		}
		if driver.ID == driverSlowTurnover && driver.CategoryLabel == "Кабинет" {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "risk_driver_slow_turnover_invalid_label", Severity: "critical",
				Message: "driver-slow-turnover must not have category_label Кабинет",
			})
		}
		if strings.TrimSpace(driver.CategoryLabel) == "" {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("risk_driver_%d_missing_category_label", i), Severity: "critical",
				Message: fmt.Sprintf("risk_driver[%d] missing category_label", i),
			})
		}
	}
	if containsAISeesSellerFacingPhrase(resp.AIDiagnostics.AccountSummary.ExecutiveSummary) {
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "account_summary_ai_sees_phrase", Severity: "critical",
			Message: "account_summary executive_summary contains forbidden AI/system phrasing",
		})
	}
	if containsInternalMetricName(resp.AIDiagnostics.AccountSummary.ExecutiveSummary) {
		report.Issues = append(report.Issues, DashboardUIContractIssue{
			ID: "account_summary_internal_metric_name", Severity: "critical",
			Message: "account_summary executive_summary contains internal metric identifier",
		})
	}

	for i, card := range resp.AIDiagnostics.CategorySummaries {
		if card.CategoryScore == nil && card.HealthScore == nil {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("category_%d_missing_score", i), Severity: "critical",
				Message: fmt.Sprintf("category_summaries[%d] missing category_score/health_score", i),
			})
			continue
		}
		score := int32(0)
		if card.CategoryScore != nil {
			score = *card.CategoryScore
		} else if card.HealthScore != nil {
			score = (*card.HealthScore + 5) / 10
		}
		if score < 0 || score > categoryScoreMax {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("category_%d_score_out_of_range", i), Severity: "critical",
				Message: fmt.Sprintf("category_summaries[%d] score=%d out of 0..10", i, score),
			})
		}
		if card.ScoreMax > 0 && card.ScoreMax != categoryScoreMax {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("category_%d_score_max_mismatch", i), Severity: "warning",
				Message: fmt.Sprintf("category_summaries[%d] score_max=%d want %d", i, card.ScoreMax, categoryScoreMax),
			})
		}
		if !isCategoryScoreSeverityConsistent(score, card.Severity) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: fmt.Sprintf("category_%d_score_severity_mismatch", i), Severity: "critical",
				Message: fmt.Sprintf("category_summaries[%d] score=%d severity=%q inconsistent", i, score, card.Severity),
			})
		}
	}

	if ad := resp.AIDiagnostics.AdEfficiency.Narrative; ad != nil && ad.Status == "available" {
		for i, item := range ad.Checklist {
			if strings.EqualFold(strings.TrimSpace(item.Title), strings.TrimSpace(item.Text)) {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: fmt.Sprintf("ad_efficiency_checklist_%d_title_equals_text", i), Severity: "critical",
					Message: fmt.Sprintf("ad_efficiency checklist[%d] title equals text", i),
				})
			}
		}
	}
	if wc.Status == WidgetAvailable {
		summary := strings.TrimSpace(wc.Summary)
		main := strings.TrimSpace(wc.MainConclusion)
		if summary != "" && main != "" && strings.EqualFold(summary, main) {
			report.Issues = append(report.Issues, DashboardUIContractIssue{
				ID: "what_changed_main_conclusion_equals_summary", Severity: "critical",
				Message: "what_changed main_conclusion must differ from summary",
			})
		}
		for gi, group := range wc.Groups {
			for ii, item := range group.Items {
				if hasDuplicateWhatChangedLabelPrefix(item.Label, item.Text) {
					report.Issues = append(report.Issues, DashboardUIContractIssue{
						ID: fmt.Sprintf("what_changed_duplicate_label_prefix_%d_%d", gi, ii), Severity: "critical",
						Message: "what_changed item repeats label prefix in text",
					})
				}
			}
		}
	}
	if funnelStageDayOverDayAvailable(resp.AIDiagnostics.SalesFunnel.Stages) {
		fa := resp.AIDiagnostics.FunnelAnalysis.AIAnalysis
		funnelTexts := []string{
			fa.MainConclusion,
			fa.ShortSummary,
			fa.ComparisonsCommentary.PreviousPeriod,
			fa.ComparisonsCommentary.SevenDayAverage,
		}
		funnelTexts = append(funnelTexts, fa.ActionItems...)
		for _, item := range fa.CheckDirections {
			funnelTexts = append(funnelTexts, item.Text)
		}
		for _, factor := range fa.DropFactors {
			funnelTexts = append(funnelTexts, factor.Explanation)
		}
		for _, text := range funnelTexts {
			if funnelComparisonUnavailablePhrases(text) {
				report.Issues = append(report.Issues, DashboardUIContractIssue{
					ID: "funnel_comparison_unavailable_contradiction", Severity: "critical",
					Message: "funnel analysis claims prior-day comparison unavailable while sales_funnel has day-over-day facts",
				})
				break
			}
		}
	}

	return finalizeUIContractReport(report)
}

func finalizeUIContractReport(report DashboardUIContractValidationReport) DashboardUIContractValidationReport {
	for _, issue := range report.Issues {
		if issue.Severity == "critical" {
			report.Summary.Critical++
		} else {
			report.Summary.Warning++
		}
	}
	if report.Summary.Critical > 0 {
		report.Status = "failed"
	}
	return report
}

func WriteDashboardUIContractValidationReport(path string, report DashboardUIContractValidationReport) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
