package dashboardfacts

import (
	"math"
	"testing"
)

func TestComputeAccountCTRPercent_IsSumOfSumsNotMeanOfSKU(t *testing.T) {
	t.Parallel()
	// Two SKUs with different CTRs: 10% and 50%. Mean would be 30%.
	// Aggregate: (100+50)/(1000+100) = 150/1100 ≈ 13.64%.
	sku1Imp, sku1Views := int64(1000), int64(100)
	sku2Imp, sku2Views := int64(100), int64(50)
	got, ok := computeAccountCTRPercent(sku1Imp+sku2Imp, sku1Views+sku2Views)
	if !ok || got == nil {
		t.Fatal("expected CTR")
	}
	want := math.Round((150.0/1100.0)*10000) / 100
	if math.Abs(*got-want) > 0.001 {
		t.Fatalf("got=%.4f want=%.4f", *got, want)
	}
	meanOfSKU := (10.0 + 50.0) / 2
	if math.Abs(*got-meanOfSKU) < 0.01 {
		t.Fatalf("CTR must not be mean of SKU CTRs: got=%.2f mean=%.2f", *got, meanOfSKU)
	}
}

func TestComputeAccountCTRPercent_ZeroImpressionsNil(t *testing.T) {
	t.Parallel()
	if _, ok := computeAccountCTRPercent(0, 10); ok {
		t.Fatal("expected false for zero impressions")
	}
}
