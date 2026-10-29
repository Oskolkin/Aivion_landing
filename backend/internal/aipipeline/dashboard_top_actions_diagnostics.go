package aipipeline

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func populateFinalTopActionsDiagnostics(diag *StepDiagnostics, actRes *actions.GenerateActionsResult, dashboardOutcome *DashboardRunOutcome) {
	if diag == nil {
		return
	}
	if actRes != nil && actRes.AcceptedCount > 0 && len(actRes.TopActionKeys) > 0 {
		topSet := map[string]struct{}{}
		for _, key := range actRes.TopActionKeys {
			topSet[key] = struct{}{}
		}
		finalDiag := dashboardhome.TopActionsDiagnostics{Count: len(actRes.TopActionKeys)}
		for _, item := range actRes.Actions {
			if _, ok := topSet[item.ActionKey]; !ok {
				continue
			}
			if len(item.EvidenceFactIDs) == 0 {
				finalDiag.MissingFactIDsCount++
			}
		}
		diag.FinalTopActionsSource = "actions_override"
		diag.FinalTopActionsCount = finalDiag.Count
		diag.FinalTopActionsMissingFactIDsCount = finalDiag.MissingFactIDsCount
		return
	}
	if dashboardOutcome != nil && dashboardOutcome.Result != nil {
		diag.FinalTopActionsSource = "dashboard_ai"
		diag.FinalTopActionsCount = dashboardOutcome.Result.DashboardPreOverrideTopActionsCount
		diag.FinalTopActionsMissingFactIDsCount = dashboardOutcome.Result.DashboardPreOverrideTopActionsMissingFactIDsCount
		return
	}
	diag.FinalTopActionsSource = "none"
}
