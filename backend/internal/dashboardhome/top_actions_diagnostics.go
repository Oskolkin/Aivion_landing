package dashboardhome

// TopActionsDiagnostics captures factual grounding checks for dashboard Top 3 observability.
type TopActionsDiagnostics struct {
	Count               int
	MissingFactIDsCount int
}

func dashboardPreOverrideTopActionsDiagnostics(payload *dashboardNarrativePayload) TopActionsDiagnostics {
	if payload == nil {
		return TopActionsDiagnostics{}
	}
	diag := TopActionsDiagnostics{Count: len(payload.TopActions)}
	for _, action := range payload.TopActions {
		if len(action.FactIDs) == 0 {
			diag.MissingFactIDsCount++
		}
	}
	return diag
}

func dashboardFinalTopActionsDiagnostics(items []TopActionItem) TopActionsDiagnostics {
	diag := TopActionsDiagnostics{Count: len(items)}
	for _, item := range items {
		if len(item.SupportingMetrics) == 0 {
			diag.MissingFactIDsCount++
		}
	}
	return diag
}

func applyPreOverrideTopActionsDiagnostics(result *NarrativeGenerationResult, payload *dashboardNarrativePayload) {
	if result == nil {
		return
	}
	diag := dashboardPreOverrideTopActionsDiagnostics(payload)
	result.DashboardPreOverrideTopActionsCount = diag.Count
	result.DashboardPreOverrideTopActionsMissingFactIDsCount = diag.MissingFactIDsCount
}
