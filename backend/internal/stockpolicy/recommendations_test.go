package stockpolicy

import (
	"math"
	"testing"
	"time"
)

func TestCoverageDays7D_CanonicalCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		stock  int64
		sales  int64
		want   *float64
		approx float64
	}{
		{"stock0_sales19", 0, 19, cov(0), 0},
		{"stock2_sales7", 2, 7, nil, 2.0},
		{"stock5_sales7", 5, 7, nil, 5.0},
		{"stock20_sales12", 20, 12, nil, 11.67},
		{"stock90_sales7", 90, 7, nil, 90.0},
		{"sales0", 10, 0, nil, -1},
		{"stock0_sales0", 0, 0, nil, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CoverageDays7D(tc.stock, tc.sales)
			if tc.approx < 0 {
				if got != nil {
					t.Fatalf("got=%v want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil")
			}
			if math.Abs(*got-tc.approx) > 0.02 {
				t.Fatalf("got=%.4f want≈%.2f", *got, tc.approx)
			}
		})
	}
}

func TestRecommendedSupplyUnits(t *testing.T) {
	t.Parallel()
	// avg=1/day, target=14 → need 14; stock=2, transit=3 → need 9
	got := RecommendedSupplyUnits(2, 3, 0, 1.0, 14)
	if got != 9 {
		t.Fatalf("got=%d want 9", got)
	}
	if RecommendedSupplyUnits(20, 0, 0, 1.0, 14) != 0 {
		t.Fatal("over-covered must yield 0")
	}
	if RecommendedSupplyUnits(0, 0, 0, 0, 14) != 0 {
		t.Fatal("no velocity must yield 0")
	}
}

func TestDefaultTargetCoverageDays(t *testing.T) {
	t.Parallel()
	days, source := DefaultTargetCoverageDays()
	if days != 14 {
		t.Fatalf("days=%v want 14", days)
	}
	if source != TargetSourcePolicyDefault {
		t.Fatalf("source=%q", source)
	}
}

func TestEstimatedStockoutAndOrderDate(t *testing.T) {
	t.Parallel()
	asOf := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	stockout := EstimatedStockoutDate(asOf, 14, 2.0) // 7 days
	if stockout == nil {
		t.Fatal("nil stockout")
	}
	wantStockout := asOf.Add(7 * 24 * time.Hour)
	if !stockout.Equal(wantStockout) {
		t.Fatalf("stockout=%v want %v", stockout, wantStockout)
	}
	order := RecommendedOrderDate(asOf, stockout, 7)
	if order == nil || !order.Equal(asOf) {
		t.Fatalf("order=%v want asOf (window already tight)", order)
	}
}
