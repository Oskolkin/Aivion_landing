package dashboardhome

func buildFunnelStageDayOverDayFromStages(stages []FunnelStage) []map[string]any {
	out := make([]map[string]any, 0, len(stages))
	for _, s := range stages {
		if s.Value == nil {
			continue
		}
		item := map[string]any{
			"stage_key":     s.Key,
			"stage_label":   s.Label,
			"current_value": *s.Value,
		}
		if s.PreviousDayValue != nil {
			item["previous_day_value"] = *s.PreviousDayValue
		}
		if s.ChangeVsPreviousDayPct != nil {
			item["change_vs_previous_day_pct"] = *s.ChangeVsPreviousDayPct
		}
		if s.ConversionFromPrevious != nil {
			item["conversion_from_previous_stage_pct"] = *s.ConversionFromPrevious
		}
		out = append(out, item)
	}
	return out
}

func funnelStageDayOverDayAvailable(stages []FunnelStage) bool {
	for _, s := range stages {
		if s.PreviousDayValue != nil || s.ChangeVsPreviousDayPct != nil {
			return true
		}
	}
	return false
}

func enrichFunnelContextWithDayOverDay(funnelCtx map[string]any, stages []FunnelStage) {
	if funnelCtx == nil || len(stages) == 0 {
		return
	}
	comparisons := buildFunnelStageDayOverDayFromStages(stages)
	if len(comparisons) == 0 {
		return
	}
	funnelCtx["stage_day_over_day"] = comparisons
	funnelCtx["comparison_available"] = funnelStageDayOverDayAvailable(stages)
	funnelCtx["comparison_rule"] = "stage_day_over_day contains neutral day-over-day facts. AI must use them for weakest_stage and largest_deterioration_stage; do not claim prior-day comparison is unavailable."
}
