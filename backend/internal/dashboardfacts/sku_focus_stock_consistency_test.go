package dashboardfacts

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

func f64(v float64) *float64 { return &v }

func TestQualifiesStockRisk_IncludesPlanning(t *testing.T) {
	t.Parallel()
	rec := SKUFactRecord{
		OzonProductID:  1,
		StockAvailable: 20,
		DaysOfCover:    f64(10),
		CoverageBucket: stockpolicy.SegmentPlanning8_14,
		Revenue7d:      7000,
	}
	if !qualifiesStockRisk(rec) {
		t.Fatal("planning_8_14 must qualify for SKU Focus → Остатки")
	}
}

func TestBuildSKUFocusCandidates_StockCoverageInvariant(t *testing.T) {
	t.Parallel()

	idx := SKUFactsIndex{ByOzonProductID: map[string]SKUFactRecord{
		"1":  {OzonProductID: 1, StockAvailable: 0, DaysOfCover: f64(0), CoverageBucket: stockpolicy.SegmentOutOfStock, Revenue7d: 1000},
		"2":  {OzonProductID: 2, StockAvailable: 0, DaysOfCover: f64(0), CoverageBucket: stockpolicy.SegmentOutOfStock, Revenue7d: 900},
		"3":  {OzonProductID: 3, StockAvailable: 4, DaysOfCover: f64(1.5), CoverageBucket: stockpolicy.SegmentCritical0_3, Revenue7d: 2100},
		"4":  {OzonProductID: 4, StockAvailable: 5, DaysOfCover: f64(2.0), CoverageBucket: stockpolicy.SegmentCritical0_3, Revenue7d: 1800},
		"5":  {OzonProductID: 5, StockAvailable: 8, DaysOfCover: f64(5.0), CoverageBucket: stockpolicy.SegmentShortage4_7, Revenue7d: 1400},
		"6":  {OzonProductID: 6, StockAvailable: 12, DaysOfCover: f64(6.5), CoverageBucket: stockpolicy.SegmentShortage4_7, Revenue7d: 1200},
		"7":  {OzonProductID: 7, StockAvailable: 30, DaysOfCover: f64(9.0), CoverageBucket: stockpolicy.SegmentPlanning8_14, Revenue7d: 800},
		"8":  {OzonProductID: 8, StockAvailable: 40, DaysOfCover: f64(12.0), CoverageBucket: stockpolicy.SegmentPlanning8_14, Revenue7d: 600},
		"9":  {OzonProductID: 9, StockAvailable: 80, DaysOfCover: f64(25.0), CoverageBucket: stockpolicy.SegmentHealthy14Plus, Revenue7d: 500},
		"10": {OzonProductID: 10, StockAvailable: 50, DaysOfCover: f64(40.0), CoverageBucket: stockpolicy.SegmentNoSales, Revenue7d: 0, Orders7d: 0},
	}}

	// Mirror Stock Coverage counters from the same CoverageBucket field.
	coverageCounts := map[string]int{}
	for _, rec := range idx.ByOzonProductID {
		coverageCounts[rec.CoverageBucket]++
	}

	cands := BuildSKUFocusCandidates(idx)
	focusCounts := map[string]int{}
	for _, rec := range cands.StockRisk {
		focusCounts[rec.CoverageBucket]++
		if !stockpolicy.IsStockFocusStockTabSegment(rec.CoverageBucket) {
			t.Fatalf("unexpected segment in stock focus list: %s", rec.CoverageBucket)
		}
	}

	for _, seg := range []string{
		stockpolicy.SegmentOutOfStock,
		stockpolicy.SegmentCritical0_3,
		stockpolicy.SegmentShortage4_7,
		stockpolicy.SegmentPlanning8_14,
	} {
		if coverageCounts[seg] != focusCounts[seg] {
			t.Fatalf("%s: coverage=%d focus=%d", seg, coverageCounts[seg], focusCounts[seg])
		}
	}

	if focusCounts[stockpolicy.SegmentPlanning8_14] == 0 {
		t.Fatal("planning_8_14 SKUs must not be lost from SKU Focus")
	}

	wantAttention := coverageCounts[stockpolicy.SegmentOutOfStock] +
		coverageCounts[stockpolicy.SegmentCritical0_3] +
		coverageCounts[stockpolicy.SegmentShortage4_7]
	if cands.StockRiskTotal != wantAttention {
		t.Fatalf("StockRiskTotal=%d want attention %d", cands.StockRiskTotal, wantAttention)
	}
	if len(cands.StockRisk) != wantAttention+focusCounts[stockpolicy.SegmentPlanning8_14] {
		t.Fatalf("list len=%d want %d", len(cands.StockRisk), wantAttention+focusCounts[stockpolicy.SegmentPlanning8_14])
	}

	// One SKU → one segment; no duplicates in focus list.
	seen := map[int64]string{}
	for _, rec := range cands.StockRisk {
		if prev, ok := seen[rec.OzonProductID]; ok {
			t.Fatalf("SKU %d in multiple segments: %s and %s", rec.OzonProductID, prev, rec.CoverageBucket)
		}
		seen[rec.OzonProductID] = rec.CoverageBucket
	}
}

func TestCompareStockFocusRecords_DeterministicOrder(t *testing.T) {
	t.Parallel()
	a := SKUFactRecord{OzonProductID: 1, CoverageBucket: stockpolicy.SegmentCritical0_3, DaysOfCover: f64(2), Revenue7d: 100}
	b := SKUFactRecord{OzonProductID: 2, CoverageBucket: stockpolicy.SegmentOutOfStock, DaysOfCover: f64(0), Revenue7d: 50}
	c := SKUFactRecord{OzonProductID: 3, CoverageBucket: stockpolicy.SegmentPlanning8_14, DaysOfCover: f64(10), Revenue7d: 200}
	if !compareStockFocusRecords(b, a) {
		t.Fatal("out_of_stock must rank before critical")
	}
	if !compareStockFocusRecords(a, c) {
		t.Fatal("critical must rank before planning")
	}
}
