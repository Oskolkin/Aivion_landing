package dashboardhome

import "strings"

func applyAIOnlyPayloadToResponse(resp *Response, payload *dashboardNarrativePayload, narrativeApplied bool) {
	if !dashboardAIOnlyEnabled() || resp == nil || !narrativeApplied || payload == nil {
		return
	}
	if payload.SellerPulseNarrative != nil {
		p := cloneSellerPulseNarrative(payload.SellerPulseNarrative)
		p.Status = "available"
		p.DataQualityNote = ""
		resp.SellerPulse.Narrative = p
	}
	if payload.FunnelAIAnalysis != nil {
		ai := *payload.FunnelAIAnalysis
		ai.Status = WidgetAvailable
		resp.AIDiagnostics.FunnelAnalysis.AIAnalysis = ai
	}
	if len(payload.CategorySummaries) > 0 {
		resp.AIDiagnostics.CategorySummaries = normalizeCategorySummaryCardsAIOnly(
			append([]CategorySummaryCard(nil), payload.CategorySummaries...),
		)
	}
	if payload.RiskOverview != nil {
		overview := *payload.RiskOverview
		overview.Status = WidgetAvailable
		resp.AIDiagnostics.RiskDistribution = overview
	}
}

func aiOnlyRecommendedActionsFromPayload(payload *dashboardNarrativePayload) []ActionItem {
	if payload == nil || len(payload.RecommendedActions) == 0 {
		return nil
	}
	out := make([]ActionItem, 0, len(payload.RecommendedActions))
	for _, item := range payload.RecommendedActions {
		a := item
		a.Source = firstNonEmpty(strings.TrimSpace(a.Source), "dashboard_narrative_ai_only")
		a.UrgencyGroup = firstNonEmpty(strings.TrimSpace(a.UrgencyGroup), urgencyGroupForPriority(a.Priority))
		out = append(out, a)
	}
	return out
}
