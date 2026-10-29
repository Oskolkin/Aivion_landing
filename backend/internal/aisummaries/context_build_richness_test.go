package aisummaries

import "testing"

func TestClassifyCategoryDataRichness_oneSKUSmallStockNoSalesIsLow(t *testing.T) {
	t.Parallel()
	if g := classifyCategoryDataRichness(1, 0, 0, 0, 10); g != "low" {
		t.Fatalf("got %q want low", g)
	}
}

func TestClassifyCategoryDataRichness_oneSKUWithRecsNotLow(t *testing.T) {
	t.Parallel()
	if g := classifyCategoryDataRichness(1, 0, 0, 1, 10); g == "low" {
		t.Fatalf("expected non-low when recommendations present, got %q", g)
	}
}
