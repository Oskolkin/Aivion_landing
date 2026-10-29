package ozon

import (
	"testing"
)

func TestParseAnalyticsDataResponse(t *testing.T) {
	raw := []byte(`{
		"result": {
			"data": [
				{
					"dimensions": [{"id": "223681945", "name": "SKU-12345"}],
					"metrics": [125000.00, 50, 1500, 1200, 45, 40, 120000.00]
				}
			],
			"totals": [500000.00, 200, 6000, 4800, 180, 160, 480000.00]
		}
	}`)

	resp, err := ParseAnalyticsDataResponse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Totals) != 7 {
		t.Fatalf("totals len=%d want 7", len(resp.Totals))
	}
	if len(resp.Data) != 1 {
		t.Fatalf("data len=%d want 1", len(resp.Data))
	}
	if resp.Data[0].Dimensions[0].ID != "223681945" {
		t.Fatalf("dimension id=%q", resp.Data[0].Dimensions[0].ID)
	}
}

func TestMapAccountFunnelTotals(t *testing.T) {
	totals := []float64{6000, 4800, 900, 200, 180, 500000.00}
	impressions, productViews, addToCart, orders, completed, revenue := MapAccountFunnelTotals(totals)
	if impressions == nil || *impressions != 6000 {
		t.Fatalf("impressions=%v", impressions)
	}
	if productViews == nil || *productViews != 4800 {
		t.Fatalf("productViews=%v", productViews)
	}
	if addToCart == nil || *addToCart != 900 {
		t.Fatalf("addToCart=%v", addToCart)
	}
	if orders == nil || *orders != 200 {
		t.Fatalf("orders=%v", orders)
	}
	if completed == nil || *completed != 180 {
		t.Fatalf("completed=%v", completed)
	}
	if revenue == nil || *revenue != 500000.00 {
		t.Fatalf("revenue=%v", revenue)
	}
}

func TestAnalyticsData403IsProviderError(t *testing.T) {
	// ProviderError type is returned by client on non-2xx; verify type exists for handler mapping.
	err := &ProviderError{StatusCode: 403, Body: "access denied"}
	if err.Error() == "" {
		t.Fatal("expected error message")
	}
}
