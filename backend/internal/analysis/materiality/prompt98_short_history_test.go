package materiality

import "testing"

func TestPrompt98LessThanSevenDaysNoFabricatedSalesLanes(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct"},
		SKUs: []SKUFeatures{{
			SemanticKey: "sku-short", CategorySemanticKey: "c", SalesAvailable: true,
			OrderedAmountCurrent7:  exUnknown(),
			OrderedAmountPrevious7: exUnknown(),
		}},
		Capabilities: baseCapabilities(),
	}
	if got := skuSalesExposure(core, false); len(got) != 0 {
		t.Fatalf("unknown CURRENT_7D must not create exposure: %#v", got)
	}
	if got := skuSalesAbsDelta(core, true); len(got) != 0 {
		t.Fatalf("unknown periods must not create absolute increase: %#v", got)
	}
	if got := skuSalesAbsDelta(core, false); len(got) != 0 {
		t.Fatalf("unknown periods must not create absolute decrease: %#v", got)
	}
	if got := skuSalesRelDelta(core, CurrentPolicy(), true); len(got) != 0 {
		t.Fatalf("unknown periods must not create relative increase: %#v", got)
	}
	if got := skuSalesFromZero(core); len(got) != 0 {
		t.Fatalf("unknown previous must not create FROM_ZERO: %#v", got)
	}
	if got := skuSalesToZero(core); len(got) != 0 {
		t.Fatalf("unknown current must not create TO_ZERO: %#v", got)
	}
}

func TestPrompt98SevenToThirteenDaysExposureWithoutComparison(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000)},
		SKUs: []SKUFeatures{{
			SemanticKey: "sku-mid", CategorySemanticKey: "c", SalesAvailable: true,
			OrderedAmountCurrent7:  ex(400_000),
			OrderedAmountPrevious7: exUnknown(),
		}},
		Capabilities: baseCapabilities(),
	}
	exp := skuSalesExposure(core, false)
	if len(exp) != 1 || exp[0].SemanticKey != "sku-mid" {
		t.Fatalf("KNOWN CURRENT_7D must allow SALES_CURRENT_EXPOSURE: %#v", exp)
	}
	if got := skuSalesAbsDelta(core, true); len(got) != 0 {
		t.Fatalf("unknown PREVIOUS_7D must not create absolute increase: %#v", got)
	}
	if got := skuSalesAbsDelta(core, false); len(got) != 0 {
		t.Fatalf("unknown PREVIOUS_7D must not create absolute decrease: %#v", got)
	}
	if got := skuSalesRelDelta(core, CurrentPolicy(), true); len(got) != 0 {
		t.Fatalf("unknown PREVIOUS_7D must not create relative increase: %#v", got)
	}
	if got := skuSalesRelDelta(core, CurrentPolicy(), false); len(got) != 0 {
		t.Fatalf("unknown PREVIOUS_7D must not create relative decrease: %#v", got)
	}
	if got := skuSalesFromZero(core); len(got) != 0 {
		t.Fatalf("unknown PREVIOUS_7D must not create FROM_ZERO: %#v", got)
	}
	if got := skuSalesToZero(core); len(got) != 0 {
		t.Fatalf("unknown PREVIOUS_7D must not create TO_ZERO: %#v", got)
	}
}

func TestPrompt98FullHistoryComparisonLanesRemain(t *testing.T) {
	core := CoreFacts{
		Account: AccountFeatures{SemanticKey: "acct", OrderedAmountCurrent7: ex(1_000_000), OrderedAmountPrevious7: ex(800_000)},
		SKUs: []SKUFeatures{{
			SemanticKey: "sku-full", CategorySemanticKey: "c", SalesAvailable: true,
			OrderedAmountCurrent7:  ex(400_000),
			OrderedAmountPrevious7: ex(100_000),
		}},
		Capabilities: baseCapabilities(),
	}
	if got := skuSalesExposure(core, false); len(got) != 1 {
		t.Fatalf("full history exposure: %#v", got)
	}
	if got := skuSalesAbsDelta(core, true); len(got) != 1 {
		t.Fatalf("full history absolute increase: %#v", got)
	}
}
