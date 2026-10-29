package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
)

func TestBuildFunnelEvidenceSnapshot_flagsSignals(t *testing.T) {
	resp := &Response{
		AIDiagnostics: AIDiagnostics{
			AdEfficiency:   AdEfficiencyWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}, Points: []AdEfficiencyPoint{{CampaignExternalID: 1}}},
			StockCoverage:  StockCoverageWidget{WidgetBase: WidgetBase{Status: WidgetAvailable}, TotalSKUs: 2},
			SalesFunnel:    SalesFunnelWidget{},
			FunnelAnalysis: FunnelAnalysisWidget{},
		},
	}
	dto := analytics.DashboardMetricsDTO{
		Account: analytics.DashboardAccountMetrics{CancelsToday: 3},
	}
	ev := buildFunnelEvidenceSnapshot(resp, dto)
	if !ev.HasAdvertising || !ev.HasStock || !ev.HasCancellations {
		t.Fatalf("unexpected snapshot: %+v", ev)
	}
}
