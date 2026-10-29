package dashboardhome

import (
	"fmt"
	"strings"
)

func validateAIOnlyV611Content(payload *dashboardNarrativePayload, input DashboardSynthesisInput) []DashboardHomeConsistencyIssue {
	if payload == nil {
		return nil
	}
	issues := make([]DashboardHomeConsistencyIssue, 0, 8)
	issues = append(issues, validateAIOnlyInternalMetricNames(payload)...)
	issues = append(issues, validateAIOnlyCategoryCatalogCoverage(payload, input)...)
	return issues
}

func validateAIOnlyInternalMetricNames(payload *dashboardNarrativePayload) []DashboardHomeConsistencyIssue {
	issues := make([]DashboardHomeConsistencyIssue, 0)
	check := func(field, text string) {
		if containsInternalMetricName(text) {
			issues = append(issues, DashboardHomeConsistencyIssue{
				Code:     "seller_facing_internal_metric_name",
				Severity: "failed",
				Message:  field + " contains internal metric identifier",
			})
		}
	}
	check("headline", payload.Headline)
	check("executive_summary", payload.ExecutiveSummary)
	if payload.AccountHealth != nil {
		check("account_health.headline", payload.AccountHealth.Headline)
		check("account_health.executive_summary", payload.AccountHealth.ExecutiveSummary)
		for i, d := range payload.AccountHealth.RiskDrivers {
			check(fmt.Sprintf("account_health.risk_drivers[%d].title", i), d.Title)
			check(fmt.Sprintf("account_health.risk_drivers[%d].reason", i), d.Reason)
		}
	}
	if payload.WhatChanged != nil {
		check("what_changed.summary", payload.WhatChanged.Summary)
		check("what_changed.main_conclusion", payload.WhatChanged.MainConclusion)
		for gi, group := range payload.WhatChanged.Groups {
			for ii, item := range group.Items {
				check(fmt.Sprintf("what_changed.groups[%d].items[%d]", gi, ii), item.Label+" "+item.Text)
			}
		}
	}
	if n := payload.SellerPulseNarrative; n != nil {
		check("seller_pulse.summary", n.Summary)
		for i, qa := range n.QuickActions {
			check(fmt.Sprintf("seller_pulse.quick_actions[%d]", i), qa.Label+" "+qa.Description)
		}
	}
	if n := payload.AdEfficiencyNarrative; n != nil {
		check("ad_efficiency.summary", n.Summary)
		for i, item := range n.Checklist {
			check(fmt.Sprintf("ad_efficiency.checklist[%d]", i), item.Title+" "+item.Text)
		}
	}
	for i, card := range payload.CategorySummaries {
		check(fmt.Sprintf("category_summaries[%d]", i), strings.Join([]string{card.Headline, card.KeyRisk, card.RecommendedAction}, " "))
	}
	return issues
}

func validateAIOnlyCategoryCatalogCoverage(payload *dashboardNarrativePayload, input DashboardSynthesisInput) []DashboardHomeConsistencyIssue {
	if input.CategorySelectionPolicy != CategorySummariesSelectionPolicyAllCatalog {
		return nil
	}
	expected := input.CategoryCatalogCount
	if expected == 0 {
		expected = len(input.CategoryCatalog)
	}
	if expected == 0 {
		return nil
	}
	got := len(payload.CategorySummaries)
	if got != expected {
		return []DashboardHomeConsistencyIssue{{
			Code:     "category_summaries_catalog_count_mismatch",
			Severity: "failed",
			Message:  fmt.Sprintf("category_summaries count=%d expected all %d catalog categories (policy=%s)", got, expected, input.CategorySelectionPolicy),
		}}
	}
	catalogIDs := map[string]struct{}{}
	for _, row := range input.CategoryCatalog {
		id := strings.TrimSpace(row.CategoryID)
		if id != "" {
			catalogIDs[id] = struct{}{}
		}
	}
	for i, card := range payload.CategorySummaries {
		id := strings.TrimSpace(card.CategoryID)
		if id == "" {
			return []DashboardHomeConsistencyIssue{{
				Code:     "category_summaries_missing_category_id",
				Severity: "failed",
				Message:  fmt.Sprintf("category_summaries[%d] missing category_id for catalog coverage", i),
			}}
		}
		if _, ok := catalogIDs[id]; !ok {
			return []DashboardHomeConsistencyIssue{{
				Code:     "category_summaries_unknown_category_id",
				Severity: "failed",
				Message:  fmt.Sprintf("category_summaries[%d] category_id %q not in catalog", i, id),
			}}
		}
		delete(catalogIDs, id)
	}
	if len(catalogIDs) > 0 {
		missing := make([]string, 0, len(catalogIDs))
		for id := range catalogIDs {
			missing = append(missing, id)
		}
		return []DashboardHomeConsistencyIssue{{
			Code:     "category_summaries_missing_catalog_categories",
			Severity: "failed",
			Message:  fmt.Sprintf("category_summaries missing catalog categories: %v", missing),
		}}
	}
	return nil
}
