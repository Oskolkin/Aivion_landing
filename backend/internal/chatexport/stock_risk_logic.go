package chatexport

// Stock risk classification mirrors chat tool_data_repository_sqlc (all SKUs for as_of, not critical-only).

func classifyStockDepletionRisk(stock int32, days *float64) string {
	switch {
	case stock <= 0:
		return "critical_out_of_stock"
	case days != nil && *days <= 3:
		return "high_depletion_risk"
	case stock <= 3 || (days != nil && *days <= 7):
		return "medium_depletion_risk"
	default:
		return "low_depletion_risk"
	}
}

func stockRiskSeverity(depletionRisk string) string {
	switch depletionRisk {
	case "critical_out_of_stock", "high_depletion_risk":
		return "critical"
	case "medium_depletion_risk":
		return "high"
	default:
		return "medium"
	}
}

func stockRiskReasonText(stock int32, days *float64, depletionRisk string) string {
	switch depletionRisk {
	case "critical_out_of_stock":
		return "current stock is zero"
	case "high_depletion_risk":
		return "days_of_cover is below 3"
	case "medium_depletion_risk":
		if stock <= 3 {
			return "available stock is below threshold"
		}
		return "days_of_cover is below 7"
	default:
		return depletionRisk
	}
}

func isExportableStockRisk(depletionRisk string) bool {
	return depletionRisk != "low_depletion_risk"
}

func stockRiskRank(depletionRisk string) int {
	switch depletionRisk {
	case "critical_out_of_stock":
		return 4
	case "high_depletion_risk":
		return 3
	case "medium_depletion_risk":
		return 2
	default:
		return 0
	}
}
