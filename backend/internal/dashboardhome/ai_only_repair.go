package dashboardhome

import (
	"encoding/json"
	"strings"
)

// repairDashboardNarrativePayloadAIOnly applies technical normalization and non-dropping content repairs for AI-only mode.
func repairDashboardNarrativePayloadAIOnly(payload *dashboardNarrativePayload) narrativeRepairResult {
	result := narrativeRepairResult{
		Payload:           payload,
		AIActionsReceived: 0,
	}
	if payload == nil {
		result.HardErrors = append(result.HardErrors, "nil_dashboard_narrative_payload")
		return result
	}
	result.AIActionsReceived = len(payload.TopActions)

	payload.Headline = trimNarrativeText(payload.Headline)
	payload.ExecutiveSummary = trimNarrativeText(payload.ExecutiveSummary)
	payload.ImprovementSummary = trimNarrativeText(payload.ImprovementSummary)

	repairAIOnlyRiskOverviewGroups(payload, &result)
	repairAIOnlyFunnelAnalysis(payload, &result)
	repairAIOnlyCategorySummaries(payload, &result)
	repairAIOnlyAccountHealthFields(payload, &result)
	repairAIOnlyWhatChangedFields(payload, &result)
	repairAIOnlySellerFacingPhrases(payload, &result)

	if payload.SellerPulseNarrative != nil {
		if raw, err := json.Marshal(payload.SellerPulseNarrative); err == nil && sellerPulseNarrativeLooksNested(string(raw)) {
			if unwrapped := unwrapSellerPulseNarrativeRaw(raw); unwrapped != nil {
				payload.SellerPulseNarrative = unwrapped
				result.RepairActions = append(result.RepairActions, "seller_pulse_narrative_unwrapped")
				result.Repaired = true
			}
		}
	}
	if payload.AdEfficiencyNarrative != nil {
		repairAdEfficiencyNarrative(payload.AdEfficiencyNarrative)
		repairAdEfficiencyChecklistTitles(payload.AdEfficiencyNarrative, &result)
	}
	repairAIOnlyAdEfficiencyNarrative(payload, &result)
	repairAIOnlyStockCoverageNarrative(payload, &result)

	result.AIActionsAccepted = len(payload.TopActions)
	return result
}

func adEfficiencyNarrativeStatusInvalid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "pending", "stale", "fallback":
		return true
	default:
		return false
	}
}

func repairAIOnlyAdEfficiencyNarrative(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload == nil {
		return
	}
	n := payload.AdEfficiencyNarrative
	if n == nil {
		payload.AdEfficiencyNarrative = &AdEfficiencyNarrative{
			Status:  "available",
			Summary: defaultAdEfficiencySummaryFromPayload(payload),
		}
		result.RepairActions = append(result.RepairActions, "ad_efficiency_narrative_synthesized")
		result.Repaired = true
		return
	}
	if adEfficiencyNarrativeStatusInvalid(n.Status) {
		if strings.TrimSpace(n.Summary) == "" {
			n.Summary = defaultAdEfficiencySummaryFromPayload(payload)
			result.RepairActions = append(result.RepairActions, "ad_efficiency_summary_synthesized")
		}
		n.Status = "available"
		result.RepairActions = append(result.RepairActions, "ad_efficiency_status_normalized")
		result.Repaired = true
	}
}

func defaultAdEfficiencySummaryFromPayload(payload *dashboardNarrativePayload) string {
	if payload != nil {
		if s := strings.TrimSpace(payload.ExecutiveSummary); s != "" {
			return trimNarrativeText(s)
		}
		if payload.AccountHealth != nil {
			if s := strings.TrimSpace(payload.AccountHealth.ExecutiveSummary); s != "" {
				return trimNarrativeText(s)
			}
		}
	}
	return "Проверьте эффективность рекламных кампаний и перераспределите бюджет в пользу кампаний с лучшей окупаемостью."
}

func repairAIOnlyStockCoverageNarrative(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload == nil {
		return
	}
	n := payload.StockCoverageNarrative
	invalid := func(status string) bool {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "", "pending", "stale", "fallback":
			return true
		default:
			return false
		}
	}
	if n == nil {
		payload.StockCoverageNarrative = &StockCoverageNarrative{
			Status:  "available",
			Summary: defaultStockCoverageSummaryFromPayload(payload),
		}
		result.RepairActions = append(result.RepairActions, "stock_coverage_narrative_synthesized")
		result.Repaired = true
		return
	}
	if invalid(n.Status) {
		if strings.TrimSpace(n.Summary) == "" {
			n.Summary = defaultStockCoverageSummaryFromPayload(payload)
			result.RepairActions = append(result.RepairActions, "stock_coverage_summary_synthesized")
		}
		n.Status = "available"
		result.RepairActions = append(result.RepairActions, "stock_coverage_status_normalized")
		result.Repaired = true
	}
}

func defaultStockCoverageSummaryFromPayload(payload *dashboardNarrativePayload) string {
	if payload != nil && strings.TrimSpace(payload.ExecutiveSummary) != "" {
		return trimNarrativeText(payload.ExecutiveSummary)
	}
	return "Проверьте остатки и покрытие складов по ключевым SKU, чтобы не терять продажи из-за out-of-stock."
}

func repairAIOnlyRiskOverviewGroups(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload.RiskOverview == nil {
		return
	}
	for i := range payload.RiskOverview.Groups {
		g := &payload.RiskOverview.Groups[i]
		label := strings.TrimSpace(g.Label)
		if strings.TrimSpace(g.Group) != "" && strings.TrimSpace(g.Severity) != "" {
			continue
		}
		switch label {
		case "Высокий риск":
			g.Group, g.Severity = "high_risk", "critical"
		case "Средний риск":
			g.Group, g.Severity = "medium_risk", "medium"
		case "Зона роста":
			g.Group, g.Severity = "growth_zone", "positive"
		default:
			continue
		}
		result.RepairActions = append(result.RepairActions, "risk_overview_group_"+itoa(i)+"_repaired")
		result.Repaired = true
	}
}

func repairAIOnlyFunnelAnalysis(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload.FunnelAIAnalysis == nil {
		return
	}
	ai := payload.FunnelAIAnalysis
	if strings.TrimSpace(ai.WeakestStage.Key) == "" && strings.TrimSpace(ai.ShortSummary) != "" {
		ai.WeakestStage = FunnelWeakestStage{
			Status: "available", Label: "См. short_summary", Explanation: ai.ShortSummary,
		}
		result.RepairActions = append(result.RepairActions, "funnel_weakest_stage_from_summary")
		result.Repaired = true
	}
}

func repairAIOnlyCategorySummaries(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	for i := range payload.CategorySummaries {
		card := &payload.CategorySummaries[i]
		if card.CategoryScore == nil || *card.CategoryScore < 0 || *card.CategoryScore > categoryScoreMax {
			continue
		}
		if !isCategoryScoreSeverityConsistent(*card.CategoryScore, card.Severity) {
			card.Severity = severityForCategoryScore10(*card.CategoryScore)
			result.RepairActions = append(result.RepairActions, "category_summary_"+itoa(i)+"_severity_from_score")
			result.Repaired = true
		}
	}
}

func repairAIOnlyAccountHealthFields(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload == nil || payload.AccountHealth == nil {
		return
	}
	ah := payload.AccountHealth
	if strings.TrimSpace(payload.Headline) == "" && strings.TrimSpace(ah.Headline) != "" {
		payload.Headline = ah.Headline
		result.RepairActions = append(result.RepairActions, "headline_from_account_health")
		result.Repaired = true
	}
	if strings.TrimSpace(payload.ExecutiveSummary) == "" && strings.TrimSpace(ah.ExecutiveSummary) != "" {
		payload.ExecutiveSummary = ah.ExecutiveSummary
		result.RepairActions = append(result.RepairActions, "executive_summary_from_account_health")
		result.Repaired = true
	}
	if strings.TrimSpace(payload.ImprovementSummary) == "" && strings.TrimSpace(ah.ImprovementSummary) != "" {
		payload.ImprovementSummary = ah.ImprovementSummary
		result.RepairActions = append(result.RepairActions, "improvement_summary_from_account_health")
		result.Repaired = true
	}
	for i := range ah.RiskDrivers {
		if strings.TrimSpace(ah.RiskDrivers[i].CategoryLabel) == "" {
			ah.RiskDrivers[i].CategoryLabel = riskDriverCategoryLabel(ah.RiskDrivers[i].Category)
			result.RepairActions = append(result.RepairActions, "account_health_risk_driver_"+itoa(i)+"_category_label")
			result.Repaired = true
		}
	}
}

func containsAny(text string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := []byte{}
	n := i
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func repairAIOnlyWhatChangedFields(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload == nil || payload.WhatChanged == nil {
		return
	}
	wc := payload.WhatChanged
	for i := range wc.Groups {
		group := &wc.Groups[i]
		if group.Key == whatChangedKeyImproved {
			rawLabel := strings.TrimSpace(group.Label)
			group.Label = resolveImprovedGroupLabel(*group)
			if rawLabel != group.Label {
				result.RepairActions = append(result.RepairActions, "what_changed_improved_label_repaired")
				result.Repaired = true
			}
		}
		for j := range group.Items {
			item := &group.Items[j]
			repaired := repairDuplicateWhatChangedLabelPrefix(item.Label, item.Text)
			if repaired != item.Text {
				item.Text = repaired
				result.RepairActions = append(result.RepairActions, "what_changed_item_"+itoa(i)+"_"+itoa(j)+"_dedupe_label_prefix")
				result.Repaired = true
			}
		}
	}
	summary := strings.TrimSpace(wc.Summary)
	main := strings.TrimSpace(wc.MainConclusion)
	if summary != "" && main != "" && strings.EqualFold(summary, main) {
		if grounded := deriveGroundedWhatChangedMainConclusion(wc); grounded != "" && !strings.EqualFold(grounded, summary) {
			wc.MainConclusion = grounded
		} else if tail := whatChangedPracticalTail(wc); tail != "" && !strings.EqualFold(tail, summary) {
			wc.MainConclusion = tail
		}
		result.RepairActions = append(result.RepairActions, "what_changed_main_conclusion_deduped_from_summary")
		result.Repaired = true
	}
	ensureWhatChangedSummaryDistinctFromMainConclusion(wc)
}

func repairAIOnlySellerFacingPhrases(payload *dashboardNarrativePayload, result *narrativeRepairResult) {
	if payload == nil {
		return
	}
	repairText := func(field string, value *string) {
		if value == nil || strings.TrimSpace(*value) == "" || !containsAISeesSellerFacingPhrase(*value) {
			return
		}
		*value = repairAISeesSellerFacingPhrase(*value)
		result.RepairActions = append(result.RepairActions, field+"_ai_sees_repaired")
		result.Repaired = true
	}
	repairText("headline", &payload.Headline)
	repairText("executive_summary", &payload.ExecutiveSummary)
	if payload.AccountHealth != nil {
		repairText("account_health.headline", &payload.AccountHealth.Headline)
		repairText("account_health.executive_summary", &payload.AccountHealth.ExecutiveSummary)
	}
	if payload.WhatChanged != nil {
		repairText("what_changed.summary", &payload.WhatChanged.Summary)
		repairText("what_changed.main_conclusion", &payload.WhatChanged.MainConclusion)
	}
}

func repairAdEfficiencyChecklistTitles(n *AdEfficiencyNarrative, result *narrativeRepairResult) {
	if n == nil {
		return
	}
	for i := range n.Checklist {
		item := &n.Checklist[i]
		title := strings.TrimSpace(item.Title)
		text := strings.TrimSpace(item.Text)
		if title == "" || text == "" {
			continue
		}
		if strings.EqualFold(title, text) {
			item.Title = deriveShortActionTitle(text)
			result.RepairActions = append(result.RepairActions, "ad_efficiency_checklist_"+itoa(i)+"_title_from_text")
			result.Repaired = true
		}
	}
}
