package stockpolicy

import "testing"

func TestComputeCanonicalStockStatus_CoverageBuckets(t *testing.T) {
	t.Parallel()
	cov := func(v float64) *float64 { return &v }
	dws := func(v int64) *int64 { return &v }

	cases := []struct {
		name string
		in   Input
		want string
	}{
		{
			name: "zero_stock_with_demand",
			in:   Input{AvailableStock: 0, HasStockData: true, HasSalesHistory: true, Orders7D: 5, CoverageDays: cov(0)},
			want: StatusOutOfStock,
		},
		{
			name: "critical_2d",
			in:   Input{AvailableStock: 6, HasStockData: true, CoverageDays: cov(2), Orders7D: 10, HasSalesHistory: true},
			want: StatusCriticalShortage,
		},
		{
			name: "shortage_5d",
			in:   Input{AvailableStock: 20, HasStockData: true, CoverageDays: cov(5), Orders7D: 10, HasSalesHistory: true},
			want: StatusShortage7D,
		},
		{
			name: "planning_10d",
			in:   Input{AvailableStock: 40, HasStockData: true, CoverageDays: cov(10), Orders7D: 10, HasSalesHistory: true},
			want: StatusPlanning14D,
		},
		{
			name: "healthy_30d",
			in:   Input{AvailableStock: 90, HasStockData: true, CoverageDays: cov(30), Orders7D: 10, HasSalesHistory: true},
			want: StatusHealthy,
		},
		{
			name: "surplus_100d",
			in:   Input{AvailableStock: 500, HasStockData: true, CoverageDays: cov(100), Orders7D: 5, HasSalesHistory: true},
			want: StatusSurplus120D,
		},
		{
			name: "no_sales_persistent",
			in:   Input{AvailableStock: 40, HasStockData: true, CoverageDays: cov(40), Orders7D: 0, DaysWithoutSales: dws(30), HasSalesHistory: true},
			want: StatusNoSales,
		},
		{
			name: "raw_no_sales_overridden_by_active_sales",
			in: Input{
				AvailableStock: 40, HasStockData: true, CoverageDays: cov(20), Orders7D: 12,
				HasSalesHistory: true, OzonTurnoverGrade: "NO_SALES", DaysWithoutSales: dws(0),
			},
			want: StatusHealthy,
		},
		{
			name: "raw_deficit_does_not_override_healthy_coverage",
			in: Input{
				AvailableStock: 200, HasStockData: true, CoverageDays: cov(45), Orders7D: 8,
				HasSalesHistory: true, OzonTurnoverGrade: "DEFICIT",
			},
			want: StatusHealthy,
		},
		{
			name: "no_stock_data",
			in:   Input{HasStockData: false},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeCanonicalStockStatus(tc.in)
			if got.Status != tc.want {
				t.Fatalf("status=%q want %q (source=%s conflicts=%v)", got.Status, tc.want, got.Source, got.ConflictFlags)
			}
		})
	}
}

func TestClampDaysWithoutSalesWithOrders7D(t *testing.T) {
	t.Parallel()
	if got := ClampDaysWithoutSalesWithOrders7D(30, 5); got != 6 {
		t.Fatalf("got %d want 6", got)
	}
	if got := ClampDaysWithoutSalesWithOrders7D(2, 5); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
	if got := ClampDaysWithoutSalesWithOrders7D(30, 0); got != 30 {
		t.Fatalf("got %d want 30", got)
	}
}

func TestDaysWithoutSalesFromObserved(t *testing.T) {
	t.Parallel()
	orders := map[string]int64{
		"2026-07-08": 0,
		"2026-07-07": 0,
		"2026-07-06": 3,
	}
	dates := []string{"2026-07-08", "2026-07-07", "2026-07-06"}
	if got := DaysWithoutSalesFromObserved(orders, dates, 3); got != 2 {
		t.Fatalf("got %d want 2", got)
	}
	// Sale on metric date → 0
	orders["2026-07-08"] = 1
	if got := DaysWithoutSalesFromObserved(orders, dates, 5); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}
