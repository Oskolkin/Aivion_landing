package materiality

import (
	"math/big"
	"testing"
)

func TestMissingNotZero_FROM_ZERO(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "known-from-zero", CategorySemanticKey: "c", SalesAvailable: true,
				OrderedAmountPrevious7: exZero(), OrderedAmountCurrent7: ex(500_000)},
			{SemanticKey: "missing-previous", CategorySemanticKey: "c", SalesAvailable: true,
				OrderedAmountPrevious7: exUnknown(), OrderedAmountCurrent7: ex(500_000)},
		},
		Capabilities: baseCapabilities(),
	}
	from := AssignRanks(skuSalesFromZero(core), DirectionDesc)
	for _, r := range from {
		if r.SemanticKey == "missing-previous" {
			t.Fatalf("UNKNOWN previous must not qualify for FROM_ZERO: %#v", from)
		}
	}
	if len(from) != 1 || from[0].SemanticKey != "known-from-zero" {
		t.Fatalf("FROM_ZERO must only include KNOWN zero previous: got %#v", from)
	}
}

func TestMissingNotZero_TO_ZERO(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000)},
		SKUs: []SKUFeatures{
			{SemanticKey: "known-to-zero", CategorySemanticKey: "c", SalesAvailable: true,
				OrderedAmountPrevious7: ex(700_000), OrderedAmountCurrent7: exZero()},
			{SemanticKey: "missing-current", CategorySemanticKey: "c", SalesAvailable: true,
				OrderedAmountPrevious7: ex(700_000), OrderedAmountCurrent7: exUnknown()},
		},
		Capabilities: baseCapabilities(),
	}
	to := AssignRanks(skuSalesToZero(core), DirectionDesc)
	for _, r := range to {
		if r.SemanticKey == "missing-current" {
			t.Fatalf("UNKNOWN current must not qualify for TO_ZERO: %#v", to)
		}
	}
	if len(to) != 1 || to[0].SemanticKey != "known-to-zero" {
		t.Fatalf("TO_ZERO must only include KNOWN zero current: got %#v", to)
	}
}

func TestParseInvalidDecimalFails(t *testing.T) {
	if _, err := parseRat(Exact("not-a-number")); err == nil {
		t.Fatal("invalid decimal must fail parseRat")
	}
}

func TestParseEmptyDecimalFails(t *testing.T) {
	if _, err := parseRat(Exact("")); err == nil {
		t.Fatal("empty exact must fail parseRat")
	}
	if r, err := parseRat(Exact("12.5")); err != nil {
		t.Fatalf("valid decimal: r=%v err=%v", r, err)
	} else if r.Cmp(mustParseRat(Exact("12.5"))) != 0 {
		t.Fatalf("valid decimal mismatch")
	}
}

func mustParseRat(d Exact) *big.Rat {
	r, err := parseRat(d)
	if err != nil {
		panic(err)
	}
	return r
}
