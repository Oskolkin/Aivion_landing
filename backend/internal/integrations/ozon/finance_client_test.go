package ozon

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestFinanceAccrualByDayDecodesExactParentAndNestedBreakdown(t *testing.T) {
	body := []byte(`{"accruals":[{"accrued_category":"SALE","date":"2026-08-25","item_fees":{"fees":[{"sku":9007199254740993,"fees":[{"accrued":{"amount":"-12.340000","currency":"RUB"},"type_id":77}]}]},"total_amount":{"amount":"123456789012345678.123456","currency":"RUB"},"unit_number":"u-1"}],"last_id":""}`)
	var result FinanceAccrualResult
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	row := result.Items[0]
	if string(row.Amount) != "123456789012345678.123456" || row.CurrencyCode != "RUB" {
		t.Fatalf("parent total lost exactness: %+v", row)
	}
	if len(row.Components) != 1 || string(row.Components[0].Amount) != "-12.340000" || int64(row.Components[0].OzonSKU) != 9007199254740993 {
		t.Fatalf("nested breakdown lost exactness or SKU: %+v", row.Components)
	}
}

func TestFinanceByDayRequestUsesProviderCursorShape(t *testing.T) {
	body, err := json.Marshal(FinanceAccrualRequest{DateFrom: "2026-08-25", DateTo: "2026-08-25", Cursor: "next"})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"date":"2026-08-25","last_id":"next"}` {
		t.Fatalf("request=%s", body)
	}
}
