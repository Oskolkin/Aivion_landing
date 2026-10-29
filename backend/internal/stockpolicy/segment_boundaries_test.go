package stockpolicy

import "testing"

func cov(v float64) *float64 { return &v }

func baseInput(stock int64, coverage float64) Input {
	return Input{
		AvailableStock:  stock,
		HasStockData:    true,
		CoverageDays:    cov(coverage),
		Orders7D:        10,
		HasSalesHistory: true,
	}
}

func TestClassifyCoverageSegment_Boundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        Input
		want      string
		attention bool
		stockTab  bool
	}{
		{"stock_zero", baseInput(0, 0), SegmentOutOfStock, true, true},
		{"coverage_0_with_stock", baseInput(5, 0), SegmentCritical0_3, true, true},
		{"coverage_0_1", baseInput(5, 0.1), SegmentCritical0_3, true, true},
		{"coverage_3", baseInput(5, 3), SegmentCritical0_3, true, true},
		{"coverage_3_1", baseInput(5, 3.1), SegmentShortage4_7, true, true},
		{"coverage_7", baseInput(5, 7), SegmentShortage4_7, true, true},
		{"coverage_7_1", baseInput(5, 7.1), SegmentPlanning8_14, false, true},
		{"coverage_14", baseInput(5, 14), SegmentPlanning8_14, false, true},
		{"coverage_14_1", baseInput(5, 14.1), SegmentHealthy14Plus, false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyCoverageSegment(tc.in)
			if got != tc.want {
				t.Fatalf("segment=%q want %q", got, tc.want)
			}
			if IsStockFocusAttentionSegment(got) != tc.attention {
				t.Fatalf("attention=%v want %v for %s", IsStockFocusAttentionSegment(got), tc.attention, got)
			}
			if IsStockFocusStockTabSegment(got) != tc.stockTab {
				t.Fatalf("stockTab=%v want %v for %s", IsStockFocusStockTabSegment(got), tc.stockTab, got)
			}
		})
	}
}

func TestClassifyCoverageSegment_MutuallyExclusive(t *testing.T) {
	t.Parallel()
	segments := []string{
		ClassifyCoverageSegment(baseInput(0, 0)),
		ClassifyCoverageSegment(baseInput(5, 2)),
		ClassifyCoverageSegment(baseInput(5, 5)),
		ClassifyCoverageSegment(baseInput(5, 10)),
		ClassifyCoverageSegment(baseInput(5, 30)),
		ClassifyCoverageSegment(baseInput(5, 100)),
		ClassifyCoverageSegment(Input{
			AvailableStock: 40, HasStockData: true, Orders7D: 0,
			HasSalesHistory: true, CoverageDays: cov(40),
			DaysWithoutSales: func() *int64 { v := int64(30); return &v }(),
		}),
	}
	seen := map[string]struct{}{}
	for _, s := range segments {
		if s == "" {
			t.Fatal("empty segment")
		}
		seen[s] = struct{}{}
	}
	if len(seen) != len(segments) {
		t.Fatalf("expected unique segments, got %v", segments)
	}
}
