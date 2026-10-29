package dashboardhome

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

// FunnelEvidenceSnapshot flags deterministic dashboard signals used only for
// guardrails (e.g. dropping influence_factors without supporting data). It is
// not used to author seller-facing funnel analytics text.
type FunnelEvidenceSnapshot struct {
	HasAdvertising   bool
	HasStock         bool
	HasCancellations bool
}

func buildFunnelEvidenceSnapshot(resp *Response, dto analytics.DashboardMetricsDTO) FunnelEvidenceSnapshot {
	var ev FunnelEvidenceSnapshot
	if resp.AIDiagnostics.AdEfficiency.Status == WidgetAvailable && len(resp.AIDiagnostics.AdEfficiency.Points) > 0 {
		ev.HasAdvertising = true
	}
	if resp.AIDiagnostics.StockCoverage.Status == WidgetAvailable && resp.AIDiagnostics.StockCoverage.TotalSKUs > 0 {
		ev.HasStock = true
	}
	if dto.Account.CancelsToday > 0 {
		ev.HasCancellations = true
	} else if dto.Account.CancellationRate != nil && *dto.Account.CancellationRate > 0 {
		ev.HasCancellations = true
	}
	return ev
}
