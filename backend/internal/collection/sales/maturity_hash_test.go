package sales

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizedDayHashCoversFullSKUDistribution(t *testing.T) {
	day := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	amount := func(v string) Decimal {
		d, e := ParseAmount(json.Number(v))
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	account := accountCandidate{Date: day, Amount: amount("10"), Units: 10}
	a := []candidate{{SourceSKU: 1, Date: day, Amount: amount("5"), Units: 5}, {SourceSKU: 2, Date: day, Amount: amount("5"), Units: 5}}
	b := []candidate{{SourceSKU: 1, Date: day, Amount: amount("8"), Units: 8}, {SourceSKU: 2, Date: day, Amount: amount("2"), Units: 2}}
	resolved := map[string]bool{skuKey(a[0]): true, skuKey(a[1]): true}
	ha := normalizedDayHash(1, "2026-08-24", "RUB", account, true, a, resolved)
	hb := normalizedDayHash(1, "2026-08-24", "RUB", account, true, b, resolved)
	if ha == hb {
		t.Fatal("equal account totals hid different SKU distributions")
	}
	if ha != normalizedDayHash(1, "2026-08-24", "RUB", account, true, a, resolved) {
		t.Fatal("normalized day hash is not deterministic")
	}
}
