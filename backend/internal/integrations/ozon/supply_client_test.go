package ozon

import (
	"encoding/json"
	"testing"
)

func TestSupplyActItemKeepsIndependentUnitFacts(t *testing.T) {
	var item SupplyActItem
	if err := json.Unmarshal([]byte(`{"sku":101,"accepted":8,"shortage":1,"surplus":2,"offer_id":"o1","amount":"10.50","currency_code":"RUB"}`), &item); err != nil {
		t.Fatal(err)
	}
	if int64(item.SKU) != 101 || item.OfferID != "o1" {
		t.Fatalf("identity lost: %+v", item)
	}
	if item.Accepted == nil || *item.Accepted != 8 || item.Shortage == nil || *item.Shortage != 1 || item.Surplus == nil || *item.Surplus != 2 {
		t.Fatalf("units lost: %+v", item)
	}
	// Zero accepted is retained when explicitly present; not treated as missing.
	var zero SupplyActItem
	if err := json.Unmarshal([]byte(`{"sku":1,"accepted":0,"shortage":0}`), &zero); err != nil {
		t.Fatal(err)
	}
	if zero.Accepted == nil || *zero.Accepted != 0 || zero.Shortage == nil || *zero.Shortage != 0 {
		t.Fatalf("zero units must remain explicit: %+v", zero)
	}
	if zero.Surplus != nil {
		t.Fatal("missing surplus must stay nil, not zero-invented")
	}
}

func TestSupplyActItemDoesNotUseProductIDAsSKU(t *testing.T) {
	var item SupplyActItem
	if err := json.Unmarshal([]byte(`{"product_id":999,"offer_id":"x"}`), &item); err != nil {
		t.Fatal(err)
	}
	if int64(item.SKU) != 0 {
		t.Fatalf("product_id must not become sku: %+v", item)
	}
}
