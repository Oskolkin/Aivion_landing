package dashboardhome

import "testing"

func TestReconcileUnifiedPublicationStatus_DoesNotForceReadyOnEmptyKPIs(t *testing.T) {
	resp := &Response{
		Status: StatusReady,
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{
				Headline: "Something",
			},
		},
		SellerPulse: SellerPulse{KPIs: nil, Trend7D: nil},
	}
	reconcileUnifiedPublicationStatus(resp)
	if resp.Status != StatusPartial {
		t.Fatalf("status=%q want partial when headline exists but KPIs empty", resp.Status)
	}

	resp2 := &Response{
		Status:      StatusReady,
		SellerPulse: SellerPulse{KPIs: nil, Trend7D: nil},
	}
	reconcileUnifiedPublicationStatus(resp2)
	if resp2.Status != StatusNotEnoughData {
		t.Fatalf("status=%q want not_enough_data when empty factual layer", resp2.Status)
	}

	resp3 := &Response{
		Status: StatusPartial,
		SellerPulse: SellerPulse{
			KPIs: []KPICard{
				{Key: "revenue", Value: "1 000 ₽"},
				{Key: "orders", Value: "12"},
			},
			Trend7D: []TrendDayPoint{{Date: "2026-07-08"}},
		},
		AIDiagnostics: AIDiagnostics{
			AccountSummary: AccountSummaryWidget{Headline: "ok"},
		},
	}
	reconcileUnifiedPublicationStatus(resp3)
	if resp3.Status != StatusPartial {
		t.Fatalf("status=%q want keep partial", resp3.Status)
	}

	resp4 := &Response{
		Status: StatusNotEnoughData,
		SellerPulse: SellerPulse{
			KPIs: []KPICard{
				{Key: "revenue", Value: "1 000 ₽"},
			},
			Trend7D: []TrendDayPoint{{Date: "2026-07-08"}},
		},
	}
	reconcileUnifiedPublicationStatus(resp4)
	if resp4.Status != StatusReady {
		t.Fatalf("status=%q want ready with KPIs+trend", resp4.Status)
	}
}
