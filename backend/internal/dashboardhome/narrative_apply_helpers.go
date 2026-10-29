package dashboardhome

import (
	"fmt"
	"strings"
)

func reconcileRiskDistributionMessage(resp *Response, drivers []RiskDriver) {
	if resp == nil || len(drivers) == 0 {
		return
	}
	rd := resp.AIDiagnostics.RiskDistribution
	if rd.Total > 0 {
		return
	}
	msg := strings.TrimSpace(rd.Message)
	if !noRisksMessagePattern.MatchString(msg) {
		return
	}
	rd.Message = "Нет данных для распределения по legacy-алертам — см. risk_drivers в account_summary."
	resp.AIDiagnostics.RiskDistribution = rd
}

func reconcileSurplusStockCoverageLabels(resp *Response) {
	if resp == nil {
		return
	}
	slowRisk := len(resp.SKUFocus.Surplus)
	for i := range resp.AIDiagnostics.StockCoverage.Segments {
		if resp.AIDiagnostics.StockCoverage.Segments[i].Key == "surplus" {
			resp.AIDiagnostics.StockCoverage.Segments[i].Label = "SURPLUS (Ozon оборачиваемость)"
			break
		}
	}
	if slowRisk > 0 && strings.TrimSpace(resp.AIDiagnostics.StockCoverage.Message) == "" {
		resp.AIDiagnostics.StockCoverage.Message = fmt.Sprintf(
			"Медленная оборачиваемость / риск хранения: %d SKU — см. sku_focus.surplus.",
			slowRisk,
		)
	}
}

func reconcileSellerPulseRisksKPI(resp *Response) {
	if resp == nil || !dashboardAIOnlyEnabled() {
		return
	}
	if resp.AIDiagnostics.AccountSummary.NarrativeSource != "ai" {
		return
	}
	critHigh := countCriticalHighRiskDrivers(resp.AIDiagnostics.AccountSummary.RiskDrivers)
	for i := range resp.SellerPulse.KPIs {
		if resp.SellerPulse.KPIs[i].Key != "risks" {
			continue
		}
		resp.SellerPulse.KPIs[i].Label = "AI-риски"
		resp.SellerPulse.KPIs[i].Value = formatInt(critHigh)
		resp.SellerPulse.KPIs[i].Hint = "По AI-диагностике главной страницы (critical/high)"
	}
}

func recommendedActionsFromTopActions(items []TopActionItem) []ActionItem {
	out := make([]ActionItem, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		out = append(out, ActionItem{
			Priority:       item.Priority,
			UrgencyGroup:   urgencyGroupForPriority(item.Priority),
			Action:         item.Title,
			Reason:         item.Reason,
			ExpectedEffect: item.ExpectedEffect,
			Source:         "dashboard_narrative",
		})
	}
	return out
}
