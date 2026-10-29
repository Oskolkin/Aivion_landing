package projection

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestStockKnownZeroIsAuthoritative(t *testing.T) {
	zero := int64(0)
	it := factualapi.SKUStock{CanonicalSKUID: 1, KnownSubtotalUnits: 40, SellableUnits: factualapi.Known(zero)}
	row := stockRowFromSKU(it, nil, nil, 1)
	if row["sellable_units"] != int64(0) {
		t.Fatalf("sellable=%v", row["sellable_units"])
	}
	if row["sellable_units_state"] != string(factualapi.ValueKnown) {
		t.Fatal(row["sellable_units_state"])
	}
	if row["coverage_band"] != bandOutOfStock {
		t.Fatalf("band=%v", row["coverage_band"])
	}
}

func TestStockKnownForty(t *testing.T) {
	n := int64(40)
	it := factualapi.SKUStock{CanonicalSKUID: 2, KnownSubtotalUnits: 40, SellableUnits: factualapi.Known(n)}
	row := stockRowFromSKU(it, nil, nil, 1)
	if row["sellable_units"] != int64(40) {
		t.Fatalf("sellable=%v", row["sellable_units"])
	}
	if row["sellable_units_state"] != string(factualapi.ValueKnown) {
		t.Fatal(row["sellable_units_state"])
	}
}

func TestStockUnknownDoesNotUseKnownSubtotal(t *testing.T) {
	it := factualapi.SKUStock{
		CanonicalSKUID:     3,
		KnownSubtotalUnits: 40,
		SellableUnits:      factualapi.Missing[int64](factualapi.ValueUnknown, "INVENTORY_POOL_OVERLAP_UNVERIFIED"),
	}
	row := stockRowFromSKU(it, nil, nil, 1)
	if _, ok := row["sellable_units"]; ok {
		t.Fatalf("sellable_units must be omitted, got %v", row["sellable_units"])
	}
	if row["sellable_units_state"] != string(factualapi.ValueUnknown) {
		t.Fatal(row["sellable_units_state"])
	}
	if row["sellable_units_reason"] != "INVENTORY_POOL_OVERLAP_UNVERIFIED" {
		t.Fatal(row["sellable_units_reason"])
	}
	if row["known_subtotal_units"] != int64(40) {
		t.Fatal(row["known_subtotal_units"])
	}
	if row["known_subtotal_is_authoritative"] != false {
		t.Fatal("subtotal must not be authoritative")
	}
	if row["coverage_band"] == bandOutOfStock {
		t.Fatal("UNKNOWN must not classify as out_of_stock")
	}
}

func TestStockUnknownSubtotalZeroNotAuthoritativeZero(t *testing.T) {
	it := factualapi.SKUStock{
		CanonicalSKUID:     4,
		KnownSubtotalUnits: 0,
		SellableUnits:      factualapi.Missing[int64](factualapi.ValueUnknown, "INVENTORY_POOL_OVERLAP_UNVERIFIED"),
	}
	row := stockRowFromSKU(it, nil, nil, 1)
	if _, ok := row["sellable_units"]; ok {
		t.Fatal("must not emit sellable_units")
	}
	if row["coverage_band"] == bandOutOfStock {
		t.Fatal("unknown+subtotal 0 is not authoritative zero")
	}
	raw, _ := json.Marshal(row)
	focus := map[string]any{}
	_ = json.Unmarshal(raw, &focus)
	if _, ok := focus["sellable_units"]; ok {
		t.Fatal("sku focus must not materialize sellable_units")
	}
}

func TestAdvertisingUnknownBudgetNotZero(t *testing.T) {
	c := factualapi.Campaign{ID: 9, Title: "X", Budget: factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, "BUDGET_UNAVAILABLE")}
	row := campaignRowFrom(c, nil)
	if _, ok := row["budget_amount"]; ok {
		t.Fatalf("fabricated budget %v", row["budget_amount"])
	}
	if row["budget_state"] != string(factualapi.ValueUnknown) {
		t.Fatal(row["budget_state"])
	}
}

func TestAdvertisingKnownZeroBudget(t *testing.T) {
	z, err := factualapi.ParseDecimal("0")
	if err != nil {
		t.Fatal(err)
	}
	c := factualapi.Campaign{ID: 8, Budget: factualapi.Known(factualapi.Money{Amount: z, CurrencyCode: "RUB"})}
	row := campaignRowFrom(c, nil)
	if row["budget_amount"] != "0" {
		t.Fatalf("budget=%v", row["budget_amount"])
	}
	if row["budget_state"] != string(factualapi.ValueKnown) {
		t.Fatal(row["budget_state"])
	}
}
