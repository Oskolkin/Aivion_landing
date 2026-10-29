package performance

import (
	"encoding/json"
	"testing"
)

func TestMapCampaignProductsRequiresVerifiedSKUField(t *testing.T) {
	withSKU := mapCampaignProducts(1, []map[string]any{{"sku": json.Number("111"), "id": json.Number("999")}})
	if len(withSKU) != 1 || withSKU[0].SKU != 111 {
		t.Fatalf("sku field must win: %+v", withSKU)
	}
	idOnly := mapCampaignProducts(1, []map[string]any{{"id": json.Number("111")}})
	if len(idOnly) != 0 {
		t.Fatalf("arbitrary id must not become SKU: %+v", idOnly)
	}
}

func TestMapCampaignProductStatisticsSeparatesModelOrders(t *testing.T) {
	items := mapCampaignProductStatistics([]map[string]any{{
		"campaignId":  json.Number("11"),
		"sku":         json.Number("101"),
		"date":        "2026-08-24",
		"orders":      json.Number("5"),
		"modelOrders": json.Number("2"),
		"viewOrders":  json.Number("30"),
		"sales":       "100",
	}})
	if len(items) != 3 {
		t.Fatalf("expected DEFAULT+MODEL+VIEW, got %d %+v", len(items), items)
	}
	byKind := map[string]CampaignProductDailyMetric{}
	for _, item := range items {
		byKind[item.AttributionKind] = item
	}
	if byKind["PROVIDER_DEFAULT"].OrdersCount != 5 || byKind["MODEL"].OrdersCount != 2 || byKind["VIEW"].OrdersCount != 30 {
		t.Fatalf("kinds mislabelled: %+v", byKind)
	}
	if byKind["MODEL"].OrdersCount == 5 {
		t.Fatal("modelOrders must not relabel orders value")
	}
}

func TestParseCPOOrdersReportPreservesExactJSONNumber(t *testing.T) {
	body := []byte(`[{"campaignId":123456789012345,"sku":111,"orderedSku":222,"orderId":987654321012345,"date":"2026-08-24","sales":123456789012345.123456}]`)
	rows, err := ParseCPOOrdersReport(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	got, err := ExactNumberString(rows[0]["sales"])
	if err != nil {
		t.Fatal(err)
	}
	if got != "123456789012345.123456" {
		t.Fatalf("amount=%q", got)
	}
	id, err := ExactNumberString(rows[0]["campaignId"])
	if err != nil || id != "123456789012345" {
		t.Fatalf("campaign id=%q err=%v", id, err)
	}
}
