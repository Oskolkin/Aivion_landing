package dashboardhome

import "fmt"

func forensicDashboardUIContract(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	report := RunDashboardUIContractValidation(in.DashboardHome)
	if len(report.Issues) == 0 {
		return nil
	}
	out := make([]DashboardContentForensicIssue, 0, len(report.Issues))
	for _, issue := range report.Issues {
		out = append(out, DashboardContentForensicIssue{
			ID:               issue.ID,
			Severity:         issue.Severity,
			Symptom:          issue.Message,
			RootCauseLayer:   "dashboard_apply_mapping",
			ResponsibleFiles: []string{"backend/internal/dashboardhome/seller_pulse_ui_contract.go", "backend/internal/dashboardhome/ui_contract_validation.go"},
			FixRequired:      issue.Severity == "critical",
			FixPlan:          issue.Message,
		})
	}
	return out
}

func forensicHealthBreakdownDriverMismatch(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	summary := in.DashboardHome.AIDiagnostics.AccountSummary
	if summary.HealthScoreBreakdown == nil || len(summary.RiskDrivers) == 0 {
		return nil
	}
	riskIDs := map[string]struct{}{}
	for _, d := range summary.RiskDrivers {
		riskIDs[d.ID] = struct{}{}
	}
	breakdownIDs := map[string]struct{}{}
	for _, d := range summary.HealthScoreBreakdown.Drivers {
		breakdownIDs[d.DriverID] = struct{}{}
	}
	for id := range breakdownIDs {
		if _, ok := riskIDs[id]; !ok {
			return []DashboardContentForensicIssue{{
				ID:       "health_breakdown_driver_mismatch",
				Severity: "warning",
				Symptom:  fmt.Sprintf("health_score_breakdown has driver %q not shown in risk_drivers", id),
				DashboardHomeValue: map[string]any{
					"risk_driver_ids":      driverIDs(summary.RiskDrivers),
					"breakdown_driver_ids": breakdownDriverIDs(summary.HealthScoreBreakdown.Drivers),
				},
				RootCauseLayer:   "dashboard_apply_mapping",
				ResponsibleFiles: []string{"backend/internal/dashboardhome/risk_drivers.go"},
				FixRequired:      true,
				FixPlan:          "Recompute health_score_breakdown from seller-facing risk_drivers after AI enrichment.",
			}}
		}
	}
	for id := range riskIDs {
		if _, ok := breakdownIDs[id]; !ok {
			return []DashboardContentForensicIssue{{
				ID:       "health_breakdown_driver_mismatch",
				Severity: "warning",
				Symptom:  fmt.Sprintf("risk_drivers has driver %q missing from health_score_breakdown", id),
				DashboardHomeValue: map[string]any{
					"risk_driver_ids":      driverIDs(summary.RiskDrivers),
					"breakdown_driver_ids": breakdownDriverIDs(summary.HealthScoreBreakdown.Drivers),
				},
				RootCauseLayer:   "dashboard_apply_mapping",
				ResponsibleFiles: []string{"backend/internal/dashboardhome/risk_drivers.go"},
				FixRequired:      true,
				FixPlan:          "Recompute health_score_breakdown from seller-facing risk_drivers after AI enrichment.",
			}}
		}
	}
	return nil
}

func forensicExcessiveSellerFacingFactIDs(in DashboardContentForensicsInput) []DashboardContentForensicIssue {
	if in.DashboardHome == nil {
		return nil
	}
	for _, d := range in.DashboardHome.AIDiagnostics.AccountSummary.RiskDrivers {
		if d.Evidence == nil {
			continue
		}
		raw, ok := d.Evidence["fact_ids"]
		if !ok {
			continue
		}
		count := 0
		switch ids := raw.(type) {
		case []string:
			count = len(ids)
		case []any:
			count = len(ids)
		}
		if count > maxSellerFacingFactIDs {
			return []DashboardContentForensicIssue{{
				ID:       "excessive_seller_facing_fact_ids",
				Severity: "warning",
				Symptom:  fmt.Sprintf("risk_driver %s exposes %d fact_ids in seller-facing response", d.ID, count),
				DashboardHomeValue: map[string]any{
					"driver_id": d.ID, "fact_ids_count": count,
				},
				RootCauseLayer:   "dashboard_apply_mapping",
				ResponsibleFiles: []string{"backend/internal/dashboardhome/risk_driver_evidence.go"},
				FixRequired:      true,
				FixPlan:          "Strip debug fact_ids from seller-facing evidence; keep grounding_fact_count and compact examples.",
			}}
		}
	}
	return nil
}

func driverIDs(drivers []RiskDriver) []string {
	out := make([]string, 0, len(drivers))
	for _, d := range drivers {
		out = append(out, d.ID)
	}
	return out
}

func breakdownDriverIDs(items []HealthScoreBreakdownItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.DriverID)
	}
	return out
}
