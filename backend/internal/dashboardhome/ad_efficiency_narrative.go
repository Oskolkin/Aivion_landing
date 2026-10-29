package dashboardhome

import (
	"strings"
)

var adEfficiencyAllowedChecklistTargets = map[string]struct{}{
	"high_drr_campaigns":    {},
	"no_order_campaigns":    {},
	"advertising_campaigns": {},
}

var adEfficiencySKULevelPatterns = []string{
	"sku сливает",
	"расход по sku",
	"рекламный p&l sku",
	"sku неэффектив",
	"по sku",
	"sku-level",
	"sku level",
}

type adEfficiencyNarrativeValidationContext struct {
	WidgetStatus         WidgetStatus
	AllowedCampaignNames map[string]struct{}
	HighDRRCampaignNames map[string]struct{}
	NoOrderCampaignNames map[string]struct{}
	DeltaVsPreviousPP    *float64
	CampaignLevelOnly    bool
	HasFactualData       bool
}

func buildAdEfficiencyNarrativeValidationContext(resp *Response) adEfficiencyNarrativeValidationContext {
	ctx := adEfficiencyNarrativeValidationContext{
		WidgetStatus:         resp.AIDiagnostics.AdEfficiency.Status,
		AllowedCampaignNames: make(map[string]struct{}),
		HighDRRCampaignNames: make(map[string]struct{}),
		NoOrderCampaignNames: make(map[string]struct{}),
		CampaignLevelOnly:    true,
		HasFactualData: resp.AIDiagnostics.AdEfficiency.Status == WidgetAvailable ||
			resp.AIDiagnostics.AdEfficiency.Status == WidgetPartial,
	}
	addName := func(names map[string]struct{}, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			names[value] = struct{}{}
			ctx.AllowedCampaignNames[value] = struct{}{}
		}
	}
	for _, row := range resp.AIDiagnostics.AdEfficiency.HighDRRCampaigns {
		addName(ctx.HighDRRCampaignNames, row.CampaignName)
	}
	for _, row := range resp.AIDiagnostics.AdEfficiency.NoOrderCampaigns {
		addName(ctx.NoOrderCampaignNames, row.CampaignName)
	}
	if resp.AIDiagnostics.AdEfficiency.Summary != nil {
		ctx.DeltaVsPreviousPP = resp.AIDiagnostics.AdEfficiency.Summary.DeltaVsPreviousPp
	}
	for _, point := range resp.AIDiagnostics.AdEfficiency.Points {
		addName(ctx.AllowedCampaignNames, point.CampaignName)
	}
	return ctx
}

func buildAdEfficiencyNarrativeContextMap(resp *Response) map[string]any {
	w := resp.AIDiagnostics.AdEfficiency
	highDRR := make([]map[string]any, 0, len(w.HighDRRCampaigns))
	for _, row := range w.HighDRRCampaigns {
		item := map[string]any{
			"campaign_name": row.CampaignName,
			"spend":         row.Spend,
			"orders":        row.Orders,
			"revenue":       row.Revenue,
		}
		if row.DRRPct != nil {
			item["drr_pct"] = *row.DRRPct
		}
		highDRR = append(highDRR, item)
	}
	noOrder := make([]map[string]any, 0, len(w.NoOrderCampaigns))
	for _, row := range w.NoOrderCampaigns {
		noOrder = append(noOrder, map[string]any{
			"campaign_name": row.CampaignName,
			"spend":         row.Spend,
			"orders":        row.Orders,
			"revenue":       row.Revenue,
		})
	}
	out := map[string]any{
		"status":             w.Status,
		"high_drr_campaigns": highDRR,
		"no_order_campaigns": noOrder,
		"limitations":        []string{"campaign_level_only"},
	}
	if w.Period != nil {
		out["metric_date"] = w.Period.MetricDate
		out["sample_days"] = w.Period.SampleDays
	}
	if w.Summary != nil {
		out["campaigns_high_drr_count"] = w.Summary.CampaignsHighDRRCount
		out["campaigns_no_orders_count"] = w.Summary.CampaignsNoOrdersCount
		out["spend_without_orders"] = w.Summary.SpendWithoutOrders
		out["spend_without_orders_date_from"] = w.Summary.SpendWithoutOrdersDateFrom
		out["spend_without_orders_date_to"] = w.Summary.SpendWithoutOrdersDateTo
		if w.Summary.CurrentDRRPct != nil {
			out["current_drr_pct"] = *w.Summary.CurrentDRRPct
		}
		if w.Summary.DeltaVsPreviousPp != nil {
			out["delta_vs_previous_pp"] = *w.Summary.DeltaVsPreviousPp
		}
		if w.Summary.DeltaVsAverage7dPp != nil {
			out["delta_vs_average_7d_pp"] = *w.Summary.DeltaVsAverage7dPp
		}
	}
	return map[string]any{"ad_efficiency": out}
}

func applyAdEfficiencyNarrativeToResponse(
	resp *Response,
	narrativeApplied bool,
	payload *dashboardNarrativePayload,
) {
	widget := &resp.AIDiagnostics.AdEfficiency
	ctx := buildAdEfficiencyNarrativeValidationContext(resp)

	switch widget.Status {
	case WidgetNotEnoughData, WidgetSourceNotAvailable:
		widget.Narrative = &AdEfficiencyNarrative{Status: "not_enough_data"}
		return
	case WidgetError:
		widget.Narrative = &AdEfficiencyNarrative{Status: "unavailable"}
		return
	}

	if narrativeApplied && payload != nil && payload.AdEfficiencyNarrative != nil {
		n := cloneAdEfficiencyNarrative(payload.AdEfficiencyNarrative)
		repairAdEfficiencyNarrative(n)
		if errs := validateAdEfficiencyNarrativePayload(n, ctx); len(errs) == 0 {
			n.Status = "available"
			widget.Narrative = n
			return
		}
		widget.Narrative = &AdEfficiencyNarrative{Status: "partial"}
		return
	}

	status := "not_available"
	reason := "AI-блок рекламной эффективности не был сформирован в dashboard narrative."
	if !narrativeApplied {
		status = "not_available"
		reason = "Dashboard narrative не применён к рекламному блоку."
	}
	widget.Narrative = &AdEfficiencyNarrative{Status: status, Summary: reason}
}

func cloneAdEfficiencyNarrative(src *AdEfficiencyNarrative) *AdEfficiencyNarrative {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Checklist = append([]AdEfficiencyCheckItem(nil), src.Checklist...)
	dst.Limitations = append([]string(nil), src.Limitations...)
	dst.MetricClaims = append([]AdEfficiencyMetricClaim(nil), src.MetricClaims...)
	dst.ThresholdClaims = append([]AdEfficiencyThresholdClaim(nil), src.ThresholdClaims...)
	return &dst
}

func repairAdEfficiencyNarrative(n *AdEfficiencyNarrative) {
	if n == nil {
		return
	}
	n.Summary = cleanupSellerFacingText(n.Summary)
	n.Limitations = compactStringSlice(n.Limitations)
	repaired := make([]AdEfficiencyCheckItem, 0, len(n.Checklist))
	for _, item := range n.Checklist {
		item.Title = cleanupSellerFacingText(item.Title)
		item.Text = cleanupSellerFacingText(item.Text)
		item.Priority = strings.ToLower(strings.TrimSpace(item.Priority))
		item.Target = strings.ToLower(strings.TrimSpace(item.Target))
		if containsAdEfficiencySKULevelLanguage(item.Title, item.Text) {
			continue
		}
		if isTooGenericNarrativeText(item.Title) || isTooGenericNarrativeText(item.Text) {
			continue
		}
		if _, ok := adEfficiencyAllowedChecklistTargets[item.Target]; item.Target != "" && !ok {
			item.Target = ""
		}
		repaired = append(repaired, item)
	}
	n.Checklist = repaired
}

func validateAdEfficiencyNarrativePayload(n *AdEfficiencyNarrative, ctx adEfficiencyNarrativeValidationContext) []string {
	if n == nil {
		return []string{"nil_ad_efficiency_narrative"}
	}
	var errs []string
	if !isAdEfficiencySummarySubstantive(n.Summary) {
		errs = append(errs, "ad_efficiency_summary_not_substantive")
	}
	if hasAdEfficiencyDirectionalConflict(n.Summary, ctx.DeltaVsPreviousPP) {
		errs = append(errs, "ad_efficiency_summary_direction_conflict")
	}
	for i, item := range n.Checklist {
		if strings.TrimSpace(item.Title) == "" {
			errs = append(errs, "ad_efficiency_checklist_empty_title")
			continue
		}
		if isTooGenericNarrativeText(item.Title) || isTooGenericNarrativeText(item.Text) {
			errs = append(errs, "ad_efficiency_checklist_too_generic")
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Title), strings.TrimSpace(item.Text)) {
			errs = append(errs, "ad_efficiency_checklist_title_equals_text")
			continue
		}
		if containsAdEfficiencySKULevelLanguage(item.Title, item.Text) {
			errs = append(errs, "ad_efficiency_checklist_sku_level_language")
			continue
		}
		if item.Target != "" {
			if _, ok := adEfficiencyAllowedChecklistTargets[item.Target]; !ok {
				errs = append(errs, "ad_efficiency_checklist_unknown_target")
				_ = i
			}
		}
	}
	for _, limitation := range n.Limitations {
		if containsAdEfficiencySKULevelLanguage(limitation) {
			errs = append(errs, "ad_efficiency_limitation_sku_level_language")
		}
	}
	return errs
}

func hasAdEfficiencyDirectionalConflict(summary string, deltaVsPrevious *float64) bool {
	if deltaVsPrevious == nil {
		return false
	}
	s := strings.ToLower(strings.TrimSpace(summary))
	if s == "" {
		return false
	}
	hasGrowthWords := strings.Contains(s, "рост дrr") ||
		strings.Contains(s, "рост drr") ||
		strings.Contains(s, "рост дрр") ||
		strings.Contains(s, "дrr вырос") ||
		strings.Contains(s, "drr вырос") ||
		strings.Contains(s, "дрр вырос") ||
		strings.Contains(s, "дrr раст") ||
		strings.Contains(s, "drr раст") ||
		strings.Contains(s, "дрр раст") ||
		strings.Contains(s, "рост д р р") ||
		strings.Contains(s, "дrr повыс") ||
		strings.Contains(s, "drr повыс") ||
		strings.Contains(s, "дрр повыс") ||
		strings.Contains(s, "ухудшение дrr") ||
		strings.Contains(s, "ухудшение drr") ||
		strings.Contains(s, "ухудшение дрр")
	hasDeclineWords := strings.Contains(s, "дrr сниз") ||
		strings.Contains(s, "drr сниз") ||
		strings.Contains(s, "дрр сниз") ||
		strings.Contains(s, "снижение дrr") ||
		strings.Contains(s, "снижение drr") ||
		strings.Contains(s, "снижение дрр") ||
		strings.Contains(s, "дrr упал") ||
		strings.Contains(s, "drr упал") ||
		strings.Contains(s, "дрр упал") ||
		strings.Contains(s, "снижение д р р") ||
		strings.Contains(s, "дrr уменьш") ||
		strings.Contains(s, "drr уменьш") ||
		strings.Contains(s, "дрр уменьш") ||
		strings.Contains(s, "улучшение дrr") ||
		strings.Contains(s, "улучшение drr") ||
		strings.Contains(s, "улучшение дрр")

	if *deltaVsPrevious < 0 && hasGrowthWords {
		return true
	}
	if *deltaVsPrevious > 0 && hasDeclineWords {
		return true
	}
	return false
}

func isAdEfficiencySummarySubstantive(summary string) bool {
	s := strings.TrimSpace(summary)
	if s == "" {
		return false
	}
	return !isTooGenericNarrativeText(s)
}

func containsAdEfficiencySKULevelLanguage(values ...string) bool {
	for _, value := range values {
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "" {
			continue
		}
		for _, pattern := range adEfficiencySKULevelPatterns {
			if strings.Contains(lower, pattern) {
				return true
			}
		}
	}
	return false
}

func normalizeAdEfficiencyNarrativePayload(n *AdEfficiencyNarrative) {
	if n == nil {
		return
	}
	repairAdEfficiencyNarrative(n)
}
