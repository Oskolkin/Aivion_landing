package dashboardhome

import (
	"fmt"
	"strings"
)

func validateAIOnlyV61Content(payload *dashboardNarrativePayload, input DashboardSynthesisInput) []DashboardHomeConsistencyIssue {
	if payload == nil {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0, 16)
	issues = append(issues, validateAIOnlyAccountHealthCategoryLabels(payload)...)
	issues = append(issues, validateAIOnlyCategoryScoreSeverityMatrix(payload)...)
	issues = append(issues, validateAIOnlyAdEfficiencyChecklistTitles(payload)...)
	issues = append(issues, validateAIOnlyWhatChangedV61(payload)...)
	issues = append(issues, validateAIOnlyNoAISeesPhrases(payload)...)
	issues = append(issues, validateAIOnlyTopActionsGrounding(payload)...)
	issues = append(issues, validateAIOnlyFunnelComparisonContradiction(payload, input)...)
	return issues
}

func validateAIOnlyAccountHealthCategoryLabels(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload.AccountHealth == nil {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0)
	for i, d := range payload.AccountHealth.RiskDrivers {
		if strings.TrimSpace(d.CategoryLabel) == "" {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "account_health_missing_category_label",
				Severity: "failed",
				Message:  fmt.Sprintf("account_health.risk_drivers[%d] missing category_label", i),
			})
		}
	}
	return issues
}

func validateAIOnlyCategoryScoreSeverityMatrix(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload == nil || len(payload.CategorySummaries) == 0 {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0)
	for i, card := range payload.CategorySummaries {
		if card.CategoryScore == nil {
			continue
		}
		if !isCategoryScoreSeverityConsistent(*card.CategoryScore, card.Severity) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "category_score_severity_mismatch",
				Severity: "failed",
				Message:  fmt.Sprintf("category_summaries[%d] score=%d severity=%q inconsistent", i, *card.CategoryScore, card.Severity),
			})
		}
	}
	return issues
}

func validateAIOnlyAdEfficiencyChecklistTitles(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	n := payload.AdEfficiencyNarrative
	if n == nil {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0)
	for i, item := range n.Checklist {
		if strings.EqualFold(strings.TrimSpace(item.Title), strings.TrimSpace(item.Text)) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "ad_efficiency_checklist_title_equals_text",
				Severity: "failed",
				Message:  fmt.Sprintf("ad_efficiency_narrative.checklist[%d] title equals text", i),
			})
		}
	}
	return issues
}

func validateAIOnlyWhatChangedV61(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	wc := payload.WhatChanged
	if wc == nil {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0)
	summary := strings.TrimSpace(wc.Summary)
	main := strings.TrimSpace(wc.MainConclusion)
	if summary != "" && main != "" && strings.EqualFold(summary, main) {
		issues = append(issues, DashboardHomeConsistencyIssue{
			Code:     "what_changed_main_conclusion_equals_summary",
			Severity: "failed",
			Message:  "what_changed main_conclusion must differ from summary",
		})
	}
	for gi, group := range wc.Groups {
		if group.Key == whatChangedKeyImproved && !allowedImprovedGroupLabel(group.Label, len(group.Items) > 0) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "what_changed_improved_label_invalid",
				Severity: "failed",
				Message:  fmt.Sprintf("what_changed.groups[%d] invalid improved label %q", gi, group.Label),
			})
		}
		if group.Key == whatChangedKeyImproved && group.Label == "Улучшилось" && improvedGroupIsStrengthsNotProvenImprovement(group) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "what_changed_improved_label_strengths_mismatch",
				Severity: "failed",
				Message:  fmt.Sprintf("what_changed.groups[%d] uses Улучшилось for strengths without proven improvement", gi),
			})
		}
		for ii, item := range group.Items {
			if hasDuplicateWhatChangedLabelPrefix(item.Label, item.Text) {
				issues = append(issues, DashboardHomeConsistencyIssue{
					Code:     "what_changed_duplicate_label_prefix",
					Severity: "failed",
					Message:  fmt.Sprintf("what_changed.groups[%d].items[%d] repeats label in text", gi, ii),
				})
			}
		}
	}
	return issues
}

func validateAIOnlyNoAISeesPhrases(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	issues := make([]DashboardHomeConsistencyIssue, 0)
	check := func(field, text string) {
		if containsAISeesSellerFacingPhrase(text) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "seller_facing_ai_sees_phrase",
				Severity: "failed",
				Message:  field + " contains forbidden AI/system phrasing",
			})
		}
	}
	check("headline", payload.Headline)
	check("executive_summary", payload.ExecutiveSummary)
	if payload.AccountHealth != nil {
		check("account_health.executive_summary", payload.AccountHealth.ExecutiveSummary)
		check("account_health.headline", payload.AccountHealth.Headline)
	}
	if payload.WhatChanged != nil {
		check("what_changed.summary", payload.WhatChanged.Summary)
		check("what_changed.main_conclusion", payload.WhatChanged.MainConclusion)
	}
	return issues
}

func validateAIOnlyTopActionsGrounding(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	if payload == nil || len(payload.TopActions) == 0 {
		return nil
	}
	reasonFacts := map[string][]string{}
	for _, reason := range payload.HealthReasons {
		if len(reason.FactIDs) > 0 {
			reasonFacts[reason.DriverID] = reason.FactIDs
		}
	}
	issues := make([]DashboardHomeConsistencyIssue, 0)
	for i, action := range payload.TopActions {
		if len(action.FactIDs) > 0 {
			continue
		}
		if facts, ok := reasonFacts[action.DriverID]; ok && len(facts) > 0 {
			continue
		}
		issues = append(issues, DashboardHomeConsistencyIssue{
			Code:     "top_action_missing_fact_ids",
			Severity: "failed",
			Message:  fmt.Sprintf("top_actions[%d] missing fact_ids grounding in stored payload", i),
		})
	}
	return issues
}

func validateAIOnlyFunnelComparisonContradiction(payload *dashboardNarrativePayload, input DashboardSynthesisInput) []DashboardHomeConsistencyIssue {
	comparisonAvailable := false
	if input.FunnelSemantics != nil {
		if v, ok := input.FunnelSemantics["comparison_available"].(bool); ok && v {
			comparisonAvailable = true
		}
		if stages, ok := input.FunnelSemantics["stage_day_over_day"]; ok {
			switch typed := stages.(type) {
			case []map[string]any:
				comparisonAvailable = comparisonAvailable || len(typed) > 0
			case []any:
				comparisonAvailable = comparisonAvailable || len(typed) > 0
			}
		}
	}
	if !comparisonAvailable || payload.FunnelAIAnalysis == nil {
		return nil
	}
	texts := []string{
		payload.FunnelAIAnalysis.MainConclusion,
		payload.FunnelAIAnalysis.ShortSummary,
		payload.FunnelAIAnalysis.ComparisonsCommentary.PreviousPeriod,
		payload.FunnelAIAnalysis.ComparisonsCommentary.SevenDayAverage,
	}
	texts = append(texts, payload.FunnelAIAnalysis.ActionItems...)
	for _, text := range texts {
		if funnelComparisonUnavailablePhrases(text) {
			return []DashboardHomeConsistencyIssue{{
				Code:     "funnel_comparison_unavailable_contradiction",
				Severity: "failed",
				Message:  "funnel_ai_analysis claims prior-day comparison unavailable while stage_day_over_day facts exist",
			}}
		}
	}
	return nil
}
